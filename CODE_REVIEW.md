# SBoard 项目代码审查报告

**审查日期**: 2026-06-22
**项目版本**: v3.0.0
**代码规模**: Go ~13,600 行 (36个文件), 前端 Vue3+TS (~12个源文件)
**技术栈**: Go 1.26.4 / Gin / GORM / SQLite / Vue3 / Vite

---

## 一、总体评估

SBoard 是一个代理服务器管理面板，整体架构清晰（cmd/internal/web 三层分离），但存在若干严重安全问题和代码质量问题。最突出的问题是：**零测试覆盖、硬编码默认凭据、JWT 密钥生成不安全、OAuth state 无并发保护**。

**项目优势**:
- 目录结构遵循 Go 标准布局
- 数据库使用 GORM 的参数化查询，基本避免了 SQL 注入
- 使用 bcrypt 存储密码
- 有优雅关闭机制
- 支持多种代理协议，功能较完整

---

## 二、必须修复 (Critical)

### 2.1 [安全] JWT 密钥生成函数是确定性的，不是随机的

**文件**: `internal/config/config.go:87-94`

```go
func generateRandomString(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[i%len(charset)]  // ← 确定性！i%62 总是相同的序列
    }
    return string(b)
}
```

**问题**: 该函数生成的字符串始终是 `"abcdefghijklmnopqrstuvwxyzABCDE..."` 的前 32 个字符，完全可预测。当配置文件不存在时，`Default()` 使用此函数生成 JWT Secret，所有未自定义配置的实例共享相同的 JWT 密钥。攻击者可以用已知密钥伪造任意管理员 token。

**修复建议**: 使用 `crypto/rand` 生成随机字符串：
```go
import "crypto/rand"

func generateRandomString(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    if _, err := rand.Read(b); err != nil {
        panic("无法生成随机字符串: " + err.Error())
    }
    for i := range b {
        b[i] = charset[int(b[i])%len(charset)]
    }
    return string(b)
}
```

---

### 2.2 [安全] 硬编码默认管理员凭据 (admin/admin123)

**文件**: `internal/handler/auth.go:43-60`

```go
// 当数据库中无管理员时，登录接口自动创建 admin/admin123
defaultAdmin.SetPassword("admin123")
database.DB.Create(defaultAdmin)
if req.Username == "admin" && req.Password == "admin123" {
    admin = *defaultAdmin
}
```

**文件**: `cmd/sboard/main.go:49`

```go
flag.StringVar(&initAdminPass, "admin-pass", "admin123", "管理员密码")
```

**问题**: 
1. 登录接口在没有管理员时自动创建 `admin/admin123` 账户，攻击者可在首次部署后立即登录
2. 命令行 `--admin-pass` 默认值也是 `admin123`
3. 没有任何机制强制用户在首次登录后修改密码

**修复建议**: 
- 移除登录时自动创建默认管理员的逻辑
- 首次部署必须通过命令行或环境变量显式设置管理员密码
- 强制首次登录后修改密码（设置 `must_change_password` 标志）

---

### 2.3 [安全] OAuth State 存储无并发保护，且无清理机制

**文件**: `internal/handler/oauth.go:35-36`

```go
var oauthStateStore = make(map[string]time.Time)
```

**问题**: 
1. `oauthStateStore` 是普通 map，在 HTTP handler 中读写无 mutex 保护，高并发下会导致 `concurrent map read and map write` panic
2. `cleanupExpiredStates()` 在 goroutine 中运行，与主写入存在竞态条件
3. state 永远不会从内存中删除（只有过期后被动清理），长期运行会造成内存泄漏
4. OAuth token 通过 URL 参数传递（第240行 `/?oauth_token=...`），可能泄露到浏览器历史和服务器日志

**修复建议**:
```go
var (
    oauthStateStore = make(map[string]time.Time)
    oauthStateMu    sync.RWMutex
)
```
- 使用 `sync.RWMutex` 保护并发访问
- 使用定时器定期清理过期 state
- OAuth token 改为通过 fragment（`#token=...`）或 POST 方式传递

---

### 2.4 [安全] CORS 允许所有来源

**文件**: `internal/middleware/auth.go:113`

```go
c.Header("Access-Control-Allow-Origin", "*")
```

**问题**: 允许任意来源的跨域请求。攻击者可以构造恶意页面，利用已登录用户的浏览器发起跨域请求读取管理面板数据。

**修复建议**: 在配置文件中添加 CORS 白名单设置，或至少限制为面板域名：
```go
if origin := c.GetHeader("Origin"); isAllowedOrigin(origin) {
    c.Header("Access-Control-Allow-Origin", origin)
    c.Header("Access-Control-Allow-Credentials", "true")
}
```

