# SBoard Agent Windows 安装脚本
# 支持: Windows (amd64, arm64)
# 
# 用法 (以管理员身份运行 PowerShell):
#
#   方式1: 设置环境变量后通过管道安装 (推荐)
#     $env:TOKEN="your-token"; $env:PANEL="https://your-panel.com"; irm https://xxx/install-agent.ps1 | iex
#
#   方式2: 下载脚本后带参数运行
#     .\install-agent.ps1 -Token "your-token" -PanelUrl "https://your-panel.com"
#
#   方式3: 交互式输入
#     irm https://xxx/install-agent.ps1 | iex  # 会提示输入 Token 和 Panel

param(
    [Parameter(Mandatory=$false)]
    [string]$Token,
    
    [Parameter(Mandatory=$false)]
    [string]$PanelUrl,
    
    [Parameter(Mandatory=$false)]
    
    [Parameter(Mandatory=$false)]
    [switch]$Uninstall,
    
    [Parameter(Mandatory=$false)]
    [switch]$Help
)

# 配置
$GITHUB_REPO = "amuae/sboard"
$INSTALL_DIR = "C:\sboard\agent"
$SERVICE_NAME = "sboard-agent"
$BINARY_NAME = "sboard-agent.exe"
$CONFIG_FILE = "agent.json"
$DEV_DOMAIN_HASH = "9de17c968ada26abec13fc5fc264ddfa"
$script:DEV_MODE = $false

# GitHub 加速配置 (国内加速)
$GH_PROXY = "https://ghfast.top/"

# 颜色输出函数
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[SUCCESS] $args" -ForegroundColor Green }
function Write-Warning { Write-Host "[WARNING] $args" -ForegroundColor Yellow }
function Write-Error { Write-Host "[ERROR] $args" -ForegroundColor Red; exit 1 }

# 显示帮助
function Show-Help {
    Write-Host "SBoard Agent Windows 安装脚本"
    Write-Host ""
    Write-Host "用法:"
    Write-Host ""
    Write-Host "  方式1: 设置环境变量后通过管道安装 (推荐)"
    Write-Host '    $env:TOKEN="your-token"; $env:PANEL="https://panel.example.com"; irm <url> | iex'
    Write-Host ""
    Write-Host "  方式2: 下载脚本后带参数运行"
    Write-Host "    .\install-agent.ps1 -Token <token> -PanelUrl <url>"
    Write-Host ""
    Write-Host "参数:"
    Write-Host "  -Token <token>      Agent 认证 Token (必填，或设置 `$env:TOKEN)"
    Write-Host "  -PanelUrl <url>     面板地址 (必填，或设置 `$env:PANEL)"
    Write-Host "  -CoreType <type>    核心类型: sing-box 或 mihomo (默认: sing-box)"
    Write-Host "  -Uninstall          卸载 Agent"
    Write-Host "  -Help               显示帮助"
    Write-Host ""
    Write-Host "环境变量:"
    Write-Host "  TOKEN               Agent 认证 Token"
    Write-Host "  PANEL               面板地址"
    Write-Host "  CORE_TYPE           核心类型 (可选)"
    Write-Host ""
    Write-Host "示例:"
    Write-Host '  $env:TOKEN="abc123"; $env:PANEL="https://panel.example.com"; irm <url> | iex'
    Write-Host "  .\install-agent.ps1 -Token abc123 -PanelUrl https://panel.example.com"
    Write-Host "  .\install-agent.ps1 -Uninstall"
    Write-Host ""
    Write-Host "支持的架构: amd64, arm64, 386"
    Write-Host ""
    exit 0
}

