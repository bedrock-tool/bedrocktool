package packs

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/styledtext"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/mctext"
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
	g guim.Guim

	Packs     []*packEntry
	packsList widget.List
	l         sync.Mutex
	onlyKeys  bool

	State messages.UIState
	back  widget.Clickable

	frame int
}

type packEntry struct {
	Processing bool
	IsFinished bool
	UUID       string

	HasIcon   bool
	Icon      paint.ImageOp
	button    widget.Clickable
	keySelect widget.Selectable

	Size    uint64
	Loaded  uint64
	Name    string
	Version string
	Path    string
	Key     string
	Err     error
}

func New(g guim.Guim) pages.Page {
	return &Page{
		g: g,

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

func (p *Page) Actions(th *material.Theme) []component.AppBarAction {
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

func (p *Page) drawPackEntry(gtx C, th *material.Theme, pack *packEntry, onlyKeys bool) D {
	var size = ""
	var colorSize = th.Palette.Fg
	if pack.IsFinished {
		size = utils.SizeofFmt(float32(pack.Size))
	} else {
		size = fmt.Sprintf("%s / %s %.02f%%",
			utils.SizeofFmt(float32(pack.Loaded)),
			utils.SizeofFmt(float32(pack.Size)),
			float32(pack.Loaded)/float32(pack.Size)*100,
		)
		colorSize = color.NRGBA{0x00, 0xc9, 0xc9, 0xff}
	}

	return layout.Inset{
		Top: 5, Bottom: 5,
		Left: 5, Right: 5,
	}.Layout(gtx, func(gtx C) D {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.Fg, 10),
			},
		}).Layout(gtx, func(gtx C) D {
			return layout.UniformInset(5).
				Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.
						Layout(gtx,
							layout.Rigid(func(gtx C) D {
								iconSize := image.Pt(50, 50)
								return drawPackIcon(gtx, pack.HasIcon, pack.Icon, iconSize)
							}),
							layout.Flexed(1, func(gtx C) D {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(mctext.Label(th, th.TextSize, pack.Name+" §rv"+pack.Version, p.g.Invalidate, p.frame)),
									layout.Rigid(func(gtx C) D {
										if onlyKeys {
											t := material.Body1(th, pack.Key)
											t.State = &pack.keySelect
											return t.Layout(gtx)
										}

										var styles = []styledtext.SpanStyle{
											{
												Font:    font.Font{Typeface: th.Face},
												Size:    th.TextSize,
												Color:   colorSize,
												Content: size,
											},
										}
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
											t = ""
										} else if pack.Loaded == pack.Size {
											c = th.Fg
											t = "Downloaded"
										} else {
											c = th.Fg
											t = "Downloading"
										}

										if t != "" {
											styles = append(styles, styledtext.SpanStyle{
												Font:    font.Font{Typeface: th.Face},
												Size:    th.TextSize,
												Color:   th.Fg,
												Content: " | ",
											}, styledtext.SpanStyle{
												Font:    font.Font{Typeface: th.Face},
												Size:    th.TextSize,
												Color:   c,
												Content: t,
											})
										}

										return styledtext.Text(th.Shaper, styles...).Layout(gtx, nil)
									}),
								)
							}),
							layout.Rigid(func(gtx C) D {
								if pack.Path != "" {
									return layout.Flex{
										Axis:      layout.Vertical,
										Alignment: layout.End,
									}.Layout(gtx, layout.Rigid(material.Button(th, &pack.button, "Show").Layout))
								}
								return D{}
							}),
						)
				})
		})
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	p.frame++

	for _, pack := range p.Packs {
		if pack.button.Clicked(gtx) {
			if pack.IsFinished {
				utils.ShowFile(pack.Path)
			}
		}
	}

	if p.back.Clicked(gtx) {
		p.g.ExitSubcommand()
	}

	var title = "Downloading Packs"
	if p.State == messages.UIStateFinished {
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

						return material.List(th, &p.packsList).
							Layout(gtx, len(p.Packs), func(gtx C, index int) D {
								pack := p.Packs[index]
								return p.drawPackEntry(gtx, th, pack, p.onlyKeys)
							})
					}),
				)
			}),
			layout.Flexed(0.1, func(gtx C) D {
				if p.State != messages.UIStateFinished {
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

func (*Page) HaveFinishScreen() bool {
	return true
}

func (p *Page) HandleEvent(event any) error {
	switch event := event.(type) {
	case *messages.EventSetUIState:
		p.State = event.State

	case *messages.EventConnectStateUpdate:
		if event.State == messages.ConnectStateReceivingResources {
			p.g.ClosePopup("connect")
		}

	case *messages.EventInitialPacksInfo:
		p.l.Lock()
		for _, dp := range event.Packs {
			pack := &packEntry{
				IsFinished: false,
				UUID:       dp.UUID.String(),
				Name:       strings.ReplaceAll(dp.SubPackName, "\n", " "),
				Version:    dp.Version,
				Size:       dp.Size,
				Key:        dp.ContentKey,
			}
			if pack.Name == "" {
				pack.Name = pack.UUID
			}
			if event.KeysOnly {
				pack.IsFinished = true
				p.onlyKeys = true
			}
			p.Packs = append(p.Packs, pack)
		}
		p.l.Unlock()

	case *messages.EventProcessingPack:
		p.l.Lock()
		for _, pe := range p.Packs {
			if pe.UUID == event.ID {
				pe.Processing = true
				break
			}
		}
		p.l.Unlock()

	case *messages.EventPackDownloadProgress:
		p.l.Lock()
		for _, pe := range p.Packs {
			if pe.UUID == event.ID {
				pe.Loaded += uint64(event.BytesAdded)
				break
			}
		}
		p.l.Unlock()

	case *messages.EventFinishedPack:
		p.l.Lock()
		for _, pe := range p.Packs {
			if pe.UUID == event.ID {
				pe.Processing = false
				pe.Name = event.Name
				pe.Version = event.Version
				pe.Path = event.Filepath
				pe.IsFinished = true
				if event.Icon != nil {
					pe.HasIcon = true
					pe.Icon = paint.NewImageOp(event.Icon)
					pe.Icon.Filter = paint.FilterNearest
				}
				pe.Err = event.Error
				pe.Loaded = pe.Size
				break
			}
		}
		p.l.Unlock()
	}

	return nil
}
