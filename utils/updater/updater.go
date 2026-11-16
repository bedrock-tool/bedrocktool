package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
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

// ErrNoBuild is no release having a build for this platform yet, such as
// the Android app before its first release.
var ErrNoBuild = errors.New("no GitHub release has a build")

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
	if errors.Is(err, ErrNoBuild) {
		logrus.Debug(err)
	} else if err != nil {
		logrus.Error(err)
	}
}

func GetLatest() (*Update, error) {
	if update := cachedUpdate.Load(); update != nil {
		return update, nil
	}

	if runtime.GOOS == "js" {
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

// DoUpdate downloads the update, checks its checksum and installs it the
// platform's way (install in updater_desktop.go and updater_android.go).
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

	updatePath := downloadPath()
	if err := os.MkdirAll(filepath.Dir(updatePath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(updatePath)
	if err != nil {
		return err
	}

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
	f.Close()
	if err == nil && !bytes.Equal(checksum, sum.Sum(nil)) {
		err = fmt.Errorf("update checksum mismatch")
	}
	if err != nil {
		os.Remove(updatePath)
		return err
	}

	messages.SendEvent(&messages.EventUpdateDoInstall{
		Filepath: updatePath,
	})
	return install(updatePath, checksum)
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
	r, _, err := fetchHttp(releasesURL)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	// releasesURL is either one release (/releases/latest) or a list of them,
	// newest first (/releases)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var releases []githubRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		var release githubRelease
		if err := json.Unmarshal(data, &release); err != nil {
			return nil, err
		}
		releases = []githubRelease{release}
	}
	return updateFromReleases(releases)
}

// updateFromReleases is the newest release that has a build for this
// platform (assetMatches).
func updateFromReleases(releases []githubRelease) (*Update, error) {
	for _, release := range releases {
		for _, asset := range release.Assets {
			if !assetMatches(asset.Name) {
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
				Version:     strings.TrimPrefix(release.TagName, "r"),
				Sha256:      base64.StdEncoding.EncodeToString(digest),
				DownloadURL: asset.BrowserDownloadURL,
			}, nil
		}
	}
	return nil, fmt.Errorf("%w for %s-%s", ErrNoBuild, runtime.GOOS, runtime.GOARCH)
}
