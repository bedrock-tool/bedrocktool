package pages

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type guiAuth struct {
	router     *Router
	uri        string
	click      widget.Clickable
	code       string
	codeSelect widget.Selectable
	err        error
	close      widget.Clickable
}

func (guiAuth) ID() string {
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
	if g.click.Clicked(gtx) {
		openUrl(g.uri)
	}

	if g.close.Clicked(gtx) {
		utils.Auth.Cancel()
	}

	return LayoutPopupBackground(gtx, th, "guiAuth", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(g.router.Theme, "Authenticate at: ").Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Clickable(gtx, &g.click,
										material.Body1(g.router.Theme, g.uri).Layout,
									)
								}),
							)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(g.router.Theme, "Using Code: ").Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									t := material.Body1(g.router.Theme, g.code)
									t.State = &g.codeSelect
									return t.Layout(gtx)
								}),
							)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &g.close, "Close")
				b.CornerRadius = 8
				return b.Layout(gtx)
			}),
		)
	})
}

func (p *guiAuth) Handler(data any) messages.Response {
	return messages.Response{Ok: false}
}
