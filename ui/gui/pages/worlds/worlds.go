package worlds

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
	"github.com/bedrock-tool/bedrocktool/ui/messages"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type savedWorld struct {
	Name     string
	Filepath string
	Chunks   int
	Entities int
}

const ID = "worlds"

type Page struct {
	g guim.Guim

	worldMap           *Map2
	finishedWorldsMu   sync.Mutex
	finishedWorlds     []*savedWorld
	finishedWorldsList widget.List
	processingWorlds   []*processingWorld
	haveConnected      bool

	State     messages.UIState
	voidGen   bool
	worldName string
	back      widget.Clickable
}

func New(g guim.Guim) pages.Page {
	return &Page{
		g: g,

		worldMap: &Map2{
			tileImages: make(map[image.Point]*image.RGBA),
			imageOps:   make(map[image.Point]paint.ImageOp),
		},
		finishedWorldsList: widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
}

type processingWorld struct {
	Name  string
	State string
}

var _ pages.Page = &Page{}

func (p *Page) ID() string {
	return ID
}

func (p *Page) Actions(th *material.Theme) []component.AppBarAction {
	return []component.AppBarAction{
		//pages.AppBarSwitch(&p.worldMap.mapInput.FollowPlayer, "Follow Player", th),
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "World Downloader",
		//Icon: icon.OtherIcon,
	}
}

// saveSteps are the steps of saving a world, in order, and the state the
// saver reports for each starts with.
var saveSteps = []struct{ prefix, label string }{
	{"Saving", "Preparing"},
	{"Storing Chunks", "Storing chunks"},
	{"Storing Entities", "Storing entities"},
	{"Adding Resourcepack", "Adding resource packs"},
	{"Adding Behaviorpack", "Adding the behavior pack"},
	{"Writing mcworld file", "Writing the .mcworld file"},
}

func saveStepIndex(state string) int {
	for i := len(saveSteps) - 1; i >= 0; i-- {
		if strings.HasPrefix(state, saveSteps[i].prefix) {
			return i
		}
	}
	return 0
}

func muted(th *material.Theme) color.NRGBA {
	c := th.Fg
	c.A = 150
	return c
}

func card(gtx C, th *material.Theme, w layout.Widget) D {
	return layout.Background{}.Layout(gtx,
		func(gtx C) D {
			bg := th.Fg
			bg.A = 14
			return component.Rect{Color: bg, Size: gtx.Constraints.Min, Radii: gtx.Dp(popups.Radius)}.Layout(gtx)
		},
		func(gtx C) D { return layout.UniformInset(14).Layout(gtx, w) },
	)
}

// stepMark is the box before a save step: filled when done, a spinner
// while it runs, dim while it waits.
func stepMark(gtx C, th *material.Theme, done, current bool) D {
	size := gtx.Dp(14)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	switch {
	case current:
		return material.Loader(th).Layout(gtx)
	case done:
		return component.Rect{Color: th.ContrastBg, Size: image.Pt(size, size), Radii: gtx.Dp(2)}.Layout(gtx)
	}
	c := th.Fg
	c.A = 50
	return component.Rect{Color: c, Size: image.Pt(size, size), Radii: gtx.Dp(2)}.Layout(gtx)
}

func displayWorldProcessing(gtx C, th *material.Theme, entry *processingWorld) D {
	current := saveStepIndex(entry.State)
	return card(gtx, th, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D {
				l := material.Label(th, th.TextSize*1.25, "Saving "+entry.Name)
				l.Font.Weight = font.SemiBold
				return layout.Inset{Bottom: 10}.Layout(gtx, l.Layout)
			}),
		}
		for i, step := range saveSteps {
			label := step.label
			if i == current && strings.HasPrefix(entry.State, "Adding Resourcepack ") {
				label = "Adding resource pack " + strings.TrimPrefix(entry.State, "Adding Resourcepack ")
			}
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: 3, Bottom: 3}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx C) D { return stepMark(gtx, th, i < current, i == current) }),
						layout.Rigid(layout.Spacer{Width: 10}.Layout),
						layout.Flexed(1, func(gtx C) D {
							l := material.Body1(th, label)
							l.MaxLines = 1
							if i == current {
								l.Font.Weight = font.SemiBold
							} else {
								l.Color = muted(th)
							}
							return l.Layout(gtx)
						}),
					)
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// displayWorldEntry is a saved world's card. With dir (the folder all the
// saved worlds are in, shown above them) the card shows only its file name.
func displayWorldEntry(gtx C, th *material.Theme, entry *savedWorld, dir string) D {
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx C) D {
		return card(gtx, th, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				// name on the left, counts on the right
				layout.Rigid(func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							l := material.Body1(th, entry.Name)
							l.Font.Weight = font.SemiBold
							l.MaxLines = 1
							return l.Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							l := material.Body2(th, fmt.Sprintf("%s chunks  ·  %s entities", thousands(entry.Chunks), thousands(entry.Entities)))
							l.Color = muted(th)
							return layout.Inset{Left: 12}.Layout(gtx, l.Layout)
						}),
					)
				}),
				layout.Rigid(func(gtx C) D {
					file := filepath.ToSlash(entry.Filepath)
					if dir != "" {
						file = filepath.Base(entry.Filepath)
						if file == entry.Name+".mcworld" {
							return D{} // it would only repeat the name
						}
					}
					l := material.Body2(th, file)
					l.Color = muted(th)
					l.MaxLines = 1
					return layout.Inset{Top: 4}.Layout(gtx, l.Layout)
				}),
			)
		})
	})
}

