package connect

import (
	"gioui.org/layout"
	"gioui.org/unit"
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
	router         *pages.Router
	afterEstablish pages.Page

	Listening        bool
	ClientConnecting bool
	ServerConnecting bool
	Established      bool
}

func New(router *pages.Router, afterEstablish pages.Page) pages.Page {
	return &Page{
		router:         router,
		afterEstablish: afterEstablish,
	}
}

func init() {
	pages.NewConnect = New
}

func (p *Page) ID() string {
	return "connect"
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
		Name: p.afterEstablish.NavItem().Name + " (pre)",
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	tf := func(b bool, text string) func(C) D {
		if b {
			return material.Label(th, 80, text).Layout
		}
		return func(c C) D {
			return D{}
		}
	}
	return layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(tf(p.Listening, "Listening")),
			layout.Rigid(tf(p.ClientConnecting, "Client Connecting")),
			layout.Rigid(tf(p.ServerConnecting, "Server Connectinh")),
			layout.Rigid(tf(p.Established, "Established")),
		)
	})
}

func (u *Page) Handler(data any) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.ConnectState:
		switch m {
		case messages.ConnectStateListening:
			u.Listening = true
		case messages.ConnectStateClientConnecting:
			u.ClientConnecting = true
		case messages.ConnectStateServerConnecting:
			u.ServerConnecting = true
		case messages.ConnectStateEstablished:
			u.Established = true
		case messages.ConnectStateDone:
			u.router.SwitchTo(u.afterEstablish.ID())
		}
		u.router.Invalidate()
	}
	return r
}
