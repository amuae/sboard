#!/bin/bash
# mihomo 部署脚本
# 此脚本会被自动生成并上传到目标服务器

INSTALL_DIR="/root/mihomo"
SERVICE_FILE="/etc/systemd/system/mihomo.service"

echo "开始部署 mihomo..."

# 创建安装目录
mkdir -p "$INSTALL_DIR"

# 复制文件
cp -f mihomo "$INSTALL_DIR/"
cp -f config.yaml "$INSTALL_DIR/"
cp -f server.crt "$INSTALL_DIR/"
cp -f server.key "$INSTALL_DIR/"

# 设置执行权限
chmod +x "$INSTALL_DIR/mihomo"

# 更新 systemd service 文件
cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=mihomo Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/mihomo
ExecStart=/root/mihomo/mihomo -d /root/mihomo -f /root/mihomo/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 重新加载 systemd
systemctl daemon-reload

# 启用并重启服务
systemctl enable mihomo
systemctl restart mihomo

# 检查状态
sleep 2
systemctl status mihomo --no-pager

echo "mihomo 部署完成！"
