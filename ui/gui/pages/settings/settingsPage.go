package settings

import (
	"errors"
	"fmt"
	"image"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/ui/gui/guim"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type settingsArg struct {
	widget  any
	tooltip component.TipArea
	usage   string
}

type settingsPage struct {
	g        guim.Guim
	cmd      commands.Command
	settings any
	args     []commands.Arg
	widgets  map[string]*settingsArg

	flagsNamesOrdered []string

	grid outlay.Grid
	list widget.List
}

func (s *settingsPage) Init() {
	s.list.Axis = layout.Vertical
	s.list.Alignment = layout.Middle
	s.widgets = make(map[string]*settingsArg)
	s.settings = s.cmd.Settings()

	args, err := commands.ParseArgsType(reflect.ValueOf(s.settings), nil, nil)
	if err != nil {
		logrus.Fatal(err)
	}
	s.args = args

	hasAddress := false
	for _, arg := range args {
		extType := strings.Split(arg.ExtType, ",")
		if len(extType) > 0 && extType[0] == "file" {
			var ext string
			if len(extType) > 1 {
				ext = extType[1]
			}
			s.widgets[arg.Flag] = &settingsArg{
				widget: &fileInputWidget{g: s.g, textField: component.TextField{
					Editor: widget.Editor{
						SingleLine: true,
					},
				}, Hint: arg.Name, Ext: ext},
			}
			s.flagsNamesOrdered = append(s.flagsNamesOrdered, arg.Flag)
			continue
		}
		switch arg.Type {
		case "connectInfo":
			s.widgets[arg.Flag] = &settingsArg{
				widget: AddressInput,
				usage:  "target server address",
			}
			hasAddress = true
			continue

		case "bool":
			e := &widget.Bool{}
			e.Value = arg.Default == "true"
			s.widgets[arg.Flag] = &settingsArg{widget: e, usage: arg.Desc}

		default:
			e := &component.TextField{
				Helper: arg.Name,
				Editor: widget.Editor{
					SingleLine: true,
				},
			}
			e.SetText(arg.Default)
			s.widgets[arg.Flag] = &settingsArg{widget: e, usage: arg.Desc}
		}
		s.flagsNamesOrdered = append(s.flagsNamesOrdered, arg.Flag)
	}
	if hasAddress {
		s.flagsNamesOrdered = append([]string{"address"}, s.flagsNamesOrdered...)
	}
}

func (s *settingsPage) Valid() bool {
	addr, ok := s.widgets["address"].widget.(*addressInput)
	if !ok {
		return true
	}
	if addr.GetConnectInfo() == nil {
		return false
	}
	return true
}

type settingError struct {
	name string
	err  error
}

func (e settingError) Error() string {
	return fmt.Sprintf("%s: %s", e.name, e.err)
}

func (s *settingsPage) Apply() (any, error) {
	for _, arg := range s.args {
		flagWidget := s.widgets[arg.Flag]
		var value string
		switch flagWidget := flagWidget.widget.(type) {
		case *widget.Bool:
			value = "false"
			if flagWidget.Value {
				value = "true"
			}

		case *component.TextField:
			value = flagWidget.Text()

		case *fileInputWidget:
			value = flagWidget.textField.Text()

		case *addressInput:
			arg.SetConnectInfo(flagWidget.GetConnectInfo())
			continue

		default:
			return nil, errors.New("unknown widget " + arg.Name)
		}
		if arg.Type == "int" && value == "" {
			value = arg.Default
			if value == "" {
				value = "0"
			}
		}
		err := arg.Set(value)
		if err != nil {
			return nil, settingError{name: arg.Name, err: err}
		}
	}
	return s.settings, nil
}

func textFieldIsActive(w *component.TextField) bool {
	return *(*uint8)(unsafe.Pointer(reflect.ValueOf(w).Elem().FieldByName("state").UnsafeAddr())) == 3
}

