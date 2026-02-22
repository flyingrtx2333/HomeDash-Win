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
    if (!servicesGrid) return;
    // 显示/隐藏空状态
    if (services.length === 0) {
        servicesGrid.style.display = 'none';
        if (emptyState) emptyState.style.display = 'flex';
        return;
    }
    servicesGrid.style.display = 'grid';
    if (emptyState) emptyState.style.display = 'none';

    const ip = window.location.hostname;
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
    const ip = window.location.hostname;
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

// 测量网页延迟（使用Performance API）
async function measureWebPing() {
    const el = document.getElementById('topWebPing');
    if (!el) return;
    try {
        const start = performance.now();
        await fetch('/api/ping', { method: 'GET', cache: 'no-cache' });
        const end = performance.now();
        el.textContent = Math.round(end - start) + 'ms';
    } catch (e) {
        el.textContent = '--';
    }
}

// ========== Favicon 抓取 ==========
async function fetchFavicon() {
    const port = document.getElementById('servicePort').value;
    if (!port) {
        alert('请先填写端口号');
        return;
    }

    const ip = window.location.hostname;
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

// ========== 事件绑定（仅当元素存在时绑定，避免多路由下 null） ==========
const pingAllBtn = document.getElementById('pingAllBtn');
const importTemplateBtn = document.getElementById('importTemplateBtn');
const emptyImportBtn = document.getElementById('emptyImportBtn');
const fetchFaviconBtn = document.getElementById('fetchFaviconBtn');
if (pingAllBtn) pingAllBtn.addEventListener('click', pingAllServices);
if (importTemplateBtn) importTemplateBtn.addEventListener('click', importTemplate);
if (emptyImportBtn) emptyImportBtn.addEventListener('click', importTemplate);
if (fetchFaviconBtn) fetchFaviconBtn.addEventListener('click', fetchFavicon);

let deletingServiceId = null;

document.addEventListener('DOMContentLoaded', () => {
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
    // 保存服务
    serviceForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        const activeIcon = document.querySelector('.icon-option.active');
        const customIcon = document.getElementById('serviceIcon').value.trim();
        const icon = customIcon || (activeIcon ? activeIcon.dataset.icon : '🌐');

        // 读取高级选项
        const launchCommand = document.getElementById('serviceLaunchCommand').value.trim();
        const processName = document.getElementById('serviceProcessName').value.trim();
        const workingDir = document.getElementById('serviceWorkingDir').value.trim();
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
        data.workingDir = workingDir;

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

    addServiceBtn.addEventListener('click', () => {
        editingServiceId = null;
        if (modalTitle) modalTitle.textContent = '添加服务';
        if (serviceForm) serviceForm.reset();
        resetIconUpload();
        document.querySelectorAll('.icon-option').forEach(opt => opt.classList.remove('active'));
        const def = document.querySelector('.icon-option[data-icon="🌐"]');
        if (def) def.classList.add('active');
        if (serviceModal) serviceModal.classList.add('active');
    });

    modalClose.addEventListener('click', closeModals);
    cancelBtn.addEventListener('click', closeModals);
    cancelDeleteBtn.addEventListener('click', closeModals);
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
    document.getElementById('serviceWorkingDir').value = service.workingDir || '';
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
    if (deleteServiceName) deleteServiceName.textContent = service.name;
    if (deleteModal) deleteModal.classList.add('active');
}

function closeModals() {
    if (serviceModal) serviceModal.classList.remove('active');
    if (deleteModal) deleteModal.classList.remove('active');
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



document.addEventListener('DOMContentLoaded', async () => {
    await loadServices();

    // 首页加载完成后自动检测连通性
    setTimeout(pingAllServices, 1000);

    // 初始化ping测量
    measureWebPing();

    // 检测所有服务的进程状态
    setTimeout(checkAllServiceProcesses, 1500);

    // 每 30 秒自动刷新连通状态（仅首页）
    pingInterval = setInterval(() => {
        pingAllServices();
    }, 30000);

    // 每 5 秒自动刷新进程状态（仅首页）
    processCheckInterval = setInterval(() => {
        checkAllServiceProcesses();
    }, 5000);

    // 每5秒更新一次ping
    setInterval(() => {
        measureWebPing();
    }, 5000);
});