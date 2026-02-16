
// ========== 全局变量 ==========
const bgLayer = document.getElementById('bgLayer');
const settingsBtn = document.getElementById('settingsBtn');
const settingsPanel = document.getElementById('settingsPanel');
const bgGrid = document.getElementById('bgGrid');
const serverIpInput = document.getElementById('serverIp');
const sidebar = document.getElementById('sidebar');
const sidebarToggleBtn = document.getElementById('sidebarToggleBtn');
const servicesGrid = document.getElementById('servicesGrid');
const emptyState = document.getElementById('emptyState');
const toastContainer = document.getElementById('toastContainer');

let presetBackgrounds = [];
let currentSettings = { serverIp: 'localhost', backgroundUrl: '' };
let services = [];
let pingResults = {}; // 存储连通性检测结果
let serviceProcessStatus = {}; // 存储服务进程状态 { serviceId: { running: bool, pid: number } }
let saveTimer = null;
let monitorWs = null;
let reconnectTimer = null;
let editingServiceId = null;
let pingInterval = null;
let processCheckInterval = null; // 进程检测定时器

// 文件管理相关
let currentFilePath = '/';
let deletingFilePath = null;

// 终端相关
let terminalWs = null;
let terminalHistory = [];
let historyIndex = -1;

// ========== Toast 提示系统 ==========
function showToast(message, type = 'info') {
    if (!toastContainer) return;

    const icons = {
        success: '<span class="toast-icon-dot success" aria-hidden="true"></span>',
        warning: '<span class="toast-icon-dot warning" aria-hidden="true"></span>',
        error: '<span class="toast-icon-dot error" aria-hidden="true"></span>',
        info: '<span class="toast-icon-dot info" aria-hidden="true"></span>'
    };

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.innerHTML = `
        <span class="toast-icon">${icons[type] || icons.info}</span>
        <span class="toast-content">${message}</span>
        <button class="toast-close" onclick="this.parentElement.remove()">×</button>
    `;

    toastContainer.appendChild(toast);

    // 触发动画
    setTimeout(() => {
        toast.classList.add('show');
    }, 10);

    // 自动消失（success 3秒，其他 5秒）
    const duration = type === 'success' ? 3000 : 5000;
    setTimeout(() => {
        toast.classList.remove('show');
        toast.classList.add('hide');
        setTimeout(() => {
            if (toast.parentElement) {
                toast.remove();
            }
        }, 300);
    }, duration);
}

// ========== 服务管理 ==========
async function loadServices() {
    try {
        const response = await fetch('/api/services');
        if (response.ok) {
            services = await response.json();
        }
    } catch (e) {
        console.log('加载服务列表失败');
    }
    renderServices();
}

function renderServices() {
    // 显示/隐藏空状态
    if (services.length === 0) {
        servicesGrid.style.display = 'none';
        emptyState.style.display = 'flex';
        return;
    }
    servicesGrid.style.display = 'grid';
    emptyState.style.display = 'none';

    const ip = currentSettings.serverIp || 'localhost';
    servicesGrid.innerHTML = services.map(service => {
        const hasPort = service.port && service.port > 0;
        const isEnabled = service.enabled && hasPort;
        const url = hasPort ? `http://${ip}:${service.port}` : '#';
        const linkText = hasPort ? url : (service.enabled ? '本地应用' : '本地应用');
        const cardClass = 'card service-card'; // 所有卡片都正常显示，不显示禁用样式
        const isImage = service.icon && service.icon.startsWith('/');
        const defaultIconSvg = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="32" height="32"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>';
        const iconHtml = isImage
            ? `<img class="card-icon" src="${service.icon}" alt="${service.name}" />`
            : `<div class="card-icon">${defaultIconSvg}</div>`;

        // 连通状态指示器（放在 link 右边）
        const ping = pingResults[service.id];
        let statusHtml = '';
        if (isEnabled && ping) {
            const statusClass = ping.status === 'ok' ? 'status-ok' :
                ping.status === 'slow' ? 'status-slow' : 'status-error';
            const statusText = ping.status === 'ok' ? '通' : ping.status === 'slow' ? '慢' : '断';
            const latencyText = ping.latency > 0 ? `${ping.latency}ms` : '';
            statusHtml = `<span class="ping-status-inline ${statusClass}" title="连通状态"><span>${statusText}</span><span>${latencyText}</span></span>`;
        } else if (isEnabled) {
            statusHtml = `<span class="ping-status-inline status-unknown" title="连通状态"><span>?</span></span>`;
        }

        // 自启状态指示器
        const autostartHtml = service.autoStart ? '<div class="autostart-badge" title="已启用开机自启">自启</div>' : '';

        // 启动/停止按钮（根据进程状态动态显示）
        const processStatus = serviceProcessStatus[service.id] || { running: false };
        let actionBtnHtml = '';
        const hasLaunchConfig = service.launchCommand || service.launchPath;
        if (hasLaunchConfig) {
            if (processStatus.running) {
                actionBtnHtml = `<button class="card-stop-btn" data-id="${service.id}" title="停止服务">停止</button>`;
            } else {
                actionBtnHtml = `<button class="card-launch-btn" data-id="${service.id}" title="启动服务">启动</button>`;
            }
        }

        return `
            <div class="${cardClass}" data-id="${service.id}" data-port="${service.port || 0}">
              <div class="card-actions">
                <button class="card-action-btn edit-btn" data-id="${service.id}" title="编辑" aria-label="编辑">
                  <svg class="card-action-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                </button>
                <button class="card-action-btn delete-btn" data-id="${service.id}" title="删除" aria-label="删除">
                  <svg class="card-action-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6"/><path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><line x1="10" y1="11" x2="10" y2="17"/><line x1="14" y1="11" x2="14" y2="17"/></svg>
                </button>
              </div>
              ${autostartHtml}
              ${isEnabled ? `<a href="${url}" target="_blank" rel="noreferrer" class="card-link">` : '<div class="card-link">'}
                ${iconHtml}
                <h3>${service.name}</h3>
                <p>${service.description || ''}</p>
                <div class="link-with-status">
                  <span class="link">${linkText}</span>
                  ${statusHtml}
                </div>
              ${isEnabled ? '</a>' : '</div>'}
              ${actionBtnHtml}
            </div>
          `;
    }).join('');

    // 绑定编辑/删除事件
    document.querySelectorAll('.edit-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            openEditModal(btn.dataset.id);
        });
    });

    document.querySelectorAll('.delete-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            e.stopPropagation();
            openDeleteModal(btn.dataset.id);
        });
    });

    // 绑定启动按钮事件
    document.querySelectorAll('.card-launch-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.stopPropagation();
            const serviceId = btn.dataset.id;
            const service = services.find(s => s.id === serviceId);
            const hasLaunchConfig = service && (service.launchCommand || service.launchPath);
            if (!hasLaunchConfig) return;

            // 设置loading状态
            btn.disabled = true;
            btn.classList.add('loading');
            btn.textContent = '启动中';

            try {
                // 调用启动API
                const response = await fetch(`/api/services/${serviceId}/launch`, {
                    method: 'POST'
                });
                const result = await response.json();

                if (response.ok) {
                    // 轮询检测进程是否启动成功（最多180秒）
                    const maxAttempts = 180; // 180秒
                    let attempts = 0;
                    let started = false;

                    const checkProcess = async () => {
                        attempts++;
                        await checkServiceProcessStatus(serviceId);
                        const status = serviceProcessStatus[serviceId];

                        if (status && status.running) {
                            // 进程已启动
                            started = true;
                            showToast('服务启动成功', 'success');
                            renderServices();
                            return;
                        }

                        if (attempts < maxAttempts) {
                            // 继续检测
                            setTimeout(checkProcess, 1000); // 每秒检测一次
                        } else {
                            // 超时
                            showToast('启动超时：180秒内未检测到进程启动', 'warning');
                            btn.disabled = false;
                            btn.classList.remove('loading');
                            renderServices();
                        }
                    };

                    // 开始检测
                    setTimeout(checkProcess, 1000); // 1秒后开始检测
                } else {
                    showToast('启动失败: ' + (result.error || '未知错误'), 'error');
                    btn.disabled = false;
                    btn.classList.remove('loading');
                    renderServices();
                }
            } catch (e) {
                showToast('启动失败: ' + e.message, 'error');
                btn.disabled = false;
                btn.classList.remove('loading');
                renderServices();
            }
        });
    });

    // 绑定停止按钮事件
    document.querySelectorAll('.card-stop-btn').forEach(btn => {
        btn.addEventListener('click', async (e) => {
            e.stopPropagation();
            const serviceId = btn.dataset.id;
            const service = services.find(s => s.id === serviceId);
            const hasLaunchConfig = service && (service.launchCommand || service.launchPath);
            if (!hasLaunchConfig) return;

            // 设置loading状态
            btn.disabled = true;
            btn.classList.add('loading');
            btn.textContent = '停止中';

            try {
                // 调用停止API
                const response = await fetch(`/api/services/${serviceId}/stop`, {
                    method: 'POST'
                });
                const result = await response.json();

                if (response.ok) {
                    // 轮询检测进程是否已停止（最多180秒）
                    const maxAttempts = 180; // 180秒
                    let attempts = 0;
                    let stopped = false;

                    const checkProcess = async () => {
                        attempts++;
                        await checkServiceProcessStatus(serviceId);
                        const status = serviceProcessStatus[serviceId];

                        if (!status || !status.running) {
                            // 进程已停止
                            stopped = true;
                            showToast('服务已停止', 'success');
                            renderServices();
                            return;
                        }

                        if (attempts < maxAttempts) {
                            // 继续检测
                            setTimeout(checkProcess, 1000); // 每秒检测一次
                        } else {
                            // 超时
                            showToast('停止超时：180秒内进程仍未停止', 'warning');
                            btn.disabled = false;
                            btn.classList.remove('loading');
                            renderServices();
                        }
                    };

                    // 开始检测
                    setTimeout(checkProcess, 1000); // 1秒后开始检测
                } else {
                    showToast('停止失败: ' + (result.error || '未知错误'), 'error');
                    btn.disabled = false;
                    btn.classList.remove('loading');
                    renderServices();
                }
            } catch (e) {
                showToast('停止失败: ' + e.message, 'error');
                btn.disabled = false;
                btn.classList.remove('loading');
                renderServices();
            }
        });
    });
}

// ========== 进程状态检测 ==========
async function checkServiceProcessStatus(serviceId) {
    try {
        const response = await fetch(`/api/services/${serviceId}/process-status`);
        if (response.ok) {
            const status = await response.json();
            serviceProcessStatus[serviceId] = status;
        }
    } catch (e) {
        console.log('检测进程状态失败:', serviceId);
    }
}

async function checkAllServiceProcesses() {
    const servicesWithConfig = services.filter(s => s.launchCommand || s.launchPath);
    for (const service of servicesWithConfig) {
        await checkServiceProcessStatus(service.id);
    }
    renderServices();
}

function updateServiceLinks() {
    const ip = currentSettings.serverIp || 'localhost';
    document.querySelectorAll('.service-card').forEach(card => {
        const port = parseInt(card.dataset.port);
        if (port > 0) {
            const url = `http://${ip}:${port}`;
            const link = card.querySelector('.card-link');
            if (link && link.tagName === 'A') {
                link.href = url;
            }
            const linkText = card.querySelector('.link');
            if (linkText) {
                linkText.textContent = url;
            }
        }
    });
}

// ========== 连通性检测 ==========
async function pingAllServices() {
    const btn = document.getElementById('pingAllBtn');
    btn.disabled = true;
    btn.textContent = '检测中...';

    try {
        const response = await fetch('/api/ping-all');
        if (response.ok) {
            const results = await response.json();
            results.forEach(r => {
                pingResults[r.id] = r;
            });
            renderServices();
        }
    } catch (e) {
        console.log('连通性检测失败');
    }

    btn.disabled = false;
    btn.textContent = '检测连通';
}

