const fs = require('fs');
const http = require('http');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const root = path.resolve(__dirname, '..');
const sourceAgentDir = path.join(root, 'HDU-Smart-Course-Agent');
const distAgentDir = path.join(root, 'dist');
const releaseAgentDir = path.join(root, 'smart-agent');
const explicitAgentExe = process.env.HDU_SMART_AGENT_EXE ? path.resolve(process.env.HDU_SMART_AGENT_EXE) : '';
const agentDir = fs.existsSync(path.join(sourceAgentDir, 'HDU-Smart-Course-Agent.exe'))
  ? sourceAgentDir
  : fs.existsSync(path.join(distAgentDir, 'HDU-Smart-Course-Agent.exe'))
    ? distAgentDir
    : releaseAgentDir;
const agentExe = explicitAgentExe || path.join(agentDir, 'HDU-Smart-Course-Agent.exe');
const browserPath = [
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
].find((candidate) => fs.existsSync(candidate));

const smartAgentPort = String(process.env.HDU_SMART_AGENT_PORT || '6899').trim();
if (!/^[1-9]\d{0,4}$/.test(smartAgentPort) || Number(smartAgentPort) > 65535) {
  throw new Error(`Invalid HDU_SMART_AGENT_PORT: ${smartAgentPort}`);
}
const appBaseURL = `http://127.0.0.1:${smartAgentPort}`;
const appURL = `${appBaseURL}/`;
const tempRoot = path.join(os.tmpdir(), `hdu-smart-agent-ui-smoke-${process.pid}`);
const tempScheduler = path.join(tempRoot, 'Scheduler');
const tempDownloads = path.join(tempRoot, 'Downloads');
const tempKillCourse = path.join(tempRoot, 'KillCourse');
const tempEntry = path.join(tempKillCourse, 'cmd', 'HDU-KillCourse');
const screenshotPath = path.join(tempRoot, 'smart-agent-ui-smoke.png');
const screenshotProfile = path.join(tempRoot, 'screenshot-profile');
const keepArtifacts = process.env.HDU_SMART_AGENT_UI_KEEP_ARTIFACTS === '1';

if (!fs.existsSync(agentExe)) {
  throw new Error(`Smart Agent exe not found: ${agentExe}`);
}

function writeStep(message) {
  console.log(`[smart-agent-ui-smoke] ${message}`);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
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
      pending.delete(message.id);
      if (message.error) request.reject(new Error(message.error.message || JSON.stringify(message.error)));
      else request.resolve(message.result);
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
      return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
    },
    close() {
      socket.close();
    },
  };
}

async function devtoolsTargets(port) {
  const response = await requestText(`http://127.0.0.1:${port}/json`);
  return JSON.parse(response.body);
}

async function evaluate(cdp, expression) {
  const result = await cdp.send('Runtime.evaluate', {
    awaitPromise: true,
    returnByValue: true,
    expression,
  });
  if (result.exceptionDetails) {
    const detail = result.exceptionDetails.exception?.description || result.exceptionDetails.exception?.value || result.exceptionDetails.text || 'browser evaluation failed';
    throw new Error(String(detail));
  }
  return result.result?.value;
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

function writeText(file, text) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, text, 'utf8');
}

function requestText(url, options = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request(url, { method: options.method || 'GET' }, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => resolve({ status: res.statusCode, body }));
    });
    req.on('error', reject);
    if (options.body) req.write(options.body);
    req.end();
  });
}

async function requestJSON(url, options = {}) {
  const response = await requestText(url, options);
  return {
    status: response.status,
    json: JSON.parse(response.body),
    body: response.body,
  };
}

async function waitForHTTP(url, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await requestText(url);
      if (response.status && response.status < 500) return response;
    } catch {
      await sleep(250);
    }
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function terminateProcess(child) {
  if (!child || child.killed || child.exitCode !== null) return;
  child.kill();
  await Promise.race([
    new Promise((resolve) => child.once('exit', resolve)),
    sleep(3000),
  ]);
}

async function removeTempRoot() {
  for (let attempt = 0; attempt < 6; attempt += 1) {
    try {
      fs.rmSync(tempRoot, { recursive: true, force: true });
      return;
    } catch (error) {
      if (attempt === 5) throw error;
      await sleep(500);
    }
  }
}

