# SBoard 面板 Windows 安装脚本
# 支持: Windows (amd64, arm64, 386)
# 
# 用法 (以管理员身份运行 PowerShell):
#
#   方式1: 通过管道直接安装 (推荐)
#     irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex
#
#   方式2: 设置环境变量后安装
#     $env:PORT="9000"; $env:USER="admin"; $env:PASS="mypassword"; irm <url> | iex
#
#   方式3: 下载脚本后带参数运行
#     .\install-sboard.ps1 -Port 9000 -User admin -Pass mypassword

param(
    [Parameter(Mandatory=$false)]
    [string]$InstallDir = "C:\sboard",
    
    [Parameter(Mandatory=$false)]
    [int]$Port = 8080,
    
    [Parameter(Mandatory=$false)]
    [string]$User,
    
    [Parameter(Mandatory=$false)]
    [string]$Pass,
    
    [Parameter(Mandatory=$false)]
    [switch]$Update,
    
    [Parameter(Mandatory=$false)]
    [switch]$Uninstall,
    
    [Parameter(Mandatory=$false)]
    [switch]$NoInteractive,
    
    [Parameter(Mandatory=$false)]
    [switch]$Help
)

# 配置
$GITHUB_REPO = "amuae/sboard"
$SERVICE_NAME = "sboard"
$BINARY_NAME = "sboard.exe"
$CONFIG_FILE = "config.yaml"

# GitHub 加速配置 (国内加速)
$GH_PROXY = "https://ghfast.top/"

# 颜色输出函数
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[SUCCESS] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARNING] $args" -ForegroundColor Yellow }
function Write-Err { Write-Host "[ERROR] $args" -ForegroundColor Red; exit 1 }

# 显示帮助
function Show-Help {
    Write-Host "SBoard 面板 Windows 安装脚本"
    Write-Host ""
    Write-Host "用法:"
    Write-Host ""
    Write-Host "  方式1: 通过管道直接安装 (推荐)"
    Write-Host "    irm <url> | iex"
    Write-Host ""
    Write-Host "  方式2: 设置环境变量后安装"
    Write-Host '    $env:PORT="9000"; $env:USER="admin"; $env:PASS="mypassword"; irm <url> | iex'
    Write-Host ""
    Write-Host "  方式3: 下载脚本后带参数运行"
    Write-Host "    .\install-sboard.ps1 -Port 9000 -User admin -Pass mypassword"
    Write-Host ""
    Write-Host "参数:"
    Write-Host "  -InstallDir <path>  安装路径 (默认: C:\sboard)"
    Write-Host "  -Port <port>        监听端口 (默认: 8080)"
    Write-Host "  -User <user>        管理员用户名 (默认: admin)"
    Write-Host "  -Pass <pass>        管理员密码 (默认: admin123)"
    Write-Host "  -Update             更新 SBoard"
    Write-Host "  -Uninstall          卸载 SBoard"
    Write-Host "  -NoInteractive      非交互模式"
    Write-Host "  -Help               显示帮助"
    Write-Host ""
    Write-Host "环境变量:"
    Write-Host "  PORT                监听端口"
    Write-Host "  USER                管理员用户名"
    Write-Host "  PASS                管理员密码"
    Write-Host ""
    Write-Host "示例:"
    Write-Host "  .\install-sboard.ps1"
    Write-Host "  .\install-sboard.ps1 -Port 9000 -User admin -Pass admin123"
    Write-Host "  .\install-sboard.ps1 -Update"
    Write-Host "  .\install-sboard.ps1 -Uninstall"
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

# 检测架构
function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" { return "386" }
        "X86" { return "386" }
        default { Write-Err "不支持的架构: $arch，支持: amd64, arm64, 386" }
    }
}

# 生成随机字符串
function New-RandomString {
    param([int]$Length = 32)
    $chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    $result = ""
    for ($i = 0; $i -lt $Length; $i++) {
        $result += $chars[(Get-Random -Maximum $chars.Length)]
    }
    return $result
}

