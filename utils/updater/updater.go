package updater

import (
	"compress/gzip"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/minio/selfupdate"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

var Version string
var CmdName = "invalid"

const UpdateServer = "https://updates.yuv.pink/"

func fetch(url string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	// set user agent to know what versions are run
	h, _ := os.Hostname()       // sent as crc32 hashed
	v, _ := mem.VirtualMemory() // how much ram you have
	req.Header.Add("User-Agent", fmt.Sprintf("%s '%s' %d %d %d", CmdName, Version, crc32.ChecksumIEEE([]byte(h)), runtime.NumCPU(), v.Total))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}

	return resp.Body, nil
}

type Update struct {
	Version string
	Sha256  string
}

var updateAvailable *Update
var updateAvailableMutex sync.Mutex

func UpdateAvailable() (*Update, error) {
	updateAvailableMutex.Lock()
	defer updateAvailableMutex.Unlock()
	if updateAvailable != nil {
		return updateAvailable, nil
	}

	r, err := fetch(fmt.Sprintf("%s%s/%s-%s.json", UpdateServer, CmdName, runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	d := json.NewDecoder(r)

	var update Update
	err = d.Decode(&update)
	if err != nil {
		return nil, err
	}

	updateAvailable = &update
	return updateAvailable, nil
}

func DoUpdate() error {
	update, err := UpdateAvailable()
	if err != nil {
		return err
	}
	logrus.Debugf("updating to %s", update.Version)

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
		Verifier: selfupdate.NewVerifier(),
	})
	if err != nil {
		return err
	}
	return nil
}
