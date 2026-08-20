const state = {
  activeStage: 1,
  status: null,
  settings: null,
  current: [],
  currentLoadError: '',
  liveSchedule: null,
  liveItems: [],
  liveSnapshotAvailable: false,
  courseOptions: [],
  courseOptionsWarnings: [],
  courseCurrentRound: null,
  courseCapacity: [],
  courseCapacitySource: '',
  courseCapacityObservedAt: '',
  courseCapacitySourceUpdatedAt: '',
  courseCapacityStale: false,
  courseInspectorCode: '',
  courseSchedule: null,
  classOptions: [],
  classOptionsWarnings: [],
  classInspectorName: '',
  classSchedule: null,
  lockedCodes: new Set(),
  targetPayload: null,
  targetPath: '',
  targetUpdatedAt: '',
  targetWarnings: [],
  targetFileName: '',
  targetManualOverride: false,
  autoRefresh: true,
  refreshIntervalSeconds: 60,
  lastRefreshAt: '',
  nextRefreshAt: '',
  refreshFailureAt: '',
  liveRefreshFailureStreak: 0,
  refreshError: '',
  liveImportFailureAt: '',
  liveImportError: '',
  liveRefreshInFlight: false,
  liveRefreshPromise: null,
  liveRefreshTimer: null,
  plan: null,
  generatedConfig: null,
  generatedConfigWritten: false,
  configPreview: null,
  readiness: null,
  dryRun: null,
  authorization: null,
  executionPackage: null,
  executionLog: null,
  lastExecutionRefreshKey: '',
  fallbackRecommendations: null,
  inlineExecution: null,
  inlineExecutionTimer: null,
  inlineExecutionPolling: false,
};

const els = {
  statusMessage: document.getElementById('status-message'),
  stagePrimary: document.getElementById('stage-primary'),
  stageTabs: [...document.querySelectorAll('[data-stage]')],
  stagePanels: [...document.querySelectorAll('[data-stage-panel]')],
  settingsToggle: document.getElementById('settings-toggle'),
  settingsDrawer: document.getElementById('settings-drawer'),
  settingsContent: document.querySelector('#settings-drawer .drawer-content'),
  settingsClose: document.getElementById('settings-close'),
  statusGrid: document.getElementById('status-grid'),
  term: document.getElementById('term'),
  settingsPath: document.getElementById('settings-path'),
  schedulerDir: document.getElementById('scheduler-dir'),
  killCourseDir: document.getElementById('killcourse-dir'),
  saveSettings: document.getElementById('save-settings'),
  clearSettings: document.getElementById('clear-settings'),
  currentCount: document.getElementById('current-count'),
  currentList: document.getElementById('current-list'),
  liveFile: document.getElementById('live-file'),
  liveSyncBadge: document.getElementById('live-sync-badge'),
  liveSyncSummary: document.getElementById('live-sync-summary'),
  liveSyncDiff: document.getElementById('live-sync-diff'),
  refreshLive: document.getElementById('refresh-live'),
  autoRefresh: document.getElementById('auto-refresh'),
  refreshInterval: document.getElementById('refresh-interval'),
  liveSource: document.getElementById('live-source'),
  lastRefreshAt: document.getElementById('last-refresh-at'),
  nextRefreshAt: document.getElementById('next-refresh-at'),
  liveRefreshError: document.getElementById('live-refresh-error'),
  courseIntelSummary: document.getElementById('course-intel-summary'),
  courseIntelBadge: document.getElementById('course-intel-badge'),
  courseOptionFilter: document.getElementById('course-option-filter'),
  courseOptionSelect: document.getElementById('course-option-select'),
  courseScheduleQuery: document.getElementById('course-schedule-query'),
  courseIntelDetail: document.getElementById('course-intel-detail'),
  courseCapacitySummary: document.getElementById('course-capacity-summary'),
  classScheduleResult: document.getElementById('class-schedule-result'),
  classOptionSelect: document.getElementById('class-option-select'),
  classScheduleQuery: document.getElementById('class-schedule-query'),
  adminClassScheduleResult: document.getElementById('admin-class-schedule-result'),
  refresh: document.getElementById('refresh'),
  targetFile: document.getElementById('target-file'),
  targetCount: document.getElementById('target-count'),
  targetPreview: document.getElementById('target-preview'),
  targetSource: document.getElementById('target-source'),
  targetUpdatedAt: document.getElementById('target-updated-at'),
  targetWarning: document.getElementById('target-warning'),
  writeAction: document.getElementById('write-action'),
  writeConfig: document.getElementById('write-config'),
  planSummary: document.getElementById('plan-summary'),
  planState: document.getElementById('plan-state'),
  keepSummary: document.getElementById('keep-summary'),
  selectList: document.getElementById('select-list'),
  dropList: document.getElementById('drop-list'),
  keepList: document.getElementById('keep-list'),
  fallbackList: document.getElementById('fallback-list'),
  riskList: document.getElementById('risk-list'),
  validationList: document.getElementById('validation-list'),
  explanationList: document.getElementById('explanation-list'),
  readinessSummary: document.getElementById('readiness-summary'),
  readinessList: document.getElementById('readiness-list'),
  dryRun: document.getElementById('dry-run'),
  dryRunBadge: document.getElementById('dry-run-badge'),
  dryRunSummary: document.getElementById('dry-run-summary'),
  dryRunCommand: document.getElementById('dry-run-command'),
  dryRunEvents: document.getElementById('dry-run-events'),
  confirmHint: document.getElementById('confirm-hint'),
  confirmPhrase: document.getElementById('confirm-phrase'),
  authorizeExecution: document.getElementById('authorize-execution'),
  buildPackage: document.getElementById('build-package'),
  authorizationResult: document.getElementById('authorization-result'),
  packageResult: document.getElementById('package-result'),
  writeExecutionLog: document.getElementById('write-execution-log'),
  parseLog: document.getElementById('parse-log'),
  executionLogSummary: document.getElementById('execution-log-summary'),
  executionLogList: document.getElementById('execution-log-list'),
  writeFallbackRecommendations: document.getElementById('write-fallback-recommendations'),
  buildFallbackRecommendations: document.getElementById('build-fallback-recommendations'),
  fallbackRecommendationSummary: document.getElementById('fallback-recommendation-summary'),
  fallbackRecommendationList: document.getElementById('fallback-recommendation-list'),
  waitEnabled: document.getElementById('wait-enabled'),
  inlineExecStart: document.getElementById('inline-exec-start'),
  inlineExecStop: document.getElementById('inline-exec-stop'),
  inlineExecBadge: document.getElementById('inline-exec-badge'),
  inlineExecState: document.getElementById('inline-exec-state'),
  inlineExecList: document.getElementById('inline-exec-list'),
  configSummary: document.getElementById('config-summary'),
  configPreserveList: document.getElementById('config-preserve-list'),
  configActionList: document.getElementById('config-action-list'),
  outputJson: document.getElementById('output-json'),
  downloadPlan: document.getElementById('download-plan'),
  downloadConfig: document.getElementById('download-config'),
};

let settingsReturnFocus = null;
const focusableSelector = 'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])';

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

async function fetchJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const text = await response.text();
  if (!response.ok) {
    let payload = null;
    try { payload = text ? JSON.parse(text) : null; } catch {}
    const error = new Error(payload?.error || payload?.message || text || `HTTP ${response.status}`);
    error.payload = payload;
    error.status = response.status;
    throw error;
  }
  return text ? JSON.parse(text) : {};
}

const DEFAULT_REFRESH_INTERVAL_SECONDS = 60;
const MIN_REFRESH_INTERVAL_SECONDS = 10;
const MAX_REFRESH_INTERVAL_SECONDS = 7200;

function normalizeRefreshInterval(value) {
  const parsed = Number.parseInt(String(value), 10);
  if (!Number.isFinite(parsed)) return DEFAULT_REFRESH_INTERVAL_SECONDS;
  return Math.min(MAX_REFRESH_INTERVAL_SECONDS, Math.max(MIN_REFRESH_INTERVAL_SECONDS, parsed));
}

function refreshIntervalFromSettings(settings) {
  const seconds = Number.parseInt(String(settings?.refreshIntervalSeconds ?? ''), 10);
  if (Number.isFinite(seconds) && seconds !== 0) return normalizeRefreshInterval(seconds);
  const minutes = Number.parseInt(String(settings?.refreshIntervalMinutes ?? ''), 10);
  if (Number.isFinite(minutes) && minutes !== 0) return normalizeRefreshInterval(minutes * 60);
  return DEFAULT_REFRESH_INTERVAL_SECONDS;
}

function executionSuccessRefreshKey(log) {
  return JSON.stringify((log?.items || [])
    .filter((item) => item?.status === 'success' && (item.action === 'select' || item.action === 'drop'))
    .map((item) => [item.courseCode, item.action, item.startedAt, item.finishedAt, item.message])
    .sort());
}

