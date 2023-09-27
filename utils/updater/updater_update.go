//go:build !js

package updater

import (
	"compress/gzip"
	"crypto"
	"encoding/base64"
	"fmt"
	"runtime"

	"github.com/minio/selfupdate"
)

func DoUpdate() error {
	update, err := UpdateAvailable()
	if err != nil {
		return err
	}

	checksum, err := base64.StdEncoding.DecodeString(update.Sha256)
	if err != nil {
		return err
	}

	r, err := fetch(fmt.Sprintf("%s%s/%s/%s-%s.gz", UpdateServer, CmdName, update.Version, runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return err
	}
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	err = selfupdate.Apply(gr, selfupdate.Options{
		Checksum: checksum,
		Hash:     crypto.SHA256,
	})
	if err != nil {
		return err
	}
	return nil
}
