package pages

import "os/exec"

func openUrl(uri string) error {
	return exec.Command("xdg-open", uri).Run()
}
