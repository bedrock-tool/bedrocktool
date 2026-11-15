//go:build !android

package updater

import (
	"crypto"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/minio/selfupdate"
)

// releasesURL is where updates come from: the latest upstream release.
const releasesURL = "https://api.github.com/repos/bedrock-tool/bedrocktool/releases/latest"

// RestartsAfterUpdate: the PC build replaces its own executable, then
// Restart starts the new one.
const RestartsAfterUpdate = true

// assetMatches says whether a release file is this build's executable.
func assetMatches(name string) bool {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return strings.HasPrefix(name, fmt.Sprintf("%s-%s-%s-", utils.CmdName, runtime.GOOS, runtime.GOARCH)) &&
		strings.HasSuffix(name, suffix)
}

func downloadPath() string {
	return utils.PathCache("bedrocktool-update.bin")
}

// install replaces the running executable with the downloaded one.
func install(path string, checksum []byte) error {
	defer os.Remove(path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return selfupdate.Apply(f, selfupdate.Options{
		Checksum: checksum,
		Hash:     crypto.SHA256,
	})
}

// Restart starts the updated executable; the caller exits.
func Restart() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}

	process := exec.Command(executable, os.Args[1:]...)
	process.Dir, err = os.Getwd()
	if err != nil {
		return err
	}
	return process.Start()
}
