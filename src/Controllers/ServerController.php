<?php
namespace App\Controllers;

use App\Core\Database;
use App\Core\Response;
use App\Core\Request;

class ServerController extends BaseController
{
    public function index()
    {
        $this->requireAuth();
        return $this->view('servers/index', ['username' => $_SESSION['username']]);
    }

    public function list()
    {
        $this->requireAuth();
        
        try {
            $db = Database::getInstance();
            $stmt = $db->query("SELECT * FROM servers ORDER BY created_at DESC");
            $servers = [];
            while ($row = $stmt->fetchArray(SQLITE3_ASSOC)) {
                $servers[] = $row;
            }
            
            Response::json([
                'success' => true,
                'data' => $servers
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '获取服务器列表失败: ' . $e->getMessage()
            ], 500);
        }
    }

    public function create()
    {
        $this->requireAuth();
        
        $data = $this->request->all();
        
        if (empty($data['name']) || empty($data['host'])) {
            Response::json([
                'success' => false,
                'message' => '服务器名称和地址不能为空'
            ], 400);
            return;
        }

        try {
            $db = Database::getInstance();
            
            $stmt = $db->prepare("INSERT INTO servers (name, host, port, category, enabled, node_1, node_2, node_3, core_type, dns_resolve) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)");
            $stmt->bindValue(1, $data['name'], SQLITE3_TEXT);
            $stmt->bindValue(2, $data['host'], SQLITE3_TEXT);
            $stmt->bindValue(3, $data['port'] ?? 22, SQLITE3_INTEGER);
            $stmt->bindValue(4, $data['category'] ?? 'direct', SQLITE3_TEXT);
            $stmt->bindValue(5, $data['enabled'] ?? 1, SQLITE3_INTEGER);
            $stmt->bindValue(6, $data['node_1'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(7, $data['node_2'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(8, $data['node_3'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(9, $data['core_type'] ?? 'sing-box', SQLITE3_TEXT);
            $stmt->bindValue(10, $data['dns_resolve'] ?? 'none', SQLITE3_TEXT);
            
            $stmt->execute();
            
            Response::json([
                'success' => true,
                'message' => '服务器创建成功',
                'data' => ['id' => $db->lastInsertRowID()]
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '创建服务器失败: ' . $e->getMessage()
            ], 500);
        }
    }

    public function update($id)
    {
        $this->requireAuth();
        
        $data = $this->request->all();

        try {
            $db = Database::getInstance();
            
            $stmt = $db->prepare("UPDATE servers SET name = ?, host = ?, port = ?, category = ?, enabled = ?, node_1 = ?, node_2 = ?, node_3 = ?, core_type = ?, dns_resolve = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?");
            $stmt->bindValue(1, $data['name'], SQLITE3_TEXT);
            $stmt->bindValue(2, $data['host'], SQLITE3_TEXT);
            $stmt->bindValue(3, $data['port'] ?? 22, SQLITE3_INTEGER);
            $stmt->bindValue(4, $data['category'] ?? 'direct', SQLITE3_TEXT);
            $stmt->bindValue(5, $data['enabled'] ?? 1, SQLITE3_INTEGER);
            $stmt->bindValue(6, $data['node_1'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(7, $data['node_2'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(8, $data['node_3'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(9, $data['core_type'] ?? 'sing-box', SQLITE3_TEXT);
            $stmt->bindValue(10, $data['dns_resolve'] ?? 'none', SQLITE3_TEXT);
            $stmt->bindValue(11, $id, SQLITE3_INTEGER);
            
            $stmt->execute();
            
            Response::json([
                'success' => true,
                'message' => '服务器更新成功'
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '更新服务器失败: ' . $e->getMessage()
            ], 500);
        }
    }

    public function delete($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            $stmt = $db->prepare("DELETE FROM servers WHERE id = ?");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $stmt->execute();
            
            Response::json([
                'success' => true,
                'message' => '服务器删除成功'
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '删除服务器失败: ' . $e->getMessage()
            ], 500);
        }
    }

    public function deploy($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            
            $stmt = $db->prepare("SELECT * FROM servers WHERE id = ?");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $result = $stmt->execute();
            $server = $result->fetchArray(SQLITE3_ASSOC);
            
            if (!$server) {
                Response::json(['success' => false, 'message' => '服务器不存在'], 404);
                return;
            }

            // 源目录：整个核心目录
            $sourceDir = dirname(__DIR__, 2) . '/storage/configs/' . $server['core_type'] . '/';
            
            if (!is_dir($sourceDir)) {
                Response::json(['success' => false, 'message' => '核心目录不存在'], 404);
                return;
            }

            // 目标目录
            $targetDir = '/root/' . $server['core_type'] . '/';
            
            $logStmt = $db->prepare("INSERT INTO deploy_logs (server_id, status, message) VALUES (?, ?, ?)");
            
            // 创建临时密码文件用于 sshpass
            $tempPassFile = tempnam(sys_get_temp_dir(), 'sshpass_');
            file_put_contents($tempPassFile, $server['password']);
            chmod($tempPassFile, 0600);
            
            // 使用 rsync + sshpass 部署整个目录
            $rsyncCmd = sprintf(
                'sshpass -f %s rsync -avz --delete -e "ssh -p %d -o StrictHostKeyChecking=no" %s %s@%s:%s 2>&1',
                escapeshellarg($tempPassFile),
                $server['port'],
                escapeshellarg($sourceDir),
                escapeshellarg($server['username']),
                escapeshellarg($server['host']),
                escapeshellarg($targetDir)
            );
            
            exec($rsyncCmd, $output, $returnCode);
            unlink($tempPassFile);
            
            if ($returnCode !== 0) {
                $errorMsg = implode("\n", $output);
                $logStmt->bindValue(1, $id, SQLITE3_INTEGER);
                $logStmt->bindValue(2, 'failed', SQLITE3_TEXT);
                $logStmt->bindValue(3, 'rsync 失败: ' . $errorMsg, SQLITE3_TEXT);
                $logStmt->execute();
                
                Response::json(['success' => false, 'message' => '目录同步失败: ' . $errorMsg]);
                return;
            }

            // 执行部署脚本
            $deployCmd = sprintf(
                'sshpass -p %s ssh -p %d -o StrictHostKeyChecking=no %s@%s "cd %s && chmod +x deploy.sh && ./deploy.sh" 2>&1',
                escapeshellarg($server['password']),
                $server['port'],
                escapeshellarg($server['username']),
                escapeshellarg($server['host']),
                escapeshellarg($targetDir)
            );
            
            exec($deployCmd, $deployOutput, $deployReturnCode);
            
            $deployMessage = implode("\n", $deployOutput);
            
            if ($deployReturnCode === 0) {
                $logStmt->bindValue(1, $id, SQLITE3_INTEGER);
                $logStmt->bindValue(2, 'success', SQLITE3_TEXT);
                $logStmt->bindValue(3, '部署成功: ' . $deployMessage, SQLITE3_TEXT);
                $logStmt->execute();
                
                Response::json([
                    'success' => true,
                    'message' => '核心部署成功',
                    'data' => ['output' => $deployMessage]
                ]);
            } else {
                $logStmt->bindValue(1, $id, SQLITE3_INTEGER);
                $logStmt->bindValue(2, 'failed', SQLITE3_TEXT);
                $logStmt->bindValue(3, '部署脚本执行失败: ' . $deployMessage, SQLITE3_TEXT);
                $logStmt->execute();
                
                Response::json([
                    'success' => false,
                    'message' => '部署脚本执行失败: ' . $deployMessage
                ], 500);
            }
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '部署失败: ' . $e->getMessage()
            ], 500);
        }
    }

    public function checkStatus($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            $stmt = $db->prepare("SELECT * FROM servers WHERE id = ?");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $result = $stmt->execute();
            $server = $result->fetchArray(SQLITE3_ASSOC);
            
            if (!$server) {
                Response::json(['success' => false, 'message' => '服务器不存在'], 404);
                return;
            }

            $fp = @fsockopen($server['host'], $server['port'], $errno, $errstr, 3);
            $online = $fp !== false;
            if ($fp) fclose($fp);
            
            Response::json([
                'success' => true,
                'data' => ['online' => $online]
            ]);
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '检查状态失败: ' . $e->getMessage()
            ], 500);
        }
    }

    /**
     * 单节点部署（单个配置文件）
     */
    public function deploySingle($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            $stmt = $db->prepare("SELECT * FROM servers WHERE id = ? AND enabled = 1");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $result = $stmt->execute();
            $server = $result->fetchArray(SQLITE3_ASSOC);
            
            if (!$server) {
                Response::json(['success' => false, 'message' => '服务器不存在或已禁用'], 404);
                return;
            }

            // 检查在线状态
            $fp = @fsockopen($server['host'], $server['port'], $errno, $errstr, 3);
            if (!$fp) {
                Response::json(['success' => false, 'message' => '服务器离线'], 503);
                return;
            }
            fclose($fp);

            // 启用流式响应
            header('Content-Type: text/event-stream');
            header('Cache-Control: no-cache');
            header('Connection: keep-alive');
            header('X-Accel-Buffering: no');
            
            if (ob_get_level()) {
                ob_end_clean();
            }
            
            $this->streamLog('[' . date('H:i:s') . '] 🚀 开始单文件部署: ' . $server['name'] . ' (' . $server['core_type'] . ')');
            usleep(100000);

            $core = $server['core_type'];
            $configDir = dirname(__DIR__, 2) . '/storage/configs/' . $core;
            $localFile = $core === 'sing-box' ? $configDir . '/config.json' : $configDir . '/config.yaml';
            $remoteDir = '/root/' . $core;
            $remoteFile = $remoteDir . '/' . ($core === 'sing-box' ? 'config.json' : 'config.yaml');
            $keyPath = '/var/www/.ssh/id_rsa';
            $sshOpt = '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR';

            if (!file_exists($localFile)) {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 本地配置不存在: ' . $localFile);
                echo "data: " . json_encode(['success' => false, 'message' => '本地配置不存在']) . "\n\n";
                flush();
                return;
            }

            if (!file_exists($keyPath)) {
                $this->streamLog('[' . date('H:i:s') . '] ❌ SSH 私钥不存在: ' . $keyPath);
                echo "data: " . json_encode(['success' => false, 'message' => 'SSH 私钥不存在']) . "\n\n";
                flush();
                return;
            }

            // 上传配置文件
            $this->streamLog('[' . date('H:i:s') . '] 📤 上传配置文件...');
            $fileName = basename($localFile);
            $fileSize = filesize($localFile);
            $this->streamLog('[' . date('H:i:s') . '] 📄 文件: ' . $fileName . ' (' . round($fileSize/1024, 2) . ' KB)');
            usleep(200000);
            
            $cmd = sprintf(
                'scp -i %s %s -P %d %s root@%s:%s 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($localFile),
                escapeshellarg($server['host']),
                escapeshellarg($remoteFile)
            );
            
            // 使用proc_open实时输出
            $descriptors = [
                0 => ['pipe', 'r'],
                1 => ['pipe', 'w'],
                2 => ['pipe', 'w']
            ];
            
            $process = proc_open($cmd, $descriptors, $pipes);
            
            if (is_resource($process)) {
                fclose($pipes[0]);
                stream_set_blocking($pipes[1], false);
                stream_set_blocking($pipes[2], false);
                
                $output = '';
                while (!feof($pipes[1]) || !feof($pipes[2])) {
                    $stdout = fgets($pipes[1]);
                    $stderr = fgets($pipes[2]);
                    
                    if ($stdout !== false && trim($stdout) !== '') {
                        $output .= $stdout;
                        $this->streamLog('[' . date('H:i:s') . '] ' . trim($stdout));
                        usleep(50000);
                    }
                    
                    if ($stderr !== false && trim($stderr) !== '') {
                        $output .= $stderr;
                        if (strpos($stderr, '100%') !== false) {
                            $this->streamLog('[' . date('H:i:s') . '] ✅ ' . trim($stderr));
                        } else {
                            $this->streamLog('[' . date('H:i:s') . '] ' . trim($stderr));
                        }
                        usleep(50000);
                    }
                    
                    usleep(10000);
                }
                
                fclose($pipes[1]);
                fclose($pipes[2]);
                $returnCode = proc_close($process);
                
                if ($returnCode !== 0 || (strpos($output, '100%') === false && $output)) {
                    $this->streamLog('[' . date('H:i:s') . '] ❌ 上传失败');
                    echo "data: " . json_encode(['success' => false, 'message' => '上传失败']) . "\n\n";
                    flush();
                    return;
                }
                
                $this->streamLog('[' . date('H:i:s') . '] ✅ 文件上传完成');
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 无法启动scp进程');
                echo "data: " . json_encode(['success' => false, 'message' => 'scp启动失败']) . "\n\n";
                flush();
                return;
            }
            usleep(200000);

            // 停止对侧服务
            $otherService = $core === 'sing-box' ? 'mihomo' : 'sing-box';
            $this->streamLog('[' . date('H:i:s') . '] � 检查对侧服务 ' . $otherService . '...');
            
            // 先检查对侧服务状态
            $checkCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "systemctl is-active %s 2>/dev/null || echo inactive" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host']),
                $otherService
            );
            $otherStatus = trim(shell_exec($checkCmd) ?? 'inactive');
            
            if ($otherStatus === 'active') {
                $this->streamLog('[' . date('H:i:s') . '] �🛑 停止运行中的 ' . $otherService . '...');
                $stopCmd = sprintf(
                    'ssh -i %s %s -p %d root@%s "systemctl disable %s && systemctl stop %s 2>/dev/null || true" 2>&1',
                    escapeshellarg($keyPath),
                    $sshOpt,
                    $server['port'],
                    escapeshellarg($server['host']),
                    $otherService,
                    $otherService
                );
                shell_exec($stopCmd);
                $this->streamLog('[' . date('H:i:s') . '] ✅ 已停止 ' . $otherService);
                usleep(300000);
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ⏭️ ' . $otherService . ' 未运行，跳过停止');
                usleep(100000);
            }

            // 重启当前服务
            $this->streamLog('[' . date('H:i:s') . '] 🔄 重新加载systemd...');
            usleep(200000);
            
            $this->streamLog('[' . date('H:i:s') . '] 🚀 重启 ' . $core . ' 服务...');
            $restartCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "systemctl daemon-reload && systemctl enable %s && systemctl restart %s && systemctl is-active %s" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host']),
                $core,
                $core,
                $core
            );
            
            $status = trim(shell_exec($restartCmd) ?? '');
            if ($status !== 'active') {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 服务启动失败: ' . $status);
                echo "data: " . json_encode(['success' => false, 'message' => '服务重启失败']) . "\n\n";
                flush();
                return;
            }

            $this->streamLog('[' . date('H:i:s') . '] ✅ 服务 ' . $core . ' 已成功启动');
            usleep(200000);

            // 更新最后部署时间
            $updateStmt = $db->prepare("UPDATE servers SET last_deploy_at = CURRENT_TIMESTAMP WHERE id = ?");
            $updateStmt->bindValue(1, $id, SQLITE3_INTEGER);
            $updateStmt->execute();

            $this->streamLog('[' . date('H:i:s') . '] 🎉 单文件部署完成!');
            echo "data: " . json_encode(['success' => true, 'message' => '部署成功', 'finished' => true]) . "\n\n";
            flush();
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '部署失败: ' . $e->getMessage()
            ], 500);
        }
    }

