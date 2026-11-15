package updater

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/bedrock-tool/bedrocktool/utils"
)

// releasesURL is where the Android app's updates come from: upstream's
// releases, like the PC GUI. It lists them newest first; the newest one with
// an APK wins (see assetMatches).
const releasesURL = "https://api.github.com/repos/bedrock-tool/bedrocktool/releases"

// RestartsAfterUpdate: an app can't replace itself on Android. install
// opens Android's installer, which asks the user, then closes the app and
// installs the new one.
const RestartsAfterUpdate = false

// InstallAPK hands a downloaded APK to Android's installer. Set by
// ui/gui/android, which has the JNI to do it.
var InstallAPK func(path string) error

// assetMatches says whether a release file is the APK for this phone's CPU,
// as build.go names it: bedrocktool-gui-android-arm64-<version>.apk.
func assetMatches(name string) bool {
	return strings.HasPrefix(name, fmt.Sprintf("bedrocktool-gui-android-%s-", runtime.GOARCH)) &&
		strings.HasSuffix(name, ".apk")
}

// downloadPath is in the app's files folder, which the app's file provider
// can hand to the installer. It stays there while the installer reads it;
// ui/gui/android removes it on the next start.
func downloadPath() string {
	return utils.PathData("update", "bedrocktool.apk")
}

func install(path string, checksum []byte) error {
	if InstallAPK == nil {
		return errors.New("installing updates isn't set up")
	}
	return InstallAPK(path)
}

// Restart does nothing: Android's installer restarts the app.
func Restart() error { return nil }
