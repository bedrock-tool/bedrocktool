package settings

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"gioui.org/x/outlay"
	"github.com/bedrock-tool/bedrocktool/ui/gui/pages"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
	"github.com/sirupsen/logrus"
)

type settingsPage struct {
	router  *pages.Router
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
				Helper: f.Usage,
			}
			if f.DefValue != "" && f.DefValue != "0" {
				e.Helper = fmt.Sprintf("%s (Default: '%s')", f.Usage, f.DefValue)
			}

			e.SetText(f.DefValue)
			e.SingleLine = true
			if t == "intValue" {
				e.Filter = "0123456789"
				e.InputHint = key.HintNumeric
				if f.DefValue == "0" {
					e.SetText("")
				}
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

func (s *settingsPage) Valid() bool {
	addr, ok := s.widgets["address"].(*addressInput)
	if !ok {
		return true
	}
	if addr.Value() == "" {
		return false
	}
	return true
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
	return material.List(th, &s.list).Layout(gtx, len(s.flags)+2, func(gtx layout.Context, index int) layout.Dimensions {
		if index == 0 { // address input
			w, ok := s.widgets["address"].(*addressInput)
			if ok {
				return w.Layout(gtx, th, s.router)
			}
			return layout.Dimensions{}
		}
		if index == 1 { // bool flags
			var widgets []layout.Widget
			for _, name := range s.flags {
				name := name
				if w, ok := s.widgets[name].(*widget.Bool); ok {
					widgets = append(widgets, func(gtx C) D {
						return s.tooltips[name].Layout(gtx,
							component.PlatformTooltip(th, s.hints[name]),
							material.CheckBox(th, w, name).Layout,
						)
					})
				}
			}

			rows, cols := max(1, len(widgets)/4), min(len(widgets), 4)
			return s.grid.Layout(gtx, rows, cols, func(axis layout.Axis, index, constraint int) int {
				switch axis {
				case layout.Horizontal:
					return gtx.Dp(130)
				case layout.Vertical:
					return gtx.Dp(40)
				}
				return 0
			}, func(gtx layout.Context, row, col int) layout.Dimensions {
				idx := col + cols*row
				return widgets[idx](gtx)
			})
		}

		name := s.flags[index-2]
		w := s.widgets[name]

		switch w := w.(type) {
		case *widget.Bool:
			return layout.Dimensions{}
		case *component.TextField:
			return w.Layout(gtx, th, w.Helper)
		default:
			return layout.Dimensions{}
		}
	})
}
