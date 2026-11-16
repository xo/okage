// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// D50 is D50 in tinycss2 tinycss2/color4.py: the XYZ values of the D50 white
// point, normalized to Y=1.
var D50 = [3]float64{0.3457 / 0.3585, 1, (1 - 0.3457 - 0.3585) / 0.3585}

// D65 is D65 in tinycss2 tinycss2/color4.py: the XYZ values of the D65 white
// point, normalized to Y=1.
var D65 = [3]float64{0.3127 / 0.3290, 1, (1 - 0.3127 - 0.3290) / 0.3290}

// The names of the color spaces and of the functions that the code
// compares.
const (
	spaceSRGB     = "srgb"
	spaceXYZ      = "xyz"
	spaceXYZD50   = "xyz-d50"
	spaceXYZD65   = "xyz-d65"
	spaceHSL      = "hsl"
	spaceHWB      = "hwb"
	spaceLab      = "lab"
	spaceLCH      = "lch"
	spaceOklab    = "oklab"
	spaceOklch    = "oklch"
	functionColor = "color"
)

// functionSpaces is _FUNCTION_SPACES in tinycss2 tinycss2/color4.py.
var functionSpaces = map[string]bool{
	"srgb": true, "srgb-linear": true,
	"display-p3": true, "a98-rgb": true, "prophoto-rgb": true, "rec2020": true,
	"xyz": true, "xyz-d50": true, "xyz-d65": true,
}

// ColorSpaces is COLOR_SPACES in tinycss2 tinycss2/color4.py: the color
// spaces of CSS Color Level 4.
var ColorSpaces = map[string]bool{
	"srgb": true, "srgb-linear": true,
	"display-p3": true, "a98-rgb": true, "prophoto-rgb": true, "rec2020": true,
	"xyz": true, "xyz-d50": true, "xyz-d65": true,
	"hsl": true, spaceHWB: true, "lab": true, spaceLCH: true, spaceOklab: true, spaceOklch: true,
}

// Color is Color in tinycss2 tinycss2/color4.py, and the Color of
// tinycss2/color5.py. It is a specified color in a defined color space.
//
// The color space is one of ColorSpaces, "device-cmyk", or a custom space
// of CSS Color Level 5 that starts with "--". The coordinates are floats
// with undefined ranges, and the alpha channel is clipped to [0, 1].
//
// Upstream holds None for a coordinate that is undefined, from the keyword
// none. In Go, Missing[i] is true for it and Coordinates[i] is 0. A nil
// Missing means that each coordinate is defined.
type Color struct {
	// Space is the color space.
	Space string
	// Coordinates holds the coordinates in the color space.
	Coordinates []float64
	// Missing tells, for each coordinate, whether it is undefined.
	Missing []bool
	// Alpha is the alpha channel, in [0, 1].
	Alpha float64
}

// newColor is the constructor of Color in tinycss2 tinycss2/color4.py. A nil
// item of coordinates is the None of upstream.
func newColor(space string, coordinates []*float64, alpha float64) *Color {
	c := &Color{
		Space:       space,
		Coordinates: make([]float64, len(coordinates)),
		Alpha:       clipMaxMin(alpha),
	}
	for i, coordinate := range coordinates {
		if coordinate == nil {
			if c.Missing == nil {
				c.Missing = make([]bool, len(coordinates))
			}
			c.Missing[i] = true
		} else {
			c.Coordinates[i] = *coordinate
		}
	}
	return c
}

// newColorValues is newColor for coordinates that are all defined.
func newColorValues(space string, coordinates []float64, alpha float64) *Color {
	return &Color{
		Space:       space,
		Coordinates: coordinates,
		Alpha:       clipMaxMin(alpha),
	}
}

// IsMissing reports whether coordinate i is undefined, the None of upstream.
func (c *Color) IsMissing(i int) bool {
	return c.Missing != nil && c.Missing[i]
}

// String is Color.__repr__ in tinycss2 tinycss2/color4.py, such as
// "color(srgb 1.0 0.0 0.0 / 1.0)".
func (c *Color) String() string {
	var b strings.Builder
	b.WriteString("color(")
	b.WriteString(c.Space)
	b.WriteByte(' ')
	for i, v := range c.Coordinates {
		if i > 0 {
			b.WriteByte(' ')
		}
		if c.IsMissing(i) {
			b.WriteString("None")
		} else {
			b.WriteString(formatFloat(v))
		}
	}
	b.WriteString(" / ")
	b.WriteString(formatFloat(c.Alpha))
	b.WriteByte(')')
	return b.String()
}

