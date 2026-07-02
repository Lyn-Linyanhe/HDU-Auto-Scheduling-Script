const fs = require('fs');
const http = require('http');
const path = require('path');
const { spawn } = require('child_process');

const root = path.resolve(__dirname, '..');
const edgePath = [
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
].find((candidate) => fs.existsSync(candidate));

if (!edgePath) {
  throw new Error('No Edge/Chrome executable found for screenshot smoke test.');
}

const port = 9223;
const appURL = 'http://127.0.0.1:6789/scheduler.html?v=smoke';
const downloadDir = path.join(root, '.tmp-screenshot-downloads');
const profileDir = path.join(root, '.tmp-edge-profile');

fs.rmSync(downloadDir, { recursive: true, force: true });
fs.rmSync(profileDir, { recursive: true, force: true });
fs.mkdirSync(downloadDir, { recursive: true });
fs.mkdirSync(profileDir, { recursive: true });

function getJSON(url) {
  return new Promise((resolve, reject) => {
    http.get(url, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => {
        try {
          resolve(JSON.parse(body));
        } catch (error) {
          reject(error);
        }
      });
    }).on('error', reject);
  });
}

async function waitForDevTools() {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    try {
      return await getJSON(`http://127.0.0.1:${port}/json`);
    } catch {
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
  }
  throw new Error('Timed out waiting for browser DevTools endpoint.');
}

function connect(wsURL) {
  const ws = new WebSocket(wsURL);
  let nextId = 1;
  const pending = new Map();
  ws.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);
    if (message.id && pending.has(message.id)) {
      const { resolve, reject } = pending.get(message.id);
      pending.delete(message.id);
      if (message.error) reject(new Error(message.error.message || JSON.stringify(message.error)));
      else resolve(message.result);
    }
  });
  const opened = new Promise((resolve, reject) => {
    ws.addEventListener('open', resolve, { once: true });
    ws.addEventListener('error', reject, { once: true });
  });
  return {
    opened,
    send(method, params = {}) {
      const id = nextId;
      nextId += 1;
      ws.send(JSON.stringify({ id, method, params }));
      return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
    },
    close() {
      ws.close();
    },
  };
}

function newestPNG() {
  return fs.readdirSync(downloadDir)
    .filter((name) => name.toLowerCase().endsWith('.png'))
    .map((name) => {
      const full = path.join(downloadDir, name);
      return { name, full, size: fs.statSync(full).size };
    })
    .sort((a, b) => b.size - a.size)[0];
}

async function main() {
  const browser = spawn(edgePath, [
    '--headless=new',
    '--disable-gpu',
    '--disable-extensions',
    '--no-first-run',
    '--no-default-browser-check',
    '--window-size=1440,1200',
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profileDir}`,
    appURL,
  ], { stdio: 'ignore' });

  try {
    const targets = await waitForDevTools();
    const page = targets.find((target) => target.type === 'page' && target.url.includes('/scheduler.html')) || targets.find((target) => target.type === 'page');
    if (!page) throw new Error('No browser page target found.');
    const cdp = connect(page.webSocketDebuggerUrl);
    await cdp.opened;
    await cdp.send('Page.enable');
    await cdp.send('Runtime.enable');
    await cdp.send('Browser.setDownloadBehavior', { behavior: 'allow', downloadPath: downloadDir });
    await cdp.send('Page.setDownloadBehavior', { behavior: 'allow', downloadPath: downloadDir });
    const buttonResult = await cdp.send('Runtime.evaluate', {
      awaitPromise: true,
      returnByValue: true,
      expression: `
        new Promise((resolve, reject) => {
          const deadline = Date.now() + 15000;
          const tick = () => {
            const table = document.getElementById('timetable');
            const button = [...document.querySelectorAll('button')].find((item) => item.textContent.includes('导出截图'));
            if (table && table.children.length > 10 && button) {
              button.scrollIntoView({ block: 'center', inline: 'center' });
              const rect = button.getBoundingClientRect();
              resolve({ cells: table.children.length, x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 });
              return;
            }
            if (Date.now() > deadline) {
              reject(new Error('Timed out waiting for timetable and screenshot button.'));
              return;
            }
            setTimeout(tick, 250);
          };
          tick();
        })
      `,
    });
    const point = buttonResult.result.value;
    await cdp.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: point.x, y: point.y, button: 'none' });
    await cdp.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: point.x, y: point.y, button: 'left', clickCount: 1 });
    await cdp.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: point.x, y: point.y, button: 'left', clickCount: 1 });

    const deadline = Date.now() + 10000;
    let png;
    while (Date.now() < deadline) {
      png = newestPNG();
      if (png && png.size > 10000) break;
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    cdp.close();
    if (!png || png.size <= 10000) {
      const diagnostics = await cdp.send('Runtime.evaluate', {
        returnByValue: true,
        expression: 'JSON.stringify({ buttonPoint: ' + JSON.stringify(point) + ', buttonTexts: [...document.querySelectorAll("button")].map((item) => item.textContent.trim()).filter(Boolean).slice(0, 20) })',
      });
      throw new Error(`Screenshot was not downloaded or is too small: ${png ? `${png.name} ${png.size}` : 'none'}; diagnostics=${diagnostics.result.value}`);
    }
    console.log(`Screenshot smoke test passed: ${png.name}, ${png.size} bytes.`);
  } finally {
    browser.kill();
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
