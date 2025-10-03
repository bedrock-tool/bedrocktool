package popups

import (
	"context"
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/sandertv/gophertunnel/minecraft/realms"
)

type RealmsList struct {
	g        guim.Guim
	setRealm func(*realms.Realm)
	Show     widget.Bool
	close    widget.Clickable
	list     widget.List
	realms   []*realmButton
	loaded   bool
	loading  bool
}

type realmButton struct {
	*realms.Realm
	widget.Clickable
}

func NewRealmsList(g guim.Guim, setRealm func(*realms.Realm)) Popup {
	return &RealmsList{
		g:        g,
		setRealm: setRealm,
	}
}

func (*RealmsList) HandleEvent(event any) error {
	return nil
}

func (*RealmsList) ID() string {
	return "Realms"
}

func (*RealmsList) Close() error {
	return nil
}

var _ Popup = &RealmsList{}

func (r *RealmsList) Load() error {
	account := auth.Auth.Account()
	if account == nil {
		return auth.ErrNotLoggedIn
	}
	realmsList, err := account.Realms().Realms(context.Background())
	if err != nil {
		return err
	}
	r.realms = nil
	for _, realm := range realmsList {
		r.realms = append(r.realms, &realmButton{
			Realm: &realm,
		})
	}

	/*
		r.realms = append(r.realms, &realmButton{
			Realm: &realms.Realm{
				ID:   1,
				Name: "test",
			},
		})
	*/

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
		r.g.ClosePopup(r.ID())
	}

	if !r.loaded && !r.loading {
		r.loading = true
		go func() {
			if !auth.Auth.LoggedIn() {
				auth.Auth.RequestLogin(r.g.AccountName())
			}
			err := r.Load()
			if err != nil {
				r.g.Error(err)
				r.g.ClosePopup(r.ID())
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

				if len(r.realms) == 0 {
					return layout.Center.Layout(gtx, material.H5(th, "you have no realms").Layout)
				}

				return material.List(th, &r.list).Layout(gtx, len(r.realms), func(gtx C, index int) D {
					gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, 60)
					realm := r.realms[index]
					return material.ButtonLayoutStyle{
						Background:   component.WithAlpha(th.ContrastBg, 0x80),
						Button:       &realm.Clickable,
						CornerRadius: 8,
					}.Layout(gtx, func(gtx C) D {
						return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
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
