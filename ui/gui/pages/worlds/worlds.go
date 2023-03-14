package worlds

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	*pages.Router

	worldMap   *Map
	State      utils.UIState
	chunkCount int
	voidGen    bool
	worldName  string
}

func New(router *pages.Router) *Page {
	return &Page{
		Router:   router,
		worldMap: &Map{},
	}
}

var _ pages.Page = &Page{}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "World Downloader",
		//Icon: icon.OtherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	margin := layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}

	switch p.State {
	case utils.UIStateConnect:
		// display login page
		return margin.Layout(gtx, material.Label(th, 100, "connect Client").Layout)
	case utils.UIStateConnecting:
		// display connecting to server
		return margin.Layout(gtx, material.Label(th, 100, "Connecting").Layout)
	case utils.UIStateMain:
		// show the main ui
		return margin.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(material.Label(th, 20, "World Downloader Basic UI").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, p.worldMap.Layout)
				}),
			)
		})
	}

	return layout.Flex{}.Layout(gtx)
}

func (u *Page) handler(name string, data interface{}) utils.MessageResponse {
	r := utils.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case utils.SetUIStateName:
		state := data.(utils.UIState)
		u.State = state
		u.Router.Invalidate()
		r.Ok = true

	case utils.InitName:
		init := data.(utils.InitPayload)
		_ = init
		r.Ok = true

	case utils.UpdateMapName:
		update_map := data.(utils.UpdateMapPayload)
		u.chunkCount = update_map.ChunkCount
		u.worldMap.Update(&update_map)
		u.Router.Invalidate()
		r.Ok = true

	case utils.SetVoidGenName:
		set_void_gen := data.(utils.SetVoidGenPayload)
		u.voidGen = set_void_gen.Value
		u.Router.Invalidate()
		r.Ok = true

	case utils.SetWorldNameName:
		set_world_name := data.(utils.SetWorldNamePayload)
		u.worldName = set_world_name.WorldName
		u.Router.Invalidate()
		r.Ok = true

	}
	return r
}

func (p *Page) Handler() gui.HandlerFunc {
	return p.handler
}