    /**
     * 目录部署（整个核心目录）
     */
    /**
     * 实时流式输出日志的辅助函数
     */
    private function streamLog($message)
    {
        echo "data: " . json_encode(['log' => $message]) . "\n\n";
        if (ob_get_level() > 0) {
            ob_flush();
        }
        flush();
    }

    /**
     * 目录部署 - 流式实时日志版本
     */
    public function deployFolder($id)
    {
        $this->requireAuth();

        try {
            $db = Database::getInstance();
            $stmt = $db->prepare("SELECT * FROM servers WHERE id = ? AND enabled = 1 AND category != 'home'");
            $stmt->bindValue(1, $id, SQLITE3_INTEGER);
            $result = $stmt->execute();
            $server = $result->fetchArray(SQLITE3_ASSOC);
            
            if (!$server) {
                Response::json(['success' => false, 'message' => '服务器不存在或属于家宽'], 404);
                return;
            }

            // 检查在线状态
            $fp = @fsockopen($server['host'], $server['port'], $errno, $errstr, 3);
            if (!$fp) {
                Response::json(['success' => false, 'message' => '服务器离线'], 503);
                return;
            }
            fclose($fp);

            // 启用流式响应
            header('Content-Type: text/event-stream');
            header('Cache-Control: no-cache');
            header('Connection: keep-alive');
            header('X-Accel-Buffering: no'); // 禁用nginx缓冲
            
            // 禁用输出缓冲
            if (ob_get_level()) {
                ob_end_clean();
            }
            
            $this->streamLog('[' . date('H:i:s') . '] 🚀 开始目录部署: ' . $server['name'] . ' (' . $server['core_type'] . ')');
            usleep(100000); // 100ms延迟确保消息发送

            $core = $server['core_type'];
            $localDir = dirname(__DIR__, 2) . '/storage/configs/' . $core;
            $remoteDir = '/root/' . $core;
            $unitFile = $core . '.service';
            $unitDst = '/etc/systemd/system/' . $unitFile;
            $keyPath = '/var/www/.ssh/id_rsa';
            $sshOpt = '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR';

            if (!is_dir($localDir)) {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 本地目录不存在: ' . $localDir);
                echo "data: " . json_encode(['success' => false, 'message' => '本地目录不存在']) . "\n\n";
                flush();
                return;
            }

            if (!file_exists($keyPath)) {
                $this->streamLog('[' . date('H:i:s') . '] ❌ SSH 私钥不存在: ' . $keyPath);
                echo "data: " . json_encode(['success' => false, 'message' => 'SSH 私钥不存在']) . "\n\n";
                flush();
                return;
            }

            // 1. 检查/安装 rsync
            $this->streamLog('[' . date('H:i:s') . '] ⚙️ 检查远程rsync...');
            $installCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "which rsync || (apt update && apt install -y rsync || yum install -y rsync || dnf install -y rsync || apk add rsync)" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host'])
            );
            $output = shell_exec($installCmd);
            if (strpos($output, 'installing') !== false || strpos($output, 'install') !== false) {
                $this->streamLog('[' . date('H:i:s') . '] ⚙️ 远端已自动安装 rsync');
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ✅ rsync 已就绪');
            }
            usleep(200000);

            // 2. rsync 推送 (同步整个目录内容到远程)
            $this->streamLog('[' . date('H:i:s') . '] 📦 开始同步文件...');
            
            // 构建rsync命令，注意路径处理
            $sshCmd = sprintf('ssh -i %s %s -p %d', $keyPath, $sshOpt, $server['port']);
            // rsync路径需要在escapeshellarg之前添加斜杠
            $localDirWithSlash = $localDir . '/';
            $remoteDirWithSlash = $remoteDir . '/';
            $rsyncCmd = sprintf(
                'rsync -avz --delete -e %s %s root@%s:%s 2>&1',
                escapeshellarg($sshCmd),
                escapeshellarg($localDirWithSlash),
                escapeshellarg($server['host']),
                escapeshellarg($remoteDirWithSlash)
            );
            
            // 执行rsync并实时输出
            $descriptors = [
                0 => ['pipe', 'r'],  // stdin
                1 => ['pipe', 'w'],  // stdout
                2 => ['pipe', 'w']   // stderr
            ];
            
            $process = proc_open($rsyncCmd, $descriptors, $pipes);
            
            if (is_resource($process)) {
                fclose($pipes[0]); // 关闭stdin
                
                stream_set_blocking($pipes[1], false);
                stream_set_blocking($pipes[2], false);
                
                $output = '';
                while (!feof($pipes[1]) || !feof($pipes[2])) {
                    $stdout = fgets($pipes[1]);
                    $stderr = fgets($pipes[2]);
                    
                    if ($stdout !== false && trim($stdout) !== '') {
                        $output .= $stdout;
                        $this->streamLog('[' . date('H:i:s') . '] ' . trim($stdout));
                        usleep(50000); // 50ms
                    }
                    
                    if ($stderr !== false && trim($stderr) !== '') {
                        $output .= $stderr;
                        $this->streamLog('[' . date('H:i:s') . '] ⚠️ ' . trim($stderr));
                        usleep(50000);
                    }
                    
                    usleep(10000); // 10ms循环延迟
                }
                
                fclose($pipes[1]);
                fclose($pipes[2]);
                $returnCode = proc_close($process);
                
                if ($returnCode !== 0 || strpos($output, 'rsync error') !== false || strpos($output, 'Permission denied') !== false) {
                    $this->streamLog('[' . date('H:i:s') . '] ❌ rsync 同步失败');
                    echo "data: " . json_encode(['success' => false, 'message' => 'rsync 失败']) . "\n\n";
                    flush();
                    return;
                }
                
                $this->streamLog('[' . date('H:i:s') . '] ✅ 文件同步完成');
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 无法启动rsync进程');
                echo "data: " . json_encode(['success' => false, 'message' => 'rsync 启动失败']) . "\n\n";
                flush();
                return;
            }
            usleep(200000);

            // 3. 设置权限
            $this->streamLog('[' . date('H:i:s') . '] 🔒 设置文件权限...');
            $chmodCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "chown -R root:root %s && chmod 755 %s/%s" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host']),
                escapeshellarg($remoteDir),
                escapeshellarg($remoteDir),
                $core
            );
            $output = shell_exec($chmodCmd);
            if ($output && trim($output) !== '') {
                $this->streamLog('[' . date('H:i:s') . '] ⚠️ ' . trim($output));
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ✅ 权限设置完成');
            }
            usleep(200000);

