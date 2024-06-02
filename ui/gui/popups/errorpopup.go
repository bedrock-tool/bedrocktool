package popups

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type errorPopup struct {
	err                error
	close              widget.Clickable
	submitPanic        widget.Clickable
	haveSubmittedPanic bool
	onClose            func()
	isPanic            bool
}

func NewErrorPopup(err error, onClose func(), isPanic bool) *errorPopup {
	return &errorPopup{
		err:     err,
		onClose: onClose,
		isPanic: isPanic,
	}
}

func (errorPopup) ID() string {
	return "error"
}

func (e *errorPopup) Layout(gtx C, th *material.Theme) D {
	if e.close.Clicked(gtx) {
		e.onClose()
		messages.Router.Handle(&messages.Message{
			Source: e.ID(),
			Target: "ui",
			Data:   messages.Close{Type: "popup", ID: e.ID()},
		})
		return D{}
	}

	if e.submitPanic.Clicked(gtx) {
		e.haveSubmittedPanic = true
		go utils.UploadPanic()
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
						layout.Rigid(func(gtx C) D {
							if e.isPanic && !e.haveSubmittedPanic {
								return material.Button(th, &e.submitPanic, "Upload panic info").Layout(gtx)
							}
							return D{}
						}),
					)
				}),
			)
		})
	})
}

func (e *errorPopup) HandleMessage(msg *messages.Message) *messages.Message {
	return nil
}
