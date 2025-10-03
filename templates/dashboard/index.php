<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>仪表板 - SBoard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <link href="/assets/css/style.css" rel="stylesheet">
    <style>
        body {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            min-height: 100vh;
        }
        
        .stat-card {
            background: white;
            border-radius: 16px;
            padding: 1.75rem;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
            transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
            position: relative;
            overflow: hidden;
            animation: fadeInUp 0.6s ease-out;
        }
        
        .stat-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 4px;
            transition: all 0.3s ease;
            transform: scaleX(0);
        }
        
        .stat-card:hover::before {
            transform: scaleX(1);
        }
        
        .stat-card:hover {
            transform: translateY(-8px);
            box-shadow: 0 12px 30px rgba(0, 0, 0, 0.15);
        }
        
        .stat-card-primary::before { background: linear-gradient(90deg, #667eea, #764ba2); }
        .stat-card-success::before { background: linear-gradient(90deg, #10b981, #059669); }
        .stat-card-warning::before { background: linear-gradient(90deg, #f59e0b, #d97706); }
        .stat-card-danger::before { background: linear-gradient(90deg, #ef4444, #dc2626); }
        
        .stat-icon {
            width: 64px;
            height: 64px;
            border-radius: 16px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 2rem;
            transition: all 0.3s ease;
        }
        
        .stat-card:hover .stat-icon {
            transform: scale(1.1) rotate(-5deg);
        }
        
        .stat-icon-primary {
            background: linear-gradient(135deg, rgba(102, 126, 234, 0.1), rgba(118, 75, 162, 0.1));
            color: #667eea;
        }
        
        .stat-icon-success {
            background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(5, 150, 105, 0.1));
            color: #10b981;
        }
        
        .stat-icon-warning {
            background: linear-gradient(135deg, rgba(245, 158, 11, 0.1), rgba(217, 119, 6, 0.1));
            color: #f59e0b;
        }
        
        .stat-icon-danger {
            background: linear-gradient(135deg, rgba(239, 68, 68, 0.1), rgba(220, 38, 38, 0.1));
            color: #ef4444;
        }
        
        .stat-value {
            font-size: 2.25rem;
            font-weight: 700;
            margin: 0.75rem 0 0.25rem;
        }
        
        .stat-value-primary {
            background: linear-gradient(135deg, #667eea, #764ba2);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        
        .stat-value-success {
            background: linear-gradient(135deg, #10b981, #059669);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        
        .stat-value-warning {
            background: linear-gradient(135deg, #f59e0b, #d97706);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        
        .stat-value-danger {
            background: linear-gradient(135deg, #ef4444, #dc2626);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        
        .stat-label {
            color: #6b7280;
            font-size: 0.875rem;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        @keyframes fadeInUp {
            from {
                opacity: 0;
                transform: translateY(30px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        
        .stat-card:nth-child(1) { animation-delay: 0.1s; }
        .stat-card:nth-child(2) { animation-delay: 0.2s; }
        .stat-card:nth-child(3) { animation-delay: 0.3s; }
        .stat-card:nth-child(4) { animation-delay: 0.4s; }
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
                    <li class="nav-item"><a class="nav-link active" href="/dashboard">仪表板</a></li>
                    <li class="nav-item"><a class="nav-link" href="/users">用户管理</a></li>
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
        <h2 class="mb-4">仪表板</h2>

        <!-- 统计卡片 -->
        <div class="row mb-4">
            <div class="col-md-3 mb-3">
                <div class="stat-card stat-card-primary">
                    <div class="d-flex justify-content-between align-items-start">
                        <div class="flex-grow-1">
                            <div class="stat-label">总用户数</div>
                            <div class="stat-value stat-value-primary"><?= $stats['total_users'] ?></div>
                            <small class="text-muted">系统注册用户</small>
                        </div>
                        <div class="stat-icon stat-icon-primary">
                            <i class="bi bi-people-fill"></i>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 mb-3">
                <div class="stat-card stat-card-success">
                    <div class="d-flex justify-content-between align-items-start">
                        <div class="flex-grow-1">
                            <div class="stat-label">活跃用户</div>
                            <div class="stat-value stat-value-success"><?= $stats['active_users'] ?></div>
                            <small class="text-muted">在线活跃用户</small>
                        </div>
                        <div class="stat-icon stat-icon-success">
                            <i class="bi bi-check-circle-fill"></i>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 mb-3">
                <div class="stat-card stat-card-warning">
                    <div class="d-flex justify-content-between align-items-start">
                        <div class="flex-grow-1">
                            <div class="stat-label">总节点数</div>
                            <div class="stat-value stat-value-warning"><?= $stats['total_nodes'] ?></div>
                            <small class="text-muted">可用代理节点</small>
                        </div>
                        <div class="stat-icon stat-icon-warning">
                            <i class="bi bi-diagram-3-fill"></i>
                        </div>
                    </div>
                </div>
            </div>
            <div class="col-md-3 mb-3">
                <div class="stat-card stat-card-danger">
                    <div class="d-flex justify-content-between align-items-start">
                        <div class="flex-grow-1">
                            <div class="stat-label">总服务器</div>
                            <div class="stat-value stat-value-danger"><?= $stats['total_servers'] ?></div>
                            <small class="text-muted">部署服务器数</small>
                        </div>
                        <div class="stat-icon stat-icon-danger">
                            <i class="bi bi-server"></i>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <div class="row">
            <!-- 即将过期用户 -->
            <div class="col-md-6 mb-4">
                <div class="card">
                    <div class="card-header bg-white">
                        <h5 class="mb-0"><i class="bi bi-clock-history"></i> 即将过期用户</h5>
                    </div>
                    <div class="card-body p-0">
                        <?php if (empty($expiring_users)): ?>
                            <p class="text-muted text-center py-4">暂无即将过期的用户</p>
                        <?php else: ?>
                            <table class="table table-sm table-hover mb-0">
                                <thead class="table-light">
                                    <tr>
                                        <th>用户名</th>
                                        <th>等级</th>
                                        <th>到期日期</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <?php foreach ($expiring_users as $user): ?>
                                        <tr>
                                            <td><?= htmlspecialchars($user['name']) ?></td>
                                            <td><span class="badge bg-secondary">Lv<?= $user['level'] ?></span></td>
                                            <td><?= htmlspecialchars($user['expiry_date']) ?></td>
                                        </tr>
                                    <?php endforeach; ?>
                                </tbody>
                            </table>
                        <?php endif; ?>
                    </div>
                </div>
            </div>

            <!-- 最近部署日志 -->
            <div class="col-md-6 mb-4">
                <div class="card">
                    <div class="card-header bg-white">
                        <h5 class="mb-0"><i class="bi bi-journal-text"></i> 最近部署日志</h5>
                    </div>
                    <div class="card-body p-0">
                        <?php if (empty($recent_deploys)): ?>
                            <p class="text-muted text-center py-4">暂无部署记录</p>
                        <?php else: ?>
                            <table class="table table-sm table-hover mb-0">
                                <thead class="table-light">
                                    <tr>
                                        <th>服务器</th>
                                        <th>状态</th>
                                        <th>时间</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <?php foreach ($recent_deploys as $log): ?>
                                        <tr>
                                            <td><?= htmlspecialchars($log['server_name'] ?? 'Unknown') ?></td>
                                            <td>
                                                <?php if ($log['status'] === 'success'): ?>
                                                    <span class="badge bg-success">成功</span>
                                                <?php elseif ($log['status'] === 'failed'): ?>
                                                    <span class="badge bg-danger">失败</span>
                                                <?php else: ?>
                                                    <span class="badge bg-secondary">待处理</span>
                                                <?php endif; ?>
                                            </td>
                                            <td><?= htmlspecialchars(substr($log['created_at'], 0, 16)) ?></td>
                                        </tr>
                                    <?php endforeach; ?>
                                </tbody>
                            </table>
                        <?php endif; ?>
                    </div>
                </div>
            </div>
        </div>
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

    <!-- 账号设置模态框 -->
    <div class="modal fade" id="accountSettingsModal" tabindex="-1">
        <div class="modal-dialog">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">账号设置</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <ul class="nav nav-tabs mb-3" id="accountSettingsTabs" role="tablist">
                        <li class="nav-item" role="presentation">
                            <button class="nav-link active" id="username-tab" data-bs-toggle="tab" data-bs-target="#usernameTab" type="button">修改用户名</button>
                        </li>
                        <li class="nav-item" role="presentation">
                            <button class="nav-link" id="password-tab" data-bs-toggle="tab" data-bs-target="#passwordTab" type="button">修改密码</button>
                        </li>
                    </ul>
                    <div class="tab-content" id="accountSettingsTabContent">
                        <!-- 修改用户名标签页 -->
                        <div class="tab-pane fade show active" id="usernameTab" role="tabpanel">
                            <form id="usernameForm">
                                <div class="mb-3">
                                    <label class="form-label">当前密码</label>
                                    <input type="password" class="form-control" name="password" required>
                                    <div class="form-text">为了安全，修改用户名需要验证密码</div>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">新用户名</label>
                                    <input type="text" class="form-control" name="new_username" required minlength="3" maxlength="20" pattern="[a-zA-Z0-9_]+">
                                    <div class="form-text">3-20个字符，只能包含字母、数字和下划线</div>
                                </div>
                            </form>
                        </div>
                        <!-- 修改密码标签页 -->
                        <div class="tab-pane fade" id="passwordTab" role="tabpanel">
                            <form id="passwordForm">
                                <div class="mb-3">
                                    <label class="form-label">旧密码</label>
                                    <input type="password" class="form-control" name="old_password" required>
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">新密码</label>
                                    <input type="password" class="form-control" name="new_password" required minlength="6">
                                </div>
                                <div class="mb-3">
                                    <label class="form-label">确认新密码</label>
                                    <input type="password" class="form-control" name="confirm_password" required minlength="6">
                                </div>
                            </form>
                        </div>
                    </div>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                    <button type="button" class="btn btn-primary" id="submitAccountSettings">确认修改</button>
                </div>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        // Toast 提示函数
        function showToast(message, type = 'info') {
            const toastEl = document.getElementById('liveToast');
            const toastBody = document.getElementById('toastBody');
            const toastTitle = document.getElementById('toastTitle');
            const toast = new bootstrap.Toast(toastEl, { delay: 3000 });
            
            const icons = {
                success: '✅',
                danger: '❌',
                warning: '⚠️',
                info: 'ℹ️'
            };
            
            toastTitle.textContent = icons[type] || 'ℹ️';
            toastBody.textContent = message;
            toastEl.className = 'toast';
            toastEl.classList.add(`bg-${type}`, 'text-white');
            toast.show();
        }

        // 修改账号设置 - 提交按钮事件
        document.getElementById('submitAccountSettings').addEventListener('click', async () => {
            // 检查当前激活的标签页
            const activeTab = document.querySelector('#accountSettingsTabs .nav-link.active').id;
            
            if (activeTab === 'username-tab') {
                // 修改用户名
                const form = document.getElementById('usernameForm');
                if (!form.checkValidity()) {
                    form.reportValidity();
                    return;
                }
                
                const formData = new FormData(form);
                const password = formData.get('password');
                const newUsername = formData.get('new_username');
                
                try {
                    const response = await fetch('/api/auth/change-username', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ password, new_username: newUsername })
                    });

                    const data = await response.json();
                    showToast(data.message, data.success ? 'success' : 'danger');
                    
                    if (data.success) {
                        bootstrap.Modal.getInstance(document.getElementById('accountSettingsModal')).hide();
                        form.reset();
                        // 更新显示的用户名
                        document.querySelector('.nav-link.dropdown-toggle').innerHTML = 
                            `<i class="bi bi-person-circle"></i> ${newUsername}`;
                    }
                } catch (error) {
                    console.error('修改用户名失败:', error);
                    showToast('修改用户名失败，请重试', 'danger');
                }
            } else {
                // 修改密码
                const form = document.getElementById('passwordForm');
                if (!form.checkValidity()) {
                    form.reportValidity();
                    return;
                }
                
                const formData = new FormData(form);
                if (formData.get('new_password') !== formData.get('confirm_password')) {
                    showToast('两次输入的密码不一致', 'warning');
                    return;
                }

                try {
                    const response = await fetch('/api/auth/change-password', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            old_password: formData.get('old_password'),
                            new_password: formData.get('new_password'),
                            confirm_password: formData.get('confirm_password')
                        })
                    });

                    const data = await response.json();
                    showToast(data.message, data.success ? 'success' : 'danger');
                    
                    if (data.success) {
                        bootstrap.Modal.getInstance(document.getElementById('accountSettingsModal')).hide();
                        form.reset();
                    }
                } catch (error) {
                    console.error('修改密码失败:', error);
                    showToast('修改密码失败，请重试', 'danger');
                }
            }
        });

        // 显示账号设置模态框
        function showAccountSettingsModal() {
            document.getElementById('usernameForm').reset();
            document.getElementById('passwordForm').reset();
            new bootstrap.Modal(document.getElementById('accountSettingsModal')).show();
        }
    </script>
</body>
</html>
