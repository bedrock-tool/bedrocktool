//go:build !windows

package handlers

import "os"

func mkdirPrivateCaptureDirectory(path string) error { return os.Mkdir(path, 0o700) }
func restrictCaptureDirectory(path string) error     { return os.Chmod(path, 0o700) }
func createPrivateCapture(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}
