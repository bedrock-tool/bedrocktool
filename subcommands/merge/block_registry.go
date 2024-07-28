package merge

import (
	_ "unsafe"

	"github.com/df-mc/dragonfly/server/world"
)

type BlockRegistry struct {
	world.BlockRegistry
	Rids map[uint32]Block
}

type Block struct {
	name       string
	properties map[string]any
}

//go:linkname networkBlockHash github.com/df-mc/dragonfly/server/world.networkBlockHash
func networkBlockHash(name string, properties map[string]any) uint32

func (b BlockRegistry) RuntimeIDToState(runtimeID uint32) (name string, properties map[string]any, found bool) {
	block := b.Rids[runtimeID]
	return block.name, block.properties, true
}

func (b BlockRegistry) StateToRuntimeID(name string, properties map[string]any) (runtimeID uint32, found bool) {
	runtimeID = networkBlockHash(name, properties)
	b.Rids[runtimeID] = Block{name, properties}
	return runtimeID, true
}

func (b BlockRegistry) BlockByRuntimeID(rid uint32) (world.Block, bool) {
	block, ok := b.Rids[rid]
	return world.UnknownBlock{
		BlockState: world.BlockState{
			Name:       block.name,
			Properties: block.properties,
		},
	}, ok
}
func (b BlockRegistry) BlockRuntimeID(block world.Block) (rid uint32) {
	name, properties := block.EncodeBlock()
	return networkBlockHash(name, properties)
}
