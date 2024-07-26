package behaviourpack

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type blockBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBlock MinecraftBlock `json:"minecraft:block"`
}

func (bp *Pack) AddBlock(block protocol.BlockEntry) {
	ns, _ := ns_name_split(block.Name)
	if ns == "minecraft" {
		return
	}

	minecraftBlock, version := parseBlock(block)

	bp.blocks[block.Name] = &blockBehaviour{
		FormatVersion:  version,
		MinecraftBlock: minecraftBlock,
	}
}
