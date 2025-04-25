//go:build !windows

package utils

import "os"

func CreateShared(name string) (*os.File, error) {
	return os.Create(name)
}

func OpenShared(name string) (*os.File, error) {
	return os.Open(name)
}
