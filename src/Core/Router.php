<?php
/**
 * 路由器类
 */

namespace App\Core;

class Router
{
    private array $routes = [];
    private Request $request;

    public function __construct(Request $request)
    {
        $this->request = $request;
        $this->loadRoutes();
    }

    /**
     * 加载路由配置
     */
    private function loadRoutes(): void
    {
        $routes = Config::get('routes', []);

        // 加载 Web 路由
        if (isset($routes['web'])) {
            foreach ($routes['web'] as $path => $handler) {
                $this->routes['GET'][$path] = $handler;
            }
        }

        // 加载 API 路由
        if (isset($routes['api'])) {
            foreach ($routes['api'] as $route => $handler) {
                [$method, $path] = explode(' ', $route, 2);
                $this->routes[$method][$path] = $handler;
            }
        }

        // 加载订阅路由
        if (isset($routes['subscribe'])) {
            foreach ($routes['subscribe'] as $route => $handler) {
                [$method, $path] = explode(' ', $route, 2);
                $this->routes[$method][$path] = $handler;
            }
        }
    }

    /**
     * 分发请求
     */
    public function dispatch(): void
    {
        $method = $this->request->method();
        $path = $this->request->path();

        // 检查安装状态
        if (!$this->isInstalled() && !$this->isInstallRoute($path)) {
            Response::redirect('/install');
            return;
        }

        // 如果已安装且访问安装页面，重定向到登录页
        if ($this->isInstalled() && $this->isInstallRoute($path)) {
            Response::redirect('/login');
            return;
        }

        // 匹配路由
        $handler = $this->match($method, $path);

        if ($handler === null) {
            Response::notFound('Route not found');
            return;
        }

        // 执行控制器方法
        $this->execute($handler);
    }

    /**
     * 匹配路由
     */
    private function match(string $method, string $path): ?array
    {
        // 精确匹配
        if (isset($this->routes[$method][$path])) {
            return ['handler' => $this->routes[$method][$path], 'params' => []];
        }

        // 参数匹配
        if (isset($this->routes[$method])) {
            foreach ($this->routes[$method] as $route => $handler) {
                $pattern = preg_replace('/\{([a-zA-Z0-9_]+)\}/', '(?P<$1>[^/]+)', $route);
                $pattern = '#^' . $pattern . '$#';

                if (preg_match($pattern, $path, $matches)) {
                    $params = array_filter($matches, 'is_string', ARRAY_FILTER_USE_KEY);
                    return ['handler' => $handler, 'params' => $params];
                }
            }
        }

        return null;
    }

    /**
     * 执行控制器方法
     */
    private function execute(array $handler): void
    {
        [$controllerName, $method] = $handler['handler'];
        $params = $handler['params'] ?? [];

        // 构造完整的控制器类名
        $controllerClass = "App\\Controllers\\{$controllerName}";

        if (!class_exists($controllerClass)) {
            Response::serverError("Controller {$controllerClass} not found");
            return;
        }

        $controller = new $controllerClass($this->request);

        if (!method_exists($controller, $method)) {
            Response::serverError("Method {$method} not found in {$controllerClass}");
            return;
        }

        // 调用控制器方法
        call_user_func_array([$controller, $method], $params);
    }

    /**
     * 检查是否已安装
     */
    private function isInstalled(): bool
    {
        $lockFile = dirname(__DIR__, 2) . '/storage/.installed';
        return file_exists($lockFile);
    }

    /**
     * 检查是否为安装相关路由
     */
    private function isInstallRoute(string $path): bool
    {
        return strpos($path, '/install') === 0 || strpos($path, '/api/install') === 0;
    }
}