// Equal is Color.__eq__ in tinycss2 tinycss2/color4.py for two colors. It
// compares the spaces and the coordinates, and not the alpha channels, as
// upstream does.
func (c *Color) Equal(other *Color) bool {
	if c == nil || other == nil {
		return c == other
	}
	if c.Space != other.Space || len(c.Coordinates) != len(other.Coordinates) {
		return false
	}
	for i := range c.Coordinates {
		if c.IsMissing(i) != other.IsMissing(i) {
			return false
		}
		if !c.IsMissing(i) && c.Coordinates[i] != other.Coordinates[i] {
			return false
		}
	}
	return true
}

// To is Color.to in tinycss2 tinycss2/color4.py. It returns a new color with
// the coordinates transformed to the given space. Each undefined coordinate
// becomes 0.
//
// It supports these combinations: from hsl and hwb to srgb, from lab and lch
// to xyz-d50, from oklab and oklch to xyz-d65, and from xyz-d50, xyz-d65,
// lch, oklab and oklch to lab. It returns false for another combination,
// where upstream raises NotImplementedError.
func (c *Color) To(space string) (*Color, bool) {
	coordinates := make([]float64, len(c.Coordinates))
	copy(coordinates, c.Coordinates)
	if space == spaceXYZ {
		space = spaceXYZD65
	}
	coordinate := func(i int) float64 {
		if i < len(coordinates) {
			return coordinates[i]
		}
		return 0
	}
	if space == c.Space {
		return newColorValues(space, coordinates, c.Alpha), true
	}
	switch space {
	case spaceSRGB:
		switch c.Space {
		case spaceHSL:
			r, g, b := hlsToRGB(coordinate(0)/360, coordinate(2)/100, coordinate(1)/100)
			return newColorValues(space, []float64{r, g, b}, c.Alpha), true
		case spaceHWB:
			white, black := coordinate(1), coordinate(2)
			var rgb []float64
			if white+black >= 100 {
				v := white / (white + black)
				rgb = []float64{v, v, v}
			} else {
				r, g, b := hlsToRGB(coordinate(0)/360, 0.5, 1)
				for _, channel := range []float64{r, g, b} {
					rgb = append(rgb, (float64(channel*(100-white-black))+white)/100)
				}
			}
			return newColorValues(space, rgb, c.Alpha), true
		}
	case spaceXYZD50:
		switch c.Space {
		case spaceLab:
			x, y, z := labToXYZ(coordinate(0), coordinate(1), coordinate(2), D50)
			return newColorValues(space, []float64{x, y, z}, c.Alpha), true
		case spaceLCH:
			a := coordinate(1) * math.Cos(radians(coordinate(2)))
			b := coordinate(1) * math.Sin(radians(coordinate(2)))
			x, y, z := labToXYZ(coordinate(0), a, b, D50)
			return newColorValues(space, []float64{x, y, z}, c.Alpha), true
		}
	case spaceXYZD65:
		switch c.Space {
		case spaceOklab:
			x, y, z := oklabToXYZ(coordinate(0), coordinate(1), coordinate(2))
			return newColorValues(space, []float64{x, y, z}, c.Alpha), true
		case spaceOklch:
			a := coordinate(1) * math.Cos(radians(coordinate(2)))
			b := coordinate(1) * math.Sin(radians(coordinate(2)))
			x, y, z := oklabToXYZ(coordinate(0), a, b)
			return newColorValues(space, []float64{x, y, z}, c.Alpha), true
		}
	case spaceLab:
		switch c.Space {
		case spaceXYZD50:
			l, a, b := xyzToLab(coordinate(0), coordinate(1), coordinate(2), D50)
			return newColorValues(space, []float64{l, a, b}, c.Alpha), true
		case spaceXYZD65:
			l, a, b := xyzToLab(coordinate(0), coordinate(1), coordinate(2), D65)
			return newColorValues(space, []float64{l, a, b}, c.Alpha), true
		case spaceLCH:
			a := coordinate(1) * math.Cos(radians(coordinate(2)))
			b := coordinate(1) * math.Sin(radians(coordinate(2)))
			return newColorValues(space, []float64{coordinate(0), a, b}, c.Alpha), true
		case spaceOklab:
			x, y, z := oklabToXYZ(coordinate(0), coordinate(1), coordinate(2))
			l, a, b := xyzToLab(x, y, z, D65)
			return newColorValues(space, []float64{l, a, b}, c.Alpha), true
		case spaceOklch:
			a := coordinate(1) * math.Cos(radians(coordinate(2)))
			b := coordinate(1) * math.Sin(radians(coordinate(2)))
			x, y, z := oklabToXYZ(coordinate(0), a, b)
			l, a, b := xyzToLab(x, y, z, D65)
			return newColorValues(space, []float64{l, a, b}, c.Alpha), true
		}
	}
	return nil, false
}

