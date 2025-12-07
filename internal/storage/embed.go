package storage

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:configs
var StorageFS embed.FS

// ExtractStorage 将嵌入的 storage 目录释放到运行目录
func ExtractStorage(targetDir string) error {
	// 如果目标目录已存在，跳过释放
	if _, err := os.Stat(targetDir); err == nil {
		return nil
	}

	return fs.WalkDir(StorageFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录
		if path == "." {
			return nil
		}

		targetPath := filepath.Join(targetDir, path)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// 读取嵌入的文件
		data, err := StorageFS.ReadFile(path)
		if err != nil {
			return err
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// 写入文件
		return os.WriteFile(targetPath, data, 0644)
	})
}

// EnsureStorageDirs 确保 storage 目录结构存在（不覆盖已有文件）
func EnsureStorageDirs(baseDir string) error {
	dirs := []string{
		filepath.Join(baseDir, "configs", "sing-box"),
		filepath.Join(baseDir, "configs", "mihomo"),
		filepath.Join(baseDir, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
