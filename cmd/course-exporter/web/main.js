const els = {
  form: document.getElementById('export-form'),
  username: document.getElementById('username'),
  password: document.getElementById('password'),
  xueNian: document.getElementById('xue-nian'),
  xueQi: document.getElementById('xue-qi'),
  export: document.getElementById('export'),
  refresh: document.getElementById('refresh'),
  message: document.getElementById('message'),
  updatedAt: document.getElementById('updated-at'),
  steps: document.getElementById('steps'),
  courseCount: document.getElementById('course-count'),
  personalCount: document.getElementById('personal-count'),
  fileName: document.getElementById('file-name'),
  personalFileName: document.getElementById('personal-file-name'),
  outputPath: document.getElementById('output-path'),
  personalOutputPath: document.getElementById('personal-output-path'),
  copyPath: document.getElementById('copy-path'),
  openOutput: document.getElementById('open-output'),
};

let polling = 0;

async function postJSON(url, body = {}) {
  const response = await fetch(url, {
    method: 'POST',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json; charset=utf-8' },
    body: JSON.stringify(body),
  });
  const text = await response.text();
  if (!response.ok) throw new Error(text);
  return text ? JSON.parse(text) : {};
}

async function fetchJSON(url) {
  const response = await fetch(url, { cache: 'no-store' });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

function setMessage(text, type = '') {
  els.message.textContent = text || '等待导出课程数据。';
  els.message.className = `status-box ${type}`.trim();
}

function setStep(step, phase) {
  const order = ['validate', 'login', 'query', 'personal', 'done'];
  const currentIndex = Math.max(0, order.indexOf(step));
  for (const item of els.steps.querySelectorAll('li')) {
    const index = order.indexOf(item.dataset.step);
    item.classList.toggle('done', phase === 'success' || index < currentIndex);
    item.classList.toggle('active', index === currentIndex && phase !== 'success' && phase !== 'error');
    item.classList.toggle('error', index === currentIndex && phase === 'error');
  }
}

function renderStatus(status = {}) {
  const phase = status.phase || (status.ready ? 'success' : 'idle');
  const step = status.step || 'validate';
  const type = phase === 'success' ? 'success' : phase === 'error' ? 'error' : '';

  setStep(step, phase);
  setMessage(status.error || status.message, type);
  els.updatedAt.textContent = status.updatedAt ? `更新于 ${formatTime(status.updatedAt)}` : '等待操作';
  els.courseCount.textContent = status.count ? String(status.count) : '-';
  els.personalCount.textContent = status.personalExported ? String(status.personalCount || 0) : '-';
  els.fileName.textContent = status.fileName || '-';
  els.personalFileName.textContent = status.personalFileName || (status.personalExportError ? '导出失败' : '-');
  els.outputPath.value = status.outputPath || '';
  els.personalOutputPath.value = status.personalOutputPath || '';
  const hasPath = Boolean(status.outputPath || status.personalOutputPath);
  els.copyPath.disabled = !hasPath;
  els.openOutput.disabled = !hasPath;
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

function validatePayload(payload) {
  if (!payload.username) return '请先填写学号或工号。';
  if (!payload.password) return '请先填写密码。';
  if (!/^\d{4}$/.test(payload.xueNian)) return '学年起始年份应为 4 位数字，例如 2026。';
  return '';
}

function startPolling() {
  stopPolling();
  polling = window.setInterval(refreshStatus, 1200);
}

function stopPolling() {
  if (polling) {
    window.clearInterval(polling);
    polling = 0;
  }
}

async function refreshStatus() {
  const status = await fetchJSON('/api/export/status');
  renderStatus(status);
  if (status.phase === 'success' || status.phase === 'error') {
    stopPolling();
    els.export.disabled = false;
  }
  return status;
}

els.form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const payload = {
    method: 'password',
    username: els.username.value.trim(),
    password: els.password.value.trim(),
    xueNian: els.xueNian.value.trim(),
    xueQi: els.xueQi.value,
  };
  const validation = validatePayload(payload);
  if (validation) {
    setStep('validate', 'error');
    setMessage(validation, 'error');
    return;
  }

  els.export.disabled = true;
  setStep('validate', 'validating');
  setMessage('正在提交导出任务，请稍候。');
  startPolling();
  try {
    const result = await postJSON('/api/export', payload);
    if (result.status) renderStatus(result.status);
    if (result.ok) {
      const personalText = result.personalExported
        ? `个人课表 ${result.personalCount || 0} 门。`
        : `个人课表未导出：${result.personalExportError || '未知原因'}。`;
      setMessage(`导出完成：全校教学班 ${result.count || 0} 个，${personalText}`, result.personalExported ? 'success' : '');
    } else {
      setMessage(result.error || '导出失败。', 'error');
    }
  } catch (error) {
    setStep('login', 'error');
    setMessage(String(error.message || error), 'error');
  } finally {
    stopPolling();
    els.export.disabled = false;
    await refreshStatus().catch(() => {});
  }
});

els.refresh.addEventListener('click', async () => {
  try {
    await refreshStatus();
  } catch (error) {
    setMessage(String(error.message || error), 'error');
  }
});

els.copyPath.addEventListener('click', async () => {
  const paths = [els.outputPath.value, els.personalOutputPath.value].filter(Boolean);
  if (!paths.length) return;
  await navigator.clipboard.writeText(paths.join('\n'));
  setMessage('输出路径已复制。', 'success');
});

els.openOutput.addEventListener('click', async () => {
  try {
    const result = await postJSON('/api/export/open-output');
    if (result.ok) {
      setMessage('已打开 JSON 文件所在目录。', 'success');
    } else {
      setMessage(result.error || '打开目录失败。', 'error');
    }
  } catch (error) {
    setMessage(String(error.message || error), 'error');
  }
});

refreshStatus().catch(() => {});
