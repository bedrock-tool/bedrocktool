package worlds

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bedrock-tool/bedrocktool/utils/behaviourpack"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestBiomeDefinitionListDuplicateIDs(t *testing.T) {
	for _, tc := range []struct {
		name        string
		definitions []protocol.BiomeDefinition
		wantError   string
	}{
		{"identical repeated ID", []protocol.BiomeDefinition{{NameIndex: 0, BiomeID: 180}, {NameIndex: 1, BiomeID: 180}}, ""},
		{"different name same ID", []protocol.BiomeDefinition{{NameIndex: 0, BiomeID: 180}, {NameIndex: 2, BiomeID: 180}}, "conflicting biome ID 180"},
		{"different data same ID", []protocol.BiomeDefinition{{NameIndex: 0, BiomeID: 180}, {NameIndex: 0, BiomeID: 180, Temperature: 1}}, "conflicting biome ID 180"},
		{"negative name index", []protocol.BiomeDefinition{{NameIndex: -1, BiomeID: 180}}, "name index -1"},
		{"out of bounds name index", []protocol.BiomeDefinition{{NameIndex: 3, BiomeID: 180}}, "name index 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := world.DefaultBiomes.Clone()
			w := &worldsHandler{serverState: serverState{biomes: registry, behaviorPack: behaviourpack.New("test")}}
			original, _ := registry.BiomeByID(0)
			pk := &packet.BiomeDefinitionList{StringList: []string{"minecraft:dappled_forest", "minecraft:dappled_forest", "addon:dappled_forest"}, BiomeDefinitions: tc.definitions}
			_, err := w.packetHandlerPreLogin(pk, time.Time{})
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				if current, _ := registry.BiomeByID(0); current != original {
					t.Fatal("invalid packet mutated registry")
				}
				if len(w.serverState.exportedBiomeNames) != 0 {
					t.Fatal("invalid packet exported biomes")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if len(registry.IDToBiome) != 1 {
					t.Fatalf("IDs = %v", registry.IDToBiome)
				}
				if len(w.serverState.exportedBiomeNames) != 1 {
					t.Fatalf("exports = %v", w.serverState.exportedBiomeNames)
				}
			}
			if w.serverState.biomes != registry {
				t.Fatal("replaced registry pointer shared with worldState")
			}
		})
	}
}

func TestBiomeDefinitionListRemapsVanillaID(t *testing.T) {
	w := &worldsHandler{serverState: serverState{biomes: world.DefaultBiomes.Clone(), behaviorPack: behaviourpack.New("test")}}
	pk := &packet.BiomeDefinitionList{StringList: []string{"minecraft:ocean", "minecraft:dappled_forest", "dappled_forest"}, BiomeDefinitions: []protocol.BiomeDefinition{{NameIndex: 0, BiomeID: 9}, {NameIndex: 1, BiomeID: 0}, {NameIndex: 2, BiomeID: 180}}}
	if _, err := w.packetHandlerPreLogin(pk, time.Time{}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range pk.BiomeDefinitions {
		b, ok := w.serverState.biomes.BiomeByID(int(entry.BiomeID))
		if !ok || b.String() != pk.StringList[entry.NameIndex] {
			t.Fatalf("wrong mapping for ID %d: %v", entry.BiomeID, b)
		}
	}
	if b, _ := w.serverState.biomes.BiomeByName("ocean"); b.EncodeBiome() != 9 {
		t.Fatal("stale vanilla ID alias")
	}
	if b, _ := w.serverState.biomes.BiomeByName("dappled_forest"); b.EncodeBiome() != 180 {
		t.Fatal("namespace stripping overwrote literal name")
	}
}

func TestBiomeDefinitionListPreservesPacketIDs(t *testing.T) {
	originalDefaults := world.DefaultBiomes.Clone()
	w := &worldsHandler{serverState: serverState{biomes: world.DefaultBiomes.Clone(), behaviorPack: behaviourpack.New("test")}}
	// A vanilla ID and custom entries with duplicate names, not duplicate IDs.
	pk := &packet.BiomeDefinitionList{
		StringList: []string{"minecraft:ocean", "minecraft:dappled_forest", "addon:dappled_forest"},
		BiomeDefinitions: []protocol.BiomeDefinition{
			{NameIndex: 0, BiomeID: 0},
			{NameIndex: 1, BiomeID: 180},
			{NameIndex: 1, BiomeID: 181},
			{NameIndex: 2, BiomeID: 182},
		},
	}
	for replay := 0; replay < 2; replay++ {
		got, err := w.packetHandlerPreLogin(pk, time.Time{})
		if err != nil || got != pk {
			t.Fatalf("packet result = %v, %v", got, err)
		}
		for i, id := range []int{0, 180, 181, 182} {
			b, ok := w.serverState.biomes.BiomeByID(id)
			if !ok || b.EncodeBiome() != id {
				t.Fatalf("missing/incorrect biome ID %d: %v", id, b)
			}
			if b.String() != pk.StringList[pk.BiomeDefinitions[i].NameIndex] {
				t.Fatalf("ID %d name = %q", id, b.String())
			}
		}
	}
	if b, ok := w.serverState.biomes.BiomeByName("dappled_forest"); !ok || b.EncodeBiome() != 180 {
		t.Fatal("missing vanilla unqualified alias")
	}
	if !reflect.DeepEqual(world.DefaultBiomes, originalDefaults) {
		t.Fatal("session modified global registry")
	}
	if len(w.serverState.exportedBiomeNames) != 2 {
		t.Fatalf("exports = %v", w.serverState.exportedBiomeNames)
	}
}
