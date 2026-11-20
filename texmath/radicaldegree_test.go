// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// The degree of a radical sits inside the height of the radical sign, so
// \sqrt[3]{x} is as tall as \sqrt{x}. Two sources say it:
//
//	\setbox\rootbox\hbox{$\m@th\scriptscriptstyle{#1}$}%   latex.ltx:11187
//	\mkern5mu\raise.6\dimen@\copy\rootbox                  latex.ltx:11192
//
// The degree is raised by .6 of the height less the depth of the radical
// box, and the MATH table has RadicalDegreeBottomRaisePercent, the height of
// the bottom of the degree in proportion to the ascender of the sign.
func TestRadicalDegreeDoesNotRaiseTheBox(t *testing.T) {
	plain := mustLayout(t, `\sqrt{x}`, 20, Display)
	for _, tex := range []string{`\sqrt[3]{x}`, `\sqrt[10]{x}`, `\sqrt[n+1]{x}`} {
		rooted := mustLayout(t, tex, 20, Display)
		if rooted.Height != plain.Height || rooted.Depth != plain.Depth {
			t.Errorf("%s is %v+%v tall where \\sqrt{x} is %v+%v: the degree sits inside the sign",
				tex, rooted.Height, rooted.Depth, plain.Height, plain.Depth)
		}
	}
}

// A wide degree widens the box.
func TestAWideRadicalDegreeWidensTheBox(t *testing.T) {
	narrow := mustLayout(t, `\sqrt[3]{x}`, 20, Display).Width
	wide := mustLayout(t, `\sqrt[10]{x}`, 20, Display).Width
	if wide <= narrow {
		t.Errorf(`\sqrt[10]{x} is %v wide and \sqrt[3]{x} %v: a wider degree pushes the sign right`, wide, narrow)
	}
}

// The degree is set in scriptscript style, the size that latex.ltx names.
// TeX has three sizes, so the size is the text size times
// ScriptScriptPercentScaleDown, not the script size scaled twice.
//
// A digit added to the degree widens the box by the width of that digit at
// the size of the degree, as long as the degree is wide enough that the
// negative RadicalKernAfterDegree does not clamp the position of the sign at
// zero. The source compares with the rounded widths of a digit at the two
// sizes. Here the width is exact: the advance of the glyph of 8 at the
// scriptscript size, from the font.
func TestRadicalDegreeIsSetInScriptScriptStyle(t *testing.T) {
	const size = 20
	face := testFace(t)
	pct := float64(face.MathConstant(font.ScriptScriptPercentScaleDown))
	ss := float64(size*pct) / 100
	if s := scriptSize(face, size); ss >= s {
		t.Fatalf("the scriptscript size %v is not smaller than the script size %v", ss, s)
	}
	added := mustLayout(t, `\sqrt[888]{x}`, size, Display).Width - mustLayout(t, `\sqrt[88]{x}`, size, Display).Width
	w8, _, _ := glyphMetrics(t, face, '8', ss)
	if math.Abs(added-w8) > 1e-9 {
		t.Errorf("a digit of the degree adds %v, want %v, the digit at the scriptscript size %v", added, w8, ss)
	}
	ws, _, _ := glyphMetrics(t, face, '8', scriptSize(face, size))
	if near(added, ws) {
		t.Errorf("a digit of the degree adds %v, the width at the script size", added)
	}
}
