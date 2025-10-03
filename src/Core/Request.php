<?php
/**
 * 请求处理类
 */

namespace App\Core;

class Request
{
    private array $get;
    private array $post;
    private array $server;
    private array $files;
    private ?array $json = null;

    public function __construct()
    {
        $this->get = $_GET;
        $this->post = $_POST;
        $this->server = $_SERVER;
        $this->files = $_FILES;

        // 解析 JSON 请求体
        if ($this->isJson()) {
            $this->json = json_decode(file_get_contents('php://input'), true) ?? [];
        }
    }

    /**
     * 获取请求方法
     */
    public function method(): string
    {
        return strtoupper($this->server['REQUEST_METHOD'] ?? 'GET');
    }

    /**
     * 获取请求URI
     */
    public function uri(): string
    {
        return $this->server['REQUEST_URI'] ?? '/';
    }

    /**
     * 获取请求路径
     */
    public function path(): string
    {
        $uri = $this->uri();
        $path = parse_url($uri, PHP_URL_PATH);
        return $path ?? '/';
    }

    /**
     * 判断是否为 AJAX 请求
     */
    public function isAjax(): bool
    {
        return isset($this->server['HTTP_X_REQUESTED_WITH']) 
            && strtolower($this->server['HTTP_X_REQUESTED_WITH']) === 'xmlhttprequest';
    }

    /**
     * 判断是否为 JSON 请求
     */
    public function isJson(): bool
    {
        return isset($this->server['CONTENT_TYPE']) 
            && str_contains(strtolower($this->server['CONTENT_TYPE']), 'application/json');
    }

    /**
     * 获取 GET 参数
     */
    public function get(string $key, $default = null)
    {
        return $this->get[$key] ?? $default;
    }

    /**
     * 获取 POST 参数
     */
    public function post(string $key, $default = null)
    {
        if ($this->json !== null) {
            return $this->json[$key] ?? $default;
        }
        return $this->post[$key] ?? $default;
    }

    /**
     * 获取所有输入
     */
    public function all(): array
    {
        if ($this->json !== null) {
            return array_merge($this->get, $this->json);
        }
        return array_merge($this->get, $this->post);
    }

    /**
     * 获取指定的输入
     */
    public function only(array $keys): array
    {
        $result = [];
        $all = $this->all();
        foreach ($keys as $key) {
            if (isset($all[$key])) {
                $result[$key] = $all[$key];
            }
        }
        return $result;
    }

    /**
     * 获取 User-Agent
     */
    public function userAgent(): string
    {
        return $this->server['HTTP_USER_AGENT'] ?? '';
    }

    /**
     * 获取客户端 IP
     */
    public function ip(): string
    {
        return $this->server['REMOTE_ADDR'] ?? '';
    }

    /**
     * 获取上传的文件
     */
    public function file(string $key)
    {
        return $this->files[$key] ?? null;
    }

    /**
     * 判断是否有文件上传
     */
    public function hasFile(string $key): bool
    {
        return isset($this->files[$key]) && $this->files[$key]['error'] === UPLOAD_ERR_OK;
    }
}
