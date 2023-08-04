package resourcepackd

import (
	"io"
	"io/fs"
)

type readFile struct {
	f fs.File
	r io.Reader
}

func (r *readFile) Read(dst []byte) (n int, err error) {
	return r.r.Read(dst)
}

func (r *readFile) Stat() (fs.FileInfo, error) {
	return r.f.Stat()
}
func (r *readFile) Close() error {
	return r.f.Close()
}
