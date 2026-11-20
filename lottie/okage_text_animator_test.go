// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"fmt"
	"math"
	"testing"

	"github.com/xo/okage/kurbo"
)

// animatedTextFile returns a Lottie file of 100 by 100 pixels with the
// font "Bars", a text layer at (x, y) with the document doc, the extra keys
// extra of its key t, and the masks masks.
func animatedTextFile(x, y int, doc, extra, masks string) []byte {
	return fmt.Appendf(nil, `{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "fonts": %s, "chars": %s, "layers": [
		{"ty": 5, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [%d, %d]}}, "masksProperties": [%s],
			"t": {"d": {"k": [{"t": 0, "s": %s}]}%s}}
	]}`, barFonts, barChars, x, y, masks, doc, extra)
}

// TestTextAnimatorDeserialize checks the schema and the conversion of the
// text animators, the grouping and the path of a text layer.
func TestTextAnimatorDeserialize(t *testing.T) {
	c, err := Parse(animatedTextFile(0, 0, `{"t": "I", "f": "Bars-Regular", "s": 50}`, `,
		"a": [{"s": {"a": {"a": 0, "k": 80}, "b": 3, "sh": 4, "r": 2, "s": {"a": 0, "k": 1}, "e": {"a": 0, "k": 3}},
			"a": {"p": {"a": 0, "k": [5, 6, 0]}, "s": {"a": 0, "k": [150, 50]}, "fc": {"a": 0, "k": [1, 0, 0]}, "sw": {"a": 0, "k": 2}, "t": {"a": 0, "k": 4}}},
			{"s": {}, "a": {}}],
		"m": {"g": 2, "a": {"a": 0, "k": [7, 8]}},
		"p": {"m": 0, "f": {"a": 0, "k": 9}}`, ""))
	if err != nil {
		t.Fatal(err)
	}
	anim := c.Layers[0].Content.TextAnimation
	if anim == nil || len(anim.Animators) != 2 || anim.Group != TextGroupWords || anim.Anchor.Evaluate(0) != kurbo.NewPoint(7, 8) ||
		anim.Path == nil || anim.Path.Mask != 0 || anim.Path.FirstMargin.Evaluate(0) != 9 {
		t.Fatalf("got %+v", anim)
	}
	a := &anim.Animators[0]
	if a.Amount.Evaluate(0) != 80 || a.Based != TextBasedWords || a.Shape != TextShapeTriangle || !a.Index ||
		a.Start.Evaluate(0) != 1 || a.End.Evaluate(0) != 3 || a.Position.Evaluate(0) != kurbo.NewPoint(5, 6) ||
		a.Scale.Evaluate(0) != (kurbo.Vec2{X: 150, Y: 50}) || a.FillColor == nil || a.StrokeColor != nil ||
		a.StrokeWidth == nil || a.Tracking.Evaluate(0) != 4 || a.Opacity.Evaluate(0) != 100 {
		t.Errorf("got %+v", a)
	}
	// An animator without keys has the defaults of ThorVG, with the amount
	// 0.
	b := &anim.Animators[1]
	if b.Amount.Evaluate(0) != 0 || b.End.Evaluate(0) != 100 || b.Smoothness.Evaluate(0) != 100 ||
		b.Based != TextBasedCharacters || b.Shape != TextShapeSquare || b.Index {
		t.Errorf("got %+v", b)
	}
	// A layer with an empty path and no animators has no text animation.
	c, err = Parse(animatedTextFile(0, 0, `{"t": "I", "f": "Bars-Regular", "s": 50}`, `, "a": [], "p": {}`, ""))
	if err != nil {
		t.Fatal(err)
	}
	if c.Layers[0].Content.TextAnimation != nil {
		t.Errorf("got %+v", c.Layers[0].Content.TextAnimation)
	}
}

// TestTextSelectorFactor checks the share of a glyph in the range of each
// shape, for a text of 10 bytes and the range from 20 to 60 percent, which
// is the range of the glyphs 2 to 6.
func TestTextSelectorFactor(t *testing.T) {
	for _, c := range []struct {
		shape      TextSelectorShape
		smoothness float64
		idx        float32
		want       float32
	}{
		{TextShapeSquare, 0, 1, 0},
		{TextShapeSquare, 0, 2, 1},
		{TextShapeSquare, 0, 5, 1},
		{TextShapeSquare, 0, 6, 0},
		{TextShapeSquare, 100, 1, 0},
		{TextShapeSquare, 100, 5, 1},
		{TextShapeSquare, 100, 6, 0},
		{TextShapeSquare, 50, 5, 1},
		{TextShapeRampUp, 100, 2, 0.125},
		{TextShapeRampUp, 100, 4, 0.625},
		{TextShapeRampDown, 100, 2, 0.875},
		{TextShapeTriangle, 100, 3, 0.75},
		{TextShapeTriangle, 100, 5, 0.25},
		{TextShapeRound, 100, 3, float32(math.Sqrt(1 - 0.0625))},
		{TextShapeSmooth, 100, 3, float32(0.5 * (1 + math.Cos(1.75*math.Pi)))},
	} {
		a := TextAnimator{
			Start: fixedValue(20.0), End: fixedValue(60.0), Offset: fixedValue(0.0),
			Amount: fixedValue(100.0), Smoothness: fixedValue(c.smoothness),
			MinEase: fixedValue(0.0), MaxEase: fixedValue(0.0), Shape: c.shape,
		}
		if got := a.factor(0, 10, c.idx); math.Abs(float64(got-c.want)) > 1e-5 {
			t.Errorf("shape %d, smoothness %v, glyph %v: got %v, want %v", c.shape, c.smoothness, c.idx, got, c.want)
		}
		// The units of glyphs give the same range, and the amount 50 half
		// the share.
		a.Index, a.Start, a.End, a.Amount = true, fixedValue(2.0), fixedValue(6.0), fixedValue(50.0)
		if got := a.factor(0, 10, c.idx); math.Abs(float64(got-c.want/2)) > 1e-5 {
			t.Errorf("shape %d, smoothness %v, glyph %v, in glyphs: got %v, want %v", c.shape, c.smoothness, c.idx, got, c.want/2)
		}
	}
}

