package proxy

import "golang.org/x/sys/windows"

func checkShouldReadOnly(err error) bool {
	return err == windows.ERROR_SHARING_VIOLATION
}
