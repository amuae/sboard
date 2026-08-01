//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"syscall"
)

func applyFileOwnership(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("无法读取文件属主")
	}
	return os.Chown(path, int(stat.Uid), int(stat.Gid))
}
