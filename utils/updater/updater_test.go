//go:build !android

package updater

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/bedrock-tool/bedrocktool/utils"
)

// The newest release with a build for this platform wins, not just the
// newest release.
func TestUpdateFromReleases(t *testing.T) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	ours := fmt.Sprintf("%s-%s-%s-1.0%s", utils.CmdName, runtime.GOOS, runtime.GOARCH, ext)
	digest := "sha256:" + fmt.Sprintf("%064x", 1)
	releases := []githubRelease{
		{TagName: "r2.0", Assets: []githubAsset{{Name: "other-platform.apk", Digest: digest}}},
		{TagName: "r1.0", Assets: []githubAsset{{Name: ours, Digest: digest, BrowserDownloadURL: "https://example.com/" + ours}}},
	}
	update, err := updateFromReleases(releases)
	if err != nil {
		t.Fatal(err)
	}
	if update.Version != "1.0" || update.DownloadURL != "https://example.com/"+ours {
		t.Errorf("got %+v, want release r1.0", update)
	}
	if _, err := updateFromReleases(releases[:1]); err == nil {
		t.Error("a release without a build for this platform was taken")
	}
}
