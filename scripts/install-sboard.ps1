# SBoard 面板 Windows 安装脚本
# 支持: Windows (amd64, arm64, 386)
# 
# 用法 (以管理员身份运行 PowerShell):
#
#   方式1: 直接运行脚本进入交互式菜单
#     .\install-sboard.ps1
#
#   方式2: 通过管道安装
#     irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex
#
#   方式3: 带参数运行
#     .\install-sboard.ps1 -Install -Port 9000 -User admin -Pass mypassword

param(
    [Parameter(Mandatory=$false)]
    [switch]$Install,
    
    [Parameter(Mandatory=$false)]
    [switch]$Update,
    
    [Parameter(Mandatory=$false)]
    [switch]$Uninstall,
    
    [Parameter(Mandatory=$false)]
    [switch]$Menu,
    
    [Parameter(Mandatory=$false)]
    [string]$InstallDir = "$env:LOCALAPPDATA\sboard",
    
    [Parameter(Mandatory=$false)]
    [int]$Port = 8080,
    
    [Parameter(Mandatory=$false)]
    [string]$User,
    
    [Parameter(Mandatory=$false)]
    [string]$Pass,
    
    [Parameter(Mandatory=$false)]
    [string]$Domain,
    
    [Parameter(Mandatory=$false)]
    [switch]$Dev,
    
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
$DEV_DOMAIN_HASH = "9de17c968ada26abec13fc5fc264ddfa"
$script:DEV_MODE = $false

# GitHub 加速配置 (国内加速)
$GH_PROXY = "https://ghfast.top/"

# 颜色输出函数
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[SUCCESS] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARNING] $args" -ForegroundColor Yellow }
function Write-Err { 
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
    exit 1 
}

