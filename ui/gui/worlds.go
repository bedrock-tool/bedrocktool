package gui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type worldsUI struct {
	mapElement *mapWidget
}

func (w *worldsUI) Layout() fyne.CanvasObject {
	w.mapElement = &mapWidget{}
	return container.NewVBox(
		widget.NewRichTextFromMarkdown("# worlds Ui!"),
		w.mapElement,
	)
}

func (w *worldsUI) handler(name string, data interface{}) utils.MessageResponse {
	r := utils.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case "init_map":
		init_map := data.(struct {
			GetTiles  func() map[protocol.ChunkPos]*image.RGBA
			GetBounds func() (min, max protocol.ChunkPos)
		})
		w.mapElement.GetBounds = init_map.GetBounds
		w.mapElement.GetTiles = init_map.GetTiles
		r.Ok = true
	case "update_map":
		w.mapElement.Refresh()
		r.Ok = true
	}

	return r
}

func (w *worldsUI) Handler() HandlerFunc {
	return w.handler
}

func init() {
	CommandUIs["worlds"] = &worldsUI{}
}