// ========== 模板导入 ==========
async function importTemplate() {
    if (!confirm('是否导入推荐服务模板？已存在的同名服务不会重复添加。')) return;

    try {
        const response = await fetch('/api/services/import-template', { method: 'POST' });
        if (response.ok) {
            await loadServices();
            alert('导入成功！');
        }
    } catch (e) {
        alert('导入失败');
    }
}

// ========== Favicon 抓取 ==========
async function fetchFavicon() {
    const port = document.getElementById('servicePort').value;
    if (!port) {
        alert('请先填写端口号');
        return;
    }

    const ip = currentSettings.serverIp || 'localhost';
    const url = `http://${ip}:${port}`;

    const btn = document.getElementById('fetchFaviconBtn');
    btn.disabled = true;
    btn.textContent = '获取中...';

    try {
        const response = await fetch(`/api/favicon?url=${encodeURIComponent(url)}`);
        const result = await response.json();

        if (result.success && result.icon) {
            document.getElementById('serviceIcon').value = result.icon;
            document.querySelectorAll('.icon-option').forEach(o => o.classList.remove('active'));
            alert('图标获取成功！');
        } else {
            alert('无法获取图标: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('获取图标失败');
    }

    btn.disabled = false;
    btn.textContent = '获取图标';
}

// ========== 进程管理 ==========
let processesData = [];
let filteredProcesses = [];
let selectedProcesses = new Set();
let processAutoRefreshInterval = null;
let currentSort = { field: 'cpu', order: 'desc' };

const processTableBody = document.getElementById('processTableBody');
const refreshProcessBtn = document.getElementById('refreshProcessBtn');
const processSearch = document.getElementById('processSearch');
const processFilter = document.getElementById('processFilter');
const processSort = document.getElementById('processSort');
const selectAllCheckbox = document.getElementById('selectAllCheckbox');
const killSelectedBtn = document.getElementById('killSelectedBtn');
const totalProcessesEl = document.getElementById('totalProcesses');
const highCpuProcessesEl = document.getElementById('highCpuProcesses');
const highMemoryProcessesEl = document.getElementById('highMemoryProcesses');
const autoRefreshStatusEl = document.getElementById('autoRefreshStatus');
const processDetailModal = document.getElementById('processDetailModal');
const closeProcessDetailModal = document.getElementById('closeProcessDetailModal');
const detailName = document.getElementById('detailName');
const detailPid = document.getElementById('detailPid');
const detailCpu = document.getElementById('detailCpu');
const detailMemory = document.getElementById('detailMemory');
const detailUser = document.getElementById('detailUser');
const detailStatus = document.getElementById('detailStatus');
const detailStartTime = document.getElementById('detailStartTime');
const detailPath = document.getElementById('detailPath');
const killProcessBtn = document.getElementById('killProcessBtn');
const refreshProcessDetailBtn = document.getElementById('refreshProcessDetailBtn');
const batchKillModal = document.getElementById('batchKillModal');
const cancelBatchKillBtn = document.getElementById('cancelBatchKillBtn');
const confirmBatchKillBtn = document.getElementById('confirmBatchKillBtn');
const selectedCountEl = document.getElementById('selectedCount');

function initProcessManagement() {
    if (!document.getElementById('processTableBody')) return;
    bindProcessEvents();
    loadProcesses();
}

function bindProcessEvents() {
    if (refreshProcessBtn) refreshProcessBtn.addEventListener('click', loadProcesses);
    if (processSearch) processSearch.addEventListener('input', debounce(filterAndRenderProcesses, 300));
    if (processFilter) processFilter.addEventListener('change', filterAndRenderProcesses);
    if (processSort) processSort.addEventListener('change', sortAndRenderProcesses);
    if (selectAllCheckbox) selectAllCheckbox.addEventListener('change', toggleSelectAll);
    if (killSelectedBtn) killSelectedBtn.addEventListener('click', showBatchKillModal);
    if (closeProcessDetailModal) closeProcessDetailModal.addEventListener('click', hideProcessDetailModal);
    if (killProcessBtn) killProcessBtn.addEventListener('click', killSingleProcess);
    if (refreshProcessDetailBtn) refreshProcessDetailBtn.addEventListener('click', refreshCurrentProcessDetail);
    if (cancelBatchKillBtn) cancelBatchKillBtn.addEventListener('click', hideBatchKillModal);
    if (confirmBatchKillBtn) confirmBatchKillBtn.addEventListener('click', killSelectedProcesses);
    if (processDetailModal) {
        processDetailModal.addEventListener('click', function(e) {
            if (e.target === processDetailModal) hideProcessDetailModal();
        });
    }
    if (batchKillModal) {
        batchKillModal.addEventListener('click', function(e) {
            if (e.target === batchKillModal) hideBatchKillModal();
        });
    }
}

async function loadProcesses() {
    const container = document.getElementById('processTableBody');
    if (!container) return;
    container.innerHTML = `
        <div class="loading-overlay">
            <div class="loading-spinner"></div>
            <div class="loading-text">加载中...</div>
        </div>
    `;
    try {
        const response = await fetch('/api/processes');
        if (response.ok) {
            processesData = await response.json();
            updateSummaryStats();
            filterAndRenderProcesses();
        } else {
            throw new Error('Failed to load processes');
        }
    } catch (error) {
        container.innerHTML = `
            <div class="loading-overlay">
                <div>加载失败</div>
                <button class="btn btn-primary" onclick="loadProcesses()">重试</button>
            </div>
        `;
        console.error('加载进程失败:', error);
    }
}

function updateSummaryStats() {
    const total = processesData.length;
    const highCpu = processesData.filter(p => p.cpu > 20).length;
    const highMemory = processesData.filter(p => p.memory > 100 * 1024 * 1024).length;
    if (totalProcessesEl) totalProcessesEl.textContent = total;
    if (highCpuProcessesEl) highCpuProcessesEl.textContent = highCpu;
    if (highMemoryProcessesEl) highMemoryProcessesEl.textContent = highMemory;
}

function filterProcesses() {
    let filtered = [...processesData];
    if (processSearch && processSearch.value.trim()) {
        const searchTerm = processSearch.value.toLowerCase().trim();
        filtered = filtered.filter(p =>
            p.name.toLowerCase().includes(searchTerm) ||
            p.pid.toString().includes(searchTerm) ||
            (p.user && p.user.toLowerCase().includes(searchTerm))
        );
    }
    if (processFilter) {
        const filterType = processFilter.value;
        if (filterType === 'running') filtered = filtered.filter(p => p.status === 'running');
        else if (filterType === 'high-cpu') filtered = filtered.filter(p => p.cpu > 20);
        else if (filterType === 'high-memory') filtered = filtered.filter(p => p.memory > 100 * 1024 * 1024);
    }
    filteredProcesses = filtered;
    return filtered;
}

function sortProcesses(processes) {
    const sortValue = processSort ? processSort.value : 'cpu-desc';
    const [field, order] = sortValue.split('-');
    return [...processes].sort((a, b) => {
        let valueA = a[field];
        let valueB = b[field];
        if (field === 'name' || field === 'user' || field === 'status') {
            valueA = valueA || '';
            valueB = valueB || '';
            return order === 'asc' ? valueA.localeCompare(valueB) : valueB.localeCompare(valueA);
        }
        return order === 'asc' ? valueA - valueB : valueB - valueA;
    });
}

function filterAndRenderProcesses() {
    renderProcesses(sortProcesses(filterProcesses()));
}

function sortAndRenderProcesses() {
    renderProcesses(sortProcesses(filteredProcesses));
}

function renderProcesses(processes) {
    const container = document.getElementById('processTableBody');
    if (!container) return;
    if (!processes || processes.length === 0) {
        container.innerHTML = `
            <div class="loading-overlay">
                <div>暂无进程数据</div>
                <button class="btn btn-primary" onclick="loadProcesses()">刷新</button>
            </div>
        `;
        return;
    }
    container.innerHTML = '';
    processes.forEach(process => {
        const isSelected = selectedProcesses.has(process.pid);
        const row = document.createElement('div');
        row.className = `process-row ${isSelected ? 'selected' : ''}`;
        row.dataset.pid = process.pid;
        row.dataset.name = process.name;
        row.innerHTML = `
            <div class="process-cell checkbox-cell">
                <label class="checkbox-item">
                    <input type="checkbox" class="process-checkbox" value="${process.pid}" ${isSelected ? 'checked' : ''}>
                    <span class="checkmark"></span>
                </label>
            </div>
            <div class="process-cell pid" title="PID: ${process.pid}">${process.pid}</div>
            <div class="process-cell name" title="${process.name}">
                <span class="process-name">${truncateText(process.name, 30)}</span>
            </div>
            <div class="process-cell cpu ${process.cpu > 50 ? 'high' : process.cpu > 20 ? 'medium' : ''}" title="CPU: ${process.cpu.toFixed(2)}%">
                <div class="cpu-bar" style="width: ${Math.min(process.cpu, 100)}%"></div>
                <span class="cpu-value">${process.cpu.toFixed(1)}%</span>
            </div>
            <div class="process-cell memory" title="内存: ${formatBytes(process.memory)}">${formatBytes(process.memory)}</div>
            <div class="process-cell user" title="${process.user || 'N/A'}">${process.user || 'N/A'}</div>
            <div class="process-cell status">
                <span class="status-indicator ${getStatusClass(process.status)}" title="${process.status || 'unknown'}"></span>
                ${process.status || 'unknown'}
            </div>
            <div class="process-cell actions">
                <button class="action-btn detail" title="查看详情" data-pid="${process.pid}">详情</button>
                <button class="action-btn kill" title="结束进程" data-pid="${process.pid}">结束</button>
            </div>
        `;
        container.appendChild(row);
    });
    bindProcessRowEvents();
    updateSelectAllCheckbox();
    updateKillSelectedButton();
}

function bindProcessRowEvents() {
    document.querySelectorAll('.process-checkbox').forEach(checkbox => {
        checkbox.removeEventListener('change', handleProcessSelection);
        checkbox.addEventListener('change', handleProcessSelection);
    });
    document.querySelectorAll('.action-btn.detail').forEach(btn => {
        btn.removeEventListener('click', showProcessDetail);
        btn.addEventListener('click', showProcessDetail);
    });
    document.querySelectorAll('.action-btn.kill').forEach(btn => {
        btn.removeEventListener('click', confirmKillProcess);
        btn.addEventListener('click', confirmKillProcess);
    });
    document.querySelectorAll('.process-row').forEach(row => {
        row.removeEventListener('click', handleRowClick);
        row.addEventListener('click', handleRowClick);
    });
}

function handleProcessSelection(e) {
    const checkbox = e.target;
    const pid = parseInt(checkbox.value);
    if (checkbox.checked) selectedProcesses.add(pid);
    else selectedProcesses.delete(pid);
    const row = checkbox.closest('.process-row');
    if (row) row.classList.toggle('selected', checkbox.checked);
    updateSelectAllCheckbox();
    updateKillSelectedButton();
}

function handleRowClick(e) {
    if (e.target.classList.contains('process-checkbox') || e.target.classList.contains('action-btn') || e.target.tagName === 'BUTTON') return;
    const row = e.currentTarget;
    const checkbox = row.querySelector('.process-checkbox');
    if (checkbox) {
        checkbox.checked = !checkbox.checked;
        checkbox.dispatchEvent(new Event('change'));
    }
}

function toggleSelectAll(e) {
    const isChecked = e.target.checked;
    document.querySelectorAll('.process-checkbox').forEach(checkbox => {
        if (checkbox.checked !== isChecked) {
            checkbox.checked = isChecked;
            checkbox.dispatchEvent(new Event('change'));
        }
    });
}

function updateSelectAllCheckbox() {
    const checkboxes = document.querySelectorAll('.process-checkbox');
    const selectedCheckboxes = document.querySelectorAll('.process-checkbox:checked');
    if (selectAllCheckbox) selectAllCheckbox.checked = checkboxes.length > 0 && checkboxes.length === selectedCheckboxes.length;
}

function updateKillSelectedButton() {
    const selectedCount = selectedProcesses.size;
    if (killSelectedBtn) {
        killSelectedBtn.disabled = selectedCount === 0;
        killSelectedBtn.textContent = selectedCount > 0 ? `结束选中 (${selectedCount})` : '结束选中';
    }
}

function showBatchKillModal() {
    if (selectedProcesses.size === 0) return;
    if (selectedCountEl) selectedCountEl.textContent = selectedProcesses.size;
    if (batchKillModal) batchKillModal.classList.add('active');
}

function hideBatchKillModal() {
    if (batchKillModal) batchKillModal.classList.remove('active');
}

async function killSelectedProcesses() {
    const pids = Array.from(selectedProcesses);
    if (pids.length === 0) return;
    try {
        const response = await fetch('/api/processes/kill', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ pids }) });
        if (response.ok) {
            showToast(`成功结束 ${pids.length} 个进程`, 'success');
            hideBatchKillModal();
            selectedProcesses.clear();
            loadProcesses();
        } else {
            const error = await response.json();
            showToast(`结束进程失败: ${error.message}`, 'error');
        }
    } catch (error) {
        showToast(`结束进程时发生错误: ${error.message}`, 'error');
    }
}

