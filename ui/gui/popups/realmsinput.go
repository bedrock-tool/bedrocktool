package popups

import (
	"context"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
)

type RealmsList struct {
	ui       ui.UI
	Show     widget.Bool
	close    widget.Clickable
	l        sync.Mutex
	list     widget.List
	realms   []realms.Realm
	buttons  map[int]*widget.Clickable
	loaded   bool
	loading  bool
	setRealm func(realms.Realm)
}

func NewRealmsList(ui ui.UI, setRealm func(realms.Realm)) Popup {
	return &RealmsList{
		ui:       ui,
		setRealm: setRealm,
	}
}

func (*RealmsList) HandleMessage(msg *messages.Message) *messages.Message {
	return nil
}

func (*RealmsList) ID() string {
	return "Realms"
}

var _ Popup = &RealmsList{}

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
					r.setRealm(realm)
					r.close.Click()
					break
				}
			}
		}
	}

	if r.close.Clicked(gtx) {
		r.ui.HandleMessage(&messages.Message{
			Source:     r.ID(),
			SourceType: "popup",
			Data:       messages.Close{},
		})
	}

	return LayoutPopupBackground(gtx, th, "Realms", func(gtx C) D {
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
