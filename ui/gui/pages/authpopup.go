package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type guiAuth struct {
	router *Router
	uri    string
	code   string
	err    error
}

func (g *guiAuth) ID() string {
	return "ms-auth"
}

func (g *guiAuth) AuthCode(uri, code string) {
	g.router.PushPopup(g)
	g.uri = uri
	g.code = code
	g.router.Invalidate()
}

func (g *guiAuth) Success() {
	g.router.RemovePopup(g.ID())
}

func (g *guiAuth) PollError(err error) error {
	g.err = err
	return err
}

func (g *guiAuth) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layoutPopupBackground(gtx, th, "guiAuth", func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.Body1(g.router.Theme, "Authenticate at: "+g.uri).Layout),
				layout.Rigid(material.Body1(g.router.Theme, "Using Code: "+g.code).Layout),
			)
		})
	})
}

func (p *guiAuth) Handler(data interface{}) messages.MessageResponse {
	return messages.MessageResponse{}
}
