// Copyright 2023 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"testing"
)

// usvg src/writer.rs has no unit tests. These tests check the text of the
// impl Display for f32 of Rust, and write_num.

func nan32() float32 {
	return float32(math.NaN())
}

func TestFmtF32(t *testing.T) {
	for _, c := range []struct {
		v    float32
		want string
	}{
		{0, "0"},
		{float32(math.Copysign(0, -1)), "-0"},
		{1, "1"},
		{-2.5, "-2.5"},
		{0.1, "0.1"},
		{1e-7, "0.0000001"},
		{1e20, "100000000000000000000"},
		{16777217, "16777216"},
		{0.8660254, "0.8660254"},
		{float32(math.Inf(1)), "inf"},
		{float32(math.Inf(-1)), "-inf"},
		{nan32(), "NaN"},
	} {
		if got := fmtF32(c.v); got != c.want {
			t.Errorf("fmtF32(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestWriteNum(t *testing.T) {
	for _, c := range []struct {
		v    float32
		prec uint8
		want string
	}{
		{5, 8, "5"},
		{-0.0000001, 8, "-0.0000001"},
		{1e20, 8, "2147483647"},
		{-1e20, 8, "-2147483648"},
		{1.23456789, 3, "1.235"},
		{-1.0006, 3, "-1.001"},
		{-1.0005, 3, "-1"},
		{12.238575, 8, "12.238575"},
	} {
		if got := string(writeNum(c.v, nil, c.prec)); got != c.want {
			t.Errorf("writeNum(%v, %d) = %q, want %q", c.v, c.prec, got, c.want)
		}
	}
}

func TestWriteOptions(t *testing.T) {
	tree, err := FromStr(parserTestStyleSVG)
	if err != nil {
		t.Fatal(err)
	}
	got := tree.ToString(
		WithIDPrefix("p-"),
		WithUseSingleQuote(true),
		WithIndent(Indent{Kind: IndentNone}),
		WithAttributesIndent(Indent{Kind: IndentNone}),
		WithCoordinatesPrecision(2),
		WithTransformsPrecision(2),
		WithPreserveText(true),
	)
	want := `<svg width='10' height='10' xmlns='http://www.w3.org/2000/svg'><path id='p-r' fill='#ff0000' fill-rule='evenodd' stroke='#0000ff' stroke-opacity='0.5' stroke-miterlimit='2' stroke-linecap='square' stroke-linejoin='round' visibility='hidden' shape-rendering='crispEdges' d='M 0 0 L 5 0 L 5 5 L 0 5 Z'/></svg>`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