function confirmKillProcess(e) {
    e.stopPropagation();
    const pid = parseInt(e.currentTarget.dataset.pid);
    const process = processesData.find(p => p.pid === pid);
    if (process && confirm(`确定要结束进程 "${process.name}" (PID: ${pid}) 吗？\n\n警告：强制结束进程可能导致数据丢失`)) {
        killProcess(pid);
    }
}

async function killProcess(pid) {
    try {
        const response = await fetch(`/api/processes/${pid}/kill`, { method: 'POST' });
        if (response.ok) {
            showToast(`进程 ${pid} 已结束`, 'success');
            loadProcesses();
        } else {
            const error = await response.json();
            showToast(`结束进程失败: ${error.message}`, 'error');
        }
    } catch (error) {
        showToast(`结束进程时发生错误: ${error.message}`, 'error');
    }
}

function showProcessDetail(e) {
    e.stopPropagation();
    const pid = parseInt(e.currentTarget.dataset.pid);
    const process = processesData.find(p => p.pid === pid);
    if (process) showProcessDetailModal(process);
}

function showProcessDetailModal(process) {
    if (detailName) detailName.textContent = process.name;
    if (detailPid) detailPid.textContent = process.pid;
    if (detailCpu) detailCpu.textContent = `${process.cpu.toFixed(2)}%`;
    if (detailMemory) detailMemory.textContent = formatBytes(process.memory);
    if (detailUser) detailUser.textContent = process.user || 'N/A';
    if (detailStatus) detailStatus.textContent = process.status || 'unknown';
    if (detailStartTime) detailStartTime.textContent = process.startTime || 'N/A';
    if (detailPath) detailPath.textContent = process.path || 'N/A';
    window.currentProcessPid = process.pid;
    if (processDetailModal) processDetailModal.classList.add('active');
}

function hideProcessDetailModal() {
    if (processDetailModal) processDetailModal.classList.remove('active');
}

async function killSingleProcess() {
    const pid = window.currentProcessPid;
    if (!pid) return;
    try {
        const response = await fetch(`/api/processes/${pid}/kill`, { method: 'POST' });
        if (response.ok) {
            showToast(`进程 ${pid} 已结束`, 'success');
            hideProcessDetailModal();
            loadProcesses();
        } else {
            const error = await response.json();
            showToast(`结束进程失败: ${error.message}`, 'error');
        }
    } catch (error) {
        showToast(`结束进程时发生错误: ${error.message}`, 'error');
    }
}

async function refreshCurrentProcessDetail() {
    const pid = window.currentProcessPid;
    if (!pid) return;
    try {
        const response = await fetch(`/api/processes/${pid}`);
        if (response.ok) {
            const process = await response.json();
            showProcessDetailModal(process);
            showToast('进程信息已刷新', 'success');
        } else {
            showToast('刷新进程信息失败', 'error');
        }
    } catch (error) {
        showToast(`刷新进程信息时发生错误: ${error.message}`, 'error');
    }
}

function truncateText(text, maxLength) {
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '...';
}

function getStatusClass(status) {
    if (!status) return 'unknown';
    const lowerStatus = status.toLowerCase();
    if (lowerStatus.includes('run')) return 'running';
    if (lowerStatus.includes('sleep')) return 'sleeping';
    if (lowerStatus.includes('stop') || lowerStatus.includes('wait')) return 'stopped';
    return 'unknown';
}

function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => { clearTimeout(timeout); func(...args); };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// ========== 弹窗管理 ==========
const serviceModal = document.getElementById('serviceModal');
const serviceForm = document.getElementById('serviceForm');
const modalTitle = document.getElementById('modalTitle');
const addServiceBtn = document.getElementById('addServiceBtn');
const modalClose = document.getElementById('modalClose');
const cancelBtn = document.getElementById('cancelBtn');

const deleteModal = document.getElementById('deleteModal');
const deleteServiceName = document.getElementById('deleteServiceName');
const cancelDeleteBtn = document.getElementById('cancelDeleteBtn');
const confirmDeleteBtn = document.getElementById('confirmDeleteBtn');
let deletingServiceId = null;

addServiceBtn.addEventListener('click', () => {
    editingServiceId = null;
    modalTitle.textContent = '添加服务';
    serviceForm.reset();
    resetIconUpload();
    document.querySelectorAll('.icon-option').forEach(opt => opt.classList.remove('active'));
    document.querySelector('.icon-option[data-icon="🌐"]').classList.add('active');
    serviceModal.classList.add('active');
});

function openEditModal(id) {
    const service = services.find(s => s.id === id);
    if (!service) return;

    editingServiceId = id;
    modalTitle.textContent = '编辑服务';
    document.getElementById('serviceName').value = service.name;
    document.getElementById('serviceDesc').value = service.description || '';
    document.getElementById('servicePort').value = service.port || '';
    
    // 高级选项
    document.getElementById('serviceLaunchCommand').value = service.launchCommand || '';
    document.getElementById('serviceProcessName').value = service.processName || '';
    // 兼容旧字段（如果元素存在）
    const launchPathEl = document.getElementById('serviceLaunchPath');
    if (launchPathEl) {
        launchPathEl.value = service.launchPath || '';
    }
    
    document.getElementById('serviceAutoStart').checked = service.autoStart || false;

    // 设置图标
    const isImage = service.icon && service.icon.startsWith('/');
    document.getElementById('serviceIcon').value = isImage ? service.icon : '';

    // 重置上传区域
    resetIconUpload();

    if (isImage) {
        // 显示已有图标预览
        showIconPreview(service.icon, '当前图标');
    } else {
        // 选中对应的 emoji
        document.querySelectorAll('.icon-option').forEach(opt => {
            opt.classList.toggle('active', opt.dataset.icon === service.icon);
        });
    }

    serviceModal.classList.add('active');
}

function openDeleteModal(id) {
    const service = services.find(s => s.id === id);
    if (!service) return;

    deletingServiceId = id;
    deleteServiceName.textContent = service.name;
    deleteModal.classList.add('active');
}

function closeModals() {
    serviceModal.classList.remove('active');
    deleteModal.classList.remove('active');
    editingServiceId = null;
    deletingServiceId = null;
    // 重置上传区域
    resetIconUpload();
}

function resetIconUpload() {
    const zone = document.getElementById('iconUploadZone');
    const preview = document.getElementById('uploadPreview');
    zone.classList.remove('has-preview');
    preview.innerHTML = '';
    document.getElementById('iconFileInput').value = '';
}

modalClose.addEventListener('click', closeModals);
cancelBtn.addEventListener('click', closeModals);
cancelDeleteBtn.addEventListener('click', closeModals);

// ========== 图标上传/拖拽 ==========
const iconUploadZone = document.getElementById('iconUploadZone');
const iconFileInput = document.getElementById('iconFileInput');

iconUploadZone.addEventListener('click', () => {
    iconFileInput.click();
});

iconUploadZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    iconUploadZone.classList.add('dragover');
});

iconUploadZone.addEventListener('dragleave', () => {
    iconUploadZone.classList.remove('dragover');
});

iconUploadZone.addEventListener('drop', (e) => {
    e.preventDefault();
    iconUploadZone.classList.remove('dragover');
    const files = e.dataTransfer.files;
    if (files.length > 0) {
        handleIconFile(files[0]);
    }
});

iconFileInput.addEventListener('change', (e) => {
    if (e.target.files.length > 0) {
        handleIconFile(e.target.files[0]);
    }
});

