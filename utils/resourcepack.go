package utils

import (
	"errors"
	"io/fs"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type packBase struct {
	resource.Pack
}

func (p *packBase) CanRead() bool {
	return !p.Encrypted()
}

func (p *packBase) Open(name string) (fs.File, error) {
	if p.Encrypted() {
		return nil, errors.New("encrypted")
	}
	return p.Pack.Open(name)
}

var PackFromBase = func(pack resource.Pack) (resource.Pack, error) {
	b := &packBase{pack}
	return b, nil
}