---

### 2.5 [安全] JWT Token 可从 URL 查询参数获取

**文件**: `internal/middleware/auth.go:79-82`

```go
if tokenString == "" {
    tokenString = c.Query("token")
}
```

**问题**: 允许通过 `?token=xxx` 传递 JWT，token 会出现在：
- 浏览器历史记录
- Web 服务器访问日志
- Referer 头
- 代理日志

**修复建议**: 移除查询参数方式，仅支持 Header 和 Cookie 传递 token。

---

### 2.6 [安全] Agent 远程命令执行无限制

**文件**: `internal/handler/agent_handler.go:497-550`

```go
type req struct {
    Command string   `json:"command"`
    Args    []string `json:"args"`
    Timeout int      `json:"timeout"`
}
```

**问题**: 管理员可以通过 API 向远程 Agent 发送任意 shell 命令执行。虽然需要认证，但这等同于一个远程代码执行后门。一旦管理员账号被盗，所有 Agent 服务器全部沦陷。

**修复建议**: 
- 实现命令白名单机制，仅允许预定义的操作（restart、status、update 等）
- 记录所有远程命令执行的审计日志
- 添加操作确认机制

---

## 三、建议修复 (High)

### 3.1 [质量] 零测试覆盖

**整个项目没有任何 `_test.go` 文件。**

**影响**: 
- 无法验证核心业务逻辑的正确性（配置生成、订阅链接、用户等级降级等）
- 重构时无安全网
- Bug 回归风险极高

**修复建议**: 优先为核心逻辑添加单元测试：
1. `generateMihomoSubscription` / `generateSingBoxSubscription` — 订阅生成
2. `filterServersByLevel` / `getNodeName` — 等级降级策略
3. `GenerateServerConfig` — 服务器配置生成
4. `ParseToken` / `GenerateToken` — JWT 认证
5. `isDomesticByIP` — GeoIP 判断

---

### 3.2 [性能] linkUserToNodes / linkAllUsersToNode 中存在 N+1 查询

**文件**: `internal/handler/user.go:172-195`, `internal/handler/node.go:298-321`

```go
func (s *Server) linkUserToNodes(user *database.ProxyUser) {
    var nodes []database.InboundNode
    database.DB.Where("enabled = ?", true).Find(&nodes)
    for _, node := range nodes {
        var count int64
        database.DB.Model(&database.NodeUserRelation{}).
            Where("node_id = ? AND user_id = ?", node.ID, user.ID).
            Count(&count)  // ← N 次查询
        if count > 0 { continue }
        database.DB.Create(&relation)  // ← N 次插入
    }
}
```

**问题**: 如果有 100 个节点，创建一个用户会执行 200+ 次数据库查询。

**修复建议**: 使用批量操作：
```go
// 先查询已存在的关联
var existingNodeIDs []uint
database.DB.Model(&database.NodeUserRelation{}).
    Where("user_id = ?", user.ID).
    Pluck("node_id", &existingNodeIDs)
existingSet := make(map[uint]bool)
for _, id := range existingNodeIDs { existingSet[id] = true }

// 批量创建
var newRelations []database.NodeUserRelation
for _, node := range nodes {
    if !existingSet[node.ID] {
        newRelations = append(newRelations, database.NodeUserRelation{...})
    }
}
if len(newRelations) > 0 {
    database.DB.CreateInBatches(newRelations, 100)
}
```

---

### 3.3 [性能] 订阅生成中的 N+1 查询

**文件**: `internal/handler/sublink.go:205-224`

```go
for _, server := range servers {
    var nodeConfigs []database.ServerNodeConfig
    database.DB.Where("server_id = ?", server.ID).Find(&nodeConfigs)  // ← 每个服务器一次查询
}
for _, server := range servers {
    var outbounds []database.ServerOutbound
    database.DB.Where("server_id = ? AND enabled = ?", server.ID, true).Find(&outbounds)  // ← 又 N 次
}
```

**修复建议**: 使用批量查询 + 内存分组：
```go
var allNodeConfigs []database.ServerNodeConfig
database.DB.Where("server_id IN ?", serverIDs).Find(&allNodeConfigs)
// 按 serverID 分组到 map
```

---

### 3.4 [安全] 登录接口无速率限制

**文件**: `internal/handler/auth.go:22-98`

**问题**: 登录接口没有任何速率限制或账户锁定机制，攻击者可以无限制暴力破解密码。

**修复建议**: 
- 实现 IP 级别的速率限制（如每分钟最多 5 次尝试）
- 实现账户锁定（连续失败 N 次后锁定 15 分钟）
- 记录失败登录日志

---