// ParseColor4 is parse_color in tinycss2 tinycss2/color4.py. It parses a
// color value as CSS Color Level 4 defines it, from a component value. Parse
// a string first, as for ParseColor3.
//
// It returns nil and false for a value that is not a valid color, and nil
// and true for the keyword currentcolor, where upstream returns the string
// 'currentcolor'. It returns a Color for every other value, with the
// keywords.
//
// Upstream raises ValueError for "color()" with no argument. The port
// returns nil and false for it (D50).
func ParseColor4(token Node) (color *Color, currentColor bool) {
	switch t := token.(type) {
	case *IdentToken:
		switch t.LowerValue {
		case "currentcolor":
			return nil, true
		case "transparent":
			return newColorValues(spaceSRGB, []float64{0, 0, 0}, 0), false
		}
		if c, ok := colorKeywords4[t.LowerValue]; ok {
			return newColorValues("srgb", []float64{
				float64(c[0]) / 255, float64(c[1]) / 255, float64(c[2]) / 255}, 1), false
		}
	case *HashToken:
		if channels, ok := parseHash(t.Value); ok {
			alpha := 1.0
			if len(channels) == 4 {
				alpha = channels[3]
				channels = channels[:3]
			}
			return newColorValues("srgb", channels, alpha), false
		}
	case *FunctionBlock:
		var tokens []Node
		for _, token := range t.Arguments {
			if tt := token.Type(); tt != TypeWhitespace && tt != TypeComment {
				tokens = append(tokens, token)
			}
		}
		name := t.LowerName
		var space Node
		if name == functionColor {
			if len(tokens) == 0 {
				return nil, false
			}
			space, tokens = tokens[0], tokens[1:]
		}
		length := len(tokens)
		var oldSyntax bool
		switch {
		case (length == 5 || length == 7) && oddCommas(tokens):
			oldSyntax = true
			tokens = evenItems(tokens)
		case length == 3:
			oldSyntax = false
		case length == 5 && IsLiteral(tokens[3], "/"):
			tokens = append(tokens[:3:3], tokens[4])
			oldSyntax = false
		default:
			return nil, false
		}
		args := tokens[:3]
		alpha, ok := parseAlpha4(tokens[3:])
		if !ok {
			return nil, false
		}
		switch {
		case name == "rgb" || name == "rgba":
			return parseRGB4(args, alpha), false
		case name == spaceHSL || name == "hsla":
			return parseHSL4(args, alpha), false
		case name == spaceHWB:
			return parseHWB(args, alpha), false
		case name == "lab" && !oldSyntax:
			return parseLab(args, alpha), false
		case name == spaceLCH && !oldSyntax:
			return parseLCH(args, alpha), false
		case name == spaceOklab && !oldSyntax:
			return parseOklab(args, alpha), false
		case name == spaceOklch && !oldSyntax:
			return parseOklch(args, alpha), false
		case name == functionColor && !oldSyntax:
			return parseColorFunction4(space, args, alpha), false
		}
	}
	return nil, false
}

// oddCommas reports whether each token at an odd index is ",", as
// `all(token == ',' for token in tokens[1::2])` does.
func oddCommas(tokens []Node) bool {
	for i := 1; i < len(tokens); i += 2 {
		if !IsLiteral(tokens[i], ",") {
			return false
		}
	}
	return true
}

