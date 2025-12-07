#!/bin/bash

# 完整构建脚本
# 构建前端 + 复制到嵌入目录 + 编译后端

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=== 构建前端 ==="
cd web
npm run build
if [ $? -ne 0 ]; then
    echo "前端构建失败"
    exit 1
fi
cd ..

echo "=== 复制前端到嵌入目录 ==="
rm -rf cmd/sboard/web/*
cp -r web/dist/* cmd/sboard/web/

# 编译优化参数：-s 去掉符号表，-w 去掉调试信息
LDFLAGS="-s -w"

echo "=== 编译后端 ==="
CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o sboard ./cmd/sboard
if [ $? -ne 0 ]; then
    echo "后端编译失败"
    exit 1
fi

echo "=== 编译 Agent ==="
CGO_ENABLED=0 go build -ldflags="$LDFLAGS" -o agent ./cmd/agent
if [ $? -ne 0 ]; then
    echo "Agent 编译失败"
    exit 1
fi

# 复制 agent 到 sboard-agent（供下载使用）
cp agent sboard-agent

echo ""
echo "=== 构建完成 ==="
echo "生成文件: sboard, agent, sboard-agent"
echo "运行 ./restart-test.sh 启动测试服务"
