package resourcepacks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/resource"

	"github.com/google/uuid"
)

type PackCache interface {
	Get(id uuid.UUID, ver string) (resource.Pack, error)
	Has(id uuid.UUID, ver string) bool
	Create(id uuid.UUID, ver string) (*closeMoveWriter, error)
}

// errBadCachedPack is returned by packCache.Get when the cached file isn't a
// readable pack (cut off by a crash, or saved from a server that sent a
// broken pack). Get removes the file, so the pack is downloaded again.
var errBadCachedPack = errors.New("cached pack is broken")

type packCache struct {
	Ignore bool
}

func (packCache) cachedPath(id uuid.UUID, ver string) string {
	return utils.PathCache("packcache", id.String()+"_"+ver+".zip")
}

func (c *packCache) Get(id uuid.UUID, ver string) (resource.Pack, error) {
	if c.Ignore {
		panic("not allowed")
	}
	p := c.cachedPath(id, ver)
	f, err := utils.OpenShared(p)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	pack, err := resource.FromReaderAt(f, stat.Size())
	if err != nil {
		f.Close()
		_ = os.Remove(p)
		return nil, fmt.Errorf("%w: %w", errBadCachedPack, err)
	}
	return pack, nil
}

func (c *packCache) Has(id uuid.UUID, ver string) bool {
	if c.Ignore {
		return false
	}
	_, err := os.Stat(c.cachedPath(id, ver))
	return err == nil
}

func (c *packCache) Create(id uuid.UUID, ver string) (*closeMoveWriter, error) {
	if c.Ignore {
		return nil, nil
	}

	finalPath := c.cachedPath(id, ver)
	tmpPath := finalPath + ".tmp"

	_ = os.MkdirAll(filepath.Dir(finalPath), 0777)

	f, err := utils.CreateShared(tmpPath)
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
