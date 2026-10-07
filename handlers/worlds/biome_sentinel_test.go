package worlds

import (
	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"testing"
)

func TestBiomeDefinitionListUnassignedIDs(t *testing.T) {
	w := &worldsHandler{serverState: serverState{biomes: world.DefaultBiomes.Clone(), behaviorPack: behaviourpack.New("test")}}
	pk := &packet.BiomeDefinitionList{StringList: []string{"minecraft:ocean", "minecraft:deep_ocean", "addon:unassigned"}, BiomeDefinitions: []protocol.BiomeDefinition{
		{NameIndex: 0, BiomeID: 0}, {NameIndex: 1, BiomeID: 24},
		{NameIndex: 1, BiomeID: -1}, {NameIndex: 0, BiomeID: -1}, {NameIndex: 2, BiomeID: -1},
	}}
	if err := w.registerBiomes(pk); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.serverState.biomes.BiomeByID(-1); ok {
		t.Fatal("unassigned sentinel registered as chunk ID")
	}
	for _, id := range []int{0, 24} {
		b, ok := w.serverState.biomes.BiomeByID(id)
		if !ok || b.EncodeBiome() != id {
			t.Fatalf("lost valid mapping %d: %v", id, b)
		}
	}
	if b, ok := w.serverState.biomes.BiomeByName("ocean"); !ok || b.EncodeBiome() != 0 {
		t.Fatal("sentinel replaced valid name mapping")
	}
	if _, ok := w.serverState.biomes.BiomeByName("addon:unassigned"); ok {
		t.Fatal("invented ID for unassigned custom biome")
	}
	if !w.serverState.exportedBiomeNames["addon:unassigned"] {
		t.Fatal("lost unassigned biome metadata")
	}
}