# 停止服务
function Stop-SboardService {
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
function Uninstall-Sboard {
    Write-Info "开始卸载 SBoard..."
    
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
    
    # 询问是否删除数据
    $deleteData = $true
    if (-not $NoInteractive) {
        $response = Read-Host "是否删除数据目录? [y/N]"
        $deleteData = $response -match "^[Yy]"
    }
    
    # 删除安装目录
    if (Test-Path $InstallDir) {
        if ($deleteData) {
            Write-Info "删除安装目录..."
            Remove-Item -Path $InstallDir -Recurse -Force
        } else {
            # 只删除二进制文件
            $binaryPath = Join-Path $InstallDir $BINARY_NAME
            if (Test-Path $binaryPath) {
                Remove-Item $binaryPath -Force
            }
        }
    }
    
    Write-Success "SBoard 已卸载"
    exit 0
}

# 下载 SBoard
function Download-Sboard {
    param($Arch)
    
    Write-Info "下载 SBoard..."
    
    # 创建安装目录
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # 创建数据目录
    $dataDir = Join-Path $InstallDir "data"
    if (-not (Test-Path $dataDir)) {
        New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
    }
    
    # 构建下载 URL
    $downloadFile = "sboard_windows_${Arch}.zip"
    $downloadUrl = "${GH_PROXY}https://github.com/$GITHUB_REPO/releases/latest/download/$downloadFile"
    $tempZip = Join-Path $env:TEMP $downloadFile
    
    Write-Info "下载: $downloadUrl"
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempZip -UseBasicParsing -TimeoutSec 120
        Write-Info "下载成功"
    } catch {
        Write-Err "下载失败: $_"
    }
    
    try {
        # 解压到安装目录
        Expand-Archive -Path $tempZip -DestinationPath $InstallDir -Force
        
        # 清理
        Remove-Item $tempZip -Force
        
        Write-Success "SBoard 下载完成"
    } catch {
        Write-Err "解压失败: $_"
    }
}

# 生成配置文件
function New-Config {
    $configPath = Join-Path $InstallDir "data" $CONFIG_FILE
    
    # 如果配置已存在，跳过
    if ((Test-Path $configPath) -and -not $Update) {
        Write-Warn "配置文件已存在，跳过创建"
        return
    }
    
    Write-Info "生成配置文件..."
    
    # 生成随机 JWT 密钥
    $jwtSecret = New-RandomString -Length 32
    
    $dataDir = Join-Path $InstallDir "data"
    
    $config = @"
# SBoard 配置文件

server:
  listen: "0.0.0.0:$Port"
  debug: false

data:
  dir: "$($dataDir -replace '\\', '\\')"

security:
  jwt_secret: "$jwtSecret"
  jwt_expire_hour: 168
  session_name: "sboard_token"

# 初始管理员账号 (首次启动后会自动创建)
admin:
  username: "$User"
  password: "$Pass"
"@
    
    Set-Content -Path $configPath -Value $config -Encoding UTF8
    Write-Success "配置文件已生成: $configPath"
}

# 创建 Windows 服务
function New-WindowsService {
    Write-Info "创建 Windows 服务..."
    
    $binaryPath = Join-Path $InstallDir $BINARY_NAME
    $configPath = Join-Path $InstallDir "data" $CONFIG_FILE
    
    # 检查服务是否已存在
    $existingService = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Info "服务已存在，删除旧服务..."
        sc.exe delete $SERVICE_NAME | Out-Null
        Start-Sleep -Seconds 2
    }
    
    # 创建服务
    $binPathEscaped = "`"$binaryPath`" -c `"$configPath`""
    sc.exe create $SERVICE_NAME binPath= $binPathEscaped start= auto DisplayName= "SBoard Panel" | Out-Null
    
    # 设置服务描述
    sc.exe description $SERVICE_NAME "SBoard - Proxy Panel Management" | Out-Null
    
    # 设置失败后自动重启
    sc.exe failure $SERVICE_NAME reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
    
    Write-Success "Windows 服务已创建"
}

# 启动服务
function Start-SboardService {
    Write-Info "启动服务..."
    
    Start-Service -Name $SERVICE_NAME
    Start-Sleep -Seconds 3
    
    $service = Get-Service -Name $SERVICE_NAME
    if ($service.Status -eq "Running") {
        Write-Success "服务启动成功"
    } else {
        Write-Err "服务启动失败，请检查日志"
    }
}

