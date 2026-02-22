// ========== Docker 管理 ==========
async function loadDockerContainers() {
    console.log('正在加载 Docker 容器...');
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

document.addEventListener('DOMContentLoaded', () => {
    console.log('docker.js loaded');
    loadDockerContainers();
    const refreshDockerBtn = document.getElementById('refreshDockerBtn');
    if (refreshDockerBtn) refreshDockerBtn.addEventListener('click', loadDockerContainers);
});