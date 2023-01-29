package behaviourpack

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type blockBehaviour struct {
	FormatVersion  string               `json:"format_version"`
	MinecraftBlock world.MinecraftBlock `json:"minecraft:block"`
}

func (bp *BehaviourPack) AddBlock(block protocol.BlockEntry) {
	entry := blockBehaviour{
		FormatVersion:  bp.formatVersion,
		MinecraftBlock: world.ParseBlock(block),
	}

	bp.blocks = append(bp.blocks, entry)
}
