package utils

import (
	"image"
	"image/color"
	"reflect"
	"unsafe"
)

func Img2rgba(img *image.RGBA) []color.RGBA {
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&img.Pix))
	header.Len /= 4
	header.Cap /= 4
	return *(*[]color.RGBA)(unsafe.Pointer(&header))
}

// LERP is a linear interpolation function
func LERP(p1, p2, alpha float64) float64 {
	return (1-alpha)*p1 + alpha*p2
}

func blendColorValue(c1, c2, a uint8) uint8 {
	return uint8(LERP(float64(c1), float64(c2), float64(a)/float64(0xff)))
}

func blendAlphaValue(a1, a2 uint8) uint8 {
	return uint8(LERP(float64(a1), float64(0xff), float64(a2)/float64(0xff)))
}

func BlendColors(c1, c2 color.RGBA) (ret color.RGBA) {
	ret.R = blendColorValue(c1.R, c2.R, c2.A)
	ret.G = blendColorValue(c1.G, c2.G, c2.A)
	ret.B = blendColorValue(c1.B, c2.B, c2.A)
	ret.A = blendAlphaValue(c1.A, c2.A)
	return ret
}

// Draw_img_scaled_pos draws src onto dst at bottom_left, scaled to size
func Draw_img_scaled_pos(dst *image.RGBA, src *image.RGBA, bottom_left image.Point, size_scaled int) {
	sbx := src.Bounds().Dx()
	ratio := int(float64(sbx) / float64(size_scaled))

	for x_out := bottom_left.X; x_out < bottom_left.X+size_scaled; x_out++ {
		for y_out := bottom_left.Y; y_out < bottom_left.Y+size_scaled; y_out++ {
			x_in := (x_out - bottom_left.X) * ratio
			y_in := (y_out - bottom_left.Y) * ratio
			c := src.At(x_in, y_in)
			dst.Set(x_out, y_out, c)
		}
	}
}
