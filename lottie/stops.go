// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file holds the stops of a gradient with opacity stops (D43). It ports
// no velato file: it replaces conv_stops of velato src/import/converters.rs.

import "sort"

// gradientStops returns the stops of the raw gradient data of a Lottie file:
// count color stops of four numbers each (the offset, red, green and blue),
// then any number of opacity stops of two numbers each (the offset and the
// alpha).
//
// Port: velato reads the alpha only at the offsets of the color stops, with
// a hack. okage makes a stop at each offset of a color stop and of an opacity
// stop, with the color and the alpha that each list gives there, as ThorVG
// does (D43).
func gradientStops(raw []float64, count int) ColorStops {
	type colorStop struct{ offset, r, g, b float64 }
	type alphaStop struct{ offset, a float64 }
	var colors []colorStop
	for i := 0; i < count && 4*i+3 < len(raw); i++ {
		colors = append(colors, colorStop{raw[4*i], raw[4*i+1], raw[4*i+2], raw[4*i+3]})
	}
	var alphas []alphaStop
	for j := 4 * count; j+1 < len(raw); j += 2 {
		alphas = append(alphas, alphaStop{raw[j], raw[j+1]})
	}
	if len(colors) == 0 {
		return nil
	}
	colorAt := func(x float64) (float64, float64, float64) {
		if x <= colors[0].offset {
			return colors[0].r, colors[0].g, colors[0].b
		}
		for i := 1; i < len(colors); i++ {
			a, b := colors[i-1], colors[i]
			if x <= b.offset {
				t := normalizeToRange(a.offset, b.offset, x)
				return a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t
			}
		}
		last := colors[len(colors)-1]
		return last.r, last.g, last.b
	}
	alphaAt := func(x float64) float64 {
		if len(alphas) == 0 {
			return 1.0
		}
		if x <= alphas[0].offset {
			return alphas[0].a
		}
		for i := 1; i < len(alphas); i++ {
			a, b := alphas[i-1], alphas[i]
			if x <= b.offset {
				return a.a + (b.a-a.a)*normalizeToRange(a.offset, b.offset, x)
			}
		}
		return alphas[len(alphas)-1].a
	}
	offsets := make([]float64, 0, len(colors)+len(alphas))
	for _, c := range colors {
		offsets = append(offsets, c.offset)
	}
	for _, a := range alphas {
		offsets = append(offsets, a.offset)
	}
	sort.Float64s(offsets)
	stops := make(ColorStops, 0, len(offsets))
	for i, x := range offsets {
		if i > 0 && x == offsets[i-1] {
			continue
		}
		r, g, b := colorAt(x)
		stops = append(stops, ColorStop{
			Offset: float32(x),
			Color:  Color{float32(r), float32(g), float32(b), float32(alphaAt(x))},
		})
	}
	return stops
}
