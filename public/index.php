<?php
/**
 * SBoard - 代理配置管理系统
 * 入口文件
 */

// 错误报告
error_reporting(E_ALL);
ini_set('display_errors', 1);

// 设置时区
date_default_timezone_set('Asia/Shanghai');

// 自动加载
require_once __DIR__ . '/../src/autoload.php';

// 加载配置
use App\Core\Config;
use App\Core\Request;
use App\Core\Router;
use App\Core\Session;

Config::load();

// 创建请求对象
$request = new Request();

// 创建路由器并分发请求
$router = new Router($request);
$router->dispatch();