### 3.5 [安全] WebSocket Upgrader 不验证 Origin

**文件**: `internal/handler/agent_handler.go:17-21`

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true  // ← 允许所有来源
    },
}
```

**问题**: 任何网页都可以建立到 Agent WebSocket 端点的连接。虽然 Agent 有自己的 token 认证，但开放 WebSocket 增加了攻击面。

**修复建议**: 验证 Origin 头或使用 Agent Token 进行 WebSocket 认证（在 Upgrade 前验证）。

---

### 3.6 [设计] ProxyUser 模型硬编码 10 个 UUID 字段

**文件**: `internal/database/models.go:58-67`

```go
UUID1  string `gorm:"size:36" json:"uuid_1"`
UUID2  string `gorm:"size:36" json:"uuid_2"`
...
UUID10 string `gorm:"size:36" json:"uuid_10"`
```

**问题**: 
1. 违反数据库规范化原则，如果需要增加到 15 个 UUID，需要修改表结构
2. `GetExtraUUID` 使用 10-case switch，`GetAllExtraUUIDs` 使用循环
3. 数据库迁移脚本 (`migrateUserExtraUUIDs`) 为每个老用户执行 10 次 `DB.Save`

**修复建议**: 使用关联表：
```go
type UserExtraUUID struct {
    ID     uint   `gorm:"primaryKey"`
    UserID uint   `gorm:"index"`
    Slot   int    `gorm:"index"` // 1-N
    UUID   string `gorm:"size:36"`
}
```

---

### 3.7 [设计] sublink.go 文件过大 (2525 行, 70KB)

**文件**: `internal/handler/sublink.go`

**问题**: 单个文件包含：
- Mihomo/Clash 订阅生成 (~500 行)
- SingBox 订阅生成 (~800 行)  
- V2Ray 订阅生成 (~200 行)
- YAML 自定义序列化 (~100 行)
- 各种代理构建函数 (~900 行)

严重违反单一职责原则。

**修复建议**: 拆分为：
- `handler/sublink_mihomo.go`
- `handler/sublink_singbox.go`
- `handler/sublink_v2ray.go`
- `handler/sublink_common.go`

---

### 3.8 [设计] config_sync.go 实际为空文件

**文件**: `internal/handler/config_sync.go` (仅 1 行 `package handler`)

**问题**: 文件存在但无实质内容，可能是代码迁移残留。

**修复建议**: 删除或添加实际的配置同步逻辑。

---

### 3.9 [安全] 旧版 `/etc/passwd` 写入

**文件**: `cmd/agent/service_linux.go:176`

```go
os.WriteFile("/etc/passwd", []byte(strings.Join(lines, "\n")), 0644)
```

**问题**: Agent 程序直接写入 `/etc/passwd` 文件，这是一个极其危险的操作。权限设为 0644 可能破坏系统。

**修复建议**: 使用标准的 `useradd` 命令或 Go 的 `os/user` 包，避免直接修改 `/etc/passwd`。

---

## 四、可选优化 (Medium)

### 4.1 [质量] 全局数据库变量

**文件**: `internal/database/database.go:13`

```go
var DB *gorm.DB
```

**问题**: 全局可变状态，不利于测试和依赖注入。

**修复建议**: 使用依赖注入或 Repository 模式，将 `*gorm.DB` 通过构造函数传入 handler。

---

### 4.2 [质量] 错误处理不一致

**文件**: 多处 handler 文件

部分接口使用 `errorJSON()` 统一返回，部分直接使用 `c.JSON()`：
- `internal/handler/subscription_config.go:34`: `c.JSON(http.StatusInternalServerError, gin.H{"error": "..."})`
- `internal/handler/auth.go`: 使用 `errorJSON(c, ...)`

**修复建议**: 统一使用 `errorJSON()` 辅助函数。

---

### 4.3 [性能] generateMsgID 时间戳精度不足

**文件**: `internal/handler/agent_handler.go:704-706`

```go
func generateMsgID() string {
    return time.Now().Format("20060102150405.000000")
}
```

**问题**: 在高并发场景下，微秒级时间戳可能重复，导致 pendingReq 覆盖。

**修复建议**: 使用 UUID 或时间戳+随机数：
```go
func generateMsgID() string {
    return uuid.New().String()
}
```

---

### 4.4 [质量] init() 函数中的副作用

**文件**: `internal/handler/agent_handler.go:80-82`

```go
func init() {
    go agentHub.run()
}
```

**文件**: `internal/handler/auth.go:219-223`

```go
func init() {
    _ = bcrypt.DefaultCost
    _ = time.Now
}
```

**问题**: 
1. `agentHub.run()` 在 init 中启动 goroutine，如果 import 顺序不当可能导致问题
2. `auth.go` 的 init 是无意义的占位代码

**修复建议**: 将 `agentHub.run()` 移到 `NewServer()` 中启动，删除无意义的 init。

---

### 4.5 [质量] 数据库查询缺少错误检查

**文件**: 多处 handler

```go
// settings.go:51 - 忽略了 Find 的错误
database.DB.Find(&configs)
```

```go
// user.go:84 - 统计查询忽略错误
database.DB.Model(&database.ProxyUser{}).Count(&total)
```

**修复建议**: 检查所有数据库操作的错误返回值。

---

### 4.6 [性能] Scheduler 启动时执行两次检查

**文件**: `internal/scheduler/scheduler.go:36-37`

```go
go s.checkExpiredUsers()         // 立即执行
go s.checkMonthlyTrafficReset()  // 立即执行
go s.runPeriodically(10*time.Minute, s.checkExpiredUsers)  // 又会执行
```

**问题**: `checkExpiredUsers` 在 `Start()` 中被调用了两次（一次手动，一次由 ticker 首次触发）。`checkMonthlyTrafficReset` 虽有日期检查不会重复，但设计上不清晰。

**修复建议**: 仅保留 ticker 方式，去掉手动调用。

---

### 4.7 [安全] 数据库文件权限

**文件**: `cmd/sboard/main.go:64`

```go
if err := os.MkdirAll(dataDir, 0755); err != nil {
```

**问题**: 数据目录权限 0755 意味着所有用户可读。SQLite 数据库文件包含 JWT Secret 和用户凭据。

**修复建议**: 使用 `0700` 权限：
```go
if err := os.MkdirAll(dataDir, 0700); err != nil {
```

---

### 4.8 [质量] 未使用的 import 和死代码

**文件**: `internal/handler/auth.go:217-223`

```go
var _ io.ReadSeeker // 导入检查
func init() {
    _ = bcrypt.DefaultCost
    _ = time.Now
}
```

这是为避免编译器报未使用 import 错误的 hack，说明代码整理不彻底。

**修复建议**: 清理不必要的 import。

---

### 4.9 [设计] 前端路由守卫仅检查 token 存在性

**文件**: `web/src/router/index.ts:50-61`

```ts
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) {
    next('/login')
  }
})
```

**问题**: 仅检查 localStorage 中是否有 token，不验证 token 是否有效。过期 token 会导致用户看到空白页面或 401 错误循环。

**修复建议**: 在路由守卫中验证 token 过期时间（JWT 可以解码 payload 检查 exp）。

---

### 4.10 [安全] 配置文件包含敏感信息但权限宽松

**文件**: `internal/config/config.go:83`

```go
return os.WriteFile(path, data, 0644)
```

**问题**: 配置文件包含 JWT Secret，权限 0644（所有人可读）。

**修复建议**: 使用 `0600` 权限。

---

## 五、问题统计

| 严重级别 | 数量 | 分布 |
|---------|------|------|
| 必须修复 (Critical) | 6 | 安全 6 |
| 建议修复 (High) | 9 | 安全 3, 质量 3, 性能 2, 设计 1 |
| 可选优化 (Medium) | 10 | 安全 2, 质量 4, 性能 2, 设计 2 |
| **合计** | **25** | |

---

## 六、优先修复路线图

### 第一周（紧急）
1. ✏️ 修复 `generateRandomString` 使用 `crypto/rand` → **影响: JWT 密钥可预测**
2. ✏️ 移除自动创建默认管理员逻辑 → **影响: 默认凭据后门**
3. ✏️ 为 OAuth state 添加 mutex 保护 → **影响: 程序 panic**

### 第二周（重要）
4. ✏️ 限制 CORS 来源
5. ✏️ 移除 URL query token 支持
6. ✏️ 添加登录速率限制
7. ✏️ 修复 N+1 查询

### 第三周（改善）
8. ✏️ 添加核心模块单元测试
9. ✏️ 拆分 sublink.go
10. ✏️ 清理死代码和空文件

---

## 七、代码质量亮点

虽然存在上述问题，项目也有一些值得肯定的设计：

1. **优雅关闭** (`cmd/sboard/main.go:117-146`): 正确使用 signal 和 context 实现优雅关闭
2. **SQLite WAL 模式** (`database.go:38`): 提升并发读写性能
3. **Agent 心跳存活检测** (`agent_handler.go:38-55`): 12秒4次心跳的存活判断机制合理
4. **订阅格式自适应** (`sublink_utils.go:15-42`): 根据 User-Agent 自动选择订阅格式
5. **GeoIP 国内外分流** (`sublink_utils.go:150-168`): 自动下载和每日更新 GeoIP 数据库
