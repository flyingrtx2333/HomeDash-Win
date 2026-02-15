// ========== FRPC 内网穿透 ==========
let frpcPollTimer = null;

async function loadFrpcData() {
    if (!document.getElementById('page-frpc')) return;

    await Promise.all([
        loadFrpcConfig(),
        loadFrpcStatus(),
        loadFrpcAutoStart()
    ]);

    // 启动状态轮询
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

// 事件绑定（延迟到 DOM 就绪）
function initFrpcEvents() {
    const startBtn = document.getElementById('frpcStartBtn');
    const stopBtn = document.getElementById('frpcStopBtn');
    const saveConfigBtn = document.getElementById('frpcSaveConfigBtn');
    const autoStartToggle = document.getElementById('frpcAutoStartToggle');

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
        saveConfigBtn.addEventListener('click', async () => {
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
        });
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

// DOM 加载后初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initFrpcEvents);
} else {
    initFrpcEvents();
}
