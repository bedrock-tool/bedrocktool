package settings

import (
	"errors"
	"flag"
	"reflect"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type settingsPage struct {
	cmd     commands.Command
	f       *flag.FlagSet
	widgets map[string]any

	grid outlay.Grid
	list widget.List

	tooltips map[string]*component.TipArea
	hints    map[string]string
	flags    []string
	setters  map[string]func(string) error
}

func (s *settingsPage) Init() {
	s.list.Axis = layout.Vertical
	s.widgets = make(map[string]any)
	s.tooltips = make(map[string]*component.TipArea)
	s.hints = make(map[string]string)
	s.setters = make(map[string]func(string) error)
	s.f = flag.NewFlagSet("", flag.ContinueOnError)
	s.cmd.SetFlags(s.f)
	hasAddress := false

	visitFunc := func(f *flag.Flag) {
		if f.Name == "v" {
			f.Name = "verbose"
		}

		t := reflect.ValueOf(f.Value).Type().String()
		t = strings.Split(t, ".")[1]
		switch t {
		case "stringValue", "intValue":
			e := &component.TextField{
				Helper: f.Name,
			}
			e.SetText(f.DefValue)
			e.SingleLine = true
			if t == "intValue" {
				e.Filter = "0123456789"
				e.InputHint = key.HintNumeric
			}
			s.widgets[f.Name] = e
		case "boolValue":
			e := &widget.Bool{}
			if f.DefValue == "true" {
				e.Value = true
			}
			s.widgets[f.Name] = e
		default:
			logrus.Warnf("%s unknown flag type", t)
		}
		s.tooltips[f.Name] = &component.TipArea{}
		s.hints[f.Name] = f.Usage
		s.setters[f.Name] = f.Value.Set

		if f.Name == "address" {
			s.widgets[f.Name] = AddressInput
			hasAddress = true
		} else {
			s.flags = append(s.flags, f.Name)
		}
	}

	s.f.VisitAll(visitFunc)

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if f.Name == "debug" || f.Name == "capture" {
			visitFunc(f)
		}
	})

	if hasAddress {
		s.flags = append([]string{"address"}, s.flags...)
	}
}

func (s *settingsPage) Apply() error {
	for k, set := range s.setters {
		w := s.widgets[k]
		switch w := w.(type) {
		case *widget.Bool:
			v := "false"
			if w.Value {
				v = "true"
			}
			if err := set(v); err != nil {
				return err
			}
		case *component.TextField:
			if err := set(w.Text()); err != nil {
				return err
			}
		case *addressInput:
			if err := set(w.Value()); err != nil {
				return err
			}
		default:
			return errors.New("unknown widget " + k)
		}
	}
	return nil
}

func (s *settingsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{
		Axis: layout.Vertical,
	}.Layout(gtx,

		// address input
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			w, ok := s.widgets["address"].(*addressInput)
			if ok {
				return layout.Flex{
					Axis: layout.Vertical,
				}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return w.Layout(gtx, th)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return w.LayoutRealms(gtx, th)
					}),
				)
			}
			return layout.Dimensions{}
		}),

		// bool flags
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var widgets []layout.Widget
			for _, name := range s.flags {
				name := name
				if w, ok := s.widgets[name].(*widget.Bool); ok {
					widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
						return s.tooltips[name].Layout(gtx,
							component.PlatformTooltip(th, s.hints[name]),
							material.CheckBox(th, w, name).Layout,
						)
					})
				}
			}

			return s.grid.Layout(gtx, 2, 3, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return 130
				case layout.Vertical:
					return 40
				}
				return 0
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				idx := col + 3*row
				if idx < len(widgets) {
					return widgets[idx](gtx)
				}
				return layout.Dimensions{}
			})
		}),

		// text flags
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &s.list).Layout(gtx, len(s.flags), func(gtx layout.Context, index int) layout.Dimensions {
				name := s.flags[index]
				w := s.widgets[name]

				switch w := w.(type) {
				case *widget.Bool:
					return layout.Dimensions{}
				case *component.TextField:
					return w.Layout(gtx, th, s.hints[name])
				default:
					return layout.Dimensions{}
				}
			})
		}))
}
