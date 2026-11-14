package settings

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/connectinfo"
	"github.com/bedrock-tool/bedrocktool/utils/friendworlds"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sandertv/gophertunnel/minecraft/service/gatherings"
)

type addressInput struct {
	g               guim.Guim
	editor          widget.Editor
	showRealmsList  widget.Clickable
	showExperiences widget.Clickable
	showFeatured    widget.Clickable
	showFriends     widget.Clickable
	showRecent      widget.Clickable

	// recent lists the servers connected to before; removeRecent takes one
	// off that list.
	recent       func() []string
	removeRecent func(address string)

	connectInfo *connectinfo.ConnectInfo

	// friends finds the worlds friends are playing; the Friends button
	// only shows while one of them can be joined.
	friends *friendworlds.Watcher
}

var AddressInput = &addressInput{
	g: nil,
	editor: widget.Editor{
		SingleLine: true,
	},
	connectInfo: &connectinfo.ConnectInfo{},
	friends:     friendworlds.NewWatcher(),
}

func (a *addressInput) SetGuim(g guim.Guim) {
	a.g = g
	a.friends.OnChange = g.Invalidate
}

// SetRecent gives the input the recent servers to offer.
func (a *addressInput) SetRecent(recent func() []string, remove func(address string)) {
	a.recent = recent
	a.removeRecent = remove
}

// Text is the address as typed.
func (a *addressInput) Text() string {
	return a.editor.Text()
}

// SetText replaces the address, forgetting the Realm or Experience picked
// for the old one.
func (a *addressInput) SetText(address string) {
	a.connectInfo = &connectinfo.ConnectInfo{Value: address}
	a.editor.SetText(address)
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

	if a.showExperiences.Clicked(gtx) {
		a.g.ShowPopup(popups.NewExperiences(a.g, func(experience *gatherings.Experience) {
			a.connectInfo.SetExperience(experience)
			a.editor.SetText(a.connectInfo.Value)
		}))
	}

	if a.showFeatured.Clicked(gtx) {
		a.g.ShowPopup(popups.NewFeaturedServers(a.g, func(server *gatherings.FeaturedServer) {
			a.connectInfo.SetFeaturedServer(server)
			a.editor.SetText(a.connectInfo.Value)
		}))
	}

	if a.showRecent.Clicked(gtx) && a.recent != nil {
		a.g.ShowPopup(popups.NewRecentServers(a.g, a.recent(), a.SetText, func(address string) {
			if a.removeRecent != nil {
				a.removeRecent(address)
			}
		}))
	}

	// checked in the background while the settings page is on screen, and
	// again when it's next due
	if next := a.friends.Tick(gtx.Now); !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	friends := a.friends.Snapshot().Joinable()
	if a.showFriends.Clicked(gtx) {
		a.g.ShowPopup(popups.NewFriendWorlds(a.g, a.friends, func(world friendworlds.World) {
			a.SetText(world.Address())
		}))
	}

	type addressButton struct {
		click *widget.Clickable
		label string
	}
	buttons := []addressButton{
		{&a.showRecent, locale.Loc("gui_address_recent", nil)},
		{&a.showRealmsList, locale.Loc("gui_address_realms", nil)},
		{&a.showExperiences, locale.Loc("gui_address_experiences", nil)},
		{&a.showFeatured, locale.Loc("gui_address_featured", nil)},
	}
	if friends > 0 {
		buttons = append(buttons, addressButton{&a.showFriends, locale.Loc("gui_address_friends", locale.Strmap{"Count": strconv.Itoa(friends)})})
	}

	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				macro := op.Record(gtx.Ops)
				d := layout.UniformInset(8).Layout(gtx, func(gtx C) D {
					e := material.Editor(th, &a.editor, locale.Loc("gui_address_placeholder", nil))
					e.LineHeight += 4
					return e.Layout(gtx)
				})
				c := macro.Stop()
				component.Rect{
					Color: component.WithAlpha(th.Fg, 10),
					Size:  d.Size,
					Radii: 4,
				}.Layout(gtx)
				c.Add(gtx.Ops)
				return d
			}),
			layout.Rigid(func(gtx C) D {
				children := make([]layout.FlexChild, len(buttons))
				for i, b := range buttons {
					children[i] = layout.Flexed(1, func(gtx C) D {
						return layout.Inset{Top: 5, Bottom: 5, Right: 5}.Layout(gtx, material.Button(th, b.click, b.label).Layout)
					})
				}
				return layout.Flex{Axis: layout.Horizontal, WeightSum: float32(len(buttons))}.Layout(gtx, children...)
			}),
		)
	})
}
