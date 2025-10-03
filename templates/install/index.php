<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SBoard 安装向导</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.0/font/bootstrap-icons.css" rel="stylesheet">
    <style>
        .install-container {
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        .step-indicator {
            display: flex;
            justify-content: center;
            margin-bottom: 30px;
        }
        .step {
            display: flex;
            align-items: center;
            margin: 0 10px;
        }
        .step-number {
            width: 40px;
            height: 40px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: bold;
            margin-right: 10px;
        }
        .step.active .step-number {
            background-color: #0d6efd;
            color: white;
        }
        .step.completed .step-number {
            background-color: #198754;
            color: white;
        }
        .step.inactive .step-number {
            background-color: #e9ecef;
            color: #6c757d;
        }
        .check-item {
            border: 1px solid #dee2e6;
            border-radius: 8px;
            padding: 15px;
            margin-bottom: 10px;
            transition: all 0.3s ease;
        }
        .check-item.passed {
            border-color: #198754;
            background-color: #f8fff9;
        }
        .check-item.failed {
            border-color: #dc3545;
            background-color: #fff8f8;
        }
        .ssh-upload-area {
            border: 2px dashed #dee2e6;
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            transition: all 0.3s ease;
            cursor: pointer;
        }
        .ssh-upload-area:hover {
            border-color: #0d6efd;
            background-color: #f8f9ff;
        }
        .ssh-upload-area.dragover {
            border-color: #0d6efd;
            background-color: #e7f1ff;
        }
        .progress-step {
            display: none;
        }
        .progress-step.active {
            display: block;
        }
        .logo {
            text-align: center;
            margin-bottom: 30px;
        }
        .logo h1 {
            color: #0d6efd;
            font-weight: bold;
            margin-bottom: 5px;
        }
        .logo p {
            color: #6c757d;
            margin: 0;
        }
    </style>
</head>
<body class="bg-light">
    <div class="install-container">
        <div class="logo">
            <h1><i class="bi bi-box-seam"></i> SBoard</h1>
            <p>代理配置管理系统 - 安装向导</p>
        </div>

        <!-- 步骤指示器 -->
        <div class="step-indicator">
            <div class="step active" id="step-1-indicator">
                <div class="step-number">1</div>
                <span>环境检查</span>
            </div>
            <div class="step inactive" id="step-2-indicator">
                <div class="step-number">2</div>
                <span>SSH 配置</span>
            </div>
            <div class="step inactive" id="step-3-indicator">
                <div class="step-number">3</div>
                <span>数据库安装</span>
            </div>
            <div class="step inactive" id="step-4-indicator">
                <div class="step-number">4</div>
                <span>安装完成</span>
            </div>
        </div>

        <!-- 步骤 1: 环境检查 -->
        <div class="progress-step active" id="step-1">
            <div class="card">
                <div class="card-header">
                    <h5 class="mb-0"><i class="bi bi-gear"></i> 环境检查</h5>
                </div>
                <div class="card-body">
                    <p class="text-muted">正在检查系统环境是否满足 SBoard 运行要求...</p>
                    
                    <div id="environment-checks">
                        <div class="text-center">
                            <div class="spinner-border text-primary" role="status">
                                <span class="visually-hidden">检查中...</span>
                            </div>
                            <p class="mt-2">正在检查环境...</p>
                        </div>
                    </div>

                    <div class="mt-3 d-none" id="step-1-actions">
                        <button class="btn btn-primary" onclick="nextStep(2)" id="next-to-ssh">下一步: SSH 配置</button>
                        <button class="btn btn-outline-secondary" onclick="checkEnvironment()">重新检查</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- 步骤 2: SSH 配置 -->
        <div class="progress-step" id="step-2">
            <div class="card">
                <div class="card-header">
                    <h5 class="mb-0"><i class="bi bi-key"></i> SSH 密钥配置</h5>
                </div>
                <div class="card-body">
                    <p class="text-muted">上传 SSH 私钥以便系统自动部署到目标服务器。</p>
                    
                    <div class="ssh-upload-area" id="ssh-upload-area" onclick="document.getElementById('ssh-key-file').click()">
                        <i class="bi bi-cloud-upload fs-1 text-muted"></i>
                        <h5 class="mt-3">上传 SSH 私钥</h5>
                        <p class="text-muted">点击选择文件或拖拽文件到此处</p>
                        <p class="small text-muted">支持 id_rsa, id_ed25519 等格式的私钥文件</p>
                    </div>
                    
                    <input type="file" id="ssh-key-file" accept=".rsa,.key,*" style="display: none;">
                    <input type="file" id="ssh-pub-file" accept=".pub,*" style="display: none;">
                    
                    <div class="mt-3">
                        <div class="form-check">
                            <input class="form-check-input" type="checkbox" id="upload-public-key">
                            <label class="form-check-label" for="upload-public-key">
                                同时上传公钥文件 (可选)
                            </label>
                        </div>
                        <div class="mt-2 d-none" id="public-key-upload">
                            <button class="btn btn-outline-secondary btn-sm" onclick="document.getElementById('ssh-pub-file').click()">
                                <i class="bi bi-upload"></i> 选择公钥文件
                            </button>
                            <span id="pub-file-name" class="ms-2 text-muted"></span>
                        </div>
                    </div>

                    <div class="alert alert-info mt-3">
                        <i class="bi bi-info-circle"></i>
                        <strong>提示:</strong> SSH 私钥将被保存到 <code>/var/www/.ssh/id_rsa</code>，用于自动部署配置文件到目标服务器。
                    </div>

                    <div class="mt-3">
                        <button class="btn btn-outline-secondary" onclick="previousStep(1)">上一步</button>
                        <button class="btn btn-primary" onclick="nextStep(3)" id="next-to-database" disabled>下一步: 数据库安装</button>
                        <button class="btn btn-outline-info" onclick="skipSshConfig()">跳过 SSH 配置</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- 步骤 3: 数据库安装 -->
        <div class="progress-step" id="step-3">
            <div class="card">
                <div class="card-header">
                    <h5 class="mb-0"><i class="bi bi-database"></i> 数据库安装</h5>
                </div>
                <div class="card-body">
                    <p class="text-muted">创建数据库表并设置管理员账户。</p>
                    
                    <form id="database-form">
                        <div class="row">
                            <div class="col-md-6">
                                <div class="mb-3">
                                    <label for="admin-username" class="form-label">管理员用户名</label>
                                    <input type="text" class="form-control" id="admin-username" required minlength="3" autocomplete="username">
                                    <div class="form-text">用于登录管理后台</div>
                                </div>
                            </div>
                            <div class="col-md-6">
                                <div class="mb-3">
                                    <label for="admin-password" class="form-label">管理员密码</label>
                                    <input type="password" class="form-control" id="admin-password" required minlength="6" autocomplete="new-password">
                                    <div class="form-text">至少 6 位字符</div>
                                </div>
                            </div>
                        </div>
                        
                        <div class="mb-3">
                            <label for="confirm-password" class="form-label">确认密码</label>
                            <input type="password" class="form-control" id="confirm-password" required autocomplete="new-password">
                        </div>
                    </form>

                    <div class="alert alert-warning">
                        <i class="bi bi-exclamation-triangle"></i>
                        <strong>注意:</strong> 请牢记管理员账户信息，这是唯一的系统管理账户。
                    </div>

                    <div class="mt-3">
                        <button class="btn btn-outline-secondary" onclick="previousStep(2)">上一步</button>
                        <button class="btn btn-success" onclick="installDatabase()" id="install-database-btn">
                            <i class="bi bi-download"></i> 开始安装
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- 步骤 4: 安装完成 -->
        <div class="progress-step" id="step-4">
            <div class="card">
                <div class="card-header">
                    <h5 class="mb-0"><i class="bi bi-check-circle"></i> 安装完成</h5>
                </div>
                <div class="card-body text-center">
                    <div class="mb-4">
                        <i class="bi bi-check-circle-fill text-success" style="font-size: 4rem;"></i>
                    </div>
                    
                    <h4 class="text-success mb-3">🎉 安装成功完成!</h4>
                    <p class="text-muted mb-4">SBoard 代理配置管理系统已成功安装并配置完成。</p>
                    
                    <div class="alert alert-success text-start">
                        <h6><i class="bi bi-info-circle"></i> 安装信息</h6>
                        <ul class="mb-0">
                            <li>管理后台地址: <strong><?= $_SERVER['HTTP_HOST'] ?? 'localhost' ?></strong></li>
                            <li>管理员用户名: <strong id="final-username">-</strong></li>
                            <li>数据库类型: <strong>SQLite3</strong></li>
                            <li>SSH 密钥: <strong id="ssh-status">未配置</strong></li>
                        </ul>
                    </div>

                    <div class="alert alert-info text-start">
                        <h6><i class="bi bi-lightbulb"></i> 下一步操作</h6>
                        <ol class="mb-0">
                            <li>点击下方按钮进入管理后台</li>
                            <li>添加代理用户和节点配置</li>
                            <li>配置服务器信息用于自动部署</li>
                            <li>生成订阅链接供客户端使用</li>
                        </ol>
                    </div>

                    <div class="mt-4">
                        <a href="/login" class="btn btn-primary btn-lg">
                            <i class="bi bi-arrow-right"></i> 进入管理后台
                        </a>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
    <script>
        let currentStep = 1;
        let environmentPassed = false;
        let sshConfigured = false;

        // 页面加载完成后开始环境检查
        document.addEventListener('DOMContentLoaded', function() {
            checkEnvironment();
            setupFileUploads();
        });

        // 环境检查
        async function checkEnvironment() {
            try {
                const response = await fetch('/api/install/check-environment');
                const data = await response.json();
                
                if (data.success) {
                    displayEnvironmentChecks(data.checks);
                    environmentPassed = data.all_passed;
                    
                    if (environmentPassed) {
                        document.getElementById('step-1-actions').classList.remove('d-none');
                        updateStepIndicator(1, 'completed');
                    } else {
                        document.getElementById('step-1-actions').classList.add('d-none');
                        updateStepIndicator(1, 'active');
                    }
                }
            } catch (error) {
                console.error('环境检查失败:', error);
                document.getElementById('environment-checks').innerHTML = 
                    '<div class="alert alert-danger">环境检查失败，请检查网络连接。</div>';
            }
        }

        // 显示环境检查结果
        function displayEnvironmentChecks(checks) {
            let html = '';
            
            Object.values(checks).forEach(check => {
                const statusClass = check.passed ? 'passed' : 'failed';
                const statusIcon = check.passed ? 'check-circle-fill text-success' : 'x-circle-fill text-danger';
                
                html += `
                    <div class="check-item ${statusClass}">
                        <div class="d-flex justify-content-between align-items-center">
                            <div>
                                <strong>${check.name}</strong>
                                <div class="small text-muted">要求: ${check.required} | 当前: ${check.current}</div>
                            </div>
                            <i class="bi bi-${statusIcon} fs-4"></i>
                        </div>
                        <div class="small mt-2">${check.message}</div>
                    </div>
                `;
            });
            
            document.getElementById('environment-checks').innerHTML = html;
        }

        // 设置文件上传
        function setupFileUploads() {
            const uploadArea = document.getElementById('ssh-upload-area');
            const keyFileInput = document.getElementById('ssh-key-file');
            const pubFileInput = document.getElementById('ssh-pub-file');
            const uploadPublicKeyCheckbox = document.getElementById('upload-public-key');
            const publicKeyUpload = document.getElementById('public-key-upload');

            // 拖拽上传
            uploadArea.addEventListener('dragover', (e) => {
                e.preventDefault();
                uploadArea.classList.add('dragover');
            });

            uploadArea.addEventListener('dragleave', () => {
                uploadArea.classList.remove('dragover');
            });

            uploadArea.addEventListener('drop', (e) => {
                e.preventDefault();
                uploadArea.classList.remove('dragover');
                
                const files = e.dataTransfer.files;
                if (files.length > 0) {
                    keyFileInput.files = files;
                    handleSshKeyUpload();
                }
            });

            // 文件选择
            keyFileInput.addEventListener('change', handleSshKeyUpload);
            pubFileInput.addEventListener('change', function() {
                document.getElementById('pub-file-name').textContent = this.files[0]?.name || '';
            });

            // 公钥上传选项
            uploadPublicKeyCheckbox.addEventListener('change', function() {
                if (this.checked) {
                    publicKeyUpload.classList.remove('d-none');
                } else {
                    publicKeyUpload.classList.add('d-none');
                    pubFileInput.value = '';
                    document.getElementById('pub-file-name').textContent = '';
                }
            });
        }

        // 处理 SSH 密钥上传
        async function handleSshKeyUpload() {
            const keyFile = document.getElementById('ssh-key-file').files[0];
            const pubFile = document.getElementById('ssh-pub-file').files[0];
            
            if (!keyFile) return;

            const formData = new FormData();
            formData.append('ssh_key', keyFile);
            if (pubFile) {
                formData.append('ssh_pub', pubFile);
            }

            try {
                const response = await fetch('/api/install/upload-ssh-key', {
                    method: 'POST',
                    body: formData
                });

                const data = await response.json();
                
                if (data.success) {
                    sshConfigured = true;
                    document.getElementById('next-to-database').disabled = false;
                    document.getElementById('ssh-upload-area').innerHTML = `
                        <i class="bi bi-check-circle-fill text-success fs-1"></i>
                        <h5 class="mt-3 text-success">SSH 密钥上传成功</h5>
                        <p class="text-muted">文件: ${keyFile.name}</p>
                    `;
                    
                    showAlert('success', 'SSH 密钥上传成功！');
                } else {
                    showAlert('danger', data.message || 'SSH 密钥上传失败');
                }
            } catch (error) {
                console.error('上传失败:', error);
                showAlert('danger', '网络错误，上传失败');
            }
        }

        // 跳过 SSH 配置
        function skipSshConfig() {
            if (confirm('确定要跳过 SSH 配置吗？跳过后将无法使用自动部署功能。')) {
                sshConfigured = true;
                document.getElementById('next-to-database').disabled = false;
                showAlert('warning', '已跳过 SSH 配置，可在安装完成后手动配置。');
            }
        }

        // 数据库安装
        async function installDatabase() {
            const username = document.getElementById('admin-username').value.trim();
            const password = document.getElementById('admin-password').value;
            const confirmPassword = document.getElementById('confirm-password').value;

            if (!username || !password || !confirmPassword) {
                showAlert('warning', '请填写所有必需字段');
                return;
            }

            if (password !== confirmPassword) {
                showAlert('warning', '两次输入的密码不一致');
                return;
            }

            if (password.length < 6) {
                showAlert('warning', '密码长度至少为 6 位');
                return;
            }

            const installBtn = document.getElementById('install-database-btn');
            const originalText = installBtn.innerHTML;
            installBtn.innerHTML = '<span class="spinner-border spinner-border-sm me-2"></span>安装中...';
            installBtn.disabled = true;

            try {
                const response = await fetch('/api/install/database', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: new URLSearchParams({
                        admin_username: username,
                        admin_password: password
                    })
                });

                const data = await response.json();

                if (data.success) {
                    document.getElementById('final-username').textContent = username;
                    document.getElementById('ssh-status').textContent = sshConfigured ? '已配置' : '未配置';
                    nextStep(4);
                    showAlert('success', '数据库安装成功！');
                } else {
                    showAlert('danger', data.message || '数据库安装失败');
                    installBtn.innerHTML = originalText;
                    installBtn.disabled = false;
                }
            } catch (error) {
                console.error('安装失败:', error);
                showAlert('danger', '网络错误，安装失败');
                installBtn.innerHTML = originalText;
                installBtn.disabled = false;
            }
        }

        // 步骤导航
        function nextStep(step) {
            if (step === 2 && !environmentPassed) {
                showAlert('warning', '请先通过环境检查');
                return;
            }

            currentStep = step;
            updateStepDisplay();
        }

        function previousStep(step) {
            currentStep = step;
            updateStepDisplay();
        }

        function updateStepDisplay() {
            // 隐藏所有步骤
            document.querySelectorAll('.progress-step').forEach(step => {
                step.classList.remove('active');
            });

            // 显示当前步骤
            document.getElementById(`step-${currentStep}`).classList.add('active');

            // 更新步骤指示器
            for (let i = 1; i <= 4; i++) {
                if (i < currentStep) {
                    updateStepIndicator(i, 'completed');
                } else if (i === currentStep) {
                    updateStepIndicator(i, 'active');
                } else {
                    updateStepIndicator(i, 'inactive');
                }
            }
        }

        function updateStepIndicator(step, status) {
            const indicator = document.getElementById(`step-${step}-indicator`);
            indicator.classList.remove('active', 'completed', 'inactive');
            indicator.classList.add(status);
        }

        // 显示提示信息
        function showAlert(type, message) {
            const alertDiv = document.createElement('div');
            alertDiv.className = `alert alert-${type} alert-dismissible fade show position-fixed`;
            alertDiv.style.cssText = 'top: 20px; right: 20px; z-index: 9999; max-width: 400px;';
            alertDiv.innerHTML = `
                ${message}
                <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
            `;
            
            document.body.appendChild(alertDiv);
            
            // 5秒后自动消失
            setTimeout(() => {
                if (alertDiv.parentNode) {
                    alertDiv.remove();
                }
            }, 5000);
        }
    </script>
</body>
</html>