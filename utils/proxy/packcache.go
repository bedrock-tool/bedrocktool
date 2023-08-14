package proxy

import (
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type packCache struct {
	Ignore bool
	commit chan struct{}
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

func (c *packCache) Put(pack *resource.Pack) {
	if c.Ignore {
		return
	}
	go func() {
		<-c.commit
		p := c.cachedPath(pack.UUID() + "_" + pack.Version())
		os.MkdirAll(filepath.Dir(p), 0777)
		f, err := os.Create(p)
		if err != nil {
			logrus.Error(err)
		}
		defer f.Close()
		pack.WriteTo(f)
		pack.Seek(0, 0)
	}()
}