func (s *settingsPage) PopupActive() bool {
	for _, name := range s.flagsNamesOrdered {
		w := s.widgets[name]
		switch w := w.widget.(type) {
		case *widget.Bool:
			continue
		case *component.TextField:
			if textFieldIsActive(w) {
				return true
			}
		case *fileInputWidget:
			if textFieldIsActive(&w.textField) {
				return true
			}
		}
	}
	return false
}

func (s *settingsPage) LayoutPopupInput(gtx C, th *material.Theme) D {
	cornerRadius := 8
	return layout.UniformInset(8).Layout(gtx, func(gtx C) D {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
				toastRect := clip.RRect{
					Rect: image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y+cornerRadius),
					SE:   int(cornerRadius),
					SW:   int(cornerRadius),
					NW:   int(cornerRadius),
					NE:   int(cornerRadius),
				}.Push(gtx.Ops)
				paint.ColorOp{Color: th.Bg}.Add(gtx.Ops)
				paint.PaintOp{}.Add(gtx.Ops)
				toastRect.Pop()
				return D{Size: gtx.Constraints.Max}
			}),
			layout.Stacked(func(gtx C) D {
				return layout.Inset{
					Left: unit.Dp(cornerRadius),
				}.Layout(gtx, func(gtx C) D {
					gtx.Constraints.Max.X -= cornerRadius
					for _, name := range s.flagsNamesOrdered {
						w := s.widgets[name]
						switch w := w.widget.(type) {
						case *widget.Bool:
							continue
						case *component.TextField:
							if textFieldIsActive(w) {
								w.Update(gtx, th, w.Helper)
								return w.Layout(gtx, th, w.Helper)
							}
						case *fileInputWidget:
							if textFieldIsActive(&w.textField) {
								return w.Layout(gtx, th)
							}
						}
					}
					return D{}
				})
			}),
		)
	})
}

func (s *settingsPage) Layout(gtx C, th *material.Theme) D {
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
	}.Layout(gtx,
		layout.Flexed(1, layout.Spacer{Width: 1000}.Layout),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(138*4))
			return material.List(th, &s.list).Layout(gtx, len(s.flagsNamesOrdered)+2, func(gtx C, index int) D {
				switch index {
				case 0: // address
					w, ok := s.widgets["address"].widget.(*addressInput)
					if ok {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return w.Layout(gtx, th)
					}
					return D{}

				case 1: // checkboxes
					var widgets []layout.Widget
					for _, name := range s.flagsNamesOrdered {
						w := s.widgets[name]
						if ww, ok := w.widget.(*widget.Bool); ok {
							widgets = append(widgets, func(gtx C) D {
								return w.tooltip.Layout(gtx,
									component.PlatformTooltip(th, w.usage),
									material.CheckBox(th, ww, name).Layout,
								)
							})
						}
					}

					boxWidth := gtx.Dp(130)
					rows, cols := max(1, int(math.Ceil(float64(len(widgets))/4))), min(len(widgets), 4)
					if cols*boxWidth > gtx.Constraints.Max.X {
						cols = max(gtx.Constraints.Max.X/boxWidth, 1)
						rows = len(widgets) / cols
					}

					return s.grid.Layout(gtx, rows, cols, func(axis layout.Axis, index, constraint int) int {
						switch axis {
						case layout.Horizontal:
							return boxWidth
						case layout.Vertical:
							return gtx.Dp(40)
						}
						return 0
					}, func(gtx C, row, col int) D {
						idx := col + cols*row
						if idx >= len(widgets) {
							return D{}
						}
						return widgets[idx](gtx)
					})

				default:
					name := s.flagsNamesOrdered[index-2]
					w := s.widgets[name]
					switch w := w.widget.(type) {
					case *widget.Bool:
						return D{}
					case *component.TextField:
						w.Update(gtx, th, w.Helper)
						return w.Layout(gtx, th, w.Helper)
					case *fileInputWidget:
						return w.Layout(gtx, th)
					default:
						return D{}
					}
				}
			})
		}),
		layout.Flexed(1, layout.Spacer{Width: 1000}.Layout),
	)
}
