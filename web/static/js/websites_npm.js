// ========== Node.js 项目管理 ==========
let npmProjects = [];
let npmPollTimer = null;
let currentNpmLogId = null;
let npmStatusCache = {};

let npmCurrentBrowsePath = '';
let npmSelectedBrowsePath = '';
let npmSelectedBrowseIsDir = true;
let npmPathBrowserTarget = 'npmPath';

const npmIcons = {
  edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
  play: '<svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><polygon points="5 3 19 12 5 21 5 3"/></svg>',
  stop: '<svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>',
  logs: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16"><path d="M13 12h8"/><path d="M13 5h8"/><path d="M13 19h8"/><path d="M3 12h.01"/><path d="M3 5h.01"/><path d="M3 19h.01"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
  terminal: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>'
};

function npmEscapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text == null ? '' : text;
  return div.innerHTML;
}

async function loadNpmProjectsData() {
  await loadNpmProjects();
  if (npmPollTimer) clearInterval(npmPollTimer);
  npmPollTimer = setInterval(() => {
    npmProjects.forEach(p => checkNpmStatus(p.id));
  }, 3000);
}

function stopNpmPolling() {
  if (npmPollTimer) {
    clearInterval(npmPollTimer);
    npmPollTimer = null;
  }
}

async function loadNpmProjects() {
  try {
    const response = await fetch('/api/npm-projects');
    if (response.ok) {
      npmProjects = await response.json();
      renderNpmProjects();
    }
  } catch (e) {
    console.log('加载 Node 项目列表失败', e);
  }
}

function getNpmStatus(id) {
  return npmStatusCache[id] || { running: false, pid: 0 };
}

function renderNpmProjects() {
  const tableContainer = document.getElementById('npmList');
  const tableBody = document.getElementById('npmTableBody');
  const empty = document.getElementById('npmEmpty');
  if (!tableContainer || !tableBody || !empty) return;

  if (npmProjects.length === 0) {
    tableContainer.style.display = 'none';
    empty.style.display = 'block';
    return;
  }

  tableContainer.style.display = 'block';
  empty.style.display = 'none';
  tableBody.innerHTML = npmProjects.map(p => {
    const status = getNpmStatus(p.id);
    const statusClass = status.running ? 'running' : 'stopped';
    const statusText = status.running ? '运行中' : '已停止';
    const pidText = status.running && status.pid ? ` (PID: ${status.pid})` : '';
    const startStopBtn = status.running
      ? `<button type="button" class="website-btn website-btn-danger" onclick="stopNpmProject('${p.id}')" title="停止">${npmIcons.stop}</button>`
      : `<button type="button" class="website-btn" onclick="startNpmProject('${p.id}')" title="启动">${npmIcons.play}</button>`;
    return `
      <div class="website-table-row" data-id="${p.id}">
        <span class="website-td website-td-name">${npmEscapeHtml(p.name || '未命名')}</span>
        <span class="website-td website-td-status"><span class="website-status ${statusClass}">${statusText}${pidText}</span></span>
        <span class="website-td website-td-port">${p.port || '-'}</span>
        <span class="website-td website-td-path" title="${npmEscapeHtml(p.path || '')}">${npmEscapeHtml(p.path || '-')}</span>
        <span class="website-td website-td-actions">
          <button type="button" class="website-btn" onclick="viewNpmLogs('${p.id}')" title="查看日志">${npmIcons.logs}</button>
          <button type="button" class="website-btn" onclick="openNpmTerminal('${p.id}')" title="进入环境（cd 到工作目录）">${npmIcons.terminal}</button>
          <button type="button" class="website-btn" onclick="editNpmProject('${p.id}')" title="编辑">${npmIcons.edit}</button>
          ${startStopBtn}
          <button type="button" class="website-btn website-btn-danger" onclick="deleteNpmProject('${p.id}')" title="删除">${npmIcons.trash}</button>
        </span>
      </div>
    `;
  }).join('');
}

