package worlds

import (
	"fmt"
	"image"
	"slices"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
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

	worldMap             *Map2
	finishedWorldsMu     sync.Mutex
	finishedWorlds       []*savedWorld
	finishedWorldsList   widget.List
	processingWorlds     []*processingWorld
	processingWorldsList widget.List
	haveConnected        bool

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
		processingWorldsList: widget.List{
			List: layout.List{
				Axis:        layout.Vertical,
				ScrollToEnd: true,
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

func displayWorldEntry(gtx C, th *material.Theme, entry *savedWorld) D {
	return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.ContrastFg, 8),
			},
		}).Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
				layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d Chunks", entry.Chunks)).Layout),
				layout.Rigid(material.Label(th, th.TextSize, fmt.Sprintf("%d Entities", entry.Entities)).Layout),
			)
		})
	})
}

func displayWorldProcessing(gtx C, th *material.Theme, entry *processingWorld) D {
	return layout.Inset{Top: 0, Bottom: 8, Left: 8, Right: 8}.Layout(gtx, func(gtx C) D {
		return component.Surface(&material.Theme{
			Palette: material.Palette{
				Bg: component.WithAlpha(th.ContrastFg, 8),
			},
		}).Layout(gtx, func(gtx C) D {
			return layout.UniformInset(5).Layout(gtx, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Label(th, th.TextSize, "Saving World").Layout),
					layout.Rigid(material.Label(th, th.TextSize, entry.Name).Layout),
					layout.Rigid(material.Label(th, th.TextSize, entry.State).Layout),
				)
			})
		})
	})
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	if p.back.Clicked(gtx) {
		p.g.ExitSubcommand()
	}

	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx C) D {
			switch p.State {
			case messages.UIStateMain:
				return p.worldMap.Layout(gtx)
			case messages.UIStateFinished:
				return layout.UniformInset(25).Layout(gtx, func(gtx C) D {
					return layout.Flex{
						Axis:    layout.Vertical,
						Spacing: layout.SpaceBetween,
					}.Layout(gtx,
						layout.Flexed(1, func(gtx C) D {
							return layout.UniformInset(15).Layout(gtx, func(gtx C) D {
								return layout.Flex{
									Axis: layout.Vertical,
								}.Layout(gtx,
									layout.Rigid(material.Label(th, 20, "Worlds Saved").Layout),
									layout.Flexed(1, func(gtx C) D {
										p.finishedWorldsMu.Lock()
										defer p.finishedWorldsMu.Unlock()
										return material.List(th, &p.finishedWorldsList).
											Layout(gtx, len(p.finishedWorlds), func(gtx C, index int) D {
												entry := p.finishedWorlds[len(p.finishedWorlds)-index-1]
												return displayWorldEntry(gtx, th, entry)
											})
									}),
								)
							})
						}),
						layout.Rigid(func(gtx C) D {
							gtx.Constraints.Max.Y = gtx.Dp(40)
							gtx.Constraints.Max.X = gtx.Constraints.Max.X / 6
							return material.Button(th, &p.back, "Return").Layout(gtx)
						}),
					)
				})
			default:
				return D{}
			}
		}),
		layout.Stacked(func(gtx C) D {
			p.finishedWorldsMu.Lock()
			defer p.finishedWorldsMu.Unlock()
			return material.List(th, &p.processingWorldsList).
				Layout(gtx, len(p.processingWorlds), func(gtx C, index int) D {
					entry := p.processingWorlds[index]
					return displayWorldProcessing(gtx, th, entry)
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
