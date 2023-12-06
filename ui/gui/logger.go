package gui

import (
	"image/color"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/sirupsen/logrus"
)

type logger struct {
	router *pages.Router
	lines  []*logrus.Entry
	l      sync.Mutex
	list   widget.List
}

type C = layout.Context
type D = layout.Dimensions

func (l *logger) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.UniformInset(20).Layout(gtx, func(gtx C) D {
		component.Rect{
			Color: color.NRGBA{A: 240},
			Size:  gtx.Constraints.Max,
			Radii: 15,
		}.Layout(gtx)
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			return material.List(th, &l.list).Layout(gtx, len(l.lines), func(gtx layout.Context, index int) layout.Dimensions {
				line := l.lines[index]
				t := material.Body1(th, line.Message)
				t.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
				return t.Layout(gtx)
			})
		})
	})

}

func (l *logger) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (l *logger) Fire(e *logrus.Entry) error {
	l.l.Lock()
	l.lines = append(l.lines, e)
	l.l.Unlock()
	l.router.Invalidate()
	return nil
}
