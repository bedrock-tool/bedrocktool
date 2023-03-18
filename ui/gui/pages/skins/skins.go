package skins

import (
	"gioui.org/layout"
	"gioui.org/unit"
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

	State messages.UIState
}

func New(router *pages.Router) *Page {
	return &Page{
		Router: router,
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
			)
		})
	}

	return layout.Flex{}.Layout(gtx)
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

	}
	return r
}

func (p *Page) Handler() gui.HandlerFunc {
	return p.handler
}
