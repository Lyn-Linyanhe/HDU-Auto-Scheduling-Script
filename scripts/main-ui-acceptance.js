const fs = require('fs');
const http = require('http');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const root = path.resolve(__dirname, '..');
const browserPath = [
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
].find((candidate) => fs.existsSync(candidate));
const explicitExe = process.env.HDU_MAIN_EXE ? path.resolve(process.env.HDU_MAIN_EXE) : '';
const mainExe = explicitExe || path.join(root, 'dist', 'HDU-Auto-Scheduling-Script.exe');
const keepArtifacts = process.env.HDU_UI_KEEP_ARTIFACTS === '1';
const mainPort = String(process.env.HDU_MAIN_PORT || '6789').trim();
if (!/^[1-9]\d{0,4}$/.test(mainPort) || Number(mainPort) > 65535) {
  throw new Error(`Invalid HDU_MAIN_PORT: ${mainPort}`);
}
const mainBaseURL = `http://127.0.0.1:${mainPort}/`;
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'hdu-main-ui-'));
const courseSource = path.join(root, 'testdata', 'course.sample.json');
const personalSource = path.join(root, 'testdata', 'personal-schedule.sample.json');

if (!fs.existsSync(mainExe)) throw new Error(`Main executable not found: ${mainExe}`);
if (!browserPath) throw new Error('No Edge/Chrome executable found for UI acceptance.');

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function requestText(url, options = {}) {
  return new Promise((resolve, reject) => {
    const request = http.request(url, { method: options.method || 'GET' }, (response) => {
      let body = '';
      response.setEncoding('utf8');
      response.on('data', (chunk) => { body += chunk; });
      response.on('end', () => resolve({ status: response.statusCode, body }));
    });
    request.on('error', reject);
    if (options.body) request.write(options.body);
    request.end();
  });
}

async function waitForHTTP(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await requestText(url);
      if (response.status === 200) return response;
    } catch {
      await sleep(150);
    }
  }
  throw new Error(`Timed out waiting for ${url}`);
}

function findFreePort() {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const port = probe.address().port;
      probe.close(() => resolve(port));
    });
  });
}

function assertPortAvailable(port) {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', () => reject(new Error(`Required local port ${port} is already in use.`)));
    probe.listen(port, '127.0.0.1', () => probe.close(resolve));
  });
}

async function terminateProcess(child) {
  if (!child || child.killed || child.exitCode !== null) return;
  child.kill();
  await Promise.race([
    new Promise((resolve) => child.once('exit', resolve)),
    sleep(3000),
  ]);
}

function connectCDP(wsURL) {
  const socket = new WebSocket(wsURL);
  const pending = new Map();
  const listeners = new Map();
  let nextId = 1;
  const opened = new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);
    if (message.id && pending.has(message.id)) {
      const request = pending.get(message.id);
      const { resolve, reject } = request;
      pending.delete(message.id);
      if (message.error) reject(new Error(`${request.method || 'CDP command'}: ${message.error.message || JSON.stringify(message.error)}`));
      else resolve(message.result);
      return;
    }
    for (const listener of listeners.get(message.method) || []) listener(message.params || {});
  });
  return {
    opened,
    on(method, listener) {
      const entries = listeners.get(method) || [];
      entries.push(listener);
      listeners.set(method, entries);
    },
    send(method, params = {}) {
      const id = nextId++;
      socket.send(JSON.stringify({ id, method, params }));
      return new Promise((resolve, reject) => pending.set(id, { resolve, reject, method }));
    },
    close() {
      socket.close();
    },
  };
}

async function devtoolsTargets(port) {
  const response = await waitForHTTP(`http://127.0.0.1:${port}/json`, 10000);
  return JSON.parse(response.body);
}

