# SBoard

轻量级代理服务管理面板，支持 sing-box 和 mihomo 核心。

## 特性

- 🚀 单二进制部署，无需依赖
- 🌐 多协议支持 (VMess, VLESS, Trojan, Shadowsocks, Hysteria2)
- 📋 订阅链接生成 (Mihomo, sing-box, V2Ray)
- 📡 SSH 远程部署
- 🖥️ 服务器监控 (Agent)

## 快速安装

### 面板

```bash
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash
```

### Agent (监控端)

```bash
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-agent.sh | bash -s -- --panel https://your-panel.com --token YOUR_TOKEN
```

## 服务管理

```bash
# 面板
systemctl start|stop|restart|status sboard

# Agent
systemctl start|stop|restart|status sboard-agent
```

## 许可证

MIT License
