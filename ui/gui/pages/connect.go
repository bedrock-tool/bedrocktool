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
	ifb := func(b bool, w layout.Widget) func(C) D {
		if b {
			return w
		}
		return func(c C) D {
			return D{}
		}
	}

	tf := func(b bool, text string) func(C) D {
		if b {
			return material.Label(th, 40, text).Layout
		}
		return func(c C) D {
			return D{}
		}
	}

	return layoutPopupBackground(gtx, th, "connect", func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(
					ifb(p.Listening, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Label(th, 40, "Listening").Layout),
							layout.Rigid(func(gtx C) D {
								if !(p.ClientConnecting || p.ServerConnecting) {
									return material.Body1(th, "connect to 127.0.0.1 or this devices local address\nin the minecraft bedrock client to continue").Layout(gtx)
								}
								return D{}
							}),
						)
					}),
				),
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
