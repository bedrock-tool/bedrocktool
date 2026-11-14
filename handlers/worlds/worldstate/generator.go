package worldstate

import (
	"fmt"
	"strings"

	"github.com/df-mc/dragonfly/server/world/mcdb/leveldat"
)

// Generator is what Minecraft does with the chunks that weren't saved, when
// the world is opened.
type Generator string

const (
	// GeneratorVoid leaves them empty.
	GeneratorVoid Generator = "void"
	// GeneratorFlat fills them with Minecraft's default superflat layers.
	GeneratorFlat Generator = "flat"
	// GeneratorNormal generates normal terrain from the server's seed.
	GeneratorNormal Generator = "normal"
)

// Generators lists every generator, the default first.
var Generators = []Generator{GeneratorVoid, GeneratorFlat, GeneratorNormal}

// ParseGenerator reads a generator's name; an empty name is the default.
func ParseGenerator(name string) (Generator, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return GeneratorVoid, nil
	}
	for _, g := range Generators {
		if string(g) == name {
			return g, nil
		}
	}
	return "", fmt.Errorf("unknown generator %q (use void, flat or normal)", name)
}

// voidLayers is a superflat preset with only air in it.
const voidLayers = `{"biome_id":1,"block_layers":[{"block_data":0,"block_id":0,"count":1},{"block_data":0,"block_id":0,"count":2},{"block_data":0,"block_id":0,"count":1}],"encoding_version":3,"structure_options":null}`

// applyGenerator sets up ld so Minecraft generates missing chunks with g.
// The seed (ld.RandomSeed) is set apart, from the server's StartGame.
func applyGenerator(ld *leveldat.Data, g Generator) {
	switch g {
	case GeneratorFlat:
		// an empty preset is Minecraft's default: grass, dirt and bedrock
		ld.Generator = 2
		ld.FlatWorldLayers = ""
	case GeneratorNormal:
		ld.Generator = 1
		ld.FlatWorldLayers = ""
	default:
		ld.Generator = 2
		ld.FlatWorldLayers = voidLayers
	}
}
