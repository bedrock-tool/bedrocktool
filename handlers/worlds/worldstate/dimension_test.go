package worldstate

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"testing"
)

func TestDimensionDefinitionUsesMinimumYAndHeightRange(t *testing.T) {
	id, _ := world.DimensionID(world.Overworld)
	w := &World{dimensionDefinitions: map[int]protocol.DimensionDefinition{id: {MinimumY: -64, HeightRange: 384}}}
	w.SetDimension(world.Overworld)
	if got := w.Range(); got.Min() != -64 || got.Max() != 319 {
		t.Fatalf("incorrect dimension range: %v", got)
	}
}
