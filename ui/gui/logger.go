package gui

import (
	"bytes"
	"image/color"
	"io"
	"sync"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/sirupsen/logrus"
)

type logger struct {
	g     guim.Guim
	lines []*logrus.Entry
	l     sync.Mutex
	list  widget.List

	clickCopyLogs widget.Clickable
	clickClose    widget.Clickable
}

type C = layout.Context
type D = layout.Dimensions

func (l *logger) Layout(gtx C, th *material.Theme) D {
	if l.clickCopyLogs.Clicked(gtx) {
		var logTxt []byte
		for _, line := range l.lines {
			lineBytes, _ := line.Bytes()
			logTxt = append(logTxt, utils.StripAnsiBytes(lineBytes)...)
		}
		gtx.Execute(clipboard.WriteCmd{
			Type: "text",
			Data: io.NopCloser(bytes.NewReader(logTxt)),
		})
		l.g.Toast(gtx, "Copied!")
	}

	if l.clickClose.Clicked(gtx) {
		l.g.CloseLogs()
	}

	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.UniformInset(20).Layout(gtx, func(gtx C) D {
		component.Rect{
			Color: color.NRGBA{A: 240},
			Size:  gtx.Constraints.Max,
			Radii: 15,
		}.Layout(gtx)
		return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
			return layout.Stack{
				Alignment: layout.N,
			}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					return material.List(th, &l.list).Layout(gtx, len(l.lines), func(gtx C, index int) D {
						line := l.lines[index]
						t := material.Body1(th, line.Message)
						t.Color = color.NRGBA{0xff, 0xff, 0xff, 0xff}
						return t.Layout(gtx)
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis:      layout.Vertical,
						Spacing:   layout.SpaceBetween,
						Alignment: layout.End,
					}.Layout(gtx,
						layout.Flexed(1, layout.Spacer{Height: 10000}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{
								Axis: layout.Horizontal,
							}.Layout(gtx,
								layout.Rigid(material.Button(th, &l.clickCopyLogs, "Copy Logs").Layout),
								layout.Rigid(layout.Spacer{Width: 8}.Layout),
								layout.Rigid(material.Button(th, &l.clickClose, "Close").Layout),
							)
						}),
					)
				}),
			)
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
	l.g.Invalidate()
	return nil
}
