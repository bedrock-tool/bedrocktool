package settings

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type addressInput struct {
	editor         widget.Editor
	showRealmsList widget.Clickable
}

var AddressInput = &addressInput{
	editor: widget.Editor{
		SingleLine: true,
	},
}

func (a *addressInput) Value() string {
	return a.editor.Text()
}

func (a *addressInput) setRealm(realm realms.Realm) {
	a.editor.SetText(fmt.Sprintf("realm:%s:%d", realm.Name, realm.ID))
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
				return layout.Inset{
					Top: 6,
				}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max.X = 100
					return material.Button(th, &a.showRealmsList, "list realms").Layout(gtx)
				})
			}),
		)
	})
}
