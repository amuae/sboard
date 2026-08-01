//go:build windows

package main

import "os"

func applyFileOwnership(_ string, _ os.FileInfo) error {
	return nil
}
