package resourcepacks

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type ReplayCache struct {
	packs map[string]resource.Pack
}

func NewReplayCache() *ReplayCache {
	return &ReplayCache{}
}

func (r *ReplayCache) Get(id uuid.UUID, ver string) (resource.Pack, error) {
	return r.packs[id.String()+"_"+ver], nil
}

func (r *ReplayCache) Has(id uuid.UUID, ver string) bool {
	_, ok := r.packs[id.String()+"_"+ver]
	return ok
}

func (r *ReplayCache) Create(id uuid.UUID, ver string) (*closeMoveWriter, error) { return nil, nil }

func (r *ReplayCache) ReadFrom(reader io.ReaderAt, readerSize int64) error {
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
			r.packs[pack.UUID().String()+"_"+pack.Version()] = pack
		}
	}
	return nil
}
