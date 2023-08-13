package proxy

import (
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type packCache struct {
	Ignore bool
}

func (packCache) cachedPath(id string) string {
	return filepath.Join("packcache", id+".zip")
}

func (c *packCache) Get(id string) *resource.Pack {
	if c.Ignore {
		panic("not allowed")
	}
	return resource.MustCompile(c.cachedPath(id))
}

func (c *packCache) Has(id string) bool {
	if c.Ignore {
		return false
	}
	_, err := os.Stat(c.cachedPath(id))
	return err == nil
}

func (c *packCache) Put(pack *resource.Pack) error {
	if c.Ignore {
		return nil
	}
	p := c.cachedPath(pack.UUID() + "_" + pack.Version())
	os.MkdirAll(filepath.Dir(p), 0777)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = pack.WriteTo(f)
	pack.Seek(0, 0)
	return err
}
