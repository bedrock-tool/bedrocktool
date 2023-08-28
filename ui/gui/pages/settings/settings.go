package settings

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/settings"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type cmdItem struct {
	Clickable *widget.Clickable
	Text      string
}

type Page struct {
	router *pages.Router

	cmdMenu struct {
		state      component.GridState
		clickables map[string]cmdItem
		names      []string
		selected   string
	}

	startButton widget.Clickable
	debugButton widget.Bool
}

func New(router *pages.Router) *Page {
	p := &Page{
		router: router,
	}

	for k := range commands.Registered {
		if _, ok := settings.Settings[k]; !ok {
			continue
		}
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

	settings.Theme = router.Theme
	for _, su := range settings.Settings {
		su.Init()
	}

	return p
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return "settings"
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{
		/*
			{
				Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
					return material.Switch(p.router.Theme, &p.debugButton, "debug").Layout(gtx)
				},
			},
		*/
	}
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
	if p.startButton.Clicked() {
		if p.cmdMenu.selected != "" {
			cmd, ok := commands.Registered[p.cmdMenu.selected]
			if !ok {
				logrus.Errorf("Cmd %s not found", p.cmdMenu.selected)
			}

			if s, ok := settings.Settings[p.cmdMenu.selected]; ok {
				s.Apply()
			}

			p.router.SwitchTo(p.cmdMenu.selected)
			p.router.Execute(cmd)
		}
	}

	if p.debugButton.Changed() {
		utils.Options.Debug = p.debugButton.Value
	}

	for k, c := range p.cmdMenu.clickables {
		if c.Clickable.Clicked() {
			p.cmdMenu.selected = k
		}
	}

	return layout.UniformInset(7).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Select Command Button
			layout.Rigid(func(gtx C) D {
				return layout.Inset{
					Top:    10,
					Bottom: 10,
					Left:   unit.Dp(gtx.Constraints.Max.X / 10),
					Right:  unit.Dp(gtx.Constraints.Max.X / 10),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
								b.Background = th.ContrastFg
								b.Color = color.NRGBA{R: 0, G: 0, B: 0, A: 0xff}
							}
							return layout.Inset{Left: 5, Right: 5}.Layout(gtx, b.Layout)
						},
					)
				})
			}),

			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if p.cmdMenu.selected == "" {
					return layout.Dimensions{}
				}
				s, ok := settings.Settings[p.cmdMenu.selected]
				if !ok {
					return layout.Center.Layout(gtx, material.H4(th, "No Settings Yet (Use CLI)").Layout)
				} else {
					return layout.UniformInset(15).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return s.Layout(gtx, th)
					})
				}
			}),

			// Start Button
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(15),
						Bottom: unit.Dp(15),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = layout.Constraints{
							Min: image.Pt(300, 50),
							Max: image.Pt(400, 50),
						}
						btn := material.Button(th, &p.startButton, "Start")
						return btn.Layout(gtx)
					})
				})
			}),
		)

		return d
	})
}

func (p *Page) Handler(m any) messages.MessageResponse {
	return messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}
}
