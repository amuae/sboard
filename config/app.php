<?php
/**
 * 应用配置文件
 */

return [
    // 应用基本信息
    'app' => [
        'name' => 'SBoard',
        'version' => '2.0.0',
        'debug' => true,
        'timezone' => 'Asia/Shanghai',
        'charset' => 'UTF-8',
    ],

    // 路径配置
    'paths' => [
        'root' => dirname(__DIR__),
        'public' => dirname(__DIR__) . '/public',
        'storage' => dirname(__DIR__) . '/storage',
        'configs' => dirname(__DIR__) . '/storage/configs',
        'logs' => dirname(__DIR__) . '/storage/logs',
        'templates' => dirname(__DIR__) . '/templates',
    ],

    // 会话配置
    'session' => [
        'name' => 'SBOARD_SESSION',
        'lifetime' => 7200, // 2小时
        'path' => '/',
        'domain' => '',
        'secure' => false,
        'httponly' => true,
        'samesite' => 'Lax',
    ],

    // 安全配置
    'security' => [
        'password_algo' => PASSWORD_BCRYPT,
        'password_cost' => 10,
        'csrf_token_length' => 32,
    ],

    // 核心配置
    'cores' => [
        'sing-box' => [
            'binary' => '/root/sing-box/sing-box',
            'config_dir' => '/root/sing-box',
            'config_file' => 'config.json',
            'service' => 'sing-box',
        ],
        'mihomo' => [
            'binary' => '/root/mihomo/mihomo',
            'config_dir' => '/root/mihomo',
            'config_file' => 'config.yaml',
            'service' => 'mihomo',
        ],
    ],

    // SSH 配置
    'ssh' => [
        'default_user' => 'root',
        'default_port' => 22,
        'key_path' => '/var/www/.ssh/id_rsa',
        'timeout' => 30,
        'strict_host_key_checking' => false,
    ],

    // 订阅配置
    'subscription' => [
        'base_url' => 'http://localhost',
        'user_agent_detection' => true,
        'default_client' => 'sing-box',
    ],

    // 用户默认配置
    'user_defaults' => [
        'level' => 1,
        'dns_resolve' => 0,
        'expiry_days' => 30,
        'traffic_limit' => 0,
    ],
];
