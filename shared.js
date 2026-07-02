(() => {
  const STORAGE_KEY = 'hdu-scheduler-state-v3';
  const COURSE_SCHEMA_VERSION = 1;
  const COURSE_API = '/api/course';
  const STATUS_API = '/api/status';
  const PERSONAL_SCHEDULE_API = '/api/personal-schedule';
  const DAY_LABELS = ['周一', '周二', '周三', '周四', '周五'];
  const PERIOD_TIMES = [
    '08:05-08:50',
    '08:55-09:40',
    '10:00-10:45',
    '10:50-11:35',
    '11:40-12:25',
    '13:30-14:15',
    '14:20-15:05',
    '15:15-16:00',
    '16:05-16:50',
    '18:30-19:15',
    '19:20-20:05',
    '20:10-20:55',
    '21:00-21:45',
  ];
  const DEFAULT_STATE = {
    query: '',
    minCredit: 0,
    maxCredit: 36,
    maxEarly: 5,
    maxLunch: 5,
    maxLate: 5,
    minFreeDays: 0,
    blockedTeachers: '',
    preferredTeachers: '',
    requiredCourses: '',
    pairRules: '',
    sameTeacherRules: '',
    selectedGroups: {},
    activeCandidate: '',
    candidateCursor: 0,
    candidatePreviewEnabled: false,
    favoriteCandidates: [],
    dismissedCandidates: [],
    baseCourseIds: [],
    baseScheduleName: '',
    candidateEstimate: '',
    resultListMode: 'current',
  };

  function cloneDefault() {
    return JSON.parse(JSON.stringify(DEFAULT_STATE));
  }

  function loadState() {
    try {
      const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
      if (!saved.timeLimitDefaultsV2 && saved.maxEarly === 1 && saved.maxLunch === 1 && saved.maxLate === 1) {
        saved.maxEarly = 5;
        saved.maxLunch = 5;
        saved.maxLate = 5;
      }
      if (!saved.creditDefaultsV4 && (saved.maxCredit === undefined || saved.maxCredit === 30)) {
        saved.maxCredit = 36;
      }
      if (saved.minFreeDays === undefined) saved.minFreeDays = 0;
      saved.timeLimitDefaultsV2 = true;
      saved.creditDefaultsV4 = true;
      return {
        ...cloneDefault(),
        ...saved,
        selectedGroups: saved.selectedGroups || {},
        favoriteCandidates: saved.favoriteCandidates || [],
        dismissedCandidates: saved.dismissedCandidates || [],
        baseCourseIds: saved.baseCourseIds || [],
      };
    } catch {
      return cloneDefault();
    }
  }

  function saveState(state) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  }

  async function fetchJSON(url, options = {}) {
    const response = await fetch(url, { cache: 'no-store', ...options });
    if (!response.ok) throw new Error(await response.text());
    return response.json();
  }

  function firstText(...values) {
    for (const value of values) {
      if (Array.isArray(value)) {
        const text = value.map((item) => firstText(item)).filter(Boolean).join(' ');
        if (text) return text;
      } else if (value !== undefined && value !== null) {
        const text = String(value).trim();
        if (text) return text;
      }
    }
    return '';
  }

  function parseCredits(value) {
    const number = Number.parseFloat(firstText(value).replace(/[^\d.]/g, ''));
    return Number.isFinite(number) ? number : 0;
  }

  function looksLikeInternalId(value) {
    const text = String(value || '').trim();
    return text.length >= 20 && /^[0-9a-f]+$/i.test(text);
  }

  function extractSectionCode(...values) {
    for (const value of values) {
      const text = firstText(value);
      const match = text.match(/\(\d{4}-\d{4}-\d\)-[A-Za-z0-9]+-\d{1,3}/);
      if (match) return match[0];
    }
    return '';
  }

  function parseDay(text) {
    const source = String(text || '');
    const map = [
      ['星期一', 1], ['周一', 1], ['礼拜一', 1],
      ['星期二', 2], ['周二', 2], ['礼拜二', 2],
      ['星期三', 3], ['周三', 3], ['礼拜三', 3],
      ['星期四', 4], ['周四', 4], ['礼拜四', 4],
      ['星期五', 5], ['周五', 5], ['礼拜五', 5],
      ['星期六', 6], ['周六', 6], ['礼拜六', 6],
      ['星期日', 7], ['星期天', 7], ['周日', 7], ['周天', 7],
    ];
    for (const [label, day] of map) {
      if (source.includes(label)) return day;
    }
    return 0;
  }

  function fullWeeks() {
    return new Set(Array.from({ length: 20 }, (_, index) => index + 1));
  }

  function parseWeeks(text) {
    const weeks = new Set();
    const source = String(text || '').replace(/\s+/g, '');
    const rangeRe = /(\d{1,2})(?:[-~—至到](\d{1,2}))?周/g;
    let match;
    while ((match = rangeRe.exec(source))) {
      const start = Number(match[1]);
      const end = Number(match[2] || match[1]);
      for (let week = start; week <= end; week += 1) weeks.add(week);
    }
    const result = weeks.size ? weeks : fullWeeks();
    if (source.includes('单周')) {
      for (const week of [...result]) if (week % 2 === 0) result.delete(week);
    }
    if (source.includes('双周')) {
      for (const week of [...result]) if (week % 2 !== 0) result.delete(week);
    }
    return result.size ? result : fullWeeks();
  }

  function parsePeriods(text) {
    const source = String(text || '');
    const periods = [];
    const blockMatch = source.match(/第?\s*([0-9,\-~—至到、，\s]+)\s*节/);
    if (blockMatch) {
      const parts = blockMatch[1].split(/[,，、]/).map((item) => item.trim()).filter(Boolean);
      for (const part of parts) {
        const range = part.match(/^(\d{1,2})(?:\s*[-~—至到]\s*(\d{1,2}))?$/);
        if (range) periods.push({ start: Number(range[1]), end: Number(range[2] || range[1]) });
      }
      if (periods.length) return periods;
    }

    const fallback = /(\d{1,2})(?:\s*[-~—至到]\s*(\d{1,2}))?\s*节/g;
    let match;
    while ((match = fallback.exec(source))) {
      periods.push({ start: Number(match[1]), end: Number(match[2] || match[1]) });
    }
    return periods;
  }

  function parseSchedule(text) {
    const source = String(text || '').trim();
    if (!source) return [];
    const meetings = [];
    const chunks = source
      .replace(/；/g, ';')
      .split(/[;\n]/)
      .map((item) => item.trim())
      .filter(Boolean);

    for (const chunk of chunks) {
      const day = parseDay(chunk);
      if (!day) continue;
      const weeks = parseWeeks(chunk);
      const periods = parsePeriods(chunk);
      for (const period of periods) {
        if (period.start > 0 && period.end >= period.start) {
          meetings.push({ day, start: period.start, end: period.end, weeks, raw: chunk });
        }
      }
    }
    return meetings;
  }

  function normalizeCourseData(raw) {
    const list = Array.isArray(raw?.items) ? raw.items : Array.isArray(raw) ? raw : [];
    return list.map((item, index) => {
      const courseName = firstText(item.kcmc, item.courseName, item.name, item.Kcmc, '未命名课程');
      const sectionCode = extractSectionCode(item.displayCode, item.jxbmc, item.sectionName, item.Jxbmc);
      const sectionName = firstText(sectionCode, item.jxbmc, item.sectionName, item.Jxbmc, `${courseName}-${index + 1}`);
      const rawCourseCode = firstText(item.courseCode, item.kch, item.kch_id, item.course_id, item.courseId);
      const displayCode = firstText(sectionCode, item.displayCode, item.jxbmc, item.sectionName);
      const groupId = firstText(
        item.groupId,
        item.courseGroupId,
        sectionCode ? baseCourseCode(sectionCode) : '',
        looksLikeInternalId(rawCourseCode) ? '' : rawCourseCode,
        item.kcmc,
        courseName,
      );
      const id = firstText(item.jxb_id, item.sectionId, item.id, sectionCode, `${groupId}-${index + 1}`);
      const timeText = firstText(item.sksj, item.time, item.schedule);
      const location = firstText(item.jxdd, item.location, item.cdlbmc, item.cdejlbmc);
      return {
        schemaVersion: COURSE_SCHEMA_VERSION,
        id,
        groupId,
        displayCode,
        rawCourseCode,
        courseName,
        sectionName,
        teacher: firstText(item.jzgxx, item.jsxm, item.teacher, item.js),
        kind: firstText(item.kklxmc, item.kklx, item.courseType),
        location,
        status: firstText(item.kkztmc, item.xkbjmc),
        credits: parseCredits(firstText(item.xf, item.credits, item.credit)),
        capacity: Number(item.jxbrl || 0),
        enrolled: Number(item.jxbrs || item.xkrs || 0),
        selected: Number(item.xkrs || 0),
        timeText,
        meetings: parseSchedule(timeText),
        raw: item,
      };
    });
  }

  function groupCourses(courses) {
    const map = new Map();
    for (const course of courses) {
      const key = course.groupId || course.courseName;
      if (!map.has(key)) {
        map.set(key, { id: key, name: course.courseName, kind: course.kind, credits: course.credits, items: [] });
      }
      map.get(key).items.push(course);
    }
    return Array.from(map.values()).map((group) => {
      group.items.sort((a, b) => a.sectionName.localeCompare(b.sectionName, 'zh-Hans-CN'));
      return group;
    });
  }

  function filterCourses(courses, query) {
    const keyword = String(query || '').trim().toLowerCase();
    if (!keyword) return courses;
    return courses.filter((course) => [
      course.courseName,
      course.sectionName,
      course.teacher,
      course.kind,
      course.location,
      course.status,
      course.timeText,
      course.groupId,
      course.displayCode,
      course.rawCourseCode,
    ].join(' ').toLowerCase().includes(keyword));
  }

  function parseList(text) {
    return String(text || '').split(/[\n,，;；\s]+/).map((item) => item.trim()).filter(Boolean);
  }

  function baseCourseCode(value) {
    return String(value || '').trim().replace(/-\d{1,3}$/, '');
  }

  function courseSearchText(item) {
    return [
      item.courseName,
      item.sectionName,
      item.displayCode,
      item.groupId,
      item.rawCourseCode,
      item.id,
      baseCourseCode(item.sectionName),
      baseCourseCode(item.displayCode),
      baseCourseCode(item.groupId),
      baseCourseCode(item.id),
    ].filter(Boolean).join(' ').toLowerCase();
  }

  function normalizeCourseTitle(value) {
    return String(value || '')
      .trim()
      .replace(/[（(]?[一二三四五六七八九十0-9]+[)）]?$/g, '')
      .trim()
      .toLowerCase();
  }

  function practicalBaseName(value) {
    return normalizeCourseTitle(value)
      .replace(/(课程实践|开发实践|综合实践|课程设计|实验|实践|实训|设计)$/g, '')
      .trim();
  }

  function hasPracticalSuffix(value) {
    return /(课程实践|开发实践|综合实践|课程设计|实验|实践|实训|设计)$/i.test(normalizeCourseTitle(value));
  }

  function syntheticRequiredGroup(token, groups, strategy) {
    const items = groups.flatMap((group) => group.items || []);
    return {
      id: `required:${strategy}:${token}`,
      name: token,
      items,
      lockedItemId: '',
      optional: false,
      sourceGroups: groups.map((group) => group.id),
      requiredStrategy: strategy,
    };
  }

  function resolveRequiredCourseGroups(courses, token) {
    const rawToken = String(token || '').trim();
    const lower = rawToken.toLowerCase();
    const normalized = normalizeCourseTitle(rawToken);
    if (!rawToken) return { token: rawToken, groups: [], unresolved: true, strategy: 'empty' };

    const groups = groupCourses(courses);
    const codeMatches = groups.filter((group) => {
      const ids = [
        group.id,
        baseCourseCode(group.id),
        ...(group.items || []).flatMap((item) => [
          item.groupId,
          item.displayCode,
          item.sectionName,
          item.rawCourseCode,
          item.id,
          baseCourseCode(item.groupId),
          baseCourseCode(item.displayCode),
          baseCourseCode(item.sectionName),
          baseCourseCode(item.id),
        ]),
      ].filter(Boolean).map((item) => String(item).toLowerCase());
      return ids.includes(lower);
    });
    if (codeMatches.length) return { token: rawToken, groups: codeMatches, unresolved: false, strategy: 'code' };

    const exactNameMatches = groups.filter((group) => normalizeCourseTitle(group.name) === normalized);
    if (exactNameMatches.length) {
      return {
        token: rawToken,
        groups: [syntheticRequiredGroup(rawToken, exactNameMatches, 'name')],
        unresolved: false,
        strategy: 'name',
      };
    }

    if (hasPracticalSuffix(rawToken)) {
      const base = practicalBaseName(rawToken);
      const practicalMatches = groups.filter((group) => {
        const name = normalizeCourseTitle(group.name);
        return hasPracticalSuffix(name) && practicalBaseName(name) === base;
      });
      if (practicalMatches.length) {
        return {
          token: rawToken,
          groups: [syntheticRequiredGroup(rawToken, practicalMatches, 'practical')],
          unresolved: false,
          strategy: 'practical',
        };
      }
    }

    return { token: rawToken, groups: [], unresolved: true, strategy: 'none' };
  }

  function parsePairRules(text) {
    return String(text || '')
      .split(/[\n;；]+/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const parts = line.split(/->\/~|\/~|=>|->|=|~|≈/).map((item) => item.trim()).filter(Boolean);
        return parts.length >= 2 ? { from: parts[0], to: parts[1], mode: 'hard' } : null;
      })
      .filter(Boolean);
  }

  function sameWeeks(a, b) {
    for (const week of a) if (b.has(week)) return true;
    return false;
  }

  function meetingConflict(a, b) {
    if (a.day !== b.day) return false;
    if (a.end < b.start || b.end < a.start) return false;
    return sameWeeks(a.weeks, b.weeks);
  }

  function courseConflict(a, b) {
    return a.meetings.some((first) => b.meetings.some((second) => meetingConflict(first, second)));
  }

  function sameBaseCourse(a, b) {
    const left = baseCourseCode(a.displayCode || a.sectionName || a.groupId || a.id);
    const right = baseCourseCode(b.displayCode || b.sectionName || b.groupId || b.id);
    return Boolean(left && right && left === right);
  }

  function countMetrics(items) {
    const daySlots = new Map([[1, new Set()], [2, new Set()], [3, new Set()], [4, new Set()], [5, new Set()]]);
    const teachers = new Set();
    for (const item of items) {
      if (item.teacher) teachers.add(item.teacher);
      for (const meeting of item.meetings) {
        const slots = daySlots.get(meeting.day) || new Set();
        for (let slot = meeting.start; slot <= meeting.end; slot += 1) slots.add(slot);
        daySlots.set(meeting.day, slots);
      }
    }

    let earlyDays = 0;
    let lunchDays = 0;
    let lateDays = 0;
    let freeDays = 0;
    for (let day = 1; day <= 5; day += 1) {
      const slots = daySlots.get(day) || new Set();
      if (!slots.size) {
        freeDays += 1;
        continue;
      }
      if (slots.has(1) || slots.has(2)) earlyDays += 1;
      if (slots.has(5) && slots.has(6)) lunchDays += 1;
      if ([10, 11, 12, 13].some((slot) => slots.has(slot))) lateDays += 1;
    }
    return { earlyDays, lunchDays, lateDays, freeDays, teacherCount: teachers.size };
  }

  function constraintsFromState(state) {
    return {
      minCredit: Number(state.minCredit || 0),
      maxCredit: Number(state.maxCredit || 36),
      maxEarly: Number(state.maxEarly ?? 5),
      maxLunch: Number(state.maxLunch ?? 5),
      maxLate: Number(state.maxLate ?? 5),
      minFreeDays: Number(state.minFreeDays ?? 0),
      blockedTeachers: state.blockedTeachers || '',
      preferredTeachers: state.preferredTeachers || '',
      requiredCourses: state.requiredCourses || '',
      pairRules: state.pairRules || '',
      sameTeacherRules: state.sameTeacherRules || '',
    };
  }

  function evaluateSolution(items, constraints) {
    const credits = items.reduce((sum, item) => sum + item.credits, 0);
    if (credits < constraints.minCredit || credits > constraints.maxCredit) return null;

    const blocked = new Set(parseList(constraints.blockedTeachers).map((item) => item.toLowerCase()));
    const preferred = new Set(parseList(constraints.preferredTeachers).map((item) => item.toLowerCase()));
    const required = parseList(constraints.requiredCourses).map((item) => item.toLowerCase());
    for (let i = 0; i < items.length; i += 1) {
      for (let j = i + 1; j < items.length; j += 1) {
        if (sameBaseCourse(items[i], items[j])) return null;
      }
    }
    for (const item of items) {
      const teacher = (item.teacher || '').toLowerCase();
      for (const bad of blocked) if (bad && teacher.includes(bad)) return null;
    }
    for (const need of required) {
      if (need && !items.some((item) => courseSearchText(item).includes(need))) return null;
    }

    const metrics = countMetrics(items);
    if (metrics.earlyDays > constraints.maxEarly || metrics.lunchDays > constraints.maxLunch || metrics.lateDays > constraints.maxLate) return null;

    if (metrics.freeDays < constraints.minFreeDays) return null;

    let score = metrics.earlyDays * 20
      + metrics.lunchDays * 16
      + metrics.lateDays * 18
      - credits * 0.25;

    for (const rule of parsePairRules(constraints.pairRules)) {
      const fromNeed = rule.from.toLowerCase();
      const toNeed = rule.to.toLowerCase();
      const fromHit = items.some((item) => courseSearchText(item).includes(fromNeed));
      const toHit = items.some((item) => courseSearchText(item).includes(toNeed));
      if (fromHit !== toHit) return null;
    }

    for (const rule of parsePairRules(constraints.sameTeacherRules)) {
      const fromNeed = rule.from.toLowerCase();
      const toNeed = rule.to.toLowerCase();
      const left = items.filter((item) => courseSearchText(item).includes(fromNeed));
      const right = items.filter((item) => courseSearchText(item).includes(toNeed));
      if (!left.length || !right.length) continue;
      const leftTeachers = new Set(left.flatMap((item) => splitTeachers(item.teacher)));
      const matched = right.some((item) => splitTeachers(item.teacher).some((teacher) => leftTeachers.has(teacher)));
      if (!matched) return null;
    }

    for (const item of items) {
      const teacher = (item.teacher || '').toLowerCase();
      for (const good of preferred) if (good && teacher.includes(good)) score -= 3;
    }
    return { items: [...items], credits, metrics, score };
  }

  function lockedConstraintConflict(picked, option) {
    return picked.some((chosen) => courseConflict(chosen, option) || sameBaseCourse(chosen, option));
  }

  function splitTeachers(value) {
    return String(value || '')
      .split(/[;；,，、/\s]+/)
      .map((item) => item.replace(/^\d+\//, '').trim().toLowerCase())
      .filter(Boolean);
  }

  function orderedGroups(groups) {
    return [...groups].sort((a, b) => {
      const aLen = (a.lockedItemId ? 1 : a.items.length + (a.optional ? 1 : 0)) || 1;
      const bLen = (b.lockedItemId ? 1 : b.items.length + (b.optional ? 1 : 0)) || 1;
      return aLen - bLen;
    });
  }

  function optionsForGroup(group) {
    if (group.lockedItemId) {
      const locked = group.items.filter((item) => item.id === group.lockedItemId);
      return locked.length ? locked : group.items;
    }
    return group.optional ? [null, ...group.items] : group.items;
  }

  function optionCredits(option) {
    return option ? Number(option.credits || 0) : 0;
  }

  function boundedOptionProduct(ordered, cap) {
    let count = 1;
    for (const group of ordered) {
      const optionCount = Math.max(0, optionsForGroup(group).length);
      if (!optionCount) return { count: 0, capped: false };
      if (count > cap / optionCount) return { count: cap, capped: true };
      count *= optionCount;
    }
    return { count, capped: false };
  }

  function prepareSearch(ordered) {
    const optionsList = ordered.map((group) => optionsForGroup(group));
    const suffixMaxCredits = Array(optionsList.length + 1).fill(0);
    for (let index = optionsList.length - 1; index >= 0; index -= 1) {
      const maxCredit = optionsList[index].reduce((max, option) => Math.max(max, optionCredits(option)), 0);
      suffixMaxCredits[index] = suffixMaxCredits[index + 1] + maxCredit;
    }
    return { optionsList, suffixMaxCredits };
  }

  function generateSolutions(groups, state, limit = 500) {
    const constraints = constraintsFromState(state);
    const ordered = orderedGroups(groups);
    const { optionsList, suffixMaxCredits } = prepareSearch(ordered);
    const results = [];
    const picked = [];
    let capped = false;
    let visits = 0;
    const visitLimit = Math.max(100000, limit * 1200);

    const walk = (index, credits) => {
      if (results.length >= limit) {
        capped = true;
        return;
      }
      visits += 1;
      if (visits >= visitLimit) {
        capped = true;
        return;
      }
      if (credits > constraints.maxCredit) return;
      if (credits + suffixMaxCredits[index] < constraints.minCredit) return;
      if (index >= ordered.length) {
        const result = evaluateSolution(picked, constraints);
        if (result) results.push(result);
        return;
      }
      for (const option of optionsList[index]) {
        if (option && lockedConstraintConflict(picked, option)) continue;
        const nextCredits = credits + optionCredits(option);
        if (nextCredits > constraints.maxCredit) continue;
        if (nextCredits + suffixMaxCredits[index + 1] < constraints.minCredit) continue;
        if (option) picked.push(option);
        walk(index + 1, nextCredits);
        if (option) picked.pop();
        if (capped) return;
      }
    };

    walk(0, 0);
    results.sort((a, b) => b.metrics.freeDays - a.metrics.freeDays || a.score - b.score || b.credits - a.credits);
    return { results, capped, limit };
  }

  function estimateSolutions(groups, state, limit = 20000) {
    const constraints = constraintsFromState(state);
    const ordered = orderedGroups(groups);
    const exactProductLimit = Math.max(limit + 1, 100000);
    const upperBound = boundedOptionProduct(ordered, exactProductLimit);
    if (upperBound.capped) {
      return { count: limit, capped: true, limit, approximate: true };
    }
    const { optionsList, suffixMaxCredits } = prepareSearch(ordered);
    const picked = [];
    let count = 0;
    let capped = false;
    let visits = 0;
    const visitLimit = exactProductLimit;

    const walk = (index, credits) => {
      if (count >= limit) {
        capped = true;
        return;
      }
      visits += 1;
      if (visits >= visitLimit) {
        capped = true;
        return;
      }
      if (credits > constraints.maxCredit) return;
      if (credits + suffixMaxCredits[index] < constraints.minCredit) return;
      if (index >= ordered.length) {
        if (evaluateSolution(picked, constraints)) count += 1;
        return;
      }
      for (const option of optionsList[index]) {
        if (option && lockedConstraintConflict(picked, option)) continue;
        const nextCredits = credits + optionCredits(option);
        if (nextCredits > constraints.maxCredit) continue;
        if (nextCredits + suffixMaxCredits[index + 1] < constraints.minCredit) continue;
        if (option) picked.push(option);
        walk(index + 1, nextCredits);
        if (option) picked.pop();
        if (capped) return;
      }
    };

    walk(0, 0);
    return { count, capped, limit, approximate: capped && count < limit };
  }

  function groupFromSelection(selection, courses) {
    return {
      id: selection.groupId,
      name: selection.groupName,
      items: selection.items.map((item) => courses.find((course) => course.id === item.id)).filter(Boolean),
      lockedItemId: selection.lockedItemId || '',
      optional: Boolean(selection.optional),
    };
  }

  function courseLabel(course) {
    return [course.courseName, course.sectionName].filter(Boolean).join(' / ');
  }

  globalThis.HDU = {
    COURSE_SCHEMA_VERSION,
    DAY_LABELS,
    PERIOD_TIMES,
    COURSE_API,
    STATUS_API,
    PERSONAL_SCHEDULE_API,
    loadState,
    saveState,
    fetchJSON,
    firstText,
    normalizeCourseData,
    parseSchedule,
    parseList,
    parsePairRules,
    baseCourseCode,
    courseSearchText,
    resolveRequiredCourseGroups,
    groupCourses,
    filterCourses,
    countMetrics,
    generateSolutions,
    estimateSolutions,
    groupFromSelection,
    courseConflict,
    sameBaseCourse,
    lockedConstraintConflict,
    splitTeachers,
    courseLabel,
  };
})();
