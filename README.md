# SBoard Go

一个用 Go 重写的代理服务管理面板，支持 sing-box 和 mihomo 核心。

## 特性

- 🚀 **单二进制部署** - 无需 PHP/Apache/Nginx，一个二进制文件即可运行
- 🔐 **JWT 认证** - 安全的 Token 认证机制
- 📦 **内嵌前端** - 前端资源编译到二进制中，无需额外配置
- 🌐 **多协议支持** - VMess, VLESS, Trojan, Shadowsocks, Hysteria2
- 🔒 **Reality 支持** - 内置 Reality 密钥生成
- 📡 **SSH 部署** - 通过 SSH 一键部署配置到服务器
- 📋 **订阅链接** - 支持 Mihomo/Clash, sing-box, V2Ray 订阅格式
- 🛠️ **简单管理** - systemd 服务管理

## 快速安装

```bash
# 一键安装
curl -fsSL https://raw.githubusercontent.com/amuae/sboard/main/scripts/install-sboard.sh | bash

# 或手动运行
bash scripts/install-sboard.sh install
```

## 手动构建

### 前置要求

- Go 1.21+
- Node.js 18+ (用于构建前端)

### 构建步骤

```bash
# 克隆代码
git clone https://github.com/amuae/sboard.git
cd sboard

# 构建前端 (可选，如果有前端代码)
# cd web && npm install && npm run build && cd ..

# 构建后端
go build -o sboard ./cmd/sboard

# 运行
./sboard -c config.yaml
```

## 配置文件

默认配置文件路径: `/etc/sboard/config.yaml`

```yaml
server:
  listen: "0.0.0.0:8080"
  debug: false

data:
  database: "/var/lib/sboard/sboard.db"

security:
  jwt_secret: "your-secret-key"
  jwt_expire_hour: 168

ssh:
  timeout: 30

core:
  sing_box_path: "/etc/sing-box"
  mihomo_path: "/etc/mihomo"
```

## 命令行参数

```bash
sboard [选项]

选项:
  -c, --config    配置文件路径 (默认: config.yaml)
  -l, --listen    监听地址 (覆盖配置文件)
  -d, --debug     调试模式
  -v, --version   显示版本
```

## API 接口

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/login | 登录 |
| POST | /api/auth/logout | 登出 |
| POST | /api/auth/password | 修改密码 |
| POST | /api/auth/username | 修改用户名 |
| GET | /api/auth/me | 获取当前用户 |

### 用户管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/users | 获取用户列表 |
| POST | /api/users | 创建用户 |
| GET | /api/users/:id | 获取用户详情 |
| PUT | /api/users/:id | 更新用户 |
| DELETE | /api/users/:id | 删除用户 |
| POST | /api/users/:id/reset-uuid | 重置用户 UUID |
| POST | /api/users/batch-delete | 批量删除 |
| POST | /api/users/batch-enable | 批量启用/禁用 |

### 节点管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/nodes | 获取节点列表 |
| POST | /api/nodes | 创建节点 |
| GET | /api/nodes/:id | 获取节点详情 |
| PUT | /api/nodes/:id | 更新节点 |
| DELETE | /api/nodes/:id | 删除节点 |
| POST | /api/nodes/batch-delete | 批量删除 |
| GET | /api/nodes/generate-reality-keys | 生成 Reality 密钥 |

### 服务器管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/servers | 获取服务器列表 |
| POST | /api/servers | 创建服务器 |
| GET | /api/servers/:id | 获取服务器详情 |
| PUT | /api/servers/:id | 更新服务器 |
| DELETE | /api/servers/:id | 删除服务器 |
| POST | /api/servers/:id/nodes | 设置服务器节点 |
| POST | /api/servers/:id/deploy | 部署服务器 (SSE) |
| GET | /api/servers/:id/test | 测试连接 |
| GET | /api/servers/:id/status | 获取状态 |

### 订阅链接

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /sublink | 获取订阅 |

订阅参数:
- `uuid`: 用户 UUID (必需)
- `format`: 格式 (mihomo/singbox/v2ray，默认 mihomo)
- `server`: 服务器 ID (可选，不传返回所有)

## 管理服务

```bash
# 启动
sudo systemctl start sboard

# 停止
sudo systemctl stop sboard

# 重启
sudo systemctl restart sboard

# 查看状态
sudo systemctl status sboard

# 查看日志
sudo journalctl -u sboard -f
```

## 目录结构

```
sboard/
├── cmd/
│   └── sboard/
│       └── main.go          # 入口文件
├── internal/
│   ├── config/
│   │   └── config.go        # 配置管理
│   ├── database/
│   │   ├── database.go      # 数据库连接
│   │   └── models.go        # 数据模型
│   ├── handler/
│   │   ├── server.go        # HTTP 服务器
│   │   ├── auth.go          # 认证处理
│   │   ├── user.go          # 用户管理
│   │   ├── node.go          # 节点管理
│   │   ├── server_crud.go   # 服务器管理
│   │   └── sublink.go       # 订阅生成
│   └── middleware/
│       └── auth.go          # JWT 中间件
├── web/
│   └── dist/                # 前端构建产物
├── scripts/
│   └── install-sboard.sh    # 安装脚本
├── go.mod
├── go.sum
└── README.md
```

## 从 PHP 版本迁移

数据库结构保持兼容，可以直接复制 `sboard.db` 到新目录使用。

```bash
# 备份旧数据库
cp /path/to/old/sboard.db /var/lib/sboard/sboard.db

# 重启服务
sudo systemctl restart sboard
```

## 许可证

MIT License

## 致谢

- [Gin](https://github.com/gin-gonic/gin) - HTTP 框架
- [GORM](https://gorm.io/) - ORM 库
- [sing-box](https://github.com/SagerNet/sing-box) - 代理核心
- [mihomo](https://github.com/MetaCubeX/mihomo) - Clash 内核
