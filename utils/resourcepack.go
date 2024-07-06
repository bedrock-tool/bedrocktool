package utils

import (
	"errors"
	"io/fs"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

type Pack interface {
	resource.Pack
	CanDecrypt() bool
}

type packBase struct {
	resource.Pack
}

func (p *packBase) CanDecrypt() bool {
	return false
}

func (p *packBase) Open(name string) (fs.File, error) {
	if p.Encrypted() {
		return nil, errors.New("encrypted")
	}
	return p.Pack.Open(name)
}

var PackFromBase = func(pack resource.Pack) (Pack, error) {
	b := &packBase{pack}
	return b, nil
}