async function checkNpmStatus(id) {
  try {
    const response = await fetch(`/api/npm-projects/${id}/status`);
    if (response.ok) {
      const status = await response.json();
      npmStatusCache[id] = status;
      const row = document.querySelector(`.website-table-row[data-id="${id}"]`);
      if (row) {
        const statusEl = row.querySelector('.website-status');
        if (statusEl) {
          statusEl.className = `website-status ${status.running ? 'running' : 'stopped'}`;
          statusEl.textContent = status.running ? `运行中 (PID: ${status.pid})` : '已停止';
        }
        const actionsEl = row.querySelector('.website-td-actions');
        if (actionsEl) {
          const startStopBtn = status.running
            ? `<button type="button" class="website-btn website-btn-danger" onclick="stopNpmProject('${id}')" title="停止">${npmIcons.stop}</button>`
            : `<button type="button" class="website-btn" onclick="startNpmProject('${id}')" title="启动">${npmIcons.play}</button>`;
          actionsEl.innerHTML = `
            <button type="button" class="website-btn" onclick="viewNpmLogs('${id}')" title="查看日志">${npmIcons.logs}</button>
            <button type="button" class="website-btn" onclick="openNpmTerminal('${id}')" title="进入环境（cd 到工作目录）">${npmIcons.terminal}</button>
            <button type="button" class="website-btn" onclick="editNpmProject('${id}')" title="编辑">${npmIcons.edit}</button>
            ${startStopBtn}
            <button type="button" class="website-btn website-btn-danger" onclick="deleteNpmProject('${id}')" title="删除">${npmIcons.trash}</button>
          `;
        }
      }
      return status;
    }
  } catch (e) {
    console.log('检查 Node 项目状态失败', e);
  }
  return { running: false, pid: 0 };
}

async function openNpmModal(editId = null) {
  const modal = document.getElementById('npmModal');
  const editIdInput = document.getElementById('npmEditId');
  if (editId) {
    const project = npmProjects.find(p => p.id === editId);
    if (project) {
      editIdInput.value = editId;
      document.getElementById('npmName').value = project.name || '';
      document.getElementById('npmPath').value = project.path || '';
      document.getElementById('npmWorkingDir').value = project.workingDir || '';
      document.getElementById('npmPort').value = project.port || '';
      document.getElementById('npmStartCommand').value = project.startCommand || '';
      document.getElementById('npmAutoStart').checked = project.autoStart || false;
    }
  } else {
    editIdInput.value = '';
    document.getElementById('npmName').value = '';
    document.getElementById('npmPath').value = '';
    document.getElementById('npmWorkingDir').value = '';
    document.getElementById('npmPort').value = '';
    document.getElementById('npmStartCommand').value = '';
    document.getElementById('npmAutoStart').checked = false;
  }
  modal.classList.add('active');
}

function closeNpmModal() {
  const modal = document.getElementById('npmModal');
  if (modal) modal.classList.remove('active');
}

async function saveNpmProject() {
  const editId = document.getElementById('npmEditId').value;
  const name = document.getElementById('npmName').value.trim();
  const path = document.getElementById('npmPath').value.trim();
  const workingDir = document.getElementById('npmWorkingDir').value.trim();
  const port = parseInt(document.getElementById('npmPort').value, 10) || 0;
  const startCommand = document.getElementById('npmStartCommand').value.trim();
  const autoStart = document.getElementById('npmAutoStart').checked;

  if (!name || !path || !startCommand) {
    if (typeof showToast === 'function') showToast('请填写项目名称、路径和启动命令', 'warning');
    return;
  }

  const project = {
    name,
    path,
    workingDir: workingDir || path,
    port,
    startCommand,
    autoStart
  };

  try {
    const url = editId ? `/api/npm-projects/${editId}` : '/api/npm-projects';
    const method = editId ? 'PUT' : 'POST';
    const response = await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(project)
    });
    const result = await response.json();
    if (response.ok) {
      if (typeof showToast === 'function') showToast(editId ? '项目已更新' : '项目已创建', 'success');
      closeNpmModal();
      await loadNpmProjects();
    } else {
      if (typeof showToast === 'function') showToast(result.error || '保存失败', 'error');
    }
  } catch (e) {
    if (typeof showToast === 'function') showToast('保存失败', 'error');
  }
}

function editNpmProject(id) {
  openNpmModal(id);
}

// 进入环境：跳转终端并 cd 到工作目录
function openNpmTerminal(id) {
  const project = npmProjects.find(p => p.id === id);
  if (!project) return;
  closeNpmModal();
  try {
    const workDir = (project.workingDir || project.path || '').trim();
    sessionStorage.setItem('websiteTerminalContext', JSON.stringify({
      path: workDir,
      venvPath: ''
    }));
  } catch (e) {
    console.error('openNpmTerminal error:', e);
  }
  window.location.href = '/terminal';
}

