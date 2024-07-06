package proxy

import (
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type iPackCache interface {
	Get(id, ver string) resource.Pack
	Has(id, ver string) bool
	Create(id, ver string) (*closeMoveWriter, error)
}

type packCache struct {
	Ignore bool
}

func (packCache) cachedPath(id, ver string) string {
	return filepath.Join("packcache", id+"_"+ver+".zip")
}

func (c *packCache) Get(id, ver string) resource.Pack {
	if c.Ignore {
		panic("not allowed")
	}
	return resource.MustReadPath(c.cachedPath(id, ver))
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

	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}

	return &closeMoveWriter{
		f:         f,
		FinalName: finalPath,
	}, nil
}

type closeMoveWriter struct {
	f         *os.File
	FinalName string
}

func (c *closeMoveWriter) Write(b []byte) (n int, err error) {
	return c.f.Write(b)
}

func (c *closeMoveWriter) Close() error {
	err := c.f.Close()
	if err != nil {
		return err
	}
	return os.Rename(c.f.Name(), c.FinalName)
}
