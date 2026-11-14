package popups

import (
	"context"
	"image"
	"sort"

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
		list:     widget.List{List: layout.List{Axis: layout.Vertical}},
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
	realmsClient, err := account.Realms(context.Background())
	if err != nil {
		return err
	}
	realmsList, err := realmsClient.Realms(context.Background())
	if err != nil {
		return err
	}
	var list []*realmButton
	for _, realm := range realmsList {
		list = append(list, &realmButton{Realm: &realm})
	}
	// joinable ones first
	sort.SliceStable(list, func(i, j int) bool {
		return realmJoinable(list[i].Realm) && !realmJoinable(list[j].Realm)
	})
	r.realms = list
	r.loading = false
	r.loaded = true
	r.g.Invalidate()
	return nil
}

func realmJoinable(realm *realms.Realm) bool {
	return !realm.Expired && realm.State != "CLOSED"
}

// realmStatus is the line under a Realm's name.
func realmStatus(realm *realms.Realm) string {
	switch {
	case realm.Expired:
		return "Expired: the owner's subscription ran out, it can't be joined"
	case realm.State == "CLOSED":
		return "Closed by its owner"
	}
	return "Open"
}

func layoutRealm(gtx C, th *material.Theme, realm *realmButton) D {
	joinable := realmJoinable(realm.Realm)
	bg := component.WithAlpha(th.Fg, 14)
	if !joinable {
		bg = component.WithAlpha(th.Fg, 6)
	}
	content := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := material.Body1(th, realm.Name)
					if !joinable {
						l.Color = component.WithAlpha(th.Fg, 140)
					}
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					l := material.Caption(th, realmStatus(realm.Realm))
					l.Color = component.WithAlpha(th.Fg, 150)
					return l.Layout(gtx)
				}),
			)
		})
	}
	if !joinable {
		// not a button: a click would only fail
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				return component.Rect{Color: bg, Size: gtx.Constraints.Min, Radii: gtx.Dp(Radius)}.Layout(gtx)
			},
			content,
		)
	}
	return material.ButtonLayoutStyle{
		Background:   bg,
		Button:       &realm.Clickable,
		CornerRadius: Radius,
	}.Layout(gtx, content)
}

func (r *RealmsList) Layout(gtx C, th *material.Theme) D {
	for _, realm := range r.realms {
		if realm.Clicked(gtx) && realmJoinable(realm.Realm) {
			r.setRealm(realm.Realm)
			r.g.ClosePopup(r.ID())
			return D{}
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
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, "Realms").Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				if r.loading {
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(gtx.Dp(24), gtx.Dp(24))
						return material.Loader(th).Layout(gtx)
					})
				}
				if len(r.realms) == 0 {
					return layout.Center.Layout(gtx, material.Body1(th, "You aren't in any Realms").Layout)
				}
				return material.List(th, &r.list).Layout(gtx, len(r.realms), func(gtx C, index int) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 6, Right: 8}.Layout(gtx, func(gtx C) D {
						return layoutRealm(gtx, th, r.realms[index])
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
					return CloseButton(gtx, th, &r.close)
				})
			}),
		)
	})
}