async function startBrowser(url, viewport, screenshotPath, port, expectedPath) {
  let lastError;
  for (let attempt = 1; attempt <= 3; attempt += 1) {
    const attemptPort = attempt === 1 ? port : await findFreePort();
    const profile = path.join(tempRoot, `browser-${attempt}-${Date.now()}`);
    fs.mkdirSync(profile, { recursive: true });
    let browser;
    let cdp;
    try {
      browser = spawn(browserPath, [
        '--headless=new',
        '--disable-gpu',
        '--disable-extensions',
        '--no-first-run',
        '--no-default-browser-check',
        `--window-size=${viewport.width},${viewport.height}`,
        `--remote-debugging-port=${attemptPort}`,
        `--user-data-dir=${profile}`,
        url,
      ], { stdio: 'ignore', windowsHide: true });
      let page;
      const deadline = Date.now() + 15000;
      while (!page && Date.now() < deadline) {
        try {
          const targets = await devtoolsTargets(attemptPort);
          page = targets.find((target) => target.type === 'page' && target.url.includes(expectedPath));
        } catch {}
        if (!page) await sleep(150);
      }
      if (!page) {
        await terminateProcess(browser);
        browser = null;
        lastError = new Error(`No page target for ${url}`);
        await sleep(600);
        continue;
      }
      cdp = connectCDP(page.webSocketDebuggerUrl);
      await cdp.opened;
      await cdp.send('Page.enable');
      await cdp.send('Runtime.enable');
      await cdp.send('Log.enable');
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: viewport.width,
        height: viewport.height,
        deviceScaleFactor: 1,
        mobile: viewport.mobile,
      });
      const pageErrors = [];
      cdp.on('Runtime.exceptionThrown', (event) => pageErrors.push(event.exceptionDetails?.text || 'runtime exception'));
      cdp.on('Log.entryAdded', (event) => {
        if (event.entry?.level === 'error') pageErrors.push(event.entry.text || 'console error');
      });
      await cdp.send('Page.addScriptToEvaluateOnNewDocument', {
        source: 'window.__hduUiErrors = []; window.addEventListener("error", (event) => window.__hduUiErrors.push(String(event.message || event.error || "error")));',
      });
      return { browser, cdp, pageErrors, screenshotPath };
    } catch (error) {
      if (cdp) cdp.close();
      await terminateProcess(browser);
      lastError = error;
      await sleep(600);
    }
  }
  throw lastError || new Error(`No page target for ${url}`);
}

async function evaluate(cdp, expression) {
  const result = await cdp.send('Runtime.evaluate', {
    awaitPromise: true,
    returnByValue: true,
    expression,
  });
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || 'browser evaluation failed');
  }
  return result.result?.value;
}

async function setFileInput(cdp, selector, filePath) {
  const base64 = fs.readFileSync(filePath).toString('base64');
  const fileName = path.basename(filePath);
  await evaluate(cdp, `(() => {
    const input = document.querySelector(${JSON.stringify(selector)});
    if (!input) throw new Error('File input not found: ' + ${JSON.stringify(selector)});
    const bytes = Uint8Array.from(atob(${JSON.stringify(base64)}), (character) => character.charCodeAt(0));
    const transfer = new DataTransfer();
    transfer.items.add(new File([bytes], ${JSON.stringify(fileName)}, { type: 'application/json' }));
    input.files = transfer.files;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return input.files.length;
  })()`);
  await waitFor(cdp, `document.querySelector(${JSON.stringify(selector)})?.files?.length === 1`, 'file input selection');
}

async function installDownloadCapture(cdp) {
  await evaluate(cdp, `(() => {
    if (window.__hduDownloadCaptureInstalled) return true;
    const originalClick = HTMLAnchorElement.prototype.click;
    const originalRevoke = URL.revokeObjectURL.bind(URL);
    window.__hduDownloadCaptureInstalled = true;
    window.__hduDownload = null;
    URL.revokeObjectURL = (url) => {
      if (!String(url).startsWith('blob:')) originalRevoke(url);
    };
    HTMLAnchorElement.prototype.click = function(...args) {
      if (this.download && String(this.href).startsWith('blob:')) {
        window.__hduDownload = { href: this.href, name: this.download };
      }
      return originalClick.apply(this, args);
    };
    return true;
  })()`);
}

async function readCapturedDownload(cdp) {
  await waitFor(cdp, 'window.__hduDownload && window.__hduDownload.href', 'download capture');
  return evaluate(cdp, `(() => {
    const download = window.__hduDownload;
    return fetch(download.href).then((response) => response.text()).then((text) => ({ name: download.name, text }));
  })()`);
}

async function waitFor(cdp, predicate, label, timeoutMs = 10000) {
  const expression = `new Promise((resolve, reject) => {
    const deadline = Date.now() + ${timeoutMs};
    const tick = () => {
      try {
        if (${predicate}) { resolve(true); return; }
      } catch {}
      if (Date.now() >= deadline) { reject(new Error(${JSON.stringify(`Timed out waiting for ${label}`)})); return; }
      setTimeout(tick, 100);
    };
    tick();
  })`;
  await evaluate(cdp, expression);
}

async function waitForDownloadedFile(directory, previousNames = [], timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  const previous = new Set(previousNames);
  while (Date.now() < deadline) {
    const names = fs.existsSync(directory) ? fs.readdirSync(directory) : [];
    const candidate = names.find((name) => !previous.has(name) && name.endsWith('.json') && !name.endsWith('.crdownload'));
    if (candidate) return path.join(directory, candidate);
    await sleep(100);
  }
  throw new Error(`Timed out waiting for downloaded JSON in ${directory}`);
}

async function readProjectTimetable(directory, fileName, timeoutMs = 10000) {
  const filePath = path.join(directory, fileName);
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (fs.existsSync(filePath)) {
      const payload = JSON.parse(fs.readFileSync(filePath, 'utf8'));
      if (Array.isArray(payload.items)) return payload;
    }
    await sleep(100);
  }
  throw new Error(`Timed out waiting for project timetable ${filePath}`);
}

