package merge

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/world"
)

type blockRegistry struct {
	world.BlockRegistry
	rids map[uint32]block
}

type block struct {
	name       string
	properties map[string]any
}

//go:linkname networkBlockHash github.com/df-mc/dragonfly/server/world.networkBlockHash
func networkBlockHash(name string, properties map[string]any) uint32

func (b blockRegistry) RuntimeIDToState(runtimeID uint32) (name string, properties map[string]any, found bool) {
	block := b.rids[runtimeID]
	return block.name, block.properties, true
}

func (b blockRegistry) StateToRuntimeID(name string, properties map[string]any) (runtimeID uint32, found bool) {
	runtimeID = networkBlockHash(name, properties)
	b.rids[runtimeID] = block{name, properties}
	return runtimeID, true
}

func (b blockRegistry) BlockByRuntimeID(rid uint32) (world.Block, bool) {
	block, ok := b.rids[rid]
	return world.UnknownBlock{
		BlockState: world.BlockState{
			Name:       block.name,
			Properties: block.properties,
		},
	}, ok
}
func (b blockRegistry) BlockRuntimeID(block world.Block) (rid uint32) {
	name, properties := block.EncodeBlock()
	return networkBlockHash(name, properties)
}
