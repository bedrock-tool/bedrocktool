package utils

import (
	"syscall"
)

func RemoveDir(name string) error {
	return syscall.Rmdir(name)
}

func RemoveFile(name string) error {
	return syscall.Unlink(name)
}
