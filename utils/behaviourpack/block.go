package behaviourpack

import (
	"fmt"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type BlockBehaviour struct {
	FormatVersion  string         `json:"format_version"`
	MinecraftBlock MinecraftBlock `json:"minecraft:block"`
}

func (bp *Pack) AddBlock(block protocol.BlockEntry) {
	// Keep unsupported server data fatal rather than silently exporting an
	// incomplete world, but identify the failing stage and block in the report.
	defer func() {
		if reason := recover(); reason != nil {
			panic(fmt.Sprintf("custom-block-export block=%q: %v", block.Name, reason))
		}
	}()
	ns, _ := splitNamespace(block.Name)
	if ns == "minecraft" {
		return
	}

	minecraftBlock, version := parseBlock(block)

	bp.blocks[block.Name] = &BlockBehaviour{
		FormatVersion:  version,
		MinecraftBlock: minecraftBlock,
	}
}
