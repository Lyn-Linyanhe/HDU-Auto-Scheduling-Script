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

const courseFixture = process.env.HDU_COURSE_FIXTURE
  ? path.resolve(process.env.HDU_COURSE_FIXTURE)
  : path.join(root, 'testdata/course.sample.json');
const payload = JSON.parse(fs.readFileSync(courseFixture, 'utf8'));
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
if (!diagnosis.result.some((reason) => reason.action && /学分/.test(reason.action))) {
  throw new Error(`Worker diagnosis should include a suggested action: ${JSON.stringify(diagnosis)}`);
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

const oddOnly = { meetings: context.HDU.parseSchedule('\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468(\u5355)}') };
const evenOnly = { meetings: context.HDU.parseSchedule('\u661f\u671f\u4e00\u7b2c1-2\u8282{2-16\u5468(\u53cc)}') };
const allWeeks = { meetings: context.HDU.parseSchedule('\u661f\u671f\u4e00\u7b2c1-2\u8282{1-17\u5468}') };
if (context.HDU.courseConflict(oddOnly, evenOnly)
  || !context.HDU.courseConflict(oddOnly, allWeeks)
  || !context.HDU.courseConflict(evenOnly, allWeeks)) {
  throw new Error('Odd/even week conflict handling failed.');
}
if (!context.HDU.hasCreditData(courses) || context.HDU.hasCreditData([{ credits: 0 }])) {
  throw new Error('Credit data availability detection failed.');
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

const legacyFixture = context.HDU.normalizeCourseData([
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
const mainLegacyResolution = context.HDU.resolveLegacyCourseGroups(legacyFixture, '软件工程');
const practicalLegacyResolution = context.HDU.resolveLegacyCourseGroups(legacyFixture, '软件工程课程实践');
if (mainLegacyResolution.unresolved || mainLegacyResolution.groups.length !== 1 || mainLegacyResolution.groups[0].items.length !== 2) {
  throw new Error(`Legacy main-course matching failed: ${JSON.stringify(mainLegacyResolution)}`);
}
if (practicalLegacyResolution.unresolved || practicalLegacyResolution.groups.length !== 1 || practicalLegacyResolution.groups[0].items.length !== 1) {
  throw new Error(`Legacy practical-course matching failed: ${JSON.stringify(practicalLegacyResolution)}`);
}
const legacyGenerated = context.HDU.generateSolutions([...mainLegacyResolution.groups, ...practicalLegacyResolution.groups], state, 20);
if (legacyGenerated.results.length !== 2) {
  throw new Error(`Expected two legacy software/practical combinations, got ${legacyGenerated.results.length}.`);
}

const legacyMigration = context.HDU.migrateLegacyCourseLocks(
  legacyFixture,
  [
    '(2026-2027-1)-A9990001-01',
    '软件工程',
    '不存在的旧课程约束',
  ].join('\n'),
);
if (legacyMigration.matches.length !== 1 || legacyMigration.matches[0].id !== 'software-main-01') {
  throw new Error(`Legacy migration should lock one exact teaching class: ${JSON.stringify(legacyMigration)}`);
}
if (!legacyMigration.unresolved.includes('软件工程') || !legacyMigration.unresolved.includes('不存在的旧课程约束')) {
  throw new Error(`Legacy migration should expose ambiguous and missing tokens: ${JSON.stringify(legacyMigration)}`);
}

const semanticConstraints = {
  minCredit: 0,
  maxCredit: 45,
  maxEarly: 5,
  maxLunch: 5,
  maxLate: 5,
  minFreeDays: 0,
  blockedTeachers: '',
  pairRules: '',
  sameTeacherRules: '',
};
const neutralSolution = context.HDU.evaluateSolution([legacyFixture[0]], semanticConstraints);
const legacyFieldsSolution = context.HDU.evaluateSolution([legacyFixture[0]], {
  ...semanticConstraints,
  requiredCourses: '不存在的旧课程约束',
  preferredTeachers: '不存在的偏好教师',
});
if (!neutralSolution || !legacyFieldsSolution || neutralSolution.score !== legacyFieldsSolution.score) {
  throw new Error('Legacy state fields must not change active candidate validation or ranking.');
}

const linkedFixture = context.HDU.normalizeCourseData([
  {
    displayCode: '(2026-2027-1)-A9992001-01',
    courseCode: '(2026-2027-1)-A9992001',
    jxb_id: 'linked-software-main',
    jxbmc: '(2026-2027-1)-A9992001-01',
    kcmc: '软件工程',
    jzgxx: '甲老师',
    sksj: '星期一第1-2节{1-17周}',
    xf: '3.00',
  },
  {
    displayCode: '(2026-2027-1)-S9992001-01',
    courseCode: '(2026-2027-1)-S9992001',
    jxb_id: 'linked-software-practice',
    jxbmc: '(2026-2027-1)-S9992001-01',
    kcmc: '软件工程开发实践2（乙）',
    jzgxx: '甲老师',
    sksj: '星期二第1-2节{1-17周}',
    xf: '1.00',
  },
  {
    displayCode: '(2026-2027-1)-A9992002-01',
    courseCode: '(2026-2027-1)-A9992002',
    jxb_id: 'linked-security-main',
    jxbmc: '(2026-2027-1)-A9992002-01',
    kcmc: '计算机系统及安全2（乙）',
    jzgxx: '乙老师',
    sksj: '星期三第1-2节{1-17周}',
    xf: '3.00',
  },
  {
    displayCode: '(2026-2027-1)-S9992002-01',
    courseCode: '(2026-2027-1)-S9992002',
    jxb_id: 'linked-security-practice',
    jxbmc: '(2026-2027-1)-S9992002-01',
    kcmc: '计算机系统及安全课程实践2（乙）',
    jzgxx: '乙老师',
    sksj: '星期四第1-2节{1-17周}',
    xf: '1.00',
  },
  {
    displayCode: '(2026-2027-1)-A9992003-01',
    courseCode: '(2026-2027-1)-A9992003',
    jxb_id: 'linked-design-main',
    jxbmc: '(2026-2027-1)-A9992003-01',
    kcmc: '角色与场景',
    jzgxx: '丙老师',
    sksj: '星期五第1-2节{1-17周}',
    xf: '3.00',
  },
  {
    displayCode: '(2026-2027-1)-S9992003-01',
    courseCode: '(2026-2027-1)-S9992003',
    jxb_id: 'linked-design-course',
    jxbmc: '(2026-2027-1)-S9992003-01',
    kcmc: '角色与场景设计',
    jzgxx: '丁老师',
    sksj: '星期五第3-4节{1-17周}',
    xf: '1.00',
  },
]);
const pairRuleRejected = context.HDU.evaluateSolution([linkedFixture[0]], {
  ...semanticConstraints,
  pairRules: 'A9992001 -> S9992001',
});
const pairRuleAccepted = context.HDU.evaluateSolution([linkedFixture[0], linkedFixture[1]], {
  ...semanticConstraints,
  pairRules: 'A9992001 -> S9992001',
});
const teacherRuleRejected = context.HDU.evaluateSolution([linkedFixture[0], linkedFixture[4]], {
  ...semanticConstraints,
  sameTeacherRules: 'A9992001 = A9992003',
});
const teacherRuleAccepted = context.HDU.evaluateSolution([linkedFixture[0], linkedFixture[1]], {
  ...semanticConstraints,
  sameTeacherRules: 'A9992001 = S9992001',
});
if (pairRuleRejected || !pairRuleAccepted || teacherRuleRejected || !teacherRuleAccepted) {
  throw new Error('Scheme-level forced-together and teacher-consistency semantics regressed.');
}
const linkedPairs = context.HDU.findLinkedCoursePairs(linkedFixture);
const linkedNames = linkedPairs.map((pair) => pair.map((group) => group.name).sort().join('|'));
if (!linkedNames.some((name) => name.includes('软件工程') && name.includes('软件工程开发实践2（乙）'))) {
  throw new Error(`Development practice association was not detected: ${JSON.stringify(linkedNames)}`);
}
if (!linkedNames.some((name) => name.includes('计算机系统及安全2（乙）') && name.includes('计算机系统及安全课程实践2（乙）'))) {
  throw new Error(`Teaching-class suffix association was not detected: ${JSON.stringify(linkedNames)}`);
}
if (linkedNames.some((name) => name.includes('角色与场景') && name.includes('角色与场景设计'))) {
  throw new Error(`Generic design suffix should not create an automatic association: ${JSON.stringify(linkedNames)}`);
}
const developmentLegacyResolution = context.HDU.resolveLegacyCourseGroups(linkedFixture, '软件工程开发实践2（乙）');
if (developmentLegacyResolution.unresolved || developmentLegacyResolution.groups.length !== 1) {
  throw new Error(`Development practice legacy matching failed: ${JSON.stringify(developmentLegacyResolution)}`);
}

const conflictFixture = context.HDU.normalizeCourseData([
  {
    displayCode: '(2026-2027-1)-A9991001-01',
    courseCode: '(2026-2027-1)-A9991001',
    jxb_id: 'conflict-left',
    jxbmc: '(2026-2027-1)-A9991001-01',
    kcmc: '冲突课程甲',
    sksj: '星期一第1-2节{1-17周}',
    xf: '2.00',
  },
  {
    displayCode: '(2026-2027-1)-A9991002-01',
    courseCode: '(2026-2027-1)-A9991002',
    jxb_id: 'conflict-right',
    jxbmc: '(2026-2027-1)-A9991002-01',
    kcmc: '冲突课程乙',
    sksj: '星期一第1-2节{1-17周}',
    xf: '2.00',
  },
]);
const conflictGroups = context.HDU.groupCourses(conflictFixture).map((group) => ({
  id: group.id,
  name: group.name,
  items: group.items,
  lockedItemId: '',
  optional: false,
}));
const conflictDiagnosis = context.HDU.diagnoseNoSolutions(conflictGroups, state, {});
if (!conflictDiagnosis.some((reason) => reason.type === 'conflict' && /无法同时满足/.test(reason.text))) {
  throw new Error(`Conflict diagnosis should explain mutually exclusive mandatory groups: ${JSON.stringify(conflictDiagnosis)}`);
}

console.log(`Worker smoke test passed: ${estimate.result.count} estimated, ${generated.result.results.length} generated.`);
