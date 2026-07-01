const els = {
  username: document.getElementById('username'),
  password: document.getElementById('password'),
  xueNian: document.getElementById('xue-nian'),
  xueQi: document.getElementById('xue-qi'),
  export: document.getElementById('export'),
  status: document.getElementById('status'),
  message: document.getElementById('message'),
};

async function postJSON(url, body) {
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
  els.message.textContent = text;
  els.message.className = `status-box ${type}`;
}

els.export.addEventListener('click', async () => {
  const payload = {
    method: 'password',
    username: els.username.value.trim(),
    password: els.password.value.trim(),
    xueNian: els.xueNian.value.trim(),
    xueQi: els.xueQi.value,
  };
  if (!payload.username || !payload.password) {
    setMessage('请先填写账号和密码。', 'error');
    return;
  }
  els.export.disabled = true;
  setMessage('正在登录并导出课程数据，学校接口可能需要等待一会儿。');
  try {
    const result = await postJSON('/api/export', payload);
    if (result.ok) {
      setMessage(`导出完成：${result.count || 0} 条课程数据，文件 ${result.fileName || 'course.json'} 已生成。`, 'success');
    } else {
      setMessage(result.error || '导出失败。', 'error');
    }
  } catch (error) {
    setMessage(String(error.message || error), 'error');
  } finally {
    els.export.disabled = false;
  }
});

els.status.addEventListener('click', async () => {
  try {
    const status = await fetchJSON('/api/export/status');
    setMessage(status.message || (status.ready ? '已就绪。' : '空闲。'), status.ready ? 'success' : '');
  } catch (error) {
    setMessage(String(error.message || error), 'error');
  }
});
