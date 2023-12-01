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

type mapInput struct {
	click       f32.Point
	scaleFactor float64
	center      f32.Point
	transform   f32.Affine2D
	grabbed     bool
	cursor      image.Point
}

type Map2 struct {
	mapInput mapInput

	images   map[image.Point]*image.RGBA
	imageOps map[image.Point]paint.ImageOp
}

func (m *mapInput) HandlePointerEvent(e pointer.Event) {
	const WHEEL_DELTA = 120

	switch e.Kind {
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
		scaleFactor := math.Pow(1.01, float64(e.Scroll.Y))
		m.transform = m.transform.Scale(e.Position.Sub(m.center), f32.Pt(float32(scaleFactor), float32(scaleFactor)))
		m.scaleFactor *= scaleFactor
	}
}

func (m *mapInput) Layout(gtx layout.Context) func() {
	if m.scaleFactor == 0 {
		m.scaleFactor = 1
	}
	m.center = f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)).Div(2)

	for _, e := range gtx.Events(m) {
		if e, ok := e.(pointer.Event); ok {
			m.HandlePointerEvent(e)
		}
	}

	return func() {
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
			Grab:         true,
			Kinds:        pointer.Scroll | pointer.Drag | pointer.Press | pointer.Release,
			ScrollBounds: image.Rect(-size.X, -size.Y, size.X, size.Y),
		}.Add(gtx.Ops)
	}
}

func (m *Map2) Layout(gtx layout.Context) layout.Dimensions {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	defer m.mapInput.Layout(gtx)()

	for p, imageOp := range m.imageOps {
		scaledSize := tileSize * m.mapInput.scaleFactor
		pt := f32.Pt(float32(float64(p.X)*scaledSize), float32(float64(p.Y)*scaledSize))

		// check if this needs to be drawn
		if (image.Rectangle{Max: gtx.Constraints.Max}).Intersect(
			image.Rectangle{
				Min: pt.Round(),
				Max: pt.Add(f32.Pt(float32(scaledSize), float32(scaledSize))).Round(),
			}.Add(m.mapInput.center.Round()).Add(m.mapInput.transform.Transform(f32.Pt(0, 0)).Round()),
		).Empty() {
			continue
		}

		aff := op.Affine(m.mapInput.transform.Offset(m.mapInput.center).Offset(pt)).Push(gtx.Ops)
		imageOp.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		aff.Pop()
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
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
			Min: posInTile, Max: posInTile.Add(image.Pt(16, 16)),
		}, u.Chunks[cp], image.Point{}, draw.Src)
		updatedTiles = append(updatedTiles, tilePos)
	}

	for _, p := range updatedTiles {
		op := paint.NewImageOp(m.images[p])
		op.Filter = paint.FilterNearest
		m.imageOps[p] = op
	}
}
