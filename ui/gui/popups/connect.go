package popups

import (
	"fmt"
	"net"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

type ConnectPopup struct {
	g     guim.Guim
	state string
	close widget.Clickable

	listenIP   string
	listenPort string
	localIP    string

	connectButton widget.Clickable
}

func NewConnect(g guim.Guim, listenAddr string) Popup {
	listenIp, listenPort, _ := net.SplitHostPort(listenAddr)
	if listenIp == "0.0.0.0" {
		listenIp = "127.0.0.1"
	}

	localIP, err := utils.GetLocalIP()
	if err != nil {
		logrus.Error(err)
	}

	return &ConnectPopup{
		g:          g,
		listenIP:   listenIp,
		listenPort: listenPort,
		localIP:    localIP,
	}
}

func (*ConnectPopup) ID() string {
	return "connect"
}

func (*ConnectPopup) Close() error {
	return nil
}

func (p *ConnectPopup) Layout(gtx C, th *material.Theme) D {
	if p.connectButton.Clicked(gtx) {
		p.g.OpenUrl(fmt.Sprintf("minecraft://connect/?serverUrl=%s&serverPort=%s", p.listenIP, p.listenPort))
	}

	if p.close.Clicked(gtx) {
		p.g.ClosePopup(p.ID())
		p.g.ExitSubcommand()
	}

	var connectStr string
	connectStr += p.listenIP
	if p.localIP != "" {
		connectStr += " or " + p.localIP
	}
	if p.listenPort != "19132" {
		connectStr += " with port " + p.listenPort
	}

	return LayoutPopupBackground(gtx, th, "connect", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							switch p.state {
							case "listening":
								return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(material.Label(th, 40, "Listening").Layout),
									layout.Rigid(material.Body1(th, fmt.Sprintf("connect to %s", connectStr)).Layout),
									layout.Rigid(material.Body1(th, "in minecraft bedrock to continue").Layout),
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
				gtx.Constraints.Max.X /= 2

				return layout.Flex{
					Axis:      layout.Horizontal,
					Spacing:   layout.SpaceBetween,
					Alignment: layout.End,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						b := material.Button(th, &p.close, "Close")
						b.CornerRadius = 8
						return b.Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						if p.state == "listening" {
							return layout.Flex{
								Axis:      layout.Horizontal,
								Alignment: layout.Middle,
							}.Layout(gtx,
								layout.Flexed(1, func(gtx C) D {
									b := material.Button(th, &p.connectButton, "Open Minecraft")
									b.CornerRadius = 8
									return b.Layout(gtx)
								}),
							)
						}
						return D{}
					}),
				)
			}),
		)
	})
}

func (p *ConnectPopup) HandleEvent(event any) error {
	switch event := event.(type) {
	case *messages.EventConnectStateUpdate:
		switch event.State {
		case messages.ConnectStateListening:
			p.state = "listening"
		case messages.ConnectStateServerConnecting:
			p.state = "connecting-server"
		case messages.ConnectStateEstablished:
			p.state = "established"
		case messages.ConnectStateDone:
			p.g.ClosePopup(p.ID())
		}
	}
	return nil
}
