package behaviourpack

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type blockBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBlock MinecraftBlock `json:"minecraft:block"`
}

func (bp *BehaviourPack) AddBlock(block protocol.BlockEntry) {
	ns, _ := ns_name_split(block.Name)
	if ns == "minecraft" {
		return
	}
	entry := blockBehaviour{
		FormatVersion:  bp.formatVersion,
		MinecraftBlock: parseBlock(block),
	}

	bp.blocks = append(bp.blocks, entry)
}
