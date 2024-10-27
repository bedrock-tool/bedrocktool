//go:build !windows

package proxy

func checkShouldReadOnly(err error) bool {
	return false
}
