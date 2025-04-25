package settings

import (
	"os"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/sirupsen/logrus"
)

type fileInputWidget struct {
	g    guim.Guim
	Hint string
	Ext  string

	button    widget.Clickable
	textField component.TextField
}

func (f *fileInputWidget) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if f.button.Clicked(gtx) {
		go func() {
			var exts []string
			if f.Ext != "" {
				exts = append(exts, "."+f.Ext)
			}
			fp, err := f.g.Explorer().ChooseFile(exts...)
			if err != nil {
				logrus.Error(err)
				return
			}
			file := fp.(*os.File)
			f.textField.SetText(file.Name())
			file.Close()
		}()
	}

	f.textField.Update(gtx, th, f.Hint)

	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return f.textField.Layout(gtx, th, f.Hint)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			button := material.Button(th, &f.button, "select file")
			return layout.Inset{
				Top:  8,
				Left: 8,
			}.Layout(gtx, button.Layout)
		}),
	)
}
