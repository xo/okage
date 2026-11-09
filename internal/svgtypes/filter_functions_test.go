// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/filter_functions.rs.

// filterNext checks that the next value of p is want.
func filterNext(t *testing.T, p *FilterValueListParser, want FilterValue) {
	t.Helper()
	got, ok, err := p.Next()
	if !ok || err != nil || got != want {
		t.Errorf("got %+v, %v, %v, want %+v", got, ok, err, want)
	}
}

// filterNextErr checks that the next result of p is an error with the
// message want.
func filterNextErr(t *testing.T, p *FilterValueListParser, want string) {
	t.Helper()
	_, ok, err := p.Next()
	if !ok {
		t.Errorf("got the end, want %q", want)
		return
	}
	errText(t, err, want)
}

// filterEnd checks that p is at the end.
func filterEnd(t *testing.T, p *FilterValueListParser) {
	t.Helper()
	if v, ok, err := p.Next(); ok {
		t.Errorf("got %+v, %v, want the end", v, err)
	}
}

func dropShadow(color Color, hasColor bool, dx, dy, stdDev Length) FilterValue {
	return FilterValue{Kind: FilterValueDropShadow, Color: color, HasColor: hasColor, Dx: dx, Dy: dy, StdDev: stdDev}
}

func TestFilterEmpty(t *testing.T) {
	parser := NewFilterValueListParser("")
	filterEnd(t, parser)
}

func TestFilterNone(t *testing.T) {
	parser := NewFilterValueListParser("none")
	filterEnd(t, parser)
}

func TestFilterBlurDefault(t *testing.T) {
	parser := NewFilterValueListParser("blur()")
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: LengthZero()})
	filterEnd(t, parser)
}

func TestFilterBlur2(t *testing.T) {
	parser := NewFilterValueListParser("blur(2)")
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: NewLength(2.0, LengthUnitNone)})
	filterEnd(t, parser)
}

func TestFilterBlur2mm(t *testing.T) {
	parser := NewFilterValueListParser("blur(2mm)")
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: NewLength(2.0, LengthUnitMm)})
	filterEnd(t, parser)
}

func TestFilterBlur2percent(t *testing.T) {
	parser := NewFilterValueListParser("blur(2%)")
	filterNextErr(t, parser, "a percentage value detected at position 6")
	filterEnd(t, parser)
}

func TestFilterBlurNegative(t *testing.T) {
	parser := NewFilterValueListParser("blur(-1)")
	filterNextErr(t, parser, "a negative value detected at position 6")
	filterEnd(t, parser)
}

func TestFilterBlurTwoValues(t *testing.T) {
	parser := NewFilterValueListParser("blur(1 2)")
	filterNextErr(t, parser, "expected ')' not '2' at position 8")
	filterEnd(t, parser)
}

func TestFilterBrightnessDefault(t *testing.T) {
	parser := NewFilterValueListParser("brightness()")
	filterNext(t, parser, FilterValue{Kind: FilterValueBrightness, Number: 1.0})
	filterEnd(t, parser)
}

func TestFilterBrightness2(t *testing.T) {
	parser := NewFilterValueListParser("brightness(2)")
	filterNext(t, parser, FilterValue{Kind: FilterValueBrightness, Number: 2.0})
	filterEnd(t, parser)
}

func TestFilterBrightness50percent(t *testing.T) {
	parser := NewFilterValueListParser("brightness(50%)")
	filterNext(t, parser, FilterValue{Kind: FilterValueBrightness, Number: 0.5})
	filterEnd(t, parser)
}

func TestFilterBrightnessNegative(t *testing.T) {
	parser := NewFilterValueListParser("brightness(-1)")
	filterNextErr(t, parser, "a negative value detected at position 12")
	filterEnd(t, parser)
}

func TestFilterBrightness2mm(t *testing.T) {
	parser := NewFilterValueListParser("brightness(2mm)")
	filterNextErr(t, parser, "expected ')' not 'm' at position 13")
	filterEnd(t, parser)
}

