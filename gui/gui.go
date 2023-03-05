package gui

import (
	"context"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type GUI struct {
	UI

	selected binding.String
}

func NewGUI() *GUI {
	return &GUI{}
}

func (g *GUI) SetOptions() {
	a := app.New()
	w := a.NewWindow("Bedrocktool")

	debug := binding.NewBool()
	debug.AddListener(binding.NewDataListener(func() {
		utils.Options.Debug, _ = debug.Get()
	}))

	extra_debug := binding.NewBool()
	extra_debug.AddListener(binding.NewDataListener(func() {
		utils.Options.ExtraDebug, _ = extra_debug.Get()
		if utils.Options.ExtraDebug {
			debug.Set(true)
		}
	}))

	entries := []string{}
	forms := make(map[string]*widget.Form)
	for k, c := range utils.ValidCMDs {
		entries = append(entries, k)
		s := c.SettingsUI()
		if s != nil {
			forms[k] = s
		}
	}

	g.selected = binding.NewString()

	forms_box := container.NewVBox()

	w.SetContent(container.NewVBox(
		widget.NewRichTextFromMarkdown("## Settings"),
		container.NewHBox(
			widget.NewCheckWithData("Debug", debug),
			widget.NewCheckWithData("extra-debug", extra_debug),
		),
		widget.NewRichTextFromMarkdown("# Commands"),
		widget.NewSelect(entries, func(s string) {
			g.selected.Set(s)
		}),
		forms_box,
		widget.NewButton("Start", func() {
			w.Close()
		}),
	))

	for _, f := range forms {
		forms_box.Add(f)
	}
	g.selected.AddListener(binding.NewDataListener(func() {
		v, _ := g.selected.Get()
		for n, f := range forms {
			if n == v {
				f.Show()
			} else {
				f.Hide()
			}
		}
	}))

	w.ShowAndRun()
}

func (g *GUI) Init() {
}

func (g *GUI) Execute(ctx context.Context) error {
	sub, err := g.selected.Get()
	if err != nil {
		return err
	}
	cmd := utils.ValidCMDs[sub]
	go cmd.MainWindow()
	cmd.Execute(ctx, nil)
	return nil
}