async function deleteNpmProject(id) {
  if (!confirm('确定要删除这个项目吗？')) return;
  try {
    const response = await fetch(`/api/npm-projects/${id}`, { method: 'DELETE' });
    const result = await response.json();
    if (response.ok) {
      if (typeof showToast === 'function') showToast('项目已删除', 'success');
      await loadNpmProjects();
    } else {
      if (typeof showToast === 'function') showToast(result.error || '删除失败', 'error');
    }
  } catch (e) {
    if (typeof showToast === 'function') showToast('删除失败', 'error');
  }
}

async function startNpmProject(id) {
  try {
    const response = await fetch(`/api/npm-projects/${id}/start`, { method: 'POST' });
    const result = await response.json();
    if (response.ok) {
      if (typeof showToast === 'function') showToast('项目已启动', 'success');
      if (result.pid) npmStatusCache[id] = { running: true, pid: result.pid };
      renderNpmProjects();
      setTimeout(() => checkNpmStatus(id), 1800);
      viewNpmLogs(id);
    } else {
      if (typeof showToast === 'function') showToast(result.error || '启动失败', 'error');
    }
  } catch (e) {
    if (typeof showToast === 'function') showToast('启动失败', 'error');
  }
}

async function stopNpmProject(id) {
  try {
    const response = await fetch(`/api/npm-projects/${id}/stop`, { method: 'POST' });
    const result = await response.json();
    if (response.ok) {
      if (typeof showToast === 'function') showToast('项目已停止', 'success');
      npmStatusCache[id] = { running: false, pid: 0 };
      renderNpmProjects();
      setTimeout(() => checkNpmStatus(id), 500);
    } else {
      if (typeof showToast === 'function') showToast(result.error || '停止失败', 'error');
    }
  } catch (e) {
    if (typeof showToast === 'function') showToast('停止失败', 'error');
  }
}

async function refreshAllNpmStatuses(silent = false) {
  if (npmProjects.length === 0) return;
  const btn = document.getElementById('npmRefreshStatusBtn');
  if (!silent && btn) {
    btn.disabled = true;
    btn.classList.add('loading');
  }
  try {
    await Promise.all(npmProjects.map(p => checkNpmStatus(p.id)));
    if (!silent && typeof showToast === 'function') showToast('状态已刷新', 'success');
  } finally {
    if (!silent && btn) {
      btn.disabled = false;
      btn.classList.remove('loading');
    }
  }
}

async function viewNpmLogs(id) {
  const modal = document.getElementById('npmLogsModal');
  currentNpmLogId = id;
  modal.classList.add('active');
  await refreshNpmLogs(id);
  const modalBody = modal.querySelector('.modal-body');
  if (modalBody) modalBody.scrollTop = modalBody.scrollHeight;
}

async function refreshNpmLogs(id) {
  const content = document.getElementById('npmLogsContent');
  if (!content) return;
  try {
    const response = await fetch(`/api/npm-projects/${id}/logs`);
    if (response.ok) {
      const result = await response.json();
      content.textContent = result.logs || '暂无日志';
      content.scrollTop = content.scrollHeight;
    } else {
      content.textContent = '加载日志失败';
    }
  } catch (e) {
    content.textContent = '加载日志失败';
  }
}

function closeNpmLogs() {
  const modal = document.getElementById('npmLogsModal');
  if (modal) modal.classList.remove('active');
  currentNpmLogId = null;
}

// ---------- 路径浏览器 ----------
function openNpmPathBrowser(target) {
  npmPathBrowserTarget = target || 'npmPath';
  npmCurrentBrowsePath = '';
  npmSelectedBrowsePath = '';
  npmSelectedBrowseIsDir = true;
  const modal = document.getElementById('npmPathBrowserModal');
  const confirmBtn = document.getElementById('npmPathBrowserConfirm');
  if (confirmBtn) confirmBtn.textContent = '选择此文件夹';
  modal.classList.add('active');
  loadNpmDirectory('root');
}

function closeNpmPathBrowser() {
  const modal = document.getElementById('npmPathBrowserModal');
  if (modal) modal.classList.remove('active');
  npmCurrentBrowsePath = '';
  npmSelectedBrowsePath = '';
}

