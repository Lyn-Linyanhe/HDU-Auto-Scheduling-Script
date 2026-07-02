const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..');

function readText(file) {
  return fs.readFileSync(path.join(root, file), 'utf8');
}

const context = {
  console,
  postedMessages: [],
};
context.globalThis = context;
context.self = context;
context.postMessage = (message) => context.postedMessages.push(message);
context.importScripts = (...files) => {
  for (const file of files) {
    vm.runInContext(readText(file), context, { filename: file });
  }
};

vm.createContext(context);
vm.runInContext(readText('shared.js'), context, { filename: 'shared.js' });

const payload = JSON.parse(readText('testdata/course.sample.json'));
const courses = context.HDU.normalizeCourseData(payload);
if (!courses.length) throw new Error('Sample course data did not normalize.');
if (courses.some((course) => course.schemaVersion !== context.HDU.COURSE_SCHEMA_VERSION)) {
  throw new Error('Normalized courses are missing schemaVersion.');
}

const groups = context.HDU.groupCourses(courses).slice(0, 3).map((group) => ({
  id: group.id,
  name: group.name,
  items: group.items,
  lockedItemId: '',
  optional: false,
}));
const state = {
  minCredit: 0,
  maxCredit: 45,
  maxEarly: 5,
  maxLunch: 5,
  maxLate: 5,
  minFreeDays: 0,
  blockedTeachers: '',
  preferredTeachers: '',
  requiredCourses: '',
  pairRules: '',
  sameTeacherRules: '',
};

vm.runInContext(readText('scheduler-worker.js'), context, { filename: 'scheduler-worker.js' });

context.self.onmessage({ data: { id: 'estimate', type: 'estimate', groups, state, limit: 1000 } });
context.self.onmessage({ data: { id: 'generate', type: 'generate', groups, state, limit: 20 } });

const estimate = context.postedMessages.find((message) => message.id === 'estimate');
const generated = context.postedMessages.find((message) => message.id === 'generate');

if (!estimate?.ok || estimate.result.count < 1) {
  throw new Error(`Worker estimate failed: ${JSON.stringify(estimate)}`);
}
if (!generated?.ok || !generated.result.results.length) {
  throw new Error(`Worker generation failed: ${JSON.stringify(generated)}`);
}
if (generated.result.results.some((solution) => !solution.signature)) {
  throw new Error('Worker generated a solution without a signature.');
}

console.log(`Worker smoke test passed: ${estimate.result.count} estimated, ${generated.result.results.length} generated.`);