# 检查管理员权限
function Test-Administrator {
    $currentUser = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentUser.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# 从面板 URL 提取域名并检查是否为开发者域名
function Test-DevDomainFromUrl {
    param([string]$Url)
    
    if ([string]::IsNullOrEmpty($Url)) {
        return
    }
    
    # 提取域名 (去掉协议和端口)
    $panelDomain = $Url -replace '^https?://', '' -replace ':[0-9]+.*', '' -replace '/.*', ''
    
    if ([string]::IsNullOrEmpty($panelDomain)) {
        return
    }
    
    # 计算域名的 MD5
    $md5 = [System.Security.Cryptography.MD5]::Create()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($panelDomain)
    $hash = $md5.ComputeHash($bytes)
    $domainHash = [BitConverter]::ToString($hash).Replace("-", "").ToLower()
    
    # 检查是否匹配开发者域名
    if ($domainHash -eq $DEV_DOMAIN_HASH) {
        $script:DEV_MODE = $true
    }
}

# 检测架构
function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" { return "386" }
        "X86" { return "386" }
        default { Write-Error "不支持的架构: $arch，支持: amd64, arm64, 386" }
    }
}

# 停止服务
function Stop-AgentService {
    Write-Info "检查现有服务..."
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -eq "Running") {
            Write-Info "停止服务..."
            Stop-Service -Name $SERVICE_NAME -Force
            Start-Sleep -Seconds 2
        }
    }
}

# 卸载
function Uninstall-Agent {
    Write-Info "开始卸载 SBoard Agent..."
    
    # 停止并删除服务
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -eq "Running") {
            Stop-Service -Name $SERVICE_NAME -Force
            Start-Sleep -Seconds 2
        }
        Write-Info "删除 Windows 服务..."
        sc.exe delete $SERVICE_NAME | Out-Null
    }
    
    # 删除安装目录
    if (Test-Path $INSTALL_DIR) {
        Write-Info "删除安装目录..."
        Remove-Item -Path $INSTALL_DIR -Recurse -Force
    }
    
    Write-Success "SBoard Agent 已卸载"
    exit 0
}

# 下载 Agent
function Download-Agent {
    param($Arch)
    
    Write-Info "下载 Agent..."
    
    # 创建安装目录
    if (-not (Test-Path $INSTALL_DIR)) {
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
    }
    
    # 构建下载 URL
    $downloadFile = "${BINARY_NAME.Replace('.exe', '')}_windows_${Arch}.zip"
    if ($script:DEV_MODE) {
        Write-Warning "开发者模式：使用预发布版本"
        $downloadUrl = "${GH_PROXY}https://github.com/$GITHUB_REPO/releases/download/pre-release/$downloadFile"
    } else {
        $downloadUrl = "${GH_PROXY}https://github.com/$GITHUB_REPO/releases/latest/download/$downloadFile"
    }
    $tempZip = Join-Path $env:TEMP $downloadFile
    
    Write-Info "下载: $downloadUrl"
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempZip -UseBasicParsing -TimeoutSec 60
        Write-Info "下载成功"
    } catch {
        Write-Error "下载失败: $_"
    }
    
    try {
        # 解压
        Expand-Archive -Path $tempZip -DestinationPath $INSTALL_DIR -Force
        
        # 清理
        Remove-Item $tempZip -Force
        
        Write-Success "Agent 下载完成"
    } catch {
        Write-Error "解压失败: $_"
    }
}

# 生成配置文件
function Generate-Config {
    Write-Info "生成配置文件..."
    
    # 获取主机名作为 Agent ID
    $agentId = $env:COMPUTERNAME
    
    # 设置核心路径
    # 固定使用 sing-box
    $corePath = "C:\sboard\sing-box\sing-box.exe"
    $configDir = "C:\sboard\sing-box"
    
    # 生成配置 JSON
    $config = @{
        panel_url = $PanelUrl
        token = $Token
        agent_id = $AgentId
        core_path = $corePath
        config_dir = $configDir
    } | ConvertTo-Json -Depth 10
    
    $configPath = Join-Path $INSTALL_DIR $CONFIG_FILE
    Set-Content -Path $configPath -Value $config -Encoding UTF8
    
    Write-Success "配置文件已生成: $configPath"
}