function formatTimestamp(value) {
  if (!value) return '未记录';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

function sourceText(source) {
  if (source === 'main-exporter') return '主站教务会话';
  if (source === 'file-bridge') return '本地课表文件';
  return source || '尚未刷新';
}

function hasLiveSnapshot() {
  return state.liveSnapshotAvailable === true;
}

function courseItemSignature(items) {
  return JSON.stringify((Array.isArray(items) ? items : [])
    .map((item) => ({
      displayCode: String(item?.displayCode || item?.jxbmc || item?.sectionName || item?.Jxbmc || '').trim(),
      groupId: String(item?.groupId || item?.courseCode || item?.rawCourseCode || '').trim(),
      courseName: String(item?.courseName || item?.kcmc || item?.name || '').trim(),
      sectionName: String(item?.sectionName || item?.jxbmc || item?.Jxbmc || '').trim(),
      teacher: String(item?.teacher || item?.jzgxx || '').trim(),
      timeText: String(item?.timeText || item?.sksj || item?.schedule || '').trim(),
      location: String(item?.location || item?.jxdd || '').trim(),
      className: String(item?.className || item?.jxbzc || '').trim(),
      credits: Number(item?.credits ?? item?.xf ?? 0) || 0,
    }))
    .filter((item) => item.displayCode)
    .sort((left, right) => left.displayCode.localeCompare(right.displayCode)));
}

function liveScheduleSignature(items, sync) {
  return String(sync?.liveHash || courseItemSignature(items));
}

function latestRefreshTimestamp() {
  const candidates = [state.lastRefreshAt, hasLiveSnapshot() ? state.liveSchedule?.syncedAt : '']
    .map((value) => ({ value, time: Date.parse(value || '') }))
    .filter((item) => Number.isFinite(item.time));
  if (!candidates.length) return '';
  return candidates.sort((left, right) => right.time - left.time)[0].value;
}

function clearLiveRefreshTimer() {
  if (state.liveRefreshTimer) clearTimeout(state.liveRefreshTimer);
  state.liveRefreshTimer = null;
}

function applyRefreshSettings(settings) {
  if (!settings) return;
  state.autoRefresh = settings.autoRefresh !== false;
  state.refreshIntervalSeconds = refreshIntervalFromSettings(settings);
}

function resetPlanState() {
  state.plan = null;
  state.generatedConfig = null;
  state.generatedConfigWritten = false;
  state.configPreview = null;
  state.readiness = null;
  state.dryRun = null;
  state.authorization = null;
  state.executionPackage = null;
  state.executionLog = null;
  state.lastExecutionRefreshKey = '';
  state.fallbackRecommendations = null;
  stopInlinePolling();
  state.inlineExecution = null;
  state.activeStage = 1;
}

function statCard(label, value, ok) {
  return `
    <div class="stat-card ${ok ? 'ok' : 'warn'}">
      <span>${escapeHtml(label)}</span>
      <strong>${escapeHtml(value)}</strong>
    </div>
  `;
}

function availableStage() {
  if (!state.targetPayload?.items?.length) return 1;
  if (!state.plan) return 2;
  if (!state.executionPackage && !state.executionLog) return 3;
  return 4;
}

function setActiveStage(stage) {
  const next = Number(stage);
  if (!Number.isInteger(next) || next < 1 || next > availableStage()) return;
  state.activeStage = next;
  renderWorkflow();
}

function renderWorkflow() {
  const available = availableStage();
  if (state.activeStage > available) state.activeStage = available;
  els.stagePanels.forEach((panel) => {
    panel.hidden = Number(panel.dataset.stagePanel) !== state.activeStage;
  });
  els.stageTabs.forEach((tab) => {
    const stage = Number(tab.dataset.stage);
    const active = stage === state.activeStage;
    tab.disabled = stage > available;
    tab.classList.toggle('is-active', active);
    tab.classList.toggle('is-complete', stage < available);
    tab.setAttribute('aria-selected', active ? 'true' : 'false');
    tab.tabIndex = active && !tab.disabled ? 0 : -1;
    if (active) tab.setAttribute('aria-current', 'step');
    else tab.removeAttribute('aria-current');
  });

  const primary = els.stagePrimary;
  if (state.activeStage === 1) {
    primary.textContent = state.targetPayload?.items?.length ? '生成执行计划' : '导入目标课表';
    primary.disabled = !state.targetPayload?.items?.length;
  } else if (state.activeStage === 2) {
    primary.textContent = '进入执行准备';
    primary.disabled = !state.plan;
  } else if (state.activeStage === 3) {
    primary.textContent = els.writeConfig.checked ? '更新配置并运行安全检查' : '确认写入执行配置';
    primary.disabled = !state.plan;
  } else {
    primary.textContent = state.executionPackage ? '解析执行日志' : '等待人工执行';
    primary.disabled = !state.executionPackage;
  }
}

function setSettingsDrawer(open) {
  const isOpen = els.settingsDrawer.classList.contains('is-open');
  if (isOpen === open) return;
  if (open) {
    settingsReturnFocus = document.activeElement instanceof HTMLElement && document.activeElement !== document.body
      ? document.activeElement
      : els.settingsToggle;
    els.settingsDrawer.removeAttribute('inert');
  }
  els.settingsDrawer.classList.toggle('is-open', open);
  els.settingsDrawer.setAttribute('aria-hidden', open ? 'false' : 'true');
  if (open) {
    els.settingsClose.focus();
    return;
  }
  els.settingsDrawer.setAttribute('inert', '');
  const returnFocus = settingsReturnFocus;
  settingsReturnFocus = null;
  if (returnFocus && returnFocus.isConnected && !returnFocus.disabled) returnFocus.focus();
  else els.settingsToggle.focus();
}

function handleSettingsKeydown(event) {
  if (!els.settingsDrawer.classList.contains('is-open')) return;
  if (event.key === 'Escape') {
    event.preventDefault();
    setSettingsDrawer(false);
    return;
  }
  if (event.key !== 'Tab') return;
  const focusable = [...els.settingsContent.querySelectorAll(focusableSelector)]
    .filter((element) => element.getClientRects().length > 0);
  if (!focusable.length) {
    event.preventDefault();
    els.settingsClose.focus();
    return;
  }
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function handleStageKeydown(event) {
  const direction = {
    ArrowRight: 1,
    ArrowDown: 1,
    ArrowLeft: -1,
    ArrowUp: -1,
  }[event.key];
  const availableTabs = els.stageTabs.filter((tab) => !tab.disabled);
  if (event.key !== 'Home' && event.key !== 'End' && direction === undefined) return;
  if (!availableTabs.length) return;
  const currentIndex = Math.max(0, availableTabs.indexOf(event.currentTarget));
  let nextIndex = currentIndex;
  if (event.key === 'Home') nextIndex = 0;
  else if (event.key === 'End') nextIndex = availableTabs.length - 1;
  else nextIndex = (currentIndex + direction + availableTabs.length) % availableTabs.length;
  event.preventDefault();
  const next = availableTabs[nextIndex];
  setActiveStage(next.dataset.stage);
  next.focus();
}

function renderSettings() {
  const settings = state.settings || {};
  const s = state.status || {};
  els.settingsPath.textContent = settings.schedulerDir || settings.killCourseDir ? '已保存覆盖' : '自动检测';
  if (document.activeElement !== els.schedulerDir) {
    els.schedulerDir.value = settings.schedulerDir || '';
    els.schedulerDir.placeholder = s.schedulerDir || '留空则自动检测';
  }
  if (document.activeElement !== els.killCourseDir) {
    els.killCourseDir.value = settings.killCourseDir || '';
    els.killCourseDir.placeholder = s.killCourseDir || '留空则自动检测';
  }
}

function renderStatus() {
  const s = state.status || {};
  els.statusMessage.textContent = s.message || '等待状态。';
  els.term.textContent = `学期 ${s.term || '-'}`;
  els.statusGrid.innerHTML = [
    statCard('course.json', s.courseExists ? `${s.courseCount || 0} 门` : '未检测到', s.courseExists),
    statCard('personal-schedule.json', s.personalExists ? `${s.personalCount || 0} 门` : '可选缺失', true),
    statCard('personal-schedule-live.json', s.livePersonalExists ? `${s.livePersonalCount || 0} 门` : '未同步', Boolean(s.livePersonalExists)),
    statCard('HDU-KillCourse', s.killCourseExists ? '已检测到' : '未检测到', s.killCourseExists),
    statCard('备选教学班', s.canFallback ? '可生成' : '缺少 course.json', s.canFallback),
  ].join('');
  renderSettings();
}

function isKnown(value) {
  return value !== null && value !== undefined && value !== '';
}

function unknownText(value) {
  return isKnown(value) ? String(value) : '未知';
}

function countText(value) {
  return isKnown(value) ? `${value} 人` : '未知';
}

function capabilityText(value) {
  if (value === true) return '可';
  if (value === false) return '不可';
  return '未知';
}

function roundText(rounds) {
  if (!Array.isArray(rounds) || !rounds.length) return '未提供';
  return rounds.map((round) => `第${round}轮`).join('、');
}

function normalizeCourseOption(raw) {
  return {
    ...raw,
    displayCode: raw.displayCode || raw.jxbmc || raw.sectionName || '',
    courseName: raw.courseName || raw.kcmc || raw.name || '未命名课程',
    teacher: raw.teacher || raw.jzgxx || raw.jsxm || raw.js || '',
    timeText: raw.timeText || raw.sksj || raw.time || raw.schedule || '',
    location: raw.location || raw.jxdd || '',
    className: raw.className || raw.jxbzc || '',
    groupId: raw.groupId || raw.courseCode || raw.kch_id || '',
  };
}

function capacityFor(item) {
  const code = item?.displayCode || '';
  return state.courseCapacity.find((candidate) => candidate.displayCode === code) || {};
}

function courseFact(label, value, className = '') {
  return `
    <div class="course-fact ${className}">
      <span>${escapeHtml(label)}</span>
      <strong>${escapeHtml(value)}</strong>
    </div>
  `;
}

function renderCourseDetail(item) {
  if (!item) {
    els.courseIntelDetail.classList.add('empty');
    els.courseIntelDetail.textContent = state.courseOptions.length ? '没有匹配的课程，请调整筛选条件。' : '暂未读取课程库。';
    els.courseCapacitySummary.classList.add('empty');
    els.courseCapacitySummary.textContent = '容量和人数来自当前 course.json 本地快照，非实时；缺失时保持未知。';
    return;
  }

  const capacity = capacityFor(item);
  const courseCapacity = isKnown(capacity.capacity) ? capacity.capacity : item.capacity;
  const enrolled = isKnown(capacity.enrolled) ? capacity.enrolled : item.enrolled;
  const selected = isKnown(capacity.selected) ? capacity.selected : item.selected;
  const remaining = isKnown(capacity.remaining) ? capacity.remaining : null;
  const full = isKnown(capacity.full) ? capacity.full : null;
  const fullText = full === true ? '已满' : full === false ? '未满' : '未知';
  const fullClass = full === true ? 'danger-text' : full === false ? 'ok-text' : 'warn-text';

  els.courseIntelDetail.classList.remove('empty');
  els.courseIntelDetail.innerHTML = `
    <article class="course-detail-card">
      <div class="course-detail-head">
        <div>
          <h3>${escapeHtml(item.courseName)}</h3>
          <p class="course-detail-meta">${escapeHtml(item.displayCode || '未提供教学班号')} · ${escapeHtml(item.teacher || '未填写教师')}</p>
        </div>
        <span class="badge ${fullClass}">${escapeHtml(fullText)}</span>
      </div>
      <div class="course-facts">
        ${courseFact('可选', capabilityText(item.selectEnabled))}
        ${courseFact('可退', capabilityText(item.dropEnabled))}
        ${courseFact('允许轮次', roundText(item.selectRounds))}
        ${courseFact('当前轮次', isKnown(state.courseCurrentRound) ? `第${state.courseCurrentRound}轮` : '未提供')}
        ${courseFact('容量', countText(courseCapacity))}
        ${courseFact('教学班人数', countText(enrolled))}
        ${courseFact('选课人数', countText(selected))}
        ${courseFact('剩余名额', countText(remaining))}
      </div>
      <div class="course-detail-meta">${escapeHtml(item.className || '未提供行政班')} · ${escapeHtml(item.location || '未提供地点')} · ${escapeHtml(item.timeText || '未提供时间')} · ${escapeHtml(isKnown(item.credits) ? `${item.credits} 学分` : '学分未知')}</div>
    </article>
  `;

  const source = state.courseCapacitySource || 'course.json 快照';
  const observedAt = state.courseCapacityObservedAt ? `；接口读取时间：${state.courseCapacityObservedAt}` : '';
  const sourceUpdatedAt = state.courseCapacitySourceUpdatedAt ? `；快照更新时间：${state.courseCapacitySourceUpdatedAt}` : '';
  const freshness = state.courseCapacityStale ? '；状态：本地快照，非实时' : '';
  els.courseCapacitySummary.classList.remove('empty');
  els.courseCapacitySummary.innerHTML = `
    <strong>容量与人数快照</strong>
    <div>容量 ${escapeHtml(countText(courseCapacity))}，教学班人数 ${escapeHtml(countText(enrolled))}，选课人数 ${escapeHtml(countText(selected))}，剩余 ${escapeHtml(countText(remaining))}。</div>
    <div>来源：${escapeHtml(source)}${escapeHtml(sourceUpdatedAt)}${escapeHtml(observedAt)}${escapeHtml(freshness)}。未知字段不会被推算。</div>
  `;
}

function scheduleCard(item) {
  const capacity = capacityFor(item);
  const remaining = isKnown(capacity.remaining) ? capacity.remaining : null;
  return `
    <article class="schedule-item">
      <div class="schedule-item-head">
        <strong>${escapeHtml(item.displayCode || '未提供教学班号')}</strong>
        <span>${escapeHtml(item.className || '未提供行政班')}</span>
      </div>
      <div>${escapeHtml(item.courseName || '未命名课程')} · ${escapeHtml(item.teacher || '未填写教师')}</div>
      <div>${escapeHtml(item.timeText || '未提供时间')} · ${escapeHtml(item.location || '未提供地点')}</div>
      <div>容量 ${escapeHtml(countText(capacity.capacity))} · 教学班人数 ${escapeHtml(countText(capacity.enrolled))} · 剩余 ${escapeHtml(countText(remaining))}</div>
    </article>
  `;
}

function renderCourseSchedule() {
  const schedule = state.courseSchedule;
  if (!schedule) {
    els.classScheduleResult.classList.add('empty');
    els.classScheduleResult.textContent = '选择教学班后可查看同课程号的教学班课表。';
    return;
  }
  const items = (schedule.items || []).map(normalizeCourseOption);
  const warningHtml = (schedule.warnings || []).length
    ? `<div class="course-warning">${escapeHtml(schedule.warnings.join('；'))}</div>`
    : '';
  if (!items.length) {
    els.classScheduleResult.classList.add('empty');
    els.classScheduleResult.innerHTML = `${warningHtml}未找到匹配的教学班课表。`;
    return;
  }
  els.classScheduleResult.classList.remove('empty');
  els.classScheduleResult.innerHTML = `
    <div class="schedule-result-head"><strong>教学班课表</strong><span>${items.length} 个教学班</span></div>
    ${warningHtml}
    <div class="schedule-list">${items.map(scheduleCard).join('')}</div>
  `;
}

function renderClassOptions() {
  const options = state.classOptions || [];
  const select = els.classOptionSelect;
  select.innerHTML = '';
  if (!options.length) {
    const empty = document.createElement('option');
    empty.value = '';
    empty.textContent = '无行政班数据';
    select.appendChild(empty);
    select.disabled = true;
    els.classScheduleQuery.disabled = true;
    if (state.classOptionsWarnings.length) {
      els.adminClassScheduleResult.classList.remove('empty');
      els.adminClassScheduleResult.innerHTML = `<div class="course-warning">${escapeHtml(state.classOptionsWarnings.join('；'))}</div>`;
    } else {
      els.adminClassScheduleResult.classList.add('empty');
      els.adminClassScheduleResult.textContent = '选择行政班后可查看该班全部课程。';
    }
    return;
  }
  const placeholder = document.createElement('option');
  placeholder.value = '';
  placeholder.textContent = '选择行政班';
  select.appendChild(placeholder);
  for (const item of options) {
    const option = document.createElement('option');
    option.value = item.name;
    option.textContent = `${item.name}（${item.count}）`;
    select.appendChild(option);
  }
  select.disabled = false;
  if (state.classInspectorName && options.some((item) => item.name === state.classInspectorName)) {
    select.value = state.classInspectorName;
  } else {
    select.value = '';
    state.classInspectorName = '';
  }
  els.classScheduleQuery.disabled = !state.classInspectorName;
  renderAdminClassSchedule();
}

async function queryClassSchedule() {
  const name = els.classOptionSelect.value;
  if (!name) return;
  state.classInspectorName = name;
  els.classScheduleQuery.disabled = true;
  els.classScheduleQuery.textContent = '正在查询...';
  try {
    const params = new URLSearchParams({ className: name });
    const result = await fetchJSON(`/api/class-schedule?${params.toString()}`);
    if (!result.ok) throw new Error(result.error || '读取班级课表失败');
    state.classSchedule = result;
    renderAdminClassSchedule();
  } catch (error) {
    state.classSchedule = { items: [], warnings: [String(error.message || error)] };
    renderAdminClassSchedule();
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.classScheduleQuery.disabled = !state.classInspectorName;
    els.classScheduleQuery.textContent = '查看班级课表';
  }
}

function renderAdminClassSchedule() {
  const schedule = state.classSchedule;
  if (!schedule) {
    els.adminClassScheduleResult.classList.add('empty');
    els.adminClassScheduleResult.textContent = '选择行政班后可查看该班全部课程。';
    return;
  }
  const items = (schedule.items || []).map(normalizeCourseOption);
  const warningHtml = (schedule.warnings || []).length
    ? `<div class="course-warning">${escapeHtml(schedule.warnings.join('；'))}</div>`
    : '';
  if (!items.length) {
    els.adminClassScheduleResult.classList.add('empty');
    els.adminClassScheduleResult.innerHTML = `${warningHtml}未找到该行政班的课程。`;
    return;
  }
  els.adminClassScheduleResult.classList.remove('empty');
  els.adminClassScheduleResult.innerHTML = `
    <div class="schedule-result-head"><strong>行政班课表${state.classInspectorName ? ' · ' + escapeHtml(state.classInspectorName) : ''}</strong><span>${items.length} 门课程</span></div>
    ${warningHtml}
    <div class="schedule-list">${items.map(scheduleCard).join('')}</div>
  `;
}

function renderCourseIntel() {
  const options = state.courseOptions.map(normalizeCourseOption);
  const filter = (els.courseOptionFilter?.value || '').trim().toLowerCase();
  const filtered = options.filter((item) => {
    if (!filter) return true;
    return [item.courseName, item.displayCode, item.teacher, item.className]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(filter));
  });
  const selected = filtered.find((item) => item.displayCode === state.courseInspectorCode) || filtered[0] || null;
  const nextCode = selected?.displayCode || '';
  if (nextCode !== state.courseInspectorCode) {
    state.courseInspectorCode = nextCode;
    state.courseSchedule = null;
  }

  if (!options.length) {
    els.courseIntelBadge.textContent = '不可用';
    els.courseIntelBadge.className = 'badge danger-badge';
    els.courseIntelSummary.textContent = '未检测到可读取的课程库。';
    els.courseOptionSelect.innerHTML = '<option value="">暂无课程</option>';
    els.courseOptionSelect.disabled = true;
    els.courseScheduleQuery.disabled = true;
    renderCourseDetail(null);
    renderCourseSchedule();
    return;
  }

  els.courseIntelBadge.textContent = `${options.length} 门`;
  els.courseIntelBadge.className = 'badge ok-badge';
  const roundSummary = isKnown(state.courseCurrentRound) ? `当前第${state.courseCurrentRound}轮` : '当前轮次未知';
  const warningSummary = state.courseOptionsWarnings.length ? `；${state.courseOptionsWarnings.join('；')}` : '';
  els.courseIntelSummary.textContent = `课程库 ${options.length} 门，${roundSummary}。${warningSummary}`;
  els.courseOptionSelect.innerHTML = filtered.length
    ? filtered.map((item) => `<option value="${escapeHtml(item.displayCode)}">${escapeHtml(`${item.courseName} · ${item.displayCode}${item.teacher ? ` · ${item.teacher}` : ''}`)}</option>`).join('')
    : '<option value="">没有匹配课程</option>';
  els.courseOptionSelect.value = nextCode;
  els.courseOptionSelect.disabled = !filtered.length;
  els.courseScheduleQuery.disabled = !selected;
  renderCourseDetail(selected);
  renderCourseSchedule();
}

function courseCard(item, danger = false) {
  const meta = [
    item.displayCode,
    item.teacher,
    item.timeText,
  ].filter(Boolean).join(' · ');
  return `
    <article class="course-card ${danger ? 'danger' : ''}">
      <h4>${escapeHtml(item.courseName || '未命名课程')}</h4>
      <div>${escapeHtml(meta)}</div>
    </article>
  `;
}

function currentCourseCard(item) {
  const code = item.displayCode || '';
  const meta = [
    code,
    item.teacher,
    item.timeText,
  ].filter(Boolean).join(' · ');
  const checked = state.lockedCodes.has(code) ? 'checked' : '';
  return `
    <article class="course-card lock-card">
      <label>
        <input type="checkbox" data-lock-code="${escapeHtml(code)}" ${checked} />
        <span>
          <strong>${escapeHtml(item.courseName || '未命名课程')}</strong>
          <small>${escapeHtml(meta)}</small>
        </span>
      </label>
    </article>
  `;
}

function renderCourseList(el, items, emptyText, danger = false) {
  if (!items || !items.length) {
    el.classList.add('empty');
    el.innerHTML = escapeHtml(emptyText);
    return;
  }
  el.classList.remove('empty');
  el.innerHTML = items.map((item) => courseCard(item, danger)).join('');
}

function renderCurrent() {
  const items = hasLiveSnapshot() ? state.liveItems : state.current;
  els.currentCount.textContent = `${items.length} 门`;
  if (!items.length) {
    els.currentList.classList.add('empty');
    els.currentList.textContent = hasLiveSnapshot()
      ? '已导入实时个人课表快照，当前为 0 门课程。'
      : (state.currentLoadError || '未检测到 personal-schedule.json，当前课表按空处理。');
    return;
  }
  els.currentList.classList.remove('empty');
  els.currentList.innerHTML = items.map(currentCourseCard).join('');
  els.currentList.querySelectorAll('[data-lock-code]').forEach((input) => {
    input.addEventListener('change', () => {
      const code = input.dataset.lockCode || '';
      if (!code) return;
      if (input.checked) state.lockedCodes.add(code);
      else state.lockedCodes.delete(code);
    });
  });
}

function renderLiveSync() {
  renderRefreshControls();
  const sync = state.liveSchedule || {};
  const hasLive = hasLiveSnapshot();
  if (!els.liveSyncBadge) return;
  els.liveSyncBadge.textContent = hasLive ? '已导入快照' : '未同步';
  els.liveSyncBadge.className = `badge ${hasLive ? 'ok-badge' : 'danger-badge'}`;
  if (!hasLive) {
    els.liveSyncSummary.classList.add('empty');
    els.liveSyncSummary.textContent = '含退课计划授权前必须先导入真实个人课表快照，并重新生成执行计划。';
    renderCourseList(els.liveSyncDiff, [], '尚未导入真实课表快照。');
    return;
  }
  els.liveSyncSummary.classList.remove('empty');
  els.liveSyncSummary.innerHTML = `
    <strong>${sync.hasDrift ? '检测到课表差异' : '未检测到课表差异'}</strong>
    <div>真实课表 ${escapeHtml(sync.liveCount ?? state.liveItems.length)} 门，本地底板 ${escapeHtml(sync.localCount ?? 0)} 门。</div>
    <div>来源：${escapeHtml(sourceText(sync.source))}；同步时间：${escapeHtml(formatTimestamp(sync.syncedAt || state.lastRefreshAt))}</div>
  `;
  const added = (sync.added || []).map((item) => ({ ...item, courseName: `[新增] ${item.courseName || ''}` }));
  const removed = (sync.removed || []).map((item) => ({ ...item, courseName: `[移除] ${item.courseName || ''}` }));
  const changed = (sync.changed || []).map((item) => ({ ...item, courseName: `[信息变更] ${item.courseName || ''}` }));
  const diff = [...added, ...removed, ...changed];
  renderCourseList(els.liveSyncDiff, diff, '真实课表与本地底板没有课程增删或信息差异。', sync.hasDrift);
}

function setTimeElement(element, value, emptyText = '未记录') {
  if (!element) return;
  element.textContent = value ? formatTimestamp(value) : emptyText;
  if (value) element.dateTime = value;
  else element.removeAttribute('datetime');
  element.title = value || '';
}

function renderRefreshControls() {
  if (!els.refreshLive) return;
  const sync = state.liveSchedule || {};
  const hasLive = hasLiveSnapshot();
  const last = state.lastRefreshAt || (hasLive ? sync.syncedAt : '');
  els.autoRefresh.checked = state.autoRefresh;
  if (document.activeElement !== els.refreshInterval) els.refreshInterval.value = state.refreshIntervalSeconds;
  els.refreshInterval.disabled = false;
  els.refreshLive.disabled = state.liveRefreshInFlight;
  els.refreshLive.textContent = state.liveRefreshInFlight ? '刷新中…' : '立即刷新';
  els.liveSource.textContent = hasLive ? sourceText(sync.source) : '尚未刷新';
  els.liveSource.title = hasLive ? (sync.source || '') : '';
  setTimeElement(els.lastRefreshAt, last, '未刷新');
  setTimeElement(els.nextRefreshAt, state.autoRefresh ? state.nextRefreshAt : '', state.autoRefresh ? '未计划' : '已关闭');
  if (state.refreshError) {
    els.liveRefreshError.hidden = false;
    const streakText = state.liveRefreshFailureStreak > 1
      ? `连续失败 ${state.liveRefreshFailureStreak} 次，下次约 ${Math.round(effectiveLiveRefreshWait())} 秒后重试。`
      : '';
    els.liveRefreshError.textContent = `最近一次刷新失败（${formatTimestamp(state.refreshFailureAt)}）：${state.refreshError} 已保留上次成功课表。${streakText}`;
  } else if (state.liveImportError) {
    els.liveRefreshError.hidden = false;
    els.liveRefreshError.textContent = `最近一次手动导入失败（${formatTimestamp(state.liveImportFailureAt)}）：${state.liveImportError} 已保留上次成功课表。`;
  } else {
    els.liveRefreshError.hidden = true;
    els.liveRefreshError.textContent = '';
  }
}

function renderTarget() {
  const items = state.targetPayload?.items || [];
  els.targetCount.textContent = items.length ? `${items.length} 门` : '未导入';
  if (els.targetSource) {
    const source = state.targetManualOverride
      ? `手动导入${state.targetFileName ? `：${state.targetFileName}` : ''}`
      : state.targetPath
        ? `自动导入：${state.targetPath}`
        : '尚未自动发现';
    els.targetSource.textContent = source;
    els.targetSource.title = state.targetPath || state.targetFileName || '';
  }
  setTimeElement(els.targetUpdatedAt, state.targetUpdatedAt, '未发现');
  if (els.targetWarning) {
    els.targetWarning.hidden = !state.targetWarnings.length;
    els.targetWarning.textContent = state.targetWarnings.join('；');
  }
  renderCourseList(els.targetPreview, items.map(normalizePreviewItem), '尚未导入目标课表。');
  renderWorkflow();
}

function normalizePreviewItem(raw) {
  return {
    displayCode: raw.displayCode || raw.jxbmc || raw.sectionName || '',
    courseName: raw.courseName || raw.kcmc || raw.name || '未命名课程',
    teacher: raw.teacher || raw.jzgxx || raw.jsxm || raw.js || '',
    timeText: raw.timeText || raw.sksj || raw.time || raw.schedule || '',
  };
}

function renderPlan() {
  const plan = state.plan;
  if (!plan) {
    els.planState.textContent = '等待生成';
    els.planState.className = 'badge';
    els.keepSummary.textContent = '保留课程将在详情中显示。';
    els.planSummary.innerHTML = '';
    renderCourseList(els.selectList, [], '暂无');
    renderCourseList(els.dropList, [], '暂无', true);
    renderCourseList(els.keepList, [], '暂无');
    renderCourseList(els.fallbackList, [], '生成计划后显示。');
    renderCourseList(els.riskList, [], '生成计划后显示。', true);
    renderCourseList(els.validationList, [], '生成计划后显示。');
    renderCourseList(els.explanationList, [], '生成计划后显示。');
    renderReadiness();
    renderDryRun();
    renderExecutionLog();
    renderFallbackRecommendations();
    renderConfigPreview();
    els.outputJson.textContent = '{}';
    els.downloadPlan.disabled = true;
    els.downloadConfig.disabled = true;
    renderWorkflow();
    return;
  }
  els.planState.textContent = plan.validationIssues?.some((item) => item.level === 'error') ? '存在阻断问题' : '已生成';
  els.planState.className = `badge ${plan.validationIssues?.some((item) => item.level === 'error') ? 'danger-badge' : 'ok-badge'}`;
  els.planSummary.innerHTML = [
    statCard('需要选课', `${plan.select.length} 门`, true),
    statCard('需要退课', `${plan.drop.length} 门`, plan.drop.length === 0),
    statCard('保留课程', `${plan.keep.length} 门`, true),
    statCard('备选分组', `${plan.fallbackGroups.length} 组`, true),
  ].join('');
  renderCourseList(els.selectList, plan.select, '暂无');
  renderCourseList(els.dropList, plan.drop, '暂无', true);
  renderCourseList(els.keepList, plan.keep, '暂无');
  els.keepSummary.textContent = `保留 ${plan.keep.length} 门课程。完整列表和计划说明可在下方详情展开。`;
  renderFallbacks(plan.fallbackGroups);
  renderRisks(plan.risks);
  renderValidation(plan.validationIssues);
  renderExplanations(plan.explanations);
  renderReadiness();
  renderDryRun();
  renderExecutionLog();
  renderFallbackRecommendations();
  renderConfigPreview();
  els.outputJson.textContent = JSON.stringify(plan, null, 2);
  els.downloadPlan.disabled = false;
  els.downloadConfig.disabled = !state.generatedConfig;
  renderWorkflow();
}

function renderFallbacks(groups) {
  if (!groups || !groups.length) {
    els.fallbackList.classList.add('empty');
    els.fallbackList.textContent = '没有可替代教学班。';
    return;
  }
  els.fallbackList.classList.remove('empty');
  els.fallbackList.innerHTML = groups.map((group) => `
    <article class="course-card">
      <h4>${escapeHtml(group.courseName)} / ${escapeHtml(group.courseBase)}</h4>
      <div>目标：${escapeHtml(group.preferred)}</div>
      <div class="chips">
        ${group.alternatives.map((item) => `<span>${escapeHtml(item.displayCode)} · ${escapeHtml(item.teacher)}</span>`).join('')}
      </div>
    </article>
  `).join('');
}

function levelClass(level) {
  if (level === 'high' || level === 'error' || level === 'warning') return 'danger';
  return '';
}

function renderRisks(risks) {
  if (!risks || !risks.length) {
    els.riskList.classList.add('empty');
    els.riskList.textContent = '暂无风险。';
    return;
  }
  els.riskList.classList.remove('empty');
  els.riskList.innerHTML = risks.map((risk) => `
    <article class="course-card danger">
      <h4>${escapeHtml(risk.level || 'risk')}</h4>
      <div>${escapeHtml(risk.message)}</div>
      ${risk.code ? `<div>${escapeHtml(risk.code)}</div>` : ''}
    </article>
  `).join('');
}

function renderValidation(issues) {
  if (!issues || !issues.length) {
    els.validationList.classList.add('empty');
    els.validationList.textContent = '校验通过，未发现明显问题。';
    return;
  }
  els.validationList.classList.remove('empty');
  els.validationList.innerHTML = issues.map((issue) => `
    <article class="course-card ${levelClass(issue.level)}">
      <h4>${escapeHtml(issue.level || '提示')}</h4>
      <div>${escapeHtml(issue.message)}</div>
      ${issue.code ? `<div>${escapeHtml(issue.code)}</div>` : ''}
    </article>
  `).join('');
}

function renderExplanations(explanations) {
  if (!explanations || !explanations.length) {
    els.explanationList.classList.add('empty');
    els.explanationList.textContent = '暂无计划解释。';
    return;
  }
  els.explanationList.classList.remove('empty');
  els.explanationList.innerHTML = explanations.map((item) => `
    <article class="course-card">
      <h4>${escapeHtml(item.category || '说明')}</h4>
      <div>${escapeHtml(item.message)}</div>
      ${item.code ? `<div>${escapeHtml(item.code)}</div>` : ''}
    </article>
  `).join('');
}

function renderReadiness() {
  const readiness = state.readiness;
  if (!readiness) {
    els.readinessSummary.classList.add('empty');
    els.readinessSummary.textContent = '生成计划后显示。';
    renderCourseList(els.readinessList, [], '生成计划后显示。');
    return;
  }
  els.readinessSummary.classList.remove('empty');
  els.readinessSummary.textContent = readiness.summary || '-';
  const checks = readiness.checks || [];
  if (!checks.length) {
    renderCourseList(els.readinessList, [], '没有检查项。');
    return;
  }
  els.readinessList.classList.remove('empty');
  els.readinessList.innerHTML = checks.map((check) => {
    const danger = !check.passed && (check.level === 'error' || check.level === 'warning');
    const status = check.passed ? '通过' : (check.level === 'error' ? '错误' : check.level === 'warning' ? '警告' : '提示');
    return `
      <article class="course-card ${danger ? 'danger' : ''}">
        <h4>${escapeHtml(status)} · ${escapeHtml(check.level || 'info')}</h4>
        <div>${escapeHtml(check.message)}</div>
      </article>
    `;
  }).join('');
}

function renderDryRun() {
  const dryRun = state.dryRun;
  els.dryRun.disabled = !state.plan || !state.generatedConfig || !els.writeConfig.checked;
  if (!dryRun) {
    els.dryRunBadge.textContent = '未检查';
    els.dryRunBadge.className = 'badge';
    els.dryRunSummary.classList.add('empty');
    els.dryRunSummary.textContent = state.plan ? '可以进行 dry-run。该检查只验证执行条件，不会真实选课或退课。' : '生成执行计划后可进行 dry-run。该检查不会真实执行选课或退课。';
    els.dryRunCommand.classList.add('empty');
    els.dryRunCommand.textContent = '等待 dry-run。';
    renderCourseList(els.dryRunEvents, [], '等待 dry-run。');
    renderAuthorization();
    return;
  }
  els.dryRunBadge.textContent = dryRun.ready ? 'Dry-run 通过' : 'Dry-run 未通过';
  els.dryRunBadge.className = `badge ${dryRun.ready ? 'ok-badge' : 'danger-badge'}`;
  els.dryRunSummary.classList.remove('empty');
  els.dryRunSummary.textContent = dryRun.summary || '-';
  els.dryRunCommand.classList.remove('empty');
  els.dryRunCommand.innerHTML = `
    <strong>命令</strong><code>${escapeHtml(dryRun.command || '-')}</code>
    <strong>工作目录</strong><code>${escapeHtml(dryRun.killCourseDir || '-')}</code>
    <strong>配置文件</strong><code>${escapeHtml(dryRun.configPath || '-')}</code>
    <strong>启动入口</strong><code>${escapeHtml(dryRun.entryPath || '-')}</code>
    <strong>日志文件</strong><code>${escapeHtml(dryRun.logPath || '-')}</code>
    <span>选课 ${escapeHtml(dryRun.actionCounts?.select || 0)} 条，退课 ${escapeHtml(dryRun.actionCounts?.drop || 0)} 条，总计 ${escapeHtml(dryRun.actionCounts?.total || 0)} 条。</span>
  `;
  const events = dryRun.events || [];
  if (!events.length) {
    renderCourseList(els.dryRunEvents, [], '没有 dry-run 日志。');
    renderAuthorization();
    return;
  }
  els.dryRunEvents.classList.remove('empty');
  els.dryRunEvents.innerHTML = events.map((event) => `
    <article class="course-card ${levelClass(event.level)}">
      <h4>${escapeHtml(event.level || 'info')}</h4>
      <div>${escapeHtml(event.message)}</div>
    </article>
  `).join('');
  renderAuthorization();
}

function renderAuthorization() {
  const dryRun = state.dryRun;
  const authorization = state.authorization;
  const executionPackage = state.executionPackage;
  const canAuthorize = Boolean(dryRun?.canExecute && state.plan && state.generatedConfig && els.writeConfig.checked);
  const authorizationExpired = Boolean(authorization && Date.parse(authorization.expiresAt || '') <= Date.now());
  els.authorizeExecution.disabled = !canAuthorize;
  els.confirmPhrase.disabled = !canAuthorize;
  els.buildPackage.disabled = !authorization || authorizationExpired;
  els.confirmPhrase.placeholder = canAuthorize ? '输入确认短语' : '等待 dry-run 通过';
  const inlineRunning = Boolean(state.inlineExecution?.active || state.inlineExecutionPolling);
  els.inlineExecStart.disabled = !authorization || authorizationExpired || inlineRunning;
  els.inlineExecStop.disabled = !inlineRunning;
  renderInlineExecution();
  if (canAuthorize) {
    els.confirmHint.textContent = `请输入确认短语：${dryRun.confirmationPhrase}`;
  } else {
    els.confirmHint.textContent = 'Dry-run 通过后，这里会显示需要输入的确认短语。生成授权票据仍不会真实执行选课或退课。';
  }
  if (!authorization) {
    els.authorizationResult.classList.add('empty');
    els.authorizationResult.textContent = '尚未生成授权票据。';
    els.packageResult.classList.add('empty');
    els.packageResult.textContent = '尚未生成启动包。';
    renderExecutionLog();
    return;
  }
  els.authorizationResult.classList.remove('empty');
  els.authorizationResult.innerHTML = `
    <strong>授权票据已生成</strong>
    <div>票据：${escapeHtml(authorization.ticketId || '-')}</div>
    <div>有效期至：${escapeHtml(authorization.expiresAt || '-')}</div>
    <div>选课 ${escapeHtml(authorization.actionCounts?.select || 0)} 条，退课 ${escapeHtml(authorization.actionCounts?.drop || 0)} 条。</div>
    ${authorizationExpired ? '<div>授权票据已过期，请重新 dry-run 并确认。</div>' : ''}
  `;
  if (!executionPackage) {
    els.packageResult.classList.add('empty');
    els.packageResult.textContent = '尚未生成启动包。';
    renderExecutionLog();
    return;
  }
  els.packageResult.classList.remove('empty');
  els.packageResult.innerHTML = `
    <strong>启动包已生成</strong>
    <div>${escapeHtml(executionPackage.summary || '')}</div>
    <div>bat：${escapeHtml(executionPackage.batchPath || '-')}</div>
    <div>说明：${escapeHtml(executionPackage.runbookPath || '-')}</div>
    <div>清单：${escapeHtml(executionPackage.manifestPath || '-')}</div>
  `;
  renderExecutionLog();
}

function renderExecutionLog() {
  els.parseLog.disabled = !state.plan || !state.generatedConfig;
  const log = state.executionLog;
  if (!log) {
    els.executionLogSummary.classList.add('empty');
    els.executionLogSummary.textContent = '手动运行 KillCourse 后，可解析 log_files/app.log。';
    renderCourseList(els.executionLogList, [], '尚未解析执行结果。');
    return;
  }
  const summary = log.summary || {};
  els.executionLogSummary.classList.remove('empty');
  els.executionLogSummary.innerHTML = `
    <strong>执行结果</strong>
    <div>总计 ${escapeHtml(summary.total || 0)} 条，成功 ${escapeHtml(summary.success || 0)} 条，失败 ${escapeHtml(summary.failed || 0)} 条，待确认 ${escapeHtml(summary.pending || 0)} 条。</div>
    <div>来源：${escapeHtml(log.logPath || '-')}</div>
  `;
  const items = log.items || [];
  if (!items.length) {
    renderCourseList(els.executionLogList, [], '日志中未识别到课程动作。');
    return;
  }
  els.executionLogList.classList.remove('empty');
  els.executionLogList.innerHTML = items.map((item) => `
    <article class="course-card ${item.status === 'failed' ? 'danger' : ''}">
      <h4>${escapeHtml(item.status || 'unknown')} · ${escapeHtml(item.action || 'unknown')} · ${escapeHtml(item.courseCode || '-')}</h4>
      <div>${escapeHtml(item.courseName || '')}</div>
      <div>${escapeHtml(item.message || item.timeText || '')}</div>
      ${item.failureType ? `<div>失败类型：${escapeHtml(item.failureType)}</div>` : ''}
    </article>
  `).join('');
}

function renderFallbackRecommendations() {
  if (!els.buildFallbackRecommendations) return;
  els.buildFallbackRecommendations.disabled = !state.plan || !state.executionLog;
  const recs = state.fallbackRecommendations;
  if (!recs) {
    els.fallbackRecommendationSummary.classList.add('empty');
    els.fallbackRecommendationSummary.textContent = state.executionLog
      ? '可以生成失败课程的备选推荐。该步骤只读分析，不会修改 config.json。'
      : '解析执行日志后，可为选课失败项推荐同课程号备选教学班。';
    renderCourseList(els.fallbackRecommendationList, [], '尚未生成备选推荐。');
    return;
  }
  const summary = recs.summary || {};
  els.fallbackRecommendationSummary.classList.remove('empty');
  els.fallbackRecommendationSummary.innerHTML = `
    <strong>备选推荐</strong>
    <div>失败选课 ${escapeHtml(summary.failedSelectCount || 0)} 门，有备选 ${escapeHtml(summary.withOptions || 0)} 门，无备选 ${escapeHtml(summary.withoutOptions || 0)} 门。</div>
  `;
  const items = recs.items || [];
  if (!items.length) {
    renderCourseList(els.fallbackRecommendationList, [], '没有需要推荐备选的失败选课项。');
    return;
  }
  els.fallbackRecommendationList.classList.remove('empty');
  els.fallbackRecommendationList.innerHTML = items.map((item) => {
    const options = item.options || [];
    const optionHtml = options.length ? options.map((option) => `
      <div class="fallback-option ${option.timeCompatible ? '' : 'danger-option'}">
        <strong>#${escapeHtml(option.rank)} · ${escapeHtml(option.course?.displayCode || '-')} · ${escapeHtml(option.course?.courseName || '')}</strong>
        <span>评分 ${escapeHtml(option.score || 0)} · ${escapeHtml(option.course?.teacher || '未填写教师')} · ${escapeHtml(option.course?.timeText || '无时间')}</span>
        <span>${escapeHtml((option.reasons || []).join('；') || '无补充理由')}</span>
        ${(option.conflicts || []).length ? `<span class="danger-text">冲突：${escapeHtml((option.conflicts || []).map((course) => course.courseName || course.displayCode).join('；'))}</span>` : ''}
        ${(option.warnings || []).length ? `<span class="warn-text">${escapeHtml(option.warnings.join('；'))}</span>` : ''}
      </div>
    `).join('') : '<div class="muted">暂无可推荐备选。</div>';
    return `
      <article class="course-card ${options.length ? '' : 'danger'}">
        <h4>${escapeHtml(item.failedCourse || '-')} · ${escapeHtml(item.courseName || '')}</h4>
        <div>失败类型：${escapeHtml(item.failureType || 'unknown')} · ${escapeHtml(item.message || '')}</div>
        <div>${escapeHtml(item.recommendation || '')}</div>
        <div class="fallback-options">${optionHtml}</div>
      </article>
    `;
  }).join('');
}

function actionText(action) {
  if (action === '1') return '选课';
  if (action === '0') return '退课';
  return '无';
}

function statusText(status) {
  return {
    added: '新增',
    removed: '移除',
    changed: '变更',
    unchanged: '不变',
  }[status] || status || '-';
}

function renderConfigPreview() {
  const preview = state.configPreview;
  if (!preview) {
    els.configSummary.innerHTML = '';
    renderCourseList(els.configPreserveList, [], '生成计划后显示。');
    renderCourseList(els.configActionList, [], '生成计划后显示。');
    return;
  }
  els.configSummary.innerHTML = [
    statCard('已有配置', preview.existingConfigFound ? '已读取' : '未找到，使用默认模板', true),
    statCard('旧 course 动作', `${preview.oldActionCount || 0} 条`, true),
    statCard('新 course 动作', `${preview.newActionCount || 0} 条`, true),
    statCard('写入路径', preview.path || '-', Boolean(preview.path)),
  ].join('');

  const preserved = [
    `CAS 登录：${preview.hasCasLogin ? '已保留账号/密码' : '未配置'}`,
    `新教务登录：${preview.hasNewjwLogin ? '已保留账号/密码' : '未配置'}`,
    `Cookie：enabled=${preview.cookiesEnabled || '0'}`,
    `蹲课：enabled=${preview.waitCourseEnabled || '0'}`,
    `邮箱：enabled=${preview.smtpEnabled || '0'}`,
    `开始时间：${preview.startTime || '-'}`,
    `学年学期：${preview.xueNian || '-'} / ${preview.xueQi || '-'}`,
  ];
  els.configPreserveList.classList.remove('empty');
  els.configPreserveList.innerHTML = preserved.map((text) => `
    <article class="course-card">
      <div>${escapeHtml(text)}</div>
    </article>
  `).join('');

  const actions = preview.actions || [];
  if (!actions.length) {
    els.configActionList.classList.add('empty');
    els.configActionList.textContent = '没有 course 动作。';
    return;
  }
  els.configActionList.classList.remove('empty');
  els.configActionList.innerHTML = actions.map((item) => `
    <article class="course-card ${item.newAction === '0' ? 'danger' : ''}">
      <h4>${escapeHtml(statusText(item.status))} · ${escapeHtml(item.code)}</h4>
      <div>旧动作：${escapeHtml(actionText(item.oldAction))} → 新动作：${escapeHtml(actionText(item.newAction))}</div>
    </article>
  `).join('');
}

function scheduleLiveRefresh() {
  clearLiveRefreshTimer();
  if (!state.autoRefresh) {
    state.nextRefreshAt = '';
    renderRefreshControls();
    return;
  }
  const effectiveWait = effectiveLiveRefreshWait();
  const referenceTime = (state.liveRefreshFailureStreak || 0) > 0
    ? Date.now()
    : (() => {
        const reference = latestRefreshTimestamp();
        return reference ? Date.parse(reference) : Date.now();
      })();
  const dueAt = referenceTime + effectiveWait * 1000;
  state.nextRefreshAt = new Date(dueAt).toISOString();
  state.liveRefreshTimer = setTimeout(() => {
    state.liveRefreshTimer = null;
    refreshLiveSchedule({ reason: 'auto' }).catch(() => {});
  }, Math.max(1000, dueAt - Date.now()));
  renderRefreshControls();
}

function effectiveLiveRefreshWait() {
  const base = state.refreshIntervalSeconds || 60;
  const streak = state.liveRefreshFailureStreak || 0;
  if (typeof window.HDUExponentialBackoff?.liveRefreshWaitingSeconds === 'function') {
    return window.HDUExponentialBackoff.liveRefreshWaitingSeconds(base, streak, 7200);
  }
  return base;
}

async function maybeStartAutoRefresh() {
  if (!state.autoRefresh) {
    scheduleLiveRefresh();
    return;
  }
  const reference = latestRefreshTimestamp();
  const stale = !reference || Date.now() - Date.parse(reference) >= state.refreshIntervalSeconds * 1000;
  if (stale) {
    await refreshLiveSchedule({ reason: 'auto' });
    return;
  }
  scheduleLiveRefresh();
}

async function refreshLiveSchedule({ reason = 'manual' } = {}) {
  if (state.liveRefreshInFlight) return state.liveRefreshPromise;
  state.liveRefreshInFlight = true;
  const promise = (async () => {
    const attemptAt = new Date().toISOString();
    const previousSignature = hasLiveSnapshot()
      ? liveScheduleSignature(state.liveItems, state.liveSchedule)
      : liveScheduleSignature(state.current, null);
    const hadPlanArtifacts = Boolean(state.plan || state.generatedConfig || state.executionPackage || state.executionLog);
    state.refreshError = '';
    renderRefreshControls();
    try {
      const result = await fetchJSON('/api/live-schedule/refresh', { method: 'POST' });
      if (!result.ok) throw new Error(result.error || '刷新个人课表失败');
      state.liveSchedule = result.sync || null;
      state.liveItems = result.items || [];
      state.liveSnapshotAvailable = result.hasSnapshot === true;
      state.status = result.status || state.status;
      state.lastRefreshAt = result.sync?.syncedAt || attemptAt;
      state.refreshFailureAt = '';
      state.refreshError = '';
      state.liveImportFailureAt = '';
      state.liveImportError = '';
      state.liveRefreshFailureStreak = 0;
      const nextSignature = liveScheduleSignature(state.liveItems, state.liveSchedule);
      if (hadPlanArtifacts && previousSignature !== nextSignature) resetPlanState();
      renderStatus();
      renderCurrent();
      renderLiveSync();
      renderPlan();
      renderTarget();
      els.statusMessage.textContent = reason === 'auto'
        ? '个人课表自动刷新完成。'
        : reason === 'execution-success'
          ? '检测到选课或退课成功，个人课表已刷新。'
          : '个人课表已从主站刷新。';
      return true;
    } catch (error) {
      state.refreshFailureAt = attemptAt;
      state.refreshError = String(error.message || error);
      state.liveRefreshFailureStreak = Math.min((state.liveRefreshFailureStreak || 0) + 1, 12);
      if (error.payload?.status) state.status = error.payload.status;
      renderStatus();
      renderCurrent();
      renderLiveSync();
      els.statusMessage.textContent = reason === 'auto'
        ? `个人课表自动刷新失败：${state.refreshError}`
        : reason === 'execution-success'
          ? `选课或退课成功后的个人课表刷新失败：${state.refreshError}`
          : `个人课表刷新失败：${state.refreshError}`;
      return false;
    } finally {
      state.liveRefreshInFlight = false;
      state.liveRefreshPromise = null;
      scheduleLiveRefresh();
      renderLiveSync();
      renderWorkflow();
    }
  })();
  state.liveRefreshPromise = promise;
  return promise;
}

async function persistRefreshSettings() {
  const previous = {
    autoRefresh: state.autoRefresh,
    refreshIntervalSeconds: state.refreshIntervalSeconds,
  };
  state.autoRefresh = els.autoRefresh.checked;
  state.refreshIntervalSeconds = normalizeRefreshInterval(els.refreshInterval.value);
  els.refreshInterval.value = state.refreshIntervalSeconds;
  scheduleLiveRefresh();
  try {
    const result = await fetchJSON('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        schedulerDir: state.settings?.schedulerDir || '',
        killCourseDir: state.settings?.killCourseDir || '',
        mainBaseURL: state.settings?.mainBaseURL || '',
        autoRefresh: state.autoRefresh,
        refreshIntervalSeconds: state.refreshIntervalSeconds,
      }),
    });
    if (!result.ok) throw new Error(result.error || '保存刷新设置失败');
    state.settings = result.settings || state.settings;
    state.status = result.status || state.status;
    applyRefreshSettings(state.settings);
    renderStatus();
    renderLiveSync();
    els.statusMessage.textContent = '刷新设置已保存。';
  } catch (error) {
    state.autoRefresh = previous.autoRefresh;
    state.refreshIntervalSeconds = previous.refreshIntervalSeconds;
    scheduleLiveRefresh();
    renderLiveSync();
    els.statusMessage.textContent = String(error.message || error);
  }
}

