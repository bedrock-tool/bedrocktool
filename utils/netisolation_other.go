//go:build !windows

package utils

func Netisolation() error {
	return nil
}
