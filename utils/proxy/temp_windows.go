//go:build windows

package proxy

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func createShared(name string) (*os.File, error) {
	path, _ := syscall.UTF16PtrFromString(name)
	hand, err := windows.CreateFile(path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.CREATE_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(hand), name), nil
}

func openShared(name string) (*os.File, error) {
	path, _ := syscall.UTF16PtrFromString(name)
	hand, err := windows.CreateFile(path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(hand), name), nil
}
