#!/bin/bash

# SBoard 面板安装入口
# 自动检测平台并下载执行对应的安装脚本
#
# 用法:
#   curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install.sh | bash

set -e

# GitHub 配置
GH_PROXY="https://ghfast.top/"
SCRIPT_URL="${GH_PROXY}https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh"

# 检测操作系统
case "$(uname -s 2>/dev/null)" in
    Linux|linux|Darwin|darwin)
        # Linux/macOS: 下载脚本到临时文件后执行（保持交互能力）
        TMP_SCRIPT=$(mktemp)
        curl -fsSL "$SCRIPT_URL" -o "$TMP_SCRIPT"
        chmod +x "$TMP_SCRIPT"
        bash "$TMP_SCRIPT"
        rm -f "$TMP_SCRIPT"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        echo "检测到 Windows 环境，请使用 PowerShell 安装:"
        echo ""
        echo "  irm ${GH_PROXY}https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex"
        echo ""
        ;;
    *)
        echo "不支持的操作系统: $(uname -s)"
        exit 1
        ;;
esac
