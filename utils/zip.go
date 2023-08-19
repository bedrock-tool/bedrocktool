package utils

import (
	"archive/zip"
	"compress/flate"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type closePutback struct {
	*flate.Writer
}

func (c closePutback) Close() error {
	deflate.ReturnWriter(c.Writer)
	return nil
}

type DeflatePool struct {
	pool sync.Pool
}

var deflate DeflatePool

func (pool *DeflatePool) GetWriter(dst io.Writer) (writer *flate.Writer) {
	if w := pool.pool.Get(); w != nil {
		writer = w.(*flate.Writer)
		writer.Reset(dst)
	} else {
		writer, _ = flate.NewWriter(dst, flate.HuffmanOnly)
	}
	return writer
}

func (pool *DeflatePool) ReturnWriter(writer *flate.Writer) {
	_ = writer.Close()
	pool.pool.Put(writer)
}

func ZipFolder(filename, folder string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	// Register a custom Deflate compressor.
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		w := deflate.GetWriter(out)
		return closePutback{w}, nil
	})

	folder = strings.ReplaceAll(folder, "./", "")
	err = filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if !d.Type().IsDir() {
			rel := path[len(folder)+1:]
			zwf, _ := zw.Create(rel)
			f, err := os.Open(path)
			if err != nil {
				logrus.Error(err)
				return nil
			}
			_, err = io.Copy(zwf, f)
			if err != nil {
				logrus.Error(err)
				return nil
			}
		}
		return nil
	})

	return zw.Close()
}
