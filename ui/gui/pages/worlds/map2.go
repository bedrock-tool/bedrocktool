package worlds

import (
	"image"
	"image/draw"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type Map2 struct {
	click f32.Point

	scaleFactor float32
	center      f32.Point
	transform   f32.Affine2D
	grabbed     bool
	cursor      image.Point

	images   map[image.Point]*image.RGBA
	imageOps map[image.Point]paint.ImageOp

	BoundsMin protocol.ChunkPos
	BoundsMax protocol.ChunkPos
}

func (m *Map2) HandlePointerEvent(e pointer.Event) {
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
		scaleFactor := float32(math.Pow(1.01, float64(e.Scroll.Y)))
		m.transform = m.transform.Scale(e.Position.Sub(m.center), f32.Pt(scaleFactor, scaleFactor))
		m.scaleFactor *= scaleFactor
	}
}

func (m *Map2) Layout(gtx layout.Context) layout.Dimensions {
	m.center = f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)).Div(2)

	for _, e := range gtx.Events(m) {
		if e, ok := e.(pointer.Event); ok {
			m.HandlePointerEvent(e)
		}
	}

	for p, imageOp := range m.imageOps {
		pt := f32.Pt(float32(math.Floor(float64(p.X)*32)), float32(math.Floor(float64(p.Y)*32)))
		aff := op.Affine(m.transform.Offset(m.center).Offset(pt)).Push(gtx.Ops)
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

	/*
		if m.MapImage != nil {
			// Calculate the size of the widget based on the size of the image and the current scale factor.
			dx := float32(m.MapImage.Bounds().Dx())
			dy := float32(m.MapImage.Bounds().Dy())
			size := f32.Pt(dx*m.scaleFactor, dy*m.scaleFactor)

			// Draw the image at the correct position and scale.
			defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
			aff := op.Affine(m.transform.Offset(m.center.Sub(size.Div(2)))).Push(gtx.Ops)
			m.imageOp.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			aff.Pop()
		}
	*/

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
	tile.X = int(cp.X()*16) / 32
	tile.Y = int(cp.Z()*16) / 32
	offset.X = int(cp.X()*16) % 32
	offset.Y = int(cp.Z()*16) % 32
	return
}

func (m *Map2) Update(u *messages.UpdateMap) {
	var updatedTiles []image.Point
	for _, cp := range u.UpdatedChunks {
		tilePos, posInTile := chunkPosToTilePos(cp)
		img, ok := m.images[tilePos]
		if !ok {
			img = image.NewRGBA(image.Rect(0, 0, 32, 32))
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
