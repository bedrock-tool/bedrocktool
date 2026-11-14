package updater

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/minio/selfupdate"
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
	percent := 100
	if p.Total > 0 {
		percent = (p.done * 100) / p.Total
	}
	if p.percent != percent {
		p.OnProgress(percent)
		p.percent = percent
	}
	return len(b), nil
}

const updateFilename = "bedrocktool-update.bin"

const githubLatestReleaseURL = "https://api.github.com/repos/bedrock-tool/bedrocktool/releases/latest"

func fetchHttp(url string) (io.ReadCloser, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("bad http status from %s: %v", url, resp.Status)
	}
	return resp.Body, int(resp.ContentLength), nil
}

var cachedUpdate atomic.Pointer[Update]

func CheckUpdate() {
	err := func() error {
		update, err := GetLatest()
		if err != nil {
			return err
		}
		isNew := isNewVersion(update.Version)
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

func GetLatest() (*Update, error) {
	if update := cachedUpdate.Load(); update != nil {
		return update, nil
	}

	if runtime.GOOS == "android" || runtime.GOOS == "js" {
		cachedUpdate.Store(&Update{
			Version: utils.Version,
		})
		return cachedUpdate.Load(), nil
	}

	update, err := latestUpdate()
	if err != nil {
		return nil, err
	}
	cachedUpdate.Store(update)
	return update, nil
}

func DoUpdate(update *Update) error {
	checksum, err := base64.StdEncoding.DecodeString(update.Sha256)
	if err != nil {
		return err
	}

	r, size, err := fetchHttp(update.DownloadURL)
	if err != nil {
		return err
	}
	defer r.Close()

	updatePath := utils.PathCache(updateFilename)
	f, err := os.Create(updatePath)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(updatePath)
	}()

	sum := sha256.New()
	mw := io.MultiWriter(f, sum, &progressWriter{
		OnProgress: func(percent int) {
			messages.SendEvent(&messages.EventUpdateDownloadProgress{
				Progress: percent,
			})
		},
		Total: size,
	})
	_, err = io.Copy(mw, r)
	if err != nil {
		return err
	}

	if !bytes.Equal(checksum, sum.Sum(nil)) {
		return fmt.Errorf("update checksum mismatch")
	}

	// install

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

		checksum, err := base64.StdEncoding.DecodeString(update.Sha256)
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

type Update struct {
	Version     string
	Sha256      string
	DownloadURL string
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// isNewVersion says whether version differs from the running one. A local
// build ("1.63.0-19-g8737703-dirty") counts as having the release it's
// built on, and a leading "v" doesn't matter.
func isNewVersion(version string) bool {
	current := strings.TrimPrefix(utils.Version, "v")
	version = strings.TrimPrefix(version, "v")
	return current != version && !strings.HasPrefix(current, version+"-")
}

func (u *Update) IsNew() bool {
	return isNewVersion(u.Version)
}

func latestUpdate() (*Update, error) {
	r, _, err := fetchHttp(githubLatestReleaseURL)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var release githubRelease
	if err := json.NewDecoder(r).Decode(&release); err != nil {
		return nil, err
	}

	version := strings.TrimPrefix(release.TagName, "r")
	assetPrefix := fmt.Sprintf("%s-%s-%s-", utils.CmdName, runtime.GOOS, runtime.GOARCH)
	assetSuffix := ""
	if runtime.GOOS == "windows" {
		assetSuffix = ".exe"
	}
	for _, asset := range release.Assets {
		if !strings.HasPrefix(asset.Name, assetPrefix) || !strings.HasSuffix(asset.Name, assetSuffix) {
			continue
		}
		if !strings.HasPrefix(asset.Digest, "sha256:") {
			return nil, fmt.Errorf("GitHub asset %s has no SHA-256 digest", asset.Name)
		}
		digest, err := hex.DecodeString(strings.TrimPrefix(asset.Digest, "sha256:"))
		if err != nil {
			return nil, fmt.Errorf("invalid SHA-256 digest for GitHub asset %s: %w", asset.Name, err)
		}
		return &Update{
			Version:     version,
			Sha256:      base64.StdEncoding.EncodeToString(digest),
			DownloadURL: asset.BrowserDownloadURL,
		}, nil
	}

	return nil, fmt.Errorf("no GitHub release asset matching %s", assetPrefix)
}