function npmEscapePath(path) {
  return (path || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

async function loadNpmDirectory(path) {
  const body = document.getElementById('npmPathBrowserBody');
  const currentPathInput = document.getElementById('npmPathBrowserCurrentPath');
  body.innerHTML = '<div class="path-browser-loading">加载中...</div>';
  npmCurrentBrowsePath = path;
  try {
    const response = await fetch(`/api/websites/browse?path=${encodeURIComponent(path)}`);
    if (response.ok) {
      const data = await response.json();
      const displayPath = data.path === 'root' ? '选择盘符' : data.path;
      currentPathInput.value = displayPath;
      if (data.path !== 'root') {
        npmSelectedBrowsePath = data.path;
        npmSelectedBrowseIsDir = true;
      }
      renderNpmDirectory(data.files || [], data.path);
      updateNpmBreadcrumb(data.path);
    } else {
      const error = await response.json();
      body.innerHTML = `<div class="path-browser-error">加载失败: ${error.error || '未知错误'}</div>`;
    }
  } catch (e) {
    body.innerHTML = `<div class="path-browser-error">加载失败: ${e.message}</div>`;
  }
}

function renderNpmDirectory(files, currentPath) {
  const body = document.getElementById('npmPathBrowserBody');
  if (!body) return;
  if (files.length === 0) {
    body.innerHTML = '<div class="path-browser-empty">文件夹为空</div>';
    return;
  }
  const folderIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
  const fileIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
  body.innerHTML = files.map(file => {
    const icon = file.isDir ? folderIcon : fileIcon;
    const className = file.isDir ? 'path-item path-item-dir' : 'path-item path-item-file';
    return `<div class="${className}" data-path="${npmEscapePath(file.path)}" data-isdir="${file.isDir}">
      <span class="path-item-icon">${icon}</span>
      <span class="path-item-name">${npmEscapeHtml(file.name)}</span>
    </div>`;
  }).join('');

  body.querySelectorAll('.path-item-dir').forEach(item => {
    let clickTimer = null;
    item.addEventListener('click', () => {
      const path = item.dataset.path;
      npmSelectedBrowsePath = path;
      npmSelectedBrowseIsDir = true;
      const currentPathInput = document.getElementById('npmPathBrowserCurrentPath');
      if (currentPathInput) currentPathInput.value = path;
      body.querySelectorAll('.path-item').forEach(i => i.classList.remove('path-item-selected'));
      item.classList.add('path-item-selected');
      if (clickTimer === null) clickTimer = setTimeout(() => { clickTimer = null; }, 300);
    });
    item.addEventListener('dblclick', (e) => {
      e.stopPropagation();
      if (clickTimer) clearTimeout(clickTimer);
      clickTimer = null;
      loadNpmDirectory(item.dataset.path);
    });
  });

  body.querySelectorAll('.path-item-file').forEach(item => {
    item.addEventListener('click', () => {
      if (npmPathBrowserTarget !== 'file') return;
      npmSelectedBrowsePath = item.dataset.path;
      npmSelectedBrowseIsDir = false;
      const currentPathInput = document.getElementById('npmPathBrowserCurrentPath');
      if (currentPathInput) currentPathInput.value = item.dataset.path;
      body.querySelectorAll('.path-item').forEach(i => i.classList.remove('path-item-selected'));
      item.classList.add('path-item-selected');
    });
  });
}

function updateNpmBreadcrumb(path) {
  const breadcrumb = document.getElementById('npmPathBreadcrumb');
  if (!breadcrumb) return;
  if (!path || path === '' || path === 'root') {
    breadcrumb.innerHTML = '<span class="breadcrumb-item active" data-path="root">选择盘符</span>';
    return;
  }
  const isWindows = path.includes('\\');
  const parts = isWindows ? path.split('\\').filter(p => p) : path.split('/').filter(p => p);
  let html = '<span class="breadcrumb-item" data-path="root">选择盘符</span>';
  let currentPath = '';
  if (isWindows && parts.length > 0) {
    const drive = parts[0];
    currentPath = drive + '\\';
    html += `<span class="breadcrumb-sep">\\</span><span class="breadcrumb-item" data-path="${currentPath}">${drive}</span>`;
    for (let i = 1; i < parts.length; i++) {
      currentPath += parts[i] + (i < parts.length - 1 ? '\\' : '');
      const isLast = i === parts.length - 1;
      html += `<span class="breadcrumb-sep">\\</span><span class="breadcrumb-item${isLast ? ' active' : ''}" data-path="${currentPath}">${npmEscapeHtml(parts[i])}</span>`;
    }
  } else {
    html = '<span class="breadcrumb-item" data-path="/">根目录</span>';
    for (let i = 0; i < parts.length; i++) {
      currentPath += '/' + parts[i];
      html += `<span class="breadcrumb-sep">/</span><span class="breadcrumb-item" data-path="${currentPath}">${npmEscapeHtml(parts[i])}</span>`;
    }
  }
  breadcrumb.innerHTML = html;
  breadcrumb.querySelectorAll('.breadcrumb-item').forEach(el => {
    const p = el.dataset.path;
    if (p) el.addEventListener('click', () => loadNpmDirectory(p));
  });
}

function confirmNpmPathSelection() {
  const currentPathInput = document.getElementById('npmPathBrowserCurrentPath');
  const pathToUse = npmSelectedBrowsePath || (currentPathInput ? currentPathInput.value : '');
  if (!pathToUse || pathToUse.trim() === '' || pathToUse === '选择盘符' || pathToUse === 'root') {
    if (typeof showToast === 'function') showToast('请先选择一个文件夹', 'warning');
    return;
  }
  const finalPath = pathToUse.trim();
  if (npmPathBrowserTarget === 'npmPath') {
    document.getElementById('npmPath').value = finalPath;
    const nameEl = document.getElementById('npmName');
    if (nameEl && !nameEl.value.trim()) {
      const parts = finalPath.split(/[/\\]/);
      nameEl.value = parts[parts.length - 1] || '未命名项目';
    }
  } else if (npmPathBrowserTarget === 'npmWorkingDir') {
    document.getElementById('npmWorkingDir').value = finalPath;
  }
  closeNpmPathBrowser();
}

function closeNpmInstallModal() {
  const modal = document.getElementById('npmInstallModal');
  if (modal) modal.classList.remove('active');
}

async function npmInstallEnv() {
  const editId = document.getElementById('npmEditId')?.value?.trim();
  if (!editId) {
    if (typeof showToast === 'function') showToast('请先保存项目后再安装依赖', 'warning');
    return;
  }
  const packageManager = (document.getElementById('npmPackageManager')?.value || 'npm').trim() || 'npm';
  const modal = document.getElementById('npmInstallModal');
  const contentEl = document.getElementById('npmInstallContent');
  const closeBtn = document.getElementById('npmInstallCloseBtn');
  if (modal) modal.classList.add('active');
  if (contentEl) contentEl.textContent = `正在执行 ${packageManager} install...\n`;
  if (closeBtn) {
    closeBtn.disabled = true;
    closeBtn.textContent = '安装完成后可关闭';
  }
  try {
    const response = await fetch(`/api/npm-projects/${editId}/install`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ packageManager })
    });
    if (!response.ok && !response.body) {
      const err = await response.json().catch(() => ({}));
      if (typeof showToast === 'function') showToast(err.error || '请求失败', 'error');
      if (closeBtn) closeBtn.disabled = false;
      return;
    }
    const reader = response.body?.getReader();
    if (!reader) {
      const text = await response.text();
      if (contentEl) contentEl.textContent += text + '\n';
      if (!response.ok && typeof showToast === 'function') showToast('安装失败', 'error');
      if (closeBtn) closeBtn.disabled = false;
      if (closeBtn) closeBtn.textContent = '关闭';
      return;
    }
    const decoder = new TextDecoder();
    let fullText = contentEl?.textContent || '';
    let hasFailed = false;
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      const chunk = decoder.decode(value, { stream: true });
      fullText += chunk;
      if (contentEl) {
        contentEl.textContent = fullText;
        contentEl.scrollTop = contentEl.scrollHeight;
      }
      if (chunk.includes('[INSTALL_FAILED]')) hasFailed = true;
    }
    if (hasFailed || !response.ok) {
      if (typeof showToast === 'function') showToast('安装失败，详见上方日志', 'error');
    } else {
      if (typeof showToast === 'function') showToast('依赖安装成功', 'success');
    }
  } catch (e) {
    if (contentEl) contentEl.textContent += '\n[错误] ' + (e.message || '安装失败');
    if (typeof showToast === 'function') showToast('安装失败', 'error');
  } finally {
    if (closeBtn) {
      closeBtn.disabled = false;
      closeBtn.textContent = '关闭';
    }
  }
}