# 显示帮助
function Show-Help {
    Write-Host "SBoard 面板 Windows 安装脚本" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "用法:"
    Write-Host ""
    Write-Host "  方式1: 直接运行进入交互式菜单"
    Write-Host "    .\install-sboard.ps1"
    Write-Host ""
    Write-Host "  方式2: 通过管道安装"
    Write-Host '    irm <url> | iex'
    Write-Host ""
    Write-Host "  方式3: 带参数运行"
    Write-Host "    .\install-sboard.ps1 -Install -Port 9000 -User admin -Pass mypassword"
    Write-Host ""
    Write-Host "命令:"
    Write-Host "  (无参数)       进入交互式菜单"
    Write-Host "  -Install       安装 SBoard"
    Write-Host "  -Update        更新 SBoard"
    Write-Host "  -Uninstall     卸载 SBoard"
    Write-Host "  -Menu          进入交互式菜单"
    Write-Host "  -Help          显示帮助"
    Write-Host ""
    Write-Host "参数:"
    Write-Host "  -InstallDir <path>  安装路径 (默认: %LOCALAPPDATA%\sboard)"
    Write-Host "  -Domain <domain>    面板入口域名"
    Write-Host "  -Port <port>        监听端口 (默认: 8080)"
    Write-Host "  -User <user>        管理员用户名 (默认: admin)"
    Write-Host "  -Pass <pass>        管理员密码 (默认: admin123)"
    Write-Host "  -Dev                强制使用预发布版本"
    Write-Host "  -NoInteractive      非交互模式"
    Write-Host ""
    Write-Host "环境变量:"
    Write-Host "  PORT                监听端口"
    Write-Host "  USER                管理员用户名"
    Write-Host "  PASS                管理员密码"
    Write-Host ""
    Write-Host "示例:"
    Write-Host "  .\install-sboard.ps1                                    # 交互式菜单"
    Write-Host "  .\install-sboard.ps1 -Install                           # 交互式安装"
    Write-Host "  .\install-sboard.ps1 -Install -NoInteractive            # 使用默认值安装"
    Write-Host "  .\install-sboard.ps1 -Install -Port 9000 -User admin    # 指定参数安装"
    Write-Host "  .\install-sboard.ps1 -Update                            # 更新"
    Write-Host "  .\install-sboard.ps1 -Uninstall                         # 卸载"
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

# 检查域名是否为开发者域名
function Test-DevDomain {
    param([string]$DomainName)
    
    if ([string]::IsNullOrEmpty($DomainName)) {
        return
    }
    
    # 计算域名的 MD5
    $md5 = [System.Security.Cryptography.MD5]::Create()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($DomainName)
    $hash = $md5.ComputeHash($bytes)
    $domainHash = [BitConverter]::ToString($hash).Replace("-", "").ToLower()
    
    # 检查是否匹配开发者域名
    if ($domainHash -eq $DEV_DOMAIN_HASH) {
        $script:DEV_MODE = $true
    }
}

# 从配置文件读取域名
function Get-ConfigDomain {
    $configPath = Join-Path $InstallDir "data" $CONFIG_FILE
    if (Test-Path $configPath) {
        $content = Get-Content $configPath -Raw
        if ($content -match 'domain:\s*"?([^"\s]+)"?') {
            return $matches[1]
        }
    }
    return $null
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

# 检查是否已安装
function Test-Installed {
    $binaryPath = Join-Path $InstallDir $BINARY_NAME
    return (Test-Path $binaryPath)
}

# 获取服务状态
function Get-ServiceStatus {
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        return $service.Status
    }
    return "NotInstalled"
}

# 显示交互式菜单
function Show-Menu {
    while ($true) {
        Clear-Host
        Write-Host ""
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host "         SBoard 管理菜单" -ForegroundColor Cyan
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host ""
        
        # 显示当前状态
        $installed = Test-Installed
        $status = Get-ServiceStatus
        
        if ($installed) {
            Write-Host "  安装状态: " -NoNewline
            Write-Host "已安装" -ForegroundColor Green
            Write-Host "  安装路径: $InstallDir"
            Write-Host "  服务状态: " -NoNewline
            switch ($status) {
                "Running" { Write-Host "运行中" -ForegroundColor Green }
                "Stopped" { Write-Host "已停止" -ForegroundColor Yellow }
                default { Write-Host "$status" -ForegroundColor Gray }
            }
        } else {
            Write-Host "  安装状态: " -NoNewline
            Write-Host "未安装" -ForegroundColor Yellow
        }
        
        Write-Host ""
        Write-Host "  [1] 安装 SBoard" -ForegroundColor Green
        Write-Host "  [2] 更新 SBoard" -ForegroundColor Green
        Write-Host "  [3] 卸载 SBoard" -ForegroundColor Green
        Write-Host "  [4] 查看状态" -ForegroundColor Green
        Write-Host "  [5] 启动服务" -ForegroundColor Green
        Write-Host "  [6] 停止服务" -ForegroundColor Green
        Write-Host "  [7] 重启服务" -ForegroundColor Green
        Write-Host "  [8] 查看日志" -ForegroundColor Green
        Write-Host "  [0] 退出" -ForegroundColor Green
        Write-Host ""
        
        $choice = Read-Host "请选择操作 [0-8]"
        
        switch ($choice) {
            "1" {
                Install-Sboard
                Read-Host "按回车键继续..."
            }
            "2" {
                Update-Sboard
                Read-Host "按回车键继续..."
            }
            "3" {
                Uninstall-Sboard
                Read-Host "按回车键继续..."
            }
            "4" {
                Show-DetailedStatus
                Read-Host "按回车键继续..."
            }
            "5" {
                Start-SboardService
                Read-Host "按回车键继续..."
            }
            "6" {
                Stop-SboardService
                Read-Host "按回车键继续..."
            }
            "7" {
                Restart-SboardService
                Read-Host "按回车键继续..."
            }
            "8" {
                Show-Logs
                Read-Host "按回车键继续..."
            }
            "0" {
                Write-Host "再见!" -ForegroundColor Cyan
                exit 0
            }
            default {
                Write-Warn "无效选择，请重新选择"
                Start-Sleep -Seconds 1
            }
        }
    }
}

# 显示详细状态
function Show-DetailedStatus {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "         SBoard 状态" -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""
    
    $installed = Test-Installed
    if ($installed) {
        Write-Host "  安装状态: " -NoNewline
        Write-Host "已安装" -ForegroundColor Green
        Write-Host "  安装路径: $InstallDir"
        
        $binaryPath = Join-Path $InstallDir $BINARY_NAME
        $fileInfo = Get-Item $binaryPath -ErrorAction SilentlyContinue
        if ($fileInfo) {
            Write-Host "  文件大小: $([math]::Round($fileInfo.Length / 1MB, 2)) MB"
            Write-Host "  修改时间: $($fileInfo.LastWriteTime)"
        }
    } else {
        Write-Host "  安装状态: " -NoNewline
        Write-Host "未安装" -ForegroundColor Yellow
        return
    }
    
    # 服务状态
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        Write-Host "  服务状态: " -NoNewline
        switch ($service.Status) {
            "Running" { Write-Host "运行中" -ForegroundColor Green }
            "Stopped" { Write-Host "已停止" -ForegroundColor Yellow }
            default { Write-Host "$($service.Status)" -ForegroundColor Gray }
        }
        Write-Host "  启动类型: $($service.StartType)"
    }
    
    # 读取配置文件获取端口
    $configPath = Join-Path $InstallDir "data" $CONFIG_FILE
    if (Test-Path $configPath) {
        $content = Get-Content $configPath -Raw
        if ($content -match 'listen:\s*"[^:]+:(\d+)"') {
            Write-Host "  监听端口: $($Matches[1])"
        }
    }
    
    # 检查进程
    $process = Get-Process -Name "sboard" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Host "  进程 ID:  $($process.Id)"
        Write-Host "  内存使用: $([math]::Round($process.WorkingSet64 / 1MB, 2)) MB"
    }
    
    Write-Host ""
}

