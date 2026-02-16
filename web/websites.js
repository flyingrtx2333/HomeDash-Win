// ========== 网站项目管理 ==========
let websites = [];
let pythonVersions = [];
let websitePollTimer = null;
let currentWebsiteId = null;
let logsWs = null;

// 加载网站项目数据
async function loadWebsitesData() {
    if (!document.getElementById('page-websites')) return;

    await Promise.all([
        loadWebsites(),
        loadPythonVersions()
    ]);

    if (websitePollTimer) clearInterval(websitePollTimer);
    websitePollTimer = setInterval(() => {
        websites.forEach(w => {
            checkWebsiteStatus(w.id);
        });
    }, 3000);
}

function stopWebsitePolling() {
    if (websitePollTimer) {
        clearInterval(websitePollTimer);
        websitePollTimer = null;
    }
}

// 加载网站项目列表
async function loadWebsites() {
    try {
        const response = await fetch('/api/websites');
        if (response.ok) {
            websites = await response.json();
            renderWebsites();
        }
    } catch (e) {
        console.log('加载网站项目列表失败', e);
    }
}

// 操作按钮图标
const icons = {
    detail: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
    edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    play: '<svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><polygon points="5 3 19 12 5 21 5 3"/></svg>',
    stop: '<svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>',
    trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>'
};

// 渲染网站项目列表
function renderWebsites() {
    const tableContainer = document.getElementById('websitesList');
    const tableBody = document.getElementById('websitesTableBody');
    const empty = document.getElementById('websitesEmpty');
    if (!tableContainer || !tableBody || !empty) return;

    if (websites.length === 0) {
        tableContainer.style.display = 'none';
        empty.style.display = 'block';
        return;
    }

    tableContainer.style.display = 'block';
    empty.style.display = 'none';
    tableBody.innerHTML = websites.map(w => {
        const status = getWebsiteStatus(w.id);
        const statusClass = status.running ? 'running' : 'stopped';
        const statusText = status.running ? '运行中' : '已停止';
        const pidText = status.running && status.pid ? ` (PID: ${status.pid})` : '';

        const startStopBtn = status.running
            ? `<button type="button" class="website-btn website-btn-danger" onclick="stopWebsite('${w.id}')" title="停止">${icons.stop}</button>`
            : `<button type="button" class="website-btn" onclick="startWebsite('${w.id}')" title="启动">${icons.play}</button>`;

        const nameHtml = `<a href="javascript:void(0)" class="website-td website-td-name website-td-name-link" onclick="openWebsiteUrl('${w.id}')" title="打开 ${getWebsiteUrl(w)}">${escapeHtml(w.name || '未命名')}</a>`;

        return `
            <div class="website-table-row" data-id="${w.id}">
                ${nameHtml}
                <span class="website-td website-td-status"><span class="website-status ${statusClass}">${statusText}${pidText}</span></span>
                <span class="website-td website-td-port">${w.port || '-'}</span>
                <span class="website-td website-td-path" title="${escapeHtml(w.path || '')}">${escapeHtml(w.path || '-')}</span>
                <span class="website-td website-td-actions">
                    <button type="button" class="website-btn" onclick="openWebsiteDetail('${w.id}')" title="详情">${icons.detail}</button>
                    <button type="button" class="website-btn" onclick="editWebsite('${w.id}')" title="编辑">${icons.edit}</button>
                    ${startStopBtn}
                    <button type="button" class="website-btn website-btn-danger" onclick="deleteWebsite('${w.id}')" title="删除">${icons.trash}</button>
                </span>
            </div>
        `;
    }).join('');
}

// 网站状态缓存
let websiteStatusCache = {};

// 获取网站状态（从缓存）
function getWebsiteStatus(id) {
    return websiteStatusCache[id] || { running: false, pid: 0 };
}

// 获取项目访问地址（服务器IP:端口）
function getWebsiteUrl(website) {
    const serverIpEl = document.getElementById('serverIp');
    const serverIp = (serverIpEl && serverIpEl.value) || (typeof currentSettings !== 'undefined' && currentSettings?.serverIp) || 'localhost';
    const port = website?.port || 0;
    if (!port) return '-';
    return `http://${serverIp}:${port}`;
}

