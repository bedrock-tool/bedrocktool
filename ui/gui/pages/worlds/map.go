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

type Map struct {
	click   f32.Point
	imageOp paint.ImageOp

	scaleFactor float32
	center      f32.Point
	transform   f32.Affine2D
	grabbed     bool
	cursor      image.Point

	MapImage  *image.RGBA
	BoundsMin protocol.ChunkPos
	BoundsMax protocol.ChunkPos
}

func (m *Map) HandlePointerEvent(e pointer.Event) {
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

func (m *Map) Layout(gtx layout.Context) layout.Dimensions {
	m.center = f32.Pt(float32(gtx.Constraints.Max.X), float32(gtx.Constraints.Max.Y)).Div(2)

	for _, e := range gtx.Events(m) {
		if e, ok := e.(pointer.Event); ok {
			m.HandlePointerEvent(e)
		}
	}

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

		if m.cursor.In(image.Rectangle(gtx.Constraints)) {
			if m.grabbed {
				pointer.CursorGrabbing.Add(gtx.Ops)
			} else {
				pointer.CursorGrab.Add(gtx.Ops)
			}
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

func drawTile(img *image.RGBA, min, pos protocol.ChunkPos, tile *image.RGBA) {
	px := image.Pt(
		int((pos.X()-min[0])*16),
		int((pos.Z()-min[1])*16),
	)
	draw.Draw(img, image.Rect(
		px.X, px.Y,
		px.X+16, px.Y+16,
	), tile, image.Point{}, draw.Src)
}

func (m *Map) Update(u *messages.UpdateMap) {
	if m.MapImage == nil {
		m.scaleFactor = 1
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
		for pos, tile := range u.Chunks {
			drawTile(m.MapImage, m.BoundsMin, pos, tile)
		}
	} else {
		for _, pos := range u.UpdatedChunks {
			drawTile(m.MapImage, m.BoundsMin, pos, u.Chunks[pos])
		}
	}

	m.imageOp = paint.NewImageOpFilter(m.MapImage, paint.FilterNearest)
}
