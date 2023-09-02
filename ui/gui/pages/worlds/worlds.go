package worlds

import (
	"fmt"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	router *pages.Router

	worldMap   *Map2
	State      messages.UIState
	chunkCount int
	voidGen    bool
	worldName  string

	worldsList widget.List
	worlds     []*messages.SavedWorld
	l          sync.Mutex
}

func New(router *pages.Router) *Page {
	return &Page{
		router: router,
		worldMap: &Map2{
			images:   make(map[image.Point]*image.RGBA),
			imageOps: make(map[image.Point]paint.ImageOp),
		},
		worldsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return "worlds"
}

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

func displayWorldEntry(gtx C, th *material.Theme, entry *messages.SavedWorld) D {
	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
					layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d Chunks", entry.Chunks)).Layout),
				)
			}),
		)
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	switch p.State {
	case messages.UIStateMain:
		// show the main ui
		return p.worldMap.Layout(gtx)
	case messages.UIStateFinished:
		return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.UniformInset(15).
						Layout(gtx, material.Label(th, 20, "Worlds Saved").Layout)
				}),
				layout.Flexed(1, func(gtx C) D {
					p.l.Lock()
					defer p.l.Unlock()
					return material.List(th, &p.worldsList).Layout(gtx, len(p.worlds), func(gtx C, index int) D {
						entry := p.worlds[len(p.worlds)-index-1]
						return displayWorldEntry(gtx, th, entry)
					})
				}),
			)
		})
	default:
		return D{}
	}
}

func (u *Page) Handler(data any) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.SetUIState:
		u.State = m
		u.router.Invalidate()
		r.Ok = true
	case messages.UpdateMap:
		u.chunkCount = m.ChunkCount
		u.worldMap.Update(&m)
		u.router.Invalidate()
		r.Ok = true
	case messages.SetVoidGen:
		u.voidGen = m.Value
		u.router.Invalidate()
		r.Ok = true
	case messages.SetWorldName:
		u.worldName = m.WorldName
		u.router.Invalidate()
		r.Ok = true
	case messages.SavingWorld:
		u.l.Lock()
		u.worlds = append(u.worlds, m.World)
		u.l.Unlock()
		u.router.Invalidate()
		r.Ok = true
	}
	return r
}
