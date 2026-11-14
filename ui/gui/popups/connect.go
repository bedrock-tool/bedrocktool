package popups

import (
	"fmt"
	"github.com/bedrock-tool/bedrocktool/locale"
	"net"

	"gioui.org/layout"
	"gioui.org/op"
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
	thisComputer := locale.Loc("gui_connect_this_computer", nil)
	otherDevice := locale.Loc("gui_connect_other_device", nil)
	// the labels' column is as wide as the longest label, which depends
	// on the language
	labelWidth := gtx.Dp(150)
	for _, s := range []string{thisComputer, otherDevice} {
		m := op.Record(gtx.Ops)
		mgtx := gtx
		mgtx.Constraints.Min.X = 0
		w := material.Body1(th, s).Layout(mgtx).Size.X + gtx.Dp(16)
		m.Stop()
		labelWidth = max(labelWidth, w)
	}
	addressLine := func(gtx C, where, ip string) D {
		return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = min(labelWidth, gtx.Constraints.Max.X)
					return muted(material.Body1(th, where)).Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					l := material.Body1(th, locale.Loc("gui_connect_address_port", locale.Strmap{"Ip": ip, "Port": p.listenPort}))
					return l.Layout(gtx)
				}),
			)
		})
	}

	var title, detail string
	switch p.state {
	case "":
		title, detail = locale.Loc("gui_connect_starting", nil), locale.Loc("gui_connect_starting_detail", nil)
	case "listening":
		title = locale.Loc("gui_connect_title", nil)
	case "connecting-server":
		title, detail = locale.Loc("gui_connect_connecting", nil), locale.Loc("gui_connect_connecting_detail", nil)
	case "receiving-resources":
		title, detail = locale.Loc("gui_connect_resources", nil), locale.Loc("gui_connect_resources_detail", nil)
	case "established":
		title, detail = locale.Loc("gui_connect_connected", nil), locale.Loc("gui_connect_connected_detail", nil)
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
					layout.Rigid(material.Body1(th, locale.Loc("gui_connect_instructions", nil)).Layout),
					layout.Rigid(func(gtx C) D { return addressLine(gtx, thisComputer, p.listenIP) }),
					layout.Rigid(func(gtx C) D {
						if p.localIP == "" {
							return D{}
						}
						return addressLine(gtx, otherDevice, p.localIP)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx C) D {
				label := locale.Loc("gui_cancel", nil)
				if p.state == "established" {
					label = locale.Loc("gui_connect_hide", nil)
				}
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &p.close, label) }),
					layout.Rigid(func(gtx C) D {
						if p.state != "listening" {
							return D{}
						}
						b := material.Button(th, &p.connectButton, locale.Loc("gui_connect_open_minecraft", nil))
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
