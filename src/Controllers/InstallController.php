<?php
/**
 * 安装控制器
 */

namespace App\Controllers;

use App\Core\Database;
use App\Core\Response;

class InstallController extends BaseController
{
    /**
     * 显示安装页面
     */
    public function index(): void
    {
        // 检查是否已安装
        if ($this->isInstalled()) {
            Response::redirect('/login');
            return;
        }

        $this->view('install/index', [
            'step' => 'environment'
        ]);
    }

    /**
     * 环境检查 API
     */
    public function checkEnvironment(): void
    {
        $checks = [
            'php_version' => $this->checkPhpVersion(),
            'sqlite' => $this->checkSqlite(),
            'json' => $this->checkJson(),
            'openssl' => $this->checkOpenssl(),
            'curl' => $this->checkCurl(),
            'file_permissions' => $this->checkFilePermissions(),
            'ssh_keys' => $this->checkSshKeys()
        ];

        $allPassed = array_reduce($checks, function($carry, $check) {
            return $carry && $check['passed'];
        }, true);

        Response::json([
            'success' => true,
            'checks' => $checks,
            'all_passed' => $allPassed
        ]);
    }

    /**
     * SSH 密钥上传
     */
    public function uploadSshKey(): void
    {
        if (!isset($_FILES['ssh_key']) || $_FILES['ssh_key']['error'] !== UPLOAD_ERR_OK) {
            Response::json([
                'success' => false,
                'message' => 'SSH 密钥文件上传失败'
            ], 400);
            return;
        }

        $uploadedFile = $_FILES['ssh_key'];
        $keyContent = file_get_contents($uploadedFile['tmp_name']);

        // 验证私钥格式
        if (!$this->validatePrivateKey($keyContent)) {
            Response::json([
                'success' => false,
                'message' => 'SSH 私钥格式无效'
            ], 400);
            return;
        }

        // 创建 SSH 目录
        $sshDir = '/var/www/.ssh';
        if (!is_dir($sshDir)) {
            if (!mkdir($sshDir, 0700, true)) {
                Response::json([
                    'success' => false,
                    'message' => '无法创建 SSH 目录'
                ], 500);
                return;
            }
        }

        // 保存私钥
        $keyPath = $sshDir . '/id_rsa';
        if (file_put_contents($keyPath, $keyContent) === false) {
            Response::json([
                'success' => false,
                'message' => '无法保存 SSH 私钥'
            ], 500);
            return;
        }

        // 设置权限
        chmod($keyPath, 0600);

        // 如果有公钥文件，也上传
        if (isset($_FILES['ssh_pub']) && $_FILES['ssh_pub']['error'] === UPLOAD_ERR_OK) {
            $pubContent = file_get_contents($_FILES['ssh_pub']['tmp_name']);
            $pubPath = $sshDir . '/id_rsa.pub';
            file_put_contents($pubPath, $pubContent);
            chmod($pubPath, 0644);
        }

        Response::json([
            'success' => true,
            'message' => 'SSH 密钥上传成功'
        ]);
    }

    /**
     * 数据库安装
     */
    public function installDatabase(): void
    {
        try {
            $adminUsername = trim($this->request->post('admin_username', ''));
            $adminPassword = $this->request->post('admin_password', '');

            if (empty($adminUsername) || empty($adminPassword)) {
                Response::json([
                    'success' => false,
                    'message' => '管理员用户名和密码不能为空'
                ], 400);
                return;
            }

            // 初始化数据库
            $this->initializeDatabase();

            // 创建管理员账户
            $this->createAdminUser($adminUsername, $adminPassword);

            // 标记安装完成
            $this->markInstalled();

            Response::json([
                'success' => true,
                'message' => '数据库安装成功'
            ]);

        } catch (\Exception $e) {
            Response::json([
                'success' => false,
                'message' => '数据库安装失败: ' . $e->getMessage()
            ], 500);
        }
    }

    /**
     * 检查 PHP 版本
     */
    private function checkPhpVersion(): array
    {
        $version = PHP_VERSION;
        $passed = version_compare($version, '8.0.0', '>=');
        
        return [
            'name' => 'PHP 版本',
            'required' => '8.0.0 或更高',
            'current' => $version,
            'passed' => $passed,
            'message' => $passed ? '版本符合要求' : 'PHP 版本过低，需要 8.0.0 或更高版本'
        ];
    }

    /**
     * 检查 SQLite 扩展
     */
    private function checkSqlite(): array
    {
        $passed = extension_loaded('sqlite3');
        
        return [
            'name' => 'SQLite3 扩展',
            'required' => '已启用',
            'current' => $passed ? '已启用' : '未启用',
            'passed' => $passed,
            'message' => $passed ? 'SQLite3 扩展已启用' : '需要启用 SQLite3 扩展'
        ];
    }

    /**
     * 检查 JSON 扩展
     */
    private function checkJson(): array
    {
        $passed = extension_loaded('json');
        
        return [
            'name' => 'JSON 扩展',
            'required' => '已启用',
            'current' => $passed ? '已启用' : '未启用',
            'passed' => $passed,
            'message' => $passed ? 'JSON 扩展已启用' : '需要启用 JSON 扩展'
        ];
    }

    /**
     * 检查 OpenSSL 扩展
     */
    private function checkOpenssl(): array
    {
        $passed = extension_loaded('openssl');
        
        return [
            'name' => 'OpenSSL 扩展',
            'required' => '已启用',
            'current' => $passed ? '已启用' : '未启用',
            'passed' => $passed,
            'message' => $passed ? 'OpenSSL 扩展已启用' : '需要启用 OpenSSL 扩展'
        ];
    }

