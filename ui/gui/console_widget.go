package gui

import (
	"io"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/fyne-io/terminal"
	"github.com/sirupsen/logrus"
)

type consoleWidget struct {
	widget.BaseWidget

	term *terminal.Terminal
}

func (c *consoleWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.term)
}

func newConsoleWidget() *consoleWidget {
	rp, w := io.Pipe()
	r := io.TeeReader(rp, os.Stdout)
	logrus.SetOutput(w)

	term := terminal.New()
	go term.RunWithConnection(nil, r)

	c := &consoleWidget{
		term: term,
	}
	c.ExtendBaseWidget(c)
	return c
}