# 停止服务
function Stop-SboardService {
    Write-Info "停止 SBoard 服务..."
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -eq "Running") {
            Stop-Service -Name $SERVICE_NAME -Force
            Start-Sleep -Seconds 2
            Write-Success "服务已停止"
        } else {
            Write-Warn "服务未在运行"
        }
    } else {
        Write-Warn "服务不存在"
    }
}

# 启动服务
function Start-SboardService {
    Write-Info "启动 SBoard 服务..."
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -ne "Running") {
            Start-Service -Name $SERVICE_NAME
            Start-Sleep -Seconds 2
            $service = Get-Service -Name $SERVICE_NAME
            if ($service.Status -eq "Running") {
                Write-Success "服务已启动"
            } else {
                Write-Warn "服务启动失败，请检查日志"
            }
        } else {
            Write-Warn "服务已经在运行"
        }
    } else {
        Write-Warn "服务不存在，请先安装"
    }
}

# 重启服务
function Restart-SboardService {
    Write-Info "重启 SBoard 服务..."
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        Restart-Service -Name $SERVICE_NAME -Force
        Start-Sleep -Seconds 2
        $service = Get-Service -Name $SERVICE_NAME
        if ($service.Status -eq "Running") {
            Write-Success "服务已重启"
        } else {
            Write-Warn "服务重启失败，请检查日志"
        }
    } else {
        Write-Warn "服务不存在，请先安装"
    }
}

# 查看日志
function Show-Logs {
    Write-Host ""
    Write-Info "最近 50 行日志:"
    Write-Host ""
    
    $logPath = Join-Path $InstallDir "data" "sboard.log"
    if (Test-Path $logPath) {
        Get-Content $logPath -Tail 50
    } else {
        # 尝试从 Windows 事件日志获取
        try {
            Get-EventLog -LogName Application -Source $SERVICE_NAME -Newest 50 -ErrorAction SilentlyContinue | 
                Format-Table -Property TimeGenerated, Message -AutoSize
        } catch {
            Write-Warn "日志文件不存在: $logPath"
        }
    }
    Write-Host ""
}

