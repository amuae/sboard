<?php
/**
 * 数据库连接类
 */

namespace App\Core;

use SQLite3;

class Database
{
    private static ?SQLite3 $instance = null;

    /**
     * 获取数据库连接实例
     */
    public static function getInstance(): SQLite3
    {
        if (self::$instance === null) {
            $config = Config::get('database');
            self::$instance = new SQLite3($config['database']);
            
            // 设置选项
            if (isset($config['options'])) {
                if (isset($config['options']['timeout'])) {
                    self::$instance->busyTimeout($config['options']['timeout']);
                }
                if (isset($config['options']['foreign_keys']) && $config['options']['foreign_keys']) {
                    self::$instance->exec('PRAGMA foreign_keys = ON');
                }
            }
        }

        return self::$instance;
    }

    /**
     * 执行查询
     */
    public static function query(string $sql)
    {
        return self::getInstance()->query($sql);
    }

    /**
     * 执行语句
     */
    public static function exec(string $sql): bool
    {
        return self::getInstance()->exec($sql);
    }

    /**
     * 准备语句
     */
    public static function prepare(string $sql)
    {
        return self::getInstance()->prepare($sql);
    }

    /**
     * 获取最后插入的ID
     */
    public static function lastInsertRowID(): int
    {
        return self::getInstance()->lastInsertRowID();
    }

    /**
     * 转义字符串
     */
    public static function escape(string $string): string
    {
        return self::getInstance()->escapeString($string);
    }

    /**
     * 开始事务
     */
    public static function beginTransaction(): bool
    {
        return self::getInstance()->exec('BEGIN TRANSACTION');
    }

    /**
     * 提交事务
     */
    public static function commit(): bool
    {
        return self::getInstance()->exec('COMMIT');
    }

    /**
     * 回滚事务
     */
    public static function rollback(): bool
    {
        return self::getInstance()->exec('ROLLBACK');
    }

    /**
     * 关闭连接
     */
    public static function close(): void
    {
        if (self::$instance !== null) {
            self::$instance->close();
            self::$instance = null;
        }
    }
}
