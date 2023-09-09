package proxy

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type replayCache struct {
	packs map[string]*resource.Pack
}

func (r *replayCache) Get(id string) *resource.Pack {
	return r.packs[id]
}
func (r *replayCache) Has(id string) bool {
	_, ok := r.packs[id]
	return ok
}
func (r *replayCache) Put(pack *resource.Pack) {}
func (r *replayCache) Close()                  {}

func (r *replayCache) ReadFrom(z *zip.Reader) error {
	r.packs = make(map[string]*resource.Pack)
	for _, f := range z.File {
		f.Name = strings.ReplaceAll(f.Name, "\\", "/")
		if filepath.Dir(f.Name) == "packcache" {
			f, err := z.Open(f.Name)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			_ = f.Close()
			pack, err := resource.FromBytes(data)
			if err != nil {
				return err
			}
			r.packs[pack.UUID()+"_"+pack.Version()] = pack
		}
	}
	return nil
}