async function refreshStatus() {
  const settingsResp = await fetchJSON('/api/settings').catch(() => null);
  if (settingsResp?.ok) {
    state.settings = settingsResp.settings || {};
    state.status = settingsResp.status || null;
    applyRefreshSettings(state.settings);
  }
  if (!state.status) state.status = await fetchJSON('/api/status');
  const target = await fetchJSON('/api/target-schedule').catch(() => null);
  if (target?.ok) {
    if (target.status) state.status = target.status;
    if (!state.targetManualOverride) {
      const nextItems = target.exists ? (target.items || []) : [];
      const nextPayload = target.exists
        ? (target.payload?.items ? target.payload : { items: nextItems })
        : null;
      const previousSignature = courseItemSignature(state.targetPayload?.items || []);
      const nextSignature = courseItemSignature(nextItems);
      state.targetPayload = nextPayload;
      state.targetPath = target.path || '';
      state.targetUpdatedAt = target.updatedAt || '';
      state.targetWarnings = target.warnings || [];
      if (previousSignature !== nextSignature && (state.plan || state.generatedConfig || state.executionPackage || state.executionLog)) {
        resetPlanState();
      }
    }
  }
  const current = await fetchJSON('/api/current').catch((error) => {
    state.currentLoadError = String(error.message || error);
    return null;
  });
  if (current) {
    state.current = current.items || [];
    state.currentLoadError = '';
  }
  const live = await fetchJSON('/api/live-schedule').catch(() => null);
  if (live?.ok) {
    state.liveSchedule = live.sync || null;
    state.liveItems = live.items || [];
    state.liveSnapshotAvailable = live.hasSnapshot === true;
    if (live.sync?.syncedAt && state.liveSnapshotAvailable) state.lastRefreshAt = live.sync.syncedAt;
    if (live.status) state.status = live.status;
  }
  const options = await fetchJSON('/api/course-options').catch(() => null);
  if (options?.ok) {
    state.courseOptions = options.items || [];
    state.courseOptionsWarnings = options.warnings || [];
    state.courseCurrentRound = options.currentRound ?? null;
  } else {
    state.courseOptions = [];
    state.courseOptionsWarnings = options?.error ? [options.error] : [];
    state.courseCurrentRound = null;
  }
  const capacity = await fetchJSON('/api/course-capacity').catch(() => null);
  if (capacity?.ok) {
    state.courseCapacity = capacity.items || [];
    state.courseCapacitySource = capacity.source || '';
    state.courseCapacityObservedAt = capacity.observedAt || '';
    state.courseCapacitySourceUpdatedAt = capacity.sourceUpdatedAt || '';
    state.courseCapacityStale = capacity.stale === true;
  } else {
    state.courseCapacity = [];
    state.courseCapacitySource = '';
    state.courseCapacityObservedAt = '';
    state.courseCapacitySourceUpdatedAt = '';
    state.courseCapacityStale = false;
  }
  const classOptions = await fetchJSON('/api/class-options').catch(() => null);
  if (classOptions?.ok) {
    state.classOptions = classOptions.items || [];
    state.classOptionsWarnings = classOptions.warnings || [];
  } else {
    state.classOptions = [];
    state.classOptionsWarnings = [String(classOptions?.error || '读取行政班列表失败')];
  }
  state.courseSchedule = null;
  state.classSchedule = null;
  renderStatus();
  renderTarget();
  renderCurrent();
  renderLiveSync();
  renderCourseIntel();
  renderClassOptions();
  renderWorkflow();
  await maybeStartAutoRefresh();
}