// 点击项目名称打开服务器地址:端口页面
function openWebsiteUrl(id) {
    const website = websites.find(w => w.id === id);
    if (!website || !website.port) return;
    const url = getWebsiteUrl(website);
    if (url !== '-') window.open(url, '_blank');
}

// 检查网站状态
async function checkWebsiteStatus(id) {
    try {
        const response = await fetch(`/api/websites/${id}/status`);
        if (response.ok) {
            const status = await response.json();
            // 更新缓存
            websiteStatusCache[id] = status;
            // 更新UI中的状态显示
            const row = document.querySelector(`.website-table-row[data-id="${id}"]`);
            if (row) {
                const statusEl = row.querySelector('.website-status');
                if (statusEl) {
                    statusEl.className = `website-status ${status.running ? 'running' : 'stopped'}`;
                    statusEl.textContent = status.running ? `运行中 (PID: ${status.pid})` : '已停止';
                }
                // 若详情弹窗打开且显示该项目，同步更新详情弹窗中的状态
                if (currentWebsiteId === id) {
                    const detailStatus = document.getElementById('detailStatus');
                    const detailPid = document.getElementById('detailPid');
                    if (detailStatus) detailStatus.textContent = status.running ? '运行中' : '已停止';
                    if (detailPid) detailPid.textContent = status.running && status.pid ? status.pid : '-';
                }
                // 更新启动/停止按钮
                const actionsEl = row.querySelector('.website-td-actions');
                if (actionsEl) {
                    const startStopBtn = status.running
                        ? `<button type="button" class="website-btn website-btn-danger" onclick="stopWebsite('${id}')" title="停止">${icons.stop}</button>`
                        : `<button type="button" class="website-btn" onclick="startWebsite('${id}')" title="启动">${icons.play}</button>`;
                    const html = `
                        <button type="button" class="website-btn" onclick="openWebsiteDetail('${id}')" title="详情">${icons.detail}</button>
                        <button type="button" class="website-btn" onclick="editWebsite('${id}')" title="编辑">${icons.edit}</button>
                        ${startStopBtn}
                        <button type="button" class="website-btn website-btn-danger" onclick="deleteWebsite('${id}')" title="删除">${icons.trash}</button>
                    `;
                    actionsEl.innerHTML = html;
                }
            }
            return status;
        }
    } catch (e) {
        console.log('检查网站状态失败', e);
    }
    return { running: false, pid: 0 };
}

// 加载Python版本列表
async function loadPythonVersions() {
    try {
        const response = await fetch('/api/websites/python/versions');
        if (response.ok) {
            pythonVersions = await response.json();
            renderPythonVersions();
        }
    } catch (e) {
        console.log('加载Python版本失败', e);
    }
}

// 渲染Python版本选择器
function renderPythonVersions() {
    const select = document.getElementById('websitePythonPath');
    if (!select) return;

    select.innerHTML = '<option value="">请选择Python版本</option>';
    pythonVersions.forEach(v => {
        const option = document.createElement('option');
        option.value = v.path;
        option.textContent = `${v.version}${v.isDefault ? ' (默认)' : ''} - ${v.path}`;
        select.appendChild(option);
    });
}

// 打开创建项目弹窗
function openWebsiteModal(editId = null) {
    const modal = document.getElementById('websiteModal');
    const editIdInput = document.getElementById('websiteEditId');

    if (editId) {
        const website = websites.find(w => w.id === editId);
        if (website) {
            editIdInput.value = editId;
            document.getElementById('websiteName').value = website.name || '';
            document.getElementById('websitePath').value = website.path || '';
            document.getElementById('websitePort').value = website.port || '';
            document.getElementById('websitePythonPath').value = website.pythonPath || '';
            document.getElementById('websiteFramework').value = website.framework || 'flask';
            document.getElementById('websiteStartCommand').value = website.startCommand || '';
            document.getElementById('websiteVenvPath').value = website.venvPath || '';
            document.getElementById('websiteRequirementsTxt').value = website.requirementsTxt || '';
            document.getElementById('websiteEnvironmentVars').value = website.environmentVars 
                ? JSON.stringify(website.environmentVars, null, 2) 
                : '';
            document.getElementById('websiteAutoStart').checked = website.autoStart || false;
        }
    } else {
        editIdInput.value = '';
        document.getElementById('websiteName').value = '';
        document.getElementById('websitePath').value = '';
        document.getElementById('websitePort').value = '';
        document.getElementById('websitePythonPath').value = '';
        document.getElementById('websiteFramework').value = 'flask';
        document.getElementById('websiteStartCommand').value = '';
        document.getElementById('websiteVenvPath').value = '';
        document.getElementById('websiteRequirementsTxt').value = '';
        document.getElementById('websiteEnvironmentVars').value = '';
        document.getElementById('websiteAutoStart').checked = false;
    }

    modal.classList.add('active');
}

