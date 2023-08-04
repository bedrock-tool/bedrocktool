package settings

import (
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/icons"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/settings"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	*pages.Router

	cmdMenu struct {
		show     bool
		open     widget.Clickable
		state    *component.MenuState
		items    map[string]*widget.Clickable
		selected string
	}

	startButton widget.Clickable

	actions []component.AppBarAction
}

func New(router *pages.Router) *Page {
	p := &Page{
		Router:      router,
		startButton: widget.Clickable{},
	}

	cmdNames := []string{}
	for k := range commands.Registered {
		cmdNames = append(cmdNames, k)
	}
	sort.Strings(cmdNames)

	p.cmdMenu.items = make(map[string]*widget.Clickable, len(commands.Registered))
	options := make([]func(layout.Context) layout.Dimensions, 0, len(commands.Registered))
	for _, name := range cmdNames {
		if _, ok := settings.Settings[name]; !ok {
			continue
		}

		item := &widget.Clickable{}
		p.cmdMenu.items[name] = item
		options = append(options, component.MenuItem(router.Theme, item, name).Layout)
	}

	p.cmdMenu.state = &component.MenuState{
		OptionList: layout.List{},
		Options:    options,
	}

	for _, su := range settings.Settings {
		su.Init()
	}

	return p
}

var _ pages.Page = &Page{}

func (p *Page) Actions() []component.AppBarAction {
	return p.actions
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

			p.Router.SwitchTo(p.cmdMenu.selected)
			p.Router.Execute(cmd)
		}
	}

	if p.cmdMenu.open.Clicked() {
		p.cmdMenu.show = !p.cmdMenu.show
	}

	for k, c := range p.cmdMenu.items {
		if c.Clicked() {
			p.cmdMenu.selected = k
			p.cmdMenu.show = false
		}
	}

	return layout.UniformInset(7).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		d := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Select Command Button
			layout.Rigid(func(gtx C) D {
				str := p.cmdMenu.selected
				if str == "" {
					str = "Select Command"
				}
				btn := material.Button(th, &p.cmdMenu.open, str)
				return btn.Layout(gtx)
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

		if p.cmdMenu.show {
			component.Menu(th, p.cmdMenu.state).Layout(gtx)
		}

		return d
	})
}

func (p *Page) Handler(m any) messages.MessageResponse {
	switch m.(type) {
	case messages.UpdateAvailable:
		p.actions = []component.AppBarAction{
			component.SimpleIconAction(p.Router.UpdateButton, &icons.ActionUpdate, component.OverflowAction{}),
		}

		p.Router.AppBar.SetActions(p.actions, nil)
		p.Router.Invalidate()
	}

	return messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}
}