async function queryCourseSchedule() {
  const item = state.courseOptions
    .map(normalizeCourseOption)
    .find((candidate) => candidate.displayCode === state.courseInspectorCode);
  if (!item) return;
  els.courseScheduleQuery.disabled = true;
  els.courseScheduleQuery.textContent = '正在查询...';
  try {
    const params = new URLSearchParams();
    if (item.groupId) params.set('groupId', item.groupId);
    else params.set('displayCode', item.displayCode);
    const result = await fetchJSON(`/api/class-schedule?${params.toString()}`);
    if (!result.ok) throw new Error(result.error || '读取教学班课表失败');
    state.courseSchedule = result;
    renderCourseSchedule();
  } catch (error) {
    state.courseSchedule = { items: [], warnings: [String(error.message || error)] };
    renderCourseSchedule();
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.courseScheduleQuery.disabled = !item;
    els.courseScheduleQuery.textContent = '查看教学班课表';
  }
}

async function saveSettings() {
  els.saveSettings.disabled = true;
  try {
    const result = await fetchJSON('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        schedulerDir: els.schedulerDir.value.trim(),
        killCourseDir: els.killCourseDir.value.trim(),
        mainBaseURL: state.settings?.mainBaseURL || '',
        autoRefresh: state.autoRefresh,
        refreshIntervalSeconds: state.refreshIntervalSeconds,
      }),
    });
    if (!result.ok) throw new Error(result.error || '保存路径失败');
    state.settings = result.settings || {};
    state.status = result.status || null;
    applyRefreshSettings(state.settings);
    state.targetManualOverride = false;
    state.targetFileName = '';
    resetPlanState();
    await refreshStatus();
    renderTarget();
    renderPlan();
    scheduleLiveRefresh();
    els.statusMessage.textContent = '项目路径已保存。';
    setSettingsDrawer(false);
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.saveSettings.disabled = false;
  }
}

