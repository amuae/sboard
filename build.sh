#!/bin/bash

# Sboard 构建脚本
# 使用 Go 编写的构建工具完成下载核心和编译
#
# 用法:
#   ./build.sh                      # 默认构建当前平台
#   ./build.sh linux amd64          # 构建 Linux amd64
#   ./build.sh linux arm64          # 构建 Linux arm64  
#   ./build.sh windows amd64        # 构建 Windows amd64
#   ./build.sh darwin arm64         # 构建 macOS arm64
#   ./build.sh --skip-frontend ...  # 跳过前端构建
#   ./build.sh --skip-download ...  # 跳过核心下载
#   ./build.sh --only-download ...  # 只下载核心

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 解析参数
SKIP_FRONTEND=""
SKIP_DOWNLOAD=""
ONLY_DOWNLOAD=""
VERBOSE=""
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-frontend)
            SKIP_FRONTEND="-skip-frontend"
            shift
            ;;
        --skip-download)
            SKIP_DOWNLOAD="-skip-download"
            shift
            ;;
        --only-download)
            ONLY_DOWNLOAD="-only-download"
            shift
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        *)
            POSITIONAL_ARGS+=("$1")
            shift
            ;;
    esac
done

# 目标平台
TARGET_OS="${POSITIONAL_ARGS[0]:-}"
TARGET_ARCH="${POSITIONAL_ARGS[1]:-}"

# 构建参数
BUILD_ARGS=""
if [ -n "$TARGET_OS" ]; then
    BUILD_ARGS="$BUILD_ARGS -os=$TARGET_OS"
fi
if [ -n "$TARGET_ARCH" ]; then
    BUILD_ARGS="$BUILD_ARGS -arch=$TARGET_ARCH"
fi
if [ -n "$SKIP_FRONTEND" ]; then
    BUILD_ARGS="$BUILD_ARGS $SKIP_FRONTEND"
fi
if [ -n "$SKIP_DOWNLOAD" ]; then
    BUILD_ARGS="$BUILD_ARGS $SKIP_DOWNLOAD"
fi
if [ -n "$ONLY_DOWNLOAD" ]; then
    BUILD_ARGS="$BUILD_ARGS $ONLY_DOWNLOAD"
fi
if [ -n "$VERBOSE" ]; then
    BUILD_ARGS="$BUILD_ARGS $VERBOSE"
fi

# 运行 Go 构建工具
echo "运行构建工具..."
go run ./cmd/build $BUILD_ARGS
