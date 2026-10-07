package handlers

import (
	"fmt"
	"os"
	"path/filepath"
)

// prepareCaptureDirectory also tightens an existing directory. Do not follow
// symlinks: a capture directory redirect must not expose authentication traffic.
func prepareCaptureDirectory(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := mkdirPrivateCaptureDirectory(path); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("capture directory must be a real directory: %s", path)
	}
	return restrictCaptureDirectory(path)
}
