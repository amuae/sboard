<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>用户管理 - SBoard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <link href="/assets/css/style.css" rel="stylesheet">
    <style>
        body {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            min-height: 100vh;
        }
        
        .user-card {
            background: white;
            border: none;
            border-radius: 16px;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.08);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            margin-bottom: 1rem;
            position: relative;
            overflow: visible;
            animation: fadeInUp 0.5s ease-out;
            z-index: 1;
        }
        
        /* 当卡片内有打开的dropdown时,提升z-index */
        .user-card:has(.dropdown-menu.show) {
            z-index: 1070;
        }
        
        .user-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 4px;
            background: linear-gradient(90deg, #667eea, #764ba2);
            transform: scaleX(0);
            transition: transform 0.3s ease;
            border-radius: 16px 16px 0 0;
            z-index: 1;
        }
        
        .user-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 12px 30px rgba(0, 0, 0, 0.12);
        }
        
        .user-card:hover::before {
            transform: scaleX(1);
        }
        
        /* 确保卡片内容不会溢出圆角 */
        .user-card .card-body {
            border-radius: 16px;
            position: relative;
            z-index: 2;
        }
        
        /* dropdown菜单样式 */
        .user-card .dropdown-menu {
            z-index: 1071;
            box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
            max-height: 400px;
            overflow-y: auto;
        }
        
        /* 滚动条美化 */
        .dropdown-menu::-webkit-scrollbar {
            width: 6px;
        }
        
        .dropdown-menu::-webkit-scrollbar-track {
            background: #f1f1f1;
            border-radius: 3px;
        }
        
        .dropdown-menu::-webkit-scrollbar-thumb {
            background: #888;
            border-radius: 3px;
        }
        
        .dropdown-menu::-webkit-scrollbar-thumb:hover {
            background: #555;
        }
        
        .user-card .card-body {
            padding: 1.5rem;
        }
        
        .btn-group-sm .btn {
            padding: 0.375rem 0.75rem;
            font-size: 0.875rem;
            border-radius: 8px;
            transition: all 0.3s ease;
        }
        
        .btn-group-sm .btn:hover {
            transform: translateY(-2px);
        }
        
        @keyframes fadeInUp {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        
        /* 删除确认模态框样式 */
        #deleteConfirmModal {
            z-index: 9999 !important;
        }
        
        #deleteConfirmModal .modal-backdrop {
            z-index: 9998 !important;
        }
        
        #deleteConfirmModal .modal-dialog {
            margin: 1rem;
            z-index: 10000 !important;
        }
        
        #deleteConfirmModal .modal-content {
            border-radius: 16px;
            overflow: hidden;
            z-index: 10001 !important;
        }
        
        #deleteConfirmModal .modal-header {
            border-radius: 16px 16px 0 0;
        }
        
        /* 移动端优化 */
        @media (max-width: 576px) {
            #deleteConfirmModal .modal-dialog {
                margin: 0.5rem;
                max-width: calc(100% - 1rem);
            }
            
            #deleteConfirmModal .modal-body {
                padding: 1.5rem 1rem;
            }
            
            #deleteConfirmModal .modal-body i.bi-trash3 {
                font-size: 2rem !important;
            }
            
            #deleteConfirmModal .modal-footer {
                flex-direction: column;
                gap: 0.5rem;
            }
            
            #deleteConfirmModal .modal-footer .btn {
                width: 100%;
            }
        }
    </style>
