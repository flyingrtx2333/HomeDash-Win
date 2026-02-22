/**
 * 文件管理页 /webdav 独立脚本
 * 依赖：base.js（showToast、currentSettings 由 base 提供）
 */
(function() {
    'use strict';

    let currentFilePath = '/';
    let deletingFilePath = null;

    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    async function loadFiles(path) {
        currentFilePath = path;
        const tbody = document.getElementById('fileTableBody');
        if (!tbody) return;
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
        if (!tbody || !files || files.length === 0) {
            if (tbody) tbody.innerHTML = '<tr><td colspan="5" class="loading-row">文件夹为空</td></tr>';
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
                    ${file.isDir ? '' : `<button class="btn-icon" data-download="${file.path}" title="下载">下载</button>`}
                    <button class="btn-icon btn-danger-icon" data-delete-path="${file.path}" data-delete-name="${file.name}" title="删除">删除</button>
                  </td>
                </tr>
              `;
        }).join('');

        tbody.querySelectorAll('.file-row').forEach(row => {
            row.addEventListener('dblclick', () => {
                if (row.dataset.isdir === 'true') loadFiles(row.dataset.path);
            });
        });
        tbody.querySelectorAll('[data-download]').forEach(btn => {
            btn.addEventListener('click', () => downloadFile(btn.dataset.download));
        });
        tbody.querySelectorAll('[data-delete-path]').forEach(btn => {
            btn.addEventListener('click', () => openDeleteFileModal(btn.dataset.deletePath, btn.dataset.deleteName));
        });
    }

    function renderBreadcrumb(path) {
        const breadcrumb = document.getElementById('breadcrumb');
        if (!breadcrumb) return;
        const parts = path.split('/').filter(p => p);
        let html = `<span class="breadcrumb-item" data-path="/">根目录</span>`;
        let currentPath = '';
        parts.forEach((part, index) => {
            currentPath += '/' + part;
            const isLast = index === parts.length - 1;
            html += `<span class="breadcrumb-sep">/</span>`;
            html += `<span class="breadcrumb-item${isLast ? ' active' : ''}" data-path="${currentPath}">${part}</span>`;
        });
        breadcrumb.innerHTML = html;
        breadcrumb.querySelectorAll('.breadcrumb-item').forEach(span => {
            span.addEventListener('click', () => loadFiles(span.dataset.path));
        });
    }

    function downloadFile(path) {
        window.open(`/api/files/download?path=${encodeURIComponent(path)}`, '_blank');
    }

    function openDeleteFileModal(path, name) {
        deletingFilePath = path;
        const nameEl = document.getElementById('deleteFileName');
        const modal = document.getElementById('deleteFileModal');
        if (nameEl) nameEl.textContent = name;
        if (modal) modal.classList.add('active');
    }

    function closeFileModals() {
        const m1 = document.getElementById('newFolderModal');
        const m2 = document.getElementById('deleteFileModal');
        if (m1) m1.classList.remove('active');
        if (m2) m2.classList.remove('active');
        deletingFilePath = null;
    }

    function updateWebdavUrl() {
        const el = document.getElementById('webdavUrl');
        if (el) el.textContent = `${window.location.protocol}//${window.location.host}/webdav/`;
    }

    async function loadWebdavRoot() {
        try {
            const response = await fetch('/api/webdav-root');
            if (response.ok) {
                const data = await response.json();
                const input = document.getElementById('webdavRootInput');
                if (input) input.value = data.root || '';
            }
        } catch (e) {
            console.log('加载 WebDAV 根目录失败');
        }
    }

    function bindFilePageEvents() {
        const newFolderBtn = document.getElementById('newFolderBtn');
        if (newFolderBtn) {
            newFolderBtn.addEventListener('click', () => {
                const folderName = document.getElementById('folderName');
                const modal = document.getElementById('newFolderModal');
                if (folderName) folderName.value = '';
                if (modal) modal.classList.add('active');
            });
        }

        const closeFolderModal = document.getElementById('closeFolderModal');
        const cancelFolderBtn = document.getElementById('cancelFolderBtn');
        const cancelDeleteFileBtn = document.getElementById('cancelDeleteFileBtn');
        if (closeFolderModal) closeFolderModal.addEventListener('click', closeFileModals);
        if (cancelFolderBtn) cancelFolderBtn.addEventListener('click', closeFileModals);
        if (cancelDeleteFileBtn) cancelDeleteFileBtn.addEventListener('click', closeFileModals);

        const confirmFolderBtn = document.getElementById('confirmFolderBtn');
        if (confirmFolderBtn) {
            confirmFolderBtn.addEventListener('click', async () => {
                const nameEl = document.getElementById('folderName');
                const name = nameEl ? nameEl.value.trim() : '';
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
        }

        const confirmDeleteFileBtn = document.getElementById('confirmDeleteFileBtn');
        if (confirmDeleteFileBtn) {
            confirmDeleteFileBtn.addEventListener('click', async () => {
                if (!deletingFilePath) return;
                try {
                    const response = await fetch(`/api/files?path=${encodeURIComponent(deletingFilePath)}`, { method: 'DELETE' });
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
        }

        const uploadFileBtn = document.getElementById('uploadFileBtn');
        const fileUploadInput = document.getElementById('fileUploadInput');
        if (uploadFileBtn && fileUploadInput) {
            uploadFileBtn.addEventListener('click', () => fileUploadInput.click());
            fileUploadInput.addEventListener('change', async (e) => {
                const files = e.target.files;
                if (!files || files.length === 0) return;
                for (const file of files) {
                    const formData = new FormData();
                    formData.append('file', file);
                    formData.append('path', currentFilePath);
                    try {
                        await fetch('/api/files/upload', { method: 'POST', body: formData });
                    } catch (err) {
                        console.log('上传失败:', file.name);
                    }
                }
                e.target.value = '';
                loadFiles(currentFilePath);
            });
        }

        const setWebdavRootBtn = document.getElementById('setWebdavRootBtn');
        if (setWebdavRootBtn) {
            setWebdavRootBtn.addEventListener('click', async () => {
                const rootEl = document.getElementById('webdavRootInput');
                const root = rootEl ? rootEl.value.trim() : '';
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
        }

        const copyWebdavBtn = document.getElementById('copyWebdavBtn');
        if (copyWebdavBtn) {
            copyWebdavBtn.addEventListener('click', () => {
                const urlEl = document.getElementById('webdavUrl');
                const url = urlEl ? urlEl.textContent : '';
                navigator.clipboard.writeText(url).then(() => alert('已复制到剪贴板'));
            });
        }

        const newFolderModal = document.getElementById('newFolderModal');
        const deleteFileModal = document.getElementById('deleteFileModal');
        if (newFolderModal) newFolderModal.addEventListener('click', (e) => { if (e.target.id === 'newFolderModal') closeFileModals(); });
        if (deleteFileModal) deleteFileModal.addEventListener('click', (e) => { if (e.target.id === 'deleteFileModal') closeFileModals(); });
    }

    function init() {
        if (window.location.pathname !== '/webdav') return;
        loadWebdavRoot();
        loadFiles(currentFilePath);
        updateWebdavUrl();
        bindFilePageEvents();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.loadFiles = loadFiles;
    window.downloadFile = downloadFile;
    window.openDeleteFileModal = openDeleteFileModal;
})();