            // 4. 上传 systemd 单元文件
            $unitSrc = $localDir . '/' . $unitFile;
            if (file_exists($unitSrc)) {
                $this->streamLog('[' . date('H:i:s') . '] 📋 上传服务配置文件...');
                $scpCmd = sprintf(
                    'scp -i %s %s -P %d %s root@%s:%s 2>&1',
                    escapeshellarg($keyPath),
                    $sshOpt,
                    $server['port'],
                    escapeshellarg($unitSrc),
                    escapeshellarg($server['host']),
                    escapeshellarg($unitDst)
                );
                $output = shell_exec($scpCmd);
                if ($output && strpos($output, '100%') === false) {
                    $this->streamLog('[' . date('H:i:s') . '] ❌ 单元文件上传失败: ' . trim($output));
                    echo "data: " . json_encode(['success' => false, 'message' => '单元文件上传失败']) . "\n\n";
                    flush();
                    return;
                }
                $this->streamLog('[' . date('H:i:s') . '] ✅ 服务配置上传成功');
            }
            usleep(200000);

            // 5. 停止对侧服务
            $otherService = $core === 'sing-box' ? 'mihomo' : 'sing-box';
            $this->streamLog('[' . date('H:i:s') . '] � 检查对侧服务 ' . $otherService . '...');
            
