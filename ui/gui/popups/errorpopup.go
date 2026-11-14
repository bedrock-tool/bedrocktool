package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/locale"
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

	title := locale.Loc("gui_error_title", nil)
	if e.isPanic {
		title = locale.Loc("gui_error_fatal_title", nil)
	}

	return LayoutPopupBackground(gtx, th, "error", func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 4, Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, title).Layout)
			}),
			layout.Rigid(material.Body1(th, e.err.Error()).Layout),
			layout.Rigid(func(gtx C) D {
				if !e.isPanic {
					return D{}
				}
				return layout.Inset{Top: 8}.Layout(gtx, material.Body2(th, locale.Loc("gui_error_more_info", nil)).Layout)
			}),
			layout.Rigid(layout.Spacer{Height: 24}.Layout),
			layout.Rigid(func(gtx C) D { return CloseButton(gtx, th, &e.close) }),
		)
	})
}

func (e *errorPopup) HandleEvent(event any) error {
	return nil
}
