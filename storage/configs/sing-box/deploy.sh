#!/bin/bash
# sing-box 部署脚本
# 此脚本会被自动生成并上传到目标服务器

INSTALL_DIR="/root/sing-box"
SERVICE_FILE="/etc/systemd/system/sing-box.service"

echo "开始部署 sing-box..."

# 创建安装目录
mkdir -p "$INSTALL_DIR"

# 复制文件
cp -f sing-box "$INSTALL_DIR/"
cp -f config.json "$INSTALL_DIR/"
cp -f server.crt "$INSTALL_DIR/"
cp -f server.key "$INSTALL_DIR/"

# 设置执行权限
chmod +x "$INSTALL_DIR/sing-box"

# 更新 systemd service 文件
cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=sing-box Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/sing-box
ExecStart=/root/sing-box/sing-box run -c /root/sing-box/config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 重新加载 systemd
systemctl daemon-reload

# 启用并重启服务
systemctl enable sing-box
systemctl restart sing-box

# 检查状态
sleep 2
systemctl status sing-box --no-pager

echo "sing-box 部署完成！"
