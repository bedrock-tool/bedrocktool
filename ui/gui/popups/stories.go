package popups

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

// StoriesPopup asks for the answer to a Realm's Stories prompt, which the
// Realm wants before it lets the account join.
type StoriesPopup struct {
	g      guim.Guim
	event  *messages.EventStoriesPrompt
	optIn  widget.Clickable
	optOut widget.Clickable
	cancel widget.Clickable
	sent   bool
}

var _ Popup = &StoriesPopup{}

func NewStoriesPopup(g guim.Guim, event *messages.EventStoriesPrompt) *StoriesPopup {
	return &StoriesPopup{g: g, event: event}
}

func (*StoriesPopup) ID() string { return "realm-stories" }

// Close answers Cancel if it wasn't answered, so the join never waits on a
// popup that's gone.
func (p *StoriesPopup) Close() error {
	p.answer(messages.StoriesCancel)
	return nil
}

func (*StoriesPopup) HandleEvent(event any) error { return nil }

func (p *StoriesPopup) answer(a messages.StoriesAnswer) {
	if p.sent {
		return
	}
	p.sent = true
	p.event.Reply <- a
}

func (p *StoriesPopup) Layout(gtx C, th *material.Theme) D {
	for _, b := range []struct {
		c *widget.Clickable
		a messages.StoriesAnswer
	}{{&p.optIn, messages.StoriesOptIn}, {&p.optOut, messages.StoriesOptOut}, {&p.cancel, messages.StoriesCancel}} {
		if b.c.Clicked(gtx) {
			p.answer(b.a)
			p.g.ClosePopup(p.ID())
		}
	}

	return LayoutPopupBackground(gtx, th, p.ID(), func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, "Realm Stories").Layout)
			}),
			layout.Rigid(material.Body1(th, fmt.Sprintf(
				"%s won't let this account join until it answers the Realm's Stories prompt, the one Minecraft asks the first time you join a Realm.",
				p.event.RealmName)).Layout),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(material.Body2(th,
				"Opting in shares your activity on this Realm's Stories timeline; opting out doesn't. Either answer lets you join, and you can change it later in Minecraft.").Layout),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &p.cancel, "Cancel") }),
					layout.Rigid(func(gtx C) D {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return CloseButtonText(gtx, th, &p.optIn, "Opt in") }),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(func(gtx C) D {
								b := material.Button(th, &p.optOut, "Opt out")
								b.CornerRadius = Radius
								gtx.Constraints.Min.X = gtx.Dp(110)
								return b.Layout(gtx)
							}),
						)
					}),
				)
			}),
		)
	})
}
