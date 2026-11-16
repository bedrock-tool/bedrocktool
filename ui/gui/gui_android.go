package gui

import (
	"gioui.org/app"
	"github.com/bedrock-tool/bedrocktool/ui/gui/android"
)

// platformStart runs once the pages exist, before the window opens: it
// keeps the app running in the background while a session runs, and if the
// last run crashed, offers to report it.
func (g *GUI) platformStart() {
	g.router.OnSession = android.SessionRunning
	if android.LastCrash() != "" {
		g.router.PushPopup(android.NewReportPopup(g, true))
	}
}

// platformViewEvent keeps the window's View, which dialogs (Export) need.
func (g *GUI) platformViewEvent(event app.ViewEvent) {
	if e, ok := event.(app.AndroidViewEvent); ok {
		android.SetView(e.View)
	}
}