async function handleIconFile(file) {
    // 验证文件类型
    if (!file.type.startsWith('image/')) {
        alert('请选择图片文件');
        return;
    }

    // 验证文件大小
    if (file.size > 2 * 1024 * 1024) {
        alert('文件过大，最大 2MB');
        return;
    }

    // 上传文件
    const formData = new FormData();
    formData.append('icon', file);

    try {
        const response = await fetch('/api/upload-icon', {
            method: 'POST',
            body: formData
        });
        const result = await response.json();

        if (result.success) {
            // 显示预览
            showIconPreview(result.icon, file.name);
            // 设置到隐藏字段
            document.getElementById('serviceIcon').value = result.icon;
            // 取消 emoji 选择
            document.querySelectorAll('.icon-option').forEach(o => o.classList.remove('active'));
        } else {
            alert('上传失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('上传失败');
    }
}

function showIconPreview(iconUrl, fileName) {
    const zone = document.getElementById('iconUploadZone');
    const preview = document.getElementById('uploadPreview');

    zone.classList.add('has-preview');
    preview.innerHTML = `
          <img src="${iconUrl}" alt="icon" />
          <div class="preview-info">
            <div class="preview-name">${fileName}</div>
            <div class="preview-change">点击更换</div>
          </div>
        `;
}

// 图标选择 (Emoji)
document.querySelectorAll('.icon-option').forEach(opt => {
    opt.addEventListener('click', () => {
        document.querySelectorAll('.icon-option').forEach(o => o.classList.remove('active'));
        opt.classList.add('active');
        document.getElementById('serviceIcon').value = '';
        resetIconUpload();
    });
});


// 保存服务
serviceForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    const activeIcon = document.querySelector('.icon-option.active');
    const customIcon = document.getElementById('serviceIcon').value.trim();
    const icon = customIcon || (activeIcon ? activeIcon.dataset.icon : '🌐');

    // 读取高级选项
    const launchCommand = document.getElementById('serviceLaunchCommand').value.trim();
    const processName = document.getElementById('serviceProcessName').value.trim();
    // 兼容旧字段（如果元素存在）
    const launchPathEl = document.getElementById('serviceLaunchPath');
    const launchPath = launchPathEl ? launchPathEl.value.trim() : '';

    // 验证：如果配置了启动命令，进程名必填
    if (launchCommand && !processName) {
        alert('配置启动命令时，进程名必须填写');
        return;
    }

    const data = {
        name: document.getElementById('serviceName').value.trim(),
        description: document.getElementById('serviceDesc').value.trim(),
        port: parseInt(document.getElementById('servicePort').value) || 0,
        icon: icon,
        enabled: true, // 允许本地应用（端口为0）
        autoStart: document.getElementById('serviceAutoStart').checked
    };

    // 优先使用高级选项，否则使用旧字段
    if (launchCommand && processName) {
        data.launchCommand = launchCommand;
        data.processName = processName;
    } else if (launchPath) {
        data.launchPath = launchPath; // 向后兼容
    }

    try {
        let response;
        let serviceId = editingServiceId;

        if (editingServiceId) {
            response = await fetch(`/api/services/${editingServiceId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        } else {
            response = await fetch('/api/services', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (response.ok) {
                const newService = await response.json();
                serviceId = newService.id;
            }
        }

        if (response.ok) {
            // 设置服务自启
            if (serviceId) {
                try {
                    await fetch(`/api/services/${serviceId}/autostart`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ autoStart: data.autoStart })
                    });
                } catch (e) {
                    console.log('设置自启失败');
                }
            }

            closeModals();
            await loadServices();
        }
    } catch (e) {
        console.log('保存失败');
    }
});

// 删除服务
confirmDeleteBtn.addEventListener('click', async () => {
    if (!deletingServiceId) return;

    try {
        const response = await fetch(`/api/services/${deletingServiceId}`, {
            method: 'DELETE'
        });
        if (response.ok) {
            closeModals();
            await loadServices();
        }
    } catch (e) {
        console.log('删除失败');
    }
});

// 点击遮罩关闭
serviceModal.addEventListener('click', (e) => {
    if (e.target === serviceModal) closeModals();
});
deleteModal.addEventListener('click', (e) => {
    if (e.target === deleteModal) closeModals();
});

// ========== 页面导航 ==========
document.querySelectorAll('.nav-item').forEach(item => {
    item.addEventListener('click', () => {
        const page = item.dataset.page;
        switchPage(page);
        sidebar.classList.remove('open');
        // 更新按钮箭头方向
        if (sidebarToggleBtn) sidebarToggleBtn.classList.remove('sidebar-open');
    });
});

function switchPage(pageName) {
    if (pageName !== 'frpc' && typeof stopFrpcPolling === 'function') stopFrpcPolling();

    document.querySelectorAll('.nav-item').forEach(nav => {
        nav.classList.toggle('active', nav.dataset.page === pageName);
    });
    document.querySelectorAll('.page-view').forEach(view => {
        view.classList.toggle('active', view.id === `page-${pageName}`);
    });

    // WebSocket 始终保持连接（用于顶部栏状态显示）
    connectMonitorWs();

    if (pageName === 'process') {
        loadProcesses();
    } else if (pageName === 'files') {
        loadWebdavRoot();
        loadFiles(currentFilePath);
        updateWebdavUrl();
    } else if (pageName === 'ssh') {
        connectTerminal();
    } else if (pageName === 'docker') {
        loadDockerContainers();
    } else if (pageName === 'settings') {
        loadAppConfig();
    } else if (pageName === 'frpc') {
        if (typeof loadFrpcData === 'function') loadFrpcData();
    } else if (pageName === 'ai') {
        loadComfyUIConfig();
        loadWorkflows();
    } else if (pageName === 'ai') {
        loadComfyUIConfig();
        loadWorkflows();
    }
}

sidebarToggleBtn.addEventListener('click', () => {
    sidebar.classList.toggle('open');
    // 切换按钮箭头方向
    sidebarToggleBtn.classList.toggle('sidebar-open', sidebar.classList.contains('open'));
});

// ========== WebSocket 监控 ==========
function connectMonitorWs() {
    if (monitorWs && monitorWs.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/monitor`;

    monitorWs = new WebSocket(wsUrl);

    monitorWs.onopen = () => {
        updateConnectionStatus(true);
        if (reconnectTimer) {
            clearTimeout(reconnectTimer);
            reconnectTimer = null;
        }
    };

    monitorWs.onmessage = (event) => {
        const stats = JSON.parse(event.data);
        updateMonitorUI(stats);

        if (document.getElementById('page-monitor')?.classList.contains('active')) {
            updateDisksEnhanced(stats.disks);
        }
    };

    monitorWs.onclose = () => {
        updateConnectionStatus(false);
        if (document.getElementById('page-monitor').classList.contains('active')) {
            reconnectTimer = setTimeout(connectMonitorWs, 3000);
        }
    };

    monitorWs.onerror = () => {
        updateConnectionStatus(false);
    };
}

function disconnectMonitorWs() {
    if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
    if (monitorWs) {
        monitorWs.close();
        monitorWs = null;
    }
}

function updateConnectionStatus(connected) {
    const dot = document.getElementById('statusDot');
    const text = document.getElementById('statusText');
    dot.classList.toggle('connected', connected);
    text.textContent = connected ? '已连接 · 实时更新中' : '连接断开 · 正在重连...';
}

function updateTopBarStats(stats) {
    // CPU
    document.getElementById('topCpu').textContent = Math.round(stats.cpu.usage) + '%';

    // 内存
    document.getElementById('topMem').textContent = Math.round(stats.memory.usedPercent) + '%';

    // GPU
    if (stats.gpu.available) {
        document.getElementById('topGpu').textContent = Math.round(stats.gpu.usage) + '%';
    } else {
        document.getElementById('topGpu').textContent = 'N/A';
    }

    // 网络
    if (stats.network) {
        document.getElementById('topNetUp').textContent = formatSpeedShort(stats.network.speedSent);
        document.getElementById('topNetDown').textContent = formatSpeedShort(stats.network.speedRecv);
    }
}

// 测量网页延迟（使用Performance API）
async function measureWebPing() {
    try {
        const start = performance.now();
        await fetch('/api/ping', { method: 'GET', cache: 'no-cache' });
        const end = performance.now();
        const latency = Math.round(end - start);
        document.getElementById('topWebPing').textContent = latency + 'ms';
    } catch (e) {
        document.getElementById('topWebPing').textContent = '--';
    }
}


function formatSpeedShort(bytesPerSec) {
    if (bytesPerSec === 0) return '0';
    const k = 1024;
    if (bytesPerSec < k) return bytesPerSec + 'B';
    if (bytesPerSec < k * k) return Math.round(bytesPerSec / k) + 'K';
    if (bytesPerSec < k * k * k) return (bytesPerSec / k / k).toFixed(1) + 'M';
    return (bytesPerSec / k / k / k).toFixed(1) + 'G';
}

// ========== 监控图表（原 monitor-charts.js） ==========
const chartConfig = {
    type: 'line',
    options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 300, easing: 'easeOutQuart' },
        interaction: { mode: 'index', intersect: false },
        plugins: {
            legend: { display: false },
            tooltip: {
                backgroundColor: 'rgba(15, 23, 42, 0.9)',
                titleColor: '#f0f4f8',
                bodyColor: '#f0f4f8',
                borderColor: 'rgba(255, 255, 255, 0.1)',
                borderWidth: 1,
                cornerRadius: 8,
                displayColors: false,
                callbacks: { label: function(context) { return context.parsed.y + '%'; } }
            }
        },
        scales: {
            x: { display: false, grid: { display: false } },
            y: {
                beginAtZero: true,
                max: 100,
                grid: { color: 'rgba(255, 255, 255, 0.05)', drawBorder: false },
                ticks: {
                    color: 'rgba(148, 163, 184, 0.6)',
                    font: { size: 10 },
                    callback: function(value) { return value + '%'; }
                }
            }
        },
        elements: { line: { tension: 0.4, borderWidth: 2 }, point: { radius: 0, hitRadius: 10, hoverRadius: 4 } }
    }
};

const dataHistory = { cpu: [], memory: [], gpu: [], network: { upload: [], download: [] } };
const MAX_DATA_POINTS = 60;

function initCharts() {
    const cpuCtx = document.getElementById('cpuChart')?.getContext('2d');
    if (cpuCtx && typeof Chart !== 'undefined') {
        window.cpuChart = new Chart(cpuCtx, {
            ...chartConfig,
            data: {
                labels: Array(MAX_DATA_POINTS).fill(''),
                datasets: [{
                    label: 'CPU',
                    data: Array(MAX_DATA_POINTS).fill(0),
                    borderColor: '#6366f1',
                    backgroundColor: (context) => {
                        const ctx = context.chart.ctx;
                        const gradient = ctx.createLinearGradient(0, 0, 0, 200);
                        gradient.addColorStop(0, 'rgba(99, 102, 241, 0.3)');
                        gradient.addColorStop(1, 'rgba(99, 102, 241, 0)');
                        return gradient;
                    },
                    fill: true
                }]
            }
        });
    }
    const memoryCtx = document.getElementById('memoryChart')?.getContext('2d');
    if (memoryCtx && typeof Chart !== 'undefined') {
        window.memoryChart = new Chart(memoryCtx, {
            ...chartConfig,
            data: {
                labels: Array(MAX_DATA_POINTS).fill(''),
                datasets: [{
                    label: '内存',
                    data: Array(MAX_DATA_POINTS).fill(0),
                    borderColor: '#10b981',
                    backgroundColor: (context) => {
                        const ctx = context.chart.ctx;
                        const gradient = ctx.createLinearGradient(0, 0, 0, 200);
                        gradient.addColorStop(0, 'rgba(16, 185, 129, 0.3)');
                        gradient.addColorStop(1, 'rgba(16, 185, 129, 0)');
                        return gradient;
                    },
                    fill: true
                }]
            }
        });
    }
    const gpuCtx = document.getElementById('gpuChart')?.getContext('2d');
    if (gpuCtx && typeof Chart !== 'undefined') {
        window.gpuChart = new Chart(gpuCtx, {
            ...chartConfig,
            data: {
                labels: Array(MAX_DATA_POINTS).fill(''),
                datasets: [{
                    label: 'GPU',
                    data: Array(MAX_DATA_POINTS).fill(0),
                    borderColor: '#f59e0b',
                    backgroundColor: (context) => {
                        const ctx = context.chart.ctx;
                        const gradient = ctx.createLinearGradient(0, 0, 0, 200);
                        gradient.addColorStop(0, 'rgba(245, 158, 11, 0.3)');
                        gradient.addColorStop(1, 'rgba(245, 158, 11, 0)');
                        return gradient;
                    },
                    fill: true
                }]
            }
        });
    }
    const networkCtx = document.getElementById('networkChart')?.getContext('2d');
    if (networkCtx && typeof Chart !== 'undefined') {
        window.networkChart = new Chart(networkCtx, {
            type: 'line',
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 300, easing: 'easeOutQuart' },
                interaction: { mode: 'index', intersect: false },
                plugins: {
                    legend: { display: true, labels: { color: 'rgba(148, 163, 184, 0.8)', font: { size: 11 }, boxWidth: 12, padding: 10 } },
                    tooltip: {
                        backgroundColor: 'rgba(15, 23, 42, 0.9)',
                        titleColor: '#f0f4f8',
                        bodyColor: '#f0f4f8',
                        borderColor: 'rgba(255, 255, 255, 0.1)',
                        borderWidth: 1,
                        cornerRadius: 8,
                        callbacks: { label: function(context) { return context.dataset.label + ': ' + formatSpeed(context.parsed.y); } }
                    }
                },
                scales: {
                    x: { display: false, grid: { display: false } },
                    y: {
                        beginAtZero: true,
                        grid: { color: 'rgba(255, 255, 255, 0.05)', drawBorder: false },
                        ticks: { color: 'rgba(148, 163, 184, 0.6)', font: { size: 10 }, callback: function(value) { return formatSpeed(value); } }
                    }
                },
                elements: { line: { tension: 0.4, borderWidth: 2 }, point: { radius: 0, hitRadius: 10, hoverRadius: 4 } }
            },
            data: {
                labels: Array(MAX_DATA_POINTS).fill(''),
                datasets: [
                    { label: '上传', data: Array(MAX_DATA_POINTS).fill(0), borderColor: '#34d399', backgroundColor: 'rgba(52, 211, 153, 0.1)', fill: true },
                    { label: '下载', data: Array(MAX_DATA_POINTS).fill(0), borderColor: '#60a5fa', backgroundColor: 'rgba(96, 165, 250, 0.1)', fill: true }
                ]
            }
        });
    }
}

