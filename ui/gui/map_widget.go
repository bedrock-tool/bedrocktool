package gui

import (
	"image"
	"image/draw"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type mapWidget struct {
	widget.BaseWidget

	GetTiles  func() map[protocol.ChunkPos]*image.RGBA
	GetBounds func() (min, max protocol.ChunkPos)

	pixels image.Image
	w, h   int
}

func (m *mapWidget) MinSize() fyne.Size {
	return fyne.NewSize(128, 128)
}

func (m *mapWidget) CreateRenderer() fyne.WidgetRenderer {
	m.ExtendBaseWidget(m)
	c := container.NewMax(canvas.NewRaster(m.draw))
	return widget.NewSimpleRenderer(c)
}

func (m *mapWidget) draw(w, h int) image.Image {
	if m.w != w || m.h != h {
		m.pixels = image.NewNRGBA(image.Rect(0, 0, w, h))
	}

	if m.GetBounds == nil {
		return m.pixels
	}

	min, max := m.GetBounds()
	//chunksX := int(max[0] - min[0] + 1) // how many chunk lengths is x coordinate
	//chunksY := int(max[1] - min[1] + 1)
	_ = max

	for pos, tile := range m.GetTiles() {
		px := image.Pt(
			int((pos[0]-min[0])*16),
			int((pos[1]-min[1])*16),
		)
		draw.Draw(m.pixels.(*image.NRGBA), image.Rect(
			px.X, px.Y,
			px.X+16, px.Y+16,
		), tile, image.Pt(0, 0), draw.Src)
	}

	return m.pixels
}