// 关闭创建/编辑弹窗
function closeWebsiteModal() {
    const modal = document.getElementById('websiteModal');
    if (modal) modal.classList.remove('active');
}

// 保存网站项目
async function saveWebsite() {
    const editId = document.getElementById('websiteEditId').value;
    const name = document.getElementById('websiteName').value.trim();
    const path = document.getElementById('websitePath').value.trim();
    const port = parseInt(document.getElementById('websitePort').value, 10);
    const pythonPath = document.getElementById('websitePythonPath').value.trim();
    const framework = document.getElementById('websiteFramework').value;
    const startCommand = document.getElementById('websiteStartCommand').value.trim();
    const venvPath = document.getElementById('websiteVenvPath').value.trim();
    const requirementsTxt = document.getElementById('websiteRequirementsTxt').value.trim();
    const envVarsStr = document.getElementById('websiteEnvironmentVars').value.trim();
    const autoStart = document.getElementById('websiteAutoStart').checked;

    if (!name || !path || !port || !startCommand) {
        if (typeof showToast === 'function') showToast('请填写项目名称、路径、端口和启动命令', 'warning');
        return;
    }
    if (!pythonPath && !venvPath) {
        if (typeof showToast === 'function') showToast('请填写 Python 路径或虚拟环境路径（二选一）', 'warning');
        return;
    }

    let environmentVars = {};
    if (envVarsStr) {
        try {
            environmentVars = JSON.parse(envVarsStr);
        } catch (e) {
            if (typeof showToast === 'function') showToast('环境变量格式错误，请使用JSON格式', 'error');
            return;
        }
    }

    const website = {
        name,
        path,
        port,
        pythonPath,
        framework,
        startCommand,
        venvPath: venvPath || undefined,
        requirementsTxt: requirementsTxt || undefined,
        environmentVars,
        autoStart
    };

    try {
        const url = editId ? `/api/websites/${editId}` : '/api/websites';
        const method = editId ? 'PUT' : 'POST';
        const response = await fetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(website)
        });

        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast(editId ? '项目已更新' : '项目已创建', 'success');
            closeWebsiteModal();
            await loadWebsites();
        } else {
            if (typeof showToast === 'function') showToast(result.error || '保存失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('保存失败', 'error');
    }
}

// 编辑项目
function editWebsite(id) {
    openWebsiteModal(id);
}