// evenItems is tokens[::2] of Python.
func evenItems(tokens []Node) []Node {
	items := make([]Node, 0, (len(tokens)+1)/2)
	for i := 0; i < len(tokens); i += 2 {
		items = append(items, tokens[i])
	}
	return items
}

// parseAlpha4 is _parse_alpha in tinycss2 tinycss2/color4.py. If args is
// empty, it returns 1. If args is a list of a single INTEGER, NUMBER or
// PERCENTAGE token, it returns its value clipped to the range 0 to 1.
func parseAlpha4(args []Node) (float64, bool) {
	switch len(args) {
	case 0:
		return 1, true
	case 1:
		switch t := args[0].(type) {
		case *NumberToken:
			return clipMinMax(t.Value), true
		case *PercentageToken:
			return clipMinMax(t.Value / 100), true
		}
	}
	return 0, false
}

// typeSet is the result of _types in tinycss2 tinycss2/color4.py: the set of
// the types of tokens, without the keyword none.
type typeSet map[string]bool

// types is _types in tinycss2 tinycss2/color4.py.
func types(tokens []Node) typeSet {
	s := typeSet{}
	for _, token := range tokens {
		if t, ok := token.(*IdentToken); ok && t.LowerValue == "none" {
			continue
		}
		s[token.Type()] = true
	}
	return s
}

// is reports whether the set holds the given types only, as set equality.
func (s typeSet) is(types ...string) bool {
	if len(s) != len(types) {
		return false
	}
	for _, t := range types {
		if !s[t] {
			return false
		}
	}
	return true
}

// subsetOf is the operator <= of Python on sets.
func (s typeSet) subsetOf(types ...string) bool {
	for t := range s {
		if !slices.Contains(types, t) {
			return false
		}
	}
	return true
}

// isNoneOrValue returns nil for an ident token, which is the keyword none
// after the check of the types, and otherwise the value of a number, or the
// value of a percentage times percentageScale.
func isNoneOrValue(token Node, percentageScale float64) *float64 {
	var v float64
	switch t := token.(type) {
	case *NumberToken:
		v = t.Value
	case *PercentageToken:
		v = t.Value * percentageScale
	case *DimensionToken:
		v = t.Value
	default:
		return nil
	}
	return &v
}

// value returns the value of a numeric token, or nil for an ident token.
func value(token Node) *float64 {
	return isNoneOrValue(token, 1)
}

// parseRGB4 is _parse_rgb in tinycss2 tinycss2/color4.py. If args is a list
// of 3 NUMBER tokens or 3 PERCENTAGE tokens, it returns an sRGB Color. The
// ranges of R, G and B are [0, 255] on input, and [0, 1] on output.
func parseRGB4(args []Node, alpha float64) *Color {
	s := types(args)
	if !s.is() && !s.is(TypeNumber) && !s.is(TypePercentage) {
		return nil
	}
	coordinates := make([]*float64, len(args))
	for i, arg := range args {
		switch t := arg.(type) {
		case *NumberToken:
			v := t.Value / 255
			coordinates[i] = &v
		case *PercentageToken:
			v := t.Value / 100
			coordinates[i] = &v
		}
	}
	return newColor("srgb", coordinates, alpha)
}

// parseHSL4 is _parse_hsl in tinycss2 tinycss2/color4.py. If args is a list
// of 1 NUMBER or ANGLE token and 2 PERCENTAGE tokens, it returns an HSL
// Color. The range of H is [0, 360), and the ranges of S and L are
// [0, 100].
func parseHSL4(args []Node, alpha float64) *Color {
	s := types(args[1:])
	if !s.is() && !s.is(TypeNumber) && !s.is(TypePercentage) {
		return nil
	}
	hue, ok := parseHue(args[0])
	if !ok {
		return nil
	}
	return newColor("hsl", []*float64{noneOr(args[0], hue), value(args[1]), value(args[2])}, alpha)
}

// noneOr returns nil for an ident token, and &v otherwise.
func noneOr(token Node, v float64) *float64 {
	if token.Type() == TypeIdent {
		return nil
	}
	return &v
}

