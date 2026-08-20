const path = require('path');

const root = path.resolve(__dirname, '..');
const mod = require(path.join(root, 'HDU-Smart-Course-Agent', 'web', 'backoff.js'));
const wait = mod.liveRefreshWaitingSeconds;

const cases = [
  [60, 0, 7200, 60],
  [60, 1, 7200, 120],
  [60, 2, 7200, 240],
  [10, 7, 7200, 1280],
  [60, 10, 7200, 7200], // capped
  [60, -1, 7200, 60],   // negative streak clamped
  [0, 0, 7200, 60],     // default base
  [7200, 1, 7200, 7200], // base at cap stays at cap
];

for (const [base, streak, cap, want] of cases) {
  const got = wait(base, streak, cap);
  if (got !== want) {
    throw new Error(`backoff(${base}, ${streak}, ${cap}) = ${got}, want ${want}`);
  }
}

console.log(JSON.stringify({ ok: true, check: 'send_refresh_backoff', cases: cases.length }, null, 2));
