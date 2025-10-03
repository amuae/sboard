<?php
/**
 * 节点管理控制器
 */

namespace App\Controllers;

use App\Core\Database;
use App\Core\Response;

class NodeController extends BaseController
{
    /**
     * 显示节点列表页面
     */
    public function index(): void
    {
        $this->requireAuth();
        $this->view('nodes/index', [
            'username' => $this->getUsername(),
        ]);
    }

    /**
     * 获取节点列表 (API)
     */
    public function list(): void
    {
        $this->requireAuth();

        $db = Database::getInstance();
        $nodes = [];
        
        $result = $db->query('SELECT * FROM inbound_nodes ORDER BY id DESC');
        while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            $nodes[] = $row;
        }

        $this->success($nodes);
    }

    /**
     * 创建节点 (API)
     */
    public function create(): void
    {
        $this->requireAuth();

        $data = $this->request->all();
        
        $tag = trim($data['tag'] ?? '');
        $protocol = $data['protocol'] ?? 'trojan';
        $listen = trim($data['listen'] ?? '::');
        $port = (int)($data['port'] ?? 0);
        
        if (empty($tag) || $port < 1 || $port > 65535) {
            $this->error('标签和端口不能为空', 400);
            return;
        }

        $db = Database::getInstance();
        
        // 检查端口是否已使用
        $stmt = $db->prepare('SELECT id FROM inbound_nodes WHERE port = ?');
        $stmt->bindValue(1, $port, SQLITE3_INTEGER);
        if ($stmt->execute()->fetchArray()) {
            $this->error('端口已被占用', 400);
            return;
        }

        // 生成随机 short_id（如果启用了 Reality）
        $reality_short_id = null;
        if (!empty($data['reality_enabled'])) {
            // 生成 8-16 位随机十六进制字符串
            $reality_short_id = bin2hex(random_bytes(rand(4, 8)));
        }

        $stmt = $db->prepare('
            INSERT INTO inbound_nodes (
                tag, protocol, listen, port,
                tls_enabled, server_name, cert_path, key_path,
                reality_enabled, reality_server, reality_pubkey, reality_privkey, reality_short_id,
                transport_enabled, transport_type, ws_path, transport_host, grpc_service,
                flow, enabled
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ');

        $stmt->bindValue(1, $tag, SQLITE3_TEXT);
        $stmt->bindValue(2, $protocol, SQLITE3_TEXT);
        $stmt->bindValue(3, $listen, SQLITE3_TEXT);
        $stmt->bindValue(4, $port, SQLITE3_INTEGER);
        $stmt->bindValue(5, !empty($data['tls_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(6, trim($data['server_name'] ?? 'down.dingtalk.com'), SQLITE3_TEXT);
        $stmt->bindValue(7, trim($data['cert_path'] ?? 'server.crt'), SQLITE3_TEXT);
        $stmt->bindValue(8, trim($data['key_path'] ?? 'server.key'), SQLITE3_TEXT);
        $stmt->bindValue(9, !empty($data['reality_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(10, trim($data['reality_server'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(11, trim($data['reality_pubkey'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(12, trim($data['reality_privkey'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(13, $reality_short_id, SQLITE3_TEXT);
        $stmt->bindValue(14, !empty($data['transport_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(15, $data['transport_type'] ?? 'http', SQLITE3_TEXT);
        $stmt->bindValue(16, trim($data['ws_path'] ?? '/'), SQLITE3_TEXT);
        $stmt->bindValue(17, trim($data['transport_host'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(18, trim($data['grpc_service'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(19, trim($data['flow'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(20, !empty($data['enabled']) ? 1 : 0, SQLITE3_INTEGER);

        if ($stmt->execute()) {
            $nodeId = $db->lastInsertRowID();
            
            // 关联所有有效用户到这个节点
            $this->linkAllUsersToNode($nodeId);
            
            // 自动同步配置
            $this->autoSyncConfigs();
            
            $this->success(['id' => $nodeId], '节点创建成功');
        } else {
            $this->error('创建失败', 500);
        }
    }

    /**
     * 更新节点 (API)
     */
    public function update(string $id): void
    {
        $this->requireAuth();

        $data = $this->request->all();
        
        $tag = trim($data['tag'] ?? '');
        $protocol = $data['protocol'] ?? 'trojan';
        $listen = trim($data['listen'] ?? '::');
        $port = (int)($data['port'] ?? 0);
        
        if (empty($tag) || $port < 1 || $port > 65535) {
            $this->error('标签和端口不能为空', 400);
            return;
        }

        $db = Database::getInstance();
        
        // 检查端口是否被其他节点占用
        $stmt = $db->prepare('SELECT id FROM inbound_nodes WHERE port = ? AND id != ?');
        $stmt->bindValue(1, $port, SQLITE3_INTEGER);
        $stmt->bindValue(2, $id, SQLITE3_INTEGER);
        if ($stmt->execute()->fetchArray()) {
            $this->error('端口已被占用', 400);
            return;
        }

        // 生成随机 short_id（如果启用了 Reality）
        $reality_short_id = null;
        if (!empty($data['reality_enabled'])) {
            // 生成 8-16 位随机十六进制字符串
            $reality_short_id = bin2hex(random_bytes(rand(4, 8)));
        }

        $stmt = $db->prepare('
            UPDATE inbound_nodes SET
                tag = ?, protocol = ?, listen = ?, port = ?,
                tls_enabled = ?, server_name = ?, cert_path = ?, key_path = ?,
                reality_enabled = ?, reality_server = ?, reality_pubkey = ?, reality_privkey = ?, reality_short_id = ?,
                transport_enabled = ?, transport_type = ?, ws_path = ?, transport_host = ?, grpc_service = ?,
                flow = ?, enabled = ?
            WHERE id = ?
        ');

        $stmt->bindValue(1, $tag, SQLITE3_TEXT);
        $stmt->bindValue(2, $protocol, SQLITE3_TEXT);
        $stmt->bindValue(3, $listen, SQLITE3_TEXT);
        $stmt->bindValue(4, $port, SQLITE3_INTEGER);
        $stmt->bindValue(5, !empty($data['tls_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(6, trim($data['server_name'] ?? 'down.dingtalk.com'), SQLITE3_TEXT);
        $stmt->bindValue(7, trim($data['cert_path'] ?? 'server.crt'), SQLITE3_TEXT);
        $stmt->bindValue(8, trim($data['key_path'] ?? 'server.key'), SQLITE3_TEXT);
        $stmt->bindValue(9, !empty($data['reality_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(10, trim($data['reality_server'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(11, trim($data['reality_pubkey'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(12, trim($data['reality_privkey'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(13, $reality_short_id, SQLITE3_TEXT);
        $stmt->bindValue(14, !empty($data['transport_enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(15, $data['transport_type'] ?? 'http', SQLITE3_TEXT);
        $stmt->bindValue(16, trim($data['ws_path'] ?? '/'), SQLITE3_TEXT);
        $stmt->bindValue(17, trim($data['transport_host'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(18, trim($data['grpc_service'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(19, trim($data['flow'] ?? ''), SQLITE3_TEXT);
        $stmt->bindValue(20, !empty($data['enabled']) ? 1 : 0, SQLITE3_INTEGER);
        $stmt->bindValue(21, $id, SQLITE3_INTEGER);

        if ($stmt->execute()) {
            // 自动同步配置
            $this->autoSyncConfigs();
            
            $this->success(null, '节点更新成功');
        } else {
            $this->error('更新失败', 500);
        }
    }

    /**
     * 删除节点 (API)
     */
    public function delete($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            $stmt = $db->prepare("DELETE FROM inbound_nodes WHERE id = ?");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $stmt->execute();
            
            // 自动同步配置
            $this->autoSyncConfigs();
            
            Response::json([
                'success' => true,
                'message' => '节点删除成功'
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '删除节点失败: ' . $e->getMessage()
            ], 500);
        }
    }

    /**
     * 生成 Reality 密钥对
     */
    public function generateRealityKeys()
    {
        $this->requireAuth();

        try {
            // 检查 sing-box 是否存在
            $singboxPath = dirname(__DIR__, 2) . '/storage/configs/sing-box/sing-box';
            
            if (!file_exists($singboxPath)) {
                Response::json([
                    'success' => false,
                    'message' => 'sing-box 二进制文件不存在'
                ], 404);
                return;
            }

            // 执行 sing-box generate reality-keypair 命令
            $output = [];
            $returnCode = 0;
            exec("$singboxPath generate reality-keypair 2>&1", $output, $returnCode);
            
            if ($returnCode !== 0) {
                Response::json([
                    'success' => false,
                    'message' => '生成密钥对失败: ' . implode("\n", $output)
                ], 500);
                return;
            }

            // 解析输出
            $privateKey = '';
            $publicKey = '';
            
            foreach ($output as $line) {
                if (strpos($line, 'PrivateKey:') !== false) {
                    $privateKey = trim(str_replace('PrivateKey:', '', $line));
                } elseif (strpos($line, 'PublicKey:') !== false) {
                    $publicKey = trim(str_replace('PublicKey:', '', $line));
                }
            }

            if (empty($privateKey) || empty($publicKey)) {
                Response::json([
                    'success' => false,
                    'message' => '无法解析密钥对输出'
                ], 500);
                return;
            }

            Response::json([
                'success' => true,
                'message' => '密钥对生成成功',
                'data' => [
                    'private_key' => $privateKey,
                    'public_key' => $publicKey
                ]
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '生成密钥对失败: ' . $e->getMessage()
            ], 500);
        }
    }

    /**
     * 关联所有用户到节点
     */
    private function linkAllUsersToNode(int $nodeId): void
    {
        $db = Database::getInstance();
        
        // 获取节点信息
        $stmt = $db->prepare('SELECT protocol, flow FROM inbound_nodes WHERE id = ?');
        $stmt->bindValue(1, $nodeId, SQLITE3_INTEGER);
        $node = $stmt->execute()->fetchArray(SQLITE3_ASSOC);
        
        if (!$node) return;

        // 获取所有有效用户
        $users = $db->query('SELECT id, uuid FROM proxy_users WHERE enabled = 1 AND expiry_date >= date("now")');
        
        while ($user = $users->fetchArray(SQLITE3_ASSOC)) {
            $stmt = $db->prepare('
                INSERT OR REPLACE INTO node_user_relations (node_id, user_id, uuid, flow)
                VALUES (?, ?, ?, ?)
            ');
            $stmt->bindValue(1, $nodeId, SQLITE3_INTEGER);
            $stmt->bindValue(2, $user['id'], SQLITE3_INTEGER);
            $stmt->bindValue(3, $user['uuid'], SQLITE3_TEXT);
            $stmt->bindValue(4, $node['flow'], SQLITE3_TEXT);
            $stmt->execute();
        }
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
            
            $request = new \App\Core\Request();
            $userController = new UserController($request);
            
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "UserController 创建成功\n", FILE_APPEND);
            
            // 使用反射调用私有方法
            $reflection = new \ReflectionClass($userController);
            
            $singboxMethod = $reflection->getMethod('generateSingBoxConfig');
            $singboxMethod->setAccessible(true);
            $singboxMethod->invoke($userController);
            
            file_put_contents($logFile, date('[Y-m-d H:i:s] ') . "sing-box 配置生成成功\n", FILE_APPEND);
            
            $mihomoMethod = $reflection->getMethod('generateMihomoConfig');
            $mihomoMethod->setAccessible(true);
            $mihomoMethod->invoke($userController);
            
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
