package utils

import (
	"os"
	"syscall"
)

func RemoveDir(name string) error {
	p, e := syscall.UTF16PtrFromString(name)
	if e != nil {
		return &os.PathError{Op: "remove", Path: name, Err: e}
	}
	return syscall.RemoveDirectory(p)
}

func RemoveFile(name string) error {
	p, e := syscall.UTF16PtrFromString(name)
	if e != nil {
		return &os.PathError{Op: "remove", Path: name, Err: e}
	}
	return syscall.DeleteFile(p)
}