async function clearSettings() {
  els.clearSettings.disabled = true;
  try {
    const result = await fetchJSON('/api/settings', { method: 'DELETE' });
    if (!result.ok) throw new Error(result.error || '恢复自动检测失败');
    state.settings = {};
    state.status = result.status || null;
    applyRefreshSettings(result.settings || {});
    state.targetManualOverride = false;
    state.targetFileName = '';
    resetPlanState();
    await refreshStatus();
    renderTarget();
    renderPlan();
    scheduleLiveRefresh();
    els.statusMessage.textContent = '已恢复自动检测项目路径。';
    setSettingsDrawer(false);
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.clearSettings.disabled = false;
  }
}

async function readTargetFile(file) {
  const text = await file.text();
  const payload = JSON.parse(text);
  if (!Array.isArray(payload.items)) throw new Error('目标课表 JSON 缺少 items 数组');
  if (!payload.items.length) throw new Error('目标课表 JSON 的 items 不能为空');
  state.targetPayload = payload;
  state.targetManualOverride = true;
  state.targetPath = '';
  state.targetFileName = file.name || '';
  state.targetUpdatedAt = file.lastModified ? new Date(file.lastModified).toISOString() : '';
  state.targetWarnings = [];
  resetPlanState();
  renderTarget();
  renderPlan();
}

