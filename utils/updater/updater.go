package updater

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/minio/selfupdate"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
)

type progressWriter struct {
	OnProgress func(percent int)
	percent    int
	done       int
	Total      int
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += len(b)
	percent := (p.done * 100) / (p.Total * 100)
	if p.percent != percent {
		p.OnProgress(percent)
		p.percent = percent
	}
	return len(b), nil
}

const updateFilename = "bedrocktool-update.bin"

const UpdateServer = "https://updates.yuv.pink/"

func fetchHttp(url string) (io.ReadCloser, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	// set user agent to know what versions are run
	h, _ := os.Hostname()       // sent as crc32 hashed
	v, _ := mem.VirtualMemory() // how much ram you have
	req.Header.Add("User-Agent", fmt.Sprintf("%s '%s' %d %d %d", utils.CmdName, utils.Version, crc32.ChecksumIEEE([]byte(h)), runtime.NumCPU(), v.Total))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}

	return resp.Body, int(resp.ContentLength), nil
}

type Updater struct {
	Update *Update
}

func (u *Updater) CheckUpdate() {
	err := func() error {
		r, _, err := fetchHttp(fmt.Sprintf("%s%s/%s-%s.json", UpdateServer, utils.CmdName, runtime.GOOS, runtime.GOARCH))
		if err != nil {
			return err
		}
		defer r.Close()
		d := json.NewDecoder(r)

		var update Update
		if err = d.Decode(&update); err != nil {
			return err
		}
		u.Update = &update

		isNew := update.Version != utils.Version
		if isNew {
			logrus.Info(locale.Loc("update_available", locale.Strmap{"Version": update.Version}))
			messages.SendEvent(&messages.EventUpdateAvailable{
				Version: update.Version,
			})
		}
		return nil
	}()
	if err != nil {
		logrus.Error(err)
	}
}

func (u *Updater) DownloadUpdate() error {
	if u.Update == nil {
		return fmt.Errorf("no update available")
	}
	checksum, err := base64.StdEncoding.DecodeString(u.Update.Sha256)
	if err != nil {
		return err
	}

	r, size, err := fetchHttp(fmt.Sprintf("%s%s/%s/%s-%s.gz", UpdateServer, utils.CmdName, u.Update.Version, runtime.GOOS, runtime.GOARCH))
	if err != nil {
		return err
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	updatePath := utils.PathCache(updateFilename)
	f, err := os.Create(updatePath)
	if err != nil {
		return err
	}
	defer f.Close()

	sum := sha256.New()
	mw := io.MultiWriter(f, sum, &progressWriter{
		OnProgress: func(percent int) {
			messages.SendEvent(&messages.EventUpdateDownloadProgress{
				Progress: percent,
			})
		},
		Total: size,
	})
	_, err = io.Copy(mw, gr)
	if err != nil {
		return err
	}

	if !bytes.Equal(checksum, sum.Sum(nil)) {
		return fmt.Errorf("update checksum mismatch")
	}
	return nil
}

func (u *Updater) InstallUpdate() error {
	updatePath := utils.PathCache(updateFilename)
	messages.SendEvent(&messages.EventUpdateDoInstall{
		Filepath: updatePath,
	})

	if runtime.GOOS != "android" {
		f, err := os.Open(updatePath)
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()

		checksum, err := base64.StdEncoding.DecodeString(u.Update.Sha256)
		if err != nil {
			return err
		}

		if err = selfupdate.Apply(f, selfupdate.Options{
			Checksum: checksum,
			Hash:     crypto.SHA256,
		}); err != nil {
			return err
		}
	}
	return nil
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

	if runtime.GOOS == "android" {
		updateAvailable = &Update{
			Version: utils.Version,
			Sha256:  "",
		}
		return updateAvailable, nil
	}

	if runtime.GOOS == "js" {
		updateAvailable = &Update{
			Version: utils.Version,
			Sha256:  "",
		}
		return updateAvailable, nil
	}

	r, _, err := fetchHttp(fmt.Sprintf("%s%s/%s-%s.json", UpdateServer, utils.CmdName, runtime.GOOS, runtime.GOARCH))
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

func UpdateCheck(ui ui.UI) {
	update, err := UpdateAvailable()
	if err != nil {
		logrus.Warn(err)
		return
	}
	isNew := update.Version != utils.Version

	if isNew {
		logrus.Info(locale.Loc("update_available", locale.Strmap{"Version": update.Version}))
		messages.SendEvent(&messages.EventUpdateAvailable{
			Version: update.Version,
		})
	}
}
