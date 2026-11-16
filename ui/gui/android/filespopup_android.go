package android

import (
	"fmt"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/locale"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
)

// FilesPopup lists the captures and saved worlds in the app's folder,
// which no file manager can open on a phone, with what can be done to
// each: replay a capture (as it was saved, or merged into one world), open
// a world in Minecraft, export either, or delete it.
type FilesPopup struct {
	g guim.Guim
	// replay starts saving worlds from a capture; merge is "Merge all
	// worlds into one".
	replay func(path string, merge bool)

	list  widget.List
	close widget.Clickable

	mu       sync.Mutex
	loaded   bool
	captures []*fileRow
	worlds   []*fileRow
}

type fileRow struct {
	File

	replay, replayMerged, open, export, remove widget.Clickable

	mu     sync.Mutex
	busy   bool
	status string
	failed bool
}

var _ popups.Popup = (*FilesPopup)(nil)

func NewFilesPopup(g guim.Guim, replay func(path string, merge bool)) *FilesPopup {
	p := &FilesPopup{g: g, replay: replay}
	p.list.Axis = layout.Vertical
	go p.load()
	return p
}

func (*FilesPopup) ID() string                  { return "files" }
func (*FilesPopup) Close() error                { return nil }
func (*FilesPopup) HandleEvent(event any) error { return nil }

// load reads the folders off the GUI's goroutine: worlds/ can be big.
func (p *FilesPopup) load() {
	var captures, worlds []*fileRow
	for _, f := range Captures() {
		captures = append(captures, &fileRow{File: f})
	}
	for _, f := range Worlds() {
		worlds = append(worlds, &fileRow{File: f})
	}
	p.mu.Lock()
	p.captures, p.worlds, p.loaded = captures, worlds, true
	p.mu.Unlock()
	p.g.Invalidate()
}

// run does a slow action off the GUI's goroutine and shows how it went
// under the file.
func (r *fileRow) run(g guim.Guim, f func() (string, error)) {
	r.mu.Lock()
	if r.busy {
		r.mu.Unlock()
		return
	}
	r.busy, r.status, r.failed = true, T("android_working", nil), false
	r.mu.Unlock()
	go func() {
		msg, err := f()
		r.mu.Lock()
		r.busy, r.status, r.failed = false, msg, err != nil
		if err != nil {
			r.status = err.Error()
		}
		r.mu.Unlock()
		g.Invalidate()
	}()
}

func (p *FilesPopup) confirmDelete(r *fileRow) {
	name := r.Name
	if r.Group != "" {
		name = r.Group + " / " + r.Name
	}
	p.g.ShowPopup(popups.NewConfirm(p.g, "files-delete",
		T("android_delete_title", locale.Strmap{"Name": name}),
		[]string{T("android_delete_text", nil)},
		T("android_keep", nil), T("android_delete", nil),
		func(proceed bool) {
			if !proceed {
				return
			}
			if err := DeleteFile(r.File); err != nil {
				r.mu.Lock()
				r.status, r.failed = err.Error(), true
				r.mu.Unlock()
			} else {
				go p.load()
			}
			p.g.Invalidate()
		}))
}

func (p *FilesPopup) Layout(gtx C, th *material.Theme) D {
	if p.close.Clicked(gtx) {
		p.g.ClosePopup(p.ID())
		return D{}
	}

	p.mu.Lock()
	loaded, captures, worlds := p.loaded, p.captures, p.worlds
	p.mu.Unlock()

	for _, r := range captures {
		if r.replay.Clicked(gtx) {
			p.g.ClosePopup(p.ID())
			p.replay(r.Path, false)
			return D{}
		}
		if r.replayMerged.Clicked(gtx) {
			p.g.ClosePopup(p.ID())
			p.replay(r.Path, true)
			return D{}
		}
		if r.export.Clicked(gtx) {
			path := r.Path
			r.run(p.g, func() (string, error) { return ExportCapture(path) })
		}
		if r.remove.Clicked(gtx) {
			p.confirmDelete(r)
		}
	}
	for _, r := range worlds {
		path := r.Path
		if r.open.Clicked(gtx) {
			r.run(p.g, func() (string, error) { return OpenInMinecraft(path) })
		}
		if r.export.Clicked(gtx) {
			r.run(p.g, func() (string, error) { return ExportWorld(path) })
		}
		if r.remove.Clicked(gtx) {
			p.confirmDelete(r)
		}
	}

	// one list of: title, captures heading, captures, worlds heading, worlds
	type item func(gtx C) D
	var items []item
	items = append(items, func(gtx C) D {
		return layout.Inset{Bottom: 4}.Layout(gtx, material.Label(th, th.TextSize*1.4, T("android_files_title", nil)).Layout)
	})
	if !loaded {
		items = append(items, func(gtx C) D { return mutedText(gtx, th, T("android_working", nil)) })
	} else {
		items = append(items, heading(th, T("android_captures", nil), T("android_captures_text", nil), len(captures)))
		for _, r := range captures {
			items = append(items, func(gtx C) D { return p.layoutCapture(gtx, th, r) })
		}
		items = append(items, heading(th, T("android_worlds", nil), T("android_worlds_text", nil), len(worlds)))
		for _, r := range worlds {
			items = append(items, func(gtx C) D { return p.layoutWorld(gtx, th, r) })
		}
	}

	return popups.LayoutPopupBackground(gtx, th, p.ID(), func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return material.List(th, &p.list).Layout(gtx, len(items), func(gtx C, i int) D {
					return items[i](gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D { return popups.CloseButton(gtx, th, &p.close) })
			}),
		)
	})
}

