(() => {
  const state = HDU.loadState();
  let courses = [];
  let selectedGroups = [];
  let solutions = [];
  let activeSolution = null;
  let selectedCourseId = '';
  let modalCourseId = '';
  let workerJobId = 0;
  let workerUnavailable = false;
  let schedulingBusy = false;
  let creditDataAvailable = true;

  const els = {
    subtitle: document.getElementById('subtitle'),
    candidateTitle: document.getElementById('candidate-title'),
    courseCount: document.getElementById('course-count'),
    searchInput: document.getElementById('search-input'),
    courseList: document.getElementById('course-list'),
    selectedList: document.getElementById('selected-list'),
    resultList: document.getElementById('result-list'),
    resultCount: document.getElementById('result-count'),
    tableResultCount: document.getElementById('table-result-count'),
    candidatePage: document.getElementById('candidate-page'),
    tableCandidatePage: document.getElementById('table-candidate-page'),
    estimateText: document.getElementById('estimate-text'),
    timetable: document.getElementById('timetable'),
    summaryChips: document.getElementById('summary-chips'),
    clearSelected: document.getElementById('clear-selected'),
    generate: document.getElementById('generate'),
    estimate: document.getElementById('estimate'),
    reloadCourse: document.getElementById('reload-course'),
    minCredit: document.getElementById('min-credit'),
    maxCredit: document.getElementById('max-credit'),
    creditWarning: document.getElementById('credit-data-warning'),
    maxEarly: document.getElementById('max-early'),
    maxLunch: document.getElementById('max-lunch'),
    maxLate: document.getElementById('max-late'),
    minFreeDays: document.getElementById('min-free-days'),
    blockedTeachers: document.getElementById('blocked-teachers'),
    lockedSelectedPicks: document.getElementById('locked-selected-picks'),
    lockedQuickPicks: document.getElementById('locked-quick-picks'),
    lockedSearch: document.getElementById('locked-search'),
    lockedSearchResults: document.getElementById('locked-search-results'),
    legacyWarning: document.getElementById('legacy-course-lock-warning'),
    pairRules: document.getElementById('pair-rules'),
    sameTeacherRules: document.getElementById('same-teacher-rules'),
    schemeCurrentRules: document.getElementById('scheme-current-rules'),
    linkedCourseSuggestions: document.getElementById('linked-course-suggestions'),
    schemeLeftCourse: document.getElementById('scheme-left-course'),
    schemeRightCourse: document.getElementById('scheme-right-course'),
    schemeMode: document.getElementById('scheme-mode'),
    schemeAddRule: document.getElementById('scheme-add-rule'),
    baseFile: document.getElementById('base-file'),
    baseSummary: document.getElementById('base-summary'),
    clearBase: document.getElementById('clear-base'),
    exportCurrent: document.getElementById('export-current'),
    candidatePrev: document.getElementById('candidate-prev'),
    candidateNext: document.getElementById('candidate-next'),
    candidatePreview: document.getElementById('candidate-preview'),
    candidateReturn: document.getElementById('candidate-return'),
    tableCandidatePrev: document.getElementById('table-candidate-prev'),
    tableCandidateNext: document.getElementById('table-candidate-next'),
    tableCandidatePreview: document.getElementById('table-candidate-preview'),
    tableCandidateReturn: document.getElementById('table-candidate-return'),
    candidateFavorite: document.getElementById('candidate-favorite'),
    candidateDismiss: document.getElementById('candidate-dismiss'),
    candidateFavoritesView: document.getElementById('candidate-favorites-view'),
    modal: document.getElementById('course-modal'),
    modalTitle: document.getElementById('course-modal-title'),
    modalStatus: document.getElementById('course-modal-status'),
    modalBody: document.getElementById('course-modal-body'),
    modalSelectToggle: document.getElementById('modal-select-toggle'),
    modalLockToggle: document.getElementById('modal-lock-toggle'),
  };

  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function formatCredit(value) {
    if (!creditDataAvailable) return '未知';
    const number = Number(value);
    return Number.isFinite(number) ? number.toFixed(2) : '0.00';
  }

  function syncCreditAvailability() {
    const unavailable = !creditDataAvailable;
    els.minCredit.disabled = unavailable;
    els.maxCredit.disabled = unavailable;
    els.creditWarning.hidden = !unavailable;
  }

  function syncForm() {
    els.searchInput.value = state.query || '';
    els.minCredit.value = state.minCredit ?? 0;
    els.maxCredit.value = state.maxCredit ?? 36;
    els.maxEarly.value = state.maxEarly ?? 5;
    els.maxLunch.value = state.maxLunch ?? 5;
    els.maxLate.value = state.maxLate ?? 5;
    els.minFreeDays.value = state.minFreeDays ?? 0;
    els.blockedTeachers.value = state.blockedTeachers || '';
    els.pairRules.value = state.pairRules || '';
    els.sameTeacherRules.value = state.sameTeacherRules || '';
    syncCreditAvailability();
  }

  function persistState() {
    state.query = els.searchInput.value;
    state.minCredit = Number(els.minCredit.value || 0);
    state.maxCredit = Number(els.maxCredit.value || 36);
    state.maxEarly = clampDayLimit(els.maxEarly.value);
    state.maxLunch = clampDayLimit(els.maxLunch.value);
    state.maxLate = clampDayLimit(els.maxLate.value);
    els.maxEarly.value = state.maxEarly;
    els.maxLunch.value = state.maxLunch;
    els.maxLate.value = state.maxLate;
    state.minFreeDays = clampDayLimit(els.minFreeDays.value);
    els.minFreeDays.value = state.minFreeDays;
    state.blockedTeachers = els.blockedTeachers.value;
    state.pairRules = els.pairRules.value;
    state.sameTeacherRules = els.sameTeacherRules.value;
    HDU.saveState(state);
  }

  function clampDayLimit(value) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) return 5;
    return Math.max(0, Math.min(5, parsed));
  }

  function getSelectionMap() {
    return state.selectedGroups || {};
  }

  function setSelectionMap(next) {
    state.selectedGroups = next;
    clearCandidateState();
    persistState();
    rebuildSelection();
    renderAll();
  }

  function clearCandidateState() {
    state.activeCandidate = '';
    state.candidateCursor = 0;
    state.candidatePreviewEnabled = false;
    activeSolution = null;
    solutions = [];
  }

  function rebuildSelection() {
    selectedGroups = Object.values(getSelectionMap()).map((selection) => HDU.groupFromSelection(selection, courses));
  }

  function selectionSnapshot(course, source = 'manual') {
    return {
      id: course.id,
      groupId: course.groupId,
      courseName: course.courseName,
      sectionName: course.sectionName,
      displayCode: course.displayCode,
      rawCourseCode: course.rawCourseCode,
      teacher: course.teacher,
      timeText: course.timeText,
      source,
    };
  }

  function addSection(course, source = 'manual') {
    const map = getSelectionMap();
    const entry = map[course.groupId] || {
      groupId: course.groupId,
      groupName: course.courseName,
      items: [],
      lockedItemId: '',
    };
    if (!entry.items.some((item) => item.id === course.id)) {
      entry.items.push(selectionSnapshot(course, source));
    }
    map[course.groupId] = entry;
    setSelectionMap(map);
  }

  function addManySections(items, source = 'base') {
    const map = getSelectionMap();
    for (const course of items) {
      const entry = map[course.groupId] || {
        groupId: course.groupId,
        groupName: course.courseName,
        items: [],
        lockedItemId: '',
      };
      if (!entry.items.some((item) => item.id === course.id)) {
        entry.items.push(selectionSnapshot(course, source));
      }
      if (source === 'base') entry.lockedItemId = course.id;
      map[course.groupId] = entry;
    }
    setSelectionMap(map);
  }

  function applyLegacyCourseLockMigration() {
    const legacyText = String(state.legacyCourseLockText || '').trim();
    if (!legacyText) return;
    const migration = HDU.migrateLegacyCourseLocks(courses, legacyText);
    const map = getSelectionMap();
    const warnings = [...migration.unresolved];
    for (const course of migration.matches) {
      const entry = map[course.groupId] || {
        groupId: course.groupId,
        groupName: course.courseName,
        items: [],
        lockedItemId: '',
      };
      if (entry.lockedItemId && entry.lockedItemId !== course.id) {
        warnings.push(`${course.courseName}（已有其他锁定教学班）`);
        continue;
      }
      if (!entry.items.some((item) => item.id === course.id)) {
        entry.items.push(selectionSnapshot(course, 'legacy'));
      }
      entry.lockedItemId = course.id;
      map[course.groupId] = entry;
    }
    state.selectedGroups = map;
    state.legacyCourseLockText = '';
    state.legacyCourseLockWarnings = [...new Set(warnings)];
    persistState();
  }

  async function autoImportPersonalSchedule() {
    if ((state.baseCourseIds || []).length || state.personalScheduleAutoImported) return;
    let data;
    try {
      data = await HDU.fetchJSON(HDU.PERSONAL_SCHEDULE_API);
    } catch {
      return;
    }
    const list = Array.isArray(data?.items) ? data.items : Array.isArray(data) ? data : [];
    if (!list.length) return;
    const matched = list.map(matchImportedCourse).filter(Boolean);
    if (!matched.length) {
      state.personalScheduleAutoImported = true;
      state.baseScheduleName = '个人课表未匹配';
      persistState();
      renderBaseSummary();
      return;
    }
    state.baseCourseIds = [...new Set(matched.map((item) => item.id))];
    state.baseScheduleName = '个人课表';
    state.personalScheduleAutoImported = true;
    addManySections(matched, 'base');
  }

  function removeGroup(groupId) {
    const map = getSelectionMap();
    delete map[groupId];
    setSelectionMap(map);
  }

  function removeOption(groupId, itemId) {
    const map = getSelectionMap();
    const entry = map[groupId];
    if (!entry) return;
    entry.items = entry.items.filter((item) => item.id !== itemId);
    if (entry.lockedItemId === itemId) entry.lockedItemId = '';
    if (entry.items.length === 0) delete map[groupId];
    setSelectionMap(map);
  }

  function toggleLockOption(groupId, itemId) {
    const map = getSelectionMap();
    const entry = map[groupId];
    if (!entry) return;
    entry.lockedItemId = entry.lockedItemId === itemId ? '' : itemId;
    setSelectionMap(map);
  }

  function isSelected(course) {
    const entry = getSelectionMap()[course.groupId];
    return Boolean(entry?.items?.some((item) => item.id === course.id));
  }

  function isLocked(course) {
    const entry = getSelectionMap()[course.groupId];
    return entry?.lockedItemId === course.id;
  }

  function lockedItemIds() {
    return new Set(selectedGroups.map((group) => group.lockedItemId).filter(Boolean));
  }

  function baseIdSet() {
    return new Set(state.baseCourseIds || []);
  }

  function allSelectedItems() {
    return selectedGroups.flatMap((group) => group.items);
  }

  function courseWarnings(course) {
    const others = allSelectedItems().filter((item) => item.id !== course.id);
    return {
      time: others.some((item) => HDU.courseConflict(item, course)),
      sameBase: others.some((item) => HDU.sameBaseCourse(item, course)),
    };
  }

  function activeItems() {
    if (state.candidatePreviewEnabled && activeSolution) return activeSolution.items;
    if (state.candidatePreviewEnabled && state.activeCandidate) {
      const found = solutions.find((item) => item.signature === state.activeCandidate);
      if (found) return found.items;
    }
    const preview = [];
    for (const group of selectedGroups) {
      const picked = group.lockedItemId
        ? group.items.find((item) => item.id === group.lockedItemId)
        : group.items[0];
      if (picked) preview.push(picked);
    }
    return preview;
  }

  function signatureForItems(items) {
    return items.map((item) => item.id).sort().join('|');
  }

  function withSolutionSignatures(generated) {
    const dismissed = new Set(state.dismissedCandidates || []);
    const rows = (generated.results || []).map((solution) => ({
      ...solution,
      signature: solution.signature || signatureForItems(solution.items || []),
    })).filter((solution) => !dismissed.has(solution.signature));
    return { ...generated, results: rows };
  }

  function runSchedulerSync(type, groups, snapshot, limit) {
    if (type === 'estimate') return HDU.estimateSolutions(groups, snapshot, limit);
    if (type === 'generate') return withSolutionSignatures(HDU.generateSolutions(groups, snapshot, limit));
    if (type === 'diagnose') return HDU.diagnoseNoSolutions(groups, snapshot, limit || {});
    throw new Error(`Unknown scheduler job: ${type}`);
  }

  function runSchedulerWorker(type, groups, snapshot, limit, context = {}) {
    if (workerUnavailable || typeof Worker === 'undefined') {
      return Promise.resolve(runSchedulerSync(type, groups, snapshot, type === 'diagnose' ? context : limit));
    }
    return new Promise((resolve, reject) => {
      const jobId = `${Date.now()}-${workerJobId += 1}`;
      let worker;
      try {
        worker = new Worker(new URL('scheduler-worker.js', window.location.href));
      } catch (error) {
        workerUnavailable = true;
        resolve(runSchedulerSync(type, groups, snapshot, limit));
        return;
      }
      const finish = (callback, value) => {
        worker.terminate();
        callback(value);
      };
      worker.onmessage = (event) => {
        const message = event.data || {};
        if (message.id !== jobId) return;
        if (message.ok) finish(resolve, type === 'generate' ? withSolutionSignatures(message.result) : message.result);
        else finish(reject, new Error(message.error || 'Scheduler worker failed'));
      };
      worker.onerror = (event) => {
        workerUnavailable = true;
        finish(reject, new Error(event.message || 'Scheduler worker failed'));
      };
      worker.postMessage({ id: jobId, type, groups, state: snapshot, limit, context });
    }).catch(() => runSchedulerSync(type, groups, snapshot, type === 'diagnose' ? context : limit));
  }

  function setSchedulingBusy(busy) {
    schedulingBusy = busy;
    [els.generate, els.estimate].forEach((button) => {
      if (button) button.disabled = busy;
    });
  }

  function constraintToken(course) {
    return HDU.baseCourseCode(course.displayCode || course.sectionName || course.groupId || course.courseName);
  }

  function schedulerState() {
    return {
      minCredit: creditDataAvailable ? state.minCredit : 0,
      maxCredit: creditDataAvailable ? state.maxCredit : 45,
      maxEarly: state.maxEarly,
      maxLunch: state.maxLunch,
      maxLate: state.maxLate,
      minFreeDays: state.minFreeDays,
      blockedTeachers: state.blockedTeachers || '',
      pairRules: state.pairRules || '',
      sameTeacherRules: state.sameTeacherRules || '',
    };
  }

  function courseNameCodeText(course) {
    return [
      course.courseName,
      course.sectionName,
      course.displayCode,
      course.groupId,
      HDU.baseCourseCode(course.displayCode),
      HDU.baseCourseCode(course.sectionName),
    ].filter(Boolean).join(' ').toLowerCase();
  }

  function appendTextareaLine(textarea, value) {
    const token = String(value || '').trim();
    if (!token) return;
    const lines = String(textarea.value || '').split(/\n+/).map((item) => item.trim()).filter(Boolean);
    if (!lines.includes(token)) lines.push(token);
    textarea.value = lines.join('\n');
    persistState();
    clearCandidateState();
    renderAll();
  }

  function addAndLockSection(course, source = 'manual') {
    if (!course) return;
    const map = getSelectionMap();
    const entry = map[course.groupId] || {
      groupId: course.groupId,
      groupName: course.courseName,
      items: [],
      lockedItemId: '',
    };
    if (!entry.items.some((item) => item.id === course.id)) {
      entry.items.push(selectionSnapshot(course, source));
    }
    entry.lockedItemId = course.id;
    map[course.groupId] = entry;
    setSelectionMap(map);
  }

  function renderLegacyCourseLockWarning() {
    if (!els.legacyWarning) return;
    const warnings = Array.isArray(state.legacyCourseLockWarnings) ? state.legacyCourseLockWarnings : [];
    els.legacyWarning.hidden = !warnings.length;
    els.legacyWarning.textContent = warnings.length
      ? `旧版本课程约束未自动锁定：${warnings.join('、')}。请搜索并选择具体教学班后锁定。`
      : '';
  }

  function appendPairRule(left, right) {
    appendTextareaLine(els.pairRules, `${constraintToken(left)} -> ${constraintToken(right)}`);
  }

  function appendSameTeacherRule(left, right) {
    appendTextareaLine(els.sameTeacherRules, `${constraintToken(left)} = ${constraintToken(right)}`);
  }

  function tokenForGroup(group) {
    return group.name || group.id;
  }

  function parseRuleTriples(text) {
    return String(text || '')
      .split(/[\n;；]+/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const match = line.match(/^(.*?)(->|~|=|\/~|->\/~)(.*)$/);
        if (!match) return null;
        const op = match[2];
        const left = match[1].trim();
        const right = match[3].trim();
        if (!left || !right) return null;
        return { left, right, op };
      })
      .filter(Boolean);
  }

  function normalizeRuleKey(left, right, op) {
    return [String(left).trim().toLowerCase(), op, String(right).trim().toLowerCase()].join('||');
  }

  function upsertLinkRule(textarea, left, right, op) {
    const key = normalizeRuleKey(left, right, op);
    const rules = parseRuleTriples(textarea.value);
    const existsIndex = rules.findIndex((rule) => normalizeRuleKey(rule.left, rule.right, op) === key);
    if (existsIndex >= 0) {
      rules.splice(existsIndex, 1);
    } else {
      rules.push({ left, right, op });
    }
    textarea.value = rules.map((rule) => `${rule.left} ${rule.op} ${rule.right}`).join('\n');
    persistState();
    clearCandidateState();
    renderAll();
  }

  function setRulePresence(textarea, left, right, op, present) {
    const key = normalizeRuleKey(left, right, op);
    const rules = parseRuleTriples(textarea.value)
      .filter((rule) => normalizeRuleKey(rule.left, rule.right, op) !== key);
    if (present) rules.push({ left, right, op });
    textarea.value = rules.map((rule) => `${rule.left} ${rule.op} ${rule.right}`).join('\n');
  }

  function hasRule(textarea, left, right, op) {
    const key = normalizeRuleKey(left, right, op);
    return parseRuleTriples(textarea.value).some((rule) => normalizeRuleKey(rule.left, rule.right, op) === key);
  }

  function removeLinkRule(textarea, left, right, op) {
    const key = normalizeRuleKey(left, right, op);
    const rules = parseRuleTriples(textarea.value)
      .filter((rule) => normalizeRuleKey(rule.left, rule.right, op) !== key);
    textarea.value = rules.map((rule) => `${rule.left} ${rule.op} ${rule.right}`).join('\n');
    persistState();
    clearCandidateState();
    renderAll();
  }

  function renderSchemeCurrentRules() {
    if (!els.schemeCurrentRules) return;
    const rows = [
      ...parseRuleTriples(els.pairRules.value).map((rule) => ({ ...rule, op: '->', label: '强制一起', textarea: els.pairRules })),
      ...parseRuleTriples(els.sameTeacherRules.value).map((rule) => ({ ...rule, op: '=', label: '教师一致', textarea: els.sameTeacherRules })),
    ];
    if (!rows.length) {
      els.schemeCurrentRules.innerHTML = '<span class="quick-empty">还没有加入方案级约束</span>';
      return;
    }
    els.schemeCurrentRules.innerHTML = rows.map((rule, index) => `
      <div class="scheme-rule-item">
        <div>
          <strong>${escapeHtml(rule.left)} ${rule.op} ${escapeHtml(rule.right)}</strong>
          <span>${escapeHtml(rule.label)}</span>
        </div>
        <button class="danger-btn small" type="button" data-remove-scheme-rule="${index}">删除</button>
      </div>
    `).join('');
    els.schemeCurrentRules.querySelectorAll('[data-remove-scheme-rule]').forEach((button) => {
      button.addEventListener('click', () => {
        const rule = rows[Number(button.dataset.removeSchemeRule)];
        if (rule) removeLinkRule(rule.textarea, rule.left, rule.right, rule.op);
      });
    });
  }

  function addCustomSchemeRule() {
    const left = els.schemeLeftCourse.value.trim();
    const right = els.schemeRightCourse.value.trim();
    if (!left || !right) return;
    const textarea = els.schemeMode.value === 'teacher' ? els.sameTeacherRules : els.pairRules;
    const op = els.schemeMode.value === 'teacher' ? '=' : '->';
    upsertLinkRule(textarea, left, right, op);
    els.schemeLeftCourse.value = '';
    els.schemeRightCourse.value = '';
  }

  function currentSolution() {
    return solutions[state.candidateCursor] || null;
  }

  function applyCandidate(index, preview = state.candidatePreviewEnabled) {
    if (!solutions.length) {
      activeSolution = null;
      state.activeCandidate = '';
      state.candidateCursor = 0;
      state.candidatePreviewEnabled = false;
      return;
    }
    const nextIndex = Math.max(0, Math.min(index, solutions.length - 1));
    state.candidateCursor = nextIndex;
    activeSolution = solutions[nextIndex];
    state.activeCandidate = activeSolution.signature;
    state.candidatePreviewEnabled = Boolean(preview);
    persistState();
    renderAll();
  }

  function setCandidatePage(value) {
    const page = Number(value);
    if (!Number.isInteger(page) || page < 1 || !solutions.length) {
      const currentPage = solutions.length ? (state.candidateCursor || 0) + 1 : '';
      if (els.candidatePage) els.candidatePage.value = String(currentPage);
      if (els.tableCandidatePage) els.tableCandidatePage.value = String(currentPage);
      return;
    }
    applyCandidate(page - 1);
  }

  function previewCurrentCandidate() {
    const solution = currentSolution();
    if (!solution) return;
    activeSolution = solution;
    state.activeCandidate = solution.signature;
    state.candidatePreviewEnabled = true;
    persistState();
    renderAll();
  }

  function returnToOriginalTimetable() {
    state.candidatePreviewEnabled = false;
    persistState();
    renderAll();
  }

  function renderCourseList() {
    const filtered = HDU.filterCourses(courses, state.query);
    els.courseCount.textContent = `${filtered.length} 门`;
    const grouped = HDU.groupCourses(filtered);
    if (!grouped.length) {
      els.courseList.innerHTML = '<div class="empty-state">没有匹配的课程</div>';
      return;
    }

    els.courseList.innerHTML = grouped.map((group) => {
      const tags = [group.kind, group.credits ? `${formatCredit(group.credits)} 学分` : ''].filter(Boolean);
      const itemsHtml = group.items.slice(0, 5).map((item) => {
        const selected = isSelected(item);
        const warnings = selected ? courseWarnings(item) : { time: false, sameBase: false };
        const warningClass = warnings.time ? 'has-time-conflict' : warnings.sameBase ? 'has-same-base' : '';
        const warningText = warnings.time ? '时间冲突' : warnings.sameBase ? '同课程号' : '';
        return `
        <div class="option ${warningClass}">
          <div>
            <strong>${escapeHtml(item.sectionName)}</strong>
            <div class="meta">${escapeHtml(item.teacher || '未填写教师')} · ${escapeHtml(item.timeText || '无时间信息')}</div>
            ${warningText ? `<div class="option-warning">${escapeHtml(warningText)}</div>` : ''}
          </div>
          <button class="${selected ? 'danger-btn' : 'ghost-btn'} small" data-toggle-course="${escapeHtml(item.id)}">${selected ? '移除' : '加入'}</button>
        </div>
      `;
      }).join('');
      return `
        <article class="course-card">
          <h4>${escapeHtml(group.name)}</h4>
          <div class="meta">${escapeHtml(group.id)} · ${escapeHtml(group.kind || '未知类型')} · ${group.items.length} 个教学班</div>
          <div class="tag-row">${tags.map((tag) => `<span class="tag">${escapeHtml(tag)}</span>`).join('')}</div>
          <div class="option-row">${itemsHtml}</div>
        </article>
      `;
    }).join('');

    els.courseList.querySelectorAll('[data-toggle-course]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.toggleCourse);
        if (!course) return;
        if (isSelected(course)) removeOption(course.groupId, course.id);
        else addSection(course);
      });
    });
  }

  function renderSelectedList() {
    if (!selectedGroups.length) {
      els.selectedList.innerHTML = '<div class="empty-state">还没有加入任何课组</div>';
      return;
    }
    const bases = baseIdSet();
    els.selectedList.innerHTML = selectedGroups.map((group) => `
      <article class="selected-card">
        <h4>${escapeHtml(group.name)}</h4>
        <div class="meta">${group.items.length} 个可选教学班${group.lockedItemId ? ' · 已锁定' : ''}</div>
        <div class="chosen">
          ${group.items.map((item) => {
            const locked = group.lockedItemId === item.id;
            const isBase = bases.has(item.id);
            const warnings = courseWarnings(item);
            const warningClass = warnings.time ? 'has-time-conflict' : warnings.sameBase ? 'has-same-base' : '';
            return `
              <span class="pill ${locked ? 'is-locked' : ''} ${isBase ? 'is-base' : ''} ${warningClass}">
                <span>${escapeHtml(item.sectionName)}</span>
                ${isBase ? '<span class="mini-badge">底板</span>' : ''}
                <button class="lock-btn ${locked ? 'is-locked' : ''}" data-lock="${escapeHtml(group.id)}:${escapeHtml(item.id)}">${locked ? '已锁' : '锁定'}</button>
                <button class="ghost-btn small" data-remove="${escapeHtml(group.id)}:${escapeHtml(item.id)}">删</button>
              </span>
            `;
          }).join('')}
        </div>
      </article>
    `).join('');

    els.selectedList.querySelectorAll('[data-remove]').forEach((button) => {
      button.addEventListener('click', () => {
        const [groupId, itemId] = button.dataset.remove.split(':');
        removeOption(groupId, itemId);
      });
    });
    els.selectedList.querySelectorAll('[data-lock]').forEach((button) => {
      button.addEventListener('click', () => {
        const [groupId, itemId] = button.dataset.lock.split(':');
        toggleLockOption(groupId, itemId);
      });
    });
    renderConstraintQuickPicks();
  }

  function withdrawalCount(items) {
    const shown = new Set(items.map((item) => item.id));
    return (state.baseCourseIds || []).filter((id) => !shown.has(id)).length;
  }

  function courseBrief(item) {
    if (!item) return '';
    const code = item.displayCode || item.sectionName || item.groupId || '';
    return [item.courseName, code].filter(Boolean).join(' / ');
  }

  function compactCourseList(items, limit = 4) {
    const labels = [...new Set((items || []).map(courseBrief).filter(Boolean))];
    if (!labels.length) return '';
    const shown = labels.slice(0, limit).join('、');
    return labels.length > limit ? `${shown} 等 ${labels.length} 门` : shown;
  }

  function baseScheduleItems() {
    return (state.baseCourseIds || []).map((id) => courses.find((course) => course.id === id)).filter(Boolean);
  }

  function solutionDiff(items) {
    const solutionIds = new Set((items || []).map((item) => item.id));
    const baseIds = baseIdSet();
    const baseItems = baseScheduleItems();
    return {
      kept: baseItems.filter((item) => solutionIds.has(item.id)),
      dropped: baseItems.filter((item) => !solutionIds.has(item.id)),
      added: (items || []).filter((item) => !baseIds.has(item.id)),
      locked: (items || []).filter((item) => lockedItemIds().has(item.id)),
      hasBase: Boolean((state.baseCourseIds || []).length),
    };
  }

  function solutionExplanation(solution) {
    if (!solution) return [];
    const items = solution.items || [];
    const metrics = solution.metrics || HDU.countMetrics(items);
    const diff = solutionDiff(items);
    const sections = [
      {
        title: '排序依据',
        rows: [
          `全天无课 ${metrics.freeDays} 天，候选排序会优先把全天无课更多的方案放前面。`,
          `排序指标 ${Number(solution.score || 0).toFixed(1)}，数值越低代表时间负担越小。`,
        ],
      },
      {
        title: '方案统计',
        rows: [
          `总学分 ${formatCredit(solution.credits)}；早八 ${metrics.earlyDays} 天，午间压缩 ${metrics.lunchDays} 天，晚课 ${metrics.lateDays} 天。`,
          diff.hasBase
            ? `相对“${state.baseScheduleName || '现有课表'}”需退选 ${diff.dropped.length} 门，保留 ${diff.kept.length} 门，新增/替换 ${diff.added.length} 门。`
            : '尚未导入个人/班级底板，退课数按 0 计算。',
        ],
      },
    ];

    if (diff.hasBase) {
      const rows = [];
      if (diff.dropped.length) rows.push(`需退选：${compactCourseList(diff.dropped)}。`);
      if (diff.added.length) rows.push(`新增/替换：${compactCourseList(diff.added)}。`);
      if (diff.kept.length) rows.push(`继续保留：${compactCourseList(diff.kept)}。`);
      sections.push({ title: '底板变化', rows: rows.length ? rows : ['与当前底板完全一致，不需要退选或新增。'] });
    }

    if (diff.locked.length) {
      sections.push({
        title: '锁定课程',
        rows: [`已保留锁定课程：${compactCourseList(diff.locked)}。`],
      });
    }

    const pairHits = HDU.parsePairRules(els.pairRules.value).filter((rule) => {
      const left = rule.from.toLowerCase();
      const right = rule.to.toLowerCase();
      return items.some((item) => HDU.courseSearchText(item).includes(left))
        && items.some((item) => HDU.courseSearchText(item).includes(right));
    });
    if (pairHits.length) {
      sections.push({
        title: '强制一起',
        rows: [`满足：${pairHits.slice(0, 4).map((rule) => `${rule.from} -> ${rule.to}`).join('；')}。`],
      });
    }

    const sameTeacherHits = HDU.parsePairRules(els.sameTeacherRules.value).filter((rule) => {
      const left = items.filter((item) => HDU.courseSearchText(item).includes(rule.from.toLowerCase()));
      const right = items.filter((item) => HDU.courseSearchText(item).includes(rule.to.toLowerCase()));
      if (!left.length || !right.length) return false;
      const leftTeachers = new Set(left.flatMap((item) => HDU.splitTeachers(item.teacher)));
      return right.some((item) => HDU.splitTeachers(item.teacher).some((teacher) => leftTeachers.has(teacher)));
    });
    if (sameTeacherHits.length) {
      sections.push({
        title: '教师一致',
        rows: [`满足：${sameTeacherHits.slice(0, 4).map((rule) => `${rule.from} = ${rule.to}`).join('；')}。`],
      });
    }

    if (diff.dropped.length) {
      sections.push({
        title: '注意',
        rows: ['退课是真实执行中的高风险动作；当前页面只用于模拟与导出，执行前请再次确认。'],
      });
    }

    return sections.filter((section) => section.rows?.length);
  }

  function renderSolutionDetails(solution) {
    const sections = solutionExplanation(solution);
    if (!sections.length) return '';
    return `
      <div class="solution-explain">
        ${sections.map((section) => `
          <section class="solution-explain-section">
            <h5>${escapeHtml(section.title)}</h5>
            ${section.rows.map((row) => `<div>${escapeHtml(row)}</div>`).join('')}
          </section>
        `).join('')}
      </div>
    `;
  }

  function renderDiagnostics(reasons) {
    const rows = Array.isArray(reasons) ? reasons : [];
    if (!rows.length) {
      els.resultList.innerHTML = '<div class="empty-state">没有找到候选方案，但暂时无法判断具体原因。建议先减少约束条件后重试。</div>';
      return;
    }
    els.resultList.innerHTML = `
      <div class="empty-state diagnostic-state">
        <strong>没有候选方案，可能原因如下：</strong>
        ${rows.map((reason) => `
          <div class="diagnostic-item diagnostic-${escapeHtml(reason.type || 'unknown')}">
            <div class="diagnostic-type">${escapeHtml(diagnosticTypeLabel(reason.type))}</div>
            <div>${escapeHtml(reason.text || reason)}</div>
            ${reason.action ? `<div class="diagnostic-action">建议：${escapeHtml(reason.action)}</div>` : ''}
          </div>
        `).join('')}
      </div>
    `;
  }

  function diagnosticTypeLabel(type) {
    return ({
      course: '课组数据',
      locked: '锁定冲突',
      conflict: '课程互斥',
      credit: '学分范围',
      teacher: '教师条件',
      time: '时间约束',
      scheme: '方案约束',
      unknown: '综合约束',
    })[type] || '诊断';
  }

  function renderSummary() {
    const items = activeItems();
    const credits = items.reduce((sum, item) => sum + item.credits, 0);
    const metrics = HDU.countMetrics(items);
    const withdraw = withdrawalCount(items);
    els.summaryChips.innerHTML = [
      `学分 ${formatCredit(credits)}`,
      `早八 ${metrics.earlyDays}`,
      `午间压缩 ${metrics.lunchDays}`,
      `晚课 ${metrics.lateDays}`,
      `全天无课 ${metrics.freeDays}`,
      `需退课 ${withdraw}`,
    ].map((text) => `<span class="chip primary">${escapeHtml(text)}</span>`).join('');
  }

  function weekLabel(meeting) {
    const weeks = [...meeting.weeks].sort((a, b) => a - b);
    if (!weeks.length || weeks.length === 20) return '全周';
    const odd = weeks.every((week) => week % 2 === 1);
    const even = weeks.every((week) => week % 2 === 0);
    if (odd) return '单周';
    if (even) return '双周';
    if (weeks.length <= 4) return `${weeks.join(',')}周`;
    return `${weeks[0]}-${weeks[weeks.length - 1]}周`;
  }

  function timetableCard(entry, total) {
    const item = entry.item;
    const detail = [item.teacher, item.location].filter(Boolean).join(' · ');
    const mode = state.candidatePreviewEnabled ? 'best' : 'selected';
    const compact = total > 1 ? 'is-compact' : '';
    const warnings = courseWarnings(item);
    const warningClass = warnings.time ? 'has-time-conflict' : warnings.sameBase ? 'has-same-base' : '';
    const locked = lockedItemIds().has(item.id);
    return `
      <button class="slot-item ${mode} ${compact} ${warningClass} ${locked ? 'is-locked' : ''} ${selectedCourseId === item.id ? 'is-focused' : ''}" type="button" data-open-course="${escapeHtml(item.id)}">
        <div class="course-name">${escapeHtml(item.courseName)}</div>
        <div class="section-name">${escapeHtml(item.sectionName)}</div>
        ${detail ? `<div class="course-detail">${escapeHtml(detail)}</div>` : ''}
        ${locked ? '<span class="lock-badge">锁定</span>' : ''}
        <span class="week-badge">${escapeHtml(weekLabel(entry.meeting))}</span>
      </button>
    `;
  }

  function rawText(course, keys) {
    for (const key of keys) {
      const value = course.raw?.[key];
      if (value !== undefined && value !== null && String(value).trim()) return String(value).trim();
    }
    return '';
  }

  function courseModalRows(course) {
    return [
      ['教学班', course.sectionName],
      ['课程编号', course.groupId],
      ['原始课程号', course.rawCourseCode && course.rawCourseCode !== course.groupId ? course.rawCourseCode : '同课程编号'],
      ['教师', course.teacher || '未填写'],
      ['时间', course.timeText || '无时间信息'],
      ['地点', course.location || rawText(course, ['cdlbmc', 'cdejlbmc']) || '未填写'],
      ['类型', course.kind || '未填写'],
      ['学分', course.credits ? formatCredit(course.credits) : '未填写'],
      ['容量', course.capacity ? `${course.enrolled || 0}/${course.capacity}` : '未填写'],
      ['状态', course.status || '未填写'],
      ['备注', rawText(course, ['xkbz', 'bz', 'skdxssxy']) || '无'],
    ];
  }

  function refreshModal(course) {
    const selected = isSelected(course);
    const locked = isLocked(course);
    els.modalTitle.textContent = course.courseName;
    els.modalStatus.textContent = selected ? (locked ? '已锁定' : '已加入') : '未加入';
    els.modalStatus.className = `modal-status ${locked ? 'is-locked' : selected ? 'is-selected' : ''}`;
    els.modalBody.innerHTML = `
      <div class="modal-course-card">
        <h3>${escapeHtml(course.courseName)}</h3>
        <p>${escapeHtml(course.timeText || '无时间信息')}</p>
        <strong>${escapeHtml(course.teacher || '未填写教师')}</strong>
      </div>
      <div class="course-detail-grid">
        ${courseModalRows(course).map(([label, value]) => `
          <div class="detail-label">${escapeHtml(label)}</div>
          <div class="detail-value">${escapeHtml(value)}</div>
        `).join('')}
      </div>
    `;
    els.modalSelectToggle.textContent = selected ? '退课' : '选课';
    els.modalSelectToggle.className = selected ? 'danger-btn' : 'primary-btn';
    els.modalLockToggle.textContent = locked ? '取消锁定' : '锁定';
    els.modalLockToggle.className = locked ? 'danger-btn ghost-danger' : 'ghost-btn';
  }

  function openCourseModal(courseId) {
    const course = courses.find((item) => item.id === courseId);
    if (!course) return;
    modalCourseId = course.id;
    selectedCourseId = course.id;
    refreshModal(course);
    els.modal.classList.add('is-open');
    els.modal.setAttribute('aria-hidden', 'false');
  }

  function closeCourseModal() {
    modalCourseId = '';
    selectedCourseId = '';
    els.modal.classList.remove('is-open');
    els.modal.setAttribute('aria-hidden', 'true');
    renderTimetable();
  }

  function selectedModalCourse() {
    return courses.find((course) => course.id === modalCourseId) || null;
  }

  function renderTimetable() {
    const items = activeItems();
    const slots = new Map();
    for (const item of items) {
      for (const meeting of item.meetings) {
        for (let slot = meeting.start; slot <= meeting.end; slot += 1) {
          const key = `${meeting.day}:${slot}`;
          if (!slots.has(key)) slots.set(key, []);
          const bucket = slots.get(key);
          if (!bucket.some((entry) => entry.item.id === item.id && entry.meeting.raw === meeting.raw)) {
            bucket.push({ item, meeting });
          }
        }
      }
    }

    const grid = ['<div class="time-cell header-spacer"></div>'];
    for (const day of HDU.DAY_LABELS) grid.push(`<div class="day-cell">${day}</div>`);
    for (let period = 1; period <= HDU.PERIOD_TIMES.length; period += 1) {
      const [start, end] = HDU.PERIOD_TIMES[period - 1].split('-');
      grid.push(`
        <div class="time-cell">
          <div class="period-index">${period}</div>
          <div class="period-time">${start}<br>${end}</div>
        </div>
      `);
      for (let day = 1; day <= HDU.DAY_LABELS.length; day += 1) {
        const entries = slots.get(`${day}:${period}`) || [];
        const inner = entries.map((entry) => timetableCard(entry, entries.length)).join('');
        grid.push(`<div class="slot ${entries.length ? 'blocked' : ''} ${entries.length > 1 ? 'is-split' : ''}">${inner}</div>`);
      }
    }
    els.timetable.innerHTML = grid.join('');
    els.timetable.querySelectorAll('[data-open-course]').forEach((button) => {
      button.addEventListener('click', () => openCourseModal(button.dataset.openCourse));
    });
  }

  function renderBaseSummary() {
    if (!state.baseCourseIds?.length) {
      els.baseSummary.textContent = '尚未导入现有课表。';
      return;
    }
    const baseItems = state.baseCourseIds.map((id) => courses.find((course) => course.id === id)).filter(Boolean);
    const credits = baseItems.reduce((sum, item) => sum + item.credits, 0);
    els.baseSummary.textContent = `${state.baseScheduleName || '现有课表'}：${baseItems.length} 门，${formatCredit(credits)} 学分。`;
  }

  function renderConstraintQuickPicks() {
    const items = allSelectedItems();
    renderLockedQuickPicks(items);
    renderLockedSelectedPicks();
    renderLockedSearchResults();
    renderLegacyCourseLockWarning();
    renderSchemeCurrentRules();
    renderLinkedCourseSuggestions();
  }

  function renderLockedQuickPicks(items) {
    if (!els.lockedQuickPicks) return;
    if (!items.length) {
      els.lockedQuickPicks.innerHTML = '<span class="quick-empty">先在左侧加入课程</span>';
      return;
    }
    const unique = [...new Map(items.map((item) => [item.id, item])).values()];
    els.lockedQuickPicks.innerHTML = unique.map((item) => {
      const locked = isLocked(item);
      return `
        <button class="ghost-btn small locked-add-btn" type="button" data-quick-lock="${escapeHtml(item.id)}" ${locked ? 'disabled' : ''}>
          <span>${escapeHtml(item.courseName)}</span>
          <small>${escapeHtml(item.sectionName || item.displayCode || '教学班')}</small>
        </button>
      `;
    }).join('');
    els.lockedQuickPicks.querySelectorAll('[data-quick-lock]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.quickLock);
        if (course) addAndLockSection(course);
      });
    });
  }

  function renderLockedSelectedPicks() {
    if (!els.lockedSelectedPicks) return;
    const lockedItems = selectedGroups.flatMap((group) => group.items.filter((item) => group.lockedItemId === item.id));
    if (!lockedItems.length) {
      els.lockedSelectedPicks.innerHTML = '<span class="quick-empty">还没有锁定教学班</span>';
      return;
    }
    els.lockedSelectedPicks.innerHTML = lockedItems.map((item) => {
      return `
        <div class="locked-picked-item">
          <div>
            <strong>${escapeHtml(item.courseName)}</strong>
            <span>${escapeHtml(item.sectionName || item.displayCode || '教学班')} · ${escapeHtml(item.teacher || '未填写教师')}</span>
          </div>
          <button class="danger-btn small" type="button" data-unlock-course="${escapeHtml(item.id)}">取消锁定</button>
        </div>
      `;
    }).join('');
    els.lockedSelectedPicks.querySelectorAll('[data-unlock-course]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.unlockCourse);
        if (course) toggleLockOption(course.groupId, course.id);
      });
    });
  }

  function renderLockedSearchResults() {
    if (!els.lockedSearchResults || !els.lockedSearch) return;
    const keyword = els.lockedSearch.value.trim().toLowerCase();
    if (!keyword) {
      els.lockedSearchResults.innerHTML = '<span class="quick-empty">输入关键词后搜索课程</span>';
      return;
    }
    const matches = courses
      .filter((course) => courseNameCodeText(course).includes(keyword))
      .sort((a, b) => courseNameCodeText(a).localeCompare(courseNameCodeText(b), 'zh-Hans-CN'))
      .slice(0, 30);
    if (!matches.length) {
      els.lockedSearchResults.innerHTML = '<span class="quick-empty">没有匹配课程</span>';
      return;
    }
    els.lockedSearchResults.innerHTML = matches.map((course) => {
      const locked = isLocked(course);
      return `
        <div class="locked-search-item">
          <div>
            <strong>${escapeHtml(course.courseName)}</strong>
            <span>${escapeHtml(course.sectionName || course.displayCode || '教学班')} · ${escapeHtml(course.teacher || '未填写教师')}</span>
          </div>
          <button class="${locked ? 'ghost-btn' : 'primary-btn'} small" type="button" data-search-lock="${escapeHtml(course.id)}" ${locked ? 'disabled' : ''}>
            ${locked ? '已锁定' : '加入并锁定'}
          </button>
        </div>
      `;
    }).join('');
    els.lockedSearchResults.querySelectorAll('[data-search-lock]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.searchLock);
        if (course) addAndLockSection(course);
      });
    });
  }

  function linkedCoursePairs(limit = 500) {
    return HDU.findLinkedCoursePairs(courses, limit);
  }

  function renderLinkedCourseSuggestions() {
    if (!els.linkedCourseSuggestions) return;
    const pairs = linkedCoursePairs();
    if (!pairs.length) {
      els.linkedCourseSuggestions.innerHTML = '<span class="quick-empty">暂未识别到关联课程</span>';
      return;
    }
    const pairRuleKeys = new Set(parseRuleTriples(els.pairRules.value).map((rule) => normalizeRuleKey(rule.left, rule.right, '->')));
    const teacherRuleKeys = new Set(parseRuleTriples(els.sameTeacherRules.value).map((rule) => normalizeRuleKey(rule.left, rule.right, '=')));
    els.linkedCourseSuggestions.innerHTML = pairs.map(([left, right], index) => `
      ${(() => {
        const leftToken = tokenForGroup(left);
        const rightToken = tokenForGroup(right);
        const hardActive = pairRuleKeys.has(normalizeRuleKey(leftToken, rightToken, '->'));
        const teacherActive = teacherRuleKeys.has(normalizeRuleKey(leftToken, rightToken, '='));
        return `
          <div class="linked-course-item">
            <div>
              <strong>${escapeHtml(left.name)} + ${escapeHtml(right.name)}</strong>
              <span>${escapeHtml(left.id)} / ${escapeHtml(right.id)}</span>
            </div>
            <div class="linked-actions">
              <button class="primary-btn small ${hardActive ? 'is-active' : ''}" type="button" data-link-hard="${index}">强制一起</button>
              <button class="ghost-btn small ${teacherActive ? 'is-active' : ''}" type="button" data-link-teacher="${index}">教师一致</button>
            </div>
          </div>
        `;
      })()}
    `).join('');
    els.linkedCourseSuggestions.querySelectorAll('[data-link-hard]').forEach((button) => {
      button.addEventListener('click', () => {
        const pair = pairs[Number(button.dataset.linkHard)];
        if (pair) {
          upsertLinkRule(els.pairRules, tokenForGroup(pair[0]), tokenForGroup(pair[1]), '->');
        }
      });
    });
    els.linkedCourseSuggestions.querySelectorAll('[data-link-teacher]').forEach((button) => {
      button.addEventListener('click', () => {
        const pair = pairs[Number(button.dataset.linkTeacher)];
        if (pair) upsertLinkRule(els.sameTeacherRules, tokenForGroup(pair[0]), tokenForGroup(pair[1]), '=');
      });
    });
  }

  function renderResults() {
    if (state.resultListMode === 'favorites') {
      renderFavoriteCandidates();
      return;
    }
    const visible = solutions.filter((solution) => !(state.dismissedCandidates || []).includes(solution.signature));
    if (visible.length !== solutions.length) {
      solutions = visible;
      if (state.candidateCursor >= solutions.length) state.candidateCursor = Math.max(0, solutions.length - 1);
      activeSolution = currentSolution();
      state.activeCandidate = activeSolution?.signature || '';
    }

    if (!solutions.length) {
      els.resultCount.textContent = '0 个方案';
      els.tableResultCount.textContent = '0 个方案';
      if (els.candidatePage) els.candidatePage.value = '';
      if (els.tableCandidatePage) els.tableCandidatePage.value = '';
      const emptyMessage = (state.dismissedCandidates || []).length
        ? '\u5df2\u5220\u9664\u672c\u8f6e\u5019\u9009\u65b9\u6848\uff0c\u53ef\u91cd\u65b0\u751f\u6210\u4ee5\u83b7\u53d6\u65b0\u7ed3\u679c\u3002'
        : '\u70b9\u51fb\u201c\u751f\u6210\u5019\u9009\u8bfe\u8868\u201d\u5f00\u59cb\u679a\u4e3e\u3002';
      els.resultList.innerHTML = `<div class="empty-state">${emptyMessage}</div>`;
      els.candidateTitle.textContent = '当前显示：已选课程预览';
      els.candidatePreview.disabled = true;
      els.candidateReturn.disabled = !state.candidatePreviewEnabled;
      els.tableCandidatePreview.disabled = true;
      els.tableCandidateReturn.disabled = !state.candidatePreviewEnabled;
      els.candidateFavorite.disabled = true;
      els.candidateDismiss.disabled = true;
      return;
    }

    const solution = currentSolution() || solutions[0];
    const index = state.candidateCursor || 0;
    const favorite = (state.favoriteCandidates || []).includes(solution.signature);
    els.resultCount.textContent = `${index + 1} / ${solutions.length} 个方案`;
    els.tableResultCount.textContent = `${index + 1} / ${solutions.length} 个方案`;
    if (els.candidatePage) els.candidatePage.value = String(index + 1);
    if (els.tableCandidatePage) els.tableCandidatePage.value = String(index + 1);
    els.candidateTitle.textContent = state.candidatePreviewEnabled
      ? `当前显示：候选课表 ${index + 1}`
      : `当前显示：原课表；候选选中 ${index + 1}`;
    els.candidateFavorite.textContent = favorite ? '已收藏' : '收藏';
    els.candidatePreview.textContent = state.candidatePreviewEnabled ? '正在显示' : '显示此方案';
    els.candidatePreview.disabled = state.candidatePreviewEnabled && state.activeCandidate === solution.signature;
    els.candidateReturn.disabled = !state.candidatePreviewEnabled;
    els.tableCandidatePreview.textContent = state.candidatePreviewEnabled ? '正在显示' : '显示此方案';
    els.tableCandidatePreview.disabled = state.candidatePreviewEnabled && state.activeCandidate === solution.signature;
    els.tableCandidateReturn.disabled = !state.candidatePreviewEnabled;
    els.candidateFavorite.disabled = false;
    els.candidateDismiss.disabled = false;
    els.resultList.innerHTML = `
      <article class="result-card active">
        <h4>方案 ${index + 1}${favorite ? ' · 已收藏' : ''}</h4>
        <div class="meta">排序指标 ${solution.score.toFixed(1)}（越低越好） · ${formatCredit(solution.credits)} 学分 · 需退课 ${withdrawalCount(solution.items)}</div>
        <div class="result-stats">
          <span class="stat">早八 ${solution.metrics.earlyDays}</span>
          <span class="stat">午间 ${solution.metrics.lunchDays}</span>
          <span class="stat">晚课 ${solution.metrics.lateDays}</span>
          <span class="stat">全天无课 ${solution.metrics.freeDays}</span>
        </div>
        <div class="candidate-course-list">
          ${solution.items.map((item) => `
            <span>${escapeHtml(item.courseName)} / ${escapeHtml(item.sectionName)}</span>
          `).join('')}
        </div>
        ${renderSolutionDetails(solution)}
      </article>
    `;
  }

  function showFavoriteList() {
    state.resultListMode = state.resultListMode === 'favorites' ? 'current' : 'favorites';
    persistState();
    renderResults();
  }

  function buildSolutions(limit = 500) {
    return withSolutionSignatures(HDU.generateSolutions(candidateGroups(), schedulerState(), limit));
  }

  function candidateGroups() {
    return selectedGroups.map((group) => ({
        id: group.id,
        name: group.name,
        items: [...group.items],
        lockedItemId: group.lockedItemId || '',
        optional: !group.lockedItemId,
      })).filter((group) => group.items.length);
  }

  function renderAll() {
    if (els.exportCurrent) {
      els.exportCurrent.textContent = state.candidatePreviewEnabled ? '保存为目标课表' : '保存当前课表';
    }
    renderSummary();
    renderTimetable();
    renderCourseList();
    renderSelectedList();
    renderConstraintQuickPicks();
    renderBaseSummary();
    renderResults();
  }

  async function estimateCandidates() {
    if (schedulingBusy) return;
    persistState();
    const groups = candidateGroups();
    setSchedulingBusy(true);
    els.estimateText.textContent = '正在估算候选课表数量...';
    try {
      const estimate = await runSchedulerWorker('estimate', groups, schedulerState(), 20000);
      const text = estimate.capped
        ? (estimate.approximate && estimate.count < estimate.limit ? `估算已快速截断，至少 ${estimate.count} 个候选课表` : `候选数量超过 ${estimate.limit} 个`)
        : `预计 ${estimate.count} 个候选课表`;
      state.candidateEstimate = text;
      els.estimateText.textContent = text;
      if (!estimate.capped && estimate.count === 0) {
        const reasons = await runSchedulerWorker('diagnose', groups, schedulerState(), 0);
        renderDiagnostics(reasons);
      }
      persistState();
    } catch (error) {
      els.estimateText.textContent = `估算失败：${error.message || error}`;
    } finally {
      setSchedulingBusy(false);
    }
  }

  async function generateCandidates() {
    if (schedulingBusy) return;
    persistState();
    const groups = candidateGroups();
    setSchedulingBusy(true);
    els.estimateText.textContent = '正在检查候选规模...';
    try {
      const estimate = await runSchedulerWorker('estimate', groups, schedulerState(), 501);
      if (estimate.capped || estimate.count > 500) {
        const message = '当前候选课表过多，建议添加更多约束条件。是否仍然继续生成前 500 个候选方案？';
        if (!window.confirm(message)) {
          els.estimateText.textContent = '已取消生成。建议添加更多约束条件后再试。';
          return;
        }
      }
      els.estimateText.textContent = '正在生成候选课表...';
      const generated = await runSchedulerWorker('generate', groups, schedulerState(), 500);
      solutions = generated.results;
      state.candidateCursor = 0;
      activeSolution = solutions[0] || null;
      state.activeCandidate = activeSolution ? activeSolution.signature : '';
      state.candidatePreviewEnabled = Boolean(activeSolution);
      els.estimateText.textContent = generated.capped
        ? `已生成 ${solutions.length} 个较优方案，候选可能更多。`
        : `已生成 ${solutions.length} 个候选方案。`;
      persistState();
      renderAll();
      if (!solutions.length) {
        const reasons = await runSchedulerWorker('diagnose', groups, schedulerState(), 0);
        renderDiagnostics(reasons);
      }
    } catch (error) {
      els.estimateText.textContent = `生成失败：${error.message || error}`;
    } finally {
      setSchedulingBusy(false);
    }
  }

  async function restoreCandidates() {
    if (!state.activeCandidate && !state.candidateCursor && !(state.favoriteCandidates || []).length) return;
    const generated = await runSchedulerWorker('generate', candidateGroups(), schedulerState(), 500);
    solutions = generated.results;
    if (solutions.length) {
      const available = new Set(solutions.map((solution) => solution.signature));
      state.favoriteCandidates = (state.favoriteCandidates || []).filter((signature) => available.has(signature));
    }
    const bySignature = solutions.findIndex((solution) => solution.signature === state.activeCandidate);
    const nextIndex = bySignature >= 0 ? bySignature : Math.min(state.candidateCursor || 0, Math.max(0, solutions.length - 1));
    activeSolution = solutions[nextIndex] || null;
    state.candidateCursor = activeSolution ? nextIndex : 0;
    state.activeCandidate = activeSolution?.signature || '';
    if (!activeSolution) state.candidatePreviewEnabled = false;
    persistState();
  }

  function moveCandidate(delta) {
    if (!solutions.length) return;
    applyCandidate((state.candidateCursor || 0) + delta);
  }

  function toggleFavoriteCandidate() {
    const solution = currentSolution();
    if (!solution) return;
    toggleFavoriteSignature(solution.signature);
  }

  function toggleFavoriteSignature(signature) {
    const set = new Set(state.favoriteCandidates || []);
    if (set.has(signature)) set.delete(signature);
    else set.add(signature);
    state.favoriteCandidates = [...set];
    persistState();
    renderResults();
  }

  function previewFavoriteCandidate(signature) {
    const index = solutions.findIndex((solution) => solution.signature === signature);
    if (index < 0) return;
    state.resultListMode = 'current';
    applyCandidate(index, true);
  }

  function removeFavoriteCandidate(signature) {
    state.favoriteCandidates = (state.favoriteCandidates || []).filter((item) => item !== signature);
    persistState();
    renderResults();
  }

  function renderFavoriteCandidates() {
    const list = solutions.filter((solution) => (state.favoriteCandidates || []).includes(solution.signature));
    if (!els.resultList) return;
    const favoriteCount = list.length;
    els.resultCount.textContent = `${favoriteCount} \u4e2a\u6536\u85cf\u65b9\u6848`;
    els.tableResultCount.textContent = `${favoriteCount} \u4e2a\u6536\u85cf\u65b9\u6848`;
    if (els.candidatePage) els.candidatePage.value = '';
    if (els.tableCandidatePage) els.tableCandidatePage.value = '';
    [
      els.candidatePrev,
      els.candidateNext,
      els.tableCandidatePrev,
      els.tableCandidateNext,
      els.candidatePage,
      els.tableCandidatePage,
      els.candidatePreview,
      els.tableCandidatePreview,
      els.candidateFavorite,
      els.candidateDismiss,
    ].forEach((control) => {
      if (control) control.disabled = true;
    });
    if (els.candidateReturn) els.candidateReturn.disabled = !state.candidatePreviewEnabled;
    if (els.tableCandidateReturn) els.tableCandidateReturn.disabled = !state.candidatePreviewEnabled;
    if (!list.length) {
      els.resultList.innerHTML = '<div class="empty-state">还没有收藏任何候选方案</div>';
      return;
    }
    els.resultList.innerHTML = list.map((solution, index) => `
      <article class="result-card ${solution.signature === state.activeCandidate ? 'active' : ''}">
        <h4>收藏方案 ${index + 1}</h4>
        <div class="meta">排序指标 ${solution.score.toFixed(1)}（越低越好） · ${formatCredit(solution.credits)} 学分 · 需退课 ${withdrawalCount(solution.items)}</div>
        <div class="result-stats">
          <span class="stat">早八 ${solution.metrics.earlyDays}</span>
          <span class="stat">午间 ${solution.metrics.lunchDays}</span>
          <span class="stat">晚课 ${solution.metrics.lateDays}</span>
          <span class="stat">全天无课 ${solution.metrics.freeDays}</span>
        </div>
        <div class="candidate-course-list">
          ${solution.items.map((item) => `<span>${escapeHtml(item.courseName)} / ${escapeHtml(item.sectionName)}</span>`).join('')}
        </div>
        ${renderSolutionDetails(solution)}
        <div class="result-actions">
          <button class="primary-btn small" type="button" data-favorite-preview="${escapeHtml(solution.signature)}">\u663e\u793a\u6b64\u65b9\u6848</button>
          <button class="ghost-btn small" type="button" data-favorite-remove="${escapeHtml(solution.signature)}">\u53d6\u6d88\u6536\u85cf</button>
        </div>
      </article>
    `).join('');
    els.resultList.querySelectorAll('[data-favorite-preview]').forEach((button) => {
      button.addEventListener('click', () => previewFavoriteCandidate(button.dataset.favoritePreview));
    });
    els.resultList.querySelectorAll('[data-favorite-remove]').forEach((button) => {
      button.addEventListener('click', () => removeFavoriteCandidate(button.dataset.favoriteRemove));
    });
  }

  function dismissCandidate() {
    const solution = currentSolution();
    if (!solution) return;
    const set = new Set(state.dismissedCandidates || []);
    set.add(solution.signature);
    state.dismissedCandidates = [...set];
    state.favoriteCandidates = (state.favoriteCandidates || []).filter((item) => item !== solution.signature);
    solutions = solutions.filter((item) => item.signature !== solution.signature);
    state.candidateCursor = Math.min(state.candidateCursor || 0, Math.max(0, solutions.length - 1));
    activeSolution = currentSolution();
    state.activeCandidate = activeSolution?.signature || '';
    if (!solutions.length) {
      state.candidateCursor = 0;
      state.candidatePreviewEnabled = false;
    }
    persistState();
    renderAll();
  }

  function exportTimetableScreenshot() {
    const target = els.timetable;
    if (!target) return;
    const width = Math.ceil(Math.max(target.scrollWidth, target.getBoundingClientRect().width));
    const height = Math.ceil(Math.max(target.scrollHeight, target.getBoundingClientRect().height));
    if (!width || !height) {
      window.alert('课表区域为空，暂时无法导出截图。');
      return;
    }
    const canvas = document.createElement('canvas');
    const scale = 2;
    canvas.width = width * scale;
    canvas.height = height * scale;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.scale(scale, scale);
    paintTimetableToCanvas(ctx, target);
    downloadCanvas(canvas, `hdu-timetable-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.png`);
  }

  function paintTimetableToCanvas(ctx, target) {
    const rootRect = target.getBoundingClientRect();
    for (const cell of target.children) {
      drawTimetableCell(ctx, cell, rootRect);
    }
  }

  function relativeRect(element, rootRect) {
    const rect = element.getBoundingClientRect();
    return {
      x: rect.left - rootRect.left,
      y: rect.top - rootRect.top,
      width: rect.width,
      height: rect.height,
    };
  }

  function drawBox(ctx, rect, style, radius = 0) {
    const background = style.backgroundColor && style.backgroundColor !== 'rgba(0, 0, 0, 0)' ? style.backgroundColor : '#ffffff';
    const border = style.borderTopColor || '#cfdae8';
    ctx.save();
    ctx.beginPath();
    if (ctx.roundRect && radius) ctx.roundRect(rect.x, rect.y, rect.width, rect.height, radius);
    else ctx.rect(rect.x, rect.y, rect.width, rect.height);
    ctx.fillStyle = background;
    ctx.fill();
    ctx.strokeStyle = border;
    ctx.lineWidth = 1;
    ctx.stroke();
    ctx.restore();
  }

  function fontFromStyle(style, fallbackSize = 12) {
    const weight = style.fontWeight || '400';
    const size = style.fontSize || `${fallbackSize}px`;
    const family = style.fontFamily || 'Arial, sans-serif';
    return `${weight} ${size} ${family}`;
  }

  function drawEllipsisText(ctx, text, x, y, maxWidth, style, fallbackSize = 12) {
    const value = String(text || '').trim();
    if (!value || maxWidth <= 0) return;
    ctx.save();
    ctx.font = fontFromStyle(style, fallbackSize);
    ctx.fillStyle = style.color || '#0b1f3a';
    ctx.textBaseline = 'top';
    let output = value;
    while (output.length > 1 && ctx.measureText(output).width > maxWidth) {
      output = `${output.slice(0, -2)}…`;
    }
    ctx.fillText(output, x, y);
    ctx.restore();
  }

  function drawPillText(ctx, element, rootRect) {
    if (!element) return;
    const rect = relativeRect(element, rootRect);
    const style = window.getComputedStyle(element);
    drawBox(ctx, rect, style, Math.min(rect.height / 2, 10));
    ctx.save();
    ctx.font = fontFromStyle(style, 10);
    ctx.fillStyle = style.color || '#1e3a8a';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(element.textContent.trim(), rect.x + rect.width / 2, rect.y + rect.height / 2);
    ctx.restore();
  }

  function drawTimetableCell(ctx, cell, rootRect) {
    const rect = relativeRect(cell, rootRect);
    const style = window.getComputedStyle(cell);
    drawBox(ctx, rect, style, 0);
    if (cell.classList.contains('day-cell')) {
      ctx.save();
      ctx.font = fontFromStyle(style, 18);
      ctx.fillStyle = style.color || '#0b1f3a';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      ctx.fillText(cell.textContent.trim(), rect.x + rect.width / 2, rect.y + 13);
      ctx.restore();
      return;
    }
    if (cell.classList.contains('time-cell') && !cell.classList.contains('header-spacer')) {
      const indexEl = cell.querySelector('.period-index');
      const timeEl = cell.querySelector('.period-time');
      drawEllipsisText(ctx, indexEl?.textContent, rect.x + 10, rect.y + 10, rect.width - 20, window.getComputedStyle(indexEl || cell), 16);
      const timeStyle = window.getComputedStyle(timeEl || cell);
      const lines = String(timeEl?.textContent || '').trim().match(/\d{2}:\d{2}/g) || [];
      lines.forEach((line, index) => drawEllipsisText(ctx, line, rect.x + 10, rect.y + 34 + index * 16, rect.width - 20, timeStyle, 13));
      return;
    }
    if (cell.classList.contains('slot')) {
      for (const card of cell.querySelectorAll('.slot-item')) {
        drawCourseCard(ctx, card, rootRect);
      }
    }
  }

  function drawCourseCard(ctx, card, rootRect) {
    const rect = relativeRect(card, rootRect);
    const style = window.getComputedStyle(card);
    drawBox(ctx, rect, style, 8);
    const paddingX = 9;
    const courseName = card.querySelector('.course-name');
    const sectionName = card.querySelector('.section-name');
    const detail = card.querySelector('.course-detail');
    drawEllipsisText(ctx, courseName?.textContent, rect.x + paddingX, rect.y + 8, rect.width - paddingX * 2, window.getComputedStyle(courseName || card), 13);
    drawEllipsisText(ctx, sectionName?.textContent, rect.x + paddingX, rect.y + 29, rect.width - paddingX * 2, window.getComputedStyle(sectionName || card), 12);
    drawEllipsisText(ctx, detail?.textContent, rect.x + paddingX, rect.y + 47, rect.width - paddingX * 2, window.getComputedStyle(detail || card), 10);
    drawPillText(ctx, card.querySelector('.week-badge'), rootRect);
    drawPillText(ctx, card.querySelector('.lock-badge'), rootRect);
  }

  function downloadCanvas(canvas, filename) {
    const dataURL = canvas.toDataURL('image/png');
    const [header, body] = dataURL.split(',');
    const mime = header.match(/data:(.*?);/)?.[1] || 'image/png';
    const binary = atob(body);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }
    const url = URL.createObjectURL(new Blob([bytes], { type: mime }));
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  function matchImportedCourse(imported) {
    const rawId = HDU.firstText(imported.jxb_id, imported.sectionId, imported.id);
    if (rawId) {
      const byId = courses.find((course) => course.id === rawId);
      if (byId) return byId;
    }
    const courseName = HDU.firstText(imported.kcmc, imported.courseName, imported.name);
    const sectionName = HDU.firstText(imported.jxbmc, imported.sectionName);
    return courses.find((course) => {
      if (sectionName && course.sectionName === sectionName) return true;
      return courseName && course.courseName === courseName && (!sectionName || course.sectionName.includes(sectionName));
    }) || null;
  }

  async function importBaseSchedule(file) {
    if (!file) return;
    const text = await file.text();
    const raw = JSON.parse(text);
    const list = Array.isArray(raw?.items) ? raw.items : Array.isArray(raw) ? raw : [];
    const matched = list.map(matchImportedCourse).filter(Boolean);
    state.baseCourseIds = [...new Set(matched.map((item) => item.id))];
    state.baseScheduleName = file.name.replace(/\.json$/i, '');
    addManySections(matched, 'base');
  }

  function clearBaseSchedule() {
    const map = getSelectionMap();
    const baseCourseIds = new Set(state.baseCourseIds || []);
    for (const [groupId, entry] of Object.entries(map)) {
      const removedBaseIds = new Set((entry.items || [])
        .filter((item) => item.source === 'base')
        .map((item) => item.id));
      entry.items = (entry.items || []).filter((item) => item.source !== 'base');
      if (entry.lockedItemId && (
        baseCourseIds.has(entry.lockedItemId)
        || removedBaseIds.has(entry.lockedItemId)
        || !(entry.items || []).some((item) => item.id === entry.lockedItemId)
      )) {
        entry.lockedItemId = '';
      }
      if (!entry.items.length) delete map[groupId];
    }
    state.selectedGroups = map;
    state.baseCourseIds = [];
    state.baseScheduleName = '';
    state.personalScheduleAutoImported = false;
    clearCandidateState();
    persistState();
    rebuildSelection();
    renderAll();
  }

  async function exportCurrentTimetable() {
    const items = activeItems();
    if (!items.length) {
      els.estimateText.textContent = '当前没有可导出的课程。';
      return;
    }
    const kind = state.candidatePreviewEnabled ? 'target' : 'current';
    const payload = {
      schemaVersion: HDU.COURSE_SCHEMA_VERSION,
      exportedAt: new Date().toISOString(),
      source: kind === 'target' ? 'candidate' : 'current',
      items: items.map((item) => ({
        ...(item.raw || {}),
        schemaVersion: HDU.COURSE_SCHEMA_VERSION,
        id: item.id,
        sectionId: item.id,
        jxb_id: item.raw?.jxb_id || item.id,
        groupId: item.groupId,
        courseCode: item.groupId,
        displayCode: item.displayCode || item.sectionName,
        jxbmc: item.raw?.jxbmc || item.sectionName,
        sectionName: item.sectionName,
        kcmc: item.raw?.kcmc || item.courseName,
        courseName: item.courseName,
        jzgxx: item.raw?.jzgxx || item.teacher,
        sksj: item.raw?.sksj || item.timeText,
        jxdd: item.raw?.jxdd || item.location,
        xf: item.raw?.xf || item.credits,
      })),
    };
    els.exportCurrent.disabled = true;
    try {
      const result = await HDU.fetchJSON('/api/export/timetable', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        body: JSON.stringify({ kind, payload }),
      });
      const label = kind === 'target' ? '目标课表' : '当前课表';
      els.estimateText.textContent = `${label}已保存到项目目录：${result.path}（${result.count} 个教学班）`;
    } catch (error) {
      els.estimateText.textContent = `保存课表失败：${error.message || error}`;
    } finally {
      els.exportCurrent.disabled = false;
    }
  }

  function wireEvents() {
    els.searchInput.addEventListener('input', () => {
      state.query = els.searchInput.value;
      persistState();
      renderCourseList();
    });
    els.lockedSearch.addEventListener('input', renderLockedSearchResults);
    [
      els.minCredit,
      els.maxCredit,
      els.maxEarly,
      els.maxLunch,
      els.maxLate,
      els.minFreeDays,
      els.blockedTeachers,
      els.pairRules,
      els.sameTeacherRules,
    ].forEach((element) => element.addEventListener('input', () => {
      persistState();
      clearCandidateState();
      renderSummary();
      renderTimetable();
      renderResults();
    }));
    els.schemeAddRule.addEventListener('click', addCustomSchemeRule);
    els.clearSelected.addEventListener('click', () => {
      state.selectedGroups = {};
      state.baseCourseIds = [];
      state.baseScheduleName = '';
      state.personalScheduleAutoImported = false;
      clearCandidateState();
      persistState();
      rebuildSelection();
      renderAll();
    });
    els.generate.addEventListener('click', generateCandidates);
    els.estimate.addEventListener('click', estimateCandidates);
    els.reloadCourse.addEventListener('click', loadCourses);
    els.candidatePrev.addEventListener('click', () => moveCandidate(-1));
    els.candidateNext.addEventListener('click', () => moveCandidate(1));
    els.candidatePage.addEventListener('change', () => setCandidatePage(els.candidatePage.value));
    els.candidatePreview.addEventListener('click', previewCurrentCandidate);
    els.candidateReturn.addEventListener('click', returnToOriginalTimetable);
    els.tableCandidatePrev.addEventListener('click', () => moveCandidate(-1));
    els.tableCandidateNext.addEventListener('click', () => moveCandidate(1));
    els.tableCandidatePage.addEventListener('change', () => setCandidatePage(els.tableCandidatePage.value));
    els.tableCandidatePreview.addEventListener('click', previewCurrentCandidate);
    els.tableCandidateReturn.addEventListener('click', returnToOriginalTimetable);
    els.candidateFavorite.addEventListener('click', toggleFavoriteCandidate);
    els.candidateDismiss.addEventListener('click', dismissCandidate);
    els.candidateFavoritesView.addEventListener('click', showFavoriteList);
    els.exportCurrent.addEventListener('click', exportCurrentTimetable);
    const screenshotButton = document.createElement('button');
    screenshotButton.className = 'ghost-btn small';
    screenshotButton.type = 'button';
    screenshotButton.textContent = '导出截图';
    screenshotButton.addEventListener('click', exportTimetableScreenshot);
    els.exportCurrent.insertAdjacentElement('afterend', screenshotButton);
    els.baseFile.addEventListener('change', () => {
      importBaseSchedule(els.baseFile.files[0]).catch((error) => {
        els.baseSummary.textContent = `导入失败：${error.message || error}`;
      });
    });
    els.clearBase.addEventListener('click', clearBaseSchedule);
    document.querySelectorAll('[data-modal-close]').forEach((button) => {
      button.addEventListener('click', closeCourseModal);
    });
    els.modalSelectToggle.addEventListener('click', () => {
      const course = selectedModalCourse();
      if (!course) return;
      if (isSelected(course)) removeOption(course.groupId, course.id);
      else addSection(course);
      const fresh = selectedModalCourse();
      if (fresh) refreshModal(fresh);
      renderTimetable();
      renderSelectedList();
    });
    els.modalLockToggle.addEventListener('click', () => {
      const course = selectedModalCourse();
      if (!course) return;
      if (!isSelected(course)) addSection(course);
      toggleLockOption(course.groupId, course.id);
      const fresh = selectedModalCourse();
      if (fresh) refreshModal(fresh);
      renderTimetable();
      renderSelectedList();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') closeCourseModal();
      if (event.key === 'ArrowLeft' && event.altKey) moveCandidate(-1);
      if (event.key === 'ArrowRight' && event.altKey) moveCandidate(1);
    });
  }

  async function loadCourses() {
    const data = await HDU.fetchJSON(HDU.COURSE_API);
    courses = HDU.normalizeCourseData(data);
    creditDataAvailable = HDU.hasCreditData(courses);
    syncCreditAvailability();
    await autoImportPersonalSchedule();
    applyLegacyCourseLockMigration();
    rebuildSelection();
    await restoreCandidates();
    els.subtitle.textContent = `当前加载 ${courses.length} 个教学班，${HDU.groupCourses(courses).length} 个课组`;
    els.estimateText.textContent = state.candidateEstimate || '调整约束后，可先估算候选数量。';
    renderAll();
  }

  async function bootstrap() {
    syncForm();
    wireEvents();
    try {
      await loadCourses();
    } catch {
      els.subtitle.textContent = '未找到 course.json，请先回到导入页。';
      els.courseList.innerHTML = '<div class="empty-state">course.json 缺失或不可读</div>';
      els.resultList.innerHTML = '<div class="empty-state">导入课程后再生成候选方案。</div>';
      renderSelectedList();
      renderTimetable();
      renderBaseSummary();
    }
  }

  bootstrap();
})();
