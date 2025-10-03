<?php
/**
 * 用户管理控制器
 */

namespace App\Controllers;

use App\Core\Database;

class UserController extends BaseController
{
    /**
     * 显示用户列表页面
     */
    public function index(): void
    {
        $this->requireAuth();
        $this->view('users/index', [
            'username' => $this->getUsername(),
        ]);
    }

    /**
     * 获取用户列表 (API)
     */
    public function list(): void
    {
        $this->requireAuth();

        $db = Database::getInstance();
        $users = [];
        
        $result = $db->query('SELECT * FROM proxy_users ORDER BY id DESC');
        while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            $users[] = $row;
        }

        $this->success($users);
    }

    /**
     * 创建用户 (API)
     */
    public function create(): void
    {
        $this->requireAuth();

        $name = trim($this->request->post('name', ''));
        $uuid = trim($this->request->post('uuid', ''));
        $level = (int)$this->request->post('level', 1);
        $dns_resolve = (int)$this->request->post('dns_resolve', 0);
        $expiry_date = trim($this->request->post('expiry_date', ''));

        if (empty($name) || empty($expiry_date)) {
            $this->error('用户名和到期日期不能为空', 400);
            return;
        }

        // 如果未提供UUID，自动生成
        if (empty($uuid)) {
            $uuid = $this->generateUUID();
        }

        $db = Database::getInstance();
        $stmt = $db->prepare('
            INSERT INTO proxy_users (name, uuid, level, dns_resolve, expiry_date) 
            VALUES (?, ?, ?, ?, ?)
        ');
        
        $stmt->bindValue(1, $name, SQLITE3_TEXT);
        $stmt->bindValue(2, $uuid, SQLITE3_TEXT);
        $stmt->bindValue(3, $level, SQLITE3_INTEGER);
        $stmt->bindValue(4, $dns_resolve, SQLITE3_INTEGER);
        $stmt->bindValue(5, $expiry_date, SQLITE3_TEXT);

        if ($stmt->execute()) {
            $userId = $db->lastInsertRowID();
            
            // 关联到所有启用的节点
            $this->linkUserToNodes($userId, $uuid);
            
            // 自动同步配置
            $this->autoSyncConfigs();
            
            $this->success(['id' => $userId, 'uuid' => $uuid], '用户创建成功');
        } else {
            $this->error('创建失败', 500);
        }
    }

    /**
     * 更新用户 (API)
     */
    public function update(string $id): void
    {
        $this->requireAuth();

        $name = trim($this->request->post('name', ''));
        $uuid = trim($this->request->post('uuid', ''));
        $level = (int)$this->request->post('level', 1);
        $dns_resolve = (int)$this->request->post('dns_resolve', 0);
        $expiry_date = trim($this->request->post('expiry_date', ''));
        $enabled = $this->request->post('enabled', 1);

        if (empty($name) || empty($expiry_date)) {
            $this->error('用户名和到期日期不能为空', 400);
            return;
        }

        $db = Database::getInstance();
        
        if (empty($uuid)) {
            // 不更新UUID
            $stmt = $db->prepare('
                UPDATE proxy_users 
                SET name = ?, level = ?, dns_resolve = ?, expiry_date = ?, enabled = ?
                WHERE id = ?
            ');
            $stmt->bindValue(1, $name, SQLITE3_TEXT);
            $stmt->bindValue(2, $level, SQLITE3_INTEGER);
            $stmt->bindValue(3, $dns_resolve, SQLITE3_INTEGER);
            $stmt->bindValue(4, $expiry_date, SQLITE3_TEXT);
            $stmt->bindValue(5, $enabled, SQLITE3_INTEGER);
            $stmt->bindValue(6, $id, SQLITE3_INTEGER);
        } else {
            // 更新UUID
            $stmt = $db->prepare('
                UPDATE proxy_users 
                SET name = ?, uuid = ?, level = ?, dns_resolve = ?, expiry_date = ?, enabled = ?
                WHERE id = ?
            ');
            $stmt->bindValue(1, $name, SQLITE3_TEXT);
            $stmt->bindValue(2, $uuid, SQLITE3_TEXT);
            $stmt->bindValue(3, $level, SQLITE3_INTEGER);
            $stmt->bindValue(4, $dns_resolve, SQLITE3_INTEGER);
            $stmt->bindValue(5, $expiry_date, SQLITE3_TEXT);
            $stmt->bindValue(6, $enabled, SQLITE3_INTEGER);
            $stmt->bindValue(7, $id, SQLITE3_INTEGER);
            
            // 更新节点关联中的UUID
            $this->updateNodeRelations($id, $uuid);
        }

        if ($stmt->execute()) {
            // 自动同步配置
            $this->autoSyncConfigs();
            
            $this->success(null, '用户更新成功');
        } else {
            $this->error('更新失败', 500);
        }
    }

    /**
     * 删除用户 (API)
     */
    public function delete(string $id): void
    {
        $this->requireAuth();

        $db = Database::getInstance();
        $stmt = $db->prepare('DELETE FROM proxy_users WHERE id = ?');
        $stmt->bindValue(1, $id, SQLITE3_INTEGER);

        if ($stmt->execute()) {
            // 自动同步配置
            $this->autoSyncConfigs();
            
            $this->success(null, '用户删除成功');
        } else {
            $this->error('删除失败', 500);
        }
    }

    /**
     * 同步配置 (API)
     */
    public function syncConfigs(): void
    {
        $this->requireAuth();

        try {
            // 生成 sing-box 配置
            $this->generateSingBoxConfig();
            
            // 生成 mihomo 配置
            $this->generateMihomoConfig();

            $this->success(null, '配置同步成功');
        } catch (\Exception $e) {
            $this->error('同步失败: ' . $e->getMessage(), 500);
        }
    }

    /**
     * 生成UUID
     */
    private function generateUUID(): string
    {
        return sprintf('%04x%04x-%04x-%04x-%04x-%04x%04x%04x',
            random_int(0, 0xffff), random_int(0, 0xffff),
            random_int(0, 0xffff),
            random_int(0, 0x0fff) | 0x4000,
            random_int(0, 0x3fff) | 0x8000,
            random_int(0, 0xffff), random_int(0, 0xffff), random_int(0, 0xffff)
        );
    }

    /**
     * 关联用户到所有节点
     */
    private function linkUserToNodes(int $userId, string $uuid): void
    {
        $db = Database::getInstance();
        
        $nodes = $db->query('SELECT id, protocol, flow FROM inbound_nodes WHERE enabled = 1');
        while ($node = $nodes->fetchArray(SQLITE3_ASSOC)) {
            $stmt = $db->prepare('
                INSERT OR REPLACE INTO node_user_relations (node_id, user_id, uuid, flow)
                VALUES (?, ?, ?, ?)
            ');
            $stmt->bindValue(1, $node['id'], SQLITE3_INTEGER);
            $stmt->bindValue(2, $userId, SQLITE3_INTEGER);
            $stmt->bindValue(3, $uuid, SQLITE3_TEXT);
            $stmt->bindValue(4, $node['flow'], SQLITE3_TEXT);
            $stmt->execute();
        }
    }

    /**
     * 更新节点关联
     */
    private function updateNodeRelations(int $userId, string $uuid): void
    {
        $db = Database::getInstance();
        $stmt = $db->prepare('UPDATE node_user_relations SET uuid = ? WHERE user_id = ?');
        $stmt->bindValue(1, $uuid, SQLITE3_TEXT);
        $stmt->bindValue(2, $userId, SQLITE3_INTEGER);
        $stmt->execute();
    }

    /**
     * 生成 sing-box 配置 (v1.13+)
     */
    private function generateSingBoxConfig(): void
    {
        $db = Database::getInstance();
        $config = [
            'log' => [
                'level' => 'info',
                'timestamp' => true
            ],
            'inbounds' => [],
            'outbounds' => [
                [
                    'type' => 'direct',
                    'tag' => 'direct'
                ]
            ]
        ];

        $nodes = $db->query('SELECT * FROM inbound_nodes WHERE enabled = 1 ORDER BY id');
        while ($node = $nodes->fetchArray(SQLITE3_ASSOC)) {
            $users = [];
            
            $stmt = $db->prepare('
                SELECT r.uuid, r.flow, u.name 
                FROM node_user_relations r
                JOIN proxy_users u ON u.id = r.user_id
                WHERE r.node_id = ? AND u.enabled = 1 AND u.expiry_date >= date("now")
            ');
            $stmt->bindValue(1, $node['id'], SQLITE3_INTEGER);
            $result = $stmt->execute();

            while ($user = $result->fetchArray(SQLITE3_ASSOC)) {
                $userEntry = ['name' => $user['name']];
                
                if ($node['protocol'] === 'trojan') {
                    $userEntry['password'] = $user['uuid'];
                } elseif ($node['protocol'] === 'vless') {
                    $userEntry['uuid'] = $user['uuid'];
                    // VLESS Flow: 仅在 TLS 启用且没有传输层时使用
                    if ($node['tls_enabled'] && !$node['transport_enabled'] && !empty($user['flow'])) {
                        $userEntry['flow'] = $user['flow'];
                    }
                } elseif ($node['protocol'] === 'vmess') {
                    $userEntry['uuid'] = $user['uuid'];
                    $userEntry['alterId'] = 0; // VMess AEAD
                }
                
                $users[] = $userEntry;
            }

            if (empty($users)) continue;

            $inbound = [
                'type' => $node['protocol'],
                'tag' => $node['tag'],
                'listen' => $node['listen'],
                'listen_port' => (int)$node['port'],
                'users' => $users
            ];

            // TLS 配置 (sing-box 1.13+)
            if ($node['tls_enabled']) {
                $tls = [
                    'enabled' => true,
                    'server_name' => $node['server_name']
                ];

                // Reality 配置（Reality 和传输层互斥）
                if ($node['reality_enabled'] && !empty($node['reality_pubkey']) && !$node['transport_enabled']) {
                    $tls['reality'] = [
                        'enabled' => true,
                        'handshake' => [
                            'server' => $node['reality_server'] ?: 'www.apple.com',
                            'server_port' => 443
                        ],
                        'private_key' => $node['reality_privkey'] ?: '', // 从数据库读取私钥
                        'short_id' => [$node['reality_short_id'] ?: '']
                    ];
                } else {
                    // 普通 TLS
                    $tls['certificate_path'] = '/root/sing-box/' . $node['cert_path'];
                    $tls['key_path'] = '/root/sing-box/' . $node['key_path'];
                }

                $inbound['tls'] = $tls;
            }

            // 传输层配置 (sing-box 1.13+)
            // 传输层和 Reality/Flow 互斥
            if ($node['transport_enabled'] && !$node['reality_enabled']) {
                $transport = [];
                
                switch ($node['transport_type']) {
                    case 'ws':
                        $transport = [
                            'type' => 'ws',
                            'path' => $node['ws_path'] ?: '/',
                            'max_early_data' => 2048,
                            'early_data_header_name' => 'Sec-WebSocket-Protocol'
                        ];
                        // 添加 Host 伪装
                        if (!empty($node['transport_host'])) {
                            $transport['headers'] = ['Host' => $node['transport_host']];
                        }
                        break;
                    
                    case 'grpc':
                        $transport = [
                            'type' => 'grpc',
                            'service_name' => $node['grpc_service'] ?: 'GunService'
                        ];
                        break;
                    
                    case 'http':
                        $transport = [
                            'type' => 'http',
                            'host' => !empty($node['transport_host']) ? [$node['transport_host']] : [$node['server_name']],
                            'path' => $node['ws_path'] ?: '/'
                        ];
                        break;
                    
                    case 'httpupgrade':
                        $transport = [
                            'type' => 'httpupgrade',
                            'host' => !empty($node['transport_host']) ? $node['transport_host'] : $node['server_name'],
                            'path' => $node['ws_path'] ?: '/'
                        ];
                        break;
                }
                
                if (!empty($transport)) {
                    $inbound['transport'] = $transport;
                }
            }

            $config['inbounds'][] = $inbound;
        }

        $configPath = dirname(__DIR__, 2) . '/storage/configs/sing-box/config.json';
        $json = json_encode($config, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
        // 将4个空格缩进改为2个空格
        $json = preg_replace_callback('/^(  +)/m', function($matches) {
            return str_repeat(' ', strlen($matches[1]) / 2);
        }, $json);
        file_put_contents($configPath, $json);
    }

    /**
     * 生成 mihomo 配置 (v1.19+)
     */
    private function generateMihomoConfig(): void
    {
        $db = Database::getInstance();
        
        $config = [
            'listeners' => []
        ];

        $nodes = $db->query('SELECT * FROM inbound_nodes WHERE enabled = 1 ORDER BY id');
        while ($node = $nodes->fetchArray(SQLITE3_ASSOC)) {
            $stmt = $db->prepare('
                SELECT r.uuid, r.flow, u.name 
                FROM node_user_relations r
                JOIN proxy_users u ON u.id = r.user_id
                WHERE r.node_id = ? AND u.enabled = 1 AND u.expiry_date >= date("now")
            ');
            $stmt->bindValue(1, $node['id'], SQLITE3_INTEGER);
            $result = $stmt->execute();

            $userList = [];
            while ($user = $result->fetchArray(SQLITE3_ASSOC)) {
                $userList[] = [
                    'name' => $user['name'],
                    'uuid' => $user['uuid'],
                    'flow' => $user['flow']
                ];
            }

            if (empty($userList)) continue;

            $listener = [
                'name' => $node['tag'],
                'type' => $node['protocol'],
                'port' => (int)$node['port'],
                'listen' => $node['listen']
            ];

            // 根据协议类型设置用户格式
            if ($node['protocol'] === 'anytls') {
                // anytls: users 是对象 {username: password}
                $usersObj = [];
                foreach ($userList as $user) {
                    $usersObj[$user['name']] = $user['uuid'];
                }
                $listener['users'] = $usersObj;
            } elseif ($node['protocol'] === 'vless') {
                // vless: users 数组包含 username, uuid, flow
                $usersArray = [];
                foreach ($userList as $user) {
                    $userEntry = [
                        'username' => $user['name'],
                        'uuid' => $user['uuid']
                    ];
                    // 如果有 flow，添加 flow 字段
                    if (!empty($user['flow'])) {
                        $userEntry['flow'] = $user['flow'];
                    }
                    $usersArray[] = $userEntry;
                }
                $listener['users'] = $usersArray;
            } elseif ($node['protocol'] === 'vmess') {
                // vmess: users 数组包含 username, uuid, alterId
                $usersArray = [];
                foreach ($userList as $user) {
                    $usersArray[] = [
                        'username' => $user['name'],
                        'uuid' => $user['uuid'],
                        'alterId' => 0  // 默认 0
                    ];
                }
                $listener['users'] = $usersArray;
            } elseif ($node['protocol'] === 'trojan') {
                // trojan: users 数组包含 username, password
                $usersArray = [];
                foreach ($userList as $user) {
                    $usersArray[] = [
                        'username' => $user['name'],
                        'password' => $user['uuid']
                    ];
                }
                $listener['users'] = $usersArray;
            }

            // 传输层配置（ws-path 或 grpc-service-name）
            if ($node['transport_enabled'] && $node['transport_type']) {
                switch ($node['transport_type']) {
                    case 'ws':
                        $listener['ws-path'] = $node['ws_path'] ?: '/';
                        break;
                    
                    case 'grpc':
                        $listener['grpc-service-name'] = $node['grpc_service'] ?: 'GunService';
                        break;
                }
            }

            // TLS/Reality 配置
            if ($node['tls_enabled']) {
                if ($node['reality_enabled'] && !empty($node['reality_privkey'])) {
                    // Reality 配置
                    $destServer = $node['reality_server'] ?: 'www.apple.com';
                    
                    // 处理多域名情况（如 "down.dingtalk.com,dingtalk.com"）
                    if (strpos($destServer, ',') !== false) {
                        $destServer = trim(explode(',', $destServer)[0]);
                    }
                    
                    // 如果没有端口，添加 :443
                    if (strpos($destServer, ':') === false) {
                        $destServer .= ':443';
                    }
                    
                    // server-names 可以包含多个域名
                    $serverNames = [];
                    if (!empty($node['server_name'])) {
                        $serverNames[] = $node['server_name'];
                    } else {
                        // 从 dest 中提取域名
                        $serverNames[] = explode(':', $destServer)[0];
                    }
                    
                    $realityConfig = [
                        'dest' => $destServer,
                        'private-key' => $node['reality_privkey']
                    ];
                    
                    // short-id 必须是数组
                    if (!empty($node['reality_short_id'])) {
                        $realityConfig['short-id'] = [$node['reality_short_id']];
                    }
                    
                    $realityConfig['server-names'] = $serverNames;
                    
                    $listener['reality-config'] = $realityConfig;
                } else {
                    // 普通 TLS - 证书配置（平级字段）
                    $listener['certificate'] = './server.crt';
                    $listener['private-key'] = './server.key';
                }
            }

            $config['listeners'][] = $listener;
        }

        // 生成 YAML
        $yaml = $this->arrayToYaml($config);
        
        $configPath = dirname(__DIR__, 2) . '/storage/configs/mihomo/config.yaml';
        file_put_contents($configPath, $yaml);
    }

    /**
     * 数组转 YAML 格式
     */
    private function arrayToYaml($data, $indent = 0): string
    {
        $yaml = '';
        $indentStr = str_repeat('  ', $indent);

        foreach ($data as $key => $value) {
            if (is_array($value)) {
                // 检查是否为索引数组
                if (array_keys($value) === range(0, count($value) - 1)) {
                    $yaml .= $indentStr . $key . ":\n";
                    foreach ($value as $item) {
                        if (is_array($item)) {
                            $yaml .= $indentStr . "- " . ltrim($this->arrayToYaml($item, $indent + 1));
                        } else {
                            $yaml .= $indentStr . "- " . $this->yamlValue($item) . "\n";
                        }
                    }
                } else {
                    // 关联数组
                    $yaml .= $indentStr . $key . ":\n";
                    $yaml .= $this->arrayToYaml($value, $indent + 1);
                }
            } else {
                $yaml .= $indentStr . $key . ": " . $this->yamlValue($value) . "\n";
            }
        }

        return $yaml;
    }

    /**
     * YAML 值格式化
     */
    private function yamlValue($value): string
    {
        if (is_bool($value)) {
            return $value ? 'true' : 'false';
        }
        if (is_null($value)) {
            return 'null';
        }
        if (is_numeric($value)) {
            return (string)$value;
        }
        // 字符串需要引号的情况
        if (preg_match('/[:\[\]{},&*#?|\-<>=!%@`]/', $value) || $value === '') {
            return '"' . str_replace('"', '\\"', $value) . '"';
        }
        return $value;
    }

    /**
     * 自动同步配置（静默模式）
     */
    private function autoSyncConfigs(): void
    {
        try {
            $logFile = dirname(__DIR__, 2) . '/storage/logs/sync.log';
            $logDir = dirname($logFile);
            if (!is_dir($logDir)) {
                mkdir($logDir, 0755, true);
            }
            
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "开始同步配置\n", FILE_APPEND);
            
            $this->generateSingBoxConfig();
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "sing-box 配置生成成功\n", FILE_APPEND);
            
            $this->generateMihomoConfig();
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "mihomo 配置生成成功\n", FILE_APPEND);
        } catch (\Exception $e) {
            // 静默失败，不影响主操作
            $logFile = dirname(__DIR__, 2) . '/storage/logs/sync.log';
            $logDir = dirname($logFile);
            if (!is_dir($logDir)) {
                mkdir($logDir, 0755, true);
            }
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "自动同步配置失败: " . $e->getMessage() . "\n" . $e->getTraceAsString() . "\n", FILE_APPEND);
            error_log('自动同步配置失败: ' . $e->getMessage());
        }
    }
}
