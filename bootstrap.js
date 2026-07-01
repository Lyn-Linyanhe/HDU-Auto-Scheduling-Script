(() => {
  const els = {
    statusPill: document.getElementById('status-pill'),
    statusText: document.getElementById('status-text'),
    refreshStatus: document.getElementById('refresh-status'),
    copyLoginNote: document.getElementById('copy-login-note'),
    skipIfReady: document.getElementById('skip-if-ready'),
    skipIfReadyBottom: document.getElementById('skip-if-ready-bottom'),
    dropzone: document.getElementById('dropzone'),
    fileInput: document.getElementById('file-input'),
    rawJson: document.getElementById('raw-json'),
    pasteSave: document.getElementById('paste-save'),
  };

  function setStatus(kind, text) {
    els.statusPill.textContent = kind;
    els.statusText.textContent = text;
    els.statusPill.classList.toggle('is-ready', kind === '可用');
    els.statusPill.classList.toggle('is-error', kind === '不可用');
  }

  function goScheduler() {
    location.replace('/scheduler.html?v=ui2');
  }

  async function checkStatus({ redirect = false } = {}) {
    try {
      const data = await HDU.fetchJSON(HDU.STATUS_API);
      if (data.ready) {
        const name = data.courseName ? `，示例课程：${data.courseName}` : '';
        setStatus('可用', `已读取 ${data.count || 0} 条课程数据${name}。`);
        if (redirect) setTimeout(goScheduler, 350);
        return true;
      }
      setStatus('不可用', data.message || '当前目录没有可用的 course.json。');
      return false;
    } catch (err) {
      setStatus('不可用', err.message || '无法检查本地课程数据。');
      return false;
    }
  }

  async function importText(text) {
    const raw = String(text || '').trim();
    if (!raw) {
      setStatus('不可用', '请先选择 course.json，或粘贴文件内容。');
      return;
    }

    try {
      JSON.parse(raw);
    } catch {
      setStatus('不可用', 'JSON 格式不正确，请检查文件内容。');
      return;
    }

    try {
      const response = await fetch('/api/bootstrap/import', {
        method: 'POST',
        cache: 'no-store',
        headers: { 'Content-Type': 'application/json; charset=utf-8' },
        body: raw,
      });
      const body = await response.text();
      if (!response.ok) throw new Error(body || '导入失败');
      const result = body ? JSON.parse(body) : {};
      setStatus('可用', `导入完成：${result.count || 0} 条课程数据，正在进入排课页。`);
      setTimeout(goScheduler, 450);
    } catch (err) {
      setStatus('不可用', err.message || '保存 course.json 失败。');
    }
  }

  async function importFile(file) {
    if (!file) return;
    const text = await file.text();
    els.rawJson.value = text;
    await importText(text);
  }

  async function copyNote() {
    const note = [
      '1. 运行 hdu-course-exporter.exe。',
      '2. 在浏览器页面登录并导出 course.json。',
      '3. 把 course.json 放到 hdu-offline-scheduler.exe 同目录，或在当前页面拖拽导入。',
    ].join('\n');
    try {
      await navigator.clipboard.writeText(note);
      setStatus('提示', '说明已复制到剪贴板。');
    } catch {
      setStatus('提示', note);
    }
  }

  function wireEvents() {
    els.refreshStatus.addEventListener('click', () => checkStatus({ redirect: false }));
    els.skipIfReady.addEventListener('click', () => checkStatus({ redirect: true }));
    els.skipIfReadyBottom.addEventListener('click', () => checkStatus({ redirect: true }));
    els.copyLoginNote.addEventListener('click', copyNote);
    els.pasteSave.addEventListener('click', () => importText(els.rawJson.value));
    els.fileInput.addEventListener('change', () => importFile(els.fileInput.files[0]));

    ['dragenter', 'dragover'].forEach((eventName) => {
      els.dropzone.addEventListener(eventName, (event) => {
        event.preventDefault();
        els.dropzone.classList.add('dragover');
      });
    });
    ['dragleave', 'drop'].forEach((eventName) => {
      els.dropzone.addEventListener(eventName, (event) => {
        event.preventDefault();
        els.dropzone.classList.remove('dragover');
      });
    });
    els.dropzone.addEventListener('drop', (event) => {
      const file = event.dataTransfer.files[0];
      importFile(file);
    });
  }

  wireEvents();
  checkStatus({ redirect: true });
})();