async function readLiveFile(file) {
  const previousSignature = hasLiveSnapshot()
    ? liveScheduleSignature(state.liveItems, state.liveSchedule)
    : liveScheduleSignature(state.current, null);
  const hadPlanArtifacts = Boolean(state.plan || state.generatedConfig || state.executionPackage || state.executionLog);
  const text = await file.text();
  const payload = JSON.parse(text);
  if (!Array.isArray(payload.items)) throw new Error('真实课表 JSON 缺少 items 数组');
  const result = await fetchJSON('/api/live-schedule', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify({ payload }),
  });
  if (!result.ok) throw new Error(result.error || '导入真实课表失败');
  state.liveSchedule = result.sync || null;
  state.liveItems = result.items || [];
  state.liveSnapshotAvailable = result.hasSnapshot === true;
  state.status = result.status || state.status;
  state.lastRefreshAt = result.sync?.syncedAt || new Date().toISOString();
  state.refreshFailureAt = '';
  state.refreshError = '';
  state.liveImportFailureAt = '';
  state.liveImportError = '';
  state.liveRefreshFailureStreak = 0;
  const nextSignature = liveScheduleSignature(state.liveItems, state.liveSchedule);
  if (hadPlanArtifacts && previousSignature !== nextSignature) resetPlanState();
  renderStatus();
  renderCurrent();
  renderLiveSync();
  renderPlan();
  scheduleLiveRefresh();
  els.statusMessage.textContent = '真实课表快照已导入，请重新生成执行计划。';
}

