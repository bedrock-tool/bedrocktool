package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type guiAuth struct {
	invalidate func()
	cancel     func()
	onError    func(error)
	uri        string
	click      widget.Clickable
	code       string
	codeSelect widget.Selectable
	close      widget.Clickable
}

func (g *guiAuth) AuthCode(uri string, code string) {
	g.uri = uri
	g.code = code
	g.invalidate()
}

func (g *guiAuth) Finished(err error) {
	if err != nil {
		g.onError(err)
	}
	messages.Router.Handle(&messages.Message{
		Source: "ui",
		Target: "ui",
		Data:   messages.Close{Type: "popup", ID: g.ID()},
	})
	g.cancel()
}

func NewGuiAuth(invalidate func(), cancel func(), onError func(error)) *guiAuth {
	return &guiAuth{invalidate: invalidate, cancel: cancel, onError: onError}
}

func (guiAuth) ID() string {
	return "ms-auth"
}

func (g *guiAuth) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if g.click.Clicked(gtx) {
		utils.OpenUrl(g.uri)
	}

	if g.close.Clicked(gtx) {
		messages.Router.Handle(&messages.Message{
			Source: "ui",
			Target: "ui",
			Data:   messages.Close{Type: "popup", ID: g.ID()},
		})
		g.cancel()
	}

	return LayoutPopupBackground(gtx, th, "guiAuth", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					if g.code == "" {
						return material.Body1(th, "Loading").Layout(gtx)
					}
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(th, "Authenticate at: ").Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return material.Clickable(gtx, &g.click,
										material.Body1(th, g.uri).Layout,
									)
								}),
							)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(th, "Using Code: ").Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									t := material.Body1(th, g.code)
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

func (p *guiAuth) HandleMessage(msg *messages.Message) *messages.Message {
	return nil
}
