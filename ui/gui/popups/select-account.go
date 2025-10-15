package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/sirupsen/logrus"
)

type SelectAccount struct {
	g      guim.Guim
	editor widget.Editor
	close  widget.Clickable
	done   widget.Clickable
}

func NewSelectAccount(g guim.Guim) Popup {
	return &SelectAccount{
		g: g,
	}
}

func (*SelectAccount) ID() string {
	return "select-account"
}

func (*SelectAccount) Close() error {
	return nil
}

func (p *SelectAccount) Layout(gtx C, th *material.Theme) D {
	if p.done.Clicked(gtx) && p.editor.Len() != 0 {
		name := p.editor.Text()
		logrus.WithField("name", name).Info("switched account")
		p.g.ClosePopup(p.ID())
		p.g.SetAccountName(name)
		err := auth.Auth.LoadAccount(name)
		if err != nil {
			logrus.WithField("name", name).WithError(err).Error("LoadAccount")
		}
	}

	if p.close.Clicked(gtx) {
		p.g.ClosePopup(p.ID())
	}

	return LayoutPopupBackground(gtx, th, "switch-account", func(gtx C) D {
		gtx.Constraints.Max = gtx.Constraints.Min
		return layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceBetween,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return material.H4(th, "Select Account Instance").Layout(gtx)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return material.Editor(th, &p.editor, "enter instance name").Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X /= 2
				return layout.Flex{
					Axis:      layout.Horizontal,
					Spacing:   layout.SpaceSides,
					Alignment: layout.End,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = 80
							b := material.Button(th, &p.close, "Close")
							b.CornerRadius = 8
							return b.Layout(gtx)
						})
					}),
					layout.Rigid(func(gtx C) D {
						return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = 80
							b := material.Button(th, &p.done, "Done")
							b.CornerRadius = 8
							if p.editor.Len() == 0 {
								b.Color = th.Bg
								b.Background = th.Fg
							}
							return b.Layout(gtx)
						})
					}),
				)
			}),
		)
	})
}

func (p *SelectAccount) HandleEvent(event any) error {
	return nil
}
