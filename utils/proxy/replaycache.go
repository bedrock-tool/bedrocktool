package proxy

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type replayCache struct {
	packs map[string]resource.Pack
}

func (r *replayCache) Get(id, ver string) resource.Pack {
	return r.packs[id+"_"+ver]
}

func (r *replayCache) Has(id, ver string) bool {
	_, ok := r.packs[id+"_"+ver]
	return ok
}

func (r *replayCache) Create(id, ver string) (*closeMoveWriter, error) { return nil, nil }

func (r *replayCache) ReadFrom(reader io.ReaderAt, readerSize int64) error {
	z, err := zip.NewReader(reader, readerSize)
	if err != nil {
		return err
	}

	r.packs = make(map[string]resource.Pack)
	for _, f := range z.File {
		f.Name = strings.ReplaceAll(f.Name, "\\", "/")
		if filepath.Dir(f.Name) == "packcache" {
			if f.Method != zip.Store {
				return errors.New("packcache compressed")
			}
			offset, err := f.DataOffset()
			if err != nil {
				return err
			}
			packReader := io.NewSectionReader(reader, offset, int64(f.CompressedSize64))
			pack, err := resource.FromReaderAt(packReader, int64(f.CompressedSize64))
			if err != nil {
				return err
			}
			r.packs[pack.UUID()+"_"+pack.Version()] = pack
		}
	}
	return nil
}
