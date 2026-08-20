const fs = require('fs');
const http = require('http');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const root = path.resolve(__dirname, '..');
const sourceAgentDir = path.join(root, 'HDU-Smart-Course-Agent');
const explicitAgentExe = process.env.HDU_SMART_AGENT_EXE ? path.resolve(process.env.HDU_SMART_AGENT_EXE) : '';
const agentExe = explicitAgentExe || path.join(sourceAgentDir, 'HDU-Smart-Course-Agent.exe');
if (!fs.existsSync(agentExe)) {
  throw new Error(`Smart Agent exe not found: ${agentExe}`);
}

const tempRoot = path.join(os.tmpdir(), `hdu-class-schedule-check-${process.pid}`);
const schedulerDir = path.join(tempRoot, 'Scheduler');
const killDir = path.join(tempRoot, 'KillCourse');

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

function request(base, pathname) {
  return new Promise((resolve) => {
    const req = http.get(base + pathname, (res) => {
      let data = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        try { resolve({ status: res.statusCode, body: JSON.parse(data) }); }
        catch (error) { resolve({ status: res.statusCode, body: data }); }
      });
    });
    req.on('error', (error) => resolve({ status: 0, error }));
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
  fs.writeFileSync(path.join(tempRoot, 'agent-settings.json'), JSON.stringify({ schedulerDir, killCourseDir: killDir }));

  const agent = spawn(agentExe, {
    cwd: tempRoot,
    env: { ...process.env, HDU_AGENT_NO_BROWSER: '1', HDU_SMART_AGENT_PORT: String(port) },
    stdio: 'ignore',
    windowsHide: true,
  });

  try {
    await waitHTTP(base, '/api/status');
    const options = (await request(base, '/api/class-options')).body;
    if (!options.ok || options.total < 3 || !options.items.some((item) => item.name === '202601' && item.count >= 2)) {
      throw new Error(`Unexpected class options: ${JSON.stringify(options)}`);
    }
    const class202601 = (await request(base, `/api/class-schedule?className=${encodeURIComponent('202601')}`)).body;
    const class202602 = (await request(base, `/api/class-schedule?className=${encodeURIComponent('202602')}`)).body;
    if (!class202601.ok || class202601.items.length !== 2) {
      throw new Error(`Expected 2 courses for 202601, got ${JSON.stringify(class202601)}`);
    }
    if (!class202602.ok || class202602.items.length !== 1) {
      throw new Error(`Expected 1 course for 202602, got ${JSON.stringify(class202602)}`);
    }
    console.log(JSON.stringify({
      ok: true,
      classOptionsTotal: options.total,
      class202601: class202601.items.length,
      class202602: class202602.items.length,
    }, null, 2));
  } finally {
    await terminate(agent);
    try {
      fs.rmSync(tempRoot, { recursive: true, force: true });
    } catch (error) {
      // Best-effort cleanup only.
    }
  }
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
