//go:build gui || android || true

package gui

import (
	"context"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/utils"
)

var settings = map[string]func(utils.Command) *widget.Form{
	"worlds": func(cc utils.Command) *widget.Form {
		c := cc.(*world.WorldCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("packs", binding.BindBool(&c.Packs)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("void", binding.BindBool(&c.EnableVoid)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("saveImage", binding.BindBool(&c.SaveImage)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("experimentInventory", binding.BindBool(&c.ExperimentInventory)),
			),
		)
	},
	"skins": func(cc utils.Command) *widget.Form {
		c := cc.(*skins.SkinCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			), widget.NewFormItem(
				"filter", widget.NewEntryWithData(binding.BindString(&c.Filter)),
			),
		)
	},
	"capture": func(cc utils.Command) *widget.Form {
		c := cc.(*subcommands.CaptureCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			),
		)
	},
	"chat-log": func(cc utils.Command) *widget.Form {
		c := cc.(*subcommands.ChatLogCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			),
			widget.NewFormItem(
				"", widget.NewCheckWithData("Verbose", binding.BindBool(&c.Verbose)),
			),
		)
	},
	"debug-proxy": func(cc utils.Command) *widget.Form {
		c := cc.(*subcommands.DebugProxyCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			), widget.NewFormItem(
				"filter", widget.NewEntryWithData(binding.BindString(&c.Filter)),
			),
		)
	},
	"packs": func(cc utils.Command) *widget.Form {
		c := cc.(*subcommands.ResourcePackCMD)
		return widget.NewForm(
			widget.NewFormItem(
				"serverAddress", widget.NewEntryWithData(binding.BindString(&c.ServerAddress)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("saveEncrypted", binding.BindBool(&c.SaveEncrypted)),
			), widget.NewFormItem(
				"", widget.NewCheckWithData("only-keys", binding.BindBool(&c.OnlyKeys)),
			),
		)
	},
}

type GUI struct {
	utils.UI

	selected binding.String
}

func (g *GUI) SetOptions(ctx context.Context) bool {
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

		f := settings[k]
		if f != nil {
			s := f(c)
			forms[k] = s
		}
	}

	g.selected = binding.NewString()

	forms_box := container.NewVBox()

	var quit = true

	w.SetContent(container.NewVBox(
		widget.NewRichTextFromMarkdown("## Settings"),
		container.NewHBox(
			widget.NewCheckWithData("Debug", debug),
			widget.NewCheckWithData("extra-debug", extra_debug),
		),
		container.NewHBox(
			widget.NewRichTextFromMarkdown("Custom Userdata:"),
			widget.NewEntryWithData(binding.BindString(&utils.Options.PathCustomUserData)),
		),
		widget.NewRichTextFromMarkdown("# Commands"),
		widget.NewSelect(entries, func(s string) {
			g.selected.Set(s)
			quit = false
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
	return quit
}

func (g *GUI) Init() {
}

func (g *GUI) Execute(ctx context.Context) error {
	sub, err := g.selected.Get()
	if err != nil {
		return err
	}
	cmd := utils.ValidCMDs[sub]
	cmd.Execute(ctx, nil)
	return nil
}

func init() {
	utils.MakeGui = func() utils.UI {
		return &GUI{}
	}
}
