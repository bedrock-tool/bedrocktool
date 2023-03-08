package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type worldsUI struct {
	mapElement    *mapWidget
	pageB         binding.String
	voidGen       binding.Bool
	worldName     binding.String
	chunkCount    binding.Int
	returnMessage HandlerFunc

	logs *consoleWidget
}

func (u *worldsUI) Layout(w fyne.Window) {
	u.mapElement = &mapWidget{}
	u.logs = newConsoleWidget()

	u.voidGen = binding.NewBool()
	u.voidGen.AddListener(binding.NewDataListener(func() {
		val, _ := u.voidGen.Get()
		u.returnMessage(utils.SetVoidGenName, utils.SetVoidGenPayload{Value: val})
	}))
	u.worldName = binding.NewString()
	u.worldName.Set("world")
	u.worldName.AddListener(binding.NewDataListener(func() {
		val, _ := u.worldName.Get()
		u.returnMessage(utils.SetWorldNameName, utils.SetWorldNamePayload{WorldName: val})
	}))
	u.chunkCount = binding.NewInt()

	u.pageB = binding.NewString()
	u.pageB.AddListener(binding.NewDataListener(func() {
		page, _ := u.pageB.Get()
		switch page {
		case "login":
		case "connect":
			w.SetContent(container.NewVBox(
				widget.NewRichTextFromMarkdown("# connect"),
				u.logs,
			))
		case "connecting":
			w.SetContent(container.NewVBox(
				widget.NewRichTextFromMarkdown("# Connecting"),
				u.logs,
			))
		case "map":
			w.SetContent(container.NewVBox(
				container.NewCenter(
					widget.NewRichTextFromMarkdown("# World Downloader"),
				),
				container.NewHBox(
					widget.NewCheckWithData("void generator", u.voidGen),
					widget.NewRichTextWithText("world name:"),
					widget.NewEntryWithData(u.worldName),
					widget.NewLabelWithData(binding.NewSprintf("Chunks: %d", u.chunkCount)),
				),
				container.NewMax(u.mapElement),
				container.NewMax(u.logs),
			))
		}
	}))
	u.pageB.Set("connect")
}

func (u *worldsUI) handler(name string, data interface{}) utils.MessageResponse {
	r := utils.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case "need_login":
		u.pageB.Set("login")
	case utils.InitName:
		init := data.(utils.InitPayload)
		u.returnMessage = HandlerFunc(init.Handler)

	case utils.InitMapName:
		init_map := data.(utils.InitMapPayload)
		u.mapElement.GetBounds = init_map.GetBounds
		u.mapElement.GetTiles = init_map.GetTiles
		r.Ok = true
		u.pageB.Set("map")

	case utils.UpdateMapName:
		update_map := data.(utils.UpdateMapPayload)
		u.chunkCount.Set(update_map.ChunkCount)
		u.mapElement.Refresh()
		r.Ok = true

	case utils.SetVoidGenName:
		set_void_gen := data.(utils.SetVoidGenPayload)
		u.voidGen.Set(set_void_gen.Value)

	case utils.SetWorldNameName:
		set_world_name := data.(utils.SetWorldNamePayload)
		u.worldName.Set(set_world_name.WorldName)
	}
	return r
}

func (w *worldsUI) Handler() HandlerFunc {
	return w.handler
}

func dummyHandler(name string, data interface{}) utils.MessageResponse {
	return utils.MessageResponse{}
}

func init() {
	CommandUIs["worlds"] = &worldsUI{
		returnMessage: dummyHandler,
	}
}
