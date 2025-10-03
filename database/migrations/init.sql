-- SBoard 数据库初始化脚本
-- 版本: 2.0.0
-- 日期: 2025-10-01

-- ============================================
-- 1. 管理员用户表
-- ============================================
CREATE TABLE IF NOT EXISTS admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL,
    email TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 2. 代理用户表
-- ============================================
CREATE TABLE IF NOT EXISTS proxy_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    uuid TEXT UNIQUE NOT NULL,
    level INTEGER NOT NULL DEFAULT 1 CHECK(level IN (1,2,3)),
    expiry_date DATE NOT NULL,
    enabled INTEGER DEFAULT 1,
    traffic_limit INTEGER DEFAULT 0,  -- 流量限制(GB)，0表示无限制
    traffic_used INTEGER DEFAULT 0,   -- 已用流量(GB)
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 3. 入站节点表
-- ============================================
CREATE TABLE IF NOT EXISTS inbound_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT UNIQUE NOT NULL,
    protocol TEXT CHECK(protocol IN ('trojan','vless','vmess','anytls')) NOT NULL,
    listen TEXT DEFAULT '::',
    port INTEGER NOT NULL UNIQUE,
    -- TLS 配置
    tls_enabled INTEGER DEFAULT 1,
    server_name TEXT DEFAULT 'down.dingtalk.com',
    cert_path TEXT DEFAULT 'server.crt',
    key_path TEXT DEFAULT 'server.key',
    -- Reality 配置
    reality_enabled INTEGER DEFAULT 0,
    reality_server TEXT,
    reality_pubkey TEXT,
    reality_privkey TEXT,
    reality_short_id TEXT,
    -- 传输层配置
    transport_enabled INTEGER DEFAULT 0,
    transport_type TEXT CHECK(transport_type IN ('http','ws','grpc','httpupgrade')),
    ws_path TEXT DEFAULT '/',
    grpc_service TEXT,
    transport_host TEXT,
    -- VLESS Flow
    flow TEXT,
    -- 状态
    enabled INTEGER DEFAULT 1,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 4. 节点-用户关联表
-- ============================================
CREATE TABLE IF NOT EXISTS node_user_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    uuid TEXT NOT NULL,
    flow TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (node_id) REFERENCES inbound_nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES proxy_users(id) ON DELETE CASCADE,
    UNIQUE(node_id, user_id)
);

-- ============================================
-- 5. 服务器表
-- ============================================
CREATE TABLE IF NOT EXISTS servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 22,
    category TEXT NOT NULL CHECK(category IN ('direct','relay','home')),
    enabled INTEGER DEFAULT 1,
    -- 节点名称
    node_1 TEXT,
    node_2 TEXT,
    node_3 TEXT,
    -- 核心类型
    core_type TEXT NOT NULL DEFAULT 'sing-box' CHECK(core_type IN ('mihomo','sing-box')),
    -- DNS 解析
    dns_resolve TEXT DEFAULT 'none' CHECK(dns_resolve IN ('none','ipv4','ipv6')),
    -- SSH 配置
    ssh_user TEXT DEFAULT 'root',
    ssh_key_path TEXT DEFAULT '/var/www/.ssh/id_rsa',
    -- 其他
    notes TEXT,
    last_deploy_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 6. 部署日志表
-- ============================================
CREATE TABLE IF NOT EXISTS deploy_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    status TEXT CHECK(status IN ('success','failed','pending')) DEFAULT 'pending',
    message TEXT,
    log_content TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

-- ============================================
-- 7. 系统配置表
-- ============================================
CREATE TABLE IF NOT EXISTS system_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    value TEXT,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- 索引优化
-- ============================================
CREATE INDEX IF NOT EXISTS idx_proxy_users_uuid ON proxy_users(uuid);
CREATE INDEX IF NOT EXISTS idx_proxy_users_level ON proxy_users(level);
CREATE INDEX IF NOT EXISTS idx_proxy_users_expiry ON proxy_users(expiry_date);
CREATE INDEX IF NOT EXISTS idx_inbound_nodes_tag ON inbound_nodes(tag);
CREATE INDEX IF NOT EXISTS idx_servers_category ON servers(category);
CREATE INDEX IF NOT EXISTS idx_servers_enabled ON servers(enabled);
CREATE INDEX IF NOT EXISTS idx_deploy_logs_server ON deploy_logs(server_id);
CREATE INDEX IF NOT EXISTS idx_deploy_logs_created ON deploy_logs(created_at);

-- ============================================
-- 初始数据
-- ============================================

-- 系统配置
INSERT OR IGNORE INTO system_configs (key, value, description) VALUES
('site_name', 'SBoard', '网站名称'),
('site_url', 'http://localhost', '网站URL'),
('max_users', '100', '最大用户数'),
('default_expiry_days', '30', '默认到期天数'),
('enable_traffic_limit', '0', '启用流量限制'),
('ssh_timeout', '30', 'SSH连接超时(秒)');

-- ============================================
-- 触发器：自动更新 updated_at
-- ============================================
CREATE TRIGGER IF NOT EXISTS update_admins_timestamp 
AFTER UPDATE ON admins
BEGIN
    UPDATE admins SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_proxy_users_timestamp 
AFTER UPDATE ON proxy_users
BEGIN
    UPDATE proxy_users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_inbound_nodes_timestamp 
AFTER UPDATE ON inbound_nodes
BEGIN
    UPDATE inbound_nodes SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_servers_timestamp 
AFTER UPDATE ON servers
BEGIN
    UPDATE servers SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_system_configs_timestamp 
AFTER UPDATE ON system_configs
BEGIN
    UPDATE system_configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