function updateCharts(data) {
    if (window.cpuChart && data.cpu) {
        dataHistory.cpu.push(data.cpu.percent || 0);
        if (dataHistory.cpu.length > MAX_DATA_POINTS) dataHistory.cpu.shift();
        window.cpuChart.data.datasets[0].data = [...dataHistory.cpu];
        window.cpuChart.update('none');
        const cpuValue = document.getElementById('currentCpu');
        if (cpuValue) { cpuValue.textContent = (data.cpu.percent || 0) + '%'; cpuValue.style.color = getColorByValue(data.cpu.percent || 0); }
    }
    if (window.memoryChart && data.memory) {
        const memPercent = data.memory.usedPercent || 0;
        dataHistory.memory.push(memPercent);
        if (dataHistory.memory.length > MAX_DATA_POINTS) dataHistory.memory.shift();
        window.memoryChart.data.datasets[0].data = [...dataHistory.memory];
        window.memoryChart.update('none');
        const memValue = document.getElementById('currentMemory');
        if (memValue) { memValue.textContent = memPercent + '%'; memValue.style.color = getColorByValue(memPercent); }
    }
    if (window.gpuChart && data.gpu && data.gpu.available) {
        const gpuPercent = data.gpu.usage || 0;
        dataHistory.gpu.push(gpuPercent);
        if (dataHistory.gpu.length > MAX_DATA_POINTS) dataHistory.gpu.shift();
        window.gpuChart.data.datasets[0].data = [...dataHistory.gpu];
        window.gpuChart.update('none');
        const gpuValue = document.getElementById('currentGpu');
        if (gpuValue) { gpuValue.textContent = gpuPercent + '%'; gpuValue.style.color = getColorByValue(gpuPercent); }
    }
    if (window.networkChart && data.network) {
        const uploadSpeed = data.network.uploadSpeed || 0;
        const downloadSpeed = data.network.downloadSpeed || 0;
        dataHistory.network.upload.push(uploadSpeed);
        dataHistory.network.download.push(downloadSpeed);
        if (dataHistory.network.upload.length > MAX_DATA_POINTS) {
            dataHistory.network.upload.shift();
            dataHistory.network.download.shift();
        }
        window.networkChart.data.datasets[0].data = [...dataHistory.network.upload];
        window.networkChart.data.datasets[1].data = [...dataHistory.network.download];
        window.networkChart.update('none');
    }
}

function getColorByValue(value) {
    if (value < 50) return '#10b981';
    if (value < 80) return '#f59e0b';
    return '#ef4444';
}

function initTimeRangeSelector() {
    document.querySelectorAll('.time-range-selector .time-btn').forEach(btn => {
        btn.addEventListener('click', function() {
            document.querySelectorAll('.time-range-selector .time-btn').forEach(b => b.classList.remove('active'));
            this.classList.add('active');
        });
    });
}

function enhanceDiskDisplay(disks) {
    const container = document.getElementById('diskListEnhanced');
    if (!container || !disks) return;
    container.innerHTML = disks.map(disk => {
        const usedPercent = Math.round((disk.used / disk.total) * 100);
        const colorClass = usedPercent > 90 ? 'critical' : usedPercent > 70 ? 'warning' : 'normal';
        const letter = disk.letter || disk.mountPoint || '-';
        return `
            <div class="disk-item-enhanced ${colorClass}">
                <div class="disk-header">
                    <div class="disk-info">
                        <span class="disk-letter">${letter}</span>
                        <span class="disk-name">${disk.name || '本地磁盘'}</span>
                    </div>
                    <div class="disk-usage">
                        <span class="disk-percent">${usedPercent}%</span>
                        <span class="disk-size">${formatBytes(disk.used)} / ${formatBytes(disk.total)}</span>
                    </div>
                </div>
                <div class="disk-progress">
                    <div class="disk-progress-bar" style="width: ${usedPercent}%"></div>
                </div>
            </div>
        `;
    }).join('');
}

function formatUptime(seconds) {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (days > 0) return `${days}天 ${hours}小时`;
    if (hours > 0) return `${hours}小时 ${minutes}分钟`;
    return `${minutes}分钟`;
}

function updateSystemInfo(data) {
    const uptimeEl = document.getElementById('uptime');
    if (uptimeEl && data.uptime) uptimeEl.textContent = formatUptime(data.uptime);
    const processCountEl = document.getElementById('processCount');
    if (processCountEl && data.processCount) processCountEl.textContent = data.processCount.toLocaleString();
    const systemLoadEl = document.getElementById('systemLoad');
    if (systemLoadEl && data.load) systemLoadEl.textContent = data.load.toFixed(2);
    const osInfoEl = document.getElementById('osInfo');
    if (osInfoEl && data.os) osInfoEl.textContent = `${data.os.name} ${data.os.version}`;
}

function updateMonitorUI(stats) {
    // 更新顶部栏状态小图标
    updateTopBarStats(stats);

    updateRing('cpuRing', stats.cpu.usage);
    const cpuValEl = document.getElementById('cpuValue') || document.getElementById('currentCpu');
    if (cpuValEl) cpuValEl.textContent = Math.round(stats.cpu.usage) + '%';
    const cpuModelEl = document.getElementById('cpuModel');
    if (cpuModelEl) cpuModelEl.textContent = stats.cpu.modelName || '-';
    const cpuCoresEl = document.getElementById('cpuCores');
    if (cpuCoresEl) cpuCoresEl.textContent = `${stats.cpu.cores} 核心`;

    // CPU 温度
    const cpuTempEl = document.getElementById('cpuTemp');
    if (cpuTempEl) {
        if (stats.cpu.temperature > 0) {
            cpuTempEl.style.display = 'flex';
            const tempVal = cpuTempEl.querySelector('.temp-value');
            if (tempVal) tempVal.textContent = Math.round(stats.cpu.temperature);
            else cpuTempEl.textContent = Math.round(stats.cpu.temperature) + '°C';
            cpuTempEl.className = 'temp-badge' + getTempClass(stats.cpu.temperature);
        } else {
            cpuTempEl.style.display = 'none';
        }
    }

    updateRing('memoryRing', stats.memory.usedPercent);
    const memValEl = document.getElementById('memoryValue') || document.getElementById('currentMemory');
    if (memValEl) memValEl.textContent = Math.round(stats.memory.usedPercent) + '%';
    const memUsedEl = document.getElementById('memoryUsed');
    if (memUsedEl) memUsedEl.textContent = `${formatBytes(stats.memory.used)} / ${formatBytes(stats.memory.total)}`;
    const memAvailEl = document.getElementById('memoryAvailable');
    if (memAvailEl) memAvailEl.textContent = `可用: ${formatBytes(stats.memory.available)}`;

    const gpuContentEl = document.getElementById('gpuContent');
    const gpuUnavailEl = document.getElementById('gpuUnavailable');
    if (stats.gpu.available) {
        if (gpuContentEl) gpuContentEl.style.display = 'block';
        if (gpuUnavailEl) gpuUnavailEl.style.display = 'none';
        updateRing('gpuRing', stats.gpu.usage);
        const gpuValEl = document.getElementById('gpuValue') || document.getElementById('currentGpu');
        if (gpuValEl) gpuValEl.textContent = Math.round(stats.gpu.usage) + '%';
        const gpuModelEl = document.getElementById('gpuModel');
        if (gpuModelEl) gpuModelEl.textContent = stats.gpu.name || '-';
        const gpuMemEl = document.getElementById('gpuMemory');
        if (gpuMemEl) gpuMemEl.textContent = `显存: ${stats.gpu.memoryUsed} / ${stats.gpu.memoryTotal} MB`;

        const gpuTempEl = document.getElementById('gpuTemp');
        if (gpuTempEl && stats.gpu.temperature > 0) {
            gpuTempEl.style.display = 'flex';
            const gpuTempVal = gpuTempEl.querySelector('.temp-value');
            if (gpuTempVal) gpuTempVal.textContent = Math.round(stats.gpu.temperature);
            else gpuTempEl.textContent = Math.round(stats.gpu.temperature) + '°C';
            gpuTempEl.className = 'temp-badge' + getTempClass(stats.gpu.temperature);
        }
    } else {
        if (gpuContentEl) gpuContentEl.style.display = 'none';
        if (gpuUnavailEl) gpuUnavailEl.style.display = 'block';
    }

    // 网络流量
    if (stats.network) {
        const netUp = document.getElementById('netSpeedUp');
        if (netUp) netUp.textContent = formatSpeed(stats.network.speedSent);
        const netDown = document.getElementById('netSpeedDown');
        if (netDown) netDown.textContent = formatSpeed(stats.network.speedRecv);
        const totalUp = document.getElementById('netTotalUp');
        if (totalUp) totalUp.textContent = formatBytes(stats.network.bytesSent);
        const totalDown = document.getElementById('netTotalDown');
        if (totalDown) totalDown.textContent = formatBytes(stats.network.bytesRecv);
    }

    updateDisks(stats.disks);

    updateCharts({
        cpu: { percent: stats.cpu.usage },
        memory: { usedPercent: stats.memory.usedPercent },
        gpu: stats.gpu.available ? { usage: stats.gpu.usage, available: true } : { available: false },
        network: stats.network ? { uploadSpeed: stats.network.speedSent, downloadSpeed: stats.network.speedRecv } : null
    });
    updateSystemInfo({ uptime: stats.uptime, processCount: stats.processCount, load: stats.load, os: stats.os });
}

function getTempClass(temp) {
    if (temp >= 80) return ' temp-danger';
    if (temp >= 60) return ' temp-warning';
    return '';
}

function updateRing(id, percent) {
    const ring = document.getElementById(id);
    if (!ring) return;
    const circumference = 2 * Math.PI * 60;
    const offset = circumference - (percent / 100) * circumference;
    ring.style.strokeDashoffset = offset;
}

function updateDisks(disks) {
    const container = document.getElementById('diskList');
    if (container) {
        container.innerHTML = disks.map(disk => {
            const percent = disk.usedPercent;
            let barClass = '';
            if (percent >= 90) barClass = 'danger';
            else if (percent >= 75) barClass = 'warning';

            return `
                <div class="disk-item">
                  <div class="disk-header">
                    <span class="disk-name">${disk.mountPoint}</span>
                    <span class="disk-usage">${formatBytes(disk.used)} / ${formatBytes(disk.total)}</span>
                  </div>
                  <div class="disk-bar">
                    <div class="disk-bar-fill ${barClass}" style="width: ${percent}%"></div>
                  </div>
                </div>
              `;
        }).join('');
    }
}

function updateDisksEnhanced(disks) {
    enhanceDiskDisplay(disks);
}

