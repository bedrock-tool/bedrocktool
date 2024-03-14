package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type guiAuth struct {
	ui         ui.UI
	uri        string
	click      widget.Clickable
	code       string
	codeSelect widget.Selectable
	err        error
	close      widget.Clickable
}

func NewGuiAuth(ui ui.UI, uri, code string) Popup {
	return &guiAuth{
		ui:   ui,
		uri:  uri,
		code: code,
	}
}

func (guiAuth) ID() string {
	return "ms-auth"
}

func (g *guiAuth) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if g.click.Clicked(gtx) {
		utils.OpenUrl(g.uri)
	}

	if g.close.Clicked(gtx) {
		utils.Auth.Cancel()
		g.ui.HandleMessage(&messages.Message{
			Source:     g.ID(),
			SourceType: "popup",
			Data:       messages.Close{},
		})
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
	switch m := msg.Data.(type) {
	case messages.Error:
		p.err = m
	}
	return nil
}
