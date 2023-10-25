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
	bp.blocks[block.Name] = &blockBehaviour{
		FormatVersion:  "1.14.0",
		MinecraftBlock: parseBlock(block),
	}
}