function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatSpeed(bytesPerSec) {
    if (bytesPerSec === 0) return '0 B/s';
    const k = 1024;
    const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
    const i = Math.floor(Math.log(bytesPerSec) / Math.log(k));
    return parseFloat((bytesPerSec / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// ========== 设置管理 ==========
async function loadSettingsFromServer() {
    try {
        const response = await fetch('/api/settings');
        if (response.ok) {
            currentSettings = await response.json();
        }
    } catch (e) {
        console.log('无法加载服务器设置');
    }
    applySettings();
}

function saveSettingsToServer() {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(async () => {
        try {
            await fetch('/api/settings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(currentSettings)
            });
        } catch (e) {
            console.log('保存设置失败');
        }
    }, 500);
}

async function loadPresetBackgrounds() {
    try {
        const response = await fetch('/api/backgrounds');
        if (response.ok) {
            presetBackgrounds = await response.json();
        }
    } catch (e) {
        console.log('无法加载预设背景');
    }
    renderBackgroundOptions();
}

function applySettings() {
    if (currentSettings.serverIp) {
        serverIpInput.value = currentSettings.serverIp;
    }
    if (currentSettings.backgroundUrl) {
        bgLayer.style.backgroundImage = `url('${currentSettings.backgroundUrl}')`;
    }
    applyTheme(currentSettings.theme || 'dark');
}

function applyTheme(theme) {
    const t = theme || 'dark';
    document.documentElement.setAttribute('data-theme', t);
    const options = document.getElementById('themeOptions');
    if (options) {
        options.querySelectorAll('.theme-option').forEach(btn => {
            btn.classList.toggle('active', btn.getAttribute('data-theme') === t);
        });
    }
}

(function initThemeOptions() {
    const options = document.getElementById('themeOptions');
    if (!options) return;
    options.addEventListener('click', (e) => {
        const btn = e.target.closest('.theme-option');
        if (!btn) return;
        const theme = btn.getAttribute('data-theme');
        if (theme) {
            currentSettings.theme = theme;
            applyTheme(theme);
            saveSettingsToServer();
        }
    });
})();

serverIpInput.addEventListener('input', (e) => {
    const ip = e.target.value.trim();
    if (ip) {
        currentSettings.serverIp = ip;
        saveSettingsToServer();
        updateServiceLinks();
    }
});

function renderBackgroundOptions() {
    bgGrid.innerHTML = '';
    presetBackgrounds.forEach(bg => {
        const option = document.createElement('div');
        option.className = 'bg-option' + (currentSettings.backgroundUrl === bg.url ? ' active' : '');
        option.innerHTML = `<img src="${bg.thumb || bg.url}" alt="${bg.name}" />`;
        option.addEventListener('click', () => setBackground(bg.url));
        bgGrid.appendChild(option);
    });
}

function setBackground(url) {
    bgLayer.style.backgroundImage = `url('${url}')`;
    currentSettings.backgroundUrl = url;
    saveSettingsToServer();
    document.querySelectorAll('.bg-option').forEach((opt, i) => {
        opt.classList.toggle('active', presetBackgrounds[i]?.url === url);
    });
}

settingsBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    settingsPanel.classList.toggle('active');
});

document.addEventListener('click', (e) => {
    if (!settingsPanel.contains(e.target) && e.target !== settingsBtn) {
        settingsPanel.classList.remove('active');
    }
});

// ========== 事件绑定 ==========
document.getElementById('pingAllBtn').addEventListener('click', pingAllServices);
document.getElementById('importTemplateBtn').addEventListener('click', importTemplate);
document.getElementById('emptyImportBtn').addEventListener('click', importTemplate);
document.getElementById('fetchFaviconBtn').addEventListener('click', fetchFavicon);

// ========== 文件管理 ==========
async function loadFiles(path) {
    currentFilePath = path;
    const tbody = document.getElementById('fileTableBody');
    tbody.innerHTML = '<tr><td colspan="5" class="loading-row">加载中...</td></tr>';

    try {
        const response = await fetch(`/api/files?path=${encodeURIComponent(path)}`);
        if (response.ok) {
            const data = await response.json();
            renderFiles(data.files || []);
            renderBreadcrumb(path);
        } else {
            tbody.innerHTML = '<tr><td colspan="5" class="loading-row">加载失败</td></tr>';
        }
    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="5" class="loading-row">加载失败</td></tr>';
    }
}

function renderFiles(files) {
    const tbody = document.getElementById('fileTableBody');
    if (!files || files.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="loading-row">文件夹为空</td></tr>';
        return;
    }

    const folderIconSvg = '<svg class="file-list-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
        const fileIconSvg = '<svg class="file-list-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>';
        tbody.innerHTML = files.map(file => {
        const icon = file.isDir ? folderIconSvg : fileIconSvg;
        const size = file.isDir ? '-' : formatBytes(file.size);
        const time = new Date(file.modTime).toLocaleString('zh-CN');

        return `
            <tr class="file-row" data-path="${file.path}" data-isdir="${file.isDir}">
              <td class="col-icon">${icon}</td>
              <td class="col-name">${file.name}</td>
              <td class="col-size">${size}</td>
              <td class="col-time">${time}</td>
              <td class="col-actions">
                ${file.isDir ? '' : `<button class="btn-icon" onclick="downloadFile('${file.path}')" title="下载">下载</button>`}
                <button class="btn-icon btn-danger-icon" onclick="openDeleteFileModal('${file.path}', '${file.name}')" title="删除">删除</button>
              </td>
            </tr>
          `;
    }).join('');

    // 绑定双击事件进入文件夹
    document.querySelectorAll('.file-row').forEach(row => {
        row.addEventListener('dblclick', () => {
            if (row.dataset.isdir === 'true') {
                loadFiles(row.dataset.path);
            }
        });
    });
}

function renderBreadcrumb(path) {
    const breadcrumb = document.getElementById('breadcrumb');
    const parts = path.split('/').filter(p => p);

    let html = `<span class="breadcrumb-item" data-path="/" onclick="loadFiles('/')">根目录</span>`;
    let currentPath = '';

    parts.forEach((part, index) => {
        currentPath += '/' + part;
        const isLast = index === parts.length - 1;
        html += `<span class="breadcrumb-sep">/</span>`;
        html += `<span class="breadcrumb-item${isLast ? ' active' : ''}" data-path="${currentPath}" onclick="loadFiles('${currentPath}')">${part}</span>`;
    });

    breadcrumb.innerHTML = html;
}

function getFileIcon(filename) {
    // 仅用于扩展名分类，图标由 renderFiles 中统一 SVG 展示
    const ext = filename.split('.').pop().toLowerCase();
    const types = ['pdf','doc','docx','txt','xls','xlsx','csv','ppt','pptx','jpg','jpeg','png','gif','webp','svg','mp3','wav','flac','aac','mp4','mkv','avi','mov','wmv','zip','rar','7z','tar','gz','exe','msi','bat','sh','js','ts','py','go','java','html','css','json','xml'];
    return types.includes(ext) ? ext : 'file';
}

function downloadFile(path) {
    window.open(`/api/files/download?path=${encodeURIComponent(path)}`, '_blank');
}

function openDeleteFileModal(path, name) {
    deletingFilePath = path;
    document.getElementById('deleteFileName').textContent = name;
    document.getElementById('deleteFileModal').classList.add('active');
}

function closeFileModals() {
    document.getElementById('newFolderModal').classList.remove('active');
    document.getElementById('deleteFileModal').classList.remove('active');
    deletingFilePath = null;
}

// 新建文件夹
document.getElementById('newFolderBtn').addEventListener('click', () => {
    document.getElementById('folderName').value = '';
    document.getElementById('newFolderModal').classList.add('active');
});

document.getElementById('closeFolderModal').addEventListener('click', closeFileModals);
document.getElementById('cancelFolderBtn').addEventListener('click', closeFileModals);
document.getElementById('cancelDeleteFileBtn').addEventListener('click', closeFileModals);

document.getElementById('confirmFolderBtn').addEventListener('click', async () => {
    const name = document.getElementById('folderName').value.trim();
    if (!name) {
        alert('请输入文件夹名称');
        return;
    }

    const newPath = currentFilePath === '/' ? '/' + name : currentFilePath + '/' + name;

    try {
        const response = await fetch('/api/files/mkdir', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: newPath })
        });

        if (response.ok) {
            closeFileModals();
            loadFiles(currentFilePath);
        } else {
            alert('创建文件夹失败');
        }
    } catch (e) {
        alert('创建文件夹失败');
    }
});

// 删除文件
document.getElementById('confirmDeleteFileBtn').addEventListener('click', async () => {
    if (!deletingFilePath) return;

    try {
        const response = await fetch(`/api/files?path=${encodeURIComponent(deletingFilePath)}`, {
            method: 'DELETE'
        });

        if (response.ok) {
            closeFileModals();
            loadFiles(currentFilePath);
        } else {
            alert('删除失败');
        }
    } catch (e) {
        alert('删除失败');
    }
});

// 上传文件
document.getElementById('uploadFileBtn').addEventListener('click', () => {
    document.getElementById('fileUploadInput').click();
});

document.getElementById('fileUploadInput').addEventListener('change', async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    for (const file of files) {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('path', currentFilePath);

        try {
            await fetch('/api/files/upload', {
                method: 'POST',
                body: formData
            });
        } catch (e) {
            console.log('上传失败:', file.name);
        }
    }

    e.target.value = '';
    loadFiles(currentFilePath);
});

// WebDAV URL
function updateWebdavUrl() {
    const protocol = window.location.protocol;
    const host = window.location.host;
    document.getElementById('webdavUrl').textContent = `${protocol}//${host}/webdav/`;
}

// 加载 WebDAV 根目录
async function loadWebdavRoot() {
    try {
        const response = await fetch('/api/webdav-root');
        if (response.ok) {
            const data = await response.json();
            document.getElementById('webdavRootInput').value = data.root || '';
        }
    } catch (e) {
        console.log('加载 WebDAV 根目录失败');
    }
}

// 设置 WebDAV 根目录
document.getElementById('setWebdavRootBtn').addEventListener('click', async () => {
    const root = document.getElementById('webdavRootInput').value.trim();
    if (!root) {
        alert('请输入目录路径');
        return;
    }

    try {
        const response = await fetch('/api/webdav-root', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ root })
        });

        const result = await response.json();
        if (response.ok) {
            alert('设置成功！');
            currentFilePath = '/';
            loadFiles('/');
        } else {
            alert('设置失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('设置失败');
    }
});

document.getElementById('copyWebdavBtn').addEventListener('click', () => {
    const url = document.getElementById('webdavUrl').textContent;
    navigator.clipboard.writeText(url).then(() => {
        alert('已复制到剪贴板');
    });
});

// 弹窗关闭
document.getElementById('newFolderModal').addEventListener('click', (e) => {
    if (e.target.id === 'newFolderModal') closeFileModals();
});
document.getElementById('deleteFileModal').addEventListener('click', (e) => {
    if (e.target.id === 'deleteFileModal') closeFileModals();
});