function prepareTempWorkspace() {
  fs.rmSync(tempRoot, { recursive: true, force: true });
  fs.mkdirSync(tempScheduler, { recursive: true });
  fs.mkdirSync(tempDownloads, { recursive: true });
  fs.mkdirSync(tempEntry, { recursive: true });

  const courseCandidates = [
    process.env.HDU_COURSE_FIXTURE
      ? path.resolve(process.env.HDU_COURSE_FIXTURE)
      : '',
    path.join(root, 'course.json'),
    path.join(root, 'testdata', 'course.sample.json'),
    path.join(root, 'samples', 'course.sample.json'),
  ].filter(Boolean);
  const source = courseCandidates.find((candidate) => fs.existsSync(candidate));
  if (!source) {
    throw new Error(
      'HDU_COURSE_FIXTURE, course.json, testdata/course.sample.json, or samples/course.sample.json is required for Smart Agent UI smoke test.',
    );
  }
  const coursePayload = JSON.parse(fs.readFileSync(source, 'utf8'));
  coursePayload.currentRound = 1;
  const sortedFixtureItems = [...(coursePayload.items || [])].sort((left, right) => {
    const leftCode = String(left.displayCode || left.jxbmc || left.sectionName || '');
    const rightCode = String(right.displayCode || right.jxbmc || right.sectionName || '');
    return leftCode < rightCode ? -1 : leftCode > rightCode ? 1 : 0;
  });
  const knownFixtureCodes = new Map(sortedFixtureItems.slice(0, 8).map((item, index) => [
    String(item.displayCode || item.jxbmc || item.sectionName || ''),
    index,
  ]));
  coursePayload.items = (coursePayload.items || []).map((item) => {
    const code = String(item.displayCode || item.jxbmc || item.sectionName || '');
    const fixtureIndex = knownFixtureCodes.get(code);
    if (fixtureIndex === undefined) return item;
    return {
      ...item,
      courseCode: item.courseCode || item.kch_id || item.kchId || '',
      selectEnabled: fixtureIndex !== 1,
      dropEnabled: fixtureIndex !== 2,
      selectRounds: [1, 2],
      jxbrl: '80',
      jxbrs: '52',
      xkrs: '51',
      jxbzc: item.jxbzc || `测试班级${fixtureIndex + 1}`,
    };
  });
  writeText(path.join(tempScheduler, 'course.json'), JSON.stringify(coursePayload, null, 2));
  writeText(path.join(tempScheduler, 'target-schedule.json'), JSON.stringify({
    schemaVersion: 1,
    source: 'ui-smoke-target',
    term: '2026-2027-1',
    exportedAt: new Date().toISOString(),
    items: (coursePayload.items || []).slice(0, 1),
  }, null, 2));
  writeText(path.join(tempEntry, 'main.go'), 'package main\nfunc main() {}\n');
  writeText(path.join(tempKillCourse, 'config.json'), JSON.stringify({
    cas_login: { username: '24000000', password: 'secret' },
    cookies: { enabled: '0' },
    course: {},
    wait_course: { enabled: '1', interval: 30 },
    time: { XueNian: '2026', XueQi: '1' },
    start_time: '2026-07-20 12:00:00',
  }, null, 2));
}

async function saveTemporarySettings() {
  const body = JSON.stringify({
    schedulerDir: tempScheduler,
    killCourseDir: tempKillCourse,
    autoRefresh: false,
    refreshIntervalSeconds: 60,
  });
  const response = await requestText(`${appBaseURL}/api/settings`, {
    method: 'POST',
    body,
  });
  if (response.status !== 200) {
    throw new Error(`Saving Smart Agent settings failed: HTTP ${response.status} ${response.body}`);
  }
  const payload = JSON.parse(response.body);
  if (!payload.ok) {
    throw new Error(`Saving Smart Agent settings failed: ${payload.error || response.body}`);
  }
}

async function assertInitialPageAndAPIs() {
  const html = await requestText(appURL);
  if (html.status !== 200 || !html.body.includes('HDU 智能选课执行助手')) {
    throw new Error(`Smart Agent HTML did not render expected title. HTTP ${html.status}`);
  }
  for (const expected of [
    '确认数据',
    '确认计划',
    '执行准备',
    '结果与替代方案',
  ]) {
    if (!html.body.includes(expected)) {
      throw new Error(`Smart Agent HTML is missing expected text: ${expected}`);
    }
  }
  if (!html.body.includes('live-file') || !html.body.includes('stage-primary') || !html.body.includes('settings-toggle')) {
    throw new Error('Smart Agent HTML is missing progressive workflow controls.');
  }
  for (const expected of [
    'target-source',
    'target-updated-at',
    'refresh-live',
    'auto-refresh',
    'refresh-interval',
    'last-refresh-at',
    'next-refresh-at',
  ]) {
    if (!html.body.includes(expected)) {
      throw new Error(`Smart Agent HTML is missing schedule sync control: ${expected}`);
    }
  }
  for (const expected of [
    'role="tablist"',
    'role="tab"',
    'role="tabpanel"',
    'for="confirm-phrase"',
    'aria-live="polite"',
  ]) {
    if (!html.body.includes(expected)) {
      throw new Error(`Smart Agent HTML is missing accessibility semantics: ${expected}`);
    }
  }
  for (const expected of [
    'course-option-select',
    'course-schedule-query',
    'course-intel-detail',
    'course-capacity-summary',
  ]) {
    if (!html.body.includes(expected)) {
      throw new Error(`Smart Agent HTML is missing course intelligence control: ${expected}`);
    }
  }

  const css = await requestText(`${appBaseURL}/style.css`);
  const js = await requestText(`${appBaseURL}/app.js`);
  if (css.status !== 200 || !css.body.includes('.stage-tab')) throw new Error('Smart Agent CSS did not load.');
  if (!css.body.includes(':focus-visible')) throw new Error('Smart Agent CSS is missing visible keyboard focus styles.');
  if (js.status !== 200 || !js.body.includes('renderWorkflow')) throw new Error('Smart Agent JS did not load.');
  if (!js.body.includes('handleSettingsKeydown') || !js.body.includes("event.key === 'Escape'")) {
    throw new Error('Smart Agent JS is missing keyboard-accessible settings drawer behavior.');
  }
  for (const expected of ['/api/target-schedule', '/api/live-schedule/refresh', 'refreshIntervalSeconds', 'refreshAfterSuccess', "reason: 'execution-success'"]) {
    if (!js.body.includes(expected)) {
      throw new Error(`Smart Agent JS is missing schedule sync integration: ${expected}`);
    }
  }
  for (const expected of ['min="10"', 'max="7200"', 'value="60"', '<span>秒</span>']) {
    if (!html.body.includes(expected)) {
      throw new Error(`Smart Agent HTML is missing second-based refresh setting: ${expected}`);
    }
  }
  for (const expected of ['course-options', 'class-schedule', 'course-capacity']) {
    if (!js.body.includes(expected)) {
      throw new Error(`Smart Agent JS is missing course intelligence API integration: ${expected}`);
    }
  }

  await saveTemporarySettings();
  const status = await requestJSON(`${appBaseURL}/api/status`);
  if (!status.json.courseExists || !status.json.killCourseExists || status.json.courseCount < 1) {
    throw new Error(`Unexpected Smart Agent status: ${JSON.stringify(status.json)}`);
  }
  const course = await requestJSON(`${appBaseURL}/api/course`);
  if (!Array.isArray(course.json.items) || course.json.items.length < 4) {
    throw new Error('Smart Agent course API did not return enough courses.');
  }
  const options = await requestJSON(`${appBaseURL}/api/course-options`);
  if (options.status !== 200 || options.json.ok !== true || !Array.isArray(options.json.items) || options.json.items.length < 4) {
    throw new Error(`Smart Agent course options API did not return expected shape: ${options.body}`);
  }
  const capacity = await requestJSON(`${appBaseURL}/api/course-capacity`);
  if (capacity.status !== 200 || capacity.json.ok !== true || !Array.isArray(capacity.json.items) || capacity.json.items.length < 4) {
    throw new Error(`Smart Agent course capacity API did not return expected shape: ${capacity.body}`);
  }
  if (options.json.currentRound !== 1 || capacity.json.items[0].capacity !== 80 || capacity.json.items[0].remaining !== 28 || capacity.json.items[0].full !== false || capacity.json.stale !== true || !capacity.json.sourceUpdatedAt) {
    throw new Error(`Smart Agent course intelligence API did not preserve known fixture values: ${JSON.stringify({ currentRound: options.json.currentRound, option: options.json.items[0], capacity: capacity.json.items[0] })}`);
  }
  const firstCode = options.json.items[0]?.displayCode;
  const schedule = await requestJSON(`${appBaseURL}/api/class-schedule?displayCode=${encodeURIComponent(firstCode || '')}`);
  if (schedule.status !== 200 || schedule.json.ok !== true || !Array.isArray(schedule.json.items) || schedule.json.items.length < 1) {
    throw new Error(`Smart Agent class schedule API did not return expected shape: ${schedule.body}`);
  }
  const live = await requestJSON(`${appBaseURL}/api/live-schedule`);
  if (live.status !== 200 || live.json.ok !== true || !Array.isArray(live.json.items)) {
    throw new Error(`Smart Agent live schedule API did not return expected shape: ${live.body}`);
  }
  const target = await requestJSON(`${appBaseURL}/api/target-schedule`);
  if (target.status !== 200 || target.json.ok !== true || target.json.exists !== true || target.json.items?.length !== 1) {
    throw new Error(`Smart Agent target schedule auto-discovery did not return expected data: ${target.body}`);
  }
  return {
    term: status.json.term,
    courseCount: status.json.courseCount,
    htmlBytes: Buffer.byteLength(html.body, 'utf8'),
    cssBytes: Buffer.byteLength(css.body, 'utf8'),
    jsBytes: Buffer.byteLength(js.body, 'utf8'),
  };
}