// 删除项目
async function deleteWebsite(id) {
    if (!confirm('确定要删除这个项目吗？')) return;

    try {
        const response = await fetch(`/api/websites/${id}`, {
            method: 'DELETE'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('项目已删除', 'success');
            await loadWebsites();
        } else {
            if (typeof showToast === 'function') showToast(result.error || '删除失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('删除失败', 'error');
    }
}

// 启动项目
async function startWebsite(id) {
    try {
        const response = await fetch(`/api/websites/${id}/start`, {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('项目已启动', 'success');
            // 立即更新缓存和UI（使用接口返回的 pid）
            if (result.pid) {
                websiteStatusCache[id] = { running: true, pid: result.pid };
            }
            renderWebsites();
            // 延迟再请求一次状态接口，确保与实际进程一致
            setTimeout(() => checkWebsiteStatus(id), 800);
        } else {
            if (typeof showToast === 'function') showToast(result.error || '启动失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('启动失败', 'error');
    }
}

// 停止项目
async function stopWebsite(id) {
    try {
        const response = await fetch(`/api/websites/${id}/stop`, {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('项目已停止', 'success');
            // 立即更新缓存和UI
            websiteStatusCache[id] = { running: false, pid: 0 };
            renderWebsites();
            // 延迟再请求一次状态接口
            setTimeout(() => checkWebsiteStatus(id), 500);
        } else {
            if (typeof showToast === 'function') showToast(result.error || '停止失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('停止失败', 'error');
    }
}

// 刷新所有项目状态
async function refreshAllStatuses() {
    if (websites.length === 0) return;
    const btn = document.getElementById('websiteRefreshStatusBtn');
    if (btn) {
        btn.disabled = true;
        btn.classList.add('loading');
    }
    try {
        await Promise.all(websites.map(w => checkWebsiteStatus(w.id)));
        if (typeof showToast === 'function') showToast('状态已刷新', 'success');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.classList.remove('loading');
        }
    }
}

// 打开项目详情弹窗
async function openWebsiteDetail(id) {
    const website = websites.find(w => w.id === id);
    if (!website) return;

    const modal = document.getElementById('websiteDetailModal');
    document.getElementById('websiteDetailTitle').textContent = website.name;
    document.getElementById('detailName').textContent = website.name || '-';
    document.getElementById('detailPath').textContent = website.path || '-';
    document.getElementById('detailPort').textContent = website.port || '-';
    document.getElementById('detailFramework').textContent = website.framework || 'custom';
    document.getElementById('detailStartCommand').textContent = website.startCommand || '-';
    document.getElementById('detailVenvPath').textContent = website.venvPath || '未配置';
    document.getElementById('detailStatus').textContent = '-';
    document.getElementById('detailPid').textContent = '-';

    // 设置按钮事件
    const startBtn = document.getElementById('websiteStartBtn');
    const stopBtn = document.getElementById('websiteStopBtn');
    const logsBtn = document.getElementById('websiteViewLogsBtn');
    const createVenvBtn = document.getElementById('websiteCreateVenvBtn');
    const deleteVenvBtn = document.getElementById('websiteDeleteVenvBtn');
    const installReqBtn = document.getElementById('websiteInstallRequirementsBtn');

    // 移除旧的事件监听器
    const newStartBtn = startBtn.cloneNode(true);
    const newStopBtn = stopBtn.cloneNode(true);
    const newLogsBtn = logsBtn.cloneNode(true);
    const newCreateVenvBtn = createVenvBtn.cloneNode(true);
    const newDeleteVenvBtn = deleteVenvBtn.cloneNode(true);
    const newInstallReqBtn = installReqBtn.cloneNode(true);

    startBtn.parentNode.replaceChild(newStartBtn, startBtn);
    stopBtn.parentNode.replaceChild(newStopBtn, stopBtn);
    logsBtn.parentNode.replaceChild(newLogsBtn, logsBtn);
    createVenvBtn.parentNode.replaceChild(newCreateVenvBtn, createVenvBtn);
    deleteVenvBtn.parentNode.replaceChild(newDeleteVenvBtn, deleteVenvBtn);
    installReqBtn.parentNode.replaceChild(newInstallReqBtn, installReqBtn);

    newStartBtn.onclick = () => {
        startWebsite(id);
        setTimeout(() => checkWebsiteStatus(id), 1000);
    };
    newStopBtn.onclick = () => {
        stopWebsite(id);
        setTimeout(() => checkWebsiteStatus(id), 1000);
    };
    newLogsBtn.onclick = () => {
        closeWebsiteDetail();
        viewWebsiteLogs(id);
    };
    newCreateVenvBtn.onclick = () => createVenv(id);
    newDeleteVenvBtn.onclick = () => deleteVenv(id);
    newInstallReqBtn.onclick = () => installRequirements(id);

    currentWebsiteId = id;
    modal.classList.add('active');

    // 加载状态
    checkWebsiteStatus(id).then((status) => {
        if (status) {
            document.getElementById('detailStatus').textContent = status.running ? '运行中' : '已停止';
            document.getElementById('detailPid').textContent = status.running && status.pid ? status.pid : '-';
        }
    });
}

// 关闭项目详情弹窗
function closeWebsiteDetail() {
    const modal = document.getElementById('websiteDetailModal');
    if (modal) modal.classList.remove('active');
    currentWebsiteId = null;
}

// 创建虚拟环境
async function createVenv(id) {
    if (!confirm('确定要创建虚拟环境吗？')) return;

    try {
        const response = await fetch(`/api/websites/${id}/venv/create`, {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('虚拟环境创建成功', 'success');
            await loadWebsites();
            if (currentWebsiteId === id) {
                openWebsiteDetail(id);
            }
        } else {
            if (typeof showToast === 'function') showToast(result.error || '创建失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('创建失败', 'error');
    }
}

// 删除虚拟环境
async function deleteVenv(id) {
    if (!confirm('确定要删除虚拟环境吗？此操作不可恢复！')) return;

    try {
        const response = await fetch(`/api/websites/${id}/venv`, {
            method: 'DELETE'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('虚拟环境已删除', 'success');
            await loadWebsites();
            if (currentWebsiteId === id) {
                openWebsiteDetail(id);
            }
        } else {
            if (typeof showToast === 'function') showToast(result.error || '删除失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('删除失败', 'error');
    }
}

// 安装依赖
async function installRequirements(id) {
    if (!confirm('确定要安装依赖吗？这可能需要一些时间。')) return;

    try {
        const response = await fetch(`/api/websites/${id}/requirements/install`, {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('依赖安装成功', 'success');
        } else {
            if (typeof showToast === 'function') showToast(result.error || '安装失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('安装失败', 'error');
    }
}

// 查看日志
async function viewWebsiteLogs(id) {
    const modal = document.getElementById('websiteLogsModal');
    const content = document.getElementById('websiteLogsContent');
    currentWebsiteId = id;

    modal.classList.add('active');
    await refreshWebsiteLogs(id);
}

// 刷新日志
async function refreshWebsiteLogs(id) {
    const content = document.getElementById('websiteLogsContent');
    if (!content) return;

    try {
        const response = await fetch(`/api/websites/${id}/logs`);
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

// 关闭日志弹窗
function closeWebsiteLogs() {
    const modal = document.getElementById('websiteLogsModal');
    if (modal) modal.classList.remove('active');
    currentWebsiteId = null;
    if (logsWs) {
        logsWs.close();
        logsWs = null;
    }
}

// HTML转义
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 检测项目信息
async function detectProjectInfo(path) {
    if (!path || path.trim() === '') {
        console.log('检测项目信息：路径为空');
        return;
    }

    console.log('开始检测项目信息，路径:', path);

    try {
        const response = await fetch('/api/websites/detect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: path.trim() })
        });

        if (response.ok) {
            const result = await response.json();
            console.log('检测结果:', result);
            
            // 自动填充检测到的信息
            if (result.framework && result.framework !== 'custom') {
                const frameworkSelect = document.getElementById('websiteFramework');
                if (frameworkSelect) {
                    frameworkSelect.value = result.framework;
                }
            }
            
            if (result.venvPath) {
                const venvInput = document.getElementById('websiteVenvPath');
                if (venvInput) {
                    venvInput.value = result.venvPath;
                }
            }
            
            if (result.requirementsTxt) {
                const reqInput = document.getElementById('websiteRequirementsTxt');
                if (reqInput) {
                    reqInput.value = result.requirementsTxt;
                }
            }
            
            if (result.startCommand) {
                const cmdInput = document.getElementById('websiteStartCommand');
                if (cmdInput) {
                    cmdInput.value = result.startCommand;
                }
            }

            // 如果项目名称为空，使用路径的最后一部分作为名称
            const nameInput = document.getElementById('websiteName');
            if (nameInput && !nameInput.value.trim()) {
                const pathParts = path.trim().split(/[/\\]/);
                const folderName = pathParts[pathParts.length - 1] || '未命名项目';
                nameInput.value = folderName;
            }

            // 构建检测结果消息
            let message = '已自动检测：';
            const parts = [];
            if (result.framework && result.framework !== 'custom') {
                parts.push(`框架=${result.framework}`);
            }
            if (result.venvPath) {
                parts.push('虚拟环境');
            }
            if (result.requirementsTxt) {
                parts.push('requirements.txt');
            }
            if (result.startCommand) {
                parts.push('启动命令');
            }
            message += parts.length > 0 ? parts.join('、') : '未检测到项目信息';

            if (typeof showToast === 'function') {
                showToast(message, parts.length > 0 ? 'success' : 'info');
            }
        } else {
            const error = await response.json();
            console.error('检测项目信息失败:', error.error);
            if (typeof showToast === 'function') {
                showToast('检测项目信息失败: ' + (error.error || '未知错误'), 'error');
            }
        }
    } catch (e) {
        console.error('检测项目信息异常:', e);
        if (typeof showToast === 'function') {
            showToast('检测项目信息失败: ' + e.message, 'error');
        }
    }
}

// 文件浏览器相关变量
let currentBrowsePath = '';
let selectedBrowsePath = '';

// 打开文件浏览器
function openPathBrowser() {
    const modal = document.getElementById('pathBrowserModal');
    currentBrowsePath = '';
    selectedBrowsePath = '';
    
    // 从根目录开始（Windows显示盘符列表）
    modal.classList.add('active');
    loadDirectory('root');
}

// 关闭文件浏览器
function closePathBrowser() {
    const modal = document.getElementById('pathBrowserModal');
    modal.classList.remove('active');
    currentBrowsePath = '';
    selectedBrowsePath = '';
}

// 加载目录内容
async function loadDirectory(path) {
    const body = document.getElementById('pathBrowserBody');
    const currentPathInput = document.getElementById('pathBrowserCurrentPath');
    
    body.innerHTML = '<div class="path-browser-loading">加载中...</div>';
    currentBrowsePath = path;
    
    try {
        const response = await fetch(`/api/websites/browse?path=${encodeURIComponent(path)}`);
        if (response.ok) {
            const data = await response.json();
            const displayPath = data.path === 'root' ? '选择盘符' : data.path;
            currentPathInput.value = displayPath;
            // 更新选中路径（如果不在root状态）
            if (data.path !== 'root') {
                selectedBrowsePath = data.path;
            }
            renderDirectory(data.files || [], data.path);
            updateBreadcrumb(data.path);
        } else {
            const error = await response.json();
            body.innerHTML = `<div class="path-browser-error">加载失败: ${error.error || '未知错误'}</div>`;
        }
    } catch (e) {
        body.innerHTML = `<div class="path-browser-error">加载失败: ${e.message}</div>`;
    }
}

// 渲染目录列表
function renderDirectory(files, currentPath) {
    const body = document.getElementById('pathBrowserBody');
    
    if (files.length === 0) {
        body.innerHTML = '<div class="path-browser-empty">文件夹为空</div>';
        return;
    }
    
    const folderIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
    const fileIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
    
    body.innerHTML = files.map(file => {
        const icon = file.isDir ? folderIcon : fileIcon;
        const className = file.isDir ? 'path-item path-item-dir' : 'path-item path-item-file';
        
        return `
            <div class="${className}" data-path="${escapePath(file.path)}" data-isdir="${file.isDir}">
                <span class="path-item-icon">${icon}</span>
                <span class="path-item-name">${escapeHtml(file.name)}</span>
            </div>
        `;
    }).join('');
    
    // 添加点击事件处理
    body.querySelectorAll('.path-item-dir').forEach(item => {
        let clickTimer = null;
        item.addEventListener('click', (e) => {
            const path = item.dataset.path;
            
            // 立即更新选中状态（视觉反馈）
            selectedBrowsePath = path;
            const currentPathInput = document.getElementById('pathBrowserCurrentPath');
            if (currentPathInput) {
                currentPathInput.value = path;
            }
            body.querySelectorAll('.path-item').forEach(i => i.classList.remove('path-item-selected'));
            item.classList.add('path-item-selected');
            
            console.log('选中文件夹:', path);
            
            // 单击：选中文件夹（延迟处理，避免与双击冲突）
            if (clickTimer === null) {
                clickTimer = setTimeout(() => {
                    clickTimer = null;
                }, 300); // 300ms延迟，用于区分单击和双击
            }
        });
        
        // 双击：进入文件夹
        item.addEventListener('dblclick', (e) => {
            e.stopPropagation();
            if (clickTimer) {
                clearTimeout(clickTimer);
                clickTimer = null;
            }
            const path = item.dataset.path;
            console.log('双击进入文件夹:', path);
            navigateToDirectory(path);
        });
    });
}

// 导航到目录
function navigateToDirectory(path) {
    selectedBrowsePath = path; // 更新选中路径
    loadDirectory(path);
}

// 选择目录（双击）
function selectDirectory(path) {
    selectedBrowsePath = path;
    document.getElementById('pathBrowserCurrentPath').value = path;
    confirmPathSelection();
}

// 更新面包屑导航
function updateBreadcrumb(path) {
    const breadcrumb = document.getElementById('pathBreadcrumb');
    
    if (!path || path === '' || path === 'root') {
        breadcrumb.innerHTML = '<span class="breadcrumb-item active" data-path="root" onclick="loadDirectory(\'root\')">选择盘符</span>';
        return;
    }
    
    // Windows路径处理
    const isWindows = path.includes('\\');
    const parts = isWindows ? path.split('\\').filter(p => p) : path.split('/').filter(p => p);
    
    let html = '<span class="breadcrumb-item" data-path="root" onclick="loadDirectory(\'root\')">选择盘符</span>';
    let currentPath = '';
    
    // Windows特殊处理：第一个部分是盘符
    if (isWindows && parts.length > 0) {
        const drive = parts[0];
        currentPath = drive + '\\';
        html += `<span class="breadcrumb-sep">\\</span>`;
        html += `<span class="breadcrumb-item" data-path="${currentPath}" onclick="loadDirectory('${currentPath}')">${drive}</span>`;
        
        for (let i = 1; i < parts.length; i++) {
            currentPath += parts[i];
            if (i < parts.length - 1) {
                currentPath += '\\';
            }
            const isLast = i === parts.length - 1;
            html += `<span class="breadcrumb-sep">\\</span>`;
            html += `<span class="breadcrumb-item${isLast ? ' active' : ''}" data-path="${currentPath}" onclick="loadDirectory('${currentPath}')">${parts[i]}</span>`;
        }
    } else {
        // Unix路径
        html = '<span class="breadcrumb-item" data-path="/" onclick="loadDirectory(\'/\')">根目录</span>';
        for (let i = 0; i < parts.length; i++) {
            currentPath += '/' + parts[i];
            const isLast = i === parts.length - 1;
            html += `<span class="breadcrumb-sep">/</span>`;
            html += `<span class="breadcrumb-item${isLast ? ' active' : ''}" data-path="${currentPath}" onclick="loadDirectory('${currentPath}')">${parts[i]}</span>`;
        }
    }
    
    breadcrumb.innerHTML = html;
}

// 转义路径（用于HTML属性）
function escapePath(path) {
    return path.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '&quot;');
}

// 确认选择路径
function confirmPathSelection() {
    const currentPathInput = document.getElementById('pathBrowserCurrentPath');
    const currentPath = currentPathInput ? currentPathInput.value : '';
    
    // 优先使用selectedBrowsePath，否则使用当前路径
    const pathToUse = selectedBrowsePath || currentPath;
    
    console.log('确认选择路径 - selectedBrowsePath:', selectedBrowsePath);
    console.log('确认选择路径 - currentPath:', currentPath);
    console.log('确认选择路径 - pathToUse:', pathToUse);
    
    if (pathToUse && pathToUse.trim() !== '' && pathToUse !== '选择盘符' && pathToUse !== 'root') {
        const finalPath = pathToUse.trim();
        document.getElementById('websitePath').value = finalPath;
        closePathBrowser();
        // 自动检测项目信息
        console.log('开始检测项目信息，路径:', finalPath);
        detectProjectInfo(finalPath);
    } else {
        if (typeof showToast === 'function') {
            showToast('请先选择一个文件夹', 'warning');
        }
    }
}

// 初始化事件监听
function initWebsiteEvents() {
    const addBtn = document.getElementById('websiteAddBtn');
    const modal = document.getElementById('websiteModal');
    const modalClose = document.getElementById('websiteModalClose');
    const modalCancel = document.getElementById('websiteModalCancel');
    const modalConfirm = document.getElementById('websiteModalConfirm');
    const detailModal = document.getElementById('websiteDetailModal');
    const detailClose = document.getElementById('websiteDetailClose');
    const detailCancel = document.getElementById('websiteDetailCancel');
    const logsModal = document.getElementById('websiteLogsModal');
    const logsClose = document.getElementById('websiteLogsClose');
    const logsCancel = document.getElementById('websiteLogsCancel');
    const logsRefresh = document.getElementById('websiteLogsRefreshBtn');
    const refreshPythonBtn = document.getElementById('websiteRefreshPythonBtn');
    const pathBrowseBtn = document.getElementById('websitePathBrowseBtn');
    const pathInput = document.getElementById('websitePath');

    const refreshStatusBtn = document.getElementById('websiteRefreshStatusBtn');
    if (refreshStatusBtn) refreshStatusBtn.addEventListener('click', refreshAllStatuses);
    if (addBtn) addBtn.addEventListener('click', () => openWebsiteModal());
    if (modalClose) modalClose.addEventListener('click', closeWebsiteModal);
    if (modalCancel) modalCancel.addEventListener('click', closeWebsiteModal);
    if (modalConfirm) modalConfirm.addEventListener('click', saveWebsite);
    
    // 路径浏览按钮
    if (pathBrowseBtn) {
        pathBrowseBtn.addEventListener('click', openPathBrowser);
    }
    
    // 文件浏览器相关事件
    const pathBrowserModal = document.getElementById('pathBrowserModal');
    const pathBrowserClose = document.getElementById('pathBrowserClose');
    const pathBrowserCancel = document.getElementById('pathBrowserCancel');
    const pathBrowserConfirm = document.getElementById('pathBrowserConfirm');
    
    if (pathBrowserClose) pathBrowserClose.addEventListener('click', closePathBrowser);
    if (pathBrowserCancel) pathBrowserCancel.addEventListener('click', closePathBrowser);
    if (pathBrowserConfirm) pathBrowserConfirm.addEventListener('click', confirmPathSelection);
    
    if (pathBrowserModal) {
        pathBrowserModal.addEventListener('click', (e) => {
            if (e.target === pathBrowserModal) closePathBrowser();
        });
    }
    
    // 路径输入框双击时打开浏览器
    if (pathInput) {
        pathInput.addEventListener('dblclick', openPathBrowser);
    }
    if (detailClose) detailClose.addEventListener('click', closeWebsiteDetail);
    if (detailCancel) detailCancel.addEventListener('click', closeWebsiteDetail);
    if (logsClose) logsClose.addEventListener('click', closeWebsiteLogs);
    if (logsCancel) logsCancel.addEventListener('click', closeWebsiteLogs);
    if (logsRefresh) logsRefresh.addEventListener('click', () => {
        if (currentWebsiteId) refreshWebsiteLogs(currentWebsiteId);
    });
    if (refreshPythonBtn) refreshPythonBtn.addEventListener('click', loadPythonVersions);

    if (modal) {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeWebsiteModal();
        });
    }
    if (detailModal) {
        detailModal.addEventListener('click', (e) => {
            if (e.target === detailModal) closeWebsiteDetail();
        });
    }
    if (logsModal) {
        logsModal.addEventListener('click', (e) => {
            if (e.target === logsModal) closeWebsiteLogs();
        });
    }
}

// 页面加载时初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        initWebsiteEvents();
        // 如果当前页面是websites页面，加载数据
        if (document.getElementById('page-websites')) {
            loadWebsitesData();
        }
    });
} else {
    initWebsiteEvents();
    if (document.getElementById('page-websites')) {
        loadWebsitesData();
    }
}

// 页面切换时加载数据
document.addEventListener('pageChange', (e) => {
    if (e.detail === 'websites') {
        loadWebsitesData();
    } else {
        stopWebsitePolling();
    }
});
