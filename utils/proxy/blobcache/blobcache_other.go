//go:build !windows

package blobcache

func checkShouldReadOnly(err error) bool {
	return false
}
