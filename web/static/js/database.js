// ========== 数据库管理 ==========
let databases = [];

// 加载数据库数据
async function loadDatabasesData() {
    await loadDatabases();
}

// 加载数据库列表
async function loadDatabases() {
    try {
        const response = await fetch('/api/databases');
        if (response.ok) {
            databases = await response.json();
            renderDatabases();
        }
    } catch (e) {
        console.log('加载数据库列表失败', e);
    }
}

// 操作按钮图标
const iconsDatabase = {
    edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    test: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>',
    export: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
    import: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>',
    backup: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>',
    password: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>',
    tables: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M9 3H5a2 2 0 0 0-2 2v4m6-6h10a2 2 0 0 1 2 2v4M9 3v18m0 0h10a2 2 0 0 0 2-2V9M9 21H5a2 2 0 0 1-2-2V9m0 0h18"/></svg>',
    trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>'
};

// 渲染数据库列表
function renderDatabases() {
    const tableContainer = document.getElementById('databaseList');
    const tableBody = document.getElementById('databaseTableBody');
    const empty = document.getElementById('databaseEmpty');
    if (!tableContainer || !tableBody || !empty) return;

    if (databases.length === 0) {
        tableContainer.style.display = 'none';
        empty.style.display = 'block';
        return;
    }

    tableContainer.style.display = 'block';
    empty.style.display = 'none';
    tableBody.innerHTML = databases.map(db => {
        const passwordDisplay = db.password ? '••••••••' : '-';
        const typeDisplay = db.type === 'mysql' ? 'MySQL' : db.type === 'redis' ? 'Redis' : db.type === 'mongodb' ? 'MongoDB' : db.type;

        return `
            <div class="database-table-row" data-id="${db.id}">
                <span class="database-td database-td-name">${escapeHtml(db.name || '-')}<button type="button" class="database-btn" onclick="testConnection('${db.id}')" title="测试连接">${iconsDatabase.test}</button></span>
                <span class="database-td database-td-type">${escapeHtml(typeDisplay)}</span>
                <span class="database-td database-td-username">${escapeHtml(db.username || '-')}</span>
                <span class="database-td database-td-password">
                    <span class="database-password-display">${passwordDisplay}</span>
                    <button type="button" class="database-btn database-btn-sm" onclick="togglePasswordDisplay('${db.id}')" title="显示密码">👁</button>
                </span>
                <span class="database-td database-td-note" title="${escapeHtml(db.note || '')}">${escapeHtml(db.note || '-')}</span>
                <span class="database-td database-td-actions">
                    ${db.type === 'mysql' ? `
                        
                        <button type="button" class="database-btn" onclick="showTables('${db.id}')" title="查看表列表">${iconsDatabase.tables}</button>
                        <button type="button" class="database-btn" onclick="changePassword('${db.id}')" title="修改密码">${iconsDatabase.password}</button>
                        <button type="button" class="database-btn" onclick="exportSQL('${db.id}')" title="导出SQL">${iconsDatabase.export}</button>
                        <button type="button" class="database-btn" onclick="importSQL('${db.id}')" title="导入SQL">${iconsDatabase.import}</button>
                    ` : ''}
                    <button type="button" class="database-btn" onclick="showBackupList('${db.id}')" title="备份列表">${iconsDatabase.backup}</button>
                    <button type="button" class="database-btn" onclick="editDatabase('${db.id}')" title="编辑">${iconsDatabase.edit}</button>
                    <button type="button" class="database-btn database-btn-danger" onclick="deleteDatabase('${db.id}')" title="删除">${iconsDatabase.trash}</button>
                </span>
            </div>
        `;
    }).join('');
}

// 切换密码显示
function togglePasswordDisplay(id) {
    const db = databases.find(d => d.id === id);
    if (!db) return;

    const row = document.querySelector(`.database-table-row[data-id="${id}"]`);
    if (!row) return;

    const passwordDisplay = row.querySelector('.database-password-display');
    if (!passwordDisplay) return;

    if (passwordDisplay.textContent === '••••••••') {
        passwordDisplay.textContent = db.password || '-';
    } else {
        passwordDisplay.textContent = '••••••••';
    }
}

