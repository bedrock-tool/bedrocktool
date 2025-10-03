package gui

import (
	"context"
	"errors"
	"image/color"
	"net/url"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"gioui.org/x/pref/theme"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/packs"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/settings"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/skins"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages/worlds"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/bedrock-tool/bedrocktool/utils/updater"
	"github.com/sirupsen/logrus"

	"github.com/gioui-plugins/gio-plugins/hyperlink"
	"github.com/gioui-plugins/gio-plugins/hyperlink/giohyperlink"
	"github.com/gioui-plugins/gio-plugins/plugin/gioplugins"
)

type GUI struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	accountName string

	router    *pages.Router
	logger    logger
	theme     *material.Theme
	window    app.Window
	explorer  *explorer.Explorer
	hyperlink *hyperlink.Hyperlink
}

var _ guim.Guim = &GUI{}

func (g *GUI) AccountName() string {
	return g.accountName
}

func (g *GUI) SetAccountName(name string) {
	g.accountName = name
}

func (g *GUI) Init() error {
	messages.SetEventHandler(g.eventHandler)
	auth.Auth.SetHandler(&messages.AuthHandler{})

	g.logger.list = widget.List{
		List: layout.List{
			Axis: layout.Vertical,
		},
	}

	g.explorer = explorer.NewExplorer(&g.window)
	g.hyperlink = hyperlink.NewHyperlink(hyperlink.Config{})

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	dark, err := theme.IsDarkMode()
	if err != nil {
		logrus.Warn(err)
	}
	if dark {
		_th := th.WithPalette(paletteDark)
		th = &_th
	} else {
		_th := th.WithPalette(paletteLight)
		th = &_th
	}
	g.theme = th

	return nil
}

var paletteLight = material.Palette{
	Bg:         color.NRGBA{0xff, 0xff, 0xff, 0xff},
	Fg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	ContrastBg: color.NRGBA{142, 49, 235, 0xff},
	ContrastFg: color.NRGBA{0x00, 0x00, 0x00, 0xff},
}

var paletteDark = material.Palette{
	Bg:         color.NRGBA{0x12, 0x12, 0x12, 0xff},
	Fg:         color.NRGBA{227, 227, 227, 0xff},
	ContrastBg: color.NRGBA{142, 49, 235, 0xff},
	ContrastFg: color.NRGBA{227, 227, 227, 0xff},
}

func (g *GUI) Start(ctx context.Context, cancel context.CancelCauseFunc) (err error) {
	g.ctx = ctx
	g.cancel = cancel

	g.router = pages.NewRouter(g, ctx)
	g.router.Register(settings.New, settings.ID)
	g.router.Register(worlds.New, worlds.ID)
	g.router.Register(skins.New, skins.ID)
	g.router.Register(packs.New, packs.ID)
	g.router.SwitchTo(settings.ID)

	g.logger.g = g
	g.router.LogWidget = g.logger.Layout
	logrus.AddHook(&g.logger)

	settings.AddressInput.SetGuim(g)
	g.window.Option(app.Title("Bedrocktool " + utils.Version))
	g.window.Option(app.Size(800, 700))
	g.window.Option(app.MinSize(600, 700))

	if !utils.IsDebug() {
		go updater.UpdateCheck(g)
	}

	utils.ErrorHandler = func(err error) {
		utils.PrintPanic(err)
		g.router.RemovePopup("connect")
		g.router.PushPopup(popups.NewErrorPopup(g, err, true, func() {
			g.router.SwitchTo("settings")
		}))
	}

	go func() {
		err := g.loop()
		if err != nil && !errors.Is(err, context.Canceled) {
			cancel(err)
		}
		os.Exit(0)
	}()

	app.Main()
	return nil
}

func (g *GUI) loop() error {
	var closing = false
	var ops op.Ops

	go func() {
		<-g.ctx.Done()
		g.window.Invalidate()
	}()

	for {
		event := gioplugins.Hijack(&g.window)
		g.explorer.ListenEvents(event)

		if g.ctx.Err() != nil && !closing {
			logrus.Infof("Closing %s", context.Cause(g.ctx))
			g.cancel(errors.New("Closing"))
			g.router.Wait()
			closing = true
		}
		switch event := event.(type) {
		case app.DestroyEvent:
			g.router.ShuttingDown = true
			logrus.Info("Closing")
			g.cancel(errors.New("Closing"))
			g.router.Wait()
			return event.Err

		case app.FrameEvent:
			//event.Metric = unit.Metric{PxPerDp: 2.625, PxPerSp: 2.625}
			gtx := app.NewContext(&ops, event)
			g.router.Tick(gtx.Now)
			g.router.Layout(gtx, g.theme)
			event.Frame(gtx.Ops)

		case app.ViewEvent:
			g.hyperlink.Configure(giohyperlink.NewConfigFromViewEvent(&g.window, event))
		}
	}
}

func (g *GUI) eventHandler(event any) error {
	err := g.router.HandleEvent(event)
	g.Invalidate()
	return err
}

func (g *GUI) ClosePopup(id string) {
	g.router.RemovePopup(id)
}

func (g *GUI) StartSubcommand(subCommand string, settings any) {
	cmd, ok := commands.Registered[subCommand]
	if !ok {
		logrus.Errorf("unknown subcommand %s", subCommand)
		return
	}
	g.router.SwitchTo(cmd.Name())
	g.router.Execute(cmd, settings)
}

func (g *GUI) ExitSubcommand() {
	g.router.ExitSubcommand()
}

func (g *GUI) Invalidate() {
	g.window.Invalidate()
}

func (g *GUI) ShowPopup(pop any) {
	g.router.PushPopup(pop.(popups.Popup))
}

func (g *GUI) Error(err error) error {
	g.router.PushPopup(popups.NewErrorPopup(g, err, false, nil))
	return nil
}

func (g *GUI) Explorer() *explorer.Explorer {
	return g.explorer
}

func (g *GUI) OpenUrl(uri string) {
	_uri, _ := url.Parse(uri)
	err := g.hyperlink.OpenUnsafe(_uri)
	if err != nil {
		logrus.Errorf("OpenUrl: %s", err)
	}
}

func (g *GUI) Toast(gtx layout.Context, t string) {
	gtx.Execute(op.InvalidateCmd{At: time.Now().Add(5 * time.Second)})
	g.router.Toast(t)
}

func (g *GUI) CloseLogs() {
	g.router.CloseLogs()
}
