//go:build !windows

package proxy

import "os"

func createShared(name string) (*os.File, error) {
	return os.Create(name)
}

func openShared(name string) (*os.File, error) {
	return os.Open(name)
}
