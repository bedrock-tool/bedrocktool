package android

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils"
)

// File is a capture or a saved world in the app's folder.
type File struct {
	Path string
	// Name is the file's name without its extension; Group is the server
	// folder a world is in ("" for captures).
	Name, Group string
	Size        int64
	Modified    time.Time
}

// Captures are the .pcap2 files sessions recorded, newest first.
func Captures() []File {
	return findFiles(utils.PathData("captures"), ".pcap2")
}

// Worlds are the .mcworld files saved in worlds/<server>/, newest first.
func Worlds() []File {
	return findFiles(utils.PathData("worlds"), ".mcworld")
}

func findFiles(root, ext string) []File {
	var files []File
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ext) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		f := File{
			Path:     path,
			Name:     strings.TrimSuffix(d.Name(), filepath.Ext(path)),
			Size:     info.Size(),
			Modified: info.ModTime(),
		}
		if dir := filepath.Dir(path); dir != root {
			f.Group = filepath.Base(dir)
		}
		files = append(files, f)
		return nil
	})
	slices.SortFunc(files, func(a, b File) int { return b.Modified.Compare(a.Modified) })
	return files
}

// ExportCapture opens the "save as" dialog for a copy of a capture, to
// replay it on a PC or attach it to a bug report.
func ExportCapture(path string) (string, error) {
	if err := exportFile(path, filepath.Base(path), T("android_capture_exported", nil), T("android_export_failed", nil)); err != nil {
		return "", err
	}
	return "", nil
}

// DeleteFile removes a capture or saved world. A world's folder of the same
// name (the unzipped world) goes with it.
func DeleteFile(f File) error {
	if err := os.Remove(f.Path); err != nil {
		return err
	}
	if strings.EqualFold(filepath.Ext(f.Path), ".mcworld") {
		dir := strings.TrimSuffix(f.Path, filepath.Ext(f.Path))
		if _, err := os.Stat(filepath.Join(dir, "level.dat")); err == nil {
			return os.RemoveAll(dir)
		}
	}
	return nil
}
