package pages

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type HandlerFunc = func(data interface{}) messages.Message

type Page interface {
	ID() string
	Actions(th *material.Theme) []component.AppBarAction
	Overflow() []component.OverflowAction
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	NavItem() component.NavItem
	messages.Handler
}

var Pages = map[string]func(*Router) Page{}

func Register(name string, fun func(*Router) Page) {
	Pages[name] = fun
}

func AppBarSwitch(toggle *widget.Bool, label string, th *material.Theme) component.AppBarAction {
	return component.AppBarAction{
		Layout: func(gtx layout.Context, bg, fg color.NRGBA) layout.Dimensions {
			return layout.UniformInset(5).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Axis:      layout.Horizontal,
					Alignment: layout.Middle,
				}.Layout(gtx,
					layout.Rigid(material.Switch(th, toggle, label).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Label(th, 12, label)
						l.Alignment = text.Middle
						return layout.UniformInset(5).Layout(gtx, l.Layout)
					}),
				)
			})
		},
	}
}
