package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
)

type errorPopup struct {
	g       guim.Guim
	onClose func()
	err     error
	close   widget.Clickable
	isPanic bool
}

func NewErrorPopup(g guim.Guim, err error, isPanic bool, onClose func()) *errorPopup {
	return &errorPopup{
		g:       g,
		onClose: onClose,
		err:     err,
		isPanic: isPanic,
	}
}

func (errorPopup) ID() string {
	return "error"
}

func (errorPopup) Close() error {
	return nil
}

func (e *errorPopup) Layout(gtx C, th *material.Theme) D {
	if e.close.Clicked(gtx) {
		e.g.ClosePopup(e.ID())
		if e.onClose != nil {
			e.onClose()
		}
		return D{}
	}

	title := "Error"
	if e.isPanic {
		title = "Fatal Panic"
	}

	return LayoutPopupBackground(gtx, th, "error", func(gtx C) D {
		return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
			return layout.Flex{
				Axis:      layout.Vertical,
				Alignment: layout.Start,
				Spacing:   layout.SpaceBetween,
			}.Layout(gtx,
				layout.Rigid(material.H3(th, title).Layout),
				layout.Rigid(material.Body1(th, e.err.Error()).Layout),
				layout.Rigid(func(gtx C) D {
					if e.isPanic {
						return material.Body2(th, "More info has been printed to the console, you can submit the error to make debugging easier").Layout(gtx)
					}
					return D{}
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Flex{
						Axis:    layout.Horizontal,
						Spacing: layout.SpaceSides,
					}.Layout(gtx,
						layout.Rigid(material.Button(th, &e.close, "Close").Layout),
					)
				}),
			)
		})
	})
}

func (e *errorPopup) HandleEvent(event any) error {
	return nil
}
