package worlds

import (
	"image"
	"image/draw"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const tileSize = 256

type Map2 struct {
	click f32.Point

	scaleFactor float32
	center      f32.Point
	transform   f32.Affine2D
	grabbed     bool
	cursor      image.Point

	images   map[image.Point]*image.RGBA
	imageOps map[image.Point]paint.ImageOp
}

func (m *Map2) HandlePointerEvent(e pointer.Event) {
	const WHEEL_DELTA = 120

	switch e.Type {
	case pointer.Press:
		m.click = e.Position
		m.grabbed = true
	case pointer.Drag:
		m.transform = m.transform.Offset(e.Position.Sub(m.click))
		m.click = e.Position
	case pointer.Release:
		m.grabbed = false
	case pointer.Scroll:
		if int(e.Scroll.Y)%WHEEL_DELTA == 0 {
			e.Scroll.Y = -8 * e.Scroll.Y / WHEEL_DELTA
		}
		scaleFactor := float32(math.Pow(1.01, float64(e.Scroll.Y)))
		m.transform = m.transform.Scale(e.Position.Sub(m.center), f32.Pt(scaleFactor, scaleFactor))
		m.scaleFactor *= scaleFactor
	}
}

func (m *Map2) Layout(gtx layout.Context) layout.Dimensions {
	if m.scaleFactor == 0 {
		m.scaleFactor = 1
	}
	m.center = f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)).Div(2)
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()

	for _, e := range gtx.Events(m) {
		if e, ok := e.(pointer.Event); ok {
			m.HandlePointerEvent(e)
		}
	}

	for p, imageOp := range m.imageOps {
		pt := f32.Pt(float32(float64(p.X)*tileSize*float64(m.scaleFactor)), float32(float64(p.Y)*tileSize*float64(m.scaleFactor)))
		scaledSize := tileSize * m.scaleFactor

		// check if this needs to be drawn
		r2 := image.Rectangle{
			Min: pt.Round(),
			Max: pt.Add(f32.Pt(scaledSize, scaledSize)).Round(),
		}.Add(m.center.Round()).Add(m.transform.Transform(f32.Pt(0, 0)).Round())
		if (image.Rectangle{Max: gtx.Constraints.Max}).Intersect(r2).Empty() {
			continue
		}

		t := m.transform.Offset(m.center).Offset(pt)
		aff := op.Affine(t).Push(gtx.Ops)
		imageOp.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		aff.Pop()
	}

	if m.cursor.In(image.Rectangle(gtx.Constraints)) {
		if m.grabbed {
			pointer.CursorGrabbing.Add(gtx.Ops)
		} else {
			pointer.CursorGrab.Add(gtx.Ops)
		}
	}

	size := gtx.Constraints.Max
	pointer.InputOp{
		Tag:          m,
		Grab:         m.grabbed,
		Types:        pointer.Scroll | pointer.Drag | pointer.Press | pointer.Release,
		ScrollBounds: image.Rect(-size.X, -size.Y, size.X, size.Y),
	}.Add(gtx.Ops)

	return layout.Dimensions{Size: size}
}

func chunkPosToTilePos(cp protocol.ChunkPos) (tile image.Point, offset image.Point) {
	blockX := int(cp.X()) * 16
	blockY := int(cp.Z()) * 16
	tile.X = blockX / tileSize
	tile.Y = blockY / tileSize

	offset.X = blockX % tileSize
	offset.Y = blockY % tileSize

	if blockX < 0 && offset.X != 0 {
		tile.X--
		offset.X += tileSize
	}
	if blockY < 0 && offset.Y != 0 {
		tile.Y--
		offset.Y += tileSize
	}

	return
}

func (m *Map2) Update(u *messages.UpdateMap) {
	if u.ChunkCount == -1 {
		m.images = make(map[image.Point]*image.RGBA)
		m.imageOps = make(map[image.Point]paint.ImageOp)
		return
	}

	var updatedTiles []image.Point
	for _, cp := range u.UpdatedChunks {
		tilePos, posInTile := chunkPosToTilePos(cp)
		img, ok := m.images[tilePos]
		if !ok {
			img = image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
			m.images[tilePos] = img
		}
		draw.Draw(img, image.Rectangle{
			Min: posInTile,
			Max: posInTile.Add(image.Pt(16, 16)),
		}, u.Chunks[cp], image.Point{}, draw.Src)
		updatedTiles = append(updatedTiles, tilePos)
	}

	for _, p := range updatedTiles {
		m.imageOps[p] = paint.NewImageOpFilter(m.images[p], paint.FilterNearest)
	}
}
