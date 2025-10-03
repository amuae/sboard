<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>服务器管理 - SBoard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.11.0/font/bootstrap-icons.css" rel="stylesheet">
    <link href="/assets/css/style.css" rel="stylesheet">
    <style>
        body {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            min-height: 100vh;
        }
        
        .server-card {
            background: white;
            border: none;
            border-radius: 16px;
            padding: 1.5rem;
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.08);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            position: relative;
            overflow: hidden;
            animation: fadeInUp 0.5s ease-out;
        }
        
        .server-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 4px;
            background: linear-gradient(90deg, #667eea, #764ba2);
            transform: scaleX(0);
            transition: transform 0.3s ease;
        }
        
        .server-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 12px 30px rgba(0, 0, 0, 0.12);
        }
        
        .server-card:hover::before {
            transform: scaleX(1);
        }
        
        .server-header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 1rem;
        }
        
        .server-title {
            font-size: 1.25rem;
            font-weight: 600;
            color: #1f2937;
            margin-bottom: 0.5rem;
        }
        
        .server-host {
            color: #6b7280;
            font-size: 0.875rem;
            display: flex;
            align-items: center;
            gap: 0.25rem;
        }
        
        .card-actions {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 0.5rem;
            margin-top: 1rem;
        }
        
        .card-actions .btn {
            font-size: 0.875rem;
            padding: 0.5rem 0.75rem;
            border-radius: 8px;
            transition: all 0.3s ease;
        }
        
        .card-actions .btn:hover {
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
                    <li class="nav-item"><a class="nav-link" href="/users">用户管理</a></li>
                    <li class="nav-item"><a class="nav-link" href="/nodes">节点管理</a></li>
                    <li class="nav-item"><a class="nav-link active" href="/servers">服务器管理</a></li>
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

    </nav>

    <!-- Toast 容器 -->
    <div class="toast-container position-fixed top-0 end-0 p-3" style="z-index: 9999">
        <div id="liveToast" class="toast align-items-center border-0" role="alert">
            <div class="d-flex">
                <div class="toast-body d-flex align-items-center">
                    <i id="toastIcon" class="me-2"></i>
                    <span id="toastMessage"></span>
                </div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
            </div>
        </div>
    </div>

    <div class="container-fluid mt-4">
        <div class="d-flex justify-content-between align-items-center mb-4">
            <h2><i class="bi bi-hdd-network"></i> 服务器管理</h2>
            <div>
                <button id="deployAllBtn" class="btn btn-warning me-2">
                    <i class="bi bi-arrow-clockwise"></i> 全部部署
                </button>
                <button id="deployFolderAllBtn" class="btn btn-success me-2">
                    <i class="bi bi-lightning"></i> 目录全部
                </button>
                <button class="btn btn-primary" onclick="showAddModal()">
                    <i class="bi bi-plus-lg"></i> 添加服务器
                </button>
            </div>
        </div>

        <div id="serversList" class="row g-3"></div>
    </div>

    <!-- 添加/编辑服务器模态框 -->
    <div class="modal fade" id="serverModal" tabindex="-1">
        <div class="modal-dialog modal-dialog-centered">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">服务器信息</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <form id="serverForm">
                        <div class="mb-3">
                            <label class="form-label">备注名称 <span class="text-danger">*</span></label>
                            <input type="text" class="form-control" id="name" required>
                        </div>
                        <div class="mb-3">
                            <label class="form-label">主机/IP/域名 <span class="text-danger">*</span></label>
                            <input type="text" class="form-control" id="host" required oninput="checkHostType()">
                        </div>
                        <div class="mb-3" id="dnsResolveGroup" style="display: none;">
                            <label class="form-label">DNS 解析</label>
                            <select class="form-select" id="dns_resolve">
                                <option value="none">不解析</option>
                                <option value="ipv4">IPv4</option>
                                <option value="ipv6">IPv6</option>
                            </select>
                        </div>
                        <div class="mb-3">
                            <label class="form-label">SSH 端口</label>
                            <input type="number" class="form-control" id="port" value="22" min="1" max="65535">
                        </div>
                        <div class="mb-3">
                            <label class="form-label">分类 <span class="text-danger">*</span></label>
                            <select class="form-select" id="category" onchange="toggleNodeFields()">
                                <option value="direct">直连</option>
                                <option value="relay">中转</option>
                                <option value="home">家宽</option>
                            </select>
                        </div>
                        <div class="mb-3">
                            <label class="form-label">核心类型 <span class="text-danger">*</span></label>
                            <select class="form-select" id="core_type">
                                <option value="mihomo" selected>mihomo</option>
                                <option value="sing-box">sing-box</option>
                            </select>
                        </div>
                        <div class="mb-3 node-direct-relay">
                            <label class="form-label">节点名一</label>
                            <input type="text" class="form-control" id="node_1">
                        </div>
                        <div class="mb-3 node-direct-relay">
                            <label class="form-label">节点名二</label>
                            <input type="text" class="form-control" id="node_2">
                        </div>
                        <div class="mb-3 node-home" style="display: none;">
                            <label class="form-label">节点名三</label>
                            <input type="text" class="form-control" id="node_3">
                        </div>
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="enabled" checked>
                            <label class="form-check-label" for="enabled">启用服务器</label>
                        </div>
                    </form>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
                    <button type="button" class="btn btn-primary" onclick="saveServer()">保存</button>
                </div>
            </div>
        </div>
    </div>

    <!-- 部署结果模态框 -->
    <div class="modal fade" id="deployModal" tabindex="-1">
        <div class="modal-dialog modal-dialog-scrollable modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title">部署日志</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <div class="modal-body">
                    <pre id="deployOutput" style="white-space: pre-wrap; word-break: break-word; background: #f8f9fa; padding: 1rem; border-radius: 5px; max-height: 500px; overflow-y: auto;"></pre>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                </div>
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

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script src="/assets/js/confirm-modal.js"></script>
    <script>
        let servers = [];
        let editingId = null;

        // Toast 通知函数
        function showToast(message, type = 'info') {
            const toastEl = document.getElementById('liveToast');
            const toastBody = document.getElementById('toastMessage');
            const toastIcon = document.getElementById('toastIcon');
            
            const icons = {
                success: 'bi-check-circle-fill text-success',
                danger: 'bi-x-circle-fill text-danger',
                warning: 'bi-exclamation-triangle-fill text-warning',
                info: 'bi-info-circle-fill text-info'
            };
            
            const colors = {
                success: 'text-bg-success',
                danger: 'text-bg-danger',
                warning: 'text-bg-warning',
                info: 'text-bg-info'
            };
            
            toastIcon.className = `bi ${icons[type] || icons.info} me-2`;
            toastBody.textContent = message;
            toastEl.className = `toast align-items-center ${colors[type] || colors.info} border-0`;
            
            const toast = new bootstrap.Toast(toastEl);
            toast.show();
        }

        // 切换节点字段显示/隐藏
        function toggleNodeFields() {
            const category = document.getElementById('category').value;
            const directRelayFields = document.querySelectorAll('.node-direct-relay');
            const homeFields = document.querySelectorAll('.node-home');
            
            if (category === 'home') {
                directRelayFields.forEach(el => el.style.display = 'none');
                homeFields.forEach(el => el.style.display = 'block');
            } else {
                directRelayFields.forEach(el => el.style.display = 'block');
                homeFields.forEach(el => el.style.display = 'none');
            }
        }

        // 检查主机类型（IP 或域名）
        function checkHostType() {
            const host = document.getElementById('host').value.trim();
            const dnsGroup = document.getElementById('dnsResolveGroup');
            
            // 判断是否为 IP 地址（IPv4 或 IPv6）
            const ipv4Regex = /^(\d{1,3}\.){3}\d{1,3}$/;
            const ipv6Regex = /^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$/;
            
            if (ipv4Regex.test(host) || ipv6Regex.test(host)) {
                // 是 IP 地址，隐藏 DNS 解析选项
                dnsGroup.style.display = 'none';
                document.getElementById('dns_resolve').value = 'none';
            } else if (host.length > 0) {
                // 是域名，显示 DNS 解析选项
                dnsGroup.style.display = 'block';
            } else {
                // 空值，隐藏
                dnsGroup.style.display = 'none';
            }
        }

        async function loadServers() {
            try {
                const res = await fetch('/api/servers');
                const data = await res.json();
                servers = data.data || [];
                renderServers();
            } catch (error) {
                console.error('加载服务器列表失败:', error);
                showToast('加载服务器列表失败，请重试', 'danger');
            }
        }

        function renderServers() {
            const container = document.getElementById('serversList');
            if (servers.length === 0) {
                container.innerHTML = `
                    <div class="col-12 text-center py-5">
                        <i class="bi bi-hdd-network" style="font-size: 4rem; color: #ccc;"></i>
                        <h4 class="text-muted mt-3">暂无服务器</h4>
                        <p class="text-muted">点击"添加服务器"开始管理</p>
                    </div>
                `;
                return;
            }

            let html = '';
            servers.forEach(server => {
                const canDeploy = server.category !== 'home';
                html += `
                    <div class="col-xl-3 col-lg-4 col-md-6">
                        <div class="server-card">
                            <div class="d-flex justify-content-between align-items-center mb-2">
                                <span class="fw-bold">${escapeHtml(server.name)}</span>
                                <div>
                                    <span class="badge ${server.enabled ? 'bg-success' : 'bg-secondary'}">${server.enabled ? '启用' : '停用'}</span>
                                </div>
                            </div>
                            <p class="small mb-2">
                                <div class="d-flex justify-content-between mb-1">
                                    <span class="text-muted">主机:</span>
                                    <span title="${escapeHtml(server.host)}">${escapeHtml(maskHost(server.host))}</span>
                                </div>
                                <div class="d-flex justify-content-between mb-1">
                                    <span class="text-muted">端口:</span>
                                    <span title="${server.port}">${maskPort(server.port)}</span>
                                </div>
                                <div class="d-flex justify-content-between mb-1">
                                    <span class="text-muted">分类:</span>
                                    <span class="badge bg-info">${getCategoryName(server.category)}</span>
                                </div>
                                <div class="d-flex justify-content-between mb-1">
                                    <span class="text-muted">核心:</span>
                                    <span class="badge bg-primary">${server.core_type}</span>
                                </div>
                                ${server.node_2 ? `<div class="d-flex justify-content-between mb-1"><span class="text-muted">节点:</span><span>${escapeHtml(server.node_2)}</span></div>` : ''}
                            </p>
                            <div class="card-actions">
                                ${canDeploy ? `
                                    <button class="btn btn-outline-primary btn-sm" onclick="deploySingle(${server.id})" title="单文件部署">
                                        <i class="bi bi-cloud-upload"></i> 部署
                                    </button>
                                    <button class="btn btn-outline-success btn-sm" onclick="deployFolder(${server.id})" title="目录部署">
                                        <i class="bi bi-folder-symlink"></i> 目录
                                    </button>
                                ` : '<div></div><div></div>'}
                                <button class="btn btn-outline-warning btn-sm" onclick="editServer(${server.id})">
                                    <i class="bi bi-pencil"></i> 修改
                                </button>
                                <button class="btn btn-outline-danger btn-sm" onclick="deleteServer(${server.id})">
                                    <i class="bi bi-trash"></i> 删除
                                </button>
                            </div>
                        </div>
                    </div>
                `;
            });
            container.innerHTML = html;
        }

        function getCategoryName(category) {
            const names = { direct: '直连', relay: '中转', home: '家宽' };
            return names[category] || category;
        }

        // 隐藏主机信息
        function maskHost(host) {
            // IPv4 地址判断
            const ipv4Regex = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/;
            const ipv4Match = host.match(ipv4Regex);
            if (ipv4Match) {
                // 隐藏 C 段和 D 段：192.168.*.*
                return `${ipv4Match[1]}.${ipv4Match[2]}.*.*`;
            }
            
            // IPv6 地址判断（简化处理）
            if (host.includes(':') && host.split(':').length > 2) {
                // IPv6 隐藏后半部分
                const parts = host.split(':');
                return parts.slice(0, Math.ceil(parts.length / 2)).join(':') + '::**:**';
            }
            
            // 域名处理：隐藏一级域名
            if (host.includes('.')) {
                const parts = host.split('.');
                if (parts.length >= 2) {
                    // 保留顶级域名，隐藏一级域名：*.example.com
                    parts[parts.length - 2] = '*';
                    return parts.join('.');
                }
            }
            
            return host;
        }

        // 隐藏端口信息
        function maskPort(port) {
            const portStr = String(port);
            if (portStr.length <= 3) {
                // 3位及以下，隐藏后两位：8** 或 22
                if (portStr.length === 1) return portStr;
                if (portStr.length === 2) return portStr[0] + '*';
                return portStr[0] + '**';
            } else {
                // 4位及以上，隐藏后三位：8***
                return portStr.substring(0, portStr.length - 3) + '***';
            }
        }

        function showAddModal() {
            editingId = null;
            document.getElementById('serverForm').reset();
            document.getElementById('port').value = 22;
            document.getElementById('core_type').value = 'mihomo';
            document.getElementById('category').value = 'direct';
            document.getElementById('enabled').checked = true;
            document.getElementById('dns_resolve').value = 'none';
            document.getElementById('dnsResolveGroup').style.display = 'none';
            toggleNodeFields();
            new bootstrap.Modal(document.getElementById('serverModal')).show();
        }

        function editServer(id) {
            const server = servers.find(s => s.id === id);
            if (!server) return;

            editingId = id;
            document.getElementById('name').value = server.name || '';
            document.getElementById('host').value = server.host || '';
            document.getElementById('port').value = server.port || 22;
            document.getElementById('category').value = server.category || 'direct';
            document.getElementById('core_type').value = server.core_type || 'sing-box';
            document.getElementById('node_1').value = server.node_1 || '';
            document.getElementById('node_2').value = server.node_2 || '';
            document.getElementById('node_3').value = server.node_3 || '';
            document.getElementById('dns_resolve').value = server.dns_resolve || 'none';
            document.getElementById('enabled').checked = !!server.enabled;
            
            checkHostType(); // 检查主机类型，显示/隐藏 DNS 选项
            toggleNodeFields();
            new bootstrap.Modal(document.getElementById('serverModal')).show();
        }

        async function saveServer() {
            try {
                const data = {
                    name: document.getElementById('name').value,
                    host: document.getElementById('host').value,
                    port: parseInt(document.getElementById('port').value),
                    category: document.getElementById('category').value,
                    core_type: document.getElementById('core_type').value,
                    node_1: document.getElementById('node_1').value,
                    node_2: document.getElementById('node_2').value,
                    node_3: document.getElementById('node_3').value,
                    dns_resolve: document.getElementById('dns_resolve').value,
                    enabled: document.getElementById('enabled').checked ? 1 : 0
                };

                const url = editingId ? `/api/servers/${editingId}` : '/api/servers';
                const method = editingId ? 'PUT' : 'POST';
                
                const res = await fetch(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });

                const result = await res.json();
                showToast(result.message, result.success ? 'success' : 'danger');
                
                if (result.success) {
                    bootstrap.Modal.getInstance(document.getElementById('serverModal')).hide();
                    await loadServers();
                }
            } catch (error) {
                console.error('操作失败:', error);
                showToast('操作失败，请重试', 'danger');
            }
        }

        async function deleteServer(id) {
            const server = servers.find(s => s.id === id);
            if (!server) return;

            showDeleteConfirm({
                itemName: `${server.name} (${server.host})`,
                onConfirm: async (callback) => {
                    try {
                        const res = await fetch(`/api/servers/${id}`, { method: 'DELETE' });
                        const data = await res.json();
                        showToast(data.message, data.success ? 'success' : 'danger');
                        if (data.success) await loadServers();
                        callback(); // 完成后调用回调关闭弹框
                    } catch (error) {
                        console.error('删除服务器失败:', error);
                        showToast('删除服务器失败，请重试', 'danger');
                        callback(); // 即使失败也要关闭弹框
                    }
                }
            });
        }

        async function deploySingle(id) {
            const modal = new bootstrap.Modal(document.getElementById('deployModal'));
            const output = document.getElementById('deployOutput');
            output.textContent = '🚀 正在连接服务器...\n';
            modal.show();

            try {
                const eventSource = new EventSource(`/api/servers/${id}/deploy`);
                let hasError = false;
                let finalMessage = '';

                eventSource.onmessage = function(event) {
                    try {
                        const data = JSON.parse(event.data);
                        
                        if (data.log) {
                            output.textContent += data.log + '\n';
                            output.scrollTop = output.scrollHeight;
                        }
                        
                        if (data.finished !== undefined || data.success !== undefined) {
                            eventSource.close();
                            finalMessage = data.message || '部署完成';
                            hasError = !data.success;
                            
                            if (data.success) {
                                showToast(finalMessage, 'success');
                                loadServers();
                            } else {
                                showToast(finalMessage, 'danger');
                            }
                        }
                    } catch (e) {
                        console.error('解析消息失败:', e);
                    }
                };

                eventSource.onerror = function(error) {
                    console.error('SSE错误:', error);
                    eventSource.close();
                    
                    if (!hasError && !finalMessage) {
                        output.textContent += '\n❌ 连接中断或部署失败\n';
                        showToast('部署失败，请重试', 'danger');
                    }
                };

            } catch (error) {
                console.error('部署失败:', error);
                output.textContent = '❌ 部署请求失败: ' + error.message;
                showToast('部署失败，请重试', 'danger');
            }
        }

        async function deployFolder(id) {
            const modal = new bootstrap.Modal(document.getElementById('deployModal'));
            const output = document.getElementById('deployOutput');
            output.textContent = '🚀 正在连接服务器...\n';
            modal.show();

            try {
                const eventSource = new EventSource(`/api/servers/${id}/deploy-folder`);
                let hasError = false;
                let finalMessage = '';

                eventSource.onmessage = function(event) {
                    try {
                        const data = JSON.parse(event.data);
                        
                        // 处理日志消息
                        if (data.log) {
                            output.textContent += data.log + '\n';
                            // 自动滚动到底部
                            output.scrollTop = output.scrollHeight;
                        }
                        
                        // 处理完成或错误
                        if (data.finished !== undefined || data.success !== undefined) {
                            eventSource.close();
                            finalMessage = data.message || '部署完成';
                            hasError = !data.success;
                            
                            if (data.success) {
                                showToast(finalMessage, 'success');
                                loadServers(); // 刷新服务器列表
                            } else {
                                showToast(finalMessage, 'danger');
                            }
                        }
                    } catch (e) {
                        console.error('解析消息失败:', e);
                    }
                };

                eventSource.onerror = function(error) {
                    console.error('SSE错误:', error);
                    eventSource.close();
                    
                    if (!hasError && !finalMessage) {
                        output.textContent += '\n❌ 连接中断或部署失败\n';
                        showToast('部署失败，请重试', 'danger');
                    }
                };

            } catch (error) {
                console.error('目录部署失败:', error);
                output.textContent = '❌ 目录部署请求失败: ' + error.message;
                showToast('目录部署失败，请重试', 'danger');
            }
        }

        // 全部部署 (使用实时流式日志)
        document.getElementById('deployAllBtn').addEventListener('click', async () => {
            const enabledServers = servers.filter(s => s.enabled);
            if (enabledServers.length === 0) {
                showToast('没有启用的服务器', 'warning');
                return;
            }

            const modal = new bootstrap.Modal(document.getElementById('deployModal'));
            const output = document.getElementById('deployOutput');
            output.textContent = '🚀 批量单文件部署开始...\n\n';
            modal.show();

            let successCount = 0;
            let failCount = 0;

            for (let i = 0; i < enabledServers.length; i++) {
                const server = enabledServers[i];
                output.textContent += `\n${'='.repeat(60)}\n`;
                output.textContent += `📦 [${i + 1}/${enabledServers.length}] ${server.name}\n`;
                output.textContent += `${'='.repeat(60)}\n`;
                output.scrollTop = output.scrollHeight;

                await new Promise((resolve) => {
                    const eventSource = new EventSource(`/api/servers/${server.id}/deploy`);
                    let serverSuccess = false;

                    eventSource.onmessage = function(event) {
                        try {
                            const data = JSON.parse(event.data);
                            
                            if (data.log) {
                                output.textContent += data.log + '\n';
                                output.scrollTop = output.scrollHeight;
                            }
                            
                            if (data.finished !== undefined || data.success !== undefined) {
                                eventSource.close();
                                serverSuccess = data.success;
                                
                                if (data.success) {
                                    successCount++;
                                } else {
                                    failCount++;
                                }
                                resolve();
                            }
                        } catch (e) {
                            console.error('解析消息失败:', e);
                        }
                    };

                    eventSource.onerror = function(error) {
                        console.error('SSE错误:', error);
                        eventSource.close();
                        if (!serverSuccess) {
                            failCount++;
                            output.textContent += '❌ 连接中断或部署失败\n';
                        }
                        resolve();
                    };
                });

                if (i < enabledServers.length - 1) {
                    await new Promise(r => setTimeout(r, 500));
                }
            }

            output.textContent += `\n\n${'='.repeat(60)}\n`;
            output.textContent += `📊 批量部署完成统计\n`;
            output.textContent += `${'='.repeat(60)}\n`;
            output.textContent += `✅ 成功: ${successCount} 台\n`;
            output.textContent += `❌ 失败: ${failCount} 台\n`;
            output.textContent += `📦 总计: ${enabledServers.length} 台\n`;
            output.scrollTop = output.scrollHeight;
            
            showToast(`批量部署完成: ${successCount}成功 ${failCount}失败`, failCount === 0 ? 'success' : 'warning');
            loadServers();
        });

        // 全部目录部署 (使用实时流式日志)
        document.getElementById('deployFolderAllBtn').addEventListener('click', async () => {
            const enabledServers = servers.filter(s => s.enabled && s.category !== 'home');
            if (enabledServers.length === 0) {
                showToast('没有可用的服务器（家宽除外）', 'warning');
                return;
            }

            const modal = new bootstrap.Modal(document.getElementById('deployModal'));
            const output = document.getElementById('deployOutput');
            output.textContent = '🚀 批量目录部署开始...\n\n';
            modal.show();

            let successCount = 0;
            let failCount = 0;

            for (let i = 0; i < enabledServers.length; i++) {
                const server = enabledServers[i];
                output.textContent += `\n${'='.repeat(60)}\n`;
                output.textContent += `📦 [${i + 1}/${enabledServers.length}] ${server.name}\n`;
                output.textContent += `${'='.repeat(60)}\n`;
                output.scrollTop = output.scrollHeight;

                await new Promise((resolve) => {
                    const eventSource = new EventSource(`/api/servers/${server.id}/deploy-folder`);
                    let serverSuccess = false;

                    eventSource.onmessage = function(event) {
                        try {
                            const data = JSON.parse(event.data);
                            
                            if (data.log) {
                                output.textContent += data.log + '\n';
                                output.scrollTop = output.scrollHeight;
                            }
                            
                            if (data.finished !== undefined || data.success !== undefined) {
                                eventSource.close();
                                serverSuccess = data.success;
                                
                                if (data.success) {
                                    successCount++;
                                } else {
                                    failCount++;
                                }
                                resolve();
                            }
                        } catch (e) {
                            console.error('解析消息失败:', e);
                        }
                    };

                    eventSource.onerror = function(error) {
                        console.error('SSE错误:', error);
                        eventSource.close();
                        if (!serverSuccess) {
                            failCount++;
                            output.textContent += '❌ 连接中断或部署失败\n';
                        }
                        resolve();
                    };
                });

                // 服务器之间间隔500ms
                if (i < enabledServers.length - 1) {
                    await new Promise(r => setTimeout(r, 500));
                }
            }

            output.textContent += `\n\n${'='.repeat(60)}\n`;
            output.textContent += `📊 批量部署完成统计\n`;
            output.textContent += `${'='.repeat(60)}\n`;
            output.textContent += `✅ 成功: ${successCount} 台\n`;
            output.textContent += `❌ 失败: ${failCount} 台\n`;
            output.textContent += `📦 总计: ${enabledServers.length} 台\n`;
            output.scrollTop = output.scrollHeight;
            
            showToast(`批量部署完成: ${successCount}成功 ${failCount}失败`, failCount === 0 ? 'success' : 'warning');
            loadServers(); // 刷新服务器列表
        });

        function escapeHtml(text) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
            return String(text).replace(/[&<>"']/g, m => map[m]);
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

        loadServers();
    </script>
</body>
</html>