# 创建 sboard 管理命令
function New-SboardCommand {
    Write-Info "创建 sboard 管理命令..."
    
    $sboardScript = @'
# SBoard 管理命令
# 用法: sboard [命令]
#   sboard          - 显示管理菜单
#   sboard start    - 启动服务
#   sboard stop     - 停止服务
#   sboard restart  - 重启服务
#   sboard status   - 查看状态
#   sboard logs     - 查看日志
#   sboard update   - 更新面板
#   sboard uninstall - 卸载面板

param(
    [Parameter(Position=0)]
    [string]$Command
)

$SERVICE_NAME = "sboard"

function Show-Menu {
    while ($true) {
        Write-Host ""
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host "        SBoard 面板管理" -ForegroundColor Cyan
        Write-Host "==========================================" -ForegroundColor Cyan
        Write-Host ""
        Write-Host "  1) 启动服务"
        Write-Host "  2) 停止服务"
        Write-Host "  3) 重启服务"
        Write-Host "  4) 查看状态"
        Write-Host "  5) 查看日志"
        Write-Host "  6) 更新面板"
        Write-Host "  7) 卸载面板"
        Write-Host "  0) 退出"
        Write-Host ""
        $choice = Read-Host "请选择 [0-7]"
        
        switch ($choice) {
            "1" { Start-SboardService }
            "2" { Stop-SboardService }
            "3" { Restart-SboardService }
            "4" { Get-SboardStatus }
            "5" { Get-SboardLogs }
            "6" { Update-Sboard; break }
            "7" { Uninstall-Sboard; break }
            "0" { Write-Host "再见!"; exit 0 }
            default { Write-Host "无效选择" -ForegroundColor Red }
        }
    }
}

function Start-SboardService {
    try {
        Start-Service -Name $SERVICE_NAME -ErrorAction Stop
        Write-Host "服务已启动" -ForegroundColor Green
    } catch {
        Write-Host "启动服务失败: $_" -ForegroundColor Red
    }
}

function Stop-SboardService {
    try {
        Stop-Service -Name $SERVICE_NAME -Force -ErrorAction Stop
        Write-Host "服务已停止" -ForegroundColor Green
    } catch {
        Write-Host "停止服务失败: $_" -ForegroundColor Red
    }
}

function Restart-SboardService {
    try {
        Restart-Service -Name $SERVICE_NAME -Force -ErrorAction Stop
        Write-Host "服务已重启" -ForegroundColor Green
    } catch {
        Write-Host "重启服务失败: $_" -ForegroundColor Red
    }
}

function Get-SboardStatus {
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        Write-Host ""
        Write-Host "服务名称: $($service.Name)"
        Write-Host "显示名称: $($service.DisplayName)"
        Write-Host "状态: $($service.Status)"
        Write-Host "启动类型: $($service.StartType)"
    } else {
        Write-Host "服务未安装" -ForegroundColor Yellow
    }
}

function Get-SboardLogs {
    $logPath = "$env:LOCALAPPDATA\sboard\data\sboard.log"
    if (Test-Path $logPath) {
        Get-Content $logPath -Tail 50 -Wait
    } else {
        Write-Host "日志文件不存在: $logPath" -ForegroundColor Yellow
        Write-Host "尝试查看 Windows 事件日志..."
        Get-EventLog -LogName Application -Source $SERVICE_NAME -Newest 20 -ErrorAction SilentlyContinue
    }
}

function Update-Sboard {
    Write-Host "正在更新 SBoard..." -ForegroundColor Cyan
    irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 -OutFile "$env:TEMP\install-sboard.ps1"
    & "$env:TEMP\install-sboard.ps1" -Update
}

function Uninstall-Sboard {
    Write-Host "正在卸载 SBoard..." -ForegroundColor Cyan
    irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 -OutFile "$env:TEMP\install-sboard.ps1"
    & "$env:TEMP\install-sboard.ps1" -Uninstall
}

