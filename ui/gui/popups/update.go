package popups

import (
	"image"
	"os"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
)

const (
	stateInit = iota
	stateLoading
	stateAsk
	stateDownloading
	stateError
	stateFinished
)

type UpdatePopup struct {
	g           guim.Guim
	state       int
	startButton widget.Clickable
	close       widget.Clickable
	updating    bool

	update *updater.Update
	err    error
}

var _ Popup = &UpdatePopup{}

func NewUpdatePopup(g guim.Guim) Popup {
	return &UpdatePopup{
		g:     g,
		state: stateInit,
	}
}

func (p *UpdatePopup) ID() string {
	return "update"
}

func (p *UpdatePopup) Close() error {
	return nil
}

func (p *UpdatePopup) Layout(gtx C, th *material.Theme) D {
	if p.state == stateInit {
		p.state = stateLoading
		go func() {
			update, err := updater.GetLatest()
			if err != nil {
				p.err = err
				p.state = stateError
				return
			}
			p.update = update
			p.state = stateAsk
		}()
	}

	if p.startButton.Clicked(gtx) && !p.updating && p.update != nil && p.update.IsNew() {
		p.updating = true
		p.state = stateDownloading
		go func() {
			p.err = updater.DoUpdate(p.update)
			if p.err == nil {
				p.state = stateFinished
				p.err = updater.Restart()
				if p.err == nil {
					os.Exit(0)
					return
				}
				p.state = stateError
			} else {
				p.state = stateError
			}
			p.updating = false
		}()
	}

	if p.close.Clicked(gtx) && p.state != stateDownloading && p.state != stateFinished {
		p.g.ClosePopup(p.ID())
	}

	return LayoutPopupBackground(gtx, th, p.ID(), func(gtx C) D {
		var body layout.Widget
		switch p.state {
		case stateInit, stateLoading:
			body = func(gtx C) D {
				return layout.Center.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max = image.Pt(gtx.Dp(24), gtx.Dp(24))
					return material.Loader(th).Layout(gtx)
				})
			}
		case stateAsk:
			body = func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body1(th, "Installed: "+utils.Version).Layout),
					layout.Rigid(material.Body1(th, "Latest:    "+p.update.Version).Layout),
					layout.Rigid(layout.Spacer{Height: 14}.Layout),
					layout.Rigid(func(gtx C) D {
						if !p.update.IsNew() {
							return material.Body2(th, "You already have the latest version.").Layout(gtx)
						}
						b := material.Button(th, &p.startButton, "Update and restart")
						b.CornerRadius = Radius
						return b.Layout(gtx)
					}),
				)
			}
		case stateDownloading:
			body = material.Body1(th, "Downloading the update...").Layout
		case stateError:
			msg := "Update failed"
			if p.err != nil {
				msg = "Update failed: " + p.err.Error()
			}
			body = material.Body1(th, msg).Layout
		case stateFinished:
			body = material.Body1(th, "Restarting...").Layout
		}

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, "Update").Layout)
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: 8, Right: 8, Top: 4, Bottom: 24}.Layout(gtx, body)
			}),
			layout.Rigid(func(gtx C) D {
				if p.state == stateDownloading || p.state == stateFinished {
					return D{}
				}
				return CloseButton(gtx, th, &p.close)
			}),
		)
	})
}

func (p *UpdatePopup) HandleEvent(event any) error {
	return nil
}
