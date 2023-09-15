package update

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

const ID = "update"

type Page struct {
	*pages.Router

	State       messages.UIState
	startButton widget.Clickable
	err         error
	updating    bool
}

func New(router *pages.Router) pages.Page {
	return &Page{
		Router: router,
		State:  messages.UIStateMain,
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
		Name: "Update",
		//Icon: icon.OtherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.startButton.Clicked() && !p.updating {
		p.updating = true
		go func() {
			p.err = updater.DoUpdate()
			if p.err == nil {
				p.State = messages.UIStateFinished
			}
			p.updating = false
			p.Router.Invalidate()
		}()
	}

	update, err := updater.UpdateAvailable()
	if err != nil {
		p.err = err
	}

	return layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}.Layout(gtx, func(gtx C) D {
		if p.err != nil {
			return layout.Center.Layout(gtx, material.H1(th, p.err.Error()).Layout)
		}
		if p.updating {
			return layout.Center.Layout(gtx, material.H3(th, "Updating...").Layout)
		}
		switch p.State {
		case messages.UIStateMain:
			// show the main ui
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Label(th, 20, fmt.Sprintf("Current: %s\nNew:     %s", updater.Version, update.Version)).Layout),
				layout.Rigid(material.Button(th, &p.startButton, "Do Update").Layout),
			)
		case messages.UIStateFinished:
			return layout.Center.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H3(th, "Update Finished").Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Center.Layout(gtx, material.Label(th, th.TextSize, "restart the app").Layout)
					}),
				)
			})
		}

		return D{}
	})
}

func (p *Page) Handler(data interface{}) messages.Response {
	r := messages.Response{
		Ok:   false,
		Data: nil,
	}
	return r
}
