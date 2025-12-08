# SBoard

轻量级代理服务管理面板，支持 sing-box 和 mihomo 核心。

## 特性

- 🚀 单二进制部署，无需依赖
- 🌐 多协议支持 (VMess, VLESS, Trojan, Shadowsocks, Hysteria2)
- 📋 订阅链接生成 (Mihomo, sing-box, V2Ray)
- 📡 Agent 远程部署与管理
- 🖥️ 服务器实时监控
- 🔐 GitHub OAuth 登录 (可选)

## 支持平台

| 组件 | Linux | Windows | macOS |
|------|-------|---------|-------|
| 面板 (sboard) | amd64, arm64, armv7 | - | amd64, arm64 |
| Agent | amd64, arm64, armv7 | amd64, arm64 | amd64, arm64 |

## 快速安装

### 面板一键安装

```bash
# 直接安装
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash

# 国内加速
curl -fsSL https://ghproxy.cn/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash
```

### Agent 一键卸载

**Linux / macOS:**

```bash
# 直接卸载
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent-auto.sh | bash

# 国内加速
curl -fsSL https://ghproxy.cn/https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent-auto.sh | bash
```

**Windows (PowerShell 管理员):**

```powershell
# 直接卸载
irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex

# 国内加速
irm https://ghproxy.cn/https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex
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
