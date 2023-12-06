package settings

import (
	"image"
	"slices"
	"sort"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "settings"

type cmdItem struct {
	Clickable *widget.Clickable
	Text      string
}

type Page struct {
	router *pages.Router

	cmdMenu struct {
		clickables map[string]cmdItem
		names      []string
		state      component.GridState
		selected   string
	}

	settings map[string]*settingsPage

	startButton widget.Clickable
}

func New(router *pages.Router) pages.Page {
	p := &Page{
		router:   router,
		settings: make(map[string]*settingsPage),
	}

	for k, cmd := range commands.Registered {
		if !slices.Contains([]string{"worlds", "skins", "packs"}, k) {
			continue
		}

		settingUI := &settingsPage{router: router, cmd: cmd}
		settingUI.Init()
		p.settings[k] = settingUI
		p.cmdMenu.names = append(p.cmdMenu.names, k)
	}
	sort.Strings(p.cmdMenu.names)

	p.cmdMenu.clickables = make(map[string]cmdItem, len(commands.Registered))
	for _, name := range p.cmdMenu.names {
		p.cmdMenu.clickables[name] = cmdItem{
			Clickable: &widget.Clickable{},
			Text:      name,
		}
	}

	return p
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return ID
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Settings",
		//Icon: icon.OtherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	var validSettings = true
	if p.cmdMenu.selected != "" {
		s := p.settings[p.cmdMenu.selected]
		validSettings = s.Valid()
	}

	if p.startButton.Clicked(gtx) && validSettings {
		if p.cmdMenu.selected != "" {
			cmd, ok := commands.Registered[p.cmdMenu.selected]
			if !ok {
				logrus.Errorf("Cmd %s not found", p.cmdMenu.selected)
			}

			settingsUI := p.settings[p.cmdMenu.selected]
			settingsUI.Apply()

			p.router.SwitchTo(p.cmdMenu.selected)
			p.router.Execute(cmd)
		}
	}

	for k, c := range p.cmdMenu.clickables {
		if c.Clickable.Clicked(gtx) {
			p.cmdMenu.selected = k
		}
	}

	return layout.UniformInset(7).Layout(gtx, func(gtx C) D {
		d := layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceBetween,
		}.Layout(gtx,
			// Select Command Button
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Top:   10,
					Left:  unit.Dp(gtx.Constraints.Max.X / gtx.Dp(10)),
					Right: unit.Dp(gtx.Constraints.Max.X / gtx.Dp(10)),
				}.Layout(gtx, func(gtx C) D {
					return component.Grid(th, &p.cmdMenu.state).Layout(gtx, 1, len(p.cmdMenu.clickables),
						func(axis layout.Axis, index, constraint int) int {
							if axis == layout.Horizontal {
								return constraint / 3
							} else {
								return gtx.Dp(40)
							}
						}, func(gtx layout.Context, row, col int) layout.Dimensions {
							name := p.cmdMenu.names[col]
							c := p.cmdMenu.clickables[name]
							b := material.Button(th, c.Clickable, c.Text)
							if p.cmdMenu.selected == name {
								b.Background = component.WithAlpha(th.Fg, 70)
								b.Color = th.Fg
							}
							return layout.Inset{Left: 5, Right: 5}.Layout(gtx, b.Layout)
						},
					)
				})
			}),

			layout.Flexed(1, func(gtx layout.Context) (d layout.Dimensions) {
				if p.cmdMenu.selected == "" {
					d = layout.Center.Layout(gtx, material.H5(th, "Select a Mode").Layout)
					d.Size.Y = gtx.Constraints.Max.Y
				} else {
					s := p.settings[p.cmdMenu.selected]
					return layout.Inset{
						Left:  unit.Dp(gtx.Constraints.Max.X / gtx.Dp(10)),
						Right: unit.Dp(gtx.Constraints.Max.X / gtx.Dp(10)),
					}.Layout(gtx, func(gtx C) D {
						return s.Layout(gtx, th)
					})
				}
				return d
			}),

			// Start Button
			layout.Rigid(func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Inset{
						Top:    15,
						Bottom: 15,
					}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min = image.Pt(gtx.Dp(300), gtx.Dp(40))
						gtx.Constraints.Max = image.Pt(gtx.Constraints.Max.X/3, gtx.Dp(40))
						b := material.Button(th, &p.startButton, "Start")
						if !validSettings {
							b.Color = th.ContrastFg
							b.Background = th.ContrastBg
						}
						return b.Layout(gtx)
					})
				})
			}),
		)

		return d
	})
}

func (p *Page) Handler(m any) messages.Response {
	return messages.Response{
		Ok:   false,
		Data: nil,
	}
}
