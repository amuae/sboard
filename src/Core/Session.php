<?php
/**
 * 会话管理类
 */

namespace App\Core;

class Session
{
    private static bool $started = false;

    /**
     * 启动会话
     */
    public static function start(): void
    {
        if (self::$started) {
            return;
        }

        $config = Config::get('session', []);

        if (isset($config['name'])) {
            session_name($config['name']);
        }

        $options = [];
        if (isset($config['lifetime'])) {
            $options['cookie_lifetime'] = $config['lifetime'];
        }
        if (isset($config['path'])) {
            $options['cookie_path'] = $config['path'];
        }
        if (isset($config['domain'])) {
            $options['cookie_domain'] = $config['domain'];
        }
        if (isset($config['secure'])) {
            $options['cookie_secure'] = $config['secure'];
        }
        if (isset($config['httponly'])) {
            $options['cookie_httponly'] = $config['httponly'];
        }
        if (isset($config['samesite'])) {
            $options['cookie_samesite'] = $config['samesite'];
        }

        if (!empty($options)) {
            session_start($options);
        } else {
            session_start();
        }

        self::$started = true;
    }

    /**
     * 设置会话值
     */
    public static function set(string $key, $value): void
    {
        self::start();
        $_SESSION[$key] = $value;
    }

    /**
     * 获取会话值
     */
    public static function get(string $key, $default = null)
    {
        self::start();
        return $_SESSION[$key] ?? $default;
    }

    /**
     * 判断会话键是否存在
     */
    public static function has(string $key): bool
    {
        self::start();
        return isset($_SESSION[$key]);
    }

    /**
     * 删除会话值
     */
    public static function delete(string $key): void
    {
        self::start();
        unset($_SESSION[$key]);
    }

    /**
     * 清空所有会话
     */
    public static function clear(): void
    {
        self::start();
        $_SESSION = [];
    }

    /**
     * 销毁会话
     */
    public static function destroy(): void
    {
        self::start();
        $_SESSION = [];
        session_destroy();
        self::$started = false;
    }

    /**
     * 重新生成会话ID
     */
    public static function regenerate(bool $deleteOld = true): void
    {
        self::start();
        session_regenerate_id($deleteOld);
    }

    /**
     * 获取会话ID
     */
    public static function id(): string
    {
        self::start();
        return session_id();
    }

    /**
     * 设置闪存消息
     */
    public static function flash(string $key, $value): void
    {
        self::set('_flash_' . $key, $value);
    }

    /**
     * 获取并删除闪存消息
     */
    public static function getFlash(string $key, $default = null)
    {
        self::start();
        $value = $_SESSION['_flash_' . $key] ?? $default;
        unset($_SESSION['_flash_' . $key]);
        return $value;
    }

    /**
     * 判断是否有闪存消息
     */
    public static function hasFlash(string $key): bool
    {
        self::start();
        return isset($_SESSION['_flash_' . $key]);
    }
}
