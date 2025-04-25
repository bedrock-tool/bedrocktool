package worlds

import (
	"image"
	"image/draw"
	"math"
	"sync"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const tileSize = 256

type mapInput struct {
	click          f32.Point
	scaleFactor    float64
	center         f32.Point
	transform      f32.Affine2D
	grabbed        bool
	cursor         image.Point
	FollowPlayer   widget.Bool
	playerPosition mgl32.Vec3
}

type Map2 struct {
	mapInput mapInput

	tileImages map[image.Point]*image.RGBA
	imageOps   map[image.Point]paint.ImageOp
	l          sync.Mutex
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

func (m *mapInput) Layout(gtx C) func() {
	if m.scaleFactor == 0 {
		m.scaleFactor = 1
	}
	m.center = f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)).Div(2)

	//size := gtx.Constraints.Max

	event.Op(gtx.Ops, m)
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: m,
			Kinds:  pointer.Scroll | pointer.Drag | pointer.Press | pointer.Release,
			ScrollY: pointer.ScrollRange{
				Min: -120,
				Max: 120,
			},
		})
		if !ok {
			break
		}
		m.HandlePointerEvent(ev.(pointer.Event))
	}

	/*
		if m.FollowPlayer.Value {
		}
	*/

	return func() {
		if m.cursor.In(image.Rectangle(gtx.Constraints)) {
			if m.grabbed {
				pointer.CursorGrabbing.Add(gtx.Ops)
			} else {
				pointer.CursorGrab.Add(gtx.Ops)
			}
		}
	}
}

func (m *Map2) Layout(gtx C) D {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	defer m.mapInput.Layout(gtx)()
	m.l.Lock()
	defer m.l.Unlock()

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

	return D{Size: gtx.Constraints.Max}
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

func (m *Map2) AddTiles(tiles []messages.MapTile) {
	var updatedTiles []image.Point
	for _, mapTile := range tiles {
		tilePos, posInTile := chunkPosToTilePos(mapTile.Pos)
		tileImg, ok := m.tileImages[tilePos]
		if !ok {
			tileImg = image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
			m.tileImages[tilePos] = tileImg
		}
		draw.Draw(tileImg, image.Rectangle{
			Min: posInTile, Max: posInTile.Add(image.Pt(16, 16)),
		}, &mapTile.Img, image.Point{}, draw.Src)
		updatedTiles = append(updatedTiles, tilePos)
	}

	for _, p := range updatedTiles {
		op := paint.NewImageOp(m.tileImages[p])
		op.Filter = paint.FilterNearest
		m.imageOps[p] = op
	}
}

func (m *Map2) Reset() {
	m.l.Lock()
	defer m.l.Unlock()
	m.tileImages = make(map[image.Point]*image.RGBA)
	m.imageOps = make(map[image.Point]paint.ImageOp)
}
