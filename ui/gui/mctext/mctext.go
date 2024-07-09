package mctext

import (
	"image/color"
	"regexp"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/styledtext"
)

var splitter = regexp.MustCompile("((?:§.)?(?:[^§]+)?)")

func Label(th *material.Theme, size unit.Sp, txt string) func(gtx layout.Context) layout.Dimensions {
	split := splitter.FindAllString(txt, -1)
	var Styles []styledtext.SpanStyle

	var activeColor color.NRGBA = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	var bold bool
	var italic bool
	var obfuscated bool

	for _, part := range split {
		if len(part) == 0 {
			continue
		}
		partR := []rune(part)
		switch partR[1] {
		case '0':
			activeColor = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
		case '1':
			activeColor = color.NRGBA{R: 0x00, G: 0x00, B: 0xAA, A: 0xff}
		case '2':
			activeColor = color.NRGBA{R: 0x00, G: 0xAA, B: 0x00, A: 0xff}
		case '3':
			activeColor = color.NRGBA{R: 0x00, G: 0xAA, B: 0xAA, A: 0xff}
		case '4':
			activeColor = color.NRGBA{R: 0xAA, G: 0x00, B: 0x00, A: 0xff}
		case '5':
			activeColor = color.NRGBA{R: 0xAA, G: 0x00, B: 0xAA, A: 0xff}
		case '6':
			activeColor = color.NRGBA{R: 0xFF, G: 0xAA, B: 0x00, A: 0xff}
		case '7':
			activeColor = color.NRGBA{R: 0xc6, G: 0xc6, B: 0xc6, A: 0xff}
		case '8':
			activeColor = color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}
		case '9':
			activeColor = color.NRGBA{R: 0x55, G: 0x55, B: 0xff, A: 0xff}
		case 'a':
			activeColor = color.NRGBA{R: 0x55, G: 0xff, B: 0x55, A: 0xff}
		case 'b':
			activeColor = color.NRGBA{R: 0x55, G: 0xff, B: 0xff, A: 0xff}
		case 'c':
			activeColor = color.NRGBA{R: 0xff, G: 0x55, B: 0x55, A: 0xff}
		case 'd':
			activeColor = color.NRGBA{R: 0xff, G: 0x55, B: 0xff, A: 0xff}
		case 'e':
			activeColor = color.NRGBA{R: 0xff, G: 0xff, B: 0x55, A: 0xff}
		case 'f':
			activeColor = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		case 'g':
			activeColor = color.NRGBA{R: 0xDD, G: 0xD6, B: 0x05, A: 0xff}
		case 'h':
			activeColor = color.NRGBA{R: 0xE3, G: 0xD4, B: 0xD1, A: 0xff}
		case 'i':
			activeColor = color.NRGBA{R: 0xCE, G: 0xCA, B: 0xCA, A: 0xff}
		case 'j':
			activeColor = color.NRGBA{R: 0x44, G: 0x3A, B: 0x3B, A: 0xff}
		case 'm':
			activeColor = color.NRGBA{R: 0x97, G: 0x16, B: 0x07, A: 0xff}
		case 'n':
			activeColor = color.NRGBA{R: 0xB4, G: 0x68, B: 0x4D, A: 0xff}
		case 'p':
			activeColor = color.NRGBA{R: 0xDE, G: 0xB1, B: 0x2D, A: 0xff}
		case 'q':
			activeColor = color.NRGBA{R: 0x97, G: 0xA0, B: 0x36, A: 0xff}
		case 's':
			activeColor = color.NRGBA{R: 0x2C, G: 0xBA, B: 0xA8, A: 0xff}
		case 't':
			activeColor = color.NRGBA{R: 0x21, G: 0x49, B: 0x7B, A: 0xff}
		case 'u':
			activeColor = color.NRGBA{R: 0x21, G: 0x49, B: 0x7B, A: 0xff}
		case 'r':
			activeColor = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
			bold, italic, obfuscated = false, false, false
		case 'l':
			bold = true
		case 'o':
			italic = true
		case 'k':
			obfuscated = true
		}
		_ = obfuscated

		partT := string(partR[2:])

		if len(partT) == 0 {
			continue
		}

		var fontStyle font.Style = font.Regular
		if italic {
			fontStyle = font.Italic
		}
		var fontWeight font.Weight = font.Normal
		if bold {
			fontWeight = font.Bold
		}

		Styles = append(Styles, styledtext.SpanStyle{
			Font: font.Font{
				Typeface: th.Face,
				Style:    fontStyle,
				Weight:   fontWeight,
			},
			Size:    size,
			Color:   activeColor,
			Content: partT,
		})
	}

	return func(gtx layout.Context) layout.Dimensions {
		return styledtext.TextStyle{
			Styles: Styles,
			Shaper: th.Shaper,
		}.Layout(gtx, nil)
	}
}