// parseHWB is _parse_hwb in tinycss2 tinycss2/color4.py. If args is a list
// of 1 NUMBER or ANGLE token and 2 NUMBER or PERCENTAGE tokens, it returns an
// HWB Color. The range of H is [0, 360), and the ranges of W and B are
// [0, 100].
func parseHWB(args []Node, alpha float64) *Color {
	if !types(args[1:]).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	hue, ok := parseHue(args[0])
	if !ok {
		return nil
	}
	return newColor("hwb", []*float64{noneOr(args[0], hue), value(args[1]), value(args[2])}, alpha)
}

// parseLab is _parse_lab in tinycss2 tinycss2/color4.py. If args is a list
// of 3 NUMBER or PERCENTAGE tokens, it returns a Lab Color. The range of L
// is [0, 100], and the ranges of a and b are [-125, 125].
func parseLab(args []Node, alpha float64) *Color {
	if !types(args).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	return newColor("lab", []*float64{
		value(args[0]),
		isNoneOrValue(args[1], 1.25),
		isNoneOrValue(args[2], 1.25),
	}, alpha)
}

// parseLCH is _parse_lch in tinycss2 tinycss2/color4.py. If args is a list
// of 2 NUMBER or PERCENTAGE tokens and 1 NUMBER or ANGLE token, it returns
// an LCH Color. The range of L is [0, 100], of C [0, 150] and of H
// [0, 360).
func parseLCH(args []Node, alpha float64) *Color {
	if !types(args[:2]).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	hue, ok := parseHue(args[2])
	if !ok {
		return nil
	}
	return newColor(spaceLCH, []*float64{
		value(args[0]),
		isNoneOrValue(args[1], 1.5),
		noneOr(args[2], hue),
	}, alpha)
}

// parseOklab is _parse_oklab in tinycss2 tinycss2/color4.py. If args is a
// list of 3 NUMBER or PERCENTAGE tokens, it returns an Oklab Color. The
// range of L is [0, 100], and the ranges of a and b are [-0.4, 0.4].
func parseOklab(args []Node, alpha float64) *Color {
	if !types(args).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	return newColor(spaceOklab, []*float64{
		isNoneOrValue(args[0], 0.01),
		isNoneOrValue(args[1], 0.004),
		isNoneOrValue(args[2], 0.004),
	}, alpha)
}

// parseOklch is _parse_oklch in tinycss2 tinycss2/color4.py. If args is a
// list of 2 NUMBER or PERCENTAGE tokens and 1 NUMBER or ANGLE token, it
// returns an Oklch Color. The range of L is [0, 1], of C [0, 0.4] and of H
// [0, 360).
func parseOklch(args []Node, alpha float64) *Color {
	if !types(args[:2]).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	hue, ok := parseHue(args[2])
	if !ok {
		return nil
	}
	return newColor(spaceOklch, []*float64{
		isNoneOrValue(args[0], 0.01),
		isNoneOrValue(args[1], 0.004),
		noneOr(args[2], hue),
	}, alpha)
}

// parseColorFunction4 is _parse_color in tinycss2 tinycss2/color4.py. It
// parses the name of a color space and a list of coordinates in the range
// [0, 1].
func parseColorFunction4(space Node, args []Node, alpha float64) *Color {
	if !types(args).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	ident, ok := space.(*IdentToken)
	if !ok || !functionSpaces[ident.LowerValue] {
		return nil
	}
	name := ident.LowerValue
	if name == spaceXYZ {
		name = spaceXYZD65
	}
	return newColor(name, numberOrPercentage(args), alpha)
}

// numberOrPercentage maps a number to its value and a percentage to its
// value divided by 100, and another token to nil.
func numberOrPercentage(args []Node) []*float64 {
	coordinates := make([]*float64, len(args))
	for i, arg := range args {
		switch t := arg.(type) {
		case *NumberToken:
			v := t.Value
			coordinates[i] = &v
		case *PercentageToken:
			v := t.Value / 100
			coordinates[i] = &v
		}
	}
	return coordinates
}

// parseHue is _parse_hue in tinycss2 tinycss2/color4.py. Its range is
// [0, 360). The keyword none gives 0.
func parseHue(token Node) (float64, bool) {
	switch t := token.(type) {
	case *NumberToken:
		return pyMod(t.Value, 360), true
	case *DimensionToken:
		switch t.Unit {
		case "deg":
			return pyMod(t.Value, 360), true
		case "grad":
			return pyMod(t.Value/400*360, 360), true
		case "rad":
			return pyMod(degrees(t.Value), 360), true
		case "turn":
			return pyMod(t.Value*360, 360), true
		}
	case *IdentToken:
		if t.LowerValue == "none" {
			return 0, true
		}
	}
	return 0, false
}

