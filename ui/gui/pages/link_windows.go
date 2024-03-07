package pages

import (
	"golang.org/x/sys/windows"
)

func openUrl(uri string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(uri), nil, nil, windows.SW_SHOWNORMAL)
}
