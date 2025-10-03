<?php
/**
 * 数据库配置文件
 */

return [
    'driver' => 'sqlite',
    'database' => dirname(__DIR__) . '/database/sboard.db',
    'options' => [
        'timeout' => 5000,
        'foreign_keys' => true,
    ],
];
