package behaviourpack

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type BlockBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBlock MinecraftBlock `json:"minecraft:block"`
}

func (bp *Pack) AddBlock(block protocol.BlockEntry) {
	ns, _ := splitNamespace(block.Name)
	if ns == "minecraft" {
		return
	}

	minecraftBlock, version := parseBlock(block)
	if r := findPlaceRotation(&minecraftBlock); r != nil {
		bp.placeRotations[block.Name] = r
	} else {
		delete(bp.placeRotations, block.Name)
	}

	bp.blocks[block.Name] = &BlockBehaviour{
		FormatVersion:  version,
		MinecraftBlock: minecraftBlock,
	}
}