// 打开创建/编辑数据库弹窗
function openDatabaseModal(editId = null) {
    const modal = document.getElementById('databaseModal');
    const editIdInput = document.getElementById('databaseEditId');
    const titleEl = document.getElementById('databaseModalTitle');

    if (editId) {
        const db = databases.find(d => d.id === editId);
        if (db) {
            editIdInput.value = editId;
            titleEl.textContent = '编辑数据库';
            document.getElementById('databaseName').value = db.name || '';
            document.getElementById('databaseType').value = db.type || 'mysql';
            document.getElementById('databaseHost').value = db.host || 'localhost';
            document.getElementById('databasePort').value = db.port || '';
            document.getElementById('databaseUsername').value = db.username || '';
            document.getElementById('databasePassword').value = db.password || '';
            document.getElementById('databaseNote').value = db.note || '';
        }
    } else {
        editIdInput.value = '';
        titleEl.textContent = '添加数据库';
        document.getElementById('databaseName').value = '';
        document.getElementById('databaseType').value = 'mysql';
        document.getElementById('databaseHost').value = 'localhost';
        document.getElementById('databasePort').value = '';
        document.getElementById('databaseUsername').value = '';
        document.getElementById('databasePassword').value = '';
        document.getElementById('databaseNote').value = '';
    }

    // 根据类型设置默认端口
    const typeSelect = document.getElementById('databaseType');
    const portInput = document.getElementById('databasePort');
    typeSelect.addEventListener('change', function() {
        if (!portInput.value) {
            if (this.value === 'mysql') portInput.value = '3306';
            else if (this.value === 'redis') portInput.value = '6379';
            else if (this.value === 'mongodb') portInput.value = '27017';
        }
    });

    modal.classList.add('active');
}

// 关闭创建/编辑弹窗
function closeDatabaseModal() {
    const modal = document.getElementById('databaseModal');
    if (modal) modal.classList.remove('active');
}

// 保存数据库
async function saveDatabase() {
    const editId = document.getElementById('databaseEditId').value;
    const name = document.getElementById('databaseName').value.trim();
    const type = document.getElementById('databaseType').value;
    const host = document.getElementById('databaseHost').value.trim() || 'localhost';
    const port = parseInt(document.getElementById('databasePort').value, 10) || 0;
    const username = document.getElementById('databaseUsername').value.trim();
    const password = document.getElementById('databasePassword').value;
    const note = document.getElementById('databaseNote').value.trim();

    if (!name || !username || !password) {
        if (typeof showToast === 'function') showToast('请填写数据库名称、用户名和密码', 'warning');
        return;
    }

    const db = {
        name,
        type,
        host,
        port,
        username,
        password,
        note
    };

    try {
        const url = editId ? `/api/databases/${editId}` : '/api/databases';
        const method = editId ? 'PUT' : 'POST';
        const response = await fetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(db)
        });

        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast(editId ? '数据库已更新' : '数据库已创建', 'success');
            closeDatabaseModal();
            await loadDatabases();
        } else {
            if (typeof showToast === 'function') showToast(result.error || '保存失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('保存失败', 'error');
    }
}

// 编辑数据库
function editDatabase(id) {
    openDatabaseModal(id);
}