# 显示状态
function Show-Status {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "SBoard 安装完成" -ForegroundColor Green
    Write-Host "=========================================="
    Write-Host ""
    Write-Host "安装目录: $InstallDir"
    Write-Host "数据目录: $(Join-Path $InstallDir 'data')"
    Write-Host "配置文件: $(Join-Path $InstallDir 'data' $CONFIG_FILE)"
    Write-Host "服务名称: $SERVICE_NAME"
    Write-Host ""
    Write-Host "访问地址: http://localhost:$Port"
    Write-Host "管理员用户: $User"
    Write-Host ""
    Write-Host "常用命令:"
    Write-Host "  查看状态: Get-Service $SERVICE_NAME"
    Write-Host "  查看日志: Get-Content $(Join-Path $InstallDir 'data' 'sboard.log') -Tail 50"
    Write-Host "  重启服务: Restart-Service $SERVICE_NAME"
    Write-Host "  停止服务: Stop-Service $SERVICE_NAME"
    Write-Host ""
    Write-Host "或使用 services.msc 管理服务"
    Write-Host ""
    Write-Host "更新命令:"
    Write-Host "  .\install-sboard.ps1 -Update"
    Write-Host ""
    Write-Host "卸载命令:"
    Write-Host "  .\install-sboard.ps1 -Uninstall"
    Write-Host ""
}

# 交互式配置
function Get-InteractiveConfig {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "       SBoard 安装配置"
    Write-Host "=========================================="
    Write-Host ""
    
    # 安装路径
    $input = Read-Host "安装路径 [$InstallDir]"
    if ($input) { $script:InstallDir = $input }
    
    # 监听端口
    $input = Read-Host "监听端口 [$Port]"
    if ($input) { $script:Port = [int]$input }
    
    # 管理员用户名
    $defaultUser = if ($User) { $User } else { "admin" }
    $input = Read-Host "管理员用户名 [$defaultUser]"
    if ($input) { $script:User = $input } else { $script:User = $defaultUser }
    
    # 管理员密码
    $defaultPass = if ($Pass) { $Pass } else { "admin123" }
    $input = Read-Host "管理员密码 [$defaultPass]"
    if ($input) { $script:Pass = $input } else { $script:Pass = $defaultPass }
    
    # 确认
    Write-Host ""
    Write-Host "请确认安装配置:" -ForegroundColor Yellow
    Write-Host "  安装路径: $InstallDir"
    Write-Host "  监听端口: $Port"
    Write-Host "  管理员: $User"
    Write-Host "  密码: ******"
    Write-Host ""
    $confirm = Read-Host "确认安装? [Y/n]"
    if ($confirm -match "^[Nn]") {
        Write-Host "安装已取消"
        exit 0
    }
}

# 主函数
function Main {
    Write-Host ""
    Write-Host "=========================================="
    Write-Host "      SBoard 面板 Windows 安装脚本"
    Write-Host "=========================================="
    Write-Host ""
    
    # 显示帮助
    if ($Help) {
        Show-Help
    }
    
    # 卸载
    if ($Uninstall) {
        Uninstall-Sboard
    }
    
    # 检查管理员权限
    if (-not (Test-Administrator)) {
        Write-Err "请以管理员身份运行此脚本"
    }
    
    # 从环境变量读取参数
    if (-not $Port -or $Port -eq 8080) {
        if ($env:PORT) {
            $script:Port = [int]$env:PORT
            Write-Info "从环境变量读取端口: $Port"
        }
    }
    if (-not $User) {
        if ($env:USER) {
            $script:User = $env:USER
            Write-Info "从环境变量读取用户名"
        }
    }
    if (-not $Pass) {
        if ($env:PASS) {
            $script:Pass = $env:PASS
            Write-Info "从环境变量读取密码"
        }
    }
    
    # 交互式配置
    if (-not $NoInteractive -and -not $Update) {
        Get-InteractiveConfig
    } else {
        # 非交互模式，使用默认值
        if (-not $User) { $script:User = "admin" }
        if (-not $Pass) { $script:Pass = "admin123" }
    }
    
    # 检测架构
    $arch = Get-Architecture
    Write-Info "检测到架构: $arch"
    
    # 停止现有服务
    Stop-SboardService
    
    # 下载 SBoard
    Download-Sboard -Arch $arch
    
    # 生成配置
    if (-not $Update) {
        New-Config
    }
    
    # 创建服务
    New-WindowsService
    
    # 启动服务
    Start-SboardService
    
    # 显示状态
    Show-Status
}

# 执行
Main
