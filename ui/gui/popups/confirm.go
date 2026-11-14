package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
)

// ConfirmPopup asks before doing something the user may regret: the safe
// choice is the highlighted button, the other one does it anyway.
type ConfirmPopup struct {
	g        guim.Guim
	id       string
	title    string
	body     []string
	keep     string
	proceed  string
	onAnswer func(proceed bool)

	keepBtn    widget.Clickable
	proceedBtn widget.Clickable
	answered   bool
}

var _ Popup = &ConfirmPopup{}

// NewConfirm makes a confirm popup. onAnswer gets true when the user picks
// proceed, false when they keep things as they were or close it.
func NewConfirm(g guim.Guim, id, title string, body []string, keep, proceed string, onAnswer func(proceed bool)) *ConfirmPopup {
	return &ConfirmPopup{g: g, id: id, title: title, body: body, keep: keep, proceed: proceed, onAnswer: onAnswer}
}

func (p *ConfirmPopup) ID() string { return p.id }

// Close counts as keeping things as they were, if nothing was picked.
func (p *ConfirmPopup) Close() error {
	p.answer(false)
	return nil
}

func (*ConfirmPopup) HandleEvent(event any) error { return nil }

// Answer answers the question as if a button was pressed.
func (p *ConfirmPopup) Answer(proceed bool) { p.answer(proceed) }

func (p *ConfirmPopup) answer(proceed bool) {
	if p.answered {
		return
	}
	p.answered = true
	if p.onAnswer != nil {
		p.onAnswer(proceed)
	}
}

func (p *ConfirmPopup) Layout(gtx C, th *material.Theme) D {
	if p.keepBtn.Clicked(gtx) {
		p.answer(false)
		p.g.ClosePopup(p.id)
	}
	if p.proceedBtn.Clicked(gtx) {
		p.answer(true)
		p.g.ClosePopup(p.id)
	}

	return LayoutPopupBackground(gtx, th, p.id, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 12, Bottom: 8, Left: 20, Right: 20}.Layout(gtx, func(gtx C) D {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx C) D {
					l := centered(material.Label(th, th.TextSize*1.4, p.title))
					return layout.Inset{Bottom: 12}.Layout(gtx, l.Layout)
				}),
			}
			for i, text := range p.body {
				l := centered(material.Body1(th, text))
				if i > 0 {
					l = centered(material.Body2(th, text))
					l.Color = mutedColor(th.Fg)
				}
				children = append(children, layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 8}.Layout(gtx, l.Layout)
				}))
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 14}.Layout),
				// the safe choice first and highlighted, the other one beside it
				layout.Rigid(func(gtx C) D {
					gtx.Constraints.Min.X = 0 // centered
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							b := material.Button(th, &p.keepBtn, p.keep)
							b.CornerRadius = Radius
							gtx.Constraints.Min.X = gtx.Dp(150)
							return b.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: 10}.Layout),
						layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &p.proceedBtn, p.proceed) }),
					)
				}),
			)
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}
