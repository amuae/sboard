<?php
/**
 * 自动加载器
 */

spl_autoload_register(function ($class) {
    // 将命名空间转换为文件路径
    $prefix = 'App\\';
    $baseDir = __DIR__ . '/';

    // 检查类是否使用该命名空间前缀
    $len = strlen($prefix);
    if (strncmp($prefix, $class, $len) !== 0) {
        return;
    }

    // 获取相对类名
    $relativeClass = substr($class, $len);

    // 将命名空间分隔符替换为目录分隔符，并添加 .php
    $file = $baseDir . str_replace('\\', '/', $relativeClass) . '.php';

    // 如果文件存在，加载它
    if (file_exists($file)) {
        require $file;
    }
});
