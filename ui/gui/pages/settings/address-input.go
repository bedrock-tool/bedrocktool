package settings

import (
	"context"
	"fmt"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
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

type RealmsList struct {
	router   *pages.Router
	Show     widget.Bool
	close    widget.Clickable
	l        sync.Mutex
	list     widget.List
	realms   []realms.Realm
	buttons  map[int]*widget.Clickable
	loaded   bool
	loading  bool
	SetRealm func(realms.Realm)
}

func (*RealmsList) Handler(data any) messages.Response {
	return messages.Response{Ok: false}
}

func (*RealmsList) ID() string {
	return "Realms"
}

var _ pages.Popup = &RealmsList{}

func (r *RealmsList) Load() {
	var err error
	r.realms, err = utils.GetRealmsAPI().Realms(context.Background())
	for _, realm := range r.realms {
		r.buttons[realm.ID] = &widget.Clickable{}
	}
	r.loading = false
	r.loaded = true
	if err != nil {
		logrus.Error(err)
	}
}

func (r *RealmsList) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	for k, c := range r.buttons {
		if c.Clicked(gtx) {
			for _, realm := range r.realms {
				if realm.ID == k {
					r.SetRealm(realm)
					r.close.Click()
					break
				}
			}
		}
	}

	if r.close.Clicked(gtx) {
		r.router.RemovePopup(r.ID())
	}

	return pages.LayoutPopupBackground(gtx, th, "Realms", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				if r.loading {
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(20, 20)
						return material.Loader(th).Layout(gtx)
					})
				}

				if !r.loaded && !r.loading {
					r.loading = true
					go r.Load()
				}

				r.l.Lock()
				defer r.l.Unlock()
				if len(r.realms) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "you have no realms").Layout)
				}

				return material.List(th, &r.list).Layout(gtx, len(r.realms), func(gtx layout.Context, index int) layout.Dimensions {
					realm := r.realms[index]
					return material.ButtonLayoutStyle{
						Background:   component.WithAlpha(th.ContrastBg, 0x80),
						Button:       r.buttons[realm.ID],
						CornerRadius: 8,
					}.Layout(gtx, func(gtx C) D {
						return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(material.Label(th, th.TextSize, realm.Name).Layout),
							)
						})
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &r.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})

}
