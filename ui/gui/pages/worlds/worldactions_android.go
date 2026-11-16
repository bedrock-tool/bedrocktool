package worlds

import (
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/android"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
)

// worldActions are the buttons under a saved world's card. The world is in
// the app's own folder, which phones don't let file managers or Minecraft
// open, so Android gets ways to get it out.
type worldActions struct {
	export widget.Clickable
	open   widget.Clickable

	mu     sync.Mutex
	busy   bool
	status string
	failed bool
}

// run does a slow action off the GUI's goroutine and shows how it went.
func (a *worldActions) run(g guim.Guim, f func() (string, error)) {
	a.mu.Lock()
	if a.busy {
		a.mu.Unlock()
		return
	}
	a.busy, a.status, a.failed = true, android.T("android_working", nil), false
	a.mu.Unlock()
	go func() {
		msg, err := f()
		a.mu.Lock()
		a.busy, a.status, a.failed = false, msg, err != nil
		if err != nil {
			a.status = err.Error()
		}
		a.mu.Unlock()
		g.Invalidate()
	}()
}

func actionButton(gtx C, th *material.Theme, c *widget.Clickable, text string, primary bool) D {
	b := material.Button(th, c, text)
	b.CornerRadius = popups.Radius
	if !primary {
		b.Background = component.WithAlpha(th.Fg, 30)
		b.Color = th.Fg
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return b.Layout(gtx)
}

func (a *worldActions) Layout(gtx C, th *material.Theme, g guim.Guim, entry *savedWorld) D {
	path := entry.Filepath
	if a.export.Clicked(gtx) {
		a.run(g, func() (string, error) { return android.ExportWorld(path) })
	}
	if a.open.Clicked(gtx) {
		a.run(g, func() (string, error) { return android.OpenInMinecraft(path) })
	}

	a.mu.Lock()
	status, failed := a.status, a.failed
	a.mu.Unlock()

	return layout.Inset{Top: 10}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D {
						return actionButton(gtx, th, &a.open, android.T("android_open_in_minecraft", nil), true)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1, func(gtx C) D {
						return actionButton(gtx, th, &a.export, android.T("android_export", nil), false)
					}),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if status == "" {
					return D{}
				}
				l := material.Body2(th, status)
				l.Color = muted(th)
				if failed {
					l.Color = th.ContrastBg
				}
				return layout.Inset{Top: 6}.Layout(gtx, l.Layout)
			}),
		)
	})
}
