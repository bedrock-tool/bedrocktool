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

	listening        bool
	serverConnecting bool
	established      bool
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
		return ifb(b, material.Label(th, 40, text).Layout)
	}

	return LayoutPopupBackground(gtx, th, "connect", func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(
					ifb(p.listening, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Label(th, 40, "Listening").Layout),
							layout.Rigid(func(gtx C) D {
								if !p.serverConnecting {
									return material.Body1(th, "connect to 127.0.0.1 or this devices local address\nin the minecraft bedrock client to continue").Layout(gtx)
								}
								return D{}
							}),
						)
					}),
				),
				layout.Rigid(tf(p.serverConnecting, "Connecting to Server")),
				layout.Rigid(tf(p.established, "Established")),
			)
		})
	})
}

func (u *ConnectPopup) Handler(data any) messages.Response {
	r := messages.Response{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.ConnectState:
		switch m {
		case messages.ConnectStateListening:
			u.listening = true
		case messages.ConnectStateServerConnecting:
			u.serverConnecting = true
		case messages.ConnectStateEstablished:
			u.established = true
		case messages.ConnectStateDone:
			u.router.RemovePopup(u.ID())
		}
		u.router.Invalidate()
	}
	return r
}
