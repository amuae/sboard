<?php
/**
 * 认证控制器
 */

namespace App\Controllers;

use App\Core\Database;
use App\Core\Session;
use App\Core\Response;

class AuthController extends BaseController
{
    /**
     * 显示登录页面
     */
    public function showLogin(): void
    {
        // 如果已登录，重定向到仪表板
        if (Session::has('admin_id')) {
            $this->redirect('/dashboard');
            return;
        }

        $this->view('auth/login', [
            'error' => Session::getFlash('error'),
            'success' => Session::getFlash('success'),
        ]);
    }

    /**
     * 处理登录请求（API）
     */
    public function apiLogin(): void
    {
        $username = trim($this->request->post('username', ''));
        $password = $this->request->post('password', '');

        if (empty($username) || empty($password)) {
            $this->error('Username and password are required', 400);
            return;
        }

        $db = Database::getInstance();
        $stmt = $db->prepare('SELECT id, username, password FROM admins WHERE username = ?');
        $stmt->bindValue(1, $username, SQLITE3_TEXT);
        $result = $stmt->execute();
        $admin = $result->fetchArray(SQLITE3_ASSOC);

        if (!$admin || !password_verify($password, $admin['password'])) {
            $this->error('Invalid username or password', 401);
            return;
        }

        // 设置会话
        Session::regenerate();
        Session::set('admin_id', $admin['id']);
        Session::set('username', $admin['username']);

        $this->success([
            'redirect' => '/dashboard',
            'username' => $admin['username'],
        ], 'Login successful');
    }

    /**
     * 退出登录
     */
    public function logout(): void
    {
        Session::destroy();
        $this->redirect('/login');
    }

    /**
     * 退出登录（API）
     */
    public function apiLogout(): void
    {
        Session::destroy();
        $this->success(null, 'Logout successful');
    }

    /**
     * 修改密码
     */
    public function changePassword(): void
    {
        $this->requireAuth();

        $oldPassword = $this->request->post('old_password', '');
        $newPassword = $this->request->post('new_password', '');
        $confirmPassword = $this->request->post('confirm_password', '');

        if (empty($oldPassword) || empty($newPassword) || empty($confirmPassword)) {
            $this->error('All fields are required', 400);
            return;
        }

        if ($newPassword !== $confirmPassword) {
            $this->error('New passwords do not match', 400);
            return;
        }

        if (strlen($newPassword) < 6) {
            $this->error('Password must be at least 6 characters', 400);
            return;
        }

        $adminId = $this->getAdminId();
        $db = Database::getInstance();
        
        $stmt = $db->prepare('SELECT password FROM admins WHERE id = ?');
        $stmt->bindValue(1, $adminId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        $admin = $result->fetchArray(SQLITE3_ASSOC);

        if (!$admin || !password_verify($oldPassword, $admin['password'])) {
            $this->error('Old password is incorrect', 401);
            return;
        }

        // 更新密码
        $hashedPassword = password_hash($newPassword, PASSWORD_BCRYPT);
        $stmt = $db->prepare('UPDATE admins SET password = ? WHERE id = ?');
        $stmt->bindValue(1, $hashedPassword, SQLITE3_TEXT);
        $stmt->bindValue(2, $adminId, SQLITE3_INTEGER);
        
        if ($stmt->execute()) {
            $this->success(null, 'Password changed successfully');
        } else {
            $this->error('Failed to change password', 500);
        }
    }

    /**
     * 修改用户名
     */
    public function changeUsername(): void
    {
        $this->requireAuth();

        $password = $this->request->post('password', '');
        $newUsername = trim($this->request->post('new_username', ''));

        if (empty($password) || empty($newUsername)) {
            $this->error('Password and new username are required', 400);
            return;
        }

        if (strlen($newUsername) < 3) {
            $this->error('Username must be at least 3 characters', 400);
            return;
        }

        $adminId = $this->getAdminId();
        $db = Database::getInstance();
        
        // 验证密码
        $stmt = $db->prepare('SELECT password FROM admins WHERE id = ?');
        $stmt->bindValue(1, $adminId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        $admin = $result->fetchArray(SQLITE3_ASSOC);

        if (!$admin || !password_verify($password, $admin['password'])) {
            $this->error('Password is incorrect', 401);
            return;
        }

        // 检查用户名是否已存在
        $stmt = $db->prepare('SELECT id FROM admins WHERE username = ? AND id != ?');
        $stmt->bindValue(1, $newUsername, SQLITE3_TEXT);
        $stmt->bindValue(2, $adminId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        
        if ($result->fetchArray(SQLITE3_ASSOC)) {
            $this->error('Username already exists', 400);
            return;
        }

        // 更新用户名
        $stmt = $db->prepare('UPDATE admins SET username = ? WHERE id = ?');
        $stmt->bindValue(1, $newUsername, SQLITE3_TEXT);
        $stmt->bindValue(2, $adminId, SQLITE3_INTEGER);
        
        if ($stmt->execute()) {
            // 更新 Session
            Session::set('username', $newUsername);
            $this->success(['username' => $newUsername], 'Username changed successfully');
        } else {
            $this->error('Failed to change username', 500);
        }
    }
}