    /**
     * 检查 cURL 扩展
     */
    private function checkCurl(): array
    {
        $passed = extension_loaded('curl');
        
        return [
            'name' => 'cURL 扩展',
            'required' => '已启用',
            'current' => $passed ? '已启用' : '未启用',
            'passed' => $passed,
            'message' => $passed ? 'cURL 扩展已启用' : '需要启用 cURL 扩展'
        ];
    }

    /**
     * 检查文件权限
     */
    private function checkFilePermissions(): array
    {
        $paths = [
            dirname(__DIR__, 2) . '/database',
            dirname(__DIR__, 2) . '/storage',
            dirname(__DIR__, 2) . '/storage/logs',
            dirname(__DIR__, 2) . '/storage/configs'
        ];

        $issues = [];
        foreach ($paths as $path) {
            if (!is_dir($path)) {
                if (!mkdir($path, 0755, true)) {
                    $issues[] = "无法创建目录: $path";
                }
            } elseif (!is_writable($path)) {
                $issues[] = "目录不可写: $path";
            }
        }

        $passed = empty($issues);
        
        return [
            'name' => '文件权限',
            'required' => '目录可读写',
            'current' => $passed ? '权限正常' : '权限不足',
            'passed' => $passed,
            'message' => $passed ? '所有必需目录权限正常' : implode('; ', $issues)
        ];
    }

    /**
     * 检查 SSH 密钥
     */
    private function checkSshKeys(): array
    {
        $sshDir = '/var/www/.ssh';
        $keyPath = $sshDir . '/id_rsa';
        
        $exists = file_exists($keyPath);
        $readable = $exists && is_readable($keyPath);
        
        return [
            'name' => 'SSH 密钥',
            'required' => '私钥文件存在',
            'current' => $exists ? '已配置' : '未配置',
            'passed' => $exists,
            'message' => $exists ? 'SSH 私钥已配置' : '需要上传 SSH 私钥文件'
        ];
    }

    /**
     * 验证私钥格式
     */
    private function validatePrivateKey(string $keyContent): bool
    {
        // 检查是否包含私钥标识
        $patterns = [
            '/-----BEGIN RSA PRIVATE KEY-----/',
            '/-----BEGIN PRIVATE KEY-----/',
            '/-----BEGIN OPENSSH PRIVATE KEY-----/'
        ];

        foreach ($patterns as $pattern) {
            if (preg_match($pattern, $keyContent)) {
                return true;
            }
        }

        return false;
    }

    /**
     * 初始化数据库
     */
    private function initializeDatabase(): void
    {
        $sqlFile = dirname(__DIR__, 2) . '/database/migrations/init.sql';
        if (!file_exists($sqlFile)) {
            throw new \Exception('数据库初始化脚本不存在');
        }

        $sql = file_get_contents($sqlFile);
        if ($sql === false) {
            throw new \Exception('无法读取数据库初始化脚本');
        }

        $db = Database::getInstance();
        
        // 检查表是否已存在
        $tableExists = $db->querySingle("SELECT name FROM sqlite_master WHERE type='table' AND name='admins'");
        if ($tableExists) {
            // 表已存在，跳过初始化
            return;
        }
        
        Database::beginTransaction();
        
        try {
            // 使用 SQLite 的多语句执行
            $result = $db->exec($sql);
            if ($result === false) {
                $error = $db->lastErrorMsg();
                throw new \Exception('数据库初始化失败: ' . $error);
            }
            
            Database::commit();
        } catch (\Exception $e) {
            Database::rollback();
            throw new \Exception('数据库初始化失败: ' . $e->getMessage());
        }
    }

    /**
     * 创建管理员用户
     */
    private function createAdminUser(string $username, string $password): void
    {
        $db = Database::getInstance();
        
        // 检查是否已有管理员用户
        $existingCount = $db->querySingle('SELECT COUNT(*) FROM admins');
        if ($existingCount > 0) {
            // 如果已有管理员，更新第一个管理员的密码
            $hashedPassword = password_hash($password, PASSWORD_DEFAULT);
            $stmt = $db->prepare('UPDATE admins SET username = ?, password = ?, updated_at = datetime("now") WHERE id = (SELECT id FROM admins LIMIT 1)');
            $stmt->bindValue(1, $username, SQLITE3_TEXT);
            $stmt->bindValue(2, $hashedPassword, SQLITE3_TEXT);
            
            if (!$stmt->execute()) {
                throw new \Exception('更新管理员用户失败');
            }
        } else {
            // 创建新的管理员用户
            $hashedPassword = password_hash($password, PASSWORD_DEFAULT);
            $stmt = $db->prepare('INSERT INTO admins (username, password, email, created_at, updated_at) VALUES (?, ?, ?, datetime("now"), datetime("now"))');
            $stmt->bindValue(1, $username, SQLITE3_TEXT);
            $stmt->bindValue(2, $hashedPassword, SQLITE3_TEXT);
            $stmt->bindValue(3, '', SQLITE3_TEXT);
            
            if (!$stmt->execute()) {
                throw new \Exception('创建管理员用户失败');
            }
        }
    }

    /**
     * 标记安装完成
     */
    private function markInstalled(): void
    {
        $lockFile = dirname(__DIR__, 2) . '/storage/.installed';
        if (file_put_contents($lockFile, date('Y-m-d H:i:s')) === false) {
            throw new \Exception('无法创建安装锁定文件');
        }
    }

    /**
     * 检查是否已安装
     */
    private function isInstalled(): bool
    {
        $lockFile = dirname(__DIR__, 2) . '/storage/.installed';
        return file_exists($lockFile);
    }
}