func heading(th *material.Theme, title, text string, n int) func(gtx C) D {
	return func(gtx C) D {
		return layout.Inset{Top: 16, Bottom: 4}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					l := material.Subtitle1(th, fmt.Sprintf("%s (%d)", title, n))
					l.Font.Weight = font.Bold
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if n == 0 {
						return mutedText(gtx, th, T("android_none_yet", nil))
					}
					return mutedText(gtx, th, text)
				}),
			)
		})
	}
}

func mutedText(gtx C, th *material.Theme, text string) D {
	l := material.Body2(th, text)
	l.Color = component.WithAlpha(th.Fg, 170)
	return l.Layout(gtx)
}

// fileCard is a file's name, size and date, its buttons under it in rows,
// and how its last action went.
func fileCard(gtx C, th *material.Theme, r *fileRow, title string, rows ...[]layout.Widget) D {
	r.mu.Lock()
	status, failed := r.status, r.failed
	r.mu.Unlock()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			l := material.Body1(th, title)
			l.Font.Weight = font.SemiBold
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			return mutedText(gtx, th, fmt.Sprintf("%s · %s", sizeText(r.Size), r.Modified.Format("2006-01-02 15:04")))
		}),
	}
	for _, row := range rows {
		children = append(children, layout.Rigid(func(gtx C) D {
			var cells []layout.FlexChild
			for i, w := range row {
				if i > 0 {
					cells = append(cells, layout.Rigid(layout.Spacer{Width: 8}.Layout))
				}
				cells = append(cells, layout.Flexed(1, w))
			}
			return layout.Inset{Top: 8}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
			})
		}))
	}
	children = append(children, layout.Rigid(func(gtx C) D {
		if status == "" {
			return D{}
		}
		l := material.Body2(th, status)
		l.Color = component.WithAlpha(th.Fg, 170)
		if failed {
			l.Color = th.ContrastBg
		}
		return layout.Inset{Top: 6}.Layout(gtx, l.Layout)
	}))

	return layout.Inset{Top: 8, Bottom: 4}.Layout(gtx, func(gtx C) D {
		return component.Surface(th).Layout(gtx, func(gtx C) D {
			return layout.UniformInset(10).Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func rowButton(th *material.Theme, c *widget.Clickable, text string, primary bool) layout.Widget {
	return func(gtx C) D {
		b := material.Button(th, c, text)
		b.CornerRadius = popups.Radius
		b.Inset = layout.Inset{Top: 10, Bottom: 10, Left: 6, Right: 6}
		if !primary {
			b.Background = component.WithAlpha(th.Fg, 30)
			b.Color = th.Fg
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return b.Layout(gtx)
	}
}

func (p *FilesPopup) layoutCapture(gtx C, th *material.Theme, r *fileRow) D {
	return fileCard(gtx, th, r, r.Name,
		[]layout.Widget{
			rowButton(th, &r.replay, T("android_replay", nil), true),
			rowButton(th, &r.replayMerged, T("android_replay_merged", nil), true),
		},
		[]layout.Widget{
			rowButton(th, &r.export, T("android_export", nil), false),
			rowButton(th, &r.remove, T("android_delete", nil), false),
		},
	)
}

func (p *FilesPopup) layoutWorld(gtx C, th *material.Theme, r *fileRow) D {
	title := r.Name
	if r.Group != "" {
		title = r.Group + " / " + r.Name
	}
	return fileCard(gtx, th, r, title,
		[]layout.Widget{
			rowButton(th, &r.open, T("android_open_in_minecraft", nil), true),
		},
		[]layout.Widget{
			rowButton(th, &r.export, T("android_export", nil), false),
			rowButton(th, &r.remove, T("android_delete", nil), false),
		},
	)
}

func sizeText(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
