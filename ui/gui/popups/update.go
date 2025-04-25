package popups

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
)

type UpdatePopup struct {
	g           guim.Guim
	state       messages.UIState
	startButton widget.Clickable
	err         error
	updating    bool
}

var _ Popup = &UpdatePopup{}

func NewUpdatePopup(g guim.Guim) Popup {
	return &UpdatePopup{
		g:     g,
		state: messages.UIStateMain,
	}
}

func (p *UpdatePopup) ID() string {
	return "update"
}

func (p *UpdatePopup) Close() error {
	return nil
}

func (p *UpdatePopup) Layout(gtx C, th *material.Theme) D {
	if p.startButton.Clicked(gtx) && !p.updating {
		p.updating = true
		go func() {
			p.err = updater.DoUpdate()
			if p.err == nil {
				p.state = messages.UIStateFinished
			}
			p.updating = false
			p.g.ClosePopup(p.ID())
		}()
	}

	update, err := updater.UpdateAvailable()
	if err != nil {
		p.err = err
	}

	return LayoutPopupBackground(gtx, th, p.ID(), func(gtx C) D {
		return layout.Inset{
			Top:    unit.Dp(25),
			Bottom: unit.Dp(25),
			Right:  unit.Dp(35),
			Left:   unit.Dp(35),
		}.Layout(gtx, func(gtx C) D {
			if p.err != nil {
				return layout.Center.Layout(gtx, material.H1(th, p.err.Error()).Layout)
			}
			if p.updating {
				return layout.Center.Layout(gtx, material.H3(th, "Updating...").Layout)
			}

			var children []layout.FlexChild
			switch p.state {
			case messages.UIStateMain:
				children = append(children,
					layout.Rigid(material.Label(th, 20, fmt.Sprintf("Current: %s\nNew:     %s", updater.Version, update.Version)).Layout),
					layout.Rigid(material.Button(th, &p.startButton, "Do Update").Layout),
				)
			case messages.UIStateFinished:
				children = append(children,
					layout.Rigid(material.H3(th, "Update Finished").Layout),
					layout.Rigid(func(gtx C) D {
						return layout.Center.Layout(gtx, material.Label(th, th.TextSize, "restart the app").Layout)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (p *UpdatePopup) HandleEvent(event any) error {
	return nil
}
