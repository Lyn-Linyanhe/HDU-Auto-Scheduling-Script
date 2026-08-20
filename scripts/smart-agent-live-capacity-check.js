const fs = require('fs');
const http = require('http');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const root = path.resolve(__dirname, '..');
const agentExe = process.env.HDU_SMART_AGENT_EXE
  ? path.resolve(process.env.HDU_SMART_AGENT_EXE)
  : path.join(root, 'HDU-Smart-Course-Agent', 'HDU-Smart-Course-Agent.exe');
const testLabBase = process.env.HDU_TESTLAB_BASE;
const scenario = process.env.HDU_TESTLAB_SCENARIO || 'capacity-ok';
if (!fs.existsSync(agentExe)) throw new Error(`Smart Agent exe not found: ${agentExe}`);
if (!testLabBase) throw new Error('HDU_TESTLAB_BASE is required');
if (!/^https?:\/\/127\.0\.0\.1:\d+$/.test(testLabBase)) {
  throw new Error('HDU_TESTLAB_BASE must be a loopback URL like http://127.0.0.1:PORT');
}

const tempRoot = path.join(os.tmpdir(), `hdu-live-capacity-check-${process.pid}`);
const schedulerDir = path.join(tempRoot, 'Scheduler');
const killDir = path.join(tempRoot, 'KillCourse');
const liveCapacityPath = path.join(tempRoot, 'live-capacity.json');

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

function request(base, pathname, postBody) {
  return new Promise((resolve) => {
    const body = postBody === undefined ? null : JSON.stringify(postBody);
    const req = http.request(base + pathname, {
      method: body ? 'POST' : 'GET',
      headers: body ? { 'Content-Type': 'application/json' } : {},
    }, (res) => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        try { resolve({ status: res.statusCode, body: JSON.parse(data) }); }
        catch (error) { resolve({ status: res.statusCode, body: data }); }
      });
    });
    req.on('error', (error) => resolve({ status: 0, error }));
    if (body) req.write(body);
    req.end();
  });
}

async function waitHTTP(base, pathname, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const response = await request(base, pathname);
    if (response.status > 0 && response.body && response.body.ok !== false) return response;
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  throw new Error(`Smart Agent did not become ready at ${base}${pathname}`);
}

function terminate(processHandle) {
  return new Promise((resolve) => {
    if (!processHandle || processHandle.exitCode !== null) return resolve();
    processHandle.once('exit', resolve);
    processHandle.kill();
  });
}

async function main() {
  const port = await findFreePort();
  const base = `http://127.0.0.1:${port}`;
  fs.mkdirSync(schedulerDir, { recursive: true });
  fs.mkdirSync(killDir, { recursive: true });
  const course = JSON.parse(fs.readFileSync(path.join(root, 'testdata', 'course.sample.json'), 'utf8'));
  fs.writeFileSync(path.join(schedulerDir, 'course.json'), JSON.stringify(course));
  const killConfig = {
    cas_login: { username: 'test-user', password: 'test-password', dingDingQrLoginEnabled: '0', level: '1' },
    newjw_login: { username: 'test-user', password: 'test-password', level: '0' },
    user_agent: 'Mozilla/5.0 test',
    cookies: { JSESSIONID: '', route: '', enabled: '1' },
    time: { XueNian: '2026', XueQi: '1' },
    course: {},
    wait_course: { interval: 60, enabled: '0' },
    smtp_email: { host: '', username: '', password: '', to: '', enabled: '0' },
    start_time: '2026-08-21 12:00:00',
  };
  killConfig.course[course.items[0].displayCode || '(2026-2027-1)-A0001001-01'] = '1';
  fs.writeFileSync(path.join(killDir, 'config.json'), JSON.stringify(killConfig));
  fs.writeFileSync(path.join(tempRoot, 'agent-settings.json'), JSON.stringify({ schedulerDir, killCourseDir: killDir }));

  const agent = spawn(agentExe, {
    cwd: tempRoot,
    env: { ...process.env, HDU_AGENT_NO_BROWSER: '1', HDU_SMART_AGENT_PORT: String(port), HDU_KILLCOURSE_BASE_URL: testLabBase },
    stdio: 'ignore',
    windowsHide: true,
  });
  try {
    await waitHTTP(base, '/api/status');

    const refresh = (await request(base, '/api/course/live-capacity/refresh', {})).body;
    const read = (await request(base, '/api/course/live-capacity')).body;

    if (scenario === 'capacity-ok') {
      if (!refresh.ok) throw new Error(`refresh failed: ${JSON.stringify(refresh)}`);
      if (!(refresh.count >= 1) || !(refresh.rows >= 1) || !(refresh.queryCount >= 1)) {
        throw new Error(`refresh counts unexpected: ${JSON.stringify(refresh)}`);
      }
      if (!fs.existsSync(liveCapacityPath)) {
        throw new Error(`live-capacity.json was not written`);
      }
      if (!read.ok || read.source !== 'live' || read.stale === true) {
        throw new Error(`expected live source, got: ${JSON.stringify(read)}`);
      }
      if (!Array.isArray(read.items) || read.items.length < 2) {
        throw new Error(`expected at least 2 live items, got ${JSON.stringify(read.items)}`);
      }
      const byCode = new Map((read.items || []).map((item) => [item.displayCode, item]));
      const first = byCode.get('(2026-2027-1)-A0001001-01');
      const second = byCode.get('(2026-2027-1)-A0002001-01');
      if (!first || first.capacity !== 80 || first.enrolled !== 52 || first.selected !== 51 || first.remaining !== 29) {
        throw new Error(`first live item wrong: ${JSON.stringify(first)}`);
      }
      if (!second || second.capacity !== 60 || second.selected !== 10 || second.remaining !== 50) {
        throw new Error(`second live item wrong: ${JSON.stringify(second)}`);
      }
      console.log(JSON.stringify({ ok: true, check: 'live_capacity', scenario, count: refresh.count, items: read.items.length }, null, 2));
    } else if (scenario === 'capacity-fail') {
      if (refresh.ok) throw new Error(`refresh should have failed, got: ${JSON.stringify(refresh)}`);
      if (!refresh.error) throw new Error(`refresh failure should carry a reason: ${JSON.stringify(refresh)}`);
      if (!read.ok || read.source !== 'course.json' || read.stale !== true) {
        throw new Error(`expected course.json fallback, got: ${JSON.stringify(read)}`);
      }
      if (!read.warnings || !read.warnings.some((w) => /实时容量/.test(w))) {
        throw new Error(`expected a live-unavailable warning, got: ${JSON.stringify(read.warnings)}`);
      }
      console.log(JSON.stringify({ ok: true, check: 'live_capacity', scenario, refreshError: refresh.error, fallbackSource: read.source }, null, 2));
    } else {
      throw new Error(`unknown HDU_TESTLAB_SCENARIO: ${scenario}`);
    }
  } finally {
    await terminate(agent);
    try { fs.rmSync(tempRoot, { recursive: true, force: true }); } catch {}
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
