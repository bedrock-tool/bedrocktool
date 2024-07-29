package utils

import (
	"archive/zip"
	"compress/flate"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
)

type WriterFS interface {
	Create(filename string) (w io.WriteCloser, err error)
}

func SubFS(base WriterFS, dir string) WriterFS {
	return &subFS{base: base, dir: dir}
}

type subFS struct {
	base WriterFS
	dir  string
}

func (s *subFS) Create(filename string) (w io.WriteCloser, err error) {
	filename = path.Clean(filename)
	filename = path.Join(s.dir, filename)
	return s.base.Create(filename)
}

func CopyFS(src fs.FS, dst WriterFS) error {
	return fs.WalkDir(src, ".", func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		r, err := src.Open(fpath)
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := dst.Create(fpath)
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
		return nil
	})
}

type OSWriter struct {
	Base string
}

func (o OSWriter) Create(filename string) (w io.WriteCloser, err error) {
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

var deflatePool = sync.Pool{
	New: func() any {
		w, _ := flate.NewWriter(nil, flate.HuffmanOnly)
		return w
	},
}

type closePutback struct {
	*flate.Writer
}

func (c *closePutback) Close() error {
	if c.Writer == nil {
		return nil
	}
	err := c.Writer.Close()
	if err != nil {
		return err
	}
	deflatePool.Put(c.Writer)
	c.Writer = nil
	return nil
}

func ZipCompressPool(zw *zip.Writer) {
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		w := deflatePool.Get().(*flate.Writer)
		w.Reset(out)
		return &closePutback{w}, nil
	})
}

type ZipWriter struct {
	Writer *zip.Writer
}

func (z ZipWriter) Create(filename string) (w io.WriteCloser, err error) {
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

func (m MultiWriterFS) Create(filename string) (w io.WriteCloser, err error) {
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
