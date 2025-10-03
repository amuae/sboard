<?php
/**
 * 路由配置文件
 */

return [
    // 前端页面路由
    'web' => [
        '/' => ['AuthController', 'showLogin'],
        '/install' => ['InstallController', 'index'],
        '/dashboard' => ['DashboardController', 'index'],
        '/users' => ['UserController', 'index'],
        '/nodes' => ['NodeController', 'index'],
        '/servers' => ['ServerController', 'index'],
        '/settings' => ['SettingController', 'index'],
        '/login' => ['AuthController', 'showLogin'],
        '/logout' => ['AuthController', 'logout'],
    ],

    // API 路由
    'api' => [
        // 认证
        'POST /api/auth/login' => ['AuthController', 'apiLogin'],
        'POST /api/auth/logout' => ['AuthController', 'apiLogout'],
        'POST /api/auth/change-password' => ['AuthController', 'changePassword'],
        'POST /api/auth/change-username' => ['AuthController', 'changeUsername'],

        // 用户管理
        'GET /api/users' => ['UserController', 'list'],
        'POST /api/users' => ['UserController', 'create'],
        'PUT /api/users/{id}' => ['UserController', 'update'],
        'DELETE /api/users/{id}' => ['UserController', 'delete'],
        'POST /api/users/sync' => ['UserController', 'syncConfigs'],

        // 节点管理
        'GET /api/nodes' => ['NodeController', 'list'],
        'POST /api/nodes' => ['NodeController', 'create'],
        'PUT /api/nodes/{id}' => ['NodeController', 'update'],
        'DELETE /api/nodes/{id}' => ['NodeController', 'delete'],
        'POST /api/nodes/generate-reality-keys' => ['NodeController', 'generateRealityKeys'],

        // 服务器管理
        'GET /api/servers' => ['ServerController', 'list'],
        'POST /api/servers' => ['ServerController', 'create'],
        'PUT /api/servers/{id}' => ['ServerController', 'update'],
        'DELETE /api/servers/{id}' => ['ServerController', 'delete'],
        'GET /api/servers/{id}/deploy' => ['ServerController', 'deploySingle'],  // SSE流式响应
        'GET /api/servers/{id}/deploy-folder' => ['ServerController', 'deployFolder'],  // SSE流式响应
        'POST /api/servers/{id}/check' => ['ServerController', 'checkStatus'],

        // 配置同步
        'POST /api/config/sync' => ['ConfigController', 'sync'],
        'GET /api/config/preview' => ['ConfigController', 'preview'],

        // 系统设置
        'GET /api/settings' => ['SettingController', 'get'],
        'POST /api/settings' => ['SettingController', 'update'],

        // 安装相关
        'GET /api/install/check-environment' => ['InstallController', 'checkEnvironment'],
        'POST /api/install/upload-ssh-key' => ['InstallController', 'uploadSshKey'],
        'POST /api/install/database' => ['InstallController', 'installDatabase'],
    ],

    // 订阅路由
    'subscribe' => [
        'GET /subscribe/{uuid}' => ['SubscribeController', 'generate'],
        'GET /sub/{uuid}' => ['SubscribeController', 'generate'],
    ],
];