async function generatePlan({ stayOnCurrentStage = false } = {}) {
  if (!state.targetPayload) return false;
  els.stagePrimary.disabled = true;
  try {
    const result = await fetchJSON('/api/plan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        targetPayload: state.targetPayload,
        lockedCodes: [...state.lockedCodes],
        writeActionPlan: els.writeAction.checked,
        writeKillCourseConfig: els.writeConfig.checked,
      }),
    });
    if (!result.ok) throw new Error(result.error || '生成失败');
    state.plan = result.plan;
    state.generatedConfig = result.generatedConfig || null;
    state.generatedConfigWritten = Boolean(result.configPath);
    state.configPreview = result.configPreview || null;
    state.readiness = result.readiness || null;
    state.dryRun = null;
    state.authorization = null;
    state.executionPackage = null;
    state.executionLog = null;
    state.fallbackRecommendations = null;
    renderPlan();
    if (!stayOnCurrentStage) setActiveStage(2);
    const parts = [];
    if (result.configBlocked) {
      parts.push(`执行配置被 ${result.blockers?.length || 0} 项校验问题阻断，请查看计划详情。`);
    }
    if (result.actionPlanPath) parts.push(`已写入 ${result.actionPlanPath}`);
    if (result.configPath) parts.push(`已写入 ${result.configPath}`);
    if (result.warnings?.length) parts.push(`提示：${result.warnings.join('；')}`);
    els.statusMessage.textContent = parts.join('；') || '执行计划已生成。';
    return true;
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
    return false;
  } finally {
    renderWorkflow();
  }
}

async function runDryRun() {
  if (!state.plan) return;
  if (!els.writeConfig.checked) {
    els.statusMessage.textContent = '请先确认写入 KillCourse/config.json，dry-run 需要基于磁盘中的执行配置。';
    els.writeConfig.focus();
    return;
  }
  if (!state.generatedConfigWritten) {
    const generated = await generatePlan({ stayOnCurrentStage: true });
    if (!generated || !state.generatedConfig) return;
  }
  els.dryRun.disabled = true;
  try {
    const result = await fetchJSON('/api/execution/dry-run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        generatedConfig: state.generatedConfig,
      }),
    });
    if (!result.ok) throw new Error(result.error || 'dry-run 失败');
    state.dryRun = result.dryRun;
    state.authorization = null;
    state.executionPackage = null;
    state.executionLog = null;
    state.fallbackRecommendations = null;
    els.confirmPhrase.value = '';
    renderDryRun();
    els.statusMessage.textContent = result.dryRun?.summary || 'Dry-run 检查完成。';
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.dryRun.disabled = !state.plan || !state.generatedConfig || !els.writeConfig.checked;
  }
}

async function prepareExecution() {
  if (!state.plan) return;
  if (!els.writeConfig.checked) {
    els.statusMessage.textContent = '请先确认写入 KillCourse/config.json，安全检查需要基于磁盘中的执行配置。';
    els.writeConfig.focus();
    return;
  }
  const generated = await generatePlan({ stayOnCurrentStage: true });
  if (generated && state.generatedConfig) await runDryRun();
}

async function handleStagePrimary() {
  if (state.activeStage === 1) {
    await generatePlan();
  } else if (state.activeStage === 2) {
    setActiveStage(3);
  } else if (state.activeStage === 3) {
    await prepareExecution();
  } else if (state.executionPackage) {
    await parseExecutionLog();
  }
}

async function authorizeExecution() {
  if (!state.plan || !state.generatedConfig || !state.dryRun?.canExecute) return;
  els.authorizeExecution.disabled = true;
  try {
    const result = await fetchJSON('/api/execution/authorize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        generatedConfig: state.generatedConfig,
        confirmationPhrase: els.confirmPhrase.value.trim(),
      }),
    });
    if (!result.ok) throw new Error(result.error || '生成授权票据失败');
    state.authorization = result.authorization;
    state.executionPackage = null;
    state.executionLog = null;
    state.fallbackRecommendations = null;
    renderAuthorization();
    els.statusMessage.textContent = '执行授权票据已生成。注意：当前版本仍未真实执行选课或退课。';
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.authorizeExecution.disabled = !state.dryRun?.canExecute;
  }
}

async function buildExecutionPackage() {
  if (!state.plan || !state.generatedConfig || !state.authorization) return;
  els.buildPackage.disabled = true;
  try {
    const result = await fetchJSON('/api/execution/package', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        generatedConfig: state.generatedConfig,
        authorization: state.authorization,
      }),
    });
    if (!result.ok) throw new Error(result.error || '生成启动包失败');
    state.executionPackage = result.package;
    state.executionLog = null;
    state.fallbackRecommendations = null;
    renderAuthorization();
    setActiveStage(4);
    els.statusMessage.textContent = '执行启动包已生成。请手动运行 run-killcourse.bat。';
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.buildPackage.disabled = !state.authorization;
  }
}

async function parseExecutionLog() {
  if (!state.plan || !state.generatedConfig) return;
  els.parseLog.disabled = true;
  try {
    const result = await fetchJSON('/api/execution/parse-log', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        generatedConfig: state.generatedConfig,
        writeExecutionLog: els.writeExecutionLog.checked,
      }),
    });
    if (!result.ok) throw new Error(result.error || '解析执行日志失败');
    state.executionLog = result.log;
    state.fallbackRecommendations = null;
    renderExecutionLog();
    renderFallbackRecommendations();
    const suffix = result.path ? `，已写入 ${result.path}` : '';
    const refreshKey = executionSuccessRefreshKey(result.log);
    const shouldRefresh = result.refreshAfterSuccess === true
      && refreshKey !== '[]'
      && refreshKey !== state.lastExecutionRefreshKey;
    if (shouldRefresh) {
      els.statusMessage.textContent = '检测到选课或退课成功，正在刷新个人课表...';
      const refreshed = await refreshLiveSchedule({ reason: 'execution-success' });
      if (refreshed) state.lastExecutionRefreshKey = refreshKey;
    } else {
      if (refreshKey !== '[]') state.lastExecutionRefreshKey = refreshKey;
      els.statusMessage.textContent = `执行日志解析完成${suffix}。`;
    }
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.parseLog.disabled = !state.plan || !state.generatedConfig;
  }
}

