package packs

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/bedrock-tool/bedrocktool/utils"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	router *pages.Router

	finished        bool
	packsList       widget.List
	packShowButtons map[string]*widget.Clickable
	l               sync.Mutex
	Packs           map[string]*packEntry
}

type packEntry struct {
	IsFinished bool
	UUID       string

	HasIcon bool
	Icon    paint.ImageOp

	Size   uint64
	Loaded uint64
	Name   string
	Path   string
	Err    error
}

func New(router *pages.Router) *Page {
	return &Page{
		router: router,
		packsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Packs:           make(map[string]*packEntry),
		packShowButtons: make(map[string]*widget.Clickable),
	}
}

func (p *Page) ID() string {
	return "packs"
}

var _ pages.Page = &Page{}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "Pack Download",
		//Icon: icon.OtherIcon,
	}
}
func drawPackIcon(gtx C, hasImage bool, imageOp paint.ImageOp, bounds image.Point) D {
	return layout.Inset{
		Top:    unit.Dp(5),
		Bottom: unit.Dp(5),
		Right:  unit.Dp(5),
		Left:   unit.Dp(5),
	}.Layout(gtx, func(gtx C) D {
		if hasImage {
			imageOp.Add(gtx.Ops)
			s := imageOp.Size()
			p := f32.Pt(float32(s.X), float32(s.Y))
			p.X = 1 / (p.X / float32(bounds.X))
			p.Y = 1 / (p.Y / float32(bounds.Y))
			defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), p)).Push(gtx.Ops).Pop()
			paint.PaintOp{}.Add(gtx.Ops)
		}
		return D{Size: bounds}
	})
}

func MulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 0xFF)
	return c
}

func drawPackEntry(gtx C, th *material.Theme, entry *packEntry, button *widget.Clickable) D {
	var size = ""
	var colorSize = th.Palette.Fg
	if entry.IsFinished {
		size = utils.SizeofFmt(float32(entry.Size))
	} else {
		size = fmt.Sprintf("%s / %s  %.02f%%",
			utils.SizeofFmt(float32(entry.Loaded)),
			utils.SizeofFmt(float32(entry.Size)),
			float32(entry.Loaded)/float32(entry.Size)*100,
		)
		colorSize = color.NRGBA{0x00, 0xc9, 0xc9, 0xff}
	}

	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		fn := func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return drawPackIcon(gtx, entry.HasIcon, entry.Icon, image.Pt(50, 50))
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
						layout.Rigid(material.LabelStyle{
							Text:           size,
							Color:          colorSize,
							SelectionColor: MulAlpha(th.Palette.ContrastBg, 0x60),
							TextSize:       th.TextSize,
							Shaper:         th.Shaper,
						}.Layout),
						layout.Rigid(func(gtx C) D {
							if entry.Err != nil {
								return material.LabelStyle{
									Color: color.NRGBA{0xbb, 0x00, 0x00, 0xff},
									Text:  entry.Err.Error(),
								}.Layout(gtx)
							}
							return D{}
						}),
					)
				}),
			)
		}

		if entry.Path != "" {
			return material.ButtonLayoutStyle{
				Background:   MulAlpha(th.Palette.Bg, 0x60),
				Button:       button,
				CornerRadius: 3,
			}.Layout(gtx, fn)
		} else {
			return fn(gtx)
		}

	})
}

func (p *Page) layoutFinished(gtx C, th *material.Theme) D {
	for uuid, button := range p.packShowButtons {
		if button.Clicked() {
			pack := p.Packs[uuid]
			if pack.IsFinished {
				utils.ShowFile(pack.Path)
			}
		}
	}

	var title = "Downloading Packs"
	if p.finished {
		title = "Downloaded Packs"
	}

	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(material.Label(th, 20, title).Layout),
			layout.Flexed(1, func(gtx C) D {
				p.l.Lock()
				defer p.l.Unlock()

				keys := make([]string, 0, len(p.Packs))
				for k := range p.Packs {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				return material.List(th, &p.packsList).Layout(gtx, len(keys), func(gtx C, index int) D {
					entry := p.Packs[keys[index]]
					button := p.packShowButtons[keys[index]]
					return drawPackEntry(gtx, th, entry, button)
				})
			}),
		)
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	return layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return p.layoutFinished(gtx, th)
	})
}

func (p *Page) Handler(data interface{}) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.InitialPacksInfo:
		p.l.Lock()
		for _, dp := range m.Packs {
			e := &packEntry{
				IsFinished: false,
				UUID:       dp.UUID,
				Name:       dp.SubPackName + " v" + dp.Version,
				Size:       dp.Size,
			}
			p.Packs[e.UUID] = e
			p.packShowButtons[e.UUID] = &widget.Clickable{}
		}
		p.l.Unlock()
		p.router.Invalidate()

	case messages.PackDownloadProgress:
		p.l.Lock()
		e, ok := p.Packs[m.UUID]
		if ok {
			e.Loaded += m.LoadedAdd
			if e.Loaded == e.Size {
				e.IsFinished = true
			}
		}
		p.l.Unlock()
		p.router.Invalidate()

	case messages.FinishedDownloadingPacks:
		p.l.Lock()
		for _, dp := range m.Packs {
			e, ok := p.Packs[dp.UUID]
			if !ok {
				continue
			}
			if dp.Icon != nil {
				e.Icon = paint.NewImageOpFilter(dp.Icon, paint.FilterNearest)
				e.HasIcon = true
			}
			e.Err = dp.Err
			e.IsFinished = true
			e.Path = dp.Path
		}
		p.l.Unlock()
		p.router.Invalidate()
		r.Ok = true
	}

	return r
}
