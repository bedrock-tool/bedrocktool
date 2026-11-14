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
		if p.state != "established" {
			// before the game is in, closing stops the download
			p.g.ExitSubcommand()
		}
	}

	muted := func(l material.LabelStyle) material.LabelStyle {
		l.Color.A = 170
		return l
	}
	addressLine := func(gtx C, where, ip string) D {
		return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Dp(150)
					return muted(material.Body1(th, where)).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					l := material.Body1(th, fmt.Sprintf("%s   port %s", ip, p.listenPort))
					return l.Layout(gtx)
				}),
			)
		})
	}

	var title, detail string
	switch p.state {
	case "":
		title, detail = "Starting", "Logging in and finding the server..."
	case "listening":
		title = "Join from Minecraft"
	case "connecting-server":
		title, detail = "Connecting to the server", "Hold on in Minecraft: the server's resource packs are downloaded first, which can take a minute the first time."
	case "receiving-resources":
		title, detail = "Downloading resource packs", "Hold on in Minecraft, this can take a minute the first time."
	case "established":
		title, detail = "Connected", "Play as usual. The world is saved when you leave."
	}

	return LayoutPopupBackground(gtx, th, "connect", func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, title).Layout)
			}),
			layout.Rigid(func(gtx C) D {
				if p.state != "listening" {
					return material.Body1(th, detail).Layout(gtx)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body1(th, "In Minecraft, go to Servers, Add Server, and use:").Layout),
					layout.Rigid(func(gtx C) D { return addressLine(gtx, "On this computer", p.listenIP) }),
					layout.Rigid(func(gtx C) D {
						if p.localIP == "" {
							return D{}
						}
						return addressLine(gtx, "On another device", p.localIP)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx C) D {
				label := "Cancel"
				if p.state == "established" {
					label = "Hide"
				}
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &p.close, label) }),
					layout.Rigid(func(gtx C) D {
						if p.state != "listening" {
							return D{}
						}
						b := material.Button(th, &p.connectButton, "Open Minecraft")
						b.CornerRadius = Radius
						return b.Layout(gtx)
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
		case messages.ConnectStateReceivingResources:
			p.state = "receiving-resources"
		case messages.ConnectStateEstablished:
			p.state = "established"
		case messages.ConnectStateDone:
			p.g.ClosePopup(p.ID())
		}
	}
	return nil
}
