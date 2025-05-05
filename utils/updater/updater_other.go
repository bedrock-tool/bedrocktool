//go:build !js

package updater

import (
	"compress/gzip"
	"crypto"
	"encoding/base64"
	"fmt"
	"runtime"

	"github.com/bedrock-tool/bedrocktool/utils"
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

	r, _, err := fetchHttp(fmt.Sprintf("%s%s/%s/%s-%s.gz", UpdateServer, utils.CmdName, update.Version, runtime.GOOS, runtime.GOARCH))
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
