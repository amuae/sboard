<?php
/**
 * 配置管理类
 */

namespace App\Core;

class Config
{
    private static $config = [];
    private static $loaded = false;

    /**
     * 加载所有配置文件
     */
    public static function load(): void
    {
        if (self::$loaded) {
            return;
        }

        $configPath = dirname(__DIR__, 2) . '/config';
        $files = ['app', 'database', 'routes'];

        foreach ($files as $file) {
            $path = $configPath . '/' . $file . '.php';
            if (file_exists($path)) {
                self::$config[$file] = require $path;
            }
        }

        self::$loaded = true;
    }

    /**
     * 获取配置值
     * 
     * @param string $key 配置键，支持点号分隔
     * @param mixed $default 默认值
     * @return mixed
     */
    public static function get(string $key, $default = null)
    {
        self::load();

        $keys = explode('.', $key);
        $value = self::$config;

        foreach ($keys as $k) {
            if (!isset($value[$k])) {
                return $default;
            }
            $value = $value[$k];
        }

        return $value;
    }

    /**
     * 设置配置值
     */
    public static function set(string $key, $value): void
    {
        self::load();

        $keys = explode('.', $key);
        $config = &self::$config;

        foreach ($keys as $k) {
            if (!isset($config[$k])) {
                $config[$k] = [];
            }
            $config = &$config[$k];
        }

        $config = $value;
    }

    /**
     * 获取所有配置
     */
    public static function all(): array
    {
        self::load();
        return self::$config;
    }
}
