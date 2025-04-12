package merge

import (
	_ "unsafe"

	_ "github.com/df-mc/dragonfly/server/block"
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

func (b *BlockRegistry) RuntimeIDToState(runtimeID uint32) (name string, properties map[string]any, found bool) {
	name, properties, found = b.BlockRegistry.RuntimeIDToState(runtimeID)
	if found {
		return
	}
	block := b.Rids[runtimeID]
	return block.name, block.properties, true
}

func (b *BlockRegistry) StateToRuntimeID(name string, properties map[string]any) (runtimeID uint32, found bool) {
	runtimeID, found = b.BlockRegistry.StateToRuntimeID(name, properties)
	if found {
		return
	}
	runtimeID = networkBlockHash(name, properties)
	b.Rids[runtimeID] = Block{name, properties}
	return runtimeID, true
}

func (b *BlockRegistry) BlockByRuntimeID(rid uint32) (world.Block, bool) {
	block, ok := b.BlockRegistry.BlockByRuntimeID(rid)
	if ok {
		return block, true
	}
	block2, ok := b.Rids[rid]
	return world.UnknownBlock{
		BlockState: world.BlockState{
			Name:       block2.name,
			Properties: block2.properties,
		},
	}, ok
}
func (b *BlockRegistry) BlockRuntimeID(block world.Block) (rid uint32) {
	name, properties := block.EncodeBlock()
	return networkBlockHash(name, properties)
}

func (b *BlockRegistry) BlockCount() int {
	return b.BlockRegistry.BlockCount() + len(b.Rids)
}

func (b *BlockRegistry) RandomTickBlock(rid uint32) bool {
	if _, ok := b.Rids[rid]; ok {
		return false
	}
	return b.BlockRegistry.RandomTickBlock(rid)
}

func (b *BlockRegistry) FilteringBlock(rid uint32) uint8 {
	if _, ok := b.Rids[rid]; ok {
		return 15
	}
	return b.BlockRegistry.FilteringBlock(rid)
}

func (b *BlockRegistry) LightBlock(rid uint32) uint8 {
	if _, ok := b.Rids[rid]; ok {
		return 0
	}
	return b.BlockRegistry.LightBlock(rid)
}

func (b *BlockRegistry) NBTBlock(rid uint32) bool {
	if _, ok := b.Rids[rid]; ok {
		return false
	}
	return b.BlockRegistry.NBTBlock(rid)
}

func (b *BlockRegistry) LiquidDisplacingBlock(rid uint32) bool {
	if _, ok := b.Rids[rid]; ok {
		return true
	}
	return b.BlockRegistry.LiquidDisplacingBlock(rid)
}

func (b *BlockRegistry) LiquidBlock(rid uint32) bool {
	if _, ok := b.Rids[rid]; ok {
		return false
	}
	return b.BlockRegistry.LiquidBlock(rid)
}