async function captureScreenshotIfPossible() {
  if (!browserPath) {
    return { skipped: true, reason: 'No Edge/Chrome executable found.' };
  }

  writeStep('Capturing headless browser screenshot...');
  const browser = spawn(browserPath, [
    '--headless=new',
    '--disable-gpu',
    '--disable-software-rasterizer',
    '--disable-extensions',
    '--no-first-run',
    '--no-default-browser-check',
    '--hide-scrollbars',
    '--run-all-compositor-stages-before-draw',
    '--window-size=1440,1200',
    `--user-data-dir=${screenshotProfile}`,
    `--screenshot=${screenshotPath}`,
    appURL,
  ], { stdio: 'ignore' });

  const exitCode = await Promise.race([
    new Promise((resolve) => browser.once('exit', resolve)),
    sleep(15000).then(() => 'timeout'),
  ]);
  if (exitCode === 'timeout') {
    await terminateProcess(browser);
    return { skipped: true, reason: 'Timed out capturing Smart Agent screenshot.' };
  }
  if (exitCode !== 0) {
    return { skipped: true, reason: `Browser screenshot command failed with exit code ${exitCode}.` };
  }
  const size = fs.existsSync(screenshotPath) ? fs.statSync(screenshotPath).size : 0;
  if (size < 10000) {
    return { skipped: true, reason: `Smart Agent UI screenshot is missing or too small: ${size} bytes.` };
  }
  return { skipped: false, path: screenshotPath, bytes: size };
}