            // 先检查对侧服务状态
            $checkCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "systemctl is-active %s 2>/dev/null || echo inactive" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host']),
                $otherService
            );
            $otherStatus = trim(shell_exec($checkCmd) ?? 'inactive');
            
            if ($otherStatus === 'active') {
                $this->streamLog('[' . date('H:i:s') . '] �🛑 停止运行中的 ' . $otherService . '...');
                $stopCmd = sprintf(
                    'ssh -i %s %s -p %d root@%s "systemctl disable %s && systemctl stop %s 2>/dev/null || true" 2>&1',
                    escapeshellarg($keyPath),
                    $sshOpt,
                    $server['port'],
                    escapeshellarg($server['host']),
                    $otherService,
                    $otherService
                );
                shell_exec($stopCmd);
                $this->streamLog('[' . date('H:i:s') . '] ✅ 已停止 ' . $otherService);
                usleep(300000);
            } else {
                $this->streamLog('[' . date('H:i:s') . '] ⏭️ ' . $otherService . ' 未运行，跳过停止');
                usleep(100000);
            }

            // 6. 重启当前服务
            $this->streamLog('[' . date('H:i:s') . '] 🔄 重新加载systemd...');
            usleep(200000);
            
            $this->streamLog('[' . date('H:i:s') . '] 🚀 启动 ' . $core . ' 服务...');
            $restartCmd = sprintf(
                'ssh -i %s %s -p %d root@%s "systemctl daemon-reload && systemctl enable %s && systemctl restart %s && systemctl is-active %s" 2>&1',
                escapeshellarg($keyPath),
                $sshOpt,
                $server['port'],
                escapeshellarg($server['host']),
                $core,
                $core,
                $core
            );
            
            $status = trim(shell_exec($restartCmd) ?? '');
            if ($status !== 'active') {
                $this->streamLog('[' . date('H:i:s') . '] ❌ 服务启动失败: ' . $status);
                echo "data: " . json_encode(['success' => false, 'message' => '服务启动失败']) . "\n\n";
                flush();
                return;
            }

            $this->streamLog('[' . date('H:i:s') . '] ✅ 服务 ' . $core . ' 已成功启动');
            usleep(200000);

            // 更新最后部署时间
            $updateStmt = $db->prepare("UPDATE servers SET last_deploy_at = CURRENT_TIMESTAMP WHERE id = ?");
            $updateStmt->bindValue(1, $id, SQLITE3_INTEGER);
            $updateStmt->execute();

            $this->streamLog('[' . date('H:i:s') . '] 🎉 目录部署全部完成!');
            echo "data: " . json_encode(['success' => true, 'message' => '目录部署成功', 'finished' => true]) . "\n\n";
            flush();
        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '目录部署失败: ' . $e->getMessage()
            ], 500);
        }
    }
}
