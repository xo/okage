// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/turbulence.rs.
//
// Port: the function init of upstream is initTurbulence, because init is a
// special name in Go. A Rust usize stays an int.

// The constants of resvg src/filter/turbulence.rs.
const (
	randM   int32 = 2147483647 // 2**31 - 1
	randA   int32 = 16807      // 7**5; primitive root of m
	randQ   int32 = 127773     // m / a
	randR   int32 = 2836       // m % a
	bSize         = 0x100
	bSize32 int32 = 0x100
	bLen          = bSize + bSize + 2
	bm      int32 = 0xff
	perlinN int32 = 0x1000
)

// stitchInfo is StitchInfo in resvg src/filter/turbulence.rs.
type stitchInfo struct {
	width  int32 // How much to subtract to wrap for stitching.
	height int32
	wrapX  int32 // Minimum value to wrap.
	wrapY  int32
}

// applyFilterTurbulence is apply in resvg src/filter/turbulence.rs. It
// applies a turbulence filter.
//
// `dest` image pixels will have an **unpremultiplied alpha**.
//
//   - `offset_x` and `offset_y` indicate filter region offset.
//   - `sx` and `sy` indicate canvas scale.
func applyFilterTurbulence(
	offsetX float64,
	offsetY float64,
	sx float64,
	sy float64,
	baseFrequencyX float64,
	baseFrequencyY float64,
	numOctaves uint32,
	seed int32,
	stitchTiles bool,
	fractalNoise bool,
	dest imageRefMut,
) {
	latticeSelector, gradient := initTurbulence(seed)
	width := dest.width
	height := dest.height
	var x, y uint32
	for i := 0; i+4 <= len(dest.data); i += 4 {
		pixel := dest.data[i : i+4 : i+4]
		turb := func(channel int) uint8 {
			tx, ty := (float64(x)+offsetX)/sx, (float64(y)+offsetY)/sy
			n := turbulence(
				channel,
				tx,
				ty,
				float64(x),
				float64(y),
				float64(width),
				float64(height),
				baseFrequencyX,
				baseFrequencyY,
				numOctaves,
				fractalNoise,
				stitchTiles,
				latticeSelector,
				gradient,
			)

			if fractalNoise {
				n = (float64(n*255.0) + 255.0) / 2.0
			} else {
				n *= 255.0
			}

			return wide.TruncU8(f32BoundFilter(0.0, float32(n), 255.0) + 0.5)
		}

		pixel[0] = turb(0)
		pixel[1] = turb(1)
		pixel[2] = turb(2)
		pixel[3] = turb(3)

		x++
		if x == dest.width {
			x = 0
			y++
		}
	}
}

// initTurbulence is init in resvg src/filter/turbulence.rs.
func initTurbulence(seed int32) ([]int, [][][2]float64) {
	latticeSelector := make([]int, bLen)
	gradient := make([][][2]float64, 4)
	for k := range gradient {
		gradient[k] = make([][2]float64, bLen)
	}

	if seed <= 0 {
		seed = -seed%(randM-1) + 1
	}

	if seed > randM-1 {
		seed = randM - 1
	}

	for k := range 4 {
		for i := range bSize {
			latticeSelector[i] = i
			for j := range 2 {
				seed = random(seed)
				gradient[k][i][j] = float64((seed%(bSize32+bSize32))-bSize32) / float64(bSize32)
			}

			s := math.Sqrt(float64(gradient[k][i][0]*gradient[k][i][0]) + float64(gradient[k][i][1]*gradient[k][i][1]))

			gradient[k][i][0] /= s
			gradient[k][i][1] /= s
		}
	}

	for i := bSize - 1; i >= 1; i-- {
		k := latticeSelector[i]
		seed = random(seed)
		j := int(seed % bSize32)
		latticeSelector[i] = latticeSelector[j]
		latticeSelector[j] = k
	}

	for i := range bSize + 2 {
		latticeSelector[bSize+i] = latticeSelector[i]
		for _, g := range gradient[:4] {
			for j := range 2 {
				g[bSize+i][j] = g[i][j]
			}
		}
	}

	return latticeSelector, gradient
}

