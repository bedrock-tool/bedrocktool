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
	Base string
}

func (o *OSWriter) Create(filename string) (w io.WriteCloser, err error) {
	fullpath := o.Base + "/" + filename
	err = os.MkdirAll(path.Dir(fullpath), 0777)
	if err != nil {
		return nil, err
	}
	w, err = os.Create(fullpath)
	if err != nil {
		return nil, err
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

type MultiWriterFS struct {
	FSs []WriterFS
}

func (m *MultiWriterFS) Create(filename string) (w io.WriteCloser, err error) {
	var files []io.Writer
	var closers []func() error
	for _, fs := range m.FSs {
		f, err := fs.Create(filename)
		if err != nil {
			for _, f := range files {
				f := f.(io.Closer)
				f.Close()
			}
			return nil, err
		}
		files = append(files, f)
		closers = append(closers, f.Close)
	}

	return multiCloser{
		Writer:  io.MultiWriter(files...),
		closers: closers,
	}, nil
}

type multiCloser struct {
	io.Writer
	closers []func() error
}

func (m multiCloser) Close() error {
	for _, close := range m.closers {
		close()
	}
	return nil
}
