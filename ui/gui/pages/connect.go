package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type ConnectPopup struct {
	router *Router

	Listening        bool
	ClientConnecting bool
	ServerConnecting bool
	Established      bool
}

func NewConnect(router *Router) Popup {
	return &ConnectPopup{
		router: router,
	}
}

func (p *ConnectPopup) ID() string {
	return "connect"
}

func (p *ConnectPopup) Layout(gtx C, th *material.Theme) D {
	tf := func(b bool, text string) func(C) D {
		if b {
			return material.Label(th, 40, text).Layout
		}
		return func(c C) D {
			return D{}
		}
	}

	return layoutPopupBackground(gtx, th, "connect", func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(tf(p.Listening, "Listening")),
				layout.Rigid(tf(p.ClientConnecting, "Client Connecting")),
				layout.Rigid(tf(p.ServerConnecting, "Connecting to Server")),
				layout.Rigid(tf(p.Established, "Established")),
			)
		})
	})
}

func (u *ConnectPopup) Handler(data any) messages.MessageResponse {
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
			u.router.RemovePopup(u.ID())
		}
		u.router.Invalidate()
	}
	return r
}
