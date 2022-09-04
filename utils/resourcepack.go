package utils

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

func GetPacks(server *minecraft.Conn) (packs map[string]*resource.Pack, err error) {
	packs = make(map[string]*resource.Pack)
	for _, pack := range server.ResourcePacks() {
		packs[pack.Name()] = pack
	}
	return
}