func TestFilterDropShadowNoValues(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow()")
	filterNextErr(t, parser, "drop-shadow offset values are expected at position 13")
	filterEnd(t, parser)
}

func TestFilterDropShadowDefault(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(2 3)")
	filterNext(t, parser, dropShadow(Color{}, false, NewLengthNumber(2.0), NewLengthNumber(3.0), LengthZero()))
	filterEnd(t, parser)
}

func TestFilterDropShadowColorFirst(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(red 2 3)")
	filterNext(t, parser, dropShadow(NewColorRGB(255, 0, 0), true, NewLengthNumber(2.0), NewLengthNumber(3.0), LengthZero()))
	filterEnd(t, parser)
}

func TestFilterDropShadowColorAfter(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(2 3 red)")
	filterNext(t, parser, dropShadow(NewColorRGB(255, 0, 0), true, NewLengthNumber(2.0), NewLengthNumber(3.0), LengthZero()))
	filterEnd(t, parser)
}

func TestFilterDropShadowCurrColorFirst(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(currentColor 2 3)")
	filterNext(t, parser, dropShadow(Color{}, false, NewLengthNumber(2.0), NewLengthNumber(3.0), LengthZero()))
	filterEnd(t, parser)
}

func TestFilterDropShadowCurrColorAfter(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(2 3 currentColor)")
	filterNext(t, parser, dropShadow(Color{}, false, NewLengthNumber(2.0), NewLengthNumber(3.0), LengthZero()))
	filterEnd(t, parser)
}

func TestFilterDropShadowWithDev(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(red 2 3 4)")
	filterNext(t, parser, dropShadow(NewColorRGB(255, 0, 0), true, NewLengthNumber(2.0), NewLengthNumber(3.0), NewLengthNumber(4.0)))
	filterEnd(t, parser)
}

func TestFilterDropShadowColorTwice(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(red 2 3 4 red)")
	filterNextErr(t, parser, "expected ')' not 'r' at position 23")
	filterEnd(t, parser)
}

func TestFilterDropShadowCurrColorTwice(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(currentColor 2 3 4 currentColor)")
	filterNextErr(t, parser, "expected ')' not 'c' at position 32")
	filterEnd(t, parser)
}

func TestFilterDropShadowPercent(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(2% 3% 4%)")
	filterNextErr(t, parser, "a percentage value detected at position 13")
	filterEnd(t, parser)
}

func TestFilterDropShadowNegativeOffset(t *testing.T) {
	parser := NewFilterValueListParser("drop-shadow(-1 -2 3)")
	filterNext(t, parser, dropShadow(Color{}, false, NewLengthNumber(-1.0), NewLengthNumber(-2.0), NewLengthNumber(3.0)))
	filterEnd(t, parser)
}

func TestFilterHueRotateNoUnits(t *testing.T) {
	parser := NewFilterValueListParser("hue-rotate(45)")
	filterNextErr(t, parser, "an invalid angle at position 12")
	filterEnd(t, parser)
}

func TestFilterURL(t *testing.T) {
	parser := NewFilterValueListParser("url(#qwe)")
	filterNext(t, parser, FilterValue{Kind: FilterValueURL, URL: "qwe"})
	filterEnd(t, parser)
}

func TestFilterMultiple1(t *testing.T) {
	parser := NewFilterValueListParser("blur() blur()")
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: LengthZero()})
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: LengthZero()})
	filterEnd(t, parser)
}

func TestFilterMultiple2(t *testing.T) {
	parser := NewFilterValueListParser("blur() contrast(1)")
	filterNext(t, parser, FilterValue{Kind: FilterValueBlur, Length: LengthZero()})
	filterNext(t, parser, FilterValue{Kind: FilterValueContrast, Number: 1.0})
	filterEnd(t, parser)
}
