<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>节点管理 - SBoard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <link href="/assets/css/style.css" rel="stylesheet">
    <style>
        body {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body>
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
                    <li class="nav-item"><a class="nav-link active" href="/nodes">节点管理</a></li>
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
        <div class="d-flex justify-content-between mb-4">
            <h2>节点管理</h2>
            <button class="btn btn-primary" data-bs-toggle="modal" data-bs-target="#nodeModal" onclick="resetForm()">
                <i class="bi bi-plus-lg"></i> 添加节点
            </button>
        </div>

        <div id="nodesList" class="row"></div>
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

    <!-- 节点模态框 -->
    <div class="modal fade" id="nodeModal" tabindex="-1">
        <div class="modal-dialog modal-lg">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 id="modalTitle">添加节点</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
                </div>
                <form id="nodeForm">
                    <input type="hidden" id="nodeId">
                    <div class="modal-body">
                        <div class="row">
                            <div class="col-md-6 mb-3">
                                <label>标签 *</label>
                                <input type="text" class="form-control" id="tag" required>
                            </div>
                            <div class="col-md-6 mb-3">
                                <label>协议</label>
                                <select class="form-select" id="protocol">
                                    <option value="trojan">Trojan</option>
                                    <option value="vless">VLESS</option>
                                    <option value="vmess">VMess</option>
                                    <option value="anytls">AnyTLS</option>
                                </select>
                            </div>
                            <div class="col-md-6 mb-3">
                                <label>监听地址</label>
                                <input type="text" class="form-control" id="listen" value="::">
                            </div>
                            <div class="col-md-6 mb-3">
                                <label>端口 *</label>
                                <input type="number" class="form-control" id="port" min="1" max="65535" required>
                            </div>
                        </div>
                        <div class="form-check mb-2">
                            <input class="form-check-input" type="checkbox" id="tls_enabled" checked>
                            <label class="form-check-label">启用 TLS</label>
                        </div>
                        <div id="tlsFields">
                            <div class="mb-2">
                                <label>Server Name</label>
                                <input type="text" class="form-control" id="server_name" value="down.dingtalk.com">
                            </div>
                            
                            <!-- 证书字段（Reality 启用时隐藏） -->
                            <div id="certFields">
                                <div class="row">
                                    <div class="col-md-6 mb-2">
                                        <label>证书路径</label>
                                        <input type="text" class="form-control" id="cert_path" value="server.crt">
                                    </div>
                                    <div class="col-md-6 mb-2">
                                        <label>密钥路径</label>
                                        <input type="text" class="form-control" id="key_path" value="server.key">
                                    </div>
                                </div>
                            </div>
                            
                            <!-- Reality 配置 -->
                            <div class="form-check mb-2">
                                <input class="form-check-input" type="checkbox" id="reality_enabled">
                                <label class="form-check-label">启用 Reality</label>
                            </div>
                            <div id="realityFields" style="display: none;">
                                <div class="mb-2">
                                    <label>Reality Server (握手服务器)</label>
                                    <input type="text" class="form-control" id="reality_server" placeholder="www.apple.com">
                                    <small class="text-muted">用于伪装的真实网站，如: www.apple.com</small>
                                </div>
                                <div class="mb-2">
                                    <label>Public Key <button type="button" class="btn btn-sm btn-outline-primary" onclick="generateRealityKeys()"><i class="bi bi-key"></i> 生成密钥对</button></label>
                                    <input type="text" class="form-control" id="reality_pubkey" placeholder="点击生成按钮自动生成">
                                </div>
                                <div class="mb-2">
                                    <label>Private Key (部署时使用)</label>
                                    <input type="text" class="form-control" id="reality_privkey" placeholder="点击生成按钮自动生成" readonly>
                                    <small class="text-muted">此密钥将在部署时写入服务器配置</small>
                                </div>
                            </div>
                        </div>

                        <!-- 传输层配置 -->
                        <div class="form-check mb-2 mt-3">
                            <input class="form-check-input" type="checkbox" id="transport_enabled">
                            <label class="form-check-label">启用传输层</label>
                        </div>
                        <div id="transportFields" style="display: none;">
                            <div class="mb-2">
                                <label>传输层类型</label>
                                <select class="form-select" id="transport_type">
                                    <option value="ws">WebSocket (ws)</option>
                                    <option value="grpc">gRPC</option>
                                    <option value="http">HTTP/2</option>
                                    <option value="httpupgrade">HTTPUpgrade (仅sing-box)</option>
                                </select>
                            </div>
                            <div id="wsFields">
                                <div class="mb-2">
                                    <label>Path</label>
                                    <input type="text" class="form-control" id="ws_path" value="/" placeholder="/path">
                                </div>
                                <div class="mb-2">
                                    <label>Host (伪装域名)</label>
                                    <input type="text" class="form-control" id="transport_host" placeholder="cdn.example.com">
                                </div>
                            </div>
                            <div id="grpcFields" style="display: none;">
                                <div class="mb-2">
                                    <label>gRPC Service Name</label>
                                    <input type="text" class="form-control" id="grpc_service" value="GunService">
                                </div>
                            </div>
                        </div>

                        <!-- Flow 配置 (仅VLESS) -->
                        <div id="flowFields" style="display: none;">
                            <div class="mb-2">
                                <label>Flow (VLESS XTLS)</label>
                                <select class="form-select" id="flow">
                                    <option value="">无</option>
                                    <option value="xtls-rprx-vision">xtls-rprx-vision (推荐)</option>
                                    <option value="xtls-rprx-direct">xtls-rprx-direct (已弃用)</option>
                                </select>
                                <small class="text-muted">仅在 TLS 且不使用传输层时可用</small>
                            </div>
                        </div>

                        <div class="form-check mt-3">
                            <input class="form-check-input" type="checkbox" id="enabled" checked>
                            <label class="form-check-label">启用节点</label>
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

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script src="/assets/js/confirm-modal.js"></script>
    <script>
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

        // 检查并更新字段可用性（互斥规则）
        function updateFieldAvailability() {
            const protocol = document.getElementById('protocol').value;
            const tlsEnabled = document.getElementById('tls_enabled').checked;
            const realityEnabled = document.getElementById('reality_enabled').checked;
            const transportEnabled = document.getElementById('transport_enabled').checked;
            const flowValue = document.getElementById('flow').value;

            // 规则1: Flow 仅 VLESS 支持
            const flowFields = document.getElementById('flowFields');
            flowFields.style.display = protocol === 'vless' ? 'block' : 'none';
            if (protocol !== 'vless') {
                document.getElementById('flow').value = '';
            }

            // 规则2: Reality 和传输层互斥
            if (realityEnabled && transportEnabled) {
                showToast('Reality 不能与传输层同时启用！', 'warning');
                document.getElementById('transport_enabled').checked = false;
                document.getElementById('transportFields').style.display = 'none';
            }

            // 规则3: Flow 和传输层互斥
            if (flowValue && transportEnabled) {
                showToast('Flow (XTLS Vision) 不能与传输层同时启用！', 'warning');
                document.getElementById('transport_enabled').checked = false;
                document.getElementById('transportFields').style.display = 'none';
            }

            // 规则4: Trojan 必须启用 TLS
            if (protocol === 'trojan' && !tlsEnabled) {
                showToast('Trojan 协议必须启用 TLS！', 'warning');
                document.getElementById('tls_enabled').checked = true;
                document.getElementById('tlsFields').style.display = 'block';
            }

            // 规则5: Reality 启用时隐藏证书字段
            const certFields = document.getElementById('certFields');
            certFields.style.display = realityEnabled ? 'none' : 'block';
        }

        // 监听协议变化
        document.getElementById('protocol').addEventListener('change', () => {
            updateFieldAvailability();
        });

        // 监听 TLS 启用状态
        document.getElementById('tls_enabled').addEventListener('change', (e) => {
            document.getElementById('tlsFields').style.display = e.target.checked ? 'block' : 'none';
            if (!e.target.checked) {
                // 关闭 TLS 时也关闭 Reality
                document.getElementById('reality_enabled').checked = false;
                document.getElementById('realityFields').style.display = 'none';
                document.getElementById('certFields').style.display = 'block';
            }
            updateFieldAvailability();
        });

        // 监听 Reality 启用状态
        document.getElementById('reality_enabled').addEventListener('change', (e) => {
            document.getElementById('realityFields').style.display = e.target.checked ? 'block' : 'none';
            updateFieldAvailability();
        });

        // 监听传输层启用状态
        document.getElementById('transport_enabled').addEventListener('change', (e) => {
            document.getElementById('transportFields').style.display = e.target.checked ? 'block' : 'none';
            updateFieldAvailability();
        });

        // 监听 Flow 变化
        document.getElementById('flow').addEventListener('change', () => {
            updateFieldAvailability();
        });

        // 监听传输层类型变化
        document.getElementById('transport_type').addEventListener('change', (e) => {
            const wsFields = document.getElementById('wsFields');
            const grpcFields = document.getElementById('grpcFields');
            
            if (e.target.value === 'grpc') {
                wsFields.style.display = 'none';
                grpcFields.style.display = 'block';
            } else {
                wsFields.style.display = 'block';
                grpcFields.style.display = 'none';
            }
        });

        // 生成 Reality 密钥对
        async function generateRealityKeys() {
            try {
                const response = await fetch('/api/nodes/generate-reality-keys', { method: 'POST' });
                const data = await response.json();
                
                if (data.success) {
                    document.getElementById('reality_pubkey').value = data.data.public_key;
                    document.getElementById('reality_privkey').value = data.data.private_key;
                    showToast('Reality 密钥对生成成功！', 'success');
                } else {
                    showToast('生成失败: ' + data.message, 'danger');
                }
            } catch (error) {
                console.error('生成密钥对错误:', error);
                showToast('生成失败: ' + error.message, 'danger');
            }
        }

        async function loadNodes() {
            try {
                const res = await fetch('/api/nodes');
                const data = await res.json();
                nodes = data.data || [];
                renderNodes();
            } catch (error) {
                console.error('加载节点列表失败:', error);
                showToast('加载节点列表失败，请重试', 'danger');
            }
        }

        function renderNodes() {
            const container = document.getElementById('nodesList');
            if (nodes.length === 0) {
                container.innerHTML = '<div class="col-12 text-center py-5"><p class="text-muted">暂无节点</p></div>';
                return;
            }

            let html = '';
            nodes.forEach(node => {
                const tlsText = node.tls_enabled ? (node.reality_enabled ? 'Reality' : 'TLS') : '关闭';
                const tlsBadge = node.tls_enabled ? (node.reality_enabled ? 'bg-warning' : 'bg-info') : 'bg-secondary';
                const sni = node.server_name || '-';
                
                html += `
                    <div class="col-md-6 col-lg-4 mb-3">
                        <div class="card">
                            <div class="card-body">
                                <div class="d-flex justify-content-between align-items-start mb-2">
                                    <h6 class="mb-0">${node.tag}</h6>
                                    <span class="badge bg-${node.enabled ? 'success' : 'secondary'}">${node.enabled ? '启用' : '禁用'}</span>
                                </div>
                                <p class="small mb-2">
                                    <div class="d-flex justify-content-between mb-1">
                                        <span class="text-muted">协议:</span>
                                        <span class="badge bg-primary">${node.protocol.toUpperCase()}</span>
                                    </div>
                                    <div class="d-flex justify-content-between mb-1">
                                        <span class="text-muted">端口:</span>
                                        <span>${node.port}</span>
                                    </div>
                                    <div class="d-flex justify-content-between mb-1">
                                        <span class="text-muted">TLS:</span>
                                        <span class="badge ${tlsBadge}">${tlsText}</span>
                                    </div>
                                    <div class="d-flex justify-content-between mb-1">
                                        <span class="text-muted">SNI:</span>
                                        <span class="text-truncate" style="max-width: 150px;" title="${sni}">${sni}</span>
                                    </div>
                                </p>
                                <div class="btn-group btn-group-sm w-100">
                                    <button class="btn btn-outline-primary" onclick="editNode(${node.id})">
                                        <i class="bi bi-pencil"></i> 编辑
                                    </button>
                                    <button class="btn btn-outline-danger" onclick="deleteNode(${node.id})">
                                        <i class="bi bi-trash"></i> 删除
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                `;
            });
            container.innerHTML = html;
        }

        function resetForm() {
            editingId = null;
            document.getElementById('modalTitle').textContent = '添加节点';
            document.getElementById('nodeForm').reset();
            document.getElementById('listen').value = '::';
            document.getElementById('server_name').value = 'down.dingtalk.com';
            document.getElementById('cert_path').value = 'server.crt';
            document.getElementById('key_path').value = 'server.key';
            document.getElementById('ws_path').value = '/';
            document.getElementById('grpc_service').value = 'GunService';
            document.getElementById('tls_enabled').checked = true;
            document.getElementById('enabled').checked = true;
            
            // 重置显示状态
            document.getElementById('tlsFields').style.display = 'block';
            document.getElementById('realityFields').style.display = 'none';
            document.getElementById('transportFields').style.display = 'none';
            document.getElementById('flowFields').style.display = 'none';
            document.getElementById('reality_enabled').checked = false;
            document.getElementById('transport_enabled').checked = false;
        }

        function editNode(id) {
            const node = nodes.find(n => n.id === id);
            if (!node) return;

            editingId = id;
            document.getElementById('modalTitle').textContent = '编辑节点';
            document.getElementById('nodeId').value = node.id;
            document.getElementById('tag').value = node.tag;
            document.getElementById('protocol').value = node.protocol;
            document.getElementById('listen').value = node.listen;
            document.getElementById('port').value = node.port;
            document.getElementById('tls_enabled').checked = node.tls_enabled == 1;
            document.getElementById('server_name').value = node.server_name;
            document.getElementById('cert_path').value = node.cert_path;
            document.getElementById('key_path').value = node.key_path;
            document.getElementById('enabled').checked = node.enabled == 1;

            // Reality 字段
            document.getElementById('reality_enabled').checked = node.reality_enabled == 1;
            document.getElementById('reality_server').value = node.reality_server || '';
            document.getElementById('reality_pubkey').value = node.reality_pubkey || '';
            document.getElementById('reality_privkey').value = node.reality_privkey || '';
            
            // 传输层字段
            document.getElementById('transport_enabled').checked = node.transport_enabled == 1;
            document.getElementById('transport_type').value = node.transport_type || 'ws';
            document.getElementById('ws_path').value = node.ws_path || '/';
            document.getElementById('transport_host').value = node.transport_host || '';
            document.getElementById('grpc_service').value = node.grpc_service || 'GunService';
            
            // Flow 字段
            document.getElementById('flow').value = node.flow || '';

            // 显示/隐藏相关字段
            document.getElementById('tlsFields').style.display = node.tls_enabled == 1 ? 'block' : 'none';
            document.getElementById('realityFields').style.display = node.reality_enabled == 1 ? 'block' : 'none';
            document.getElementById('transportFields').style.display = node.transport_enabled == 1 ? 'block' : 'none';
            document.getElementById('flowFields').style.display = node.protocol === 'vless' ? 'block' : 'none';
            
            // 传输层类型字段
            if (node.transport_type === 'grpc') {
                document.getElementById('wsFields').style.display = 'none';
                document.getElementById('grpcFields').style.display = 'block';
            } else {
                document.getElementById('wsFields').style.display = 'block';
                document.getElementById('grpcFields').style.display = 'none';
            }

            new bootstrap.Modal(document.getElementById('nodeModal')).show();
        }

        async function deleteNode(id) {
            const node = nodes.find(n => n.id === id);
            if (!node) return;

            showDeleteConfirm({
                itemName: `${node.tag} (${node.protocol.toUpperCase()})`,
                onConfirm: async (callback) => {
                    try {
                        const res = await fetch(`/api/nodes/${id}`, { method: 'DELETE' });
                        const data = await res.json();
                        showToast(data.message, data.success ? 'success' : 'danger');
                        if (data.success) await loadNodes();
                        callback(); // 完成后调用回调关闭弹框
                    } catch (error) {
                        console.error('删除节点失败:', error);
                        showToast('删除节点失败，请重试', 'danger');
                        callback(); // 即使失败也要关闭弹框
                    }
                }
            });
        }

        document.getElementById('nodeForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const protocol = document.getElementById('protocol').value;
            const tlsEnabled = document.getElementById('tls_enabled').checked;
            const realityEnabled = document.getElementById('reality_enabled').checked;
            const transportEnabled = document.getElementById('transport_enabled').checked;
            const flowValue = document.getElementById('flow').value;

            // 验证规则1: Trojan 必须启用 TLS
            if (protocol === 'trojan' && !tlsEnabled) {
                showToast('Trojan 协议必须启用 TLS！', 'danger');
                return;
            }

            // 验证规则2: Reality 和传输层互斥
            if (realityEnabled && transportEnabled) {
                showToast('Reality 不能与传输层同时启用！', 'danger');
                return;
            }

            // 验证规则3: Flow 和传输层互斥
            if (flowValue && transportEnabled) {
                showToast('Flow (XTLS Vision) 不能与传输层同时启用！', 'danger');
                return;
            }

            // 验证规则4: Flow 仅 VLESS 支持
            if (flowValue && protocol !== 'vless') {
                showToast('Flow 仅 VLESS 协议支持！', 'danger');
                return;
            }

            // 验证规则5: Reality 需要 TLS
            if (realityEnabled && !tlsEnabled) {
                showToast('Reality 需要先启用 TLS！', 'danger');
                return;
            }
            
            try {
                const data = {
                    tag: document.getElementById('tag').value,
                    protocol: protocol,
                    listen: document.getElementById('listen').value,
                    port: parseInt(document.getElementById('port').value),
                    tls_enabled: tlsEnabled ? 1 : 0,
                    server_name: document.getElementById('server_name').value,
                    cert_path: document.getElementById('cert_path').value,
                    key_path: document.getElementById('key_path').value,
                    reality_enabled: realityEnabled ? 1 : 0,
                    reality_server: document.getElementById('reality_server').value,
                    reality_pubkey: document.getElementById('reality_pubkey').value,
                    reality_privkey: document.getElementById('reality_privkey').value,
                    transport_enabled: transportEnabled ? 1 : 0,
                    transport_type: document.getElementById('transport_type').value,
                    ws_path: document.getElementById('ws_path').value,
                    transport_host: document.getElementById('transport_host').value,
                    grpc_service: document.getElementById('grpc_service').value,
                    flow: protocol === 'vless' ? flowValue : '', // 仅 VLESS 保留 flow
                    enabled: document.getElementById('enabled').checked ? 1 : 0
                };

                const url = editingId ? `/api/nodes/${editingId}` : '/api/nodes';
                const method = editingId ? 'PUT' : 'POST';
                
                const res = await fetch(url, {
                    method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });

                const result = await res.json();
                showToast(result.message, result.success ? 'success' : 'danger');
                
                if (result.success) {
                    bootstrap.Modal.getInstance(document.getElementById('nodeModal')).hide();
                    await loadNodes();
                }
            } catch (error) {
                console.error('操作失败:', error);
                showToast('操作失败，请重试', 'danger');
            }
        });

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

        loadNodes();
    </script>
</body>
</html>