// TestTextAnimatorStyle checks an animator that selects the first of two
// glyphs: it moves the glyph up, makes it red and half transparent, and
// adds its tracking before the glyph.
func TestTextAnimatorStyle(t *testing.T) {
	data := animatedTextFile(0, 50, `{"t": "II", "f": "Bars-Regular", "s": 50, "fc": [0, 0, 1]}`, `,
		"a": [{"s": {"a": {"a": 0, "k": 100}, "r": 2, "s": {"a": 0, "k": 0}, "e": {"a": 0, "k": 1}},
			"a": {"p": {"a": 0, "k": [0, -20]}, "fc": {"a": 0, "k": [1, 0, 0]}, "o": {"a": 0, "k": 50}, "t": {"a": 0, "k": 20}}}]`, "")
	pm := render(t, data, 0)
	// At the size 50, the first bar is 10 wide and 35 high. The tracking
	// 20 moves it 10 to the right, to x = 15, and it goes from y = -5 to
	// 30. The second bar follows it at x = 35, from y = 15 to 50.
	if p := pixel(t, pm, 20, 40); p[3] != 0 {
		t.Errorf("the pixel below the first bar is %v", p)
	}
	if p := pixel(t, pm, 20, 10); p[3] < 126 || p[3] > 128 || p[0] != 255 || p[2] != 0 {
		t.Errorf("the first bar is %v, want red at half the opacity", p)
	}
	if p := pixel(t, pm, 40, 40); p != [4]uint8{0, 0, 255, 255} {
		t.Errorf("the second bar is %v, want blue", p)
	}
}

// TestTextCaps checks the capitals of the key ca of a text document: a
// small letter draws the glyph of its capital, at 70 percent of the size
// for small capitals, and draws nothing without capitals, because the list
// chars has no small letter.
func TestTextCaps(t *testing.T) {
	for _, c := range []struct {
		caps          int
		top, low, out bool
	}{
		{0, false, false, false},
		{1, true, true, true},
		{2, false, true, false},
	} {
		pm := render(t, textFile(barFonts, barChars, 0, 80, fmt.Sprintf(`{"t": "i", "f": "Bars-Regular", "s": 100, "fc": [0, 0, 1], "ca": %d}`, c.caps)), 0)
		// The capital goes from x = 10 to 30 and from y = 10 to 80. The
		// small capital goes from x = 7 to 21 and from y = 31 to 80.
		checkDrawn(t, pm, map[[2]uint32]bool{{14, 25}: c.top, {14, 40}: c.low, {25, 70}: c.out})
	}
}

// TestTextFollow checks the points of the path of a text: on the path,
// before its start and after its end, for an open and a closed path.
func TestTextFollow(t *testing.T) {
	line := newTextFollow([]kurbo.PathEl{kurbo.MoveTo(kurbo.NewPoint(0, 0)), kurbo.LineTo(kurbo.NewPoint(200, 0))}, 2)
	square := newTextFollow([]kurbo.PathEl{
		kurbo.MoveTo(kurbo.NewPoint(0, 0)), kurbo.LineTo(kurbo.NewPoint(100, 0)), kurbo.LineTo(kurbo.NewPoint(100, 100)),
		kurbo.LineTo(kurbo.NewPoint(0, 100)), kurbo.ClosePath(),
	}, 1)
	for _, c := range []struct {
		f    *textFollow
		at   float64
		want kurbo.Point
	}{
		{line, 50, kurbo.NewPoint(50, 0)},
		{line, -10, kurbo.NewPoint(-10, 0)},
		{line, 120, kurbo.NewPoint(120, 0)},
		{square, 150, kurbo.NewPoint(100, 50)},
		{square, -50, kurbo.NewPoint(0, 50)},
		{square, 450, kurbo.NewPoint(50, 0)},
	} {
		if got := c.f.position(c.at); got.Distance(c.want) > 0.1 {
			t.Errorf("at %v: got %v, want %v", c.at, got, c.want)
		}
	}
	if line.total != 100 || square.total != 400 {
		t.Errorf("the lengths are %v and %v", line.total, square.total)
	}
}