function renderInlineExecution() {
  const exec = state.inlineExecution;
  if (!exec) {
    els.inlineExecBadge.textContent = '未执行';
    els.inlineExecState.classList.add('empty');
    els.inlineExecState.textContent = '尚未执行。';
    els.inlineExecList.classList.add('empty');
    els.inlineExecList.innerHTML = '';
    renderExecutionLog();
    return;
  }
  if (exec.active) {
    els.inlineExecBadge.textContent = '执行中';
    els.inlineExecState.classList.remove('empty');
    els.inlineExecState.textContent = `执行中…开始于 ${formatTimestamp(exec.startedAt)}，正在轮询状态。`;
  } else {
    const done = exec.log?.summary;
    els.inlineExecBadge.textContent = done ? `已完成 ${done.success} 成功` : '已结束';
    els.inlineExecState.classList.remove('empty');
    els.inlineExecState.textContent = exec.message || '执行已结束。';
  }
  const log = exec.log;
  if (log && log.items && log.items.length) {
    els.inlineExecList.classList.remove('empty');
    els.inlineExecList.innerHTML = log.items.map((item) => `
      <div class="course-row ${item.status === 'failed' ? 'danger' : ''}">
        <div><strong>${escapeHtml(item.courseCode || '')}</strong> · ${escapeHtml(inlineActionLabel(item.action))}</div>
        <div>状态：${escapeHtml(inlineStatusLabel(item.status))}${item.message ? ' · ' + escapeHtml(item.message) : ''}</div>
      </div>`).join('');
  } else {
    els.inlineExecList.classList.add('empty');
    els.inlineExecList.innerHTML = '';
  }
  renderExecutionLog();
}

function inlineActionLabel(action) {
  if (action === 'select') return '选课';
  if (action === 'drop') return '退课';
  if (action === 'wait') return '蹲课';
  if (action === 'unknown') return '未知';
  return action || '-';
}

function inlineStatusLabel(status) {
  const labels = { pending: '排队中', running: '进行中', success: '成功', failed: '失败', skipped: '已跳过' };
  return labels[status] || status || '-';
}

function startInlinePolling() {
  stopInlinePolling();
  state.inlineExecutionPolling = true;
  const poll = async () => {
    try {
      const status = await fetchJSON('/api/execution/status');
      if (!status.ok) throw new Error(status.error || '查询执行状态失败');
      if (!state.inlineExecution) state.inlineExecution = { active: false, log: null, message: '' };
      state.inlineExecution.log = status.log || null;
      if (status.active) {
        state.inlineExecution.active = true;
        state.inlineExecution.startedAt = status.startedAt || state.inlineExecution.startedAt;
      } else {
        state.inlineExecution.active = false;
        if (state.inlineExecution.message !== '已请求停止执行。' && !status.log) {
          state.inlineExecution.message = '执行已结束。';
        }
        stopInlinePolling();
        if (state.inlineExecution.message !== '已请求停止执行。' && !status.log) {
          state.inlineExecution.message = '执行完成，但尚未生成日志。';
        }
        if (status.log) {
          state.executionLog = status.log;
          state.inlineExecution.message = status.log.summary
            ? `执行完成：成功 ${status.log.summary.success}，失败 ${status.log.summary.failed}，跳过 ${status.log.summary.skipped}。`
            : '执行完成。';
        }
        refreshAfterInlineExecution(status.log);
      }
      renderInlineExecution();
      renderAuthorization();
    } catch (error) {
      els.statusMessage.textContent = `轮询执行状态失败：${String(error.message || error)}`;
    }
  };
  poll();
  state.inlineExecutionTimer = setInterval(poll, 1500);
}

function stopInlinePolling() {
  if (state.inlineExecutionTimer) {
    clearInterval(state.inlineExecutionTimer);
    state.inlineExecutionTimer = null;
  }
  state.inlineExecutionPolling = false;
}

async function startInlineExecution() {
  if (!state.plan || !state.generatedConfig || !state.authorization) return;
  const waitEnabled = els.waitEnabled.checked;
  try {
    const result = await fetchJSON('/api/execution/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        generatedConfig: state.generatedConfig,
        authorization: state.authorization,
        waitEnabled,
      }),
    });
    if (!result.ok) throw new Error(result.error || '启动执行失败');
    state.executionPackage = null;
    state.executionLog = null;
    state.lastExecutionRefreshKey = '';
    state.inlineExecution = {
      active: true,
      ticketId: result.ticketId || state.authorization.ticketId,
      startedAt: new Date().toISOString(),
      log: null,
      message: '',
    };
    els.statusMessage.textContent = (waitEnabled ? '蹲课模式已启动' : '一键执行已启动') + '，正在执行…';
    startInlinePolling();
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    renderAuthorization();
  }
}

async function stopInlineExecution() {
  try {
    const result = await fetchJSON('/api/execution/stop', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: '{}',
    });
    if (!result.ok) throw new Error(result.error || '停止失败');
    if (state.inlineExecution) {
      state.inlineExecution.message = '已请求停止执行。';
    }
    els.statusMessage.textContent = '已请求停止执行，等待任务收尾…';
    startInlinePolling();
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
    renderAuthorization();
  }
}

function refreshAfterInlineExecution(log) {
  if (!log || !log.summary || log.summary.success === 0) return;
  const key = executionSuccessRefreshKey(log);
  if (!key || key === '[]' || key === state.lastExecutionRefreshKey) return;
  state.lastExecutionRefreshKey = key;
  els.statusMessage.textContent = '检测到选课或退课成功，正在刷新个人课表...';
  refreshLiveSchedule({ reason: 'execution-success' });
}

async function buildFallbackRecommendations() {
  if (!state.plan || !state.executionLog) return;
  els.buildFallbackRecommendations.disabled = true;
  try {
    const result = await fetchJSON('/api/execution/fallback-recommendations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json; charset=utf-8' },
      body: JSON.stringify({
        plan: state.plan,
        executionLog: state.executionLog,
        writeFallbackRecommendations: els.writeFallbackRecommendations.checked,
      }),
    });
    if (!result.ok) throw new Error(result.error || '生成备选推荐失败');
    state.fallbackRecommendations = result.recommendations;
    renderFallbackRecommendations();
    const suffix = result.path ? `，已写入 ${result.path}` : '';
    els.statusMessage.textContent = `备选推荐已生成${suffix}。`;
  } catch (error) {
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.buildFallbackRecommendations.disabled = !state.plan || !state.executionLog;
  }
}

function downloadPlan() {
  if (!state.plan) return;
  const blob = new Blob([JSON.stringify(state.plan, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'action-plan.json';
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function downloadConfig() {
  if (!state.generatedConfig) return;
  const blob = new Blob([JSON.stringify(state.generatedConfig, null, 2)], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = 'config.json';
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

els.refresh.addEventListener('click', refreshStatus);
els.refreshLive.addEventListener('click', () => refreshLiveSchedule({ reason: 'manual' }));
els.autoRefresh.addEventListener('change', persistRefreshSettings);
els.refreshInterval.addEventListener('change', persistRefreshSettings);
els.courseOptionFilter.addEventListener('input', renderCourseIntel);
els.courseOptionSelect.addEventListener('change', () => {
  state.courseInspectorCode = els.courseOptionSelect.value;
  state.courseSchedule = null;
  renderCourseIntel();
});
els.courseScheduleQuery.addEventListener('click', queryCourseSchedule);
els.classOptionSelect.addEventListener('change', () => {
  state.classInspectorName = els.classOptionSelect.value;
  state.classSchedule = null;
  els.classScheduleQuery.disabled = !state.classInspectorName;
  renderAdminClassSchedule();
});
els.classScheduleQuery.addEventListener('click', queryClassSchedule);
els.settingsToggle.addEventListener('click', () => setSettingsDrawer(true));
els.settingsClose.addEventListener('click', () => setSettingsDrawer(false));
els.settingsDrawer.addEventListener('keydown', handleSettingsKeydown);
els.settingsDrawer.querySelectorAll('[data-close-settings]').forEach((element) => {
  element.addEventListener('click', () => setSettingsDrawer(false));
});
els.stageTabs.forEach((tab) => {
  tab.addEventListener('click', () => setActiveStage(tab.dataset.stage));
  tab.addEventListener('keydown', handleStageKeydown);
});
els.saveSettings.addEventListener('click', saveSettings);
els.clearSettings.addEventListener('click', clearSettings);
els.targetFile.addEventListener('change', async () => {
  const file = els.targetFile.files?.[0];
  if (!file) return;
  try {
    await readTargetFile(file);
    els.targetFile.value = '';
  } catch (error) {
    state.targetWarnings = [String(error.message || error)];
    renderTarget();
    els.statusMessage.textContent = String(error.message || error);
    els.targetFile.value = '';
  }
});
els.liveFile.addEventListener('change', async () => {
  const file = els.liveFile.files?.[0];
  if (!file) return;
  try {
    await readLiveFile(file);
  } catch (error) {
    state.liveImportFailureAt = new Date().toISOString();
    state.liveImportError = String(error.message || error);
    renderRefreshControls();
    els.statusMessage.textContent = String(error.message || error);
  } finally {
    els.liveFile.value = '';
  }
});
els.stagePrimary.addEventListener('click', handleStagePrimary);
els.dryRun.addEventListener('click', prepareExecution);
els.writeConfig.addEventListener('change', () => {
  renderDryRun();
  renderWorkflow();
});
els.authorizeExecution.addEventListener('click', authorizeExecution);
els.buildPackage.addEventListener('click', buildExecutionPackage);
els.parseLog.addEventListener('click', parseExecutionLog);
els.inlineExecStart.addEventListener('click', startInlineExecution);
els.inlineExecStop.addEventListener('click', stopInlineExecution);
els.buildFallbackRecommendations.addEventListener('click', buildFallbackRecommendations);
els.downloadPlan.addEventListener('click', downloadPlan);
els.downloadConfig.addEventListener('click', downloadConfig);

refreshStatus().catch((error) => {
  els.statusMessage.textContent = String(error.message || error);
});
renderTarget();
renderLiveSync();
renderPlan();
renderWorkflow();