function initNpmEvents() {
  const addBtn = document.getElementById('npmAddBtn');
  const modal = document.getElementById('npmModal');
  const modalClose = document.getElementById('npmModalClose');
  const modalCancel = document.getElementById('npmModalCancel');
  const modalConfirm = document.getElementById('npmModalConfirm');
  const refreshBtn = document.getElementById('npmRefreshStatusBtn');
  const pathBrowseBtn = document.getElementById('npmPathBrowseBtn');
  const workingDirBrowseBtn = document.getElementById('npmWorkingDirBrowseBtn');

  if (refreshBtn) refreshBtn.addEventListener('click', () => refreshAllNpmStatuses());
  if (addBtn) addBtn.addEventListener('click', () => openNpmModal());
  if (modalClose) modalClose.addEventListener('click', closeNpmModal);
  if (modalCancel) modalCancel.addEventListener('click', closeNpmModal);
  if (modalConfirm) modalConfirm.addEventListener('click', saveNpmProject);
  if (pathBrowseBtn) pathBrowseBtn.addEventListener('click', () => openNpmPathBrowser('npmPath'));
  if (workingDirBrowseBtn) workingDirBrowseBtn.addEventListener('click', () => openNpmPathBrowser('npmWorkingDir'));

  const installEnvBtn = document.getElementById('npmInstallEnvBtn');
  if (installEnvBtn) installEnvBtn.addEventListener('click', npmInstallEnv);
  const npmInstallModalClose = document.getElementById('npmInstallModalClose');
  const npmInstallCloseBtn = document.getElementById('npmInstallCloseBtn');
  if (npmInstallModalClose) npmInstallModalClose.addEventListener('click', closeNpmInstallModal);
  if (npmInstallCloseBtn) npmInstallCloseBtn.addEventListener('click', closeNpmInstallModal);
  const npmInstallModal = document.getElementById('npmInstallModal');
  if (npmInstallModal) npmInstallModal.addEventListener('click', e => { if (e.target === npmInstallModal) closeNpmInstallModal(); });

  const pathBrowserModal = document.getElementById('npmPathBrowserModal');
  const pathBrowserClose = document.getElementById('npmPathBrowserClose');
  const pathBrowserCancel = document.getElementById('npmPathBrowserCancel');
  const pathBrowserConfirm = document.getElementById('npmPathBrowserConfirm');
  if (pathBrowserClose) pathBrowserClose.addEventListener('click', closeNpmPathBrowser);
  if (pathBrowserCancel) pathBrowserCancel.addEventListener('click', closeNpmPathBrowser);
  if (pathBrowserConfirm) pathBrowserConfirm.addEventListener('click', confirmNpmPathSelection);
  if (pathBrowserModal) pathBrowserModal.addEventListener('click', e => { if (e.target === pathBrowserModal) closeNpmPathBrowser(); });

  const logsModal = document.getElementById('npmLogsModal');
  const logsClose = document.getElementById('npmLogsClose');
  const logsCancel = document.getElementById('npmLogsCancel');
  const logsRefresh = document.getElementById('npmLogsRefreshBtn');
  const logsClear = document.getElementById('npmLogsClearBtn');
  if (logsClose) logsClose.addEventListener('click', closeNpmLogs);
  if (logsCancel) logsCancel.addEventListener('click', closeNpmLogs);
  if (logsRefresh) logsRefresh.addEventListener('click', () => { if (currentNpmLogId) refreshNpmLogs(currentNpmLogId); });
  if (logsClear) logsClear.addEventListener('click', async () => {
    if (!currentNpmLogId) return;
    if (!confirm('确定要清空当前项目的日志吗？')) return;
    try {
      const response = await fetch(`/api/npm-projects/${currentNpmLogId}/logs/clear`, { method: 'POST' });
      const result = await response.json();
      if (response.ok) {
        if (typeof showToast === 'function') showToast('日志已清空', 'success');
        await refreshNpmLogs(currentNpmLogId);
      } else {
        if (typeof showToast === 'function') showToast(result.error || '清空失败', 'error');
      }
    } catch (e) {
      if (typeof showToast === 'function') showToast('清空失败', 'error');
    }
  });
  if (logsModal) logsModal.addEventListener('click', e => { if (e.target === logsModal) closeNpmLogs(); });
  if (modal) modal.addEventListener('click', e => { if (e.target === modal) closeNpmModal(); });
}

document.addEventListener('DOMContentLoaded', () => {
  initNpmEvents();
  loadNpmProjectsData();
  refreshAllNpmStatuses(true);
  setInterval(() => refreshAllNpmStatuses(true), 5000);
});
