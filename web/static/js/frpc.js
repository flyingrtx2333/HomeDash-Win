// ========== FRPC 内网穿透 ==========
let frpcPollTimer = null;
let frpcProxies = []; // 简单模式下的代理列表
const FRPC_MODE_KEY = 'frpcConfigMode';

function getFrpcMode() {
    return localStorage.getItem(FRPC_MODE_KEY) || 'simple';
}

function setFrpcMode(mode) {
    localStorage.setItem(FRPC_MODE_KEY, mode);
}

async function loadFrpcData() {
    if (!document.getElementById('page-frpc')) return;

    const mode = getFrpcMode();
    document.querySelectorAll('.frpc-mode-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.mode === mode);
    });
    const simpleForm = document.getElementById('frpcSimpleForm');
    const proForm = document.getElementById('frpcProForm');
    if (simpleForm) simpleForm.style.display = mode === 'simple' ? 'block' : 'none';
    if (proForm) proForm.style.display = mode === 'pro' ? 'flex' : 'none';

    await Promise.all([
        loadFrpcConfig(),
        loadFrpcStatus(),
        loadFrpcAutoStart()
    ]);

    if (frpcPollTimer) clearInterval(frpcPollTimer);
    frpcPollTimer = setInterval(loadFrpcStatus, 3000);
}

function stopFrpcPolling() {
    if (frpcPollTimer) {
        clearInterval(frpcPollTimer);
        frpcPollTimer = null;
    }
}

async function loadFrpcConfig() {
    const mode = getFrpcMode();
    if (mode === 'simple') {
        await loadFrpcConfigParsed();
    } else {
        await loadFrpcConfigRaw();
    }
}

async function loadFrpcConfigParsed() {
    const serverAddr = document.getElementById('frpcServerAddr');
    const serverPort = document.getElementById('frpcServerPort');
    const token = document.getElementById('frpcToken');
    if (!serverAddr) return;

    try {
        const response = await fetch('/api/frpc/config/parsed');
        if (response.ok) {
            const data = await response.json();
            serverAddr.value = data.serverAddr || '';
            serverPort.value = data.serverPort || 7000;
            token.value = data.token || '';
            frpcProxies = data.proxies && Array.isArray(data.proxies) ? data.proxies : [];
            renderFrpcProxies();
        }
    } catch (e) {
        console.log('加载 frpc 解析配置失败');
    }
}

async function loadFrpcConfigRaw() {
    const textarea = document.getElementById('frpcConfigTextarea');
    if (!textarea) return;

    try {
        const response = await fetch('/api/frpc/config');
        if (response.ok) {
            const data = await response.json();
            textarea.value = data.config || '';
        }
    } catch (e) {
        console.log('加载 frpc 配置失败');
    }
}

function renderFrpcProxies() {
    const list = document.getElementById('frpcProxyList');
    const empty = document.getElementById('frpcProxyEmpty');
    if (!list || !empty) return;

    if (frpcProxies.length === 0) {
        list.innerHTML = '';
        list.style.display = 'none';
        empty.style.display = 'block';
        return;
    }

    list.style.display = 'flex';
    empty.style.display = 'none';
    list.innerHTML = frpcProxies.map((p, i) => `
        <div class="frpc-proxy-item" data-index="${i}">
            <span class="frpc-proxy-item-info">${escapeHtml(p.name || '未命名')}<span>${p.type || 'tcp'} | ${p.localIP || '127.0.0.1'}:${p.localPort || 0} → ${p.remotePort || 0}</span></span>
            <div class="frpc-proxy-item-actions">
                <button type="button" class="frpc-proxy-edit" data-index="${i}">编辑</button>
                <button type="button" class="frpc-proxy-del" data-index="${i}">删除</button>
            </div>
        </div>
    `).join('');

    list.querySelectorAll('.frpc-proxy-edit').forEach(btn => {
        btn.addEventListener('click', () => openFrpcProxyModal(parseInt(btn.dataset.index)));
    });
    list.querySelectorAll('.frpc-proxy-del').forEach(btn => {
        btn.addEventListener('click', () => {
            const idx = parseInt(btn.dataset.index);
            frpcProxies.splice(idx, 1);
            renderFrpcProxies();
        });
    });
}

function escapeHtml(s) {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
}