// 删除数据库
async function deleteDatabase(id) {
    if (!confirm('确定要删除这个数据库配置吗？')) return;

    try {
        const response = await fetch(`/api/databases/${id}`, {
            method: 'DELETE'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('数据库已删除', 'success');
            await loadDatabases();
        } else {
            if (typeof showToast === 'function') showToast(result.error || '删除失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('删除失败', 'error');
    }
}

// 测试连接
async function testConnection(id) {
    try {
        const response = await fetch(`/api/databases/${id}/test`, {
            method: 'POST'
        });
        const result = await response.json();
        if (result.success) {
            if (typeof showToast === 'function') showToast('连接成功', 'success');
        } else {
            if (typeof showToast === 'function') showToast(result.error || '连接失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('连接失败', 'error');
    }
}

// 修改密码
function changePassword(id) {
    const modal = document.getElementById('changePasswordModal');
    const dbIdInput = document.getElementById('changePasswordDatabaseId');
    dbIdInput.value = id;
    document.getElementById('changePasswordNew').value = '';
    modal.classList.add('active');
}

// 确认修改密码
async function confirmChangePassword() {
    const id = document.getElementById('changePasswordDatabaseId').value;
    const newPassword = document.getElementById('changePasswordNew').value;

    if (!newPassword) {
        if (typeof showToast === 'function') showToast('请输入新密码', 'warning');
        return;
    }

    try {
        const response = await fetch(`/api/databases/${id}/change-password`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ newPassword })
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('密码已修改', 'success');
            document.getElementById('changePasswordModal').classList.remove('active');
            await loadDatabases();
        } else {
            if (typeof showToast === 'function') showToast(result.error || '修改失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('修改失败', 'error');
    }
}

// 导出 SQL
async function exportSQL(id) {
    try {
        const response = await fetch(`/api/databases/${id}/export`);
        if (response.ok) {
            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            const db = databases.find(d => d.id === id);
            a.download = `${db.name || 'database'}_${new Date().getTime()}.sql`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            if (typeof showToast === 'function') showToast('导出成功', 'success');
        } else {
            const result = await response.json();
            if (typeof showToast === 'function') showToast(result.error || '导出失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('导出失败', 'error');
    }
}

// 导入 SQL
function importSQL(id) {
    const modal = document.getElementById('importSQLModal');
    modal.dataset.databaseId = id;
    document.getElementById('importSQLFile').value = '';
    document.getElementById('importSQLConfirmed').checked = false;
    document.getElementById('importSQLConfirm').disabled = true;
    modal.classList.add('active');
}

// 确认导入 SQL
async function confirmImportSQL() {
    const id = document.getElementById('importSQLModal').dataset.databaseId;
    const fileInput = document.getElementById('importSQLFile');
    const confirmed = document.getElementById('importSQLConfirmed').checked;

    if (!fileInput.files || fileInput.files.length === 0) {
        if (typeof showToast === 'function') showToast('请选择 SQL 文件', 'warning');
        return;
    }

    if (!confirmed) {
        if (typeof showToast === 'function') showToast('请确认导入操作', 'warning');
        return;
    }

    const file = fileInput.files[0];
    const reader = new FileReader();
    reader.onload = async function(e) {
        const sql = e.target.result;
        try {
            const response = await fetch(`/api/databases/${id}/import`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ sql, confirmed: true })
            });
            const result = await response.json();
            if (response.ok) {
                if (typeof showToast === 'function') showToast('导入成功', 'success');
                document.getElementById('importSQLModal').classList.remove('active');
            } else {
                if (typeof showToast === 'function') showToast(result.error || '导入失败', 'error');
            }
        } catch (e) {
            if (typeof showToast === 'function') showToast('导入失败', 'error');
        }
    };
    reader.readAsText(file);
}

// 显示备份列表
async function showBackupList(id) {
    const modal = document.getElementById('backupListModal');
    modal.dataset.databaseId = id;
    modal.classList.add('active');
    await loadBackupList(id);
}

// 加载备份列表
async function loadBackupList(id) {
    const container = document.getElementById('backupListContainer');
    container.innerHTML = '<div class="backup-loading">加载中...</div>';

    try {
        const response = await fetch(`/api/databases/${id}/backups`);
        if (response.ok) {
            const backups = await response.json();
            if (backups.length === 0) {
                container.innerHTML = '<div class="backup-empty">暂无备份</div>';
                return;
            }

            container.innerHTML = backups.map(backup => {
                const size = formatFileSize(backup.size);
                const date = new Date(backup.modTime * 1000).toLocaleString('zh-CN');
                return `
                    <div class="backup-item">
                        <div class="backup-info">
                            <div class="backup-filename">${escapeHtml(backup.filename)}</div>
                            <div class="backup-meta">${size} · ${date}</div>
                        </div>
                        <div class="backup-actions">
                            <button type="button" class="database-btn" onclick="downloadBackup('${id}', '${escapeHtml(backup.filename)}')" title="下载">${iconsDatabase.export}</button>
                            <button type="button" class="database-btn database-btn-danger" onclick="deleteBackup('${id}', '${escapeHtml(backup.filename)}')" title="删除">${iconsDatabase.trash}</button>
                        </div>
                    </div>
                `;
            }).join('');
        } else {
            container.innerHTML = '<div class="backup-error">加载失败</div>';
        }
    } catch (e) {
        container.innerHTML = '<div class="backup-error">加载失败</div>';
    }
}

// 创建备份
async function createBackup(id) {
    try {
        const response = await fetch(`/api/databases/${id}/backup`, {
            method: 'POST'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('备份已创建', 'success');
            await loadBackupList(id);
        } else {
            if (typeof showToast === 'function') showToast(result.error || '备份失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('备份失败', 'error');
    }
}

// 下载备份
function downloadBackup(id, filename) {
    window.open(`/api/databases/${id}/backups/${encodeURIComponent(filename)}/download`, '_blank');
}

// 删除备份
async function deleteBackup(id, filename) {
    if (!confirm(`确定要删除备份文件 "${filename}" 吗？`)) return;

    try {
        const response = await fetch(`/api/databases/${id}/backups/${encodeURIComponent(filename)}`, {
            method: 'DELETE'
        });
        const result = await response.json();
        if (response.ok) {
            if (typeof showToast === 'function') showToast('备份已删除', 'success');
            await loadBackupList(id);
        } else {
            if (typeof showToast === 'function') showToast(result.error || '删除失败', 'error');
        }
    } catch (e) {
        if (typeof showToast === 'function') showToast('删除失败', 'error');
    }
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

// 显示表列表
async function showTables(id) {
    const modal = document.getElementById('tablesListModal');
    modal.dataset.databaseId = id;
    modal.classList.add('active');
    await loadTablesList(id);
}

// 加载表列表
async function loadTablesList(id) {
    const container = document.getElementById('tablesListContainer');
    container.innerHTML = '<div class="tables-loading">加载中...</div>';

    try {
        const response = await fetch(`/api/databases/${id}/tables`);
        if (response.ok) {
            const tables = await response.json();
            if (tables.length === 0) {
                container.innerHTML = '<div class="tables-empty">暂无表</div>';
                return;
            }

            container.innerHTML = `
                <div class="tables-table-header">
                    <span class="tables-th tables-th-name">表名</span>
                    <span class="tables-th tables-th-rows">行数</span>
                    <span class="tables-th tables-th-size">数据大小</span>
                    <span class="tables-th tables-th-index">索引大小</span>
                    <span class="tables-th tables-th-engine">引擎</span>
                    <span class="tables-th tables-th-comment">注释</span>
                </div>
                <div class="tables-table-body">
                    ${tables.map(table => {
                        const totalSize = table.dataSize + table.indexSize;
                        return `
                            <div class="tables-table-row">
                                <span class="tables-td tables-td-name">${escapeHtml(table.name)}</span>
                                <span class="tables-td tables-td-rows">${table.rows.toLocaleString()}</span>
                                <span class="tables-td tables-td-size">${formatFileSize(table.dataSize)}</span>
                                <span class="tables-td tables-td-index">${formatFileSize(table.indexSize)}</span>
                                <span class="tables-td tables-td-engine">${escapeHtml(table.engine || '-')}</span>
                                <span class="tables-td tables-td-comment" title="${escapeHtml(table.comment || '')}">${escapeHtml(table.comment || '-')}</span>
                            </div>
                        `;
                    }).join('')}
                </div>
            `;
        } else {
            const result = await response.json();
            container.innerHTML = `<div class="tables-error">加载失败: ${escapeHtml(result.error || '未知错误')}</div>`;
        }
    } catch (e) {
        container.innerHTML = '<div class="tables-error">加载失败</div>';
    }
}

// HTML 转义
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 页面切换监听
document.addEventListener('pageChange', (e) => {
    if (e.detail === 'database') {
        loadDatabasesData();
    }
});

// 页面加载完成后初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() {
        initDatabasePage();
        if (document.getElementById('page-database')) {
            loadDatabasesData();
        }
    });
} else {
    initDatabasePage();
    if (document.getElementById('page-database')) {
        loadDatabasesData();
    }
}

function initDatabasePage() {
    // 添加数据库按钮
    const addBtn = document.getElementById('databaseAddBtn');
    if (addBtn) {
        addBtn.addEventListener('click', () => openDatabaseModal());
    }

    // 弹窗关闭按钮
    const modalClose = document.getElementById('databaseModalClose');
    if (modalClose) {
        modalClose.addEventListener('click', closeDatabaseModal);
    }

    const modalCancel = document.getElementById('databaseModalCancel');
    if (modalCancel) {
        modalCancel.addEventListener('click', closeDatabaseModal);
    }

    // 保存按钮
    const modalConfirm = document.getElementById('databaseModalConfirm');
    if (modalConfirm) {
        modalConfirm.addEventListener('click', saveDatabase);
    }

    // 密码显示切换
    const passwordToggle = document.getElementById('databasePasswordToggle');
    if (passwordToggle) {
        passwordToggle.addEventListener('click', function() {
            const input = document.getElementById('databasePassword');
            if (input.type === 'password') {
                input.type = 'text';
                this.textContent = '🙈';
            } else {
                input.type = 'password';
                this.textContent = '👁';
            }
        });
    }

    // 导入 SQL 弹窗
    const importSQLClose = document.getElementById('importSQLClose');
    if (importSQLClose) {
        importSQLClose.addEventListener('click', () => {
            document.getElementById('importSQLModal').classList.remove('active');
        });
    }

    const importSQLCancel = document.getElementById('importSQLCancel');
    if (importSQLCancel) {
        importSQLCancel.addEventListener('click', () => {
            document.getElementById('importSQLModal').classList.remove('active');
        });
    }

    const importSQLConfirmed = document.getElementById('importSQLConfirmed');
    if (importSQLConfirmed) {
        importSQLConfirmed.addEventListener('change', function() {
            document.getElementById('importSQLConfirm').disabled = !this.checked;
        });
    }

    const importSQLConfirm = document.getElementById('importSQLConfirm');
    if (importSQLConfirm) {
        importSQLConfirm.addEventListener('click', confirmImportSQL);
    }

    // 改密弹窗
    const changePasswordClose = document.getElementById('changePasswordClose');
    if (changePasswordClose) {
        changePasswordClose.addEventListener('click', () => {
            document.getElementById('changePasswordModal').classList.remove('active');
        });
    }

    const changePasswordCancel = document.getElementById('changePasswordCancel');
    if (changePasswordCancel) {
        changePasswordCancel.addEventListener('click', () => {
            document.getElementById('changePasswordModal').classList.remove('active');
        });
    }

    const changePasswordConfirm = document.getElementById('changePasswordConfirm');
    if (changePasswordConfirm) {
        changePasswordConfirm.addEventListener('click', confirmChangePassword);
    }

    const changePasswordToggle = document.getElementById('changePasswordToggle');
    if (changePasswordToggle) {
        changePasswordToggle.addEventListener('click', function() {
            const input = document.getElementById('changePasswordNew');
            if (input.type === 'password') {
                input.type = 'text';
                this.textContent = '🙈';
            } else {
                input.type = 'password';
                this.textContent = '👁';
            }
        });
    }

    // 备份列表弹窗
    const backupListClose = document.getElementById('backupListClose');
    if (backupListClose) {
        backupListClose.addEventListener('click', () => {
            document.getElementById('backupListModal').classList.remove('active');
        });
    }

    const backupListCancel = document.getElementById('backupListCancel');
    if (backupListCancel) {
        backupListCancel.addEventListener('click', () => {
            document.getElementById('backupListModal').classList.remove('active');
        });
    }

    const backupListRefresh = document.getElementById('backupListRefreshBtn');
    if (backupListRefresh) {
        backupListRefresh.addEventListener('click', async function() {
            const id = document.getElementById('backupListModal').dataset.databaseId;
            if (id) await loadBackupList(id);
        });
    }

    // 表列表弹窗
    const tablesListClose = document.getElementById('tablesListClose');
    if (tablesListClose) {
        tablesListClose.addEventListener('click', () => {
            document.getElementById('tablesListModal').classList.remove('active');
        });
    }

    const tablesListCancel = document.getElementById('tablesListCancel');
    if (tablesListCancel) {
        tablesListCancel.addEventListener('click', () => {
            document.getElementById('tablesListModal').classList.remove('active');
        });
    }

    const tablesListRefresh = document.getElementById('tablesListRefreshBtn');
    if (tablesListRefresh) {
        tablesListRefresh.addEventListener('click', async function() {
            const id = document.getElementById('tablesListModal').dataset.databaseId;
            if (id) await loadTablesList(id);
        });
    }
}
