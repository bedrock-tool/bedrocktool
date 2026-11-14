package popups

import (
	"bytes"
	"fmt"
	"github.com/bedrock-tool/bedrocktool/locale"
	"image/color"
	"io"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
)

type guiAuth struct {
	g          guim.Guim
	uri        string
	uriDest    string
	clickUri   widget.Clickable
	clickCode  widget.Clickable
	code       string
	codeSelect widget.Selectable
	close      widget.Clickable
}

// mutedColor is fg at about two thirds strength, for secondary text.
func mutedColor(fg color.NRGBA) color.NRGBA {
	fg.A = uint8(uint16(fg.A) * 2 / 3)
	return fg
}

func NewGuiAuth(g guim.Guim, uri, code string) *guiAuth {
	uriDest := fmt.Sprintf("https://login.live.com/oauth20_remoteconnect.srf?otc=%s", code)
	return &guiAuth{g: g, uri: uri, uriDest: uriDest, code: code}
}

func (p *guiAuth) HandleEvent(event any) error {
	return nil
}

func (guiAuth) ID() string {
	return "ms-auth"
}

func (guiAuth) Close() error {
	return nil
}

func (g *guiAuth) Layout(gtx C, th *material.Theme) D {
	if g.clickUri.Clicked(gtx) {
		g.g.OpenUrl(g.uriDest)
	}
	if g.clickCode.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{
			Type: "text",
			Data: io.NopCloser(bytes.NewReader([]byte(g.code))),
		})
		g.g.Toast(gtx, locale.Loc("gui_copied", nil))
	}

	if g.close.Clicked(gtx) {
		auth.Auth.CancelLogin()
		g.g.ClosePopup(g.ID())
	}

	return LayoutPopupBackground(gtx, th, "guiAuth", func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 16, Bottom: 4}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(material.Body1(th, locale.Loc("gui_auth_at", nil)).Layout),
						layout.Rigid(func(gtx C) D {
							uri := material.Body1(th, g.uri)
							uri.Color = color.NRGBA{R: 0x06, G: 0x4c, B: 0xa6, A: 0xff}
							return material.Clickable(gtx, &g.clickUri, uri.Layout)
						}),
						layout.Rigid(func(gtx C) D {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Body1(th, locale.Loc("gui_auth_code", nil)).Layout),
								layout.Rigid(func(gtx C) D {
									t := material.Body1(th, g.code)
									t.State = &g.codeSelect
									return material.Clickable(gtx, &g.clickCode, t.Layout)
								}),
							)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx C) D {
				// #348: a calm note about strict anti-cheat servers
				return layout.Inset{Top: 12, Bottom: 12}.Layout(gtx, func(gtx C) D {
					l := material.Body2(th, auth.AntiCheatNotice())
					l.Color = mutedColor(th.Fg)
					return l.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 4
				b := material.Button(th, &g.close, locale.Loc("gui_close", nil))
				b.CornerRadius = Radius
				return b.Layout(gtx)
			}),
		)
	})
}
