package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type ConnectPopup struct {
	ui    ui.UI
	state string
	close widget.Clickable
}

func NewConnect(ui ui.UI) Popup {
	return &ConnectPopup{ui: ui}
}

func (p *ConnectPopup) ID() string {
	return "connect"
}

func (p *ConnectPopup) Layout(gtx C, th *material.Theme) D {
	if p.close.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: p.ID(),
			Target: "ui",
			Data:   messages.ExitSubcommand{},
		})
		messages.Router.Handle(&messages.Message{
			Source: p.ID(),
			Target: "ui",
			Data:   messages.Close{Type: "popup", ID: p.ID()},
		})
	}

	return LayoutPopupBackground(gtx, th, "connect", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							switch p.state {
							case "listening":
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(material.Label(th, 40, "Listening").Layout),
									layout.Rigid(material.Body1(th, "connect to 127.0.0.1 or this devices local address\nin the minecraft bedrock client to continue").Layout),
								)
							case "connecting-server":
								return material.Label(th, 40, "Connecting to Server").Layout(gtx)
							case "established":
								return material.Label(th, 40, "Established").Layout(gtx)
							}
							return D{}
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &p.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})
}

func (p *ConnectPopup) HandleMessage(msg *messages.Message) *messages.Message {
	switch m := msg.Data.(type) {
	case messages.ConnectState:
		switch m {
		case messages.ConnectStateListening:
			p.state = "listening"
		case messages.ConnectStateServerConnecting:
			p.state = "connecting-server"
		case messages.ConnectStateEstablished:
			p.state = "established"
		case messages.ConnectStateDone:
			messages.Router.Handle(&messages.Message{
				Source: p.ID(),
				Target: "ui",
				Data:   messages.Close{Type: "popup", ID: p.ID()},
			})
		}
	}
	return nil
}
