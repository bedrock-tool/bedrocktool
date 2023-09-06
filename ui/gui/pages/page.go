package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type HandlerFunc = func(data interface{}) messages.Response

type Page interface {
	ID() string
	Actions() []component.AppBarAction
	Overflow() []component.OverflowAction
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
	NavItem() component.NavItem

	// handle events from program
	Handler(data any) messages.Response
}

var Pages = map[string]func(*Router) Page{}

func Register(name string, fun func(*Router) Page) {
	Pages[name] = fun
}
