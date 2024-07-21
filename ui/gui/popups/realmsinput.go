package popups

import (
	"context"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type RealmsList struct {
	Show     widget.Bool
	close    widget.Clickable
	l        sync.Mutex
	list     widget.List
	realms   []*realmButton
	loaded   bool
	loading  bool
	setRealm func(*realms.Realm)
}

type realmButton struct {
	*realms.Realm
	widget.Clickable
}

func NewRealmsList(setRealm func(*realms.Realm)) Popup {
	return &RealmsList{
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

func (r *RealmsList) Load() error {
	realms, err := utils.Auth.Realms.Realms(context.Background())
	if err != nil {
		return err
	}
	r.realms = nil
	for _, realm := range realms {
		r.realms = append(r.realms, &realmButton{
			Realm: &realm,
		})
	}

	r.loading = false
	r.loaded = true
	return nil
}

func (r *RealmsList) Layout(gtx C, th *material.Theme) D {
	for _, realm := range r.realms {
		if realm.Clicked(gtx) {
			r.setRealm(realm.Realm)
			r.close.Click()
		}
	}

	if r.close.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: r.ID(),
			Target: "ui",
			Data:   messages.Close{Type: "popup", ID: r.ID()},
		})
	}

	if !r.loaded && !r.loading {
		r.loading = true
		go func() {
			if !utils.Auth.LoggedIn() {
				messages.Router.Handle(&messages.Message{
					Source: r.ID(),
					Target: "ui",
					Data:   messages.RequestLogin{Wait: true},
				})
			}
			err := r.Load()
			if err != nil {
				messages.Router.Handle(&messages.Message{
					Source: r.ID(),
					Target: "ui",
					Data:   messages.Error(err),
				})
				messages.Router.Handle(&messages.Message{
					Source: r.ID(),
					Target: "ui",
					Data: messages.Close{
						Type: "popup",
						ID:   r.ID(),
					},
				})
			}
		}()
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

				r.l.Lock()
				defer r.l.Unlock()
				if len(r.realms) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "you have no realms").Layout)
				}

				return material.List(th, &r.list).Layout(gtx, len(r.realms), func(gtx C, index int) D {
					realm := r.realms[index]
					return material.ButtonLayoutStyle{
						Background:   component.WithAlpha(th.ContrastBg, 0x80),
						Button:       &realm.Clickable,
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
