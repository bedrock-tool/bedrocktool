package main

import "github.com/sandertv/gophertunnel/minecraft/resource"

func (w *WorldState) getPacks() (packs map[string]*resource.Pack, err error) {
	packs = make(map[string]*resource.Pack)
	for _, pack := range w.proxy.server.ResourcePacks() {
		packs[pack.Name()] = pack
	}
	return
}
