package skins

import (
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
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
	*pages.Router

	State     messages.UIState
	SkinsList widget.List
	l         sync.Mutex
	Skins     []messages.NewSkin
}

func New(router *pages.Router) *Page {
	return &Page{
		Router: router,
		SkinsList: widget.List{
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
		Name: "Skin Grabber",
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
		return margin.Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis: layout.Vertical,
			}.Layout(gtx,
				layout.Rigid(material.Label(th, 20, "Skin Basic UI").Layout),
				layout.Flexed(1, func(gtx C) D {
					p.l.Lock()
					defer p.l.Unlock()
					return material.List(th, &p.SkinsList).Layout(gtx, len(p.Skins), func(gtx C, index int) D {
						entry := p.Skins[len(p.Skins)-index-1]
						return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(material.Label(th, th.TextSize, entry.PlayerName).Layout),
							)
						})
					})
				}),
			)
		})
	}

	return layout.Flex{}.Layout(gtx)
}

func (p *Page) Handler(data interface{}) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.SetUIState:
		p.State = m
		p.Router.Invalidate()
		r.Ok = true
	case messages.NewSkin:
		p.l.Lock()
		p.Skins = append(p.Skins, m)
		p.l.Unlock()
		p.Router.Invalidate()
		r.Ok = true
	}
	return r
}
