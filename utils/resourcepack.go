package utils

import (
	"archive/zip"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"golang.org/x/exp/slices"
)

type Pack interface {
	CanDecrypt() bool
	Base() *resource.Pack
	FS() (fs.FS, []string, error)
	SetD()
}

type packBase struct {
	*resource.Pack
	d bool
}

func (p *packBase) CanDecrypt() bool {
	return false
}

func (p *packBase) SetD() {
	p.d = true
}

func (p *packBase) FS() (fs.FS, []string, error) {
	if p.Encrypted() && !p.d {
		return nil, nil, errors.New("encrypted")
	}
	r, err := zip.NewReader(p, int64(p.Len()))
	if err != nil {
		return nil, nil, err
	}
	var names []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		names = append(names, path.Clean(strings.ReplaceAll(f.Name, "\\", "/")))
	}
	slices.Sort(names)

	return r, names, err
}

func (p *packBase) Base() *resource.Pack {
	return p.Pack
}

var PackFromBase = func(pack *resource.Pack) Pack {
	b := &packBase{pack, false}
	return b
}

func GetPacks(server minecraft.IConn) (packs []Pack) {
	for _, pack := range server.ResourcePacks() {
		packs = append(packs, PackFromBase(pack))
	}
	return
}
