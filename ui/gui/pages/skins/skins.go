package skins

import (
	"sync"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "skins"

type Page struct {
	router *pages.Router
	Skins  []messages.NewSkin

	l         sync.Mutex
	State     messages.UIState
	SkinsList widget.List
	back      widget.Clickable
}

func New(router *pages.Router) pages.Page {
	return &Page{
		router: router,
		SkinsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
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
		Name: "Skin Grabber",
		//Icon: icon.OtherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.back.Clicked(gtx) {
		p.router.SwitchTo("settings")
		return D{}
	}

	return layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceBetween,
		}.Layout(gtx,
			layout.Flexed(0.9, func(gtx C) D {
				// show the main ui
				return layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						p.l.Lock()
						defer p.l.Unlock()
						return material.List(th, &p.SkinsList).Layout(gtx, len(p.Skins), func(gtx C, index int) D {
							entry := p.Skins[len(p.Skins)-index-1]
							return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Rigid(material.Label(th, th.TextSize, entry.PlayerName).Layout),
								)
							})
						})
					}),
				)
			}),
			layout.Flexed(0.1, func(gtx C) D {
				if p.State == messages.UIStateFinished {
					return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
						gtx.Constraints.Max.Y = gtx.Dp(40)
						gtx.Constraints.Max.X = gtx.Constraints.Max.X / 6
						return material.Button(th, &p.back, "Return").Layout(gtx)
					})
				}
				return D{}
			}),
		)
	})
}

func (p *Page) Handler(data interface{}) messages.Response {
	r := messages.Response{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.SetUIState:
		p.State = m
		p.router.Invalidate()
		r.Ok = true
	case messages.NewSkin:
		p.l.Lock()
		p.Skins = append(p.Skins, m)
		p.l.Unlock()
		p.router.Invalidate()
		r.Ok = true
	}
	return r
}
