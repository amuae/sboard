<?php
/**
 * 响应处理类
 */

namespace App\Core;

class Response
{
    /**
     * 返回 JSON 响应
     */
    public static function json(array $data, int $status = 200): void
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        echo json_encode($data, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
        exit;
    }

    /**
     * 返回成功响应
     */
    public static function success($data = null, string $message = 'Success', int $code = 200): void
    {
        self::json([
            'success' => true,
            'message' => $message,
            'data' => $data,
            'code' => $code,
        ], $code);
    }

    /**
     * 返回错误响应
     */
    public static function error(string $message = 'Error', int $code = 400, $errors = null): void
    {
        $response = [
            'success' => false,
            'message' => $message,
            'code' => $code,
        ];

        if ($errors !== null) {
            $response['errors'] = $errors;
        }

        self::json($response, $code);
    }

    /**
     * 返回 HTML 响应
     */
    public static function html(string $content, int $status = 200): void
    {
        http_response_code($status);
        header('Content-Type: text/html; charset=utf-8');
        echo $content;
        exit;
    }

    /**
     * 返回纯文本响应
     */
    public static function text(string $content, int $status = 200): void
    {
        http_response_code($status);
        header('Content-Type: text/plain; charset=utf-8');
        echo $content;
        exit;
    }

    /**
     * 重定向
     */
    public static function redirect(string $url, int $status = 302): void
    {
        http_response_code($status);
        header("Location: $url");
        exit;
    }

    /**
     * 返回未授权响应
     */
    public static function unauthorized(string $message = 'Unauthorized'): void
    {
        self::error($message, 401);
    }

    /**
     * 返回禁止访问响应
     */
    public static function forbidden(string $message = 'Forbidden'): void
    {
        self::error($message, 403);
    }

    /**
     * 返回未找到响应
     */
    public static function notFound(string $message = 'Not Found'): void
    {
        self::error($message, 404);
    }

    /**
     * 返回服务器错误响应
     */
    public static function serverError(string $message = 'Internal Server Error'): void
    {
        self::error($message, 500);
    }
}
