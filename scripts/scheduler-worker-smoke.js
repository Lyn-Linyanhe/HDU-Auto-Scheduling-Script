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
context.self.onmessage({
  data: {
    id: 'diagnose-credit',
    type: 'diagnose',
    groups,
    state: { ...state, minCredit: 99, maxCredit: 100 },
    context: {},
  },
});

const estimate = context.postedMessages.find((message) => message.id === 'estimate');
const generated = context.postedMessages.find((message) => message.id === 'generate');
const diagnosis = context.postedMessages.find((message) => message.id === 'diagnose-credit');

if (!estimate?.ok || estimate.result.count < 1) {
  throw new Error(`Worker estimate failed: ${JSON.stringify(estimate)}`);
}
if (!generated?.ok || !generated.result.results.length) {
  throw new Error(`Worker generation failed: ${JSON.stringify(generated)}`);
}
if (generated.result.results.some((solution) => !solution.signature)) {
  throw new Error('Worker generated a solution without a signature.');
}
if (!diagnosis?.ok || !diagnosis.result.some((reason) => /学分/.test(reason.text))) {
  throw new Error(`Worker diagnosis should explain impossible credits: ${JSON.stringify(diagnosis)}`);
}

const optionalCourse = courses[0];
const optionalOnlyGroups = [{
  id: optionalCourse.groupId,
  name: optionalCourse.courseName,
  items: [optionalCourse],
  lockedItemId: '',
  optional: true,
}];
const optionalGenerated = context.HDU.generateSolutions(optionalOnlyGroups, state, 10);
if (!optionalGenerated.results.some((solution) => solution.items.length === 0)) {
  throw new Error('Unlocked selected course should be removable in generated schedules.');
}
if (!optionalGenerated.results.some((solution) => solution.items.some((item) => item.id === optionalCourse.id))) {
  throw new Error('Unlocked selected course should still be allowed as an optional choice.');
}

const lockedGenerated = context.HDU.generateSolutions([{
  ...optionalOnlyGroups[0],
  lockedItemId: optionalCourse.id,
  optional: false,
}], state, 10);
if (!lockedGenerated.results.length || lockedGenerated.results.some((solution) => !solution.items.some((item) => item.id === optionalCourse.id))) {
  throw new Error('Locked selected course must appear in every generated schedule.');
}

const hugeGroups = Array.from({ length: 80 }, (_, index) => ({
  id: `huge-${index}`,
  name: `huge-${index}`,
  items: [{ ...optionalCourse, id: `huge-course-${index}`, groupId: `huge-${index}` }],
  lockedItemId: '',
  optional: true,
}));
const startedAt = Date.now();
const hugeEstimate = context.HDU.estimateSolutions(hugeGroups, state, 50);
const elapsed = Date.now() - startedAt;
if (!hugeEstimate.capped || elapsed > 500) {
  throw new Error(`Huge estimate should cap quickly, got ${JSON.stringify(hugeEstimate)} in ${elapsed}ms.`);
}

const requiredFixture = context.HDU.normalizeCourseData([
  {
    displayCode: '(2026-2027-1)-A9990001-01',
    courseCode: '(2026-2027-1)-A9990001',
    jxb_id: 'software-main-01',
    jxbmc: '(2026-2027-1)-A9990001-01',
    kcmc: '软件工程',
    jzgxx: '甲老师',
    sksj: '星期一第1-2节{1-17周}',
    xf: '3.00',
  },
  {
    displayCode: '(2026-2027-1)-A9990001-02',
    courseCode: '(2026-2027-1)-A9990001',
    jxb_id: 'software-main-02',
    jxbmc: '(2026-2027-1)-A9990001-02',
    kcmc: '软件工程',
    jzgxx: '乙老师',
    sksj: '星期二第1-2节{1-17周}',
    xf: '3.00',
  },
  {
    displayCode: '(2026-2027-1)-S9990001-01',
    courseCode: '(2026-2027-1)-S9990001',
    jxb_id: 'software-design-01',
    jxbmc: '(2026-2027-1)-S9990001-01',
    kcmc: '软件工程课程设计',
    jzgxx: '甲老师',
    sksj: '星期三第1-2节{1-17周}',
    xf: '1.00',
  },
]);
const mainRequired = context.HDU.resolveRequiredCourseGroups(requiredFixture, '软件工程');
const practicalRequired = context.HDU.resolveRequiredCourseGroups(requiredFixture, '软件工程课程实践');
if (mainRequired.unresolved || mainRequired.groups.length !== 1 || mainRequired.groups[0].items.length !== 2) {
  throw new Error(`Required main-course matching failed: ${JSON.stringify(mainRequired)}`);
}
if (practicalRequired.unresolved || practicalRequired.groups.length !== 1 || practicalRequired.groups[0].items.length !== 1) {
  throw new Error(`Required practical-course matching failed: ${JSON.stringify(practicalRequired)}`);
}
const requiredGenerated = context.HDU.generateSolutions([...mainRequired.groups, ...practicalRequired.groups], state, 20);
if (requiredGenerated.results.length !== 2) {
  throw new Error(`Expected two software/practical combinations, got ${requiredGenerated.results.length}.`);
}

console.log(`Worker smoke test passed: ${estimate.result.count} estimated, ${generated.result.results.length} generated.`);
