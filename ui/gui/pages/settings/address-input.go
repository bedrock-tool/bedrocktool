package settings

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/gatherings"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type addressInput struct {
	editor         widget.Editor
	showRealmsList widget.Clickable
	showGatherings widget.Clickable

	connectInfo *utils.ConnectInfo
}

var AddressInput = &addressInput{
	editor: widget.Editor{
		SingleLine: true,
	},
	connectInfo: &utils.ConnectInfo{},
}

func (a *addressInput) GetConnectInfo() *utils.ConnectInfo {
	t := a.editor.Text()
	if len(t) == 0 {
		return nil
	}
	if !slices.Contains([]string{"realm", "gathering"}, strings.Split(t, ":")[0]) {
		_, _, err := net.SplitHostPort(t)
		if err != nil {
			t = t + ":19132"
		}
		a.connectInfo = &utils.ConnectInfo{
			ServerAddress: t,
		}
	}

	return a.connectInfo
}

func (a *addressInput) setRealm(realm *realms.Realm) {
	a.connectInfo = &utils.ConnectInfo{
		Realm: realm,
	}
	a.editor.SetText(fmt.Sprintf("realm:%s", realm.Name))
}

func (a *addressInput) setGathering(gathering *gatherings.Gathering) {
	a.connectInfo = &utils.ConnectInfo{
		Gathering: gathering,
	}
	a.editor.SetText(fmt.Sprintf("gathering:%s", gathering.Title))
}

func (a *addressInput) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if a.showRealmsList.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: "addressInput",
			Target: "ui",
			Data: messages.ShowPopup{
				Popup: popups.NewRealmsList(a.setRealm),
			},
		})
	}

	if a.showGatherings.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: "addressInput",
			Target: "ui",
			Data: messages.ShowPopup{
				Popup: popups.NewGatherings(a.setGathering),
			},
		})
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
				return layout.Flex{
					Axis: layout.Horizontal,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = 100
						return layout.Inset{
							Top:    5,
							Bottom: 5,
							Left:   0,
							Right:  5,
						}.Layout(gtx, material.Button(th, &a.showRealmsList, "Realms").Layout)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = 100
						return layout.Inset{
							Top:    5,
							Bottom: 5,
							Left:   0,
							Right:  5,
						}.Layout(gtx, material.Button(th, &a.showGatherings, "Gatherings").Layout)
					}),
				)
			}),
		)
	})
}
