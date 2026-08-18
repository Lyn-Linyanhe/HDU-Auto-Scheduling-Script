const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const main = fs.readFileSync(path.join(root, 'HDU-Smart-Course-Agent', 'main.go'), 'utf8');
const app = fs.readFileSync(path.join(root, 'HDU-Smart-Course-Agent', 'web', 'app.js'), 'utf8');
const html = fs.readFileSync(path.join(root, 'HDU-Smart-Course-Agent', 'web', 'index.html'), 'utf8');

function assertContains(source, fragment, label) {
  if (!source.includes(fragment)) throw new Error(`${label} is missing: ${fragment}`);
}

assertContains(main, 'RefreshIntervalSeconds', 'backend second-based settings');
assertContains(main, 'RefreshAfterSuccess', 'execution refresh response');
assertContains(app, 'refreshIntervalSeconds', 'frontend second-based settings');
assertContains(app, 'refreshAfterSuccess', 'frontend execution refresh trigger');
if (app.includes('previousItems !== nextSignature')) {
  throw new Error('target schedule refresh still references undefined previousItems');
}
assertContains(app, 'previousSignature !== nextSignature', 'target schedule change comparison');
const refreshTimestampStart = app.indexOf('function latestRefreshTimestamp');
const refreshTimestampEnd = app.indexOf('function clearLiveRefreshTimer');
const refreshTimestamp = refreshTimestampStart >= 0 && refreshTimestampEnd > refreshTimestampStart
  ? app.slice(refreshTimestampStart, refreshTimestampEnd)
  : '';
if (refreshTimestamp.includes('refreshAttemptAt')) {
  throw new Error('failed refresh attempts must not reset the successful refresh timer');
}
assertContains(app, "const refreshed = await refreshLiveSchedule({ reason: 'execution-success' });", 'successful execution refresh result');
assertContains(app, 'if (refreshed) state.lastExecutionRefreshKey = refreshKey;', 'execution refresh retry dedupe');
const signatureStart = app.indexOf('function courseItemSignature');
const signatureEnd = app.indexOf('function liveScheduleSignature');
const signature = signatureStart >= 0 && signatureEnd > signatureStart ? app.slice(signatureStart, signatureEnd) : '';
for (const field of ['courseName', 'teacher', 'timeText', 'location', 'credits']) {
  if (!signature.includes(field)) throw new Error(`schedule signature does not include ${field}`);
}
assertContains(html, 'min="10"', 'refresh interval minimum');
assertContains(html, 'max="7200"', 'refresh interval maximum');
assertContains(html, 'value="60"', 'refresh interval default');
assertContains(html, '<span>秒</span>', 'refresh interval unit');

console.log('Smart Agent refresh contract passed.');
