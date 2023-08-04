package utils

import (
	"archive/zip"
	"compress/flate"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func ZipFolder(filename, folder string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	// Register a custom Deflate compressor.
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.NoCompression)
	})

	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsDir() {
			rel := path[len(folder)+1:]
			zwf, _ := zw.Create(rel)
			data, err := os.ReadFile(path)
			if err != nil {
				logrus.Error(err)
			}
			zwf.Write(data)
		}
		return nil
	})
	zw.Close()
	f.Close()
	return err
}
