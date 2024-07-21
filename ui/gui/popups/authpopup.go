package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type guiAuth struct {
	cancel     func()
	uri        string
	click      widget.Clickable
	code       string
	codeSelect widget.Selectable
	close      widget.Clickable
}

func NewGuiAuth(cancel func(), uri, code string) *guiAuth {
	return &guiAuth{cancel: cancel, uri: uri, code: code}
}

func (guiAuth) ID() string {
	return "ms-auth"
}

func (g *guiAuth) Layout(gtx C, th *material.Theme) D {
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
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(th, "Authenticate at: ").Layout),
								layout.Rigid(func(gtx C) D {
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
								layout.Rigid(func(gtx C) D {
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