async function navigateAndWait(cdp, url, expectedPath) {
  await cdp.send('Page.navigate', { url });
  await waitFor(cdp, `location.pathname === ${JSON.stringify(expectedPath)}`, `path ${expectedPath}`, 10000);
  await waitFor(cdp, 'document.readyState === "complete"', 'document ready', 5000);
}

async function capture(cdp, screenshotPath) {
  const image = await cdp.send('Page.captureScreenshot', { format: 'png', fromSurface: true });
  fs.writeFileSync(screenshotPath, Buffer.from(image.data, 'base64'));
  if (fs.statSync(screenshotPath).size < 10000) throw new Error(`Screenshot too small: ${screenshotPath}`);
}

async function assertGeometry(cdp, scenario, viewport) {
  const geometry = await evaluate(cdp, `(() => {
    const visible = (element) => {
      if (!element) return false;
      const style = getComputedStyle(element);
      if (style.display === 'none' || style.visibility === 'hidden' || element.getClientRects().length === 0) return false;
      const rect = element.getBoundingClientRect();
      for (let ancestor = element.parentElement; ancestor && ancestor !== document.body; ancestor = ancestor.parentElement) {
        const ancestorStyle = getComputedStyle(ancestor);
        const clipsX = ['auto', 'hidden', 'scroll'].includes(ancestorStyle.overflowX);
        const clipsY = ['auto', 'hidden', 'scroll'].includes(ancestorStyle.overflowY);
        if (!clipsX && !clipsY) continue;
        const clip = ancestor.getBoundingClientRect();
        if ((clipsX && (rect.right <= clip.left || rect.left >= clip.right)) || (clipsY && (rect.bottom <= clip.top || rect.top >= clip.bottom))) return false;
      }
      return true;
    };
    const rects = [...document.querySelectorAll('button, input, select, textarea, a.ghost-btn, a.primary-btn')]
      .filter(visible)
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return { label: (element.textContent || element.getAttribute('aria-label') || element.id || '').trim(), left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height };
      });
    const overlaps = [];
    for (let left = 0; left < rects.length; left += 1) {
      for (let right = left + 1; right < rects.length; right += 1) {
        const a = rects[left];
        const b = rects[right];
        const horizontal = Math.min(a.right, b.right) - Math.max(a.left, b.left);
        const vertical = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (horizontal > 1 && vertical > 1) overlaps.push({ a, b });
      }
    }
    return { innerWidth: window.innerWidth, scrollWidth: document.documentElement.scrollWidth, bodyScrollWidth: document.body.scrollWidth, rects, overlaps, errors: window.__hduUiErrors || [] };
  })()`);
  if (geometry.scrollWidth > geometry.innerWidth + 1 || geometry.bodyScrollWidth > geometry.innerWidth + 1) {
    throw new Error(`${scenario}/${viewport.name} horizontal overflow: ${JSON.stringify(geometry)}`);
  }
  const outside = geometry.rects.filter((rect) => rect.left < -1 || rect.right > geometry.innerWidth + 1 || rect.width <= 0 || rect.height <= 0);
  if (outside.length) throw new Error(`${scenario}/${viewport.name} control outside viewport: ${JSON.stringify(outside.slice(0, 3))}`);
  if (geometry.overlaps.length) throw new Error(`${scenario}/${viewport.name} overlapping controls: ${JSON.stringify(geometry.overlaps.slice(0, 3))}`);
  if (geometry.errors.length) throw new Error(`${scenario}/${viewport.name} page errors: ${geometry.errors.join('; ')}`);
  return { scrollWidth: geometry.scrollWidth, innerWidth: geometry.innerWidth, controls: geometry.rects.length };
}

async function assertExporter(cdp) {
  await waitFor(cdp, 'document.querySelector("#export-form") && document.querySelector("#username") && document.querySelector("#password")', 'export form');
  const result = await evaluate(cdp, `(async () => ({
    form: Boolean(document.querySelector('#export-form')),
    controls: ['#username', '#password', '#xue-nian', '#xue-qi', '#export', '#refresh'].filter((selector) => document.querySelector(selector)).length,
    status: Boolean(document.querySelector('#message')),
    exportStatus: await fetch('/api/export/status').then((response) => response.ok),
  }))()`);
  if (!result.form || result.controls !== 6 || !result.status || !result.exportStatus) throw new Error(`Exporter surface incomplete: ${JSON.stringify(result)}`);
}