async function startInteractiveBrowser(viewport, index) {
  const port = await findFreePort();
  const profile = path.join(tempRoot, `interactive-profile-${index}`);
  const screenshot = path.join(tempRoot, `smart-agent-ui-${viewport.name}.png`);
  fs.mkdirSync(profile, { recursive: true });
  const browser = spawn(browserPath, [
    '--headless=new',
    '--disable-gpu',
    '--disable-software-rasterizer',
    '--disable-extensions',
    '--no-first-run',
    '--no-default-browser-check',
    `--window-size=${viewport.width},${viewport.height}`,
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${profile}`,
    appURL,
  ], { stdio: 'ignore', windowsHide: true });

  let page;
  const deadline = Date.now() + 12000;
  while (!page && Date.now() < deadline) {
    try {
      const targets = await devtoolsTargets(port);
      page = targets.find((target) => target.type === 'page' && target.url.startsWith(appURL));
    } catch {}
    if (!page) await sleep(150);
  }
  if (!page) {
    await terminateProcess(browser);
    throw new Error(`No Smart Agent browser page for ${viewport.name}.`);
  }

  const cdp = connectCDP(page.webSocketDebuggerUrl);
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
    const message = event.entry?.text || 'console error';
    if (event.entry?.level === 'error' && !/favicon\.ico/i.test(message)) pageErrors.push(message);
  });
  await evaluate(cdp, 'window.__hduUiErrors = []; window.addEventListener("error", (event) => window.__hduUiErrors.push(String(event.message || event.error || "error")));');
  await waitFor(cdp, 'document.readyState === "complete" && document.querySelector("#settings-toggle") !== null', 'Smart Agent page controls', 20000);
  return { browser, cdp, pageErrors, screenshot, viewport };
}

async function assertInteractiveUI(session) {
  const { cdp, pageErrors, screenshot, viewport } = session;
  const drawerOpen = await evaluate(cdp, `(() => {
    const toggle = document.querySelector('#settings-toggle');
    toggle.focus();
    toggle.click();
    return {
      hidden: document.querySelector('#settings-drawer')?.getAttribute('aria-hidden'),
      activeId: document.activeElement?.id,
    };
  })()`);
  if (drawerOpen.hidden !== 'false' || drawerOpen.activeId !== 'settings-close') {
    throw new Error(`Smart Agent settings drawer did not move focus on open: ${JSON.stringify(drawerOpen)}`);
  }
  const drawerClosed = await evaluate(cdp, `(() => {
    document.querySelector('#settings-drawer')?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    return {
      hidden: document.querySelector('#settings-drawer')?.getAttribute('aria-hidden'),
      activeId: document.activeElement?.id,
    };
  })()`);
  if (drawerClosed.hidden !== 'true' || drawerClosed.activeId !== 'settings-toggle') {
    throw new Error(`Smart Agent settings drawer did not restore focus on Escape: ${JSON.stringify(drawerClosed)}`);
  }
  await waitFor(cdp, 'document.querySelector("#course-option-select")?.options?.length > 0 && !document.querySelector("#course-option-select").disabled', 'course option selector', 30000);
  await waitFor(cdp, 'document.querySelector("#target-count")?.textContent.includes("1")', 'automatic target schedule import');
  const syncControls = await evaluate(cdp, `(() => ({
    refresh: Boolean(document.querySelector('#refresh-live')),
    autoRefresh: Boolean(document.querySelector('#auto-refresh')),
    interval: Boolean(document.querySelector('#refresh-interval')),
    last: Boolean(document.querySelector('#last-refresh-at')),
    next: Boolean(document.querySelector('#next-refresh-at')),
  }))()`);
  if (!syncControls.refresh || !syncControls.autoRefresh || !syncControls.interval || !syncControls.last || !syncControls.next) {
    throw new Error(`Smart Agent schedule sync controls are incomplete: ${JSON.stringify(syncControls)}`);
  }
  await waitFor(cdp, 'document.querySelector("#course-intel-detail")?.textContent.includes("容量")', 'course detail');
  await evaluate(cdp, `(() => {
    const select = document.querySelector('#course-option-select');
    select.selectedIndex = 0;
    select.dispatchEvent(new Event('change', { bubbles: true }));
    return select.value;
  })()`);
  await waitFor(cdp, 'document.querySelector("#course-schedule-query")?.disabled === false', 'course schedule query button');
  await evaluate(cdp, 'document.querySelector("#course-schedule-query").click()');
  await waitFor(cdp, 'document.querySelectorAll("#class-schedule-result .schedule-item").length > 0', 'class schedule result');
  const result = await evaluate(cdp, `(() => {
    const visible = (element) => {
      if (!element) return false;
      const style = getComputedStyle(element);
      return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0' && element.getClientRects().length > 0;
    };
    const controls = [...document.querySelectorAll('button, input, select, textarea')]
      .filter((element) => element.type !== 'file' && element.type !== 'checkbox' && !element.classList.contains('visually-hidden') && visible(element))
      .map((element) => {
        const rect = element.getBoundingClientRect();
        return { label: (element.textContent || element.getAttribute('aria-label') || element.id || '').trim(), left: rect.left, right: rect.right, top: rect.top, bottom: rect.bottom, width: rect.width, height: rect.height };
      })
      .filter((item) => item.width > 16 && item.height > 16);
    const overlaps = [];
    for (let left = 0; left < controls.length; left += 1) {
      for (let right = left + 1; right < controls.length; right += 1) {
        const a = controls[left];
        const b = controls[right];
        if (Math.min(a.right, b.right) - Math.max(a.left, b.left) > 1 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 1) overlaps.push({ a, b });
      }
    }
    return {
      optionCount: document.querySelector('#course-option-select').options.length,
      detail: document.querySelector('#course-intel-detail').textContent,
      capacitySummary: document.querySelector('#course-capacity-summary').textContent,
      scheduleCount: document.querySelectorAll('#class-schedule-result .schedule-item').length,
      scrollWidth: document.documentElement.scrollWidth,
      bodyScrollWidth: document.body.scrollWidth,
      innerWidth: window.innerWidth,
      overlaps,
      controls,
      errors: window.__hduUiErrors || [],
    };
  })()`);
  if (result.optionCount < 1 || result.scheduleCount < 1) throw new Error(`Smart Agent course UI did not render expected data: ${JSON.stringify(result)}`);
  if (!result.detail.includes('容量') || !result.detail.includes('可选') || !result.detail.includes('第1轮') || !result.detail.includes('80 人')) {
    throw new Error(`Smart Agent course detail is incomplete: ${result.detail}`);
  }
  if (!result.capacitySummary.includes('非实时') || !result.capacitySummary.includes('快照更新时间')) {
    throw new Error(`Smart Agent capacity freshness semantics are incomplete: ${result.capacitySummary}`);
  }
  if (result.scrollWidth > result.innerWidth + 1 || result.bodyScrollWidth > result.innerWidth + 1) throw new Error(`Smart Agent ${viewport.name} UI overflows horizontally: ${JSON.stringify(result)}`);
  if (result.overlaps.length) throw new Error(`Smart Agent ${viewport.name} UI controls overlap: ${JSON.stringify(result.overlaps.slice(0, 3))}`);
  if (result.errors.length || pageErrors.length) throw new Error(`Smart Agent ${viewport.name} UI errors: ${[...pageErrors, ...result.errors].join('; ')}`);
  const image = await cdp.send('Page.captureScreenshot', { format: 'png', fromSurface: true });
  fs.writeFileSync(screenshot, Buffer.from(image.data, 'base64'));
  const bytes = fs.statSync(screenshot).size;
  if (bytes < 10000) throw new Error(`Smart Agent ${viewport.name} screenshot is too small: ${bytes}`);
  return { viewport: viewport.name, optionCount: result.optionCount, scheduleCount: result.scheduleCount, screenshot, bytes };
}

async function captureInteractiveUI() {
  if (!browserPath) return { skipped: true, reason: 'No Edge/Chrome executable found.' };
  const viewports = [
    { name: 'desktop', width: 1440, height: 1000, mobile: false },
    { name: 'mobile', width: 390, height: 844, mobile: true },
  ];
  const results = [];
  for (const [index, viewport] of viewports.entries()) {
    const session = await startInteractiveBrowser(viewport, index);
    try {
      results.push(await assertInteractiveUI(session));
    } finally {
      session.cdp.close();
      await terminateProcess(session.browser);
    }
  }
  return { skipped: false, results };
}

async function main() {
  prepareTempWorkspace();
  writeStep('Starting Smart Agent...');
  const agent = spawn(agentExe, {
    cwd: tempRoot,
    env: { ...process.env, HDU_AGENT_NO_BROWSER: '1', HDU_AGENT_DOWNLOADS_DIR: tempDownloads },
    stdio: 'ignore',
    windowsHide: true,
  });

  let mainError;
  try {
    writeStep('Waiting for Smart Agent API...');
    await waitForHTTP(`${appBaseURL}/api/status`);
    writeStep('Checking page resources and APIs...');
    const page = await assertInitialPageAndAPIs();
    const screenshot = await captureScreenshotIfPossible();
    const interactive = await captureInteractiveUI();
    writeStep('Passed.');
    console.log(JSON.stringify({ page, screenshot, interactive }, null, 2));
  } catch (error) {
    mainError = error;
  } finally {
    try {
      await requestText(`${appBaseURL}/api/settings`, { method: 'DELETE' });
    } catch {
      // Best-effort cleanup only.
    }
    await terminateProcess(agent);
    if (!keepArtifacts) {
      try {
        await removeTempRoot();
      } catch (cleanupError) {
        if (!mainError) mainError = cleanupError;
        else console.error(`Cleanup warning: ${cleanupError.message || cleanupError}`);
      }
    } else {
      writeStep(`Keeping UI artifacts at ${tempRoot}`);
    }
    if (mainError) throw mainError;
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
