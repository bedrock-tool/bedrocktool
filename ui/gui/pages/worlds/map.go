package worlds

import (
	"image"
	"image/draw"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Map struct {
	pressed bool

	MapImage  *image.RGBA
	BoundsMin protocol.ChunkPos
	BoundsMax protocol.ChunkPos
}

func (m *Map) Layout(gtx layout.Context) layout.Dimensions {
	// here we loop through all the events associated with this button.
	for _, e := range gtx.Events(m) {
		if e, ok := e.(pointer.Event); ok {
			switch e.Type {
			case pointer.Press:
				m.pressed = true
			case pointer.Release:
				m.pressed = false
			}
		}
	}

	return layout.Center.Layout(gtx, widget.Image{
		Src:      paint.NewImageOp(m.MapImage),
		Fit:      widget.Contain,
		Position: layout.Center,
	}.Layout)
}

func drawTile(img *image.RGBA, min, pos protocol.ChunkPos, tile *image.RGBA) {
	px := image.Pt(
		int((pos.X()-min[0])*16),
		int((pos.Z()-min[0])*16),
	)
	draw.Draw(img, image.Rect(
		px.X, px.Y,
		px.X+16, px.Y+16,
	), tile, image.Point{}, draw.Src)
}

func (m *Map) Update(u *utils.UpdateMapPayload) {
	needNewImage := false
	if m.BoundsMin != u.BoundsMin {
		needNewImage = true
		m.BoundsMin = u.BoundsMin
	}
	if m.BoundsMax != u.BoundsMax {
		needNewImage = true
		m.BoundsMax = u.BoundsMax
	}

	if needNewImage {
		chunksX := int(m.BoundsMax[0] - m.BoundsMin[0] + 1) // how many chunk lengths is x coordinate
		chunksY := int(m.BoundsMax[1] - m.BoundsMin[1] + 1)
		m.MapImage = image.NewRGBA(image.Rect(0, 0, chunksX*16, chunksY*16))
		for pos, tile := range u.Tiles {
			drawTile(m.MapImage, m.BoundsMin, pos, tile)
		}
	} else {
		for _, pos := range u.UpdatedTiles {
			tile := u.Tiles[pos]
			drawTile(m.MapImage, m.BoundsMin, pos, tile)
		}
	}
}
