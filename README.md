# SBoard

轻量级代理服务管理面板，基于 sing-box 核心。

## 特性

- 🚀 单二进制部署，无需依赖
- 🌐 多协议支持 (VMess, VLESS, Trojan, Anytls, Shadowsocks, Hysteria2)
- 📋 订阅链接生成 (支持 Mihomo/Clash, sing-box, V2Ray 格式)
- 📡 Agent 远程部署与管理
- 🖥️ 服务器实时监控
- 🔐 GitHub OAuth 登录 (可选)

## 支持平台

| 组件 | Linux | Windows | macOS |
|------|-------|---------|-------|
| 面板 (sboard) | amd64, 386, arm64, armv7, armv6 | amd64, arm64, 386 | amd64, arm64 |
| Agent | amd64, 386, arm64, armv7, armv6 | amd64, arm64, 386 | amd64, arm64 |

## 快速安装

### 面板一键安装

**Linux / macOS:**

直接安装：
```bash
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install.sh | bash
```

国内加速：
```bash
curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install.sh | bash
```

**Windows (PowerShell 管理员):**

直接安装：
```powershell
irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex
```

国内加速：
```powershell
irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.ps1 | iex
```

### Agent 安装

Agent 安装命令由面板自动生成，包含节点 Token。请在面板中添加节点后复制安装命令。

### Agent 一键卸载

**Linux / macOS:**

直接卸载：
```bash
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent-auto.sh | bash
```

国内加速：
```bash
curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent-auto.sh | bash
```

**Windows (PowerShell 管理员):**

直接卸载：
```powershell
irm https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex
```

国内加速：
```powershell
irm https://ghfast.top/https://raw.githubusercontent.com/amuae/sboard/main/scripts/uninstall-agent.ps1 | iex
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