async function assertScheduler(cdp, withPersonal, scenarioDirectory) {
  const downloadDirectory = path.join(scenarioDirectory, 'downloads');
  fs.mkdirSync(downloadDirectory, { recursive: true });
  await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'course cards');
  await waitFor(cdp, 'document.querySelectorAll("#timetable > *").length >= 13', '13-period timetable');

  if (!withPersonal) {
    await evaluate(cdp, `(() => {
      const link = document.querySelector('a[href^="/exporter/"]');
      if (!link) throw new Error('scheduler exporter return link not found');
      link.click();
      return true;
    })()`);
    await waitFor(cdp, 'location.pathname === "/exporter/"', 'return to exporter');
    await waitFor(cdp, 'document.querySelector("#export-form") && document.querySelector("#username")', 'exporter after return');
    const returnedPath = await evaluate(cdp, 'new Promise((resolve) => setTimeout(() => resolve(location.pathname), 1200))');
    if (returnedPath !== '/exporter/') throw new Error(`Exporter page redirected unexpectedly: ${returnedPath}`);
    await evaluate(cdp, 'location.replace("/scheduler.html?v=v5")');
    await waitFor(cdp, 'location.pathname === "/scheduler.html"', 'return to scheduler after exporter check');
    await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'course cards after exporter check');
  }

  await installDownloadCapture(cdp);
  const result = await evaluate(cdp, `(() => ({
    smartAgentLink: (() => {
      const link = document.querySelector('a[href="http://127.0.0.1:6899/"]');
      return link ? { text: link.textContent.trim(), target: link.target, rel: link.rel } : null;
    })(),
    courses: document.querySelectorAll('#course-list .course-card').length,
    periods: document.querySelectorAll('#timetable > *').length,
    baseItems: document.querySelectorAll('#selected-list .pill.is-base').length,
    lockedItems: document.querySelectorAll('#selected-list .lock-btn.is-locked').length,
    hasSearch: Boolean(document.querySelector('#search-input')),
    hasClearBase: Boolean(document.querySelector('#clear-base')),
    hasExportCurrent: Boolean(document.querySelector('#export-current')),
    hasTimetableScroll: Boolean(document.querySelector('#timetable-scroll')),
    timetableWidth: document.querySelector('#timetable')?.scrollWidth || 0,
    hasLockedSummary: Boolean(document.querySelector('#locked-selected-picks')),
    hasLockedQuickPicks: Boolean(document.querySelector('#locked-quick-picks')),
    hasLockedSearch: Boolean(document.querySelector('#locked-search')),
    hasLockedSearchResults: Boolean(document.querySelector('#locked-search-results')),
    hasLegacyWarning: Boolean(document.querySelector('#legacy-course-lock-warning')),
    hasRemovedRequiredControls: !document.querySelector('#required-courses, #required-search, #required-search-results, #preferred-teachers'),
    lockControlOrder: (() => {
      const summary = document.querySelector('#locked-selected-picks');
      const quick = document.querySelector('#locked-quick-picks');
      const search = document.querySelector('#locked-search');
      return Boolean(summary && quick && search && (summary.compareDocumentPosition(quick) & Node.DOCUMENT_POSITION_FOLLOWING) && (quick.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING));
    })(),
    minCreditStep: document.querySelector('#min-credit')?.step || '',
    maxCreditStep: document.querySelector('#max-credit')?.step || '',
    schemeMode: document.querySelector('#scheme-mode')?.value || '',
    periodText: [...document.querySelectorAll('#timetable .time-cell')].map((item) => item.textContent).join('|'),
  }))()`);
  if (!result.smartAgentLink || !result.smartAgentLink.text.includes('HDU') || !result.smartAgentLink.text.includes('智能选课执行助手') || result.smartAgentLink.target !== '_blank' || !result.smartAgentLink.rel.includes('noopener') || !result.hasSearch || !result.hasClearBase || !result.hasExportCurrent || !result.hasTimetableScroll || result.timetableWidth < 748 || !result.hasLockedSummary || !result.hasLockedQuickPicks || !result.hasLockedSearch || !result.hasLockedSearchResults || !result.hasLegacyWarning || !result.hasRemovedRequiredControls || !result.lockControlOrder || result.minCreditStep !== '0.25' || result.maxCreditStep !== '0.25' || result.schemeMode !== 'teacher') {
    throw new Error(`Scheduler controls incomplete: ${JSON.stringify(result)}`);
  }
  if (!result.periodText.includes('18:30') || !result.periodText.includes('21:00')) throw new Error(`HDU period times missing: ${result.periodText}`);
  if (withPersonal && (!result.baseItems || result.lockedItems < result.baseItems)) {
    throw new Error(`Personal schedule was not auto-locked: ${JSON.stringify(result)}`);
  }
  if (!withPersonal && result.baseItems !== 0) throw new Error(`Course-only scenario unexpectedly has base items: ${JSON.stringify(result)}`);

  await evaluate(cdp, `(() => { const input = document.querySelector('#search-input'); input.value = 'A0001001'; input.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`);
  await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'course search result');
  await evaluate(cdp, `(() => { const input = document.querySelector('#search-input'); input.value = ''; input.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`);

  if (!withPersonal) {
    await evaluate(cdp, `(() => {
      localStorage.setItem('hdu-scheduler-state-v3', JSON.stringify({ requiredCourses: '(2026-2027-1)-A0001001-01' }));
      location.reload();
      return true;
    })()`);
    await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'scheduler after legacy migration reload');
    await waitFor(cdp, `(() => {
      const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
      const entry = Object.values(saved.selectedGroups || {}).find((item) => (item.items || []).some((course) => course.id === 'sample-math-01'));
      return entry?.lockedItemId === 'sample-math-01' && !Object.prototype.hasOwnProperty.call(saved, 'requiredCourses');
    })()`, 'unique legacy course lock migration');
    await evaluate(cdp, `(() => {
      localStorage.setItem('hdu-scheduler-state-v3', JSON.stringify({ requiredCourses: '高等数学A' }));
      location.reload();
      return true;
    })()`);
    await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'scheduler after ambiguous migration reload');
    await waitFor(cdp, `(() => {
      const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
      return (saved.legacyCourseLockWarnings || []).some((item) => item.includes('高等数学A'))
        && !Object.values(saved.selectedGroups || {}).some((item) => item.lockedItemId === 'sample-math-01' || item.lockedItemId === 'sample-math-02');
    })()`, 'ambiguous legacy course warning');
    await installDownloadCapture(cdp);

    await evaluate(cdp, `(() => { const button = document.querySelector('#course-list [data-toggle-course]'); if (!button) throw new Error('no course toggle'); button.click(); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#selected-list [data-remove]").length > 0', 'selected course');
    await evaluate(cdp, `(() => { const button = document.querySelector('#selected-list [data-lock]'); if (!button) throw new Error('no lock button'); button.click(); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#selected-list .lock-btn.is-locked").length > 0', 'locked course');
    await evaluate(cdp, `(() => { const button = document.querySelector('#selected-list [data-remove]'); button.click(); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#selected-list [data-remove]").length === 0', 'removed course');

    await evaluate(cdp, `(() => { const input = document.querySelector('#locked-search'); input.value = 'A0001001-01'; input.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#locked-search-results [data-search-lock]").length === 1', 'exact lock search result');
    await evaluate(cdp, `(() => { const button = document.querySelector('#locked-search-results [data-search-lock="sample-math-01"]'); if (!button) throw new Error('exact lock action not found'); button.click(); return true; })()`);
    await waitFor(cdp, `(() => {
      const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
      const entry = Object.values(saved.selectedGroups || {}).find((item) => (item.items || []).some((course) => course.id === 'sample-math-01'));
      return entry?.lockedItemId === 'sample-math-01';
    })()`, 'exact course lock action');
    await evaluate(cdp, `(() => { const button = document.querySelector('#selected-list [data-remove="sample-math:sample-math-01"]') || [...document.querySelectorAll('#selected-list [data-remove]')].find((item) => item.dataset.remove.endsWith(':sample-math-01')); if (!button) throw new Error('locked search course removal not found'); button.click(); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#selected-list [data-remove]").length === 0', 'removed searched lock course');
  } else {
    await evaluate(cdp, `(() => { const button = document.querySelector('#clear-base'); button.click(); return true; })()`);
    await waitFor(cdp, 'document.querySelectorAll("#selected-list .pill.is-base").length === 0', 'cleared base');
  }

  if (withPersonal) return;

  const reimportPath = path.join(scenarioDirectory, 'base-reimport.json');
  await evaluate(cdp, `(() => {
    const button = document.querySelector('[data-toggle-course="sample-math-01"]');
    if (!button) throw new Error('sample-math-01 toggle not found');
    button.click();
    return true;
  })()`);
  await waitFor(cdp, 'document.querySelectorAll("#selected-list [data-remove]").length === 1', 'manual duplicate selection');
  await setFileInput(cdp, '#base-file', reimportPath);
  try {
    await waitFor(cdp, `(() => {
      const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
      const entry = Object.values(saved.selectedGroups || {}).find((item) => (item.items || []).some((course) => course.id === 'sample-math-01'));
      return (saved.baseCourseIds || []).includes('sample-math-01') && entry?.lockedItemId === 'sample-math-01';
    })()`, 'duplicate base import');
  } catch (error) {
    const debug = await evaluate(cdp, `(() => ({
      files: [...(document.querySelector('#base-file')?.files || [])].map((file) => file.name),
      summary: document.querySelector('#base-summary')?.textContent || '',
      state: localStorage.getItem('hdu-scheduler-state-v3') || '',
    }))()`);
    throw new Error(`${error.message}; duplicate import debug: ${JSON.stringify(debug)}`);
  }
  await evaluate(cdp, `(() => { document.querySelector('#clear-base').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    const entry = Object.values(saved.selectedGroups || {}).find((item) => (item.items || []).some((course) => course.id === 'sample-math-01'));
    const manual = entry?.items?.find((course) => course.id === 'sample-math-01');
    return manual?.source === 'manual' && entry?.lockedItemId === '' && !(saved.baseCourseIds || []).length && !saved.baseScheduleName && saved.personalScheduleAutoImported === false;
  })()`, 'clear duplicate base state');

  await setFileInput(cdp, '#base-file', reimportPath);
  await waitFor(cdp, `(() => (JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}').baseCourseIds || []).includes('sample-math-01'))()`, 'base import before clear-selected');
  await evaluate(cdp, `(() => { document.querySelector('#clear-selected').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return Object.keys(saved.selectedGroups || {}).length === 0 && !(saved.baseCourseIds || []).length && !saved.baseScheduleName && saved.personalScheduleAutoImported === false;
  })()`, 'clear-selected state');

  const candidateIds = [
    'sample-math-01',
    'sample-math-02',
    'sample-english-01',
    'sample-data-01',
    'sample-data-02',
    'sample-compile-01',
    'sample-compile-lab-01',
    'sample-physics-odd',
    'sample-politics-even',
    'sample-night-01',
  ];
  for (const id of candidateIds) {
    await evaluate(cdp, `(() => {
      const button = document.querySelector(${JSON.stringify(`[data-toggle-course="${id}"]`)});
      if (!button) throw new Error(${JSON.stringify(`course toggle not found: ${id}`)});
      button.click();
      return true;
    })()`);
  }
  await waitFor(cdp, `document.querySelectorAll('#selected-list [data-remove]').length === ${candidateIds.length}`, 'candidate course selection');

  const lockedCandidateIds = [
    'sample-english-01',
    'sample-compile-01',
    'sample-compile-lab-01',
    'sample-physics-odd',
    'sample-politics-even',
    'sample-night-01',
  ];
  for (const id of lockedCandidateIds) {
    await evaluate(cdp, `(() => {
      const button = [...document.querySelectorAll('[data-lock]')].find((item) => item.dataset.lock.endsWith(${JSON.stringify(`:${id}`)}));
      if (!button) throw new Error(${JSON.stringify(`lock toggle not found: ${id}`)});
      button.click();
      return true;
    })()`);
  }
  await waitFor(cdp, `document.querySelectorAll('#selected-list .lock-btn.is-locked').length === ${lockedCandidateIds.length}`, 'candidate locked courses');

  await evaluate(cdp, `(() => { document.querySelector('#estimate').click(); return true; })()`);
  try {
    await waitFor(cdp, `(() => {
      const button = document.querySelector('#estimate');
      return !button.disabled && /9/.test(document.querySelector('#estimate-text')?.textContent || '');
    })()`, 'candidate estimate');
  } catch (error) {
    const debug = await evaluate(cdp, `(() => ({
      estimateText: document.querySelector('#estimate-text')?.textContent || '',
      estimateDisabled: document.querySelector('#estimate')?.disabled,
      selected: document.querySelectorAll('#selected-list [data-remove]').length,
      state: localStorage.getItem('hdu-scheduler-state-v3') || '',
    }))()`);
    throw new Error(`${error.message}; candidate estimate debug: ${JSON.stringify(debug)}`);
  }
  await evaluate(cdp, `(() => { document.querySelector('#generate').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const text = document.querySelector('#result-count')?.textContent || '';
    return !document.querySelector('#generate').disabled && /1\\s*\\/\\s*9/.test(text);
  })()`, 'candidate generation');
  let candidateState = await evaluate(cdp, `(() => ({
    result: document.querySelector('#result-count')?.textContent || '',
    tableResult: document.querySelector('#table-result-count')?.textContent || '',
    cards: document.querySelectorAll('#result-list .result-card').length,
    page: document.querySelector('#candidate-page')?.value || '',
    tablePage: document.querySelector('#table-candidate-page')?.value || '',
    title: document.querySelector('#candidate-title')?.textContent || '',
    returnDisabled: document.querySelector('#candidate-return')?.disabled,
    previewDisabled: document.querySelector('#candidate-preview')?.disabled,
  }))()`);
  if (!/1\s*\/\s*9/.test(candidateState.result) || !/1\s*\/\s*9/.test(candidateState.tableResult) || candidateState.cards !== 1 || candidateState.page !== '1' || candidateState.tablePage !== '1' || candidateState.returnDisabled || !candidateState.previewDisabled) {
    throw new Error(`Candidate generation state mismatch: ${JSON.stringify(candidateState)}`);
  }

  await evaluate(cdp, `(() => { document.querySelector('#candidate-return').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return document.querySelector('#candidate-return')?.disabled === true && document.querySelector('#candidate-preview')?.disabled === false && saved.candidatePreviewEnabled === false;
  })()`, 'return to original timetable');

  await evaluate(cdp, `(() => { document.querySelector('#candidate-next').click(); return true; })()`);
  await waitFor(cdp, `document.querySelector('#candidate-page')?.value === '2' && /2\\s*\\/\\s*9/.test(document.querySelector('#result-count')?.textContent || '')`, 'candidate next page');
  await evaluate(cdp, `(() => { document.querySelector('#candidate-preview').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return saved.candidateCursor === 1 && Boolean(saved.activeCandidate) && document.querySelector('#candidate-preview')?.disabled === true && document.querySelector('#candidate-return')?.disabled === false && saved.candidatePreviewEnabled === true && document.querySelectorAll('#timetable [data-open-course]').length > 0;
  })()`, 'preview candidate two');
  await evaluate(cdp, `(() => {
    const input = document.querySelector('#candidate-page');
    input.value = '3';
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return true;
  })()`);
  await waitFor(cdp, `document.querySelector('#candidate-page')?.value === '3' && /3\\s*\\/\\s*9/.test(document.querySelector('#result-count')?.textContent || '')`, 'candidate page input');
  await evaluate(cdp, `(() => {
    const input = document.querySelector('#candidate-page');
    input.value = '1.5';
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return true;
  })()`);
  await waitFor(cdp, `document.querySelector('#candidate-page')?.value === '3' && /3\\s*\\/\\s*9/.test(document.querySelector('#result-count')?.textContent || '')`, 'invalid candidate page ignored');

  await evaluate(cdp, `(() => { document.querySelector('#export-current').click(); return true; })()`);
  const candidateExport = await readProjectTimetable(scenarioDirectory, 'target-schedule.json');
  const candidateExportIds = [...new Set((candidateExport.items || []).map((item) => item.id || item.sectionId || item.jxb_id).filter(Boolean))];
  if (candidateExport.source !== 'candidate' || !candidateExport.items?.length || candidateExportIds.length !== candidateExport.items.length) {
    throw new Error(`Candidate export mismatch: ${JSON.stringify(candidateExport)}`);
  }

  await evaluate(cdp, `(() => { document.querySelector('#candidate-favorite').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return (saved.favoriteCandidates || []).length === 1;
  })()`, 'favorite candidate');
  await evaluate(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    saved.activeCandidate = '';
    saved.candidateCursor = 0;
    saved.candidatePreviewEnabled = false;
    localStorage.setItem('hdu-scheduler-state-v3', JSON.stringify(saved));
    return true;
  })()`);
  await cdp.send('Page.reload', { ignoreCache: true });
  await waitFor(cdp, 'document.querySelectorAll("#course-list .course-card").length > 0', 'reload candidate state');
  await installDownloadCapture(cdp);
  await evaluate(cdp, `(() => { document.querySelector('#candidate-favorites-view').click(); return true; })()`);
  await waitFor(cdp, 'document.querySelectorAll("#result-list .result-card").length === 1 && document.querySelectorAll("#result-list [data-favorite-preview]").length === 1 && document.querySelectorAll("#result-list [data-favorite-remove]").length === 1', 'favorite list');
  await evaluate(cdp, `(() => { document.querySelector('#result-list [data-favorite-preview]').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return saved.resultListMode === 'current' && saved.candidateCursor === 2 && saved.candidatePreviewEnabled === true;
  })()`, 'preview favorite candidate');
  await evaluate(cdp, `(() => { document.querySelector('#candidate-favorites-view').click(); return true; })()`);
  await waitFor(cdp, 'document.querySelectorAll("#result-list [data-favorite-remove]").length === 1', 'favorite list reopen');
  await evaluate(cdp, `(() => { document.querySelector('#result-list [data-favorite-remove]').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return (saved.favoriteCandidates || []).length === 0 && document.querySelectorAll('#result-list .result-card').length === 0;
  })()`, 'remove favorite candidate');
  await evaluate(cdp, `(() => { document.querySelector('#candidate-favorites-view').click(); return true; })()`);

  for (let index = 0; index < 9; index += 1) {
    await evaluate(cdp, `(() => { document.querySelector('#candidate-dismiss').click(); return true; })()`);
    await waitFor(cdp, `document.querySelector('#candidate-dismiss')?.disabled === ${index === 8 ? 'true' : 'false'}`, `dismiss candidate ${index + 1}`);
  }
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return saved.candidatePreviewEnabled === false && document.querySelector('#candidate-return')?.disabled === true && document.querySelector('#result-count')?.textContent.includes('0') && document.querySelector('#result-list')?.textContent.includes('\u5df2\u5220\u9664');
  })()`, 'dismiss last candidate');

  await evaluate(cdp, `(() => { document.querySelector('#export-current').click(); return true; })()`);
  const currentExport = await readProjectTimetable(scenarioDirectory, 'hdu-current-timetable.json');
  const currentDownloadPath = path.join(downloadDirectory, 'hdu-current-timetable.json');
  fs.writeFileSync(currentDownloadPath, JSON.stringify(currentExport, null, 2), 'utf8');
  const currentExportIds = [...new Set((currentExport.items || []).map((item) => item.id || item.sectionId || item.jxb_id).filter(Boolean))];
  if (currentExport.source !== 'current' || !currentExport.items?.length || currentExportIds.length !== currentExport.items.length) {
    throw new Error(`Current export mismatch: ${JSON.stringify(currentExport)}`);
  }
  await setFileInput(cdp, '#base-file', currentDownloadPath);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    const locked = Object.values(saved.selectedGroups || {}).flatMap((entry) => entry.lockedItemId ? [entry.lockedItemId] : []);
    return JSON.stringify([...new Set(saved.baseCourseIds || [])].sort()) === JSON.stringify(${JSON.stringify(currentExportIds.slice().sort())})
      && JSON.stringify([...new Set(locked)].sort()) === JSON.stringify(${JSON.stringify(currentExportIds.slice().sort())});
  })()`, 'exact current export reimport');
  await evaluate(cdp, `(() => { document.querySelector('#clear-base').click(); return true; })()`);
  await waitFor(cdp, `(() => {
    const saved = JSON.parse(localStorage.getItem('hdu-scheduler-state-v3') || '{}');
    return !(saved.baseCourseIds || []).length && !saved.baseScheduleName && saved.personalScheduleAutoImported === false && Object.values(saved.selectedGroups || {}).every((entry) => !entry.lockedItemId);
  })()`, 'clear reimported current timetable');
}

function prepareScenario(scenario) {
  const directory = path.join(tempRoot, scenario);
  fs.mkdirSync(directory, { recursive: true });
  if (scenario !== 'no-data') fs.copyFileSync(courseSource, path.join(directory, 'course.json'));
  if (scenario === 'course-only') {
    const coursePayload = JSON.parse(fs.readFileSync(courseSource, 'utf8'));
    const duplicate = coursePayload.items.find((item) => item.jxb_id === 'sample-math-01');
    fs.writeFileSync(path.join(directory, 'base-reimport.json'), JSON.stringify({ schemaVersion: 1, items: [duplicate] }, null, 2));
  }
  if (scenario === 'course-and-personal') fs.copyFileSync(personalSource, path.join(directory, 'personal-schedule.json'));
  return directory;
}

async function runScenario(scenario, viewport) {
  const directory = prepareScenario(scenario);
  const stdout = path.join(directory, 'stdout.log');
  const stderr = path.join(directory, 'stderr.log');
  const port = await findFreePort();
  const server = spawn(mainExe, { cwd: directory, env: { ...process.env, HDU_NO_BROWSER: '1', HDU_MAIN_PORT: mainPort, HDU_OUTPUT_DIR: directory }, stdio: ['ignore', fs.openSync(stdout, 'w'), fs.openSync(stderr, 'w')], windowsHide: true });
  const baseURL = mainBaseURL;
  const screenshotPath = path.join(directory, `${viewport.name}.png`);
  try {
    await waitForHTTP(`${baseURL}api/status`);
    const expectedPath = scenario === 'no-data' ? '/exporter/' : '/scheduler.html';
    const browser = await startBrowser(baseURL, viewport, screenshotPath, port, expectedPath);
    try {
      await waitFor(browser.cdp, `location.pathname === ${JSON.stringify(expectedPath)}`, `path ${expectedPath}`, 10000);
      await waitFor(browser.cdp, 'document.readyState === "complete"', 'document ready', 5000);
      if (scenario === 'no-data') await assertExporter(browser.cdp);
      else await assertScheduler(browser.cdp, scenario === 'course-and-personal', directory);
      const geometry = await assertGeometry(browser.cdp, scenario, viewport);
      await capture(browser.cdp, screenshotPath);
      return { scenario, viewport: viewport.name, path: expectedPath, screenshot: screenshotPath, geometry };
    } finally {
      browser.cdp.close();
      await terminateProcess(browser.browser);
    }
  } finally {
    await terminateProcess(server);
  }
}

async function main() {
  const viewports = [
    { name: 'desktop', width: 1440, height: 1000, mobile: false },
    { name: 'mobile', width: 390, height: 844, mobile: true },
  ];
  const results = [];
  for (const scenario of ['no-data', 'course-only', 'course-and-personal']) {
    for (const viewport of viewports) {
      await assertPortAvailable(Number(mainPort));
      results.push(await runScenario(scenario, viewport));
    }
  }
  console.log(JSON.stringify({ passed: true, results, artifacts: keepArtifacts ? tempRoot : 'removed after pass' }, null, 2));
  if (!keepArtifacts) fs.rmSync(tempRoot, { recursive: true, force: true });
}

main().catch((error) => {
  console.error(error);
  if (!keepArtifacts) {
    try { fs.rmSync(tempRoot, { recursive: true, force: true }); } catch {}
  } else {
    console.error(`Artifacts: ${tempRoot}`);
  }
  process.exit(1);
});