// turbulence is turbulence in resvg src/filter/turbulence.rs.
func turbulence(
	colorChannel int,
	x float64,
	y float64,
	tileX float64,
	tileY float64,
	tileWidth float64,
	tileHeight float64,
	baseFreqX float64,
	baseFreqY float64,
	numOctaves uint32,
	fractalSum bool,
	doStitching bool,
	latticeSelector []int,
	gradient [][][2]float64,
) float64 {
	// Adjust the base frequencies if necessary for stitching.
	var stitch *stitchInfo
	if doStitching {
		// When stitching tiled turbulence, the frequencies must be adjusted
		// so that the tile borders will be continuous.
		if !ApproxZeroUlps64(baseFreqX, 4) {
			loFreq := math.Floor(tileWidth*baseFreqX) / tileWidth
			hiFreq := math.Ceil(tileWidth*baseFreqX) / tileWidth
			if baseFreqX/loFreq < hiFreq/baseFreqX {
				baseFreqX = loFreq
			} else {
				baseFreqX = hiFreq
			}
		}

		if !ApproxZeroUlps64(baseFreqY, 4) {
			loFreq := math.Floor(tileHeight*baseFreqY) / tileHeight
			hiFreq := math.Ceil(tileHeight*baseFreqY) / tileHeight
			if baseFreqY/loFreq < hiFreq/baseFreqY {
				baseFreqY = loFreq
			} else {
				baseFreqY = hiFreq
			}
		}

		// Set up initial stitch values.
		width := wide.Trunc64I32(float64(tileWidth*baseFreqX) + 0.5)
		height := wide.Trunc64I32(float64(tileHeight*baseFreqY) + 0.5)
		wrapX := wide.Trunc64I32(float64(tileX*baseFreqX) + float64(perlinN) + float64(width))
		wrapY := wide.Trunc64I32(float64(tileY*baseFreqY) + float64(perlinN) + float64(height))
		stitch = &stitchInfo{
			width:  width,
			height: height,
			wrapX:  wrapX,
			wrapY:  wrapY,
		}
	}

	sum := 0.0
	x *= baseFreqX
	y *= baseFreqY
	ratio := 1.0
	for range numOctaves {
		if fractalSum {
			sum += noise2(colorChannel, x, y, latticeSelector, gradient, stitch) / ratio
		} else {
			sum += math.Abs(noise2(colorChannel, x, y, latticeSelector, gradient, stitch)) / ratio
		}
		x *= 2.0
		y *= 2.0
		ratio *= 2.0

		if stitch != nil {
			// Update stitch values. Subtracting PerlinN before the multiplication and
			// adding it afterward simplifies to subtracting it once.
			stitch.width *= 2
			stitch.wrapX = 2*stitch.wrapX - perlinN
			stitch.height *= 2
			stitch.wrapY = 2*stitch.wrapY - perlinN
		}
	}

	return sum
}

// noise2 is noise2 in resvg src/filter/turbulence.rs.
//
// Port: upstream takes the stitch info by value as an Option. Go takes a
// pointer, nil is None, and noise2 does not change it.
func noise2(
	colorChannel int,
	x float64,
	y float64,
	latticeSelector []int,
	gradient [][][2]float64,
	stitchInfo *stitchInfo,
) float64 {
	t := x + float64(perlinN)
	bx0 := wide.Trunc64I32(t)
	bx1 := bx0 + 1
	rx0 := t - float64(wide.Trunc64I64(t))
	rx1 := rx0 - 1.0
	t = y + float64(perlinN)
	by0 := wide.Trunc64I32(t)
	by1 := by0 + 1
	ry0 := t - float64(wide.Trunc64I64(t))
	ry1 := ry0 - 1.0

	// If stitching, adjust lattice points accordingly.
	if info := stitchInfo; info != nil {
		if bx0 >= info.wrapX {
			bx0 -= info.width
		}

		if bx1 >= info.wrapX {
			bx1 -= info.width
		}

		if by0 >= info.wrapY {
			by0 -= info.height
		}

		if by1 >= info.wrapY {
			by1 -= info.height
		}
	}

	bx0 &= bm
	bx1 &= bm
	by0 &= bm
	by1 &= bm
	i := latticeSelector[bx0]
	j := latticeSelector[bx1]
	b00 := latticeSelector[i+int(by0)]
	b10 := latticeSelector[j+int(by0)]
	b01 := latticeSelector[i+int(by1)]
	b11 := latticeSelector[j+int(by1)]
	sx := sCurve(rx0)
	sy := sCurve(ry0)
	q := gradient[colorChannel][b00]
	u := float64(rx0*q[0]) + float64(ry0*q[1])
	q = gradient[colorChannel][b10]
	v := float64(rx1*q[0]) + float64(ry0*q[1])
	a := lerp(sx, u, v)
	q = gradient[colorChannel][b01]
	u = float64(rx0*q[0]) + float64(ry1*q[1])
	q = gradient[colorChannel][b11]
	v = float64(rx1*q[0]) + float64(ry1*q[1])
	b := lerp(sx, u, v)
	return lerp(sy, a, b)
}

// random is random in resvg src/filter/turbulence.rs.
func random(seed int32) int32 {
	result := randA*(seed%randQ) - randR*(seed/randQ)
	if result <= 0 {
		result += randM
	}

	return result
}

// sCurve is s_curve in resvg src/filter/turbulence.rs.
func sCurve(t float64) float64 {
	return float64(t*t) * (3.0 - float64(2.0*t))
}

// lerp is lerp in resvg src/filter/turbulence.rs.
func lerp(t, a, b float64) float64 {
	return a + float64(t*(b-a))
}
