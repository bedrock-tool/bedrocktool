package packs

import (
	"fmt"
	"image"
	"image/color"
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

const ID = "packs"

type Page struct {
	router    *pages.Router
	Packs     []*packEntry
	packsList widget.List
	l         sync.Mutex

	finished bool
	back     widget.Clickable
}

type packEntry struct {
	Processing bool
	IsFinished bool
	UUID       string

	HasIcon bool
	Icon    paint.ImageOp
	button  widget.Clickable

	Size   uint64
	Loaded uint64
	Name   string
	Path   string
	Err    error
}

func New(router *pages.Router) pages.Page {
	return &Page{
		router: router,
		packsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

func (p *Page) ID() string {
	return ID
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

func drawPackEntry(gtx C, th *material.Theme, pack *packEntry) D {
	var size = ""
	var colorSize = th.Palette.Fg
	if pack.IsFinished {
		size = utils.SizeofFmt(float32(pack.Size))
	} else {
		size = fmt.Sprintf("%s / %s  %.02f%%",
			utils.SizeofFmt(float32(pack.Loaded)),
			utils.SizeofFmt(float32(pack.Size)),
			float32(pack.Loaded)/float32(pack.Size)*100,
		)
		colorSize = color.NRGBA{0x00, 0xc9, 0xc9, 0xff}
	}

	return layout.Inset{
		Top: 5, Bottom: 5,
		Left: 0, Right: 5,
	}.Layout(gtx, func(gtx C) D {
		fn := func(gtx C) D {
			return component.Surface(&material.Theme{
				Palette: material.Palette{
					Bg: component.WithAlpha(th.Fg, 10),
				},
			}).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return drawPackIcon(gtx, pack.HasIcon, pack.Icon, image.Pt(50, 50))
					}),
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Label(th, th.TextSize, pack.Name).Layout),
							layout.Rigid(material.LabelStyle{
								Text:           size,
								Color:          colorSize,
								SelectionColor: MulAlpha(th.Palette.ContrastBg, 0x60),
								TextSize:       th.TextSize,
								Shaper:         th.Shaper,
							}.Layout),
							layout.Rigid(func(gtx C) D {
								var c color.NRGBA
								var t string

								if pack.Err != nil {
									c = color.NRGBA{0xbb, 0x00, 0x00, 0xff}
									t = pack.Err.Error()
								} else if pack.Processing {
									c = th.Fg
									t = "Processing"
								} else if pack.IsFinished {
									c = th.Fg
									t = "Finished"
								} else if pack.Loaded == pack.Size {
									c = th.Fg
									t = "Downloaded"
								} else {
									c = th.Fg
									t = "Downloading"
								}

								if t != "" {
									return material.LabelStyle{
										TextSize: th.TextSize,
										Color:    c,
										Text:     t,
										Shaper:   th.Shaper,
									}.Layout(gtx)
								} else {
									return D{}
								}
							}),
						)
					}),
				)
			})
		}

		if pack.Path != "" {
			return material.Clickable(gtx, &pack.button, fn)
		} else {
			return fn(gtx)
		}

	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	for _, pack := range p.Packs {
		if pack.button.Clicked(gtx) {
			if pack.IsFinished {
				utils.ShowFile(pack.Path)
			}
		}
	}

	if p.back.Clicked(gtx) {
		p.router.SwitchTo("settings")
		return D{}
	}

	var title = "Downloading Packs"
	if p.finished {
		title = "Downloaded Packs"
	}

	return layout.Inset{
		Top:    unit.Dp(25),
		Bottom: unit.Dp(25),
		Right:  unit.Dp(35),
		Left:   unit.Dp(35),
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{
			Axis:    layout.Vertical,
			Spacing: layout.SpaceBetween,
		}.Layout(gtx,
			layout.Flexed(0.9, func(gtx C) D {
				return layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Bottom: 5,
						}.Layout(gtx, material.Label(th, 20, title).Layout)
					}),
					layout.Flexed(1, func(gtx C) D {
						p.l.Lock()
						defer p.l.Unlock()

						return material.List(th, &p.packsList).Layout(gtx, len(p.Packs), func(gtx C, index int) D {
							pack := p.Packs[index]
							return drawPackEntry(gtx, th, pack)
						})
					}),
				)
			}),
			layout.Flexed(0.1, func(gtx C) D {
				if !p.finished {
					return D{}
				}
				return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max.Y = gtx.Dp(40)
					gtx.Constraints.Max.X = gtx.Constraints.Max.X / 6
					return material.Button(th, &p.back, "Return").Layout(gtx)
				})
			}),
		)
	})
}

func (p *Page) Handler(data interface{}) messages.Response {
	r := messages.Response{
		Ok:   false,
		Data: nil,
	}

	switch m := data.(type) {
	case messages.ConnectState:
		if m == messages.ConnectStateReceivingResources {
			p.router.RemovePopup("connect")
		}
	case messages.InitialPacksInfo:
		p.l.Lock()
		for _, dp := range m.Packs {
			p.Packs = append(p.Packs, &packEntry{
				IsFinished: false,
				UUID:       dp.UUID,
				Name:       dp.SubPackName + " v" + dp.Version,
				Size:       dp.Size,
			})
		}
		p.l.Unlock()
		p.router.Invalidate()

	case messages.PackDownloadProgress:
		p.l.Lock()
		for _, pe := range p.Packs {
			if pe.UUID == m.UUID {
				pe.Loaded += m.LoadedAdd
				break
			}
		}
		p.l.Unlock()
		p.router.Invalidate()

	case messages.FinishedPack:
		for _, pe := range p.Packs {
			if pe.UUID == m.Pack.UUID() {
				if m.Pack.Icon() != nil {
					pe.Icon = paint.NewImageOp(m.Pack.Icon())
					pe.Icon.Filter = paint.FilterNearest
					pe.HasIcon = true
				}
				pe.Name = m.Pack.Name() + " v" + m.Pack.Version()
				pe.Loaded = pe.Size
				break
			}
		}

	case messages.ProcessingPack:
		p.l.Lock()
		allFinished := true
		for _, pe := range p.Packs {
			if pe.UUID == m.ID {
				pe.Processing = m.Processing
				pe.Err = m.Err
				if m.Path != "" {
					pe.Path = m.Path
					pe.IsFinished = true
				}
			}
			if !pe.IsFinished {
				allFinished = false
			}
		}
		if allFinished {
			p.finished = true
		}
		p.l.Unlock()
		p.router.Invalidate()
		r.Ok = true
	}

	return r
}
