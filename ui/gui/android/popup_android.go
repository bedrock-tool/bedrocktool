package android

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

// ReportPopup helps the user send a problem to GitHub: it copies a report
// (see Report) and opens a new issue to paste it into. Shown on its own
// after a crash, and from "Report a problem" in the logs.
type ReportPopup struct {
	g       guim.Guim
	crashed bool

	copyReport widget.Clickable
	openIssue  widget.Clickable
	close      widget.Clickable
}

var _ popups.Popup = (*ReportPopup)(nil)

func NewReportPopup(g guim.Guim, crashed bool) *ReportPopup {
	return &ReportPopup{g: g, crashed: crashed}
}

func (*ReportPopup) ID() string                  { return "report" }
func (*ReportPopup) Close() error                { return nil }
func (*ReportPopup) HandleEvent(event any) error { return nil }

func (p *ReportPopup) copy(gtx C) {
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(Report())),
	})
	p.g.Toast(gtx, T("android_report_copied", nil))
}

// button is a full-width button: easy to hit on a phone.
func button(gtx C, th *material.Theme, c *widget.Clickable, text string, primary bool) D {
	b := material.Button(th, c, text)
	b.CornerRadius = popups.Radius
	if !primary {
		b.Background = component.WithAlpha(th.Fg, 30)
		b.Color = th.Fg
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Top: 8}.Layout(gtx, b.Layout)
}

func (p *ReportPopup) Layout(gtx C, th *material.Theme) D {
	if p.copyReport.Clicked(gtx) {
		p.copy(gtx)
	}
	if p.openIssue.Clicked(gtx) {
		p.copy(gtx)
		p.g.OpenUrl(IssueURL(p.crashed))
	}
	if p.close.Clicked(gtx) {
		if p.crashed {
			DismissCrash()
		}
		p.g.ClosePopup(p.ID())
		return D{}
	}

	title, text := T("android_report_title", nil), T("android_report_text", nil)
	if p.crashed {
		title, text = T("android_crash_title", nil), T("android_crash_text", nil)
	}
	return popups.LayoutPopupBackground(gtx, th, p.ID(), func(gtx C) D {
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 10}.Layout(gtx, material.Label(th, th.TextSize*1.4, title).Layout)
				}),
				layout.Rigid(material.Body1(th, text).Layout),
				layout.Rigid(func(gtx C) D {
					l := material.Body2(th, T("android_report_privacy", nil))
					l.Color = component.WithAlpha(th.Fg, 170)
					return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, l.Layout)
				}),
				layout.Rigid(func(gtx C) D { return button(gtx, th, &p.openIssue, T("android_open_issue", nil), true) }),
				layout.Rigid(func(gtx C) D { return button(gtx, th, &p.copyReport, T("android_copy_report", nil), false) }),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 16}.Layout(gtx, func(gtx C) D { return popups.CloseButton(gtx, th, &p.close) })
				}),
			)
		})
	})
}
