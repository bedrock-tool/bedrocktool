package worlds

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"

	"github.com/df-mc/dragonfly/server/world"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type customBiome struct {
	name  string
	biome protocol.BiomeDefinition
	pk    *packet.BiomeDefinitionList
}

func (c *customBiome) Temperature() float64 {
	return 0.5
}

func (c *customBiome) Rainfall() float64 {
	return 0.5
}

func (c *customBiome) Depth() float64 {
	return 0
}

func (c *customBiome) Scale() float64 {
	return 0
}

func (c *customBiome) WaterColour() color.RGBA {
	return color.RGBA{}
}

func (c *customBiome) Tags() []string {
	return nil
}

func (c *customBiome) String() string {
	return c.name
}

func (c *customBiome) EncodeBiome() int {
	return int(c.biome.BiomeID)
}

// registerBiomes installs the packet's session-wide ID table. BiomeDefinitionList
// has no dimension field: IDs are shared by chunks in every dimension. Names
// need not be unique, so only ID lookup can preserve every packet entry.
func (w *worldsHandler) registerBiomes(pk *packet.BiomeDefinitionList) error {
	ids := make(map[int]world.Biome, len(pk.BiomeDefinitions))
	names := make(map[string]world.Biome, len(pk.BiomeDefinitions))
	for i, definition := range pk.BiomeDefinitions {
		if definition.NameIndex < 0 || int(definition.NameIndex) >= len(pk.StringList) {
			return fmt.Errorf("biome definition %d ID %d: name index %d outside string list (%d)", i, definition.BiomeID, definition.NameIndex, len(pk.StringList))
		}
		name := pk.StringList[definition.NameIndex]
		// 0xffff is an unassigned definition, not a chunk palette ID.
		// Retain its metadata below, but never invent an ID from its name
		// or string-table position or replace an explicit packet mapping.
		if definition.BiomeID == -1 {
			continue
		}
		b := &customBiome{name: name, biome: definition, pk: pk}
		id := b.EncodeBiome()
		if previous, ok := ids[id]; ok {
			old := previous.(*customBiome)
			oldDefinition, newDefinition := old.biome, definition
			// Different string-table slots may contain the same name.
			oldDefinition.NameIndex, newDefinition.NameIndex = 0, 0
			if old.name != name || !reflect.DeepEqual(oldDefinition, newDefinition) {
				return fmt.Errorf("conflicting biome ID %d: %q (name index %d) and %q (name index %d)", id, old.name, old.biome.NameIndex, name, definition.NameIndex)
			}
			continue
		}
		ids[id] = b
		// A name lookup is inherently singular; choose the first packet ID,
		// without dropping other IDs with that name from the ID table.
		if _, ok := names[name]; !ok {
			names[name] = b
		}
	}
	// Keep fully-qualified names authoritative; vanilla short names are only
	// compatibility aliases and must never overwrite a literal packet name.
	for _, definition := range pk.BiomeDefinitions {
		name := pk.StringList[definition.NameIndex]
		if _, assigned := names[name]; !assigned {
			continue
		}
		if short, ok := strings.CutPrefix(name, "minecraft:"); ok {
			if _, exists := names[short]; !exists {
				names[short] = names[name]
			}
		}
	}
	// Validate before changing the shared registry pointer used by worldState.
	w.serverState.biomes.IDToBiome = ids
	w.serverState.biomes.NameToBiome = names
	if w.serverState.exportedBiomeNames == nil {
		w.serverState.exportedBiomeNames = make(map[string]bool)
	}
	for _, definition := range pk.BiomeDefinitions {
		name := pk.StringList[definition.NameIndex]
		short := strings.TrimPrefix(name, "minecraft:")
		if _, vanilla := world.DefaultBiomes.BiomeByName(short); vanilla && (name == short || strings.HasPrefix(name, "minecraft:")) {
			continue
		}
		if !w.serverState.exportedBiomeNames[name] {
			w.serverState.behaviorPack.AddBiome(name, definition)
			w.serverState.exportedBiomeNames[name] = true
		}
	}
	return nil
}
