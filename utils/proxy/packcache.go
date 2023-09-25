package proxy

import (
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sirupsen/logrus"
)

type iPackCache interface {
	Get(id, ver string) *resource.Pack
	Has(id, ver string) bool
	Put(pack *resource.Pack)
	Close()
}

type packCache struct {
	Ignore bool
	commit chan struct{}
}

func (packCache) cachedPath(id, ver string) string {
	return filepath.Join("packcache", id+"_"+ver+".zip")
}

func (c *packCache) Get(id, ver string) *resource.Pack {
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

func (c *packCache) Put(pack *resource.Pack) {
	if c.Ignore {
		return
	}
	go func() {
		<-c.commit
		p := c.cachedPath(pack.UUID(), pack.Version())
		_ = os.MkdirAll(filepath.Dir(p), 0777)
		f, err := os.Create(p)
		if err != nil {
			logrus.Error(err)
		}
		defer f.Close()
		_, _ = pack.WriteTo(f)
		_, _ = pack.Seek(0, 0)
	}()
}

func (c *packCache) Close() {
	close(c.commit)
}
