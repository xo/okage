// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/xo/okage/path"
)

// basicPatternStream draws the stream of basic_pattern_stream of krilla
// tests/src/main.rs on the surface of a StreamBuilder, and finishes the
// surface.
func basicPatternStream(t *testing.T, s *Surface) {
	t.Helper()
	p := rectToPath(t, 0, 0, 10, 10)
	s.SetFill(redFill(t, 1))
	s.DrawPath(p)
	s.PushTransform(path.TransformFromTranslate(10, 10))
	s.SetFill(greenFill(t, 1))
	s.DrawPath(p)
	s.Pop()
	s.Finish()
}

// TestGraphicsScenes draws scenes of gradients, patterns, opacities, blend
// modes and isolated groups on a surface, and compares the SHA-256 hash of
// each file with the hash of the file of krilla. On 2026-11-19, the
// question "krilla" of the oracle in test/krilla gave the files of the
// hashes, for a page of 160 by 160 with the settings {"pretty": true,
// "ascii_compatible": true, "compress_content_streams": false,
// "xmp_metadata": false}. On 2026-11-25, the oracle at the commit
// 63233fb4 of krilla gave the hashes of the five scenes of gradients with
// opacities again, because the mask of a gradient now has a gray
// transparency group. The comment of each scene gives its calls. The
// files are the same byte for byte, with the ID of the document. The test
// module compares the same scenes with the oracle directly, so this test
// keeps them for a machine without cargo.
func TestGraphicsScenes(t *testing.T) {
	id := path.IdentityTransform()
	stops := []Stop{
		{Offset: 0.2, Color: NewColorRGB(255, 0, 0), Opacity: 1},
		{Offset: 0.8, Color: NewColorRGB(255, 255, 0), Opacity: 0.5},
	}
	lumaStops := []Stop{
		{Offset: 0.1, Color: NewColorLuma(0), Opacity: 0.3},
		{Offset: 0.5, Color: NewColorLuma(128), Opacity: 1},
		{Offset: 0.9, Color: NewColorLuma(255), Opacity: 0.7},
	}
	// manyStops has more stops than a PostScript function takes, so
	// trimStops samples them.
	var manyStops []Stop
	for i := range 120 {
		opacity := NormalizedF32(1)
		if i%5 == 0 {
			opacity = 0.5
		}
		manyStops = append(manyStops, Stop{
			Offset:  NormalizedF32(float32(float64(i) / 119)),
			Color:   NewColorRGB(uint8(i*2%256), uint8((255-i)%256), uint8(i*7%256)),
			Opacity: opacity,
		})
	}
	cmykStops := []Stop{
		{Offset: 0.25, Color: NewColorCMYK(0, 50, 100, 0), Opacity: 1},
		{Offset: 0.75, Color: NewColorCMYK(100, 0, 0, 20), Opacity: 1},
	}
	rgbStops := []Stop{
		{Offset: 0, Color: NewColorRGB(255, 0, 0), Opacity: 1},
		{Offset: 0.4, Color: NewColorRGB(0, 255, 0), Opacity: 1},
		{Offset: 1, Color: NewColorRGB(0, 0, 255), Opacity: 1},
	}

	// Each scene fills the rectangle from 20, 20 to 140, 140.
	tests := []struct {
		name  string
		scene func(t *testing.T, s *Surface)
		hash  string
	}{
		{
			// set_fill a linear gradient from 50, 0 to 150, 0 with the spread
			// method repeat and stops, the second with the opacity 0.5.
			"linear repeat",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: LinearGradient{
					X1: 50, X2: 150, Transform: id, SpreadMethod: SpreadMethodRepeat, Stops: stops,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"81a202023b6cf87a89463e71b9b1f96e39f391089050ec7410bca35d4aac2484",
		},
		{
			// The stops of the scene above, from 50, 10 to 150, 110, the
			// spread method reflect and a translation.
			"linear reflect",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: LinearGradient{
					X1: 50, Y1: 10, X2: 150, Y2: 110, Transform: path.TransformFromTranslate(5, 7),
					SpreadMethod: SpreadMethodReflect, Stops: stops,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"90c5428a56149958d1536e2bf9c0deea3ed25dbfe0ab8ca04e570c749bb14c8b",
		},
		{
			// A sweep gradient of luma stops with opacities, anti-aliased.
			"sweep",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: SweepGradient{
					Cx: 80, Cy: 80, StartAngle: 10, EndAngle: 300, Transform: id, Stops: lumaStops, AntiAlias: true,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"99aa05f82b2f511eaad48d4e978007fa238cae14c8bb5cc3629863af13f501f5",
		},
		{
			// A radial gradient of the luma stops, with a scale, and the
			// opacity 0.5 of the fill.
			"radial",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: RadialGradient{
					Fx: 70, Fy: 70, Fr: 5, Cx: 80, Cy: 80, Cr: 60,
					Transform: path.TransformFromRow(2, 0, 0, 1, -50, 0), Stops: lumaStops,
				}, Opacity: 0.5})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"1607a28773d5b2ec7b02f789a2a10a326b4d2d9e8a66dd7f8ed015ce052a1930",
		},
		{
			// The pattern of basic_pattern_stream of krilla tests/src/main.rs,
			// 20 by 20, with the opacity 0.5 of the fill.
			"pattern",
			func(t *testing.T, s *Surface) {
				t.Helper()
				b := s.StreamBuilder()
				basicPatternStream(t, b.Surface())
				s.SetFill(&Fill{Paint: Pattern{
					Stream: b.Finish(), Transform: id, Width: 20, Height: 20,
				}, Opacity: 0.5})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"48cc21a1dbcd04c4d98e7715de83a81dc1a903332506dc7c1ebb7c981e4860cb",
		},
		{
			// push_opacity 0.4, push_blend_mode multiply, a CMYK fill, pop and
			// pop.
			"opacity and blend mode",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.PushOpacity(0.4)
				s.PushBlendMode(BlendModeMultiply)
				s.SetFill(&Fill{Paint: NewColorCMYK(10, 20, 30, 40), Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
				s.Pop()
				s.Pop()
			},
			"9ba22922bc946dc5e1b41eb93fb3ae526b31e7c114f5306d7f55cb2e8ddd87e4",
		},
		{
			// push_isolated, a luma fill with the opacity 0.25, and pop.
			"isolated",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.PushIsolated()
				s.SetFill(&Fill{Paint: NewColorLuma(40), Opacity: 0.25})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
				s.Pop()
			},
			"c56a9b700404a3cfd49515df8f501792dd35a7df0c58cec525471c3c588772a4",
		},
		{
			// A sweep gradient with the spread method repeat and the 120
			// stops of manyStops, each fifth with the opacity 0.5.
			"sweep with many stops",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: SweepGradient{
					Cx: 80, Cy: 80, StartAngle: 0, EndAngle: 120, Transform: id,
					SpreadMethod: SpreadMethodRepeat, Stops: manyStops,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"59d88ffb3b175af10b3a7394ab2c30de14a86079c81aeb167523ea1879f28bb1",
		},
		{
			// A linear gradient of two CMYK stops, padded at 0 and 1.
			"linear cmyk",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.SetFill(&Fill{Paint: LinearGradient{
					X1: 20, Y1: 20, X2: 140, Y2: 100, Transform: id, Stops: cmykStops,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
			},
			"5c582ebcd86ad1c18896de088d20bf0976617d037e1ba6b73eef389a8b325994",
		},
		{
			// push_transform [0.5 0.2 -0.2 0.5 30 10], a radial gradient of
			// three RGB stops, and pop.
			"radial transformed",
			func(t *testing.T, s *Surface) {
				t.Helper()
				s.PushTransform(path.TransformFromRow(0.5, 0.2, -0.2, 0.5, 30, 10))
				s.SetFill(&Fill{Paint: RadialGradient{
					Fx: 80, Fy: 80, Fr: 0, Cx: 80, Cy: 80, Cr: 60, Transform: id, Stops: rgbStops,
				}, Opacity: 1})
				s.DrawPath(rectToPath(t, 20, 20, 140, 140))
				s.Pop()
			},
			"b1fa57becb085bf0b47dd65848579a5dad66a885300c2f529ce94429e529650e",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDocument(WithPretty(true), WithASCIICompatible(true), WithCompression(false))
			ps, _ := PageSettingsFromWH(160, 160)
			s := d.StartPage(ps).Surface()
			tt.scene(t, s)
			var buf bytes.Buffer
			if err := d.Finish(&buf); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(buf.Bytes())
			if got := hex.EncodeToString(sum[:]); got != tt.hash {
				t.Errorf("the hash is %s, want %s", got, tt.hash)
			}
		})
	}
}
