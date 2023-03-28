package worlds

import (
	"fmt"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	*pages.Router

	worldMap   *Map
	State      messages.UIState
	chunkCount int
	voidGen    bool
	worldName  string

	worldsList widget.List
	worlds     []messages.SavingWorldPayload
	l          sync.Mutex
}

func New(router *pages.Router) *Page {
	return &Page{
		Router:   router,
		worldMap: &Map{},
		worldsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
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
	case messages.UIStateConnect:
		// display login page
		return margin.Layout(gtx, material.Label(th, 100, "connect Client").Layout)
	case messages.UIStateConnecting:
		// display connecting to server
		return margin.Layout(gtx, material.Label(th, 100, "Connecting").Layout)
	case messages.UIStateMain:
		// show the main ui
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, p.worldMap.Layout)
			}),
		)
	case messages.UIStateFinished:
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.UniformInset(20).
					Layout(gtx, material.Label(th, 20, "Worlds Saved").Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				p.l.Lock()
				defer p.l.Unlock()
				return material.List(th, &p.worldsList).Layout(gtx, len(p.worlds), func(gtx C, index int) D {
					entry := p.worlds[len(p.worlds)-index-1]
					return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Dimensions{Size: image.Pt(20, 20)}
							}),
							layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d chunks", entry.Chunks)).Layout),
						)
					})
				})
			}),
		)
	}

	return layout.Dimensions{}
}

func (u *Page) handler(name string, data interface{}) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case messages.SetUIState:
		state := data.(messages.UIState)
		u.State = state
		u.Router.Invalidate()
		r.Ok = true

	case messages.Init:
		init := data.(messages.InitPayload)
		_ = init
		r.Ok = true

	case messages.UpdateMap:
		update_map := data.(messages.UpdateMapPayload)
		u.chunkCount = update_map.ChunkCount
		u.worldMap.Update(&update_map)
		u.Router.Invalidate()
		r.Ok = true

	case messages.SetVoidGen:
		set_void_gen := data.(messages.SetVoidGenPayload)
		u.voidGen = set_void_gen.Value
		u.Router.Invalidate()
		r.Ok = true

	case messages.SetWorldName:
		set_world_name := data.(messages.SetWorldNamePayload)
		u.worldName = set_world_name.WorldName
		u.Router.Invalidate()
		r.Ok = true

	case messages.SavingWorld:
		u.l.Lock()
		saving_world := data.(messages.SavingWorldPayload)
		u.worlds = append(u.worlds, saving_world)
		u.l.Unlock()
		u.Router.Invalidate()
		r.Ok = true
	}
	return r
}

func (p *Page) Handler() gui.HandlerFunc {
	return p.handler
}
