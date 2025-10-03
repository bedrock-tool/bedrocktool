package settings

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/discovery"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type addressInput struct {
	g              guim.Guim
	editor         widget.Editor
	showRealmsList widget.Clickable
	showGatherings widget.Clickable

	connectInfo *connectinfo.ConnectInfo
}

var AddressInput = &addressInput{
	g: nil,
	editor: widget.Editor{
		SingleLine: true,
	},
	connectInfo: &connectinfo.ConnectInfo{},
}

func (a *addressInput) SetGuim(g guim.Guim) {
	a.g = g
}

func (a *addressInput) GetConnectInfo() *connectinfo.ConnectInfo {
	t := a.editor.Text()
	if len(t) == 0 {
		return nil
	}
	a.connectInfo.Value = t
	a.connectInfo.Account = auth.Auth.Account()
	return a.connectInfo
}

func (a *addressInput) Layout(gtx C, th *material.Theme) D {
	if a.showRealmsList.Clicked(gtx) {
		a.g.ShowPopup(popups.NewRealmsList(a.g, func(realm *realms.Realm) {
			a.connectInfo.SetRealm(realm)
			a.editor.SetText(a.connectInfo.Value)
		}))
	}

	if a.showGatherings.Clicked(gtx) {
		a.g.ShowPopup(popups.NewGatherings(a.g, func(gathering *discovery.Gathering) {
			a.connectInfo.SetGathering(gathering)
			a.editor.SetText(a.connectInfo.Value)
		}))
	}

	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				macro := op.Record(gtx.Ops)
				d := layout.UniformInset(8).Layout(gtx, func(gtx C) D {
					e := material.Editor(th, &a.editor, "Enter Server Address")
					e.LineHeight += 4
					return e.Layout(gtx)
				})
				c := macro.Stop()
				component.Rect{
					Color: component.WithAlpha(th.Fg, 10),
					Size:  d.Size,
					Radii: 8,
				}.Layout(gtx)
				c.Add(gtx.Ops)
				return d
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X = gtx.Dp(unit.Dp(200))
				return layout.Flex{
					Axis:      layout.Horizontal,
					WeightSum: 2,
				}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return layout.Inset{
							Top:    5,
							Bottom: 5,
							Left:   0,
							Right:  5,
						}.Layout(gtx, material.Button(th, &a.showRealmsList, "Realms").Layout)
					}),
					layout.Flexed(1, func(gtx C) D {
						return layout.Inset{
							Top:    5,
							Bottom: 5,
							Left:   0,
							Right:  5,
						}.Layout(gtx, material.Button(th, &a.showGatherings, "Events").Layout)
					}),
				)
			}),
		)
	})
}
