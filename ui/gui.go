//go:build gui || android

package ui

import (
	"context"
	"sync"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/bedrock-tool/bedrocktool/subcommands"
	"github.com/bedrock-tool/bedrocktool/subcommands/skins"
	"github.com/bedrock-tool/bedrocktool/subcommands/world"
	"github.com/bedrock-tool/bedrocktool/ui/gui"
	"github.com/bedrock-tool/bedrocktool/utils"
	"golang.org/x/exp/slices"
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
	utils.BaseUI

	commandUI gui.CommandUI
}

func (g *GUI) Init() bool {
	return true
}

func (g *GUI) Start(ctx context.Context, cancel context.CancelFunc) error {
	a := app.New()
	w := a.NewWindow("Bedrocktool")

	debug := binding.BindBool(&utils.Options.Debug)
	extra_debug := binding.BindBool(&utils.Options.ExtraDebug)

	entries := []string{}
	forms := make(map[string]*widget.Form)
	for k, c := range utils.ValidCMDs {
		entries = append(entries, k)

		f := settings[k]
		if f != nil {
			forms[k] = f(c)
		}
	}
	slices.Sort(entries)

	selected := binding.NewString()
	forms_box := container.NewVBox()
	start_button := widget.NewButton("Start", nil)
	l := sync.Mutex{}

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
			l.Lock()
			selected.Set(s)
			forms_box.RemoveAll()
			f := forms[s]
			if f != nil {
				forms_box.Add(forms[s])
			}
			l.Unlock()
		}),
		forms_box,
		start_button,
	))

	wg := sync.WaitGroup{}

	start_button.OnTapped = func() {
		sub, _ := selected.Get()
		cmd := utils.ValidCMDs[sub]
		if cmd == nil {
			return
		}

		u := gui.CommandUIs[sub]
		if u != nil {
			g.commandUI = u
			u.Layout(w)
		} else {
			w.SetContent(container.NewCenter(
				widget.NewRichTextFromMarkdown("# No UI yet, Look at the console."),
			))
		}

		utils.InitDNS()
		utils.InitExtraDebug(ctx)

		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd.Execute(ctx, g)
		}()
	}

	w.SetCloseIntercept(func() {
		cancel()
		wg.Wait()
		w.Close()
	})

	w.ShowAndRun()
	return nil
}

func (g *GUI) Message(name string, data interface{}) utils.MessageResponse {
	h := g.commandUI.Handler()
	if h != nil {
		r := h(name, data)
		if r.Ok {
			return r
		}
	}

	r := utils.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch name {
	case "can_show_images":
		r.Ok = true
	}

	return r
}

func init() {
	utils.MakeGui = func() utils.UI {
		return &GUI{}
	}
}
