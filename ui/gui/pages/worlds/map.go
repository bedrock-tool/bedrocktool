package worlds

import (
	"image"
	"image/draw"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Map struct {
	click   f32.Point
	mapPos  f32.Point
	pos     f32.Point
	imageOp paint.ImageOp
	zoom    float32

	drag   gesture.Drag
	scroll gesture.Scroll

	MapImage  *image.RGBA
	BoundsMin protocol.ChunkPos
	BoundsMax protocol.ChunkPos
	Rotation  float32
}

func (m *Map) Layout(gtx layout.Context) layout.Dimensions {
	// here we loop through all the events associated with this button.
	for _, e := range m.drag.Events(gtx.Metric, gtx.Queue, gesture.Both) {
		switch e.Type {
		case pointer.Press:
			m.click = e.Position
		case pointer.Drag:
			m.pos = m.mapPos.Sub(m.click).Add(e.Position)
		case pointer.Release:
			m.mapPos = m.pos
		}
	}

	scrollDist := m.scroll.Scroll(gtx.Metric, gtx.Queue, time.Now(), gesture.Vertical)

	m.zoom -= float32(scrollDist) / 20
	if m.zoom < 0.2 {
		m.zoom = 0.2
	}

	size := gtx.Constraints.Max

	if m.MapImage != nil {
		m.imageOp.Add(gtx.Ops)
		b := m.MapImage.Bounds()
		sx := float32(b.Dx() / 2)
		sy := float32(b.Dy() / 2)

		op.Affine(
			f32.Affine2D{}.
				Scale(f32.Pt(sx, sy), f32.Pt(m.zoom, m.zoom)).
				Offset(m.pos),
		).Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}

	m.drag.Add(gtx.Ops)
	m.scroll.Add(gtx.Ops, image.Rect(-size.X, -size.Y, size.X, size.Y))

	return layout.Dimensions{
		Size: size,
	}
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
	if m.MapImage == nil {
		m.zoom = 1
	}

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

	m.imageOp = paint.NewImageOp(m.MapImage)
}
