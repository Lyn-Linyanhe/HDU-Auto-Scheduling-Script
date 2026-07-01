(() => {
  const state = HDU.loadState();
  let courses = [];
  let selectedGroups = [];
  let solutions = [];
  let activeSolution = null;
  let selectedCourseId = '';
  let modalCourseId = '';

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
    maxEarly: document.getElementById('max-early'),
    maxLunch: document.getElementById('max-lunch'),
    maxLate: document.getElementById('max-late'),
    minFreeDays: document.getElementById('min-free-days'),
    blockedTeachers: document.getElementById('blocked-teachers'),
    preferredTeachers: document.getElementById('preferred-teachers'),
    requiredCourses: document.getElementById('required-courses'),
    requiredSelectedPicks: document.getElementById('required-selected-picks'),
    requiredQuickPicks: document.getElementById('required-quick-picks'),
    requiredSearch: document.getElementById('required-search'),
    requiredSearchResults: document.getElementById('required-search-results'),
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
    const number = Number(value);
    return Number.isFinite(number) ? number.toFixed(2) : '0.00';
  }

  function syncForm() {
    els.searchInput.value = state.query || '';
    els.minCredit.value = state.minCredit ?? 0;
    els.maxCredit.value = state.maxCredit ?? 30;
    els.maxEarly.value = state.maxEarly ?? 5;
    els.maxLunch.value = state.maxLunch ?? 5;
    els.maxLate.value = state.maxLate ?? 5;
    els.minFreeDays.value = state.minFreeDays ?? 0;
    els.blockedTeachers.value = state.blockedTeachers || '';
    els.preferredTeachers.value = state.preferredTeachers || '';
    els.requiredCourses.value = state.requiredCourses || '';
    els.pairRules.value = state.pairRules || '';
    els.sameTeacherRules.value = state.sameTeacherRules || '';
  }

  function persistState() {
    state.query = els.searchInput.value;
    state.minCredit = Number(els.minCredit.value || 0);
    state.maxCredit = Number(els.maxCredit.value || 30);
    state.maxEarly = clampDayLimit(els.maxEarly.value);
    state.maxLunch = clampDayLimit(els.maxLunch.value);
    state.maxLate = clampDayLimit(els.maxLate.value);
    els.maxEarly.value = state.maxEarly;
    els.maxLunch.value = state.maxLunch;
    els.maxLate.value = state.maxLate;
    state.minFreeDays = clampDayLimit(els.minFreeDays.value);
    els.minFreeDays.value = state.minFreeDays;
    state.blockedTeachers = els.blockedTeachers.value;
    state.preferredTeachers = els.preferredTeachers.value;
    state.requiredCourses = els.requiredCourses.value;
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

  function constraintToken(course) {
    return HDU.baseCourseCode(course.displayCode || course.sectionName || course.groupId || course.courseName);
  }

  function requiredTokens() {
    return HDU.parseList(els.requiredCourses.value);
  }

  function requiredDisplayLabel(token) {
    const lower = String(token || '').toLowerCase();
    const matches = courses.filter((item) => HDU.courseSearchText(item).includes(lower));
    if (!matches.length) return { name: token, code: token, count: 0 };
    const first = matches[0];
    const code = HDU.baseCourseCode(first.displayCode || first.sectionName || first.groupId || token);
    return {
      name: first.courseName || token,
      code,
      count: new Set(matches.map((item) => item.id)).size,
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

  function syncRequiredText(tokens) {
    const unique = [...new Set(tokens.map((item) => String(item || '').trim()).filter(Boolean))];
    els.requiredCourses.value = unique.join('\n');
    persistState();
    clearCandidateState();
    renderAll();
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

  function appendRequiredCourse(course) {
    appendTextareaLine(els.requiredCourses, constraintToken(course));
  }

  function removeRequiredToken(token) {
    syncRequiredText(requiredTokens().filter((item) => item !== token));
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
    if (!Number.isFinite(page) || !solutions.length) return;
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
    renderRequiredQuickPicks(items);
    renderRequiredSelectedPicks();
    renderRequiredSearchResults();
    renderSchemeCurrentRules();
    renderLinkedCourseSuggestions();
  }

  function renderRequiredQuickPicks(items) {
    if (!els.requiredQuickPicks) return;
    if (!items.length) {
      els.requiredQuickPicks.innerHTML = '<span class="quick-empty">先在左侧加入课程</span>';
      return;
    }
    const seen = new Set();
    els.requiredQuickPicks.innerHTML = items.map((item) => {
      const token = constraintToken(item);
      if (!token || seen.has(token)) return '';
      seen.add(token);
      return `
        <button class="ghost-btn small required-add-btn" type="button" data-quick-required="${escapeHtml(item.id)}">
          <span>${escapeHtml(item.courseName)}</span>
          <small>${escapeHtml(token)}</small>
        </button>
      `;
    }).join('');
    els.requiredQuickPicks.querySelectorAll('[data-quick-required]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.quickRequired);
        if (course) appendRequiredCourse(course);
      });
    });
  }

  function renderRequiredSelectedPicks() {
    if (!els.requiredSelectedPicks) return;
    const tokens = requiredTokens();
    if (!tokens.length) {
      els.requiredSelectedPicks.innerHTML = '<span class="quick-empty">还没有必选课程</span>';
      return;
    }
    els.requiredSelectedPicks.innerHTML = tokens.map((token) => {
      const label = requiredDisplayLabel(token);
      const countText = label.count > 1 ? `${label.count} 个教学班` : '1 个教学班';
      return `
        <div class="required-picked-item">
          <div>
            <strong>${escapeHtml(label.name)}</strong>
            <span>${escapeHtml(label.code)} · ${escapeHtml(countText)}</span>
          </div>
          <button class="danger-btn small" type="button" data-remove-required="${escapeHtml(token)}">删除</button>
        </div>
      `;
    }).join('');
    els.requiredSelectedPicks.querySelectorAll('[data-remove-required]').forEach((button) => {
      button.addEventListener('click', () => removeRequiredToken(button.dataset.removeRequired));
    });
  }

  function renderRequiredSearchResults() {
    if (!els.requiredSearchResults || !els.requiredSearch) return;
    const keyword = els.requiredSearch.value.trim().toLowerCase();
    if (!keyword) {
      els.requiredSearchResults.innerHTML = '<span class="quick-empty">输入关键词后搜索课程</span>';
      return;
    }
    const groups = HDU.groupCourses(courses.filter((course) => courseNameCodeText(course).includes(keyword))).slice(0, 20);
    if (!groups.length) {
      els.requiredSearchResults.innerHTML = '<span class="quick-empty">没有匹配课程</span>';
      return;
    }
    const existing = new Set(requiredTokens());
    els.requiredSearchResults.innerHTML = groups.map((group) => {
      const sample = group.items[0];
      const token = constraintToken(sample);
      const added = existing.has(token);
      return `
        <div class="required-search-item">
          <div>
            <strong>${escapeHtml(group.name)}</strong>
            <span>${escapeHtml(token)} · ${group.items.length} 个教学班</span>
          </div>
          <button class="${added ? 'ghost-btn' : 'primary-btn'} small" type="button" data-search-required="${escapeHtml(sample.id)}" ${added ? 'disabled' : ''}>
            ${added ? '已加入' : '加入'}
          </button>
        </div>
      `;
    }).join('');
    els.requiredSearchResults.querySelectorAll('[data-search-required]').forEach((button) => {
      button.addEventListener('click', () => {
        const course = courses.find((item) => item.id === button.dataset.searchRequired);
        if (course) appendRequiredCourse(course);
      });
    });
  }

  function normalizedCourseTitle(name) {
    return String(name || '').replace(/[（(]?[一二三四五六七八九十0-9]+[)）]?$/g, '').trim();
  }

  function linkBaseName(name) {
    return normalizedCourseTitle(name)
      .replace(/(课程实践|实验|课程设计|综合实践|实践|实训|设计)$/g, '')
      .trim();
  }

  function linkedCoursePairs(limit = 500) {
    const groups = HDU.groupCourses(courses);
    const byName = new Map();
    for (const group of groups) {
      const key = normalizedCourseTitle(group.name);
      if (!key) continue;
      if (!byName.has(key)) byName.set(key, []);
      byName.get(key).push(group);
    }
    const pairs = [];
    const seen = new Set();
    for (const group of groups) {
      const name = normalizedCourseTitle(group.name);
      if (!/(课程实践|实验|课程设计|综合实践|实践|实训|设计)$/.test(name)) continue;
      const base = linkBaseName(name);
      if (!base || base === name) continue;
      const bases = byName.get(base) || [];
      for (const main of bases) {
        if (main.id === group.id) continue;
        const key = [main.id, group.id].sort().join('|');
        if (seen.has(key)) continue;
        seen.add(key);
        pairs.push([main, group]);
        if (pairs.length >= limit) return pairs;
      }
    }
    return pairs;
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
      els.resultList.innerHTML = '<div class="empty-state">点击“生成候选课表”开始枚举。</div>';
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
        <div class="meta">代价分 ${solution.score.toFixed(1)} · ${formatCredit(solution.credits)} 学分 · 需退课 ${withdrawalCount(solution.items)}</div>
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
      </article>
    `;
  }

  function showFavoriteList() {
    state.resultListMode = state.resultListMode === 'favorites' ? 'current' : 'favorites';
    persistState();
    renderResults();
  }

  function buildSolutions(limit = 500) {
    const generated = HDU.generateSolutions(candidateGroups(), state, limit);
    const rows = generated.results.map((solution) => ({
      ...solution,
      signature: signatureForItems(solution.items),
    })).filter((solution) => !(state.dismissedCandidates || []).includes(solution.signature));
    return { ...generated, results: rows };
  }

  function candidateGroups() {
    const map = new Map();
    for (const group of selectedGroups) {
      map.set(group.id, {
        id: group.id,
        name: group.name,
        items: [...group.items],
        lockedItemId: group.lockedItemId || '',
        optional: !group.lockedItemId,
      });
    }
    for (const token of requiredTokens()) {
      const lower = token.toLowerCase();
      const matched = courses.filter((course) => HDU.courseSearchText(course).includes(lower));
      for (const group of HDU.groupCourses(matched)) {
        if (!map.has(group.id)) {
          map.set(group.id, { id: group.id, name: group.name, items: group.items, lockedItemId: '', optional: false });
        } else {
          map.get(group.id).optional = false;
        }
      }
    }
    return [...map.values()].filter((group) => group.items.length);
  }

  function renderAll() {
    renderSummary();
    renderTimetable();
    renderCourseList();
    renderSelectedList();
    renderConstraintQuickPicks();
    renderBaseSummary();
    renderResults();
  }

  function estimateCandidates() {
    persistState();
    const estimate = HDU.estimateSolutions(candidateGroups(), state);
    const text = estimate.capped ? `候选数量超过 ${estimate.limit} 个` : `预计 ${estimate.count} 个候选课表`;
    state.candidateEstimate = text;
    els.estimateText.textContent = text;
    persistState();
  }

  function generateCandidates() {
    persistState();
    const estimate = HDU.estimateSolutions(candidateGroups(), state, 501);
    if (estimate.capped || estimate.count > 500) {
      const message = '当前候选课表过多，建议添加更多约束条件。是否仍然继续生成前 500 个候选方案？';
      if (!window.confirm(message)) {
        els.estimateText.textContent = '已取消生成。建议添加更多约束条件后再试。';
        return;
      }
    }
    const generated = buildSolutions(500);
    solutions = generated.results;
    state.candidateCursor = 0;
    activeSolution = solutions[0] || null;
    state.activeCandidate = activeSolution ? activeSolution.signature : '';
    state.candidatePreviewEnabled = false;
    els.estimateText.textContent = generated.capped
      ? `已生成前 ${generated.limit} 个较优方案，候选可能更多。`
      : `已生成 ${solutions.length} 个候选方案。`;
    persistState();
    renderAll();
  }

  function restoreCandidates() {
    if (!state.activeCandidate && !state.candidateCursor) return;
    const generated = buildSolutions(500);
    solutions = generated.results;
    const bySignature = solutions.findIndex((solution) => solution.signature === state.activeCandidate);
    const nextIndex = bySignature >= 0 ? bySignature : Math.min(state.candidateCursor || 0, Math.max(0, solutions.length - 1));
    activeSolution = solutions[nextIndex] || null;
    state.candidateCursor = activeSolution ? nextIndex : 0;
    state.activeCandidate = activeSolution?.signature || '';
  }

  function moveCandidate(delta) {
    if (!solutions.length) return;
    applyCandidate((state.candidateCursor || 0) + delta);
  }

  function toggleFavoriteCandidate() {
    const solution = currentSolution();
    if (!solution) return;
    const set = new Set(state.favoriteCandidates || []);
    if (set.has(solution.signature)) set.delete(solution.signature);
    else set.add(solution.signature);
    state.favoriteCandidates = [...set];
    persistState();
    renderResults();
  }

  function renderFavoriteCandidates() {
    const list = solutions.filter((solution) => (state.favoriteCandidates || []).includes(solution.signature));
    if (!els.resultList) return;
    if (!list.length) {
      els.resultList.innerHTML = '<div class="empty-state">还没有收藏任何候选方案</div>';
      return;
    }
    els.resultList.innerHTML = list.map((solution, index) => `
      <article class="result-card ${solution.signature === state.activeCandidate ? 'active' : ''}">
        <h4>收藏方案 ${index + 1}</h4>
        <div class="meta">代价分 ${solution.score.toFixed(1)} · ${formatCredit(solution.credits)} 学分 · 需退课 ${withdrawalCount(solution.items)}</div>
        <div class="result-stats">
          <span class="stat">早八 ${solution.metrics.earlyDays}</span>
          <span class="stat">午间 ${solution.metrics.lunchDays}</span>
          <span class="stat">晚课 ${solution.metrics.lateDays}</span>
          <span class="stat">全天无课 ${solution.metrics.freeDays}</span>
        </div>
        <div class="candidate-course-list">
          ${solution.items.map((item) => `<span>${escapeHtml(item.courseName)} / ${escapeHtml(item.sectionName)}</span>`).join('')}
        </div>
      </article>
    `).join('');
  }

  function dismissCandidate() {
    const solution = currentSolution();
    if (!solution) return;
    const set = new Set(state.dismissedCandidates || []);
    set.add(solution.signature);
    state.dismissedCandidates = [...set];
    solutions = solutions.filter((item) => item.signature !== solution.signature);
    state.candidateCursor = Math.min(state.candidateCursor || 0, Math.max(0, solutions.length - 1));
    activeSolution = currentSolution();
    state.activeCandidate = activeSolution?.signature || '';
    persistState();
    renderAll();
  }

  function exportTimetableScreenshot() {
    const target = els.timetable;
    if (!target) return;
    const canvas = document.createElement('canvas');
    const scale = 2;
    canvas.width = Math.ceil(target.scrollWidth * scale);
    canvas.height = Math.ceil(target.scrollHeight * scale);
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.scale(scale, scale);
    const data = new XMLSerializer().serializeToString(target);
    const svg = `
      <svg xmlns="http://www.w3.org/2000/svg" width="${target.scrollWidth}" height="${target.scrollHeight}">
        <foreignObject width="100%" height="100%">${data}</foreignObject>
      </svg>
    `;
    const img = new Image();
    const blob = new Blob([svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    img.onload = () => {
      ctx.drawImage(img, 0, 0, target.scrollWidth, target.scrollHeight);
      URL.revokeObjectURL(url);
      const link = document.createElement('a');
      link.href = canvas.toDataURL('image/png');
      link.download = `hdu-timetable-${new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-')}.png`;
      document.body.appendChild(link);
      link.click();
      link.remove();
    };
    img.src = url;
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
    for (const [groupId, entry] of Object.entries(map)) {
      entry.items = (entry.items || []).filter((item) => item.source !== 'base');
      if (entry.lockedItemId && !(entry.items || []).some((item) => item.id === entry.lockedItemId)) {
        entry.lockedItemId = '';
      }
      if (!entry.items.length) delete map[groupId];
    }
    state.selectedGroups = map;
    state.baseCourseIds = [];
    state.baseScheduleName = '';
    clearCandidateState();
    persistState();
    rebuildSelection();
    renderAll();
  }

  function exportCurrentTimetable() {
    const items = activeItems();
    const payload = {
      exportedAt: new Date().toISOString(),
      source: state.candidatePreviewEnabled ? 'candidate' : 'current',
      items: items.map((item) => ({
        ...(item.raw || {}),
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
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    const stamp = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-');
    link.href = url;
    link.download = `hdu-current-timetable-${stamp}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  }

  function wireEvents() {
    els.searchInput.addEventListener('input', () => {
      state.query = els.searchInput.value;
      persistState();
      renderCourseList();
    });
    els.requiredSearch.addEventListener('input', renderRequiredSearchResults);
    [
      els.minCredit,
      els.maxCredit,
      els.maxEarly,
      els.maxLunch,
      els.maxLate,
      els.minFreeDays,
      els.blockedTeachers,
      els.preferredTeachers,
      els.requiredCourses,
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
    rebuildSelection();
    restoreCandidates();
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
