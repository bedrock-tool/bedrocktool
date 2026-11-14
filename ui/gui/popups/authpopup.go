package popups

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
)

type guiAuth struct {
	g         guim.Guim
	uri       string
	uriDest   string
	clickUri  widget.Clickable
	clickCode widget.Clickable
	open      widget.Clickable
	code      string
	close     widget.Clickable
}

// mutedColor is fg at about two thirds strength, for secondary text.
func mutedColor(fg color.NRGBA) color.NRGBA {
	fg.A = uint8(uint16(fg.A) * 2 / 3)
	return fg
}

var (
	linkColor   = color.NRGBA{R: 0x6a, G: 0xa8, B: 0xff, A: 0xff}
	noticeColor = color.NRGBA{R: 0xf5, G: 0xb0, B: 0x41, A: 0xff}
)

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

// spacedCode splits a login code in two halves so it reads easily:
// "2RUCWPYC" -> "2RUC WPYC".
func spacedCode(code string) string {
	if len(code) < 6 || strings.Contains(code, " ") {
		return code
	}
	h := len(code) / 2
	return code[:h] + " " + code[h:]
}

func centered(l material.LabelStyle) material.LabelStyle {
	l.Alignment = text.Middle
	return l
}

func (g *guiAuth) Layout(gtx C, th *material.Theme) D {
	if g.clickUri.Clicked(gtx) || g.open.Clicked(gtx) {
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
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 12, Bottom: 8, Left: 16, Right: 16}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := centered(material.Label(th, th.TextSize*1.4, locale.Loc("gui_auth_title", nil)))
					return layout.Inset{Bottom: 6}.Layout(gtx, l.Layout)
				}),
				layout.Rigid(func(gtx C) D {
					l := centered(material.Body1(th, locale.Loc("gui_auth_steps", nil)))
					l.Color = mutedColor(th.Fg)
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					l := centered(material.Body1(th, g.uri))
					l.Color = linkColor
					return layout.Inset{Top: 2, Bottom: 14}.Layout(gtx, func(gtx C) D {
						return material.Clickable(gtx, &g.clickUri, l.Layout)
					})
				}),
				// the code, large, in its own box: click to copy
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = 0 // as wide as the code, centered
					return g.clickCode.Layout(gtx, func(gtx C) D {
						return layout.Background{}.Layout(gtx,
							func(gtx C) D {
								bg := th.Fg
								bg.A = 16
								if g.clickCode.Hovered() {
									bg.A = 30
								}
								return component.Rect{Color: bg, Size: gtx.Constraints.Min, Radii: gtx.Dp(Radius)}.Layout(gtx)
							},
							func(gtx C) D {
								return layout.Inset{Top: 10, Bottom: 8, Left: 28, Right: 28}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											l := material.Label(th, th.TextSize*2.2, spacedCode(g.code))
											l.Font.Weight = font.Bold
											return l.Layout(gtx)
										}),
										layout.Rigid(func(gtx C) D {
											l := material.Caption(th, locale.Loc("gui_auth_copy_hint", nil))
											l.Color = mutedColor(th.Fg)
											return l.Layout(gtx)
										}),
									)
								})
							},
						)
					})
				}),
				// #348: a calm note about strict anti-cheat servers
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 18, Bottom: 18}.Layout(gtx, func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Background{}.Layout(gtx,
							func(gtx C) D {
								bg := noticeColor
								bg.A = 22
								size := gtx.Constraints.Min
								d := component.Rect{Color: bg, Size: size, Radii: gtx.Dp(Radius)}.Layout(gtx)
								component.Rect{Color: noticeColor, Size: image.Pt(gtx.Dp(3), size.Y)}.Layout(gtx)
								return d
							},
							func(gtx C) D {
								return layout.Inset{Top: 10, Bottom: 10, Left: 14, Right: 12}.Layout(gtx, func(gtx C) D {
									return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
										layout.Rigid(func(gtx C) D {
											l := material.Body2(th, locale.Loc("gui_auth_notice_title", nil))
											l.Font.Weight = font.SemiBold
											l.Color = noticeColor
											return layout.Inset{Bottom: 3}.Layout(gtx, l.Layout)
										}),
										layout.Rigid(func(gtx C) D {
											l := material.Body2(th, auth.AntiCheatNotice())
											l.Color = mutedColor(th.Fg)
											return l.Layout(gtx)
										}),
									)
								})
							},
						)
					})
				}),
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = 0 // centered
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &g.close, locale.Loc("gui_cancel", nil)) }),
						layout.Rigid(layout.Spacer{Width: 10}.Layout),
						layout.Rigid(func(gtx C) D {
							b := material.Button(th, &g.open, locale.Loc("gui_auth_open", nil))
							b.CornerRadius = Radius
							gtx.Constraints.Min.X = gtx.Dp(140)
							return b.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}