// ========== SSH 终端 ==========
function connectTerminal() {
    if (terminalWs && terminalWs.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/terminal`;

    updateTerminalStatus('connecting');
    terminalWs = new WebSocket(wsUrl);

    terminalWs.onopen = () => {
        updateTerminalStatus('connected');
        // 聚焦输入框
        document.getElementById('terminalInput').focus();
    };

    terminalWs.onmessage = (event) => {
        appendTerminalOutput(event.data);
    };

    terminalWs.onclose = () => {
        updateTerminalStatus('disconnected');
    };

    terminalWs.onerror = () => {
        updateTerminalStatus('error');
    };
}

function updateTerminalStatus(status) {
    const statusEl = document.getElementById('terminalStatus');
    const dot = statusEl.querySelector('.status-dot');
    const text = statusEl.querySelector('span:last-child');

    dot.className = 'status-dot';
    switch (status) {
        case 'connected':
            dot.classList.add('connected');
            text.textContent = '已连接';
            break;
        case 'connecting':
            text.textContent = '连接中...';
            break;
        case 'disconnected':
            text.textContent = '已断开';
            break;
        case 'error':
            dot.classList.add('error');
            text.textContent = '连接错误';
            break;
    }
}

function appendTerminalOutput(text) {
    const output = document.getElementById('terminalOutput');
    const line = document.createElement('div');
    line.className = 'terminal-line';
    // 解析 ANSI 颜色代码
    let html = text
        .replace(/\x1b\[31m/g, '<span class="text-red">')
        .replace(/\x1b\[32m/g, '<span class="text-green">')
        .replace(/\x1b\[33m/g, '<span class="text-yellow">')
        .replace(/\x1b\[34m/g, '<span class="text-blue">')
        .replace(/\x1b\[35m/g, '<span class="text-magenta">')
        .replace(/\x1b\[36m/g, '<span class="text-cyan">')
        .replace(/\x1b\[0m/g, '</span>');
    line.innerHTML = html;
    output.appendChild(line);

    // 滚动到底部
    const wrapper = document.getElementById('terminal');
    wrapper.scrollTop = wrapper.scrollHeight;
}

function sendTerminalCommand() {
    const input = document.getElementById('terminalInput');
    const cmd = input.value;
    if (!terminalWs || terminalWs.readyState !== WebSocket.OPEN) {
        appendTerminalOutput('<span class="text-red">未连接到终端</span>');
        return;
    }

    // 显示输入的命令（带提示符）
    const prompt = document.getElementById('terminalPrompt').textContent;
    appendTerminalOutput(`<span class="text-green">${prompt}</span> ${cmd}`);

    // 保存到历史（非空命令）
    if (cmd.trim()) {
        terminalHistory.push(cmd);
        historyIndex = terminalHistory.length;
    }

    // 发送命令
    terminalWs.send(cmd);
    input.value = '';
}

// 终端输入事件
document.getElementById('terminalInput').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        e.preventDefault();
        sendTerminalCommand();
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (historyIndex > 0) {
            historyIndex--;
            e.target.value = terminalHistory[historyIndex];
            // 光标移到末尾
            setTimeout(() => e.target.setSelectionRange(e.target.value.length, e.target.value.length), 0);
        }
    } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (historyIndex < terminalHistory.length - 1) {
            historyIndex++;
            e.target.value = terminalHistory[historyIndex];
        } else {
            historyIndex = terminalHistory.length;
            e.target.value = '';
        }
    } else if (e.key === 'c' && e.ctrlKey) {
        // Ctrl+C 中断
        if (terminalWs && terminalWs.readyState === WebSocket.OPEN) {
            terminalWs.send('\x03');
        }
    } else if (e.key === 'l' && e.ctrlKey) {
        // Ctrl+L 清屏
        e.preventDefault();
        document.getElementById('terminalOutput').innerHTML = '';
    }
});

// 点击终端区域聚焦输入框
document.getElementById('terminal').addEventListener('click', () => {
    document.getElementById('terminalInput').focus();
});

document.getElementById('connectTerminalBtn').addEventListener('click', () => {
    if (terminalWs) {
        terminalWs.close();
        terminalWs = null;
    }
    document.getElementById('terminalOutput').innerHTML = '';
    connectTerminal();
});

document.getElementById('clearTerminalBtn').addEventListener('click', () => {
    document.getElementById('terminalOutput').innerHTML = '';
});

// ========== Docker 管理 ==========
async function loadDockerContainers() {
    const tbody = document.getElementById('dockerTableBody');
    const status = document.getElementById('dockerStatus');
    tbody.innerHTML = '<tr><td colspan="5" class="loading-row">加载中...</td></tr>';

    try {
        const response = await fetch('/api/docker/containers');
        if (response.ok) {
            const containers = await response.json();
            renderDockerContainers(containers);

            if (containers && containers.length > 0) {
                status.innerHTML = '<span class="status-dot connected"></span><span>Docker 已连接</span>';
            } else {
                status.innerHTML = '<span class="status-dot"></span><span>未检测到容器</span>';
            }
        } else {
            tbody.innerHTML = '<tr><td colspan="5" class="loading-row">加载失败</td></tr>';
            status.innerHTML = '<span class="status-dot error"></span><span>Docker 未运行或未安装</span>';
        }
    } catch (e) {
        tbody.innerHTML = '<tr><td colspan="5" class="loading-row">无法连接 Docker</td></tr>';
        status.innerHTML = '<span class="status-dot error"></span><span>Docker 未运行或未安装</span>';
    }
}

function renderDockerContainers(containers) {
    const tbody = document.getElementById('dockerTableBody');
    if (!containers || containers.length === 0) {
        tbody.innerHTML = '<tr><td colspan="5" class="loading-row">暂无容器</td></tr>';
        return;
    }

    tbody.innerHTML = containers.map(c => {
        const isRunning = c.state === 'running';
        const statusClass = isRunning ? 'running' : 'stopped';
        const statusDot = `<span class="docker-status-dot ${statusClass}" title="${isRunning ? '运行中' : '已停止'}"></span>`;

        return `
            <tr class="docker-row">
              <td class="col-status">${statusDot}</td>
              <td class="container-name">${c.name}</td>
              <td class="container-image">${c.image}</td>
              <td class="container-status ${statusClass}">${c.status}</td>
              <td class="container-ports">${c.ports || '-'}</td>
            </tr>
          `;
    }).join('');
}

document.getElementById('refreshDockerBtn').addEventListener('click', loadDockerContainers);

// ========== 应用设置 ==========
async function loadAppConfig() {
    try {
        const response = await fetch('/api/app-config');
        if (response.ok) {
            const config = await response.json();
            document.getElementById('appPortInput').value = config.port || '';
            document.getElementById('currentPort').textContent = config.port || '-';
            document.getElementById('appAutoStartToggle').checked = config.autoStart || false;
            document.getElementById('appAutoStartStatus').textContent = config.autoStart ? '已启用' : '未启用';
            const versionEl = document.getElementById('currentAppVersion');
            if (versionEl) versionEl.textContent = config.version ? 'v' + config.version : '-';
        }
    } catch (e) {
        console.log('加载应用配置失败');
    }
}

// 保存端口设置
document.getElementById('savePortBtn').addEventListener('click', async () => {
    const port = document.getElementById('appPortInput').value.trim();
    if (!port) {
        alert('请输入端口号');
        return;
    }

    if (!/^\d+$/.test(port)) {
        alert('端口号必须是数字');
        return;
    }

    const portNum = parseInt(port);
    if (portNum < 1 || portNum > 65535) {
        alert('端口号必须在 1-65535 之间');
        return;
    }

    try {
        const response = await fetch('/api/app-config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ port: port })
        });

        const result = await response.json();
        if (response.ok) {
            alert('端口设置已保存！请重启应用使新端口生效。');
            loadAppConfig();
        } else {
            alert('保存失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('保存失败');
    }
});

// 应用开机自启切换
document.getElementById('appAutoStartToggle').addEventListener('change', async (e) => {
    const enabled = e.target.checked;

    try {
        const response = await fetch('/api/app-config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ autoStart: enabled })
        });

        const result = await response.json();
        if (response.ok) {
            document.getElementById('appAutoStartStatus').textContent = enabled ? '已启用' : '未启用';
        } else {
            e.target.checked = !enabled; // 恢复原状态
            alert('设置失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        e.target.checked = !enabled; // 恢复原状态
        alert('设置失败');
    }
});

// 检查更新（供设置页按钮与自动检查调用）
async function checkForUpdate(silent = false) {
    try {
        const response = await fetch('/api/update-check');
        const data = await response.json();
        if (data.hasUpdate) {
            if (silent) {
                showToast('发现新版本 ' + data.latestVersion + '，请到设置中检查更新', 'info');
            } else {
                const msg = `发现新版本 ${data.latestVersion}。${data.releaseNotes ? '\n' + data.releaseNotes : ''}`;
                if (data.downloadUrl) {
                    if (confirm(msg + '\n\n是否打开下载链接？')) {
                        window.open(data.downloadUrl, '_blank');
                    }
                } else {
                    showToast('发现新版本 ' + data.latestVersion, 'info');
                }
            }
        } else {
            if (!silent) showToast(data.currentVersion ? '当前已是最新版本（v' + data.currentVersion + '）' : '当前已是最新版本', 'success');
        }
        if (data.error && !silent) showToast('检查更新失败: ' + data.error, 'warning');
        return data;
    } catch (e) {
        if (!silent) showToast('检查更新失败，请稍后再试', 'warning');
        return { hasUpdate: false, error: String(e.message) };
    }
}

document.getElementById('checkUpdateBtn')?.addEventListener('click', async () => {
    const btn = document.getElementById('checkUpdateBtn');
    if (btn.disabled) return;
    btn.disabled = true;
    btn.textContent = '检查中...';
    await checkForUpdate(false);
    btn.disabled = false;
    btn.textContent = '检查更新';
});

// 重启应用
document.getElementById('restartAppBtn').addEventListener('click', async () => {
    if (!confirm('确定要重启面板吗？应用将在1秒后重启。')) {
        return;
    }

    try {
        const response = await fetch('/api/app/restart', {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            alert('应用正在重启，请稍候...');
        } else {
            alert('重启失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('重启失败');
    }
});

// ========== AI绘画 ==========
let workflows = []; // 工作流列表
let currentWorkflow = null; // 当前执行的工作流
let comfyUIConfig = { serverUrl: '' }; // ComfyUI配置

// Logo绘画工作流（预设）
const logoWorkflow = {
    id: 'logo-painting',
    name: 'Logo绘画',
    description: '生成应用Logo图标',
    icon: '',
    workflow: {
        "1": {
            "inputs": {
                "samples": ["6", 0],
                "vae": ["4", 2]
            },
            "class_type": "VAEDecode",
            "_meta": { "title": "VAE解码" }
        },
        "2": {
            "inputs": {
                "filename_prefix": "2loras_test_",
                "images": ["1", 0]
            },
            "class_type": "SaveImage",
            "_meta": { "title": "保存图像" }
        },
        "4": {
            "inputs": {
                "ckpt_name": "sd_xl_base_1.0.safetensors"
            },
            "class_type": "CheckpointLoaderSimple",
            "_meta": { "title": "Checkpoint加载器（简易）" }
        },
        "5": {
            "inputs": {
                "lora_name": "LogoRedmondV2-Logo-LogoRedmAF.safetensors",
                "strength_model": 0.75,
                "strength_clip": 1,
                "model": ["4", 0],
                "clip": ["4", 1]
            },
            "class_type": "LoraLoader",
            "_meta": { "title": "加载LoRA" }
        },
        "6": {
            "inputs": {
                "seed": 870945276144950,
                "steps": 30,
                "cfg": 7,
                "sampler_name": "dpmpp_2m",
                "scheduler": "karras",
                "denoise": 1,
                "model": ["5", 0],
                "positive": ["9", 0],
                "negative": ["8", 0],
                "latent_image": ["7", 0]
            },
            "class_type": "KSampler",
            "_meta": { "title": "K采样器" }
        },
        "7": {
            "inputs": {
                "width": 768,
                "height": 768,
                "batch_size": 1
            },
            "class_type": "EmptyLatentImage",
            "_meta": { "title": "空Latent图像" }
        },
        "8": {
            "inputs": {
                "text": "sketchy, low quality, blurry, distorted text, messy, busy background, multiple icons, flat 2D, cartoon, bright aggressive colors, shadows on background.",
                "clip": ["5", 1]
            },
            "class_type": "CLIPTextEncode",
            "_meta": { "title": "CLIP文本编码" }
        },
        "9": {
            "inputs": {
                "text": "gold app icon",
                "clip": ["5", 1]
            },
            "class_type": "CLIPTextEncode",
            "_meta": { "title": "CLIP文本编码" }
        }
    },
    parameters: [
        { key: "9.text", label: "正面提示词", type: "text", default: "gold app icon", description: "描述想要生成的内容" },
        { key: "8.text", label: "负面提示词", type: "text", default: "sketchy, low quality, blurry, distorted text, messy, busy background, multiple icons, flat 2D, cartoon, bright aggressive colors, shadows on background.", description: "描述不想要的内容" },
        { key: "6.seed", label: "随机种子", type: "number", default: "", description: "留空则随机生成" },
        { key: "6.steps", label: "采样步数", type: "number", default: "30", description: "采样步数，越多质量越好但速度越慢" },
        { key: "6.cfg", label: "CFG Scale", type: "number", default: "7", description: "提示词引导强度" },
        { key: "7.width", label: "图像宽度", type: "number", default: "768", description: "生成图像的宽度（像素）" },
        { key: "7.height", label: "图像高度", type: "number", default: "768", description: "生成图像的高度（像素）" }
    ]
};

async function loadComfyUIConfig() {
    try {
        const response = await fetch('/api/comfyui/config');
        if (response.ok) {
            comfyUIConfig = await response.json();
            document.getElementById('comfyuiServerUrl').value = comfyUIConfig.serverUrl || '';
            const urlDisplay = document.getElementById('currentComfyUIUrlDisplay');
            if (comfyUIConfig.serverUrl) {
                urlDisplay.textContent = comfyUIConfig.serverUrl;
                urlDisplay.style.opacity = '1';
            } else {
                urlDisplay.textContent = '未配置';
                urlDisplay.style.opacity = '0.5';
            }
        }
    } catch (e) {
        console.log('加载ComfyUI配置失败');
    }
}

async function saveComfyUIConfig() {
    const serverUrl = document.getElementById('comfyuiServerUrl').value.trim();
    if (!serverUrl) {
        alert('请输入服务器地址');
        return;
    }

    try {
        const response = await fetch('/api/comfyui/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ serverUrl })
        });

        const result = await response.json();
        if (response.ok) {
            comfyUIConfig.serverUrl = serverUrl;
            const urlDisplay = document.getElementById('currentComfyUIUrlDisplay');
            urlDisplay.textContent = serverUrl;
            urlDisplay.style.opacity = '1';
            document.getElementById('comfyuiConfigModal').classList.remove('show');
            alert('配置已保存');
        } else {
            alert('保存失败: ' + (result.error || '未知错误'));
        }
    } catch (e) {
        alert('保存失败');
    }
}

async function testComfyUIConnection() {
    if (!comfyUIConfig.serverUrl) {
        alert('请先配置ComfyUI服务器地址');
        return;
    }

    try {
        const response = await fetch(`${comfyUIConfig.serverUrl}/system_stats`, {
            method: 'GET'
        });
        if (response.ok) {
            alert('连接成功！');
        } else {
            alert('连接失败: ' + response.statusText);
        }
    } catch (e) {
        alert('连接失败: ' + e.message);
    }
}

function loadWorkflows() {
    // 加载预设工作流
    workflows = [logoWorkflow];
    renderWorkflows();
}

function renderWorkflows() {
    const grid = document.getElementById('workflowGrid');
    grid.innerHTML = workflows.map(workflow => `
          <div class="workflow-card" data-id="${workflow.id}">
            <div class="workflow-icon">${workflow.icon}</div>
            <h3>${workflow.name}</h3>
            <p>${workflow.description || ''}</p>
            <button class="btn-primary workflow-execute-btn" data-id="${workflow.id}">执行</button>
          </div>
        `).join('');

    // 绑定执行按钮事件
    document.querySelectorAll('.workflow-execute-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const workflowId = btn.dataset.id;
            const workflow = workflows.find(w => w.id === workflowId);
            if (workflow) {
                openWorkflowExecuteModal(workflow);
            }
        });
    });
}

function openWorkflowExecuteModal(workflow) {
    currentWorkflow = workflow;
    document.getElementById('workflowExecuteTitle').textContent = `执行: ${workflow.name}`;

    // 生成参数表单
    const form = document.getElementById('workflowParamsForm');
    form.innerHTML = workflow.parameters.map(param => `
          <div class="form-group">
            <label>${param.label}</label>
            ${param.type === 'text' ?
            `<textarea class="form-control" data-key="${param.key}" placeholder="${param.description}" rows="3">${param.default || ''}</textarea>` :
            `<input type="${param.type}" class="form-control" data-key="${param.key}" value="${param.default || ''}" placeholder="${param.description}" />`
        }
            <small class="form-hint">${param.description}</small>
          </div>
        `).join('');

    // 重置进度和结果
    document.getElementById('workflowProgress').style.display = 'none';
    document.getElementById('workflowResult').style.display = 'none';
    document.getElementById('executeWorkflowBtn').disabled = false;

    document.getElementById('workflowExecuteModal').classList.add('show');
}

async function executeWorkflow() {
    if (!comfyUIConfig.serverUrl) {
        alert('请先配置ComfyUI服务器地址');
        return;
    }

    const btn = document.getElementById('executeWorkflowBtn');
    btn.disabled = true;

    // 收集参数
    const params = {};
    document.querySelectorAll('#workflowParamsForm [data-key]').forEach(input => {
        const key = input.dataset.key;
        const value = input.type === 'number' ? (input.value ? parseFloat(input.value) : '') : input.value;
        params[key] = value;
    });

    // 构建工作流（替换参数）
    const workflow = JSON.parse(JSON.stringify(currentWorkflow.workflow));
    for (const [key, value] of Object.entries(params)) {
        const [nodeId, inputName] = key.split('.');
        if (workflow[nodeId] && workflow[nodeId].inputs) {
            if (inputName === 'seed' && value === '') {
                // 随机种子留空则生成随机数
                workflow[nodeId].inputs[inputName] = Math.floor(Math.random() * 1000000000000000);
            } else if (value !== '') {
                workflow[nodeId].inputs[inputName] = value;
            }
        }
    }

    // 显示进度（弹窗和header）
    document.getElementById('workflowProgress').style.display = 'block';
    document.getElementById('workflowProgressFill').style.width = '0%';
    document.getElementById('workflowProgressText').textContent = '正在提交工作流...';
    document.getElementById('workflowProgressHeader').style.display = 'block';
    document.getElementById('workflowProgressFillHeader').style.width = '0%';
    document.getElementById('workflowProgressTextHeader').textContent = '正在提交工作流...';

    try {
        const response = await fetch('/api/comfyui/workflow/execute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ workflow })
        });

        const result = await response.json();
        if (response.ok) {
            // 开始轮询状态
            pollWorkflowStatus(result.promptId);
        } else {
            alert('执行失败: ' + (result.error || '未知错误'));
            btn.disabled = false;
            document.getElementById('workflowProgress').style.display = 'none';
            document.getElementById('workflowProgressHeader').style.display = 'none';
        }
    } catch (e) {
        alert('执行失败: ' + e.message);
        btn.disabled = false;
        document.getElementById('workflowProgress').style.display = 'none';
        document.getElementById('workflowProgressHeader').style.display = 'none';
    }
}

async function pollWorkflowStatus(promptId) {
    const maxAttempts = 120; // 最多轮询2分钟
    let attempts = 0;

    const poll = async () => {
        attempts++;
        try {
            const response = await fetch(`/api/comfyui/workflow/status/${promptId}`);
            if (response.ok) {
                const status = await response.json();

                // 更新进度（弹窗和header）
                const progress = status.progress || 0;
                document.getElementById('workflowProgressFill').style.width = progress + '%';
                document.getElementById('workflowProgressText').textContent = status.message || `执行中... (${progress}%)`;
                document.getElementById('workflowProgressFillHeader').style.width = progress + '%';
                document.getElementById('workflowProgressTextHeader').textContent = status.message || `执行中... (${progress}%)`;

                if (status.completed) {
                    // 执行完成，显示结果
                    document.getElementById('workflowProgress').style.display = 'none';
                    document.getElementById('workflowProgressHeader').style.display = 'none';
                    displayWorkflowResult(status.images || []);
                    document.getElementById('executeWorkflowBtn').disabled = false;
                    return;
                }

                if (status.failed) {
                    alert('工作流执行失败: ' + (status.error || '未知错误'));
                    document.getElementById('workflowProgress').style.display = 'none';
                    document.getElementById('workflowProgressHeader').style.display = 'none';
                    document.getElementById('executeWorkflowBtn').disabled = false;
                    return;
                }

                if (attempts < maxAttempts) {
                    setTimeout(poll, 1000); // 每秒轮询一次
                } else {
                    alert('执行超时');
                    document.getElementById('workflowProgress').style.display = 'none';
                    document.getElementById('workflowProgressHeader').style.display = 'none';
                    document.getElementById('executeWorkflowBtn').disabled = false;
                }
            }
        } catch (e) {
            console.log('查询状态失败:', e);
            if (attempts < maxAttempts) {
                setTimeout(poll, 2000); // 失败后2秒重试
            } else {
                alert('查询状态失败');
                document.getElementById('workflowProgress').style.display = 'none';
                document.getElementById('workflowProgressHeader').style.display = 'none';
                document.getElementById('executeWorkflowBtn').disabled = false;
            }
        }
    };

    poll();
}

function displayWorkflowResult(images) {
    const resultDiv = document.getElementById('workflowResult');
    const imagesDiv = document.getElementById('workflowResultImages');

    if (images.length === 0) {
        imagesDiv.innerHTML = '<p>未生成图片</p>';
    } else {
        imagesDiv.innerHTML = images.map(img => `
            <div class="workflow-result-image">
              <img src="${img.url}" alt="生成结果" />
              <a href="${img.url}" download class="btn-secondary" style="margin-top: 8px; display: inline-block;">下载</a>
            </div>
          `).join('');
    }

    resultDiv.style.display = 'block';
}

// 绑定AI绘画事件
document.getElementById('aiConfigBtn').addEventListener('click', () => {
    document.getElementById('comfyuiConfigModal').classList.add('show');
});
document.getElementById('saveComfyUIConfigBtn').addEventListener('click', saveComfyUIConfig);
document.getElementById('cancelComfyUIConfigBtn').addEventListener('click', () => {
    document.getElementById('comfyuiConfigModal').classList.remove('show');
});
document.getElementById('closeComfyUIConfigModal').addEventListener('click', () => {
    document.getElementById('comfyuiConfigModal').classList.remove('show');
});
document.getElementById('testComfyUIConnectionBtn').addEventListener('click', testComfyUIConnection);
document.getElementById('executeWorkflowBtn').addEventListener('click', executeWorkflow);
document.getElementById('cancelWorkflowExecuteBtn').addEventListener('click', () => {
    document.getElementById('workflowExecuteModal').classList.remove('show');
});
document.getElementById('closeWorkflowExecuteModal').addEventListener('click', () => {
    document.getElementById('workflowExecuteModal').classList.remove('show');
});

// ========== 初始化 ==========
document.addEventListener('DOMContentLoaded', async () => {
    await loadSettingsFromServer();
    await loadServices();
    await loadPresetBackgrounds();

    if (document.getElementById('page-process')) initProcessManagement();
    if (document.getElementById('cpuChart') && typeof Chart !== 'undefined') {
        initCharts();
        initTimeRangeSelector();
    }

    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            const activeModal = document.querySelector('.modal-overlay.active, .modal-overlay.show');
            if (activeModal) activeModal.classList.remove('active', 'show');
        }
    });

    // 连接 WebSocket 以更新顶部栏状态
    connectMonitorWs();

    // 初始化ping测量
    measureWebPing();

    // 每5秒更新一次ping
    setInterval(() => {
        measureWebPing();
    }, 5000);

    // 首页加载完成后自动检测连通性
    setTimeout(pingAllServices, 1000);

    // 检测所有服务的进程状态
    setTimeout(checkAllServiceProcesses, 1500);

    // 每 30 秒自动刷新连通状态
    pingInterval = setInterval(() => {
        if (document.getElementById('page-home').classList.contains('active')) {
            pingAllServices();
        }
    }, 30000);

    // 每 5 秒自动刷新进程状态
    processCheckInterval = setInterval(() => {
        if (document.getElementById('page-home').classList.contains('active')) {
            checkAllServiceProcesses();
        }
    }, 5000);

    // 延迟自动检查更新（静默：仅在有新版本时提示）
    setTimeout(() => checkForUpdate(true), 3000);

    // 初始化侧边栏按钮状态
    if (sidebarToggleBtn && sidebar) {
        sidebarToggleBtn.classList.toggle('sidebar-open', sidebar.classList.contains('open'));
    }
});