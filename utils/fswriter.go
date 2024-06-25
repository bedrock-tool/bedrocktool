package utils

import (
	"archive/zip"
	"io"
	"os"
	"path"
)

type WriterFS interface {
	Create(filename string) (w io.WriteCloser, err error)
}

type OSWriter struct {
	base string
}

func (o *OSWriter) Create(filename string) (w io.WriteCloser, err error) {
	w, err = os.Create(filename)
	if err == os.ErrNotExist {
		err = os.MkdirAll(path.Dir(filename), 0777)
		if err != nil {
			return nil, err
		}
		return o.Create(filename)
	}
	return w, err
}

type ZipWriter struct {
	Writer *zip.Writer
}

func (z *ZipWriter) Create(filename string) (w io.WriteCloser, err error) {
	zw, err := z.Writer.Create(filename)
	return nullCloser{zw}, err
}

type nullCloser struct {
	io.Writer
}

func (nullCloser) Close() error {
	return nil
}
