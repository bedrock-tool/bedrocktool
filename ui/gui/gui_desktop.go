//go:build !android

package gui

import "gioui.org/app"

// platformStart runs once the pages exist, before the window opens.
// Nothing to do on the PC; Android's is in gui_android.go.
func (g *GUI) platformStart() {}

// platformViewEvent sees the window's native view as it changes.
func (g *GUI) platformViewEvent(event app.ViewEvent) {}
