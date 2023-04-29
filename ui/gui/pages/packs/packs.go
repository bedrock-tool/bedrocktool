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
	*pages.Router

	State     messages.UIState
	packsList widget.List
	l         sync.Mutex
	Packs     map[string]*packEntry
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
		Router: router,
		packsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
		Packs: make(map[string]*packEntry),
	}
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
func drawPackIcon(ops *op.Ops, imageOp paint.ImageOp, bounds image.Point) {
	imageOp.Add(ops)

	s := imageOp.Size()
	p := f32.Pt(float32(s.X), float32(s.Y))
	p.X = 1 / (p.X / float32(bounds.X))
	p.Y = 1 / (p.Y / float32(bounds.Y))
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), p)).Push(ops).Pop()

	paint.PaintOp{}.Add(ops)
}

func MulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	c.A = uint8(uint32(c.A) * uint32(alpha) / 0xFF)
	return c
}

func drawPackEntry(gtx C, th *material.Theme, entry *packEntry) D {
	var size = ""
	var colorSize = th.Palette.Fg
	if entry.IsFinished {
		size = utils.SizeofFmt(float32(entry.Size))
	} else {
		size = fmt.Sprintf("%s / %s  %.02f%%",
			utils.SizeofFmt(float32(entry.Size)),
			utils.SizeofFmt(float32(entry.Size)),
			float32(entry.Loaded)/float32(entry.Size)*100,
		)
		colorSize = color.NRGBA{0x00, 0xc9, 0xc9, 0xff}
	}

	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				s := image.Pt(50, 50)
				if entry.HasIcon {
					drawPackIcon(gtx.Ops, entry.Icon, s)
				}
				return D{Size: s.Add(image.Pt(10, 10))}
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
	})
}

func (p *Page) layoutFinished(gtx C, th *material.Theme) D {
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(material.Label(th, 20, "Downloaded Packs").Layout),
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
					return drawPackEntry(gtx, th, entry)
				})
			}),
		)
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	margin := layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}

	return margin.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch p.State {
		case messages.UIStateMain:
			return p.layoutFinished(gtx, th)
		}
		return layout.Dimensions{}
	})
}

func (p *Page) Handler(data interface{}) messages.MessageResponse {
	r := messages.MessageResponse{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.SetUIState:
		p.State = m
		p.Router.Invalidate()
		r.Ok = true

	case messages.InitialPacksInfo:
		p.State = messages.UIStateMain
		p.l.Lock()
		for _, dp := range m.Packs {
			e := &packEntry{
				IsFinished: false,
				UUID:       dp.UUID,
				Name:       dp.SubPackName + "v" + dp.Version,
				Size:       dp.Size,
			}
			p.Packs[e.UUID] = e
		}
		p.l.Unlock()
		p.Router.Invalidate()

	case messages.PackDownloadProgress:
		p.l.Lock()
		e := p.Packs[m.UUID]
		e.Loaded += m.LoadedAdd
		if e.Loaded == e.Size {
			e.IsFinished = true
		}
		p.l.Unlock()
		p.Router.Invalidate()

	case messages.FinishedDownloadingPacks:
		p.l.Lock()
		for _, dp := range m.Packs {
			e := p.Packs[dp.UUID]
			if dp.Icon != nil {
				e.Icon = paint.NewImageOpFilter(dp.Icon, paint.FilterNearest)
				e.HasIcon = true
			}
			e.Err = dp.Err
			e.IsFinished = true
		}
		p.l.Unlock()
		p.Router.Invalidate()
		r.Ok = true
	}

	return r
}
