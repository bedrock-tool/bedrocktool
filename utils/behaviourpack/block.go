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
	if addCardinalRotation(&minecraftBlock, version) {
		bp.addedStates[block.Name] = map[string]any{cardinalDirection: cardinalDefault}
	} else {
		delete(bp.addedStates, block.Name)
	}
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

// AddedBlockStates is the states the pack gives blocks on top of the ones the
// server sent, by block name, with the value a saved block takes for each.
func (bp *Pack) AddedBlockStates() map[string]map[string]any {
	return bp.addedStates
}
