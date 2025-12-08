# SBoard Agent 卸载脚本 (Windows)
# 卸载 Agent 及其部署的所有核心服务 (sing-box, mihomo)
#
# 用法: 
#   .\uninstall-agent.ps1           # 交互式菜单
#   .\uninstall-agent.ps1 -All      # 卸载全部
#   .\uninstall-agent.ps1 -Agent    # 仅卸载 Agent
#   .\uninstall-agent.ps1 -Cores    # 仅卸载核心服务

param(
    [switch]$All,
    [switch]$Agent,
    [switch]$Cores,
    [switch]$Preview,
    [switch]$Help
)

# 配置
$AgentService = "sboard-agent"
$CoreServices = @("sing-box", "mihomo")
$AgentInstallDir = "C:\sboard-agent"
$CoreInstallDirs = @(
    "C:\sboard\sing-box",
    "C:\sboard\mihomo",
    "$env:USERPROFILE\sing-box",
    "$env:USERPROFILE\mihomo"
)

# 颜色输出
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Cyan }
function Write-Success { Write-Host "[SUCCESS] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARNING] $args" -ForegroundColor Yellow }
function Write-Err { Write-Host "[ERROR] $args" -ForegroundColor Red }

# 检查管理员权限
function Test-Administrator {
    $currentUser = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $currentUser.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# 以管理员权限重新运行
function Restart-AsAdmin {
    if (-not (Test-Administrator)) {
        Write-Warn "需要管理员权限，正在提升..."
        $scriptPath = $MyInvocation.ScriptName
        if ($scriptPath) {
            Start-Process powershell.exe -ArgumentList "-ExecutionPolicy Bypass -File `"$scriptPath`"" -Verb RunAs
        } else {
            Write-Err "无法获取脚本路径，请以管理员身份运行 PowerShell"
        }
        exit
    }
}

# 检查服务是否存在
function Test-ServiceExists {
    param([string]$ServiceName)
    $service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    return $null -ne $service
}

# 卸载 Windows 服务
function Uninstall-WindowsService {
    param([string]$ServiceName)
    
    if (Test-ServiceExists $ServiceName) {
        Write-Info "停止服务: $ServiceName"
        Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
        
        Write-Info "删除服务: $ServiceName"
        sc.exe delete $ServiceName | Out-Null
        
        # 等待服务删除
        Start-Sleep -Seconds 2
        
        if (-not (Test-ServiceExists $ServiceName)) {
            Write-Success "已卸载服务: $ServiceName"
        } else {
            Write-Warn "服务 $ServiceName 可能需要重启后才能完全删除"
        }
    }
}

# 删除目录
function Remove-InstallDir {
    param([string]$DirPath)
    
    if (Test-Path $DirPath) {
        Write-Info "删除目录: $DirPath"
        Remove-Item -Path $DirPath -Recurse -Force -ErrorAction SilentlyContinue
        if (-not (Test-Path $DirPath)) {
            Write-Success "已删除: $DirPath"
        } else {
            Write-Warn "无法完全删除: $DirPath"
        }
    }
}

# 显示将要卸载的内容
function Show-UninstallPreview {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "       将要卸载以下内容" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""
    
    Write-Host "服务:" -ForegroundColor Yellow
    $allServices = @($AgentService) + $CoreServices
    foreach ($svc in $allServices) {
        if (Test-ServiceExists $svc) {
            Write-Host "  - $svc " -NoNewline
            Write-Host "(已安装)" -ForegroundColor Green
        }
    }
    
    Write-Host ""
    Write-Host "目录:" -ForegroundColor Yellow
    if (Test-Path $AgentInstallDir) {
        Write-Host "  - $AgentInstallDir " -NoNewline
        Write-Host "(存在)" -ForegroundColor Green
    }
    foreach ($dir in $CoreInstallDirs) {
        if (Test-Path $dir) {
            Write-Host "  - $dir " -NoNewline
            Write-Host "(存在)" -ForegroundColor Green
        }
    }
    Write-Host ""
}

# 卸载 Agent
function Uninstall-Agent {
    Write-Info "卸载 SBoard Agent..."
    
    Uninstall-WindowsService $AgentService
    Remove-InstallDir $AgentInstallDir
    
    Write-Success "SBoard Agent 卸载完成"
}

# 卸载核心服务
function Uninstall-Cores {
    Write-Info "卸载核心服务..."
    
    foreach ($svc in $CoreServices) {
        Uninstall-WindowsService $svc
    }
    
    foreach ($dir in $CoreInstallDirs) {
        Remove-InstallDir $dir
    }
    
    Write-Success "核心服务卸载完成"
}

# 确认卸载
function Confirm-Uninstall {
    Write-Host ""
    Write-Host "警告: 此操作将删除 Agent 及其部署的所有核心服务!" -ForegroundColor Red
    Write-Host ""
    $confirm = Read-Host "确定要继续吗? [y/N]"
    return $confirm -eq 'y' -or $confirm -eq 'Y'
}

# 显示菜单
function Show-Menu {
    Clear-Host
    Write-Host ""
    Write-Host "  _   _       _           _        _ _ " -ForegroundColor Cyan
    Write-Host " | | | |_ __ (_)_ __  ___| |_ __ _| | |" -ForegroundColor Cyan
    Write-Host " | | | | '_ \| | '_ \/ __| __/ _`` | | |" -ForegroundColor Cyan
    Write-Host " | |_| | | | | | | | \__ \ || (_| | | |" -ForegroundColor Cyan
    Write-Host "  \___/|_| |_|_|_| |_|___/\__\__,_|_|_|" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "========== Agent 卸载工具 (Windows) ==========" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  1. 卸载 Agent 和所有核心服务" -ForegroundColor White
    Write-Host "  2. 仅卸载核心服务 (保留 Agent)" -ForegroundColor White
    Write-Host "  3. 仅卸载 Agent (保留核心服务)" -ForegroundColor White
    Write-Host "  4. 查看将要卸载的内容" -ForegroundColor White
    Write-Host "  0. 退出" -ForegroundColor Blue
    Write-Host ""
    Write-Host "==============================================" -ForegroundColor Cyan
    Write-Host ""
}

# 交互式菜单
function Start-InteractiveMenu {
    while ($true) {
        Show-Menu
        $choice = Read-Host "请选择操作 [0-4]"
        Write-Host ""
        
        switch ($choice) {
            "1" {
                Show-UninstallPreview
                if (Confirm-Uninstall) {
                    Uninstall-Agent
                    Uninstall-Cores
                    Write-Host ""
                    Write-Success "全部卸载完成!"
                }
                Read-Host "按回车键继续..."
            }
            "2" {
                Write-Warn "将卸载核心服务 (sing-box, mihomo)"
                $confirm = Read-Host "确定吗? [y/N]"
                if ($confirm -eq 'y' -or $confirm -eq 'Y') {
                    Uninstall-Cores
                    Write-Success "核心服务卸载完成!"
                }
                Read-Host "按回车键继续..."
            }
            "3" {
                Write-Warn "将卸载 Agent (保留核心服务)"
                $confirm = Read-Host "确定吗? [y/N]"
                if ($confirm -eq 'y' -or $confirm -eq 'Y') {
                    Uninstall-Agent
                    Write-Success "Agent 卸载完成!"
                }
                Read-Host "按回车键继续..."
            }
            "4" {
                Show-UninstallPreview
                Read-Host "按回车键继续..."
            }
            "0" {
                Write-Host "再见!" -ForegroundColor Green
                exit 0
            }
            default {
                Write-Err "无效选择"
                Start-Sleep -Seconds 1
            }
        }
    }
}

# 显示帮助
function Show-Help {
    Write-Host "SBoard Agent 卸载脚本 (Windows)"
    Write-Host ""
    Write-Host "用法:"
    Write-Host "  .\uninstall-agent.ps1 [选项]"
    Write-Host ""
    Write-Host "选项:"
    Write-Host "  (无参数)    交互式菜单"
    Write-Host "  -All        卸载 Agent 和所有核心服务"
    Write-Host "  -Agent      仅卸载 Agent"
    Write-Host "  -Cores      仅卸载核心服务"
    Write-Host "  -Preview    预览将要卸载的内容"
    Write-Host "  -Help       显示帮助"
    Write-Host ""
}

# 主函数
function Main {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host "   SBoard Agent 卸载工具 (Windows)" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
    Write-Host ""
    
    # 检查管理员权限
    Restart-AsAdmin
    
    if ($Help) {
        Show-Help
        return
    }
    
    if ($Preview) {
        Show-UninstallPreview
        return
    }
    
    if ($All) {
        Show-UninstallPreview
        if (Confirm-Uninstall) {
            Uninstall-Agent
            Uninstall-Cores
            Write-Host ""
            Write-Success "全部卸载完成!"
        }
        return
    }
    
    if ($Agent) {
        Uninstall-Agent
        return
    }
    
    if ($Cores) {
        Uninstall-Cores
        return
    }
    
    # 无参数，显示交互式菜单
    Start-InteractiveMenu
}

Main