// degrees is math.degrees of Python.
func degrees(x float64) float64 {
	return x * (180.0 / math.Pi)
}

// radians is math.radians of Python.
func radians(x float64) float64 {
	return x * (math.Pi / 180.0)
}

// The constants of the conversions, from
// https://www.w3.org/TR/css-color-4/#color-conversion-code.
const (
	kappa   = 24389.0 / 27
	epsilon = 216.0 / 24389
)

// lmsToXYZ is _LMS_TO_XYZ in tinycss2 tinycss2/color4.py.
var lmsToXYZ = [3][3]float64{
	{1.2268798733741557, -0.5578149965554813, 0.28139105017721583},
	{-0.04057576262431372, 1.1122868293970594, -0.07171106666151701},
	{-0.07637294974672142, -0.4214933239627914, 1.5869240244272418},
}

// oklabToLMS is _OKLAB_TO_LMS in tinycss2 tinycss2/color4.py.
var oklabToLMS = [3][3]float64{
	{0.99999999845051981432, 0.39633779217376785678, 0.21580375806075880339},
	{1.0000000088817607767, -0.1055613423236563494, -0.063854174771705903402},
	{1.0000000546724109177, -0.089484182094965759684, -1.2914855378640917399},
}

// pyPow is the operator ** of Python on floats, which calls pow of C.
func pyPow(x, y float64) float64 {
	return math.Pow(x, y)
}

// xyzToLab is _xyz_to_lab in tinycss2 tinycss2/color4.py.
func xyzToLab(bigX, bigY, bigZ float64, d [3]float64) (float64, float64, float64) {
	x := bigX / d[0]
	y := bigY / d[1]
	z := bigZ / d[2]
	f := func(v float64) float64 {
		if v > epsilon {
			return pyPow(v, 1.0/3)
		}
		return (float64(kappa*v) + 16) / 116
	}
	f0, f1, f2 := f(x), f(y), f(z)
	l := float64(116*f1) - 16
	a := 500 * (f0 - f1)
	b := 200 * (f1 - f2)
	return l, a, b
}

// labToXYZ is _lab_to_xyz in tinycss2 tinycss2/color4.py.
func labToXYZ(l, a, b float64, d [3]float64) (float64, float64, float64) {
	f1 := (l + 16) / 116
	f0 := a/500 + f1
	f2 := f1 - b/200
	var x, y, z float64
	if pyPow(f0, 3) > epsilon {
		x = pyPow(f0, 3)
	} else {
		x = (float64(116*f0) - 16) / kappa
	}
	if l > kappa*epsilon {
		y = pyPow((l+16)/116, 3)
	} else {
		y = l / kappa
	}
	if pyPow(f2, 3) > epsilon {
		z = pyPow(f2, 3)
	} else {
		z = (float64(116*f2) - 16) / kappa
	}
	return x * d[0], y * d[1], z * d[2]
}

// oklabToXYZ is _oklab_to_xyz in tinycss2 tinycss2/color4.py.
func oklabToXYZ(l, a, b float64) (float64, float64, float64) {
	lab := [3]float64{l, a, b}
	var lms [3]float64
	for i := range 3 {
		sum := 0.0
		for j := range 3 {
			sum += float64(oklabToLMS[i][j] * lab[j])
		}
		lms[i] = sum
	}
	var xyz [3]float64
	for i := range 3 {
		sum := 0.0
		for j := range 3 {
			sum += float64(lmsToXYZ[i][j] * pyPow(lms[j], 3))
		}
		xyz[i] = sum
	}
	return xyz[0], xyz[1], xyz[2]
}

// colorKeywords4 is _COLOR_KEYWORDS in tinycss2 tinycss2/color4.py, with
// rebeccapurple.
var colorKeywords4 = makeColorKeywords(true)

// formatFloat is str of Python for a float, the shortest representation that
// reads back as the same value.
func formatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:])
	if exp < -4 || exp >= 16 {
		return e
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
