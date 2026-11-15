//go:build !android

package gui

import "gioui.org/widget/material"

// loggerPlatform is the state of the platform's extra buttons in the logs.
// The PC has none; Android's are in logger_android.go.
type loggerPlatform struct{}

func (l *logger) platformButtons(gtx C, th *material.Theme) D {
	return D{}
}