# 创建 Windows 服务
function Create-WindowsService {
    Write-Info "创建 Windows 服务..."
    
    $binaryPath = Join-Path $INSTALL_DIR $BINARY_NAME
    $configPath = Join-Path $INSTALL_DIR $CONFIG_FILE
    
    # 检查服务是否已存在
    $existingService = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Info "服务已存在，删除旧服务..."
        sc.exe delete $SERVICE_NAME | Out-Null
        Start-Sleep -Seconds 2
    }
    
    # 创建服务
    $binPathEscaped = "`"$binaryPath`" -c `"$configPath`""
    sc.exe create $SERVICE_NAME binPath= $binPathEscaped start= auto DisplayName= "SBoard Agent" | Out-Null
    
    # 设置服务描述
    sc.exe description $SERVICE_NAME "SBoard Agent - Proxy Node Management" | Out-Null
    
    # 设置失败后自动重启
    sc.exe failure $SERVICE_NAME reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
    
    Write-Success "Windows 服务已创建"
}

# 启动服务
function Start-AgentService {
    Write-Info "启动服务..."
    
    Start-Service -Name $SERVICE_NAME
    Start-Sleep -Seconds 3
    
    $service = Get-Service -Name $SERVICE_NAME
    if ($service.Status -eq "Running") {
        Write-Success "服务启动成功"
    } else {
        Write-Error "服务启动失败，请检查日志"
    }
}

# 显示状态
function Show-Status {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "SBoard Agent 安装完成" -ForegroundColor Green
    Write-Host "=========================================="
    Write-Host ""
    Write-Host "安装目录: $INSTALL_DIR"
    Write-Host "配置文件: $(Join-Path $INSTALL_DIR $CONFIG_FILE)"
    Write-Host "服务名称: $SERVICE_NAME"
    Write-Host "核心类型: $CoreType"
    Write-Host ""
    Write-Host "常用命令:"
    Write-Host "  查看状态: Get-Service $SERVICE_NAME"
    Write-Host "  查看日志: Get-EventLog -LogName Application -Source $SERVICE_NAME -Newest 50"
    Write-Host "  重启服务: Restart-Service $SERVICE_NAME"
    Write-Host "  停止服务: Stop-Service $SERVICE_NAME"
    Write-Host ""
    Write-Host "或使用 services.msc 管理服务"
    Write-Host ""
    Write-Host "卸载命令:"
    Write-Host "  .\install-agent.ps1 -Uninstall"
    Write-Host ""
}

# 主函数
function Main {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "    SBoard Agent Windows 安装脚本"
    Write-Host "=========================================="
    Write-Host ""
    
    # 显示帮助
    if ($Help) {
        Show-Help
    }
    
    # 卸载
    if ($Uninstall) {
        Uninstall-Agent
    }
    
    # 检查管理员权限
    if (-not (Test-Administrator)) {
        Write-Error "请以管理员身份运行此脚本"
    }
    
    # 从环境变量读取参数 (如果命令行没有提供)
    if (-not $Token -and $env:TOKEN) {
        $Token = $env:TOKEN
        Write-Info "从环境变量读取 Token"
    }
    if (-not $PanelUrl -and $env:PANEL) {
        $PanelUrl = $env:PANEL
        Write-Info "从环境变量读取 Panel 地址"
    }
    # 环境变量检查已移除，固定使用 sing-box
    
    # 交互式输入 (如果仍然缺少参数)
    if (-not $Token) {
        Write-Host ""
        $Token = Read-Host "请输入 Agent Token"
        if (-not $Token) {
            Write-Error "Token 不能为空"
        }
    }
    if (-not $PanelUrl) {
        $PanelUrl = Read-Host "请输入面板地址 (如 https://panel.example.com)"
        if (-not $PanelUrl) {
            Write-Error "面板地址不能为空"
        }
    }
    
    # 移除末尾斜杠
    $PanelUrl = $PanelUrl.TrimEnd('/')
    
    # 从面板 URL 提取域名并检查是否为开发者域名
    Test-DevDomainFromUrl -Url $PanelUrl
    
    # 检测架构
    $arch = Get-Architecture
    Write-Info "检测到架构: $arch"
    
    # 停止现有服务
    Stop-AgentService
    
    # 下载 Agent
    Download-Agent -Arch $arch
    
    # 生成配置
    Generate-Config
    
    # 创建服务
    Create-WindowsService
    
    # 启动服务
    Start-AgentService
    
    # 显示状态
    Show-Status
}

# 执行
Main