func (p *Page) layoutFinished(gtx C, th *material.Theme) D {
	// the folder every saved world is in, if they share one
	p.finishedWorldsMu.Lock()
	dir := ""
	for i, w := range p.finishedWorlds {
		d := filepath.Dir(w.Filepath)
		if i == 0 {
			dir = d
		} else if d != dir {
			dir = ""
			break
		}
	}
	p.finishedWorldsMu.Unlock()
	return layout.N.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(720))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(20).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Label(th, th.TextSize*1.4, "Saved worlds").Layout),
							layout.Rigid(func(gtx C) D {
								if dir == "" {
									return D{}
								}
								l := material.Body2(th, "In "+filepath.ToSlash(dir))
								l.Color = muted(th)
								l.MaxLines = 1
								return layout.Inset{Top: 2}.Layout(gtx, l.Layout)
							}),
						)
					})
				}),
				layout.Flexed(1, func(gtx C) D {
					p.finishedWorldsMu.Lock()
					defer p.finishedWorldsMu.Unlock()
					if len(p.finishedWorlds) == 0 && len(p.processingWorlds) == 0 {
						l := material.Body1(th, "Nothing was saved: no chunks were loaded.")
						l.Color = muted(th)
						return l.Layout(gtx)
					}
					return material.List(th, &p.finishedWorldsList).
						Layout(gtx, len(p.finishedWorlds), func(gtx C, index int) D {
							return layout.Inset{Right: 10}.Layout(gtx, func(gtx C) D {
								return displayWorldEntry(gtx, th, p.finishedWorlds[len(p.finishedWorlds)-index-1], dir)
							})
						})
				}),
				layout.Rigid(func(gtx C) D {
					return layout.Inset{Top: 12}.Layout(gtx, func(gtx C) D {
						b := material.Button(th, &p.back, "Back")
						b.CornerRadius = popups.Radius
						gtx.Constraints.Min.X = gtx.Dp(120)
						return b.Layout(gtx)
					})
				}),
			)
		})
	})
}

func (p *Page) layoutMap(gtx C, th *material.Theme) D {
	d := p.worldMap.Layout(gtx)
	if len(p.worldMap.tileImages) > 0 {
		return d
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(material.Body1(th, "Waiting for chunks").Layout),
			layout.Rigid(func(gtx C) D {
				l := material.Body2(th, "The map fills in as you walk around in Minecraft.")
				l.Color = muted(th)
				return l.Layout(gtx)
			}),
		)
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.back.Clicked(gtx) {
		p.g.ExitSubcommand()
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			switch p.State {
			case messages.UIStateMain:
				return p.layoutMap(gtx, th)
			case messages.UIStateFinished:
				return p.layoutFinished(gtx, th)
			}
			return D{}
		}),
		// worlds being saved, in the middle, over the map
		layout.Expanded(func(gtx C) D {
			p.finishedWorldsMu.Lock()
			processing := slices.Clone(p.processingWorlds)
			p.finishedWorldsMu.Unlock()
			if len(processing) == 0 {
				return D{}
			}
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Center.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X-gtx.Dp(32), gtx.Dp(420))
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				var children []layout.FlexChild
				for _, entry := range processing {
					children = append(children, layout.Rigid(func(gtx C) D {
						return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx C) D {
							// opaque, so the map doesn't show through
							return layout.Background{}.Layout(gtx,
								func(gtx C) D {
									return component.Rect{Color: th.Bg, Size: gtx.Constraints.Min, Radii: gtx.Dp(popups.Radius)}.Layout(gtx)
								},
								func(gtx C) D { return displayWorldProcessing(gtx, th, entry) },
							)
						})
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
	)
}

func (p *Page) HaveFinishScreen() bool {
	return p.haveConnected
}

func (u *Page) HandleEvent(event any) error {
	switch event := event.(type) {
	case *messages.EventSetUIState:
		u.State = event.State

	case *messages.EventMapTiles:
		u.worldMap.AddTiles(event.Tiles)

	case *messages.EventResetMap:
		u.worldMap.Reset()

	case *messages.EventPlayerPosition:
		u.worldMap.mapInput.playerPosition = event.Position

	case *messages.EventFinishedSavingWorld:
		u.finishedWorldsMu.Lock()
		u.finishedWorlds = append(u.finishedWorlds, &savedWorld{
			Name:     event.WorldName,
			Filepath: event.Filepath,
			Chunks:   event.Chunks,
			Entities: event.Entities,
		})
		u.processingWorlds = slices.DeleteFunc(u.processingWorlds, func(p *processingWorld) bool {
			return p.Name == event.WorldName
		})
		u.finishedWorldsMu.Unlock()

	case *messages.EventProcessingWorldUpdate:
		u.finishedWorldsMu.Lock()
		if event.State == "" {
			u.processingWorlds = slices.DeleteFunc(u.processingWorlds, func(w *processingWorld) bool {
				return w.Name == event.WorldName
			})
		} else {
			exists := false
			for _, w := range u.processingWorlds {
				if w.Name == event.WorldName {
					w.State = event.State
					exists = true
					break
				}
			}
			if !exists {
				u.processingWorlds = append(u.processingWorlds, &processingWorld{
					Name:  event.WorldName,
					State: event.State,
				})
			}
		}
		u.finishedWorldsMu.Unlock()

	case *messages.EventConnectStateUpdate:
		if event.State == messages.ConnectStateDone {
			u.haveConnected = true
		}
	}
	return nil
}

func (p *Page) SetValue(name, value string) {
	switch name {
	case "voidGen":
		switch value {
		case "true":
			p.voidGen = true
		case "false":
			p.voidGen = true
		}
	case "worldName":
		p.worldName = value
	}
}

// thousands writes n with commas between thousands (2311 -> "2,311").
func thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}
