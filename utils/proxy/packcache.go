package proxy

import (
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type iPackCache interface {
	Get(id, ver string) (resource.Pack, error)
	Has(id, ver string) bool
	Create(id, ver string) (*closeMoveWriter, error)
}

type packCache struct {
	Ignore bool
}

func (packCache) cachedPath(id, ver string) string {
	return filepath.Join("packcache", id+"_"+ver+".zip")
}

func (c *packCache) Get(id, ver string) (resource.Pack, error) {
	if c.Ignore {
		panic("not allowed")
	}
	return resource.ReadPath(c.cachedPath(id, ver))
}

func (c *packCache) Has(id, ver string) bool {
	if c.Ignore {
		return false
	}
	_, err := os.Stat(c.cachedPath(id, ver))
	return err == nil
}

func (c *packCache) Create(id, ver string) (*closeMoveWriter, error) {
	if c.Ignore {
		return nil, nil
	}

	finalPath := c.cachedPath(id, ver)
	tmpPath := finalPath + ".tmp"

	_ = os.MkdirAll(filepath.Dir(finalPath), 0777)

	f, err := createTemp(tmpPath)
	if err != nil {
		return nil, err
	}

	return &closeMoveWriter{
		File:      f,
		FinalName: finalPath,
	}, nil
}

type closeMoveWriter struct {
	*os.File
	FinalName string
}

func (c *closeMoveWriter) Move() error {
	return os.Rename(c.File.Name(), c.FinalName)
}
