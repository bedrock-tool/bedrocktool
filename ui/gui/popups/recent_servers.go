package popups

import (
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
)

// RecentServers lists the addresses connected to before, to pick one again
// or take it off the list.
type RecentServers struct {
	g      guim.Guim
	pick   func(address string)
	remove func(address string)
	close  widget.Clickable
	list   widget.List
	rows   []*recentRow
}

type recentRow struct {
	address string
	pick    widget.Clickable
	remove  widget.Clickable
}

func NewRecentServers(g guim.Guim, addresses []string, pick, remove func(address string)) Popup {
	r := &RecentServers{
		g:      g,
		pick:   pick,
		remove: remove,
		list:   widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	for _, a := range addresses {
		r.rows = append(r.rows, &recentRow{address: a})
	}
	return r
}

func (*RecentServers) ID() string {
	return "RecentServers"
}

func (*RecentServers) Close() error {
	return nil
}

func (*RecentServers) HandleEvent(event any) error {
	return nil
}

var _ Popup = &RecentServers{}

// recentKind is what sort of address it is, said plainly.
func recentKind(address string) (kind, name string) {
	prefix, rest, ok := strings.Cut(address, ":")
	if ok {
		switch strings.ToLower(prefix) {
		case "realm":
			return "Realm", rest
		case "experience":
			return "Experience", rest
		case "xbox":
			return "Friend's world", rest
		}
	}
	return "Server", address
}

func layoutRecentRow(gtx C, th *material.Theme, row *recentRow) D {
	kind, name := recentKind(row.address)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			return material.ButtonLayoutStyle{
				Background:   component.WithAlpha(th.Fg, 14),
				Button:       &row.pick,
				CornerRadius: Radius,
			}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							l := material.Body1(th, name)
							l.MaxLines = 1
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							l := material.Caption(th, kind)
							l.Color = component.WithAlpha(th.Fg, 150)
							return l.Layout(gtx)
						}),
					)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Rigid(func(gtx C) D {
			b := material.Button(th, &row.remove, "Remove")
			b.CornerRadius = Radius
			b.Background = component.WithAlpha(th.Fg, 22)
			b.Color = th.Fg
			b.TextSize = th.TextSize * 0.9
			return b.Layout(gtx)
		}),
	)
}

func (r *RecentServers) Layout(gtx C, th *material.Theme) D {
	for i, row := range r.rows {
		if row.pick.Clicked(gtx) {
			r.pick(row.address)
			r.g.ClosePopup(r.ID())
			return D{}
		}
		if row.remove.Clicked(gtx) {
			r.remove(row.address)
			r.rows = slices.Delete(r.rows, i, i+1)
			break
		}
	}

	if r.close.Clicked(gtx) {
		r.g.ClosePopup(r.ID())
	}

	return LayoutPopupBackground(gtx, th, "RecentServers", func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, "Recent Servers").Layout)
			}),
			layout.Flexed(1, func(gtx C) D {
				if len(r.rows) == 0 {
					return layout.Center.Layout(gtx, func(gtx C) D {
						l := material.Body1(th, "Servers you start with show up here")
						l.Color = component.WithAlpha(th.Fg, 170)
						return l.Layout(gtx)
					})
				}
				return material.List(th, &r.list).Layout(gtx, len(r.rows), func(gtx C, index int) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Inset{Bottom: 6, Right: 8}.Layout(gtx, func(gtx C) D {
						return layoutRecentRow(gtx, th, r.rows[index])
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