# 主入口
switch ($Command) {
    "start" { Start-SboardService }
    "stop" { Stop-SboardService }
    "restart" { Restart-SboardService }
    "status" { Get-SboardStatus }
    "logs" { Get-SboardLogs }
    "update" { Update-Sboard }
    "uninstall" { Uninstall-Sboard }
    "help" {
        Write-Host "SBoard 管理命令"
        Write-Host ""
        Write-Host "用法: sboard [命令]"
        Write-Host ""
        Write-Host "命令:"
        Write-Host "  start      启动服务"
        Write-Host "  stop       停止服务"
        Write-Host "  restart    重启服务"
        Write-Host "  status     查看状态"
        Write-Host "  logs       查看日志"
        Write-Host "  update     更新面板"
        Write-Host "  uninstall  卸载面板"
        Write-Host "  (无参数)   显示管理菜单"
    }
    "" { Show-Menu }
    default {
        Write-Host "未知命令: $Command" -ForegroundColor Red
        Write-Host "使用 'sboard help' 查看帮助"
        exit 1
    }
}
'@

    # 创建 sboard.ps1 脚本
    $scriptPath = Join-Path $InstallDir "sboard.ps1"
    Set-Content -Path $scriptPath -Value $sboardScript -Encoding UTF8
    
    # 创建 sboard.cmd 批处理文件（方便在 cmd 中调用）
    $cmdPath = Join-Path $InstallDir "sboard.cmd"
    $cmdContent = "@echo off`r`npowershell -ExecutionPolicy Bypass -File `"$scriptPath`" %*"
    Set-Content -Path $cmdPath -Value $cmdContent -Encoding ASCII
    
    # 添加到系统 PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*$InstallDir*") {
        $newPath = "$currentPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Write-Info "已将 $InstallDir 添加到系统 PATH"
        Write-Warn "请重新打开终端以使 sboard 命令生效"
    }
    
    Write-Success "sboard 命令已创建"
}

# 删除 sboard 管理命令
function Remove-SboardCommand {
    # 删除脚本文件
    $scriptPath = Join-Path $InstallDir "sboard.ps1"
    $cmdPath = Join-Path $InstallDir "sboard.cmd"
    
    if (Test-Path $scriptPath) {
        Remove-Item $scriptPath -Force
    }
    if (Test-Path $cmdPath) {
        Remove-Item $cmdPath -Force
    }
    
    # 从系统 PATH 中移除
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -like "*$InstallDir*") {
        $newPath = ($currentPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Write-Info "已从系统 PATH 中移除 $InstallDir"
    }
}

# 卸载
function Uninstall-Sboard {
    Write-Info "开始卸载 SBoard..."
    
    # 删除 sboard 管理命令
    Remove-SboardCommand
    
    # 停止并删除服务
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -eq "Running") {
            Stop-Service -Name $SERVICE_NAME -Force
            Start-Sleep -Seconds 2
        }
        Write-Info "删除 Windows 服务..."
        sc.exe delete $SERVICE_NAME | Out-Null
        Start-Sleep -Seconds 1
    }
    
    # 询问是否删除数据
    $deleteData = $false
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
            Write-Info "已保留数据目录: $(Join-Path $InstallDir 'data')"
        }
    }
    
    Write-Success "SBoard 已卸载"
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
    if ($script:DEV_MODE) {
        Write-Warn "开发者模式：使用预发布版本"
        $downloadUrl = "${GH_PROXY}https://github.com/$GITHUB_REPO/releases/download/pre-release/$downloadFile"
    } else {
        $downloadUrl = "${GH_PROXY}https://github.com/$GITHUB_REPO/releases/latest/download/$downloadFile"
    }
    $tempZip = Join-Path $env:TEMP $downloadFile
    
    Write-Info "下载: $downloadUrl"
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
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
    if (Test-Path $configPath) {
        Write-Warn "配置文件已存在，跳过创建"
        return
    }
    
    Write-Info "生成配置文件..."
    
    # 生成随机 JWT 密钥
    $jwtSecret = New-RandomString -Length 32
    
    $dataDir = (Join-Path $InstallDir "data") -replace '\\', '\\'
    
    # 有域名时监听本地（配合 nginx 反代），无域名时监听所有接口
    $listenAddr = "0.0.0.0"
    if ($Domain -and $Domain -ne "localhost") {
        $listenAddr = "127.0.0.1"
    }
    
    $config = @"
# SBoard 配置文件

server:
  listen: "${listenAddr}:$Port"
  debug: false
  domain: "$Domain"

data:
  dir: "$dataDir"

security:
  jwt_secret: "$jwtSecret"
  jwt_expire_hour: 168
  session_name: "sboard_token"

oauth:
  disable_password_login: false
"@
    
    Set-Content -Path $configPath -Value $config -Encoding UTF8
    Write-Success "配置文件已生成: $configPath"
}

# 初始化管理员账户
function Initialize-Admin {
    Write-Info "初始化管理员账户..."
    
    $binaryPath = Join-Path $InstallDir $BINARY_NAME
    $dataDir = Join-Path $InstallDir "data"
    
    # 运行 sboard 初始化管理员
    try {
        $result = & $binaryPath -d $dataDir -init-admin -admin-user $User -admin-pass $Pass 2>&1
        Write-Success "管理员账户初始化完成"
    } catch {
        Write-Warn "管理员账户初始化失败: $_"
    }
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

# 显示安装完成信息
function Show-InstallStatus {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host "         SBoard 安装完成" -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "  安装目录: $InstallDir"
    Write-Host "  数据目录: $(Join-Path $InstallDir 'data')"
    Write-Host "  配置文件: $(Join-Path $InstallDir 'data' $CONFIG_FILE)"
    Write-Host "  服务名称: $SERVICE_NAME"
    Write-Host ""
    if ($Domain -and $Domain -ne "localhost") {
        Write-Host "  面板域名: " -NoNewline
        Write-Host "$Domain" -ForegroundColor Cyan
        Write-Host "  监听地址: " -NoNewline
        Write-Host "127.0.0.1:$Port" -ForegroundColor Cyan
        Write-Host "  (仅本地，需配置反向代理)" -ForegroundColor Yellow
        Write-Host "  访问地址: " -NoNewline
        Write-Host "https://$Domain" -ForegroundColor Cyan
    } else {
        Write-Host "  监听地址: " -NoNewline
        Write-Host "0.0.0.0:$Port" -ForegroundColor Cyan
        Write-Host "  访问地址: " -NoNewline
        Write-Host "http://localhost:$Port" -ForegroundColor Cyan
    }
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Yellow
    Write-Host "      管理员账户信息 (请牢记!)" -ForegroundColor Yellow
    Write-Host "==========================================" -ForegroundColor Yellow
    Write-Host "  用户名: " -NoNewline
    Write-Host "$User" -ForegroundColor Cyan
    Write-Host "  密码:   " -NoNewline
    Write-Host "$Pass" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  重要提示:" -ForegroundColor Red
    Write-Host "  - 管理员账户只能初始化一次，无法通过命令行修改"
    Write-Host "  - 登录后可在 Web 界面修改密码"
    Write-Host "  - 如忘记密码，需删除数据库文件后重新安装"
    Write-Host ""
    Write-Host "常用命令:" -ForegroundColor Yellow
    Write-Host "  查看状态: Get-Service $SERVICE_NAME"
    Write-Host "  重启服务: Restart-Service $SERVICE_NAME"
    Write-Host "  停止服务: Stop-Service $SERVICE_NAME"
    Write-Host "  启动服务: Start-Service $SERVICE_NAME"
    Write-Host ""
    
    if ($Domain -and $Domain -ne "localhost") {
        Write-Host "==========================================" -ForegroundColor Yellow
        Write-Host "      Nginx 反向代理配置示例" -ForegroundColor Yellow
        Write-Host "==========================================" -ForegroundColor Yellow
        Write-Host "server {"
        Write-Host "    listen 80;"
        Write-Host "    listen 443 ssl http2;"
        Write-Host "    server_name $Domain;"
        Write-Host ""
        Write-Host "    location / {"
        Write-Host "        proxy_pass http://127.0.0.1:$Port;"
        Write-Host "        proxy_set_header Host `$host;"
        Write-Host "        proxy_set_header X-Real-IP `$remote_addr;"
        Write-Host "        proxy_set_header X-Forwarded-For `$proxy_add_x_forwarded_for;"
        Write-Host "        proxy_set_header X-Forwarded-Proto `$scheme;"
        Write-Host "        proxy_http_version 1.1;"
        Write-Host "        proxy_set_header Upgrade `$http_upgrade;"
        Write-Host "        proxy_set_header Connection `"upgrade`";"
        Write-Host "    }"
        Write-Host "}"
        Write-Host ""
        Write-Host "提示: 前端会验证访问域名是否与配置的域名一致" -ForegroundColor Yellow
        Write-Host "      其他域名访问将显示警告" -ForegroundColor Yellow
    }
    
    Write-Host "或运行此脚本进入管理菜单:"
    Write-Host "  .\install-sboard.ps1"
    Write-Host ""
}

# 交互式配置
function Get-InteractiveConfig {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "        SBoard 面板安装向导" -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""
    
    # 步骤 1: 安装路径
    Write-Host "[1/5] 设置安装路径" -ForegroundColor Yellow
    $input = Read-Host "安装路径 [$InstallDir]"
    if ($input) { $script:InstallDir = $input }
    Write-Host ""
    
    # 步骤 2: 面板入口域名
    Write-Host "[2/5] 设置面板入口域名" -ForegroundColor Yellow
    Write-Host "  提示: 用于访问面板的域名，如 panel.example.com" -ForegroundColor Blue
    while ([string]::IsNullOrEmpty($Domain)) {
        $script:Domain = Read-Host "面板域名"
        if ([string]::IsNullOrEmpty($Domain)) {
            Write-Host "  域名不能为空，请重新输入" -ForegroundColor Red
        }
    }
    Test-DevDomain -DomainName $Domain
    Write-Host ""
    
    # 步骤 3: 监听端口
    Write-Host "[3/5] 设置监听端口" -ForegroundColor Yellow
    Write-Host "  提示: 直接回车将随机生成 5000-65535 之间的端口" -ForegroundColor Blue
    $input = Read-Host "监听端口 [随机]"
    if ($input) { 
        $script:Port = [int]$input 
    } else {
        $script:Port = Get-Random -Minimum 5000 -Maximum 65535
        Write-Info "随机生成端口: $Port"
    }
    Write-Host ""
    
    # 步骤 4: 管理员用户名
    Write-Host "[4/5] 设置管理员账户" -ForegroundColor Yellow
    while ([string]::IsNullOrEmpty($User)) {
        $script:User = Read-Host "管理员用户名"
        if ([string]::IsNullOrEmpty($User)) {
            Write-Host "  用户名不能为空，请重新输入" -ForegroundColor Red
        }
    }
    Write-Host ""
    
    # 步骤 5: 管理员密码
    Write-Host "[5/5] 设置管理员密码" -ForegroundColor Yellow
    while ($true) {
        $script:Pass = Read-Host "管理员密码"
        if ([string]::IsNullOrEmpty($Pass)) {
            Write-Host "  密码不能为空，请重新输入" -ForegroundColor Red
            continue
        }
        if ($Pass.Length -lt 6) {
            Write-Host "  密码长度至少 6 位，请重新输入" -ForegroundColor Red
            $script:Pass = $null
            continue
        }
        $confirmPass = Read-Host "确认密码"
        if ($Pass -ne $confirmPass) {
            Write-Host "  两次输入的密码不一致，请重新输入" -ForegroundColor Red
            $script:Pass = $null
            continue
        }
        break
    }
    Write-Host ""
    
    # 确认配置
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "          请确认安装配置" -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  安装路径: " -NoNewline; Write-Host "$InstallDir" -ForegroundColor Green
    Write-Host "  面板域名: " -NoNewline; Write-Host "$Domain" -ForegroundColor Green
    Write-Host "  监听端口: " -NoNewline; Write-Host "$Port" -ForegroundColor Green
    Write-Host "  管理员:   " -NoNewline; Write-Host "$User" -ForegroundColor Green
    Write-Host "  密码:     " -NoNewline; Write-Host "******" -ForegroundColor Green
    if ($script:DEV_MODE) {
        Write-Host "  版本:     " -NoNewline; Write-Host "预发布版本" -ForegroundColor Yellow
    }
    Write-Host ""
    $confirm = Read-Host "确认安装? [Y/n]"
    if ($confirm -match "^[Nn]") {
        Write-Host "安装已取消"
        exit 0
    }
}

# 安装 SBoard
function Install-Sboard {
    # 检查管理员权限
    if (-not (Test-Administrator)) {
        Write-Err "请以管理员身份运行此脚本"
    }
    
    # 处理 -Dev 参数
    if ($Dev) {
        $script:DEV_MODE = $true
    }
    
    # 处理 -Domain 参数
    if ($Domain) {
        Test-DevDomain -DomainName $Domain
    }
    
    # 从环境变量读取参数
    if ($env:PORT -and (-not $Port -or $Port -eq 8080)) {
        $script:Port = [int]$env:PORT
        Write-Info "从环境变量读取端口: $Port"
    }
    if ($env:USER -and -not $User) {
        $script:User = $env:USER
        Write-Info "从环境变量读取用户名"
    }
    if ($env:PASS -and -not $Pass) {
        $script:Pass = $env:PASS
        Write-Info "从环境变量读取密码"
    }
    
    # 交互式配置
    if (-not $NoInteractive) {
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
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq "Running") {
        Write-Info "停止现有服务..."
        Stop-Service -Name $SERVICE_NAME -Force
        Start-Sleep -Seconds 2
    }
    
    # 下载 SBoard
    Download-Sboard -Arch $arch
    
    # 生成配置
    New-Config
    
    # 初始化管理员账户
    Initialize-Admin
    
    # 创建服务
    New-WindowsService
    
    # 创建 sboard 管理命令
    New-SboardCommand
    
    # 启动服务
    Write-Info "启动服务..."
    Start-Service -Name $SERVICE_NAME
    Start-Sleep -Seconds 3
    
    $service = Get-Service -Name $SERVICE_NAME
    if ($service.Status -eq "Running") {
        Write-Success "服务启动成功"
    } else {
        Write-Warn "服务可能启动失败，请检查日志"
    }
    
    # 显示状态
    Show-InstallStatus
}

# 更新 SBoard
function Update-Sboard {
    # 检查管理员权限
    if (-not (Test-Administrator)) {
        Write-Err "请以管理员身份运行此脚本"
    }
    
    if (-not (Test-Installed)) {
        Write-Err "SBoard 未安装，请先安装"
    }
    
    Write-Info "开始更新 SBoard..."
    
    # 处理 -Dev 参数
    if ($Dev) {
        $script:DEV_MODE = $true
    }
    
    # 从配置文件读取域名，判断是否使用预发布版本
    if (-not $script:DEV_MODE) {
        $configDomain = Get-ConfigDomain
        if ($configDomain) {
            Write-Info "读取配置域名: $configDomain"
            Test-DevDomain -DomainName $configDomain
        }
    }
    
    # 检测架构
    $arch = Get-Architecture
    
    # 停止服务
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service -and $service.Status -eq "Running") {
        Write-Info "停止服务..."
        Stop-Service -Name $SERVICE_NAME -Force
        Start-Sleep -Seconds 2
    }
    
    # 下载新版本
    Download-Sboard -Arch $arch
    
    # 更新 sboard 管理命令
    New-SboardCommand
    
    # 启动服务
    Write-Info "启动服务..."
    Start-Service -Name $SERVICE_NAME
    Start-Sleep -Seconds 3
    
    $service = Get-Service -Name $SERVICE_NAME
    if ($service.Status -eq "Running") {
        Write-Success "SBoard 更新完成"
    } else {
        Write-Warn "服务可能启动失败，请检查日志"
    }
}

# 主函数
function Main {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "      SBoard 面板 Windows 安装脚本" -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""
    
    # 显示帮助
    if ($Help) {
        Show-Help
        return
    }
    
    # 检查管理员权限 (除了帮助命令)
    if (-not (Test-Administrator)) {
        Write-Host "[ERROR] 请以管理员身份运行此脚本" -ForegroundColor Red
        Write-Host ""
        Write-Host "方法: 右键点击 PowerShell -> 以管理员身份运行"
        Write-Host ""
        exit 1
    }
    
    # 根据参数执行操作
    if ($Uninstall) {
        Uninstall-Sboard
        return
    }
    
    if ($Update) {
        Update-Sboard
        return
    }
    
    if ($Install) {
        Install-Sboard
        return
    }
    
    if ($Menu) {
        Show-Menu
        return
    }
    
    # 没有指定命令时的逻辑
    # 检测是否是管道模式
    $isPipeline = $false
    try {
        if ([Console]::IsInputRedirected) {
            $isPipeline = $true
        }
    } catch {
        $isPipeline = $true
    }
    
    if ($isPipeline) {
        # 管道模式，直接安装
        Write-Info "检测到管道模式，开始安装..."
        Install-Sboard
    } else {
        # 直接运行，显示菜单
        Show-Menu
    }
}

# 执行
Main
