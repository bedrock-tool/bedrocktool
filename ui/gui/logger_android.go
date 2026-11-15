package gui

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/android"
)

// loggerPlatform is the state of Android's extra button in the logs:
// "Report a problem", which opens the GitHub report popup.
type loggerPlatform struct {
	report widget.Clickable
}

func (l *logger) platformButtons(gtx C, th *material.Theme) D {
	if l.platform.report.Clicked(gtx) {
		// the logs are drawn over popups
		l.g.CloseLogs()
		l.g.ShowPopup(android.NewReportPopup(l.g, false))
	}
	return layout.Inset{Right: 8}.Layout(gtx, material.Button(th, &l.platform.report, android.T("android_report_problem", nil)).Layout)
}
