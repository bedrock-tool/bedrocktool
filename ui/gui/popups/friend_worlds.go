package popups

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/friendworlds"
	"github.com/google/uuid"
)

// FriendWorlds lists the worlds the account's friends are playing, to pick
// one to join and download.
type FriendWorlds struct {
	g       guim.Guim
	watcher *friendworlds.Watcher
	pick    func(friendworlds.World)

	close   widget.Clickable
	refresh widget.Clickable
	list    widget.List
	rows    map[uuid.UUID]*widget.Clickable
	opened  bool
}

func NewFriendWorlds(g guim.Guim, watcher *friendworlds.Watcher, pick func(friendworlds.World)) Popup {
	return &FriendWorlds{
		g:       g,
		watcher: watcher,
		pick:    pick,
		list:    widget.List{List: layout.List{Axis: layout.Vertical}},
		rows:    make(map[uuid.UUID]*widget.Clickable),
	}
}

func (*FriendWorlds) ID() string {
	return "FriendWorlds"
}

func (*FriendWorlds) Close() error {
	return nil
}

func (*FriendWorlds) HandleEvent(event any) error {
	return nil
}

var _ Popup = &FriendWorlds{}

func (f *FriendWorlds) row(id uuid.UUID) *widget.Clickable {
	c, ok := f.rows[id]
	if !ok {
		c = new(widget.Clickable)
		f.rows[id] = c
	}
	return c
}

func layoutFriendWorld(gtx C, th *material.Theme, world friendworlds.World, click *widget.Clickable) D {
	joinable := world.Status == friendworlds.Joinable
	bg := component.WithAlpha(th.Fg, 14)
	if !joinable {
		bg = component.WithAlpha(th.Fg, 6)
	}
	var details []string
	if world.Name != "" && world.Host != "" {
		details = append(details, world.Host)
	}
	if joinable {
		if p := world.PlayersText(); p != "" {
			details = append(details, p)
		}
	} else {
		details = append(details, world.StatusText())
	}
	content := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							l := material.Body1(th, world.Title())
							l.Font.Weight = font.SemiBold
							l.MaxLines = 1
							if !joinable {
								l.Color = component.WithAlpha(th.Fg, 140)
							}
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							if len(details) == 0 {
								return D{}
							}
							l := material.Caption(th, strings.Join(details, " · "))
							l.Color = component.WithAlpha(th.Fg, 150)
							return l.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					if !joinable {
						return D{}
					}
					// what clicking the row does, said on it
					return layout.Inset{Left: 10}.Layout(gtx, func(gtx C) D {
						return layout.Background{}.Layout(gtx,
							func(gtx C) D {
								return component.Rect{Color: th.ContrastBg, Size: gtx.Constraints.Min, Radii: gtx.Dp(Radius)}.Layout(gtx)
							},
							func(gtx C) D {
								return layout.Inset{Top: 6, Bottom: 6, Left: 14, Right: 14}.Layout(gtx, func(gtx C) D {
									l := material.Body2(th, locale.Loc("gui_friends_select", nil))
									l.Color = th.ContrastFg
									return l.Layout(gtx)
								})
							},
						)
					})
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
		Button:       click,
		CornerRadius: Radius,
	}.Layout(gtx, content)
}

// message is a centered line of plain text, and a fainter one under it.
func message(th *material.Theme, msg, hint string) layout.Widget {
	return func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := material.Body1(th, msg)
					l.Alignment = text.Middle
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if hint == "" {
						return D{}
					}
					return layout.Inset{Top: 6}.Layout(gtx, func(gtx C) D {
						l := material.Body2(th, hint)
						l.Alignment = text.Middle
						l.Color = component.WithAlpha(th.Fg, 150)
						return l.Layout(gtx)
					})
				}),
			)
		})
	}
}

func (f *FriendWorlds) Layout(gtx C, th *material.Theme) D {
	if !f.opened {
		// what the settings page showed may be a minute old
		f.opened = true
		f.watcher.Refresh()
	}
	snap := f.watcher.Snapshot()

	for _, world := range snap.Worlds {
		if world.Status != friendworlds.Joinable {
			continue
		}
		if f.row(world.HandleID).Clicked(gtx) {
			f.pick(world)
			f.g.ClosePopup(f.ID())
			return D{}
		}
	}
	if f.refresh.Clicked(gtx) {
		f.watcher.Refresh()
	}
	if f.close.Clicked(gtx) {
		f.g.ClosePopup(f.ID())
	}

	return LayoutPopupBackground(gtx, th, "FriendWorlds", func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, locale.Loc("gui_friends_title", nil)).Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				switch {
				case !snap.SignedIn:
					return message(th, locale.Loc("gui_friends_signed_out", nil), "")(gtx)
				case len(snap.Worlds) == 0 && (snap.Checking || snap.Checked.IsZero()):
					return layout.Center.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max = image.Pt(gtx.Dp(24), gtx.Dp(24))
						return material.Loader(th).Layout(gtx)
					})
				case len(snap.Worlds) == 0 && snap.Err != nil:
					return message(th, locale.Loc("gui_friends_error", locale.Strmap{"Err": snap.Err}), "")(gtx)
				case len(snap.Worlds) == 0:
					return message(th, locale.Loc("gui_friends_none", nil), locale.Loc("gui_friends_none_hint", nil))(gtx)
				}
				return material.List(th, &f.list).Layout(gtx, len(snap.Worlds), func(gtx C, index int) D {
					world := snap.Worlds[index]
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 6, Right: 8}.Layout(gtx, func(gtx C) D {
						return layoutFriendWorld(gtx, th, world, f.row(world.HandleID))
					})
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return CloseButton(gtx, th, &f.close)
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx C) D {
							if !snap.SignedIn {
								return D{}
							}
							return CloseButtonText(gtx, th, &f.refresh, locale.Loc("gui_refresh", nil))
						}),
					)
				})
			}),
		)
	})
}
