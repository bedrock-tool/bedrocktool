package android

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/sirupsen/logrus"
)

// DownloadsFolder is the folder in Downloads that worlds opened in
// Minecraft are copied to; file managers can see it, unlike the app's own.
const DownloadsFolder = "Bedrocktool"

// ExportWorld opens the system's "save as" dialog for a copy of the world.
// Android copies it once the user picks a place, and says how that went
// in a toast; the original stays where it is.
func ExportWorld(path string) (string, error) {
	if err := exportFile(path, filepath.Base(path), T("android_exported", nil), T("android_export_failed", nil)); err != nil {
		return "", err
	}
	return "", nil
}

var (
	downloadedMu sync.Mutex
	// downloaded are the worlds already copied to Downloads in this run, so
	// opening one twice doesn't make "name (1).mcworld" next to it
	downloaded = map[string]bool{}
)

// saveToDownloads copies the world to Downloads/Bedrocktool once, where file
// managers can see it. It reports whether a copy is there.
func saveToDownloads(path string) bool {
	downloadedMu.Lock()
	defer downloadedMu.Unlock()
	if downloaded[path] {
		return true
	}
	if _, err := copyToDownloads(path, filepath.Base(path), DownloadsFolder); err != nil {
		// Android 9 and older have no MediaStore Downloads; Minecraft
		// still gets the world, there's just no copy
		logrus.Warnf("copying %s to Downloads: %s", filepath.Base(path), err)
		return false
	}
	downloaded[path] = true
	return true
}

// OpenInMinecraft keeps a copy of the world in Downloads/Bedrocktool and has
// Minecraft import it. It blocks while copying; call it from a goroutine.
func OpenInMinecraft(path string) (string, error) {
	copied := saveToDownloads(path)
	uri, err := worldUri(path)
	if err != nil {
		return "", err
	}
	if err := openInMinecraft(uri); err != nil {
		if strings.Contains(err.Error(), "ActivityNotFoundException") {
			return "", errors.New(T("android_minecraft_missing", nil))
		}
		return "", err
	}
	if !copied {
		return T("android_opened_no_copy", nil), nil
	}
	return T("android_opened", locale.Strmap{"Folder": "Downloads/" + DownloadsFolder}), nil
}