function openFrpcProxyModal(editIndex) {
    const modal = document.getElementById('frpcProxyModal');
    const title = document.getElementById('frpcProxyModalTitle');
    const editInput = document.getElementById('frpcProxyEditIndex');
    const nameEl = document.getElementById('frpcProxyName');
    const typeEl = document.getElementById('frpcProxyType');
    const localIPEl = document.getElementById('frpcProxyLocalIP');
    const localPortEl = document.getElementById('frpcProxyLocalPort');
    const remotePortEl = document.getElementById('frpcProxyRemotePort');

    if (editIndex >= 0 && frpcProxies[editIndex]) {
        const p = frpcProxies[editIndex];
        title.textContent = '编辑应用';
        editInput.value = String(editIndex);
        nameEl.value = p.name || '';
        typeEl.value = p.type || 'tcp';
        localIPEl.value = p.localIP || '127.0.0.1';
        localPortEl.value = p.localPort || '';
        remotePortEl.value = p.remotePort || '';
    } else {
        title.textContent = '添加应用';
        editInput.value = '-1';
        nameEl.value = '';
        typeEl.value = 'tcp';
        localIPEl.value = '127.0.0.1';
        localPortEl.value = '';
        remotePortEl.value = '';
    }
    modal.classList.add('active');
}

function closeFrpcProxyModal() {
    const modal = document.getElementById('frpcProxyModal');
    if (modal) modal.classList.remove('active');
}

function confirmFrpcProxyModal() {
    const editInput = document.getElementById('frpcProxyEditIndex');
    const nameEl = document.getElementById('frpcProxyName');
    const typeEl = document.getElementById('frpcProxyType');
    const localIPEl = document.getElementById('frpcProxyLocalIP');
    const localPortEl = document.getElementById('frpcProxyLocalPort');
    const remotePortEl = document.getElementById('frpcProxyRemotePort');

    const name = (nameEl?.value || '').trim();
    if (!name) {
        if (typeof showToast === 'function') showToast('请输入应用名', 'warning');
        return;
    }

    const item = {
        name: name,
        type: typeEl?.value || 'tcp',
        localIP: (localIPEl?.value || '127.0.0.1').trim(),
        localPort: parseInt(localPortEl?.value, 10) || 0,
        remotePort: parseInt(remotePortEl?.value, 10) || 0
    };

    const idx = parseInt(editInput?.value, 10);
    if (idx >= 0 && idx < frpcProxies.length) {
        frpcProxies[idx] = item;
    } else {
        frpcProxies.push(item);
    }
    renderFrpcProxies();
    closeFrpcProxyModal();
    saveFrpcConfigSimple();
}

async function saveFrpcConfigSimple() {
    const serverAddr = document.getElementById('frpcServerAddr')?.value?.trim() || '';
    const serverPort = parseInt(document.getElementById('frpcServerPort')?.value, 10) || 7000;
    const token = document.getElementById('frpcToken')?.value?.trim() || '';

    try {
        const response = await fetch('/api/frpc/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                serverAddr,
                serverPort,
                token,
                proxies: frpcProxies
            })
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('配置已保存', 'success');
        } else {
            if (typeof showToast === 'function') showToast(result.error || '保存失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('保存失败', 'error');
    }
}

