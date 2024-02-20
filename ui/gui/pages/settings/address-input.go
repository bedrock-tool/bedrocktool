package settings

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type addressInput struct {
	Editor         widget.Editor
	showRealmsList widget.Clickable
	RealmsList     RealmsList
}

var AddressInput = &addressInput{
	Editor: widget.Editor{
		SingleLine: true,
	},
	RealmsList: RealmsList{
		buttons: make(map[int]*widget.Clickable),
		list: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	},
}

func init() {
	AddressInput.Init()
}

func (a *addressInput) Init() {
	a.RealmsList.SetRealm = func(realm realms.Realm) {
		a.Editor.SetText(fmt.Sprintf("realm:%s:%d", realm.Name, realm.ID))
	}
}

func (a *addressInput) Value() string {
	return a.Editor.Text()
}

func (a *addressInput) Layout(gtx layout.Context, th *material.Theme, r *pages.Router) layout.Dimensions {
	a.RealmsList.router = r
	if a.showRealmsList.Clicked(gtx) {
		if !r.PushPopup(&a.RealmsList) {
			a.RealmsList.close.Click()
		}
	}

	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				macro := op.Record(gtx.Ops)
				d := layout.UniformInset(8).Layout(gtx, func(gtx C) D {
					e := material.Editor(th, &a.Editor, "Enter Server Address")
					e.LineHeight += 4
					return e.Layout(gtx)
				})
				c := macro.Stop()
				component.Rect{
					Color: component.WithAlpha(th.ContrastFg, 80),
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