</head>
<body>
    <!-- 导航栏 -->
    <nav class="navbar navbar-expand-lg navbar-dark bg-primary">
        <div class="container-fluid">
            <a class="navbar-brand" href="/dashboard"><i class="bi bi-box-seam"></i> SBoard</a>
            <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbarNav">
                <span class="navbar-toggler-icon"></span>
            </button>
            <div class="collapse navbar-collapse" id="navbarNav">
                <ul class="navbar-nav me-auto">
                    <li class="nav-item"><a class="nav-link" href="/dashboard">仪表板</a></li>
                    <li class="nav-item"><a class="nav-link active" href="/users">用户管理</a></li>
                    <li class="nav-item"><a class="nav-link" href="/nodes">节点管理</a></li>
                    <li class="nav-item"><a class="nav-link" href="/servers">服务器管理</a></li>
                </ul>
                <ul class="navbar-nav">
                    <li class="nav-item dropdown">
                        <a class="nav-link dropdown-toggle" href="#" data-bs-toggle="dropdown">
                            <i class="bi bi-person-circle"></i> <?= htmlspecialchars($username) ?>
                        </a>
                        <ul class="dropdown-menu dropdown-menu-end">
                            <li><a class="dropdown-item" href="#" onclick="showAccountSettingsModal(); return false;">
                                <i class="bi bi-gear"></i> 账号设置
                            </a></li>
                            <li><hr class="dropdown-divider"></li>
                            <li><a class="dropdown-item" href="/logout">
                                <i class="bi bi-box-arrow-right"></i> 退出登录
                            </a></li>
                        </ul>
                    </li>
                </ul>
            </div>
        </div>
    </nav>

    <div class="container-fluid mt-4">
        <div class="d-flex justify-content-between align-items-center mb-4">
            <h2>用户管理</h2>
            <div>
                <button class="btn btn-primary" data-bs-toggle="modal" data-bs-target="#userModal" onclick="resetForm()">
                    <i class="bi bi-plus-lg"></i> 添加用户
                </button>
            </div>
        </div>

        <!-- 用户列表 -->
        <div id="usersList" class="row"></div>
    </div>

    <!-- Toast 容器 -->
    <div class="toast-container position-fixed bottom-0 end-0 p-3">
        <div id="liveToast" class="toast" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="toast-header">
                <strong class="me-auto" id="toastTitle">提示</strong>
                <button type="button" class="btn-close" data-bs-dismiss="toast"></button>
            </div>
            <div class="toast-body" id="toastBody"></div>
        </div>
    </div>

    <!-- 用户模态框 -->
    <div class="modal fade" id="userModal" tabindex="-1">
        <div class="modal-dialog">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title" id="modalTitle">添加用户</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <form id="userForm">
                    <input type="hidden" id="userId" name="id">
                    <div class="modal-body">
                        <div class="mb-3">
                            <label class="form-label">用户名 <span class="text-danger">*</span></label>
                            <input type="text" class="form-control" id="userName" name="name" required>
                        </div>
                        <div class="mb-3">
                            <label class="form-label">UUID <small class="text-muted">(留空自动生成)</small></label>
                            <input type="text" class="form-control" id="userUuid" name="uuid">
                        </div>
                        <div class="mb-3">
                            <label class="form-label">等级</label>
                            <select class="form-select" id="userLevel" name="level">
                                <option value="1">1级</option>
                                <option value="2">2级</option>
                                <option value="3">3级</option>
                            </select>
                        </div>
                        <div class="mb-3">
                            <label class="form-label">到期日期 <span class="text-danger">*</span></label>
                            <input type="date" class="form-control" id="userExpiry" name="expiry_date" required>
                        </div>
                        <div class="mb-3 form-check" id="enabledCheckbox" style="display:none;">
                            <input type="checkbox" class="form-check-input" id="userEnabled" name="enabled" value="1">
                            <label class="form-check-label">启用</label>
                        </div>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                        <button type="submit" class="btn btn-primary">保存</button>
                    </div>
                </form>
            </div>
        </div>
    </div>

    <!-- 账号设置模态框 -->
    <div class="modal fade" id="accountSettingsModal" tabindex="-1">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title"><i class="bi bi-gear"></i> 账号设置</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <!-- 选项卡 -->
                    <ul class="nav nav-tabs mb-3" role="tablist">
                        <li class="nav-item" role="presentation">
                            <button class="nav-link active" data-bs-toggle="tab" data-bs-target="#usernameTab" type="button">
                                <i class="bi bi-person"></i> 修改用户名
                            </button>
                        </li>
                        <li class="nav-item" role="presentation">
                            <button class="nav-link" data-bs-toggle="tab" data-bs-target="#passwordTab" type="button">
                                <i class="bi bi-key"></i> 修改密码
                            </button>
                        </li>
                    </ul>

                    <div class="tab-content">
                        <!-- 修改用户名标签页 -->
                        <div class="tab-pane fade show active" id="usernameTab">
                            <form id="changeUsernameForm">
                                <div class="mb-3">
                                    <label class="form-label">当前用户名</label>
                                    <input type="text" class="form-control" value="<?= htmlspecialchars($username) ?>" disabled>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">新用户名 <span class="text-danger">*</span></label>
                                    <input type="text" class="form-control" id="newUsername" required minlength="3" autocomplete="username">
                                    <small class="text-muted">至少3个字符</small>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">确认密码 <span class="text-danger">*</span></label>
                                    <input type="password" class="form-control" id="usernamePassword" required autocomplete="current-password">
                                    <small class="text-muted">输入当前密码以确认修改</small>
                                </div>
                                <button type="submit" class="btn btn-primary w-100">确认修改用户名</button>
                            </form>
                        </div>

                        <!-- 修改密码标签页 -->
                        <div class="tab-pane fade" id="passwordTab">
                            <form id="changePasswordForm">
                                <div class="mb-3">
                                    <label class="form-label">当前密码 <span class="text-danger">*</span></label>
                                    <input type="password" class="form-control" id="oldPassword" required autocomplete="current-password">
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">新密码 <span class="text-danger">*</span></label>
                                    <input type="password" class="form-control" id="newPassword" required minlength="6" autocomplete="new-password">
                                    <small class="text-muted">至少6个字符</small>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">确认新密码 <span class="text-danger">*</span></label>
                                    <input type="password" class="form-control" id="confirmPassword" required minlength="6" autocomplete="new-password">
                                </div>
                                <button type="submit" class="btn btn-primary w-100">确认修改密码</button>
                            </form>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <!-- 删除确认模态框 -->
    <div class="modal fade" id="deleteConfirmModal" tabindex="-1" aria-labelledby="deleteModalTitle" aria-hidden="true">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content border-0 shadow-lg">
                <div class="modal-header bg-danger text-white border-0">
                    <h5 class="modal-title" id="deleteModalTitleElement">
                        <i class="bi bi-exclamation-triangle-fill me-2"></i>
                        <span id="deleteModalTitle">确认删除</span>
                    </h5>
                    <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal" aria-label="Close"></button>
                </div>
                <div class="modal-body py-4">
                    <div class="text-center mb-3">
                        <i class="bi bi-trash3 text-danger" style="font-size: 3rem;"></i>
                    </div>
                    <p class="text-center mb-2 fs-5" id="deleteModalMessage">确定要删除吗？</p>
                    <p class="text-center text-muted small mb-0">
                        <i class="bi bi-info-circle me-1"></i>此操作不可撤销！
                    </p>
                </div>
                <div class="modal-footer border-0 justify-content-center pb-4">
                    <button type="button" class="btn btn-secondary px-4" data-bs-dismiss="modal">
                        <i class="bi bi-x-lg me-1"></i>取消
                    </button>
                    <button type="button" class="btn btn-danger px-4" id="confirmDeleteBtn">
                        <i class="bi bi-trash3 me-1"></i>确认删除
                    </button>
                </div>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        let users = [];
        let nodes = [];
        let editingId = null;

        // Toast 提示函数
        function showToast(message, type = 'info') {
            const toastEl = document.getElementById('liveToast');
            const toastBody = document.getElementById('toastBody');
            const toastTitle = document.getElementById('toastTitle');
            const toast = new bootstrap.Toast(toastEl, { delay: 3000 });
            
            // 设置图标和标题
            const icons = {
                success: '✅',
                danger: '❌',
                warning: '⚠️',
                info: 'ℹ️'
            };
            
            toastTitle.textContent = icons[type] || 'ℹ️';
            toastBody.textContent = message;
            
            // 设置背景色
            toastEl.className = 'toast';
            toastEl.classList.add(`bg-${type}`, 'text-white');
            
            toast.show();
        }

        // 加载用户列表
        async function loadUsers() {
            try {
                const response = await fetch('/api/users');
                const data = await response.json();
                users = data.data || [];
            } catch (error) {
                console.error('加载用户列表失败:', error);
                showToast('加载用户列表失败，请重试', 'danger');
            }
        }

        // 加载节点列表
        async function loadNodes() {
            try {
                const response = await fetch('/api/nodes');
                const data = await response.json();
                nodes = data.data || [];
            } catch (error) {
                console.error('加载节点列表失败:', error);
            }
        }

        // 初始化加载所有数据
        async function initializeData() {
            await Promise.all([loadUsers(), loadNodes()]);
            renderUsers();
        }

        // 渲染用户列表
        function renderUsers() {
            const container = document.getElementById('usersList');
            
            if (users.length === 0) {
                container.innerHTML = '<div class="col-12 text-center py-5"><p class="text-muted">暂无用户</p></div>';
                return;
            }

            // 按等级分组
            const byLevel = { 1: [], 2: [], 3: [] };
            users.forEach(u => byLevel[u.level].push(u));

            let html = '';
            [1, 2, 3].forEach(level => {
                if (byLevel[level].length > 0) {
                    html += `<div class="col-12 mb-3"><h5>等级 ${level}</h5></div>`;
                    byLevel[level].forEach(user => {
                        const expired = new Date(user.expiry_date) < new Date();
                        const statusClass = expired ? 'danger' : 'success';
                        const statusText = expired ? '已过期' : '有效';
                        const statusBadge = expired ? 'bg-danger' : 'bg-success';
                        
                        html += `
                            <div class="col-xl-3 col-lg-4 col-md-6">
                                <div class="card user-card">
                                    <div class="card-body">
                                        <div class="d-flex justify-content-between align-items-start mb-2">
                                            <h6 class="card-title mb-0">${escapeHtml(user.name)}</h6>
                                            <span class="badge ${statusBadge}">${statusText}</span>
                                        </div>
                                        <p class="small mb-2">
                                            <div class="d-flex justify-content-between">
                                                <span class="text-muted">到期时间:</span>
                                                <span class="text-${statusClass}">${user.expiry_date}</span>
                                            </div>
                                        </p>
                                        <div class="btn-group btn-group-sm w-100 mb-2">
                                            <button class="btn btn-outline-primary" onclick="editUser(${user.id})">
                                                <i class="bi bi-pencil"></i> 编辑
                                            </button>
                                            <button class="btn btn-outline-danger" onclick="deleteUser(${user.id})">
                                                <i class="bi bi-trash"></i> 删除
                                            </button>
                                        </div>
                                        ${generateSubscriptionButtons(user)}
                                    </div>
                                </div>
                            </div>
                        `;
                    });
                }
            });

            container.innerHTML = html;
        }

        // 重置表单
        function resetForm() {
            editingId = null;
            document.getElementById('modalTitle').textContent = '添加用户';
            document.getElementById('userForm').reset();
            document.getElementById('userId').value = '';
            document.getElementById('enabledCheckbox').style.display = 'none';
            document.getElementById('userExpiry').min = new Date().toISOString().split('T')[0];
        }

        // 生成订阅按钮
        function generateSubscriptionButtons(user) {
            if (nodes.length === 0) {
                return '';
            }
            
            let html = '<div class="dropdown w-100">';
            html += '<button class="btn btn-sm btn-success dropdown-toggle w-100" type="button" data-bs-toggle="dropdown" data-bs-auto-close="true">';
            html += '<i class="bi bi-link-45deg"></i> 订阅链接';
            html += '</button>';
            html += '<ul class="dropdown-menu w-100 shadow">';
            
            // 为每个节点生成订阅选项
            nodes.forEach(node => {
                // 普通订阅(适用于所有用户)
                const normalUrl = window.location.origin + `/sublink.php?user=${user.uuid}&type=${node.tag}`;
                html += `<li><a class="dropdown-item small copy-sub-link" href="#" data-url="${normalUrl}" data-name="${escapeHtml(node.tag)}">`;
                html += `<i class="bi bi-clipboard"></i> ${escapeHtml(node.tag)}`;
                html += `</a></li>`;
                
                // 3级用户额外的选项
                if (user.level == 3) {
                    const lv3Url = window.location.origin + `/sublink.php?user=${user.uuid}&type=${node.tag}&lv=3`;
                    html += `<li><a class="dropdown-item small text-primary copy-sub-link" href="#" data-url="${lv3Url}" data-name="${escapeHtml(node.tag)} (仅家宽)">`;
                    html += `<i class="bi bi-house"></i> ${escapeHtml(node.tag)} (仅家宽)`;
                    html += `</a></li>`;
                }
            });
            
            html += '</ul></div>';
            return html;
        }

        // 复制订阅链接 - 使用事件委托
        document.addEventListener('click', async function(e) {
            if (e.target.closest('.copy-sub-link')) {
                e.preventDefault();
                const link = e.target.closest('.copy-sub-link');
                const url = link.dataset.url;
                const name = link.dataset.name;
                
                try {
                    await navigator.clipboard.writeText(url);
                    showToast(`已复制订阅链接: ${name}`, 'success');
                } catch (err) {
                    // 降级方案
                    const textarea = document.createElement('textarea');
                    textarea.value = url;
                    textarea.style.position = 'fixed';
                    textarea.style.opacity = '0';
                    document.body.appendChild(textarea);
                    textarea.select();
                    try {
                        document.execCommand('copy');
                        showToast(`已复制订阅链接: ${name}`, 'success');
                    } catch (e) {
                        showToast('复制失败: ' + url, 'danger');
                    }
                    document.body.removeChild(textarea);
                }
            }
        });

        // 编辑用户
        function editUser(id) {
            const user = users.find(u => u.id === id);
            if (!user) return;

            editingId = id;
            document.getElementById('modalTitle').textContent = '编辑用户';
            document.getElementById('userId').value = user.id;
            document.getElementById('userName').value = user.name;
            document.getElementById('userUuid').value = user.uuid;
            document.getElementById('userLevel').value = user.level;
            document.getElementById('userExpiry').value = user.expiry_date;
            document.getElementById('userEnabled').checked = user.enabled == 1;
            document.getElementById('enabledCheckbox').style.display = 'block';

            new bootstrap.Modal(document.getElementById('userModal')).show();
        }

        // 全局变量存储待删除的ID
        let pendingDeleteId = null;
        let pendingDeleteType = null;
        
        // 删除用户
        function deleteUser(id) {
            const user = users.find(u => u.id === id);
            const userName = user ? user.name : '该用户';
            
            // 设置模态框内容
            document.getElementById('deleteModalTitle').textContent = '确认删除用户';
            document.getElementById('deleteModalMessage').innerHTML = `确定要删除用户 <strong class="text-danger">"${userName}"</strong> 吗？`;
            
            // 存储待删除的ID
            pendingDeleteId = id;
            pendingDeleteType = 'user';
            
            // 显示模态框
            const modalEl = document.getElementById('deleteConfirmModal');
            const modal = bootstrap.Modal.getInstance(modalEl) || new bootstrap.Modal(modalEl);
            modal.show();
        }
        
        // 确认删除按钮事件
        document.addEventListener('DOMContentLoaded', function() {
            const confirmBtn = document.getElementById('confirmDeleteBtn');
            if (confirmBtn) {
                confirmBtn.addEventListener('click', async function() {
                    if (!pendingDeleteId || !pendingDeleteType) return;
                    
                    // 关闭模态框
                    const modalEl = document.getElementById('deleteConfirmModal');
                    const modal = bootstrap.Modal.getInstance(modalEl);
                    if (modal) modal.hide();
                    
                    // 执行删除
                    if (pendingDeleteType === 'user') {
                        try {
                            const response = await fetch(`/api/users/${pendingDeleteId}`, { method: 'DELETE' });
                            const data = await response.json();
                            
                            showToast(data.message, data.success ? 'success' : 'danger');
                            if (data.success) {
                                await loadUsers();
                                renderUsers();
                            }
                        } catch (error) {
                            console.error('删除用户失败:', error);
                            showToast('删除用户失败，请重试', 'danger');
                        }
                    }
                    
                    // 清空pending状态
                    pendingDeleteId = null;
                    pendingDeleteType = null;
                });
            }
        });

        // 提交表单
        document.getElementById('userForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const formData = new FormData(e.target);
            const data = Object.fromEntries(formData.entries());
            
            try {
                let response;
                if (editingId) {
                    response = await fetch(`/api/users/${editingId}`, {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                } else {
                    response = await fetch('/api/users', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(data)
                    });
                }

                const result = await response.json();
                
                showToast(result.message, result.success ? 'success' : 'danger');
                
                if (result.success) {
                    bootstrap.Modal.getInstance(document.getElementById('userModal')).hide();
                    await loadUsers();
                    renderUsers();
                }
            } catch (error) {
                console.error('操作失败:', error);
                showToast('操作失败，请重试', 'danger');
            }
        });

        function escapeHtml(text) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
            return text.replace(/[&<>"']/g, m => map[m]);
        }

        // 显示账号设置模态框
        function showAccountSettingsModal() {
            document.getElementById('changeUsernameForm').reset();
            document.getElementById('changePasswordForm').reset();
            new bootstrap.Modal(document.getElementById('accountSettingsModal')).show();
        }

        // 修改用户名表单提交
        document.getElementById('changeUsernameForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const newUsername = document.getElementById('newUsername').value;
            const password = document.getElementById('usernamePassword').value;

            if (newUsername.length < 3) {
                showToast('新用户名至少需要3个字符', 'danger');
                return;
            }

            try {
                const response = await fetch('/api/auth/change-username', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        password: password,
                        new_username: newUsername
                    })
                });

                const data = await response.json();
                
                showToast(data.message, data.success ? 'success' : 'danger');
                
                if (data.success) {
                    bootstrap.Modal.getInstance(document.getElementById('accountSettingsModal')).hide();
                    document.getElementById('changeUsernameForm').reset();
                    // 更新导航栏显示的用户名
                    setTimeout(() => location.reload(), 1000);
                }
            } catch (error) {
                console.error('修改用户名失败:', error);
                showToast('修改用户名失败，请重试', 'danger');
            }
        });

        // 修改密码表单提交
        document.getElementById('changePasswordForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const oldPassword = document.getElementById('oldPassword').value;
            const newPassword = document.getElementById('newPassword').value;
            const confirmPassword = document.getElementById('confirmPassword').value;

            if (newPassword !== confirmPassword) {
                showToast('两次输入的新密码不一致', 'danger');
                return;
            }

            if (newPassword.length < 6) {
                showToast('新密码至少需要6个字符', 'danger');
                return;
            }

            try {
                const response = await fetch('/api/auth/change-password', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        old_password: oldPassword,
                        new_password: newPassword,
                        confirm_password: confirmPassword
                    })
                });

                const data = await response.json();
                
                showToast(data.message, data.success ? 'success' : 'danger');
                
                if (data.success) {
                    bootstrap.Modal.getInstance(document.getElementById('accountSettingsModal')).hide();
                    document.getElementById('changePasswordForm').reset();
                }
            } catch (error) {
                console.error('修改密码失败:', error);
                showToast('修改密码失败，请重试', 'danger');
            }
        });

        // 页面加载时初始化数据
        initializeData();
    </script>
</body>
</html>
