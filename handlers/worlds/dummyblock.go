package worlds

import (
	"image/color"

	"github.com/df-mc/dragonfly/server/world"
)

type dummyBlock struct {
	id  string
	nbt map[string]any
}

func (d *dummyBlock) EncodeBlock() (string, map[string]any) {
	return d.id, d.nbt
}

func (d *dummyBlock) Hash() uint64 {
	return 0
}

func (d *dummyBlock) Model() world.BlockModel {
	return nil
}

func (d *dummyBlock) Color() color.RGBA {
	return color.RGBA{0, 0, 0, 0}
}

func (d *dummyBlock) DecodeNBT(data map[string]any) any {
	return nil
}

func (d *dummyBlock) EncodeNBT() map[string]any {
	return d.nbt
}
