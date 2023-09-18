package worldstate

import (
	"image/color"

	"github.com/df-mc/dragonfly/server/world"
)

type DummyBlock struct {
	ID  string
	NBT map[string]any
}

func (d *DummyBlock) EncodeBlock() (string, map[string]any) {
	return d.ID, d.NBT
}

func (d *DummyBlock) Hash() uint64 {
	return 0
}

func (d *DummyBlock) Model() world.BlockModel {
	return nil
}

func (d *DummyBlock) Color() color.RGBA {
	return color.RGBA{0, 0, 0, 0}
}

func (d *DummyBlock) DecodeNBT(data map[string]any) any {
	return nil
}

func (d *DummyBlock) EncodeNBT() map[string]any {
	return d.NBT
}
