<?php
/**
 * 基础控制器
 */

namespace App\Controllers;

use App\Core\Request;
use App\Core\Response;
use App\Core\Session;

abstract class BaseController
{
    protected Request $request;

    public function __construct(Request $request)
    {
        $this->request = $request;
        Session::start();
    }

    /**
     * 渲染视图
     */
    protected function view(string $template, array $data = []): void
    {
        extract($data);
        $templatePath = dirname(__DIR__, 2) . '/templates/' . $template . '.php';
        
        if (!file_exists($templatePath)) {
            Response::serverError("Template not found: {$template}");
            return;
        }

        ob_start();
        include $templatePath;
        $content = ob_get_clean();

        Response::html($content);
    }

    /**
     * JSON响应
     */
    protected function json($data, int $status = 200): void
    {
        Response::json($data, $status);
    }

    /**
     * 成功响应
     */
    protected function success($data = null, string $message = 'Success'): void
    {
        Response::success($data, $message);
    }

    /**
     * 错误响应
     */
    protected function error(string $message, int $code = 400): void
    {
        Response::error($message, $code);
    }

    /**
     * 重定向
     */
    protected function redirect(string $url): void
    {
        Response::redirect($url);
    }

    /**
     * 检查是否已登录
     */
    protected function requireAuth(): void
    {
        if (!Session::has('admin_id')) {
            if ($this->request->isAjax() || $this->request->isJson()) {
                Response::unauthorized('Please login first');
            } else {
                Response::redirect('/login');
            }
        }
    }

    /**
     * 获取当前登录的管理员ID
     */
    protected function getAdminId(): ?int
    {
        return Session::get('admin_id');
    }

    /**
     * 获取当前登录的用户名
     */
    protected function getUsername(): ?string
    {
        return Session::get('username');
    }
}