async function saveFrpcConfigPro() {
    const textarea = document.getElementById('frpcConfigTextarea');
    if (!textarea) return;

    try {
        const response = await fetch('/api/frpc/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ config: textarea.value })
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('配置已保存', 'success');
        } else {
            if (typeof showToast === 'function') showToast(result.error || '保存失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('保存失败', 'error');
    }
}

async function loadFrpcStatus() {
    const statusEl = document.getElementById('frpcStatusText');
    const pidEl = document.getElementById('frpcPidText');
    const startBtn = document.getElementById('frpcStartBtn');
    const stopBtn = document.getElementById('frpcStopBtn');

    if (!statusEl) return;

    try {
        const response = await fetch('/api/frpc/status');
        if (response.ok) {
            const data = await response.json();
            if (data.running) {
                statusEl.textContent = '运行中';
                statusEl.className = 'frpc-status-value running';
                if (pidEl) pidEl.textContent = data.pid ? ` (PID: ${data.pid})` : '';
                if (startBtn) startBtn.disabled = true;
                if (stopBtn) stopBtn.disabled = false;
            } else {
                statusEl.textContent = '已停止';
                statusEl.className = 'frpc-status-value stopped';
                if (pidEl) pidEl.textContent = '';
                if (startBtn) startBtn.disabled = false;
                if (stopBtn) stopBtn.disabled = true;
            }
        }
    } catch (e) {
        statusEl.textContent = '-';
        statusEl.className = 'frpc-status-value';
    }
}

async function loadFrpcAutoStart() {
    const toggle = document.getElementById('frpcAutoStartToggle');
    const statusEl = document.getElementById('frpcAutoStartStatus');

    if (!toggle || !statusEl) return;

    try {
        const response = await fetch('/api/frpc/autostart');
        if (response.ok) {
            const data = await response.json();
            toggle.checked = data.autoStart || false;
            statusEl.textContent = data.autoStart ? '已启用' : '未启用';
        }
    } catch (e) {
        statusEl.textContent = '-';
    }
}

function initFrpcEvents() {
    const startBtn = document.getElementById('frpcStartBtn');
    const stopBtn = document.getElementById('frpcStopBtn');
    const saveConfigBtn = document.getElementById('frpcSaveConfigBtn');
    const saveSimpleBtn = document.getElementById('frpcSaveSimpleBtn');
    const autoStartToggle = document.getElementById('frpcAutoStartToggle');
    const modeSimple = document.getElementById('frpcModeSimple');
    const modePro = document.getElementById('frpcModePro');
    const addProxyBtn = document.getElementById('frpcAddProxyBtn');
    const proxyModal = document.getElementById('frpcProxyModal');
    const proxyModalCancel = document.getElementById('frpcProxyModalCancel');
    const proxyModalClose = document.getElementById('frpcProxyModalClose');
    const proxyModalConfirm = document.getElementById('frpcProxyModalConfirm');

    // 模式切换
    if (modeSimple) {
        modeSimple.addEventListener('click', () => {
            setFrpcMode('simple');
            loadFrpcData();
        });
    }
    if (modePro) {
        modePro.addEventListener('click', () => {
            setFrpcMode('pro');
            loadFrpcData();
        });
    }

    // 添加应用
    if (addProxyBtn) {
        addProxyBtn.addEventListener('click', () => openFrpcProxyModal(-1));
    }

    // 代理弹窗
    if (proxyModalCancel) proxyModalCancel.addEventListener('click', closeFrpcProxyModal);
    if (proxyModalClose) proxyModalClose.addEventListener('click', closeFrpcProxyModal);
    if (proxyModalConfirm) proxyModalConfirm.addEventListener('click', confirmFrpcProxyModal);
    if (proxyModal) {
        proxyModal.addEventListener('click', (e) => {
            if (e.target === proxyModal) closeFrpcProxyModal();
        });
    }

    if (startBtn) {
        startBtn.addEventListener('click', async () => {
            try {
                const response = await fetch('/api/frpc/start', { method: 'POST' });
                const result = await response.json();
                if (response.ok) {
                    if (typeof showToast === 'function') showToast('frpc 已启动', 'success');
                    await loadFrpcStatus();
                } else {
                    if (typeof showToast === 'function') showToast(result.error || '启动失败', 'error');
                }
            } catch (e) {
                if (typeof showToast === 'function') showToast('启动失败', 'error');
            }
        });
    }

    if (stopBtn) {
        stopBtn.addEventListener('click', async () => {
            try {
                const response = await fetch('/api/frpc/stop', { method: 'POST' });
                const result = await response.json();
                if (response.ok) {
                    if (typeof showToast === 'function') showToast('frpc 已停止', 'success');
                    await loadFrpcStatus();
                } else {
                    if (typeof showToast === 'function') showToast(result.error || '停止失败', 'error');
                }
            } catch (e) {
                if (typeof showToast === 'function') showToast('停止失败', 'error');
            }
        });
    }

    if (saveConfigBtn) {
        saveConfigBtn.addEventListener('click', saveFrpcConfigPro);
    }

    if (saveSimpleBtn) {
        saveSimpleBtn.addEventListener('click', saveFrpcConfigSimple);
    }

    if (autoStartToggle) {
        autoStartToggle.addEventListener('change', async (e) => {
            const enabled = e.target.checked;
            try {
                const response = await fetch('/api/frpc/autostart', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ autoStart: enabled })
                });
                const result = await response.json();
                if (response.ok) {
                    const statusEl = document.getElementById('frpcAutoStartStatus');
                    if (statusEl) statusEl.textContent = enabled ? '已启用' : '未启用';
                    if (typeof showToast === 'function') showToast(enabled ? '已启用开机自启' : '已禁用开机自启', 'success');
                } else {
                    e.target.checked = !enabled;
                    if (typeof showToast === 'function') showToast(result.error || '设置失败', 'error');
                }
            } catch (e) {
                autoStartToggle.checked = !enabled;
                if (typeof showToast === 'function') showToast('设置失败', 'error');
            }
        });
    }
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initFrpcEvents);
} else {
    initFrpcEvents();
}
