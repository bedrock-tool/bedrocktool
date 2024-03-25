package utils

import "os/exec"

func OpenUrl(uri string) error {
	return exec.Command("xdg-open", uri).Run()
}
