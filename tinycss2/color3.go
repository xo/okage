// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"math"
	"strconv"
	"strings"
)

// RGBA is RGBA in tinycss2 tinycss2/color3.py, an RGBA color: four floats
// in the range 0 to 1.
type RGBA struct {
	Red, Green, Blue, Alpha float64
}

// ParseColor3 is parse_color in tinycss2 tinycss2/color3.py. It parses a
// color value as CSS Color Level 3 defines it, from a component value.
// Upstream also takes a string. In Go, parse the string first:
//
//	tinycss2.ParseColor3(tinycss2.ParseOneComponentValue(s, tinycss2.WithSkipComments(true)))
//
// It returns nil and false for a value that is not a valid color. It returns
// nil and true for the keyword currentColor, where upstream returns the
// string 'currentColor'. It returns an RGBA for every other value, with the
// keywords, HSL and HSLA. The alpha channel is clipped to [0, 1], but red,
// green and blue can be out of range: rgb(-10%, 120%, 0%) gives
// (-0.1, 1.2, 0, 1).
func ParseColor3(token Node) (color *RGBA, currentColor bool) {
	switch t := token.(type) {
	case *IdentToken:
		if t.LowerValue == "currentcolor" {
			return nil, true
		}
		if t.LowerValue == "transparent" {
			return &RGBA{0, 0, 0, 0}, false
		}
		if c, ok := colorKeywords3[t.LowerValue]; ok {
			return &RGBA{float64(c[0]) / 255, float64(c[1]) / 255, float64(c[2]) / 255, 1}, false
		}
	case *HashToken:
		if channels, ok := parseHash(t.Value); ok {
			if len(channels) == 3 {
				channels = append(channels, 1)
			}
			return &RGBA{channels[0], channels[1], channels[2], channels[3]}, false
		}
	case *FunctionBlock:
		args, ok := parseCommaSeparated(t.Arguments)
		if ok && len(args) > 0 {
			switch t.LowerName {
			case "rgb":
				return parseRGB3(args, 1), false
			case "rgba":
				if alpha, ok := parseAlpha3(args[min(3, len(args)):]); ok {
					return parseRGB3(args[:min(3, len(args))], alpha), false
				}
			case spaceHSL:
				return parseHSL3(args, 1), false
			case "hsla":
				if alpha, ok := parseAlpha3(args[min(3, len(args)):]); ok {
					return parseHSL3(args[:min(3, len(args))], alpha), false
				}
			}
		}
	}
	return nil, false
}

// parseAlpha3 is _parse_alpha in tinycss2 tinycss2/color3.py. If args is a
// list of a single INTEGER or NUMBER token, it returns its value clipped to
// the range 0 to 1.
func parseAlpha3(args []Node) (float64, bool) {
	if len(args) == 1 {
		if t, ok := args[0].(*NumberToken); ok {
			return clipMinMax(t.Value), true
		}
	}
	return 0, false
}

// parseRGB3 is _parse_rgb in tinycss2 tinycss2/color3.py. If args is a list
// of 3 INTEGER tokens or 3 PERCENTAGE tokens, it returns the color.
func parseRGB3(args []Node, alpha float64) *RGBA {
	if len(args) != 3 {
		return nil
	}
	var ints, percentages [3]float64
	allInts, allPercentages := true, true
	for i, arg := range args {
		if n, ok := arg.(*NumberToken); ok && n.IsInteger {
			ints[i] = float64(n.IntValue) / 255
		} else {
			allInts = false
		}
		if p, ok := arg.(*PercentageToken); ok {
			percentages[i] = p.Value / 100
		} else {
			allPercentages = false
		}
	}
	if allInts {
		return &RGBA{ints[0], ints[1], ints[2], alpha}
	}
	if allPercentages {
		return &RGBA{percentages[0], percentages[1], percentages[2], alpha}
	}
	return nil
}

// parseHSL3 is _parse_hsl in tinycss2 tinycss2/color3.py. If args is a list
// of 1 NUMBER token and 2 PERCENTAGE tokens, it returns the color.
func parseHSL3(args []Node, alpha float64) *RGBA {
	if len(args) != 3 {
		return nil
	}
	h, ok1 := args[0].(*NumberToken)
	s, ok2 := args[1].(*PercentageToken)
	l, ok3 := args[2].(*PercentageToken)
	if !ok1 || !ok2 || !ok3 {
		return nil
	}
	r, g, b := hlsToRGB(h.Value/360, l.Value/100, s.Value/100)
	return &RGBA{r, g, b, alpha}
}

// parseCommaSeparated is _parse_comma_separated in tinycss2
// tinycss2/color3.py. It parses a list of tokens, such as the content of a
// function token, as arguments of a single token each, separated by
// mandatory commas, with optional white space around each argument.
//
// It returns the arguments without commas or white space, or false if the
// tokens do not match this description.
func parseCommaSeparated(tokens []Node) ([]Node, bool) {
	var filtered []Node
	for _, token := range tokens {
		if t := token.Type(); t != TypeWhitespace && t != TypeComment {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 0 {
		return []Node{}, true
	}
	if len(filtered)%2 == 1 {
		args := make([]Node, 0, len(filtered)/2+1)
		for i, token := range filtered {
			if i%2 == 1 {
				if !IsLiteral(token, ",") {
					return nil, false
				}
			} else {
				args = append(args, token)
			}
		}
		return args, true
	}
	return nil, false
}

// parseHash is the loop on _HASH_REGEXPS in tinycss2 tinycss2/color3.py. It
// returns the channels of a hash value of 4, 8, 3 or 6 hexadecimal digits,
// in the range 0 to 1. The $ of Python also matches before a newline at the
// end of the value.
func parseHash(value string) ([]float64, bool) {
	value = strings.TrimSuffix(value, "\n")
	for _, c := range value {
		if !isHexDigit(c) {
			return nil, false
		}
	}
	var size int
	switch len(value) {
	case 4, 3:
		size = 1
	case 8, 6:
		size = 2
	default:
		return nil, false
	}
	channels := make([]float64, 0, 4)
	for i := 0; i < len(value); i += size {
		group := value[i : i+size]
		if size == 1 {
			group += group
		}
		v, _ := strconv.ParseUint(group, 16, 8)
		channels = append(channels, float64(v)/255)
	}
	return channels, true
}

// The constants of colorsys of Python.
const (
	oneThird = 1.0 / 3.0
	oneSixth = 1.0 / 6.0
	twoThird = 2.0 / 3.0
)

// hlsToRGB is hls_to_rgb of colorsys of Python.
func hlsToRGB(h, l, s float64) (float64, float64, float64) {
	if s == 0.0 {
		return l, l, l
	}
	var m2 float64
	if l <= 0.5 {
		m2 = l * (1.0 + s)
	} else {
		m2 = l + s - float64(l*s)
	}
	m1 := float64(2.0*l) - m2
	return hlsV(m1, m2, h+oneThird), hlsV(m1, m2, h), hlsV(m1, m2, h-oneThird)
}

// hlsV is _v of colorsys of Python.
func hlsV(m1, m2, hue float64) float64 {
	hue = pyMod(hue, 1.0)
	if hue < oneSixth {
		return m1 + float64((m2-m1)*hue*6.0)
	}
	if hue < 0.5 {
		return m2
	}
	if hue < twoThird {
		return m1 + float64((m2-m1)*(twoThird-hue)*6.0)
	}
	return m1
}

// pyMod is the operator % of Python on floats. The result has the sign of
// y.
func pyMod(x, y float64) float64 {
	mod := math.Mod(x, y)
	if mod != 0 {
		if (y < 0) != (mod < 0) {
			mod += y
		}
	} else {
		mod = math.Copysign(0, y)
	}
	return mod
}

// clipMinMax is min(1, max(0, v)) of Python. max returns 0 unless v > 0,
// and min returns 1 unless the value is less than 1, so NaN gives 0.
func clipMinMax(v float64) float64 {
	if !(v > 0) {
		v = 0
	}
	if v < 1 {
		return v
	}
	return 1
}

// clipMaxMin is max(0., min(1., v)) of Python. min returns 1 unless v < 1,
// and max returns 0 unless the value is more than 0, so NaN gives 1.
func clipMaxMin(v float64) float64 {
	if !(v < 1) {
		v = 1
	}
	if v > 0 {
		return v
	}
	return 0
}

// colorKeywords3 is _COLOR_KEYWORDS in tinycss2 tinycss2/color3.py, without
// its special keywords. It maps a keyword to (r, g, b) in 0 to 255.
var colorKeywords3 = makeColorKeywords(false)

// makeColorKeywords merges _BASIC_COLOR_KEYWORDS and
// _EXTENDED_COLOR_KEYWORDS, and adds rebeccapurple for CSS Color Level 4.
func makeColorKeywords(level4 bool) map[string][3]uint8 {
	m := make(map[string][3]uint8, len(basicColorKeywords)+len(extendedColorKeywords)+1)
	for _, k := range basicColorKeywords {
		m[k.name] = k.rgb
	}
	for _, k := range extendedColorKeywords {
		m[k.name] = k.rgb
	}
	if level4 {
		m["rebeccapurple"] = [3]uint8{102, 51, 153}
	}
	return m
}

// colorKeyword is a keyword and its (r, g, b) in 0 to 255.
type colorKeyword struct {
	name string
	rgb  [3]uint8
}

// basicColorKeywords is _BASIC_COLOR_KEYWORDS in tinycss2
// tinycss2/color3.py.
var basicColorKeywords = []colorKeyword{
	{"black", [3]uint8{0, 0, 0}},
	{"silver", [3]uint8{192, 192, 192}},
	{"gray", [3]uint8{128, 128, 128}},
	{"white", [3]uint8{255, 255, 255}},
	{"maroon", [3]uint8{128, 0, 0}},
	{"red", [3]uint8{255, 0, 0}},
	{"purple", [3]uint8{128, 0, 128}},
	{"fuchsia", [3]uint8{255, 0, 255}},
	{"green", [3]uint8{0, 128, 0}},
	{"lime", [3]uint8{0, 255, 0}},
	{"olive", [3]uint8{128, 128, 0}},
	{"yellow", [3]uint8{255, 255, 0}},
	{"navy", [3]uint8{0, 0, 128}},
	{"blue", [3]uint8{0, 0, 255}},
	{"teal", [3]uint8{0, 128, 128}},
	{"aqua", [3]uint8{0, 255, 255}},
}

// extendedColorKeywords is _EXTENDED_COLOR_KEYWORDS in tinycss2
// tinycss2/color3.py.
var extendedColorKeywords = []colorKeyword{
	{"aliceblue", [3]uint8{240, 248, 255}},
	{"antiquewhite", [3]uint8{250, 235, 215}},
	{"aqua", [3]uint8{0, 255, 255}},
	{"aquamarine", [3]uint8{127, 255, 212}},
	{"azure", [3]uint8{240, 255, 255}},
	{"beige", [3]uint8{245, 245, 220}},
	{"bisque", [3]uint8{255, 228, 196}},
	{"black", [3]uint8{0, 0, 0}},
	{"blanchedalmond", [3]uint8{255, 235, 205}},
	{"blue", [3]uint8{0, 0, 255}},
	{"blueviolet", [3]uint8{138, 43, 226}},
	{"brown", [3]uint8{165, 42, 42}},
	{"burlywood", [3]uint8{222, 184, 135}},
	{"cadetblue", [3]uint8{95, 158, 160}},
	{"chartreuse", [3]uint8{127, 255, 0}},
	{"chocolate", [3]uint8{210, 105, 30}},
	{"coral", [3]uint8{255, 127, 80}},
	{"cornflowerblue", [3]uint8{100, 149, 237}},
	{"cornsilk", [3]uint8{255, 248, 220}},
	{"crimson", [3]uint8{220, 20, 60}},
	{"cyan", [3]uint8{0, 255, 255}},
	{"darkblue", [3]uint8{0, 0, 139}},
	{"darkcyan", [3]uint8{0, 139, 139}},
	{"darkgoldenrod", [3]uint8{184, 134, 11}},
	{"darkgray", [3]uint8{169, 169, 169}},
	{"darkgreen", [3]uint8{0, 100, 0}},
	{"darkgrey", [3]uint8{169, 169, 169}},
	{"darkkhaki", [3]uint8{189, 183, 107}},
	{"darkmagenta", [3]uint8{139, 0, 139}},
	{"darkolivegreen", [3]uint8{85, 107, 47}},
	{"darkorange", [3]uint8{255, 140, 0}},
	{"darkorchid", [3]uint8{153, 50, 204}},
	{"darkred", [3]uint8{139, 0, 0}},
	{"darksalmon", [3]uint8{233, 150, 122}},
	{"darkseagreen", [3]uint8{143, 188, 143}},
	{"darkslateblue", [3]uint8{72, 61, 139}},
	{"darkslategray", [3]uint8{47, 79, 79}},
	{"darkslategrey", [3]uint8{47, 79, 79}},
	{"darkturquoise", [3]uint8{0, 206, 209}},
	{"darkviolet", [3]uint8{148, 0, 211}},
	{"deeppink", [3]uint8{255, 20, 147}},
	{"deepskyblue", [3]uint8{0, 191, 255}},
	{"dimgray", [3]uint8{105, 105, 105}},
	{"dimgrey", [3]uint8{105, 105, 105}},
	{"dodgerblue", [3]uint8{30, 144, 255}},
	{"firebrick", [3]uint8{178, 34, 34}},
	{"floralwhite", [3]uint8{255, 250, 240}},
	{"forestgreen", [3]uint8{34, 139, 34}},
	{"fuchsia", [3]uint8{255, 0, 255}},
	{"gainsboro", [3]uint8{220, 220, 220}},
	{"ghostwhite", [3]uint8{248, 248, 255}},
	{"gold", [3]uint8{255, 215, 0}},
	{"goldenrod", [3]uint8{218, 165, 32}},
	{"gray", [3]uint8{128, 128, 128}},
	{"green", [3]uint8{0, 128, 0}},
	{"greenyellow", [3]uint8{173, 255, 47}},
	{"grey", [3]uint8{128, 128, 128}},
	{"honeydew", [3]uint8{240, 255, 240}},
	{"hotpink", [3]uint8{255, 105, 180}},
	{"indianred", [3]uint8{205, 92, 92}},
	{"indigo", [3]uint8{75, 0, 130}},
	{"ivory", [3]uint8{255, 255, 240}},
	{"khaki", [3]uint8{240, 230, 140}},
	{"lavender", [3]uint8{230, 230, 250}},
	{"lavenderblush", [3]uint8{255, 240, 245}},
	{"lawngreen", [3]uint8{124, 252, 0}},
	{"lemonchiffon", [3]uint8{255, 250, 205}},
	{"lightblue", [3]uint8{173, 216, 230}},
	{"lightcoral", [3]uint8{240, 128, 128}},
	{"lightcyan", [3]uint8{224, 255, 255}},
	{"lightgoldenrodyellow", [3]uint8{250, 250, 210}},
	{"lightgray", [3]uint8{211, 211, 211}},
	{"lightgreen", [3]uint8{144, 238, 144}},
	{"lightgrey", [3]uint8{211, 211, 211}},
	{"lightpink", [3]uint8{255, 182, 193}},
	{"lightsalmon", [3]uint8{255, 160, 122}},
	{"lightseagreen", [3]uint8{32, 178, 170}},
	{"lightskyblue", [3]uint8{135, 206, 250}},
	{"lightslategray", [3]uint8{119, 136, 153}},
	{"lightslategrey", [3]uint8{119, 136, 153}},
	{"lightsteelblue", [3]uint8{176, 196, 222}},
	{"lightyellow", [3]uint8{255, 255, 224}},
	{"lime", [3]uint8{0, 255, 0}},
	{"limegreen", [3]uint8{50, 205, 50}},
	{"linen", [3]uint8{250, 240, 230}},
	{"magenta", [3]uint8{255, 0, 255}},
	{"maroon", [3]uint8{128, 0, 0}},
	{"mediumaquamarine", [3]uint8{102, 205, 170}},
	{"mediumblue", [3]uint8{0, 0, 205}},
	{"mediumorchid", [3]uint8{186, 85, 211}},
	{"mediumpurple", [3]uint8{147, 112, 219}},
	{"mediumseagreen", [3]uint8{60, 179, 113}},
	{"mediumslateblue", [3]uint8{123, 104, 238}},
	{"mediumspringgreen", [3]uint8{0, 250, 154}},
	{"mediumturquoise", [3]uint8{72, 209, 204}},
	{"mediumvioletred", [3]uint8{199, 21, 133}},
	{"midnightblue", [3]uint8{25, 25, 112}},
	{"mintcream", [3]uint8{245, 255, 250}},
	{"mistyrose", [3]uint8{255, 228, 225}},
	{"moccasin", [3]uint8{255, 228, 181}},
	{"navajowhite", [3]uint8{255, 222, 173}},
	{"navy", [3]uint8{0, 0, 128}},
	{"oldlace", [3]uint8{253, 245, 230}},
	{"olive", [3]uint8{128, 128, 0}},
	{"olivedrab", [3]uint8{107, 142, 35}},
	{"orange", [3]uint8{255, 165, 0}},
	{"orangered", [3]uint8{255, 69, 0}},
	{"orchid", [3]uint8{218, 112, 214}},
	{"palegoldenrod", [3]uint8{238, 232, 170}},
	{"palegreen", [3]uint8{152, 251, 152}},
	{"paleturquoise", [3]uint8{175, 238, 238}},
	{"palevioletred", [3]uint8{219, 112, 147}},
	{"papayawhip", [3]uint8{255, 239, 213}},
	{"peachpuff", [3]uint8{255, 218, 185}},
	{"peru", [3]uint8{205, 133, 63}},
	{"pink", [3]uint8{255, 192, 203}},
	{"plum", [3]uint8{221, 160, 221}},
	{"powderblue", [3]uint8{176, 224, 230}},
	{"purple", [3]uint8{128, 0, 128}},
	{"red", [3]uint8{255, 0, 0}},
	{"rosybrown", [3]uint8{188, 143, 143}},
	{"royalblue", [3]uint8{65, 105, 225}},
	{"saddlebrown", [3]uint8{139, 69, 19}},
	{"salmon", [3]uint8{250, 128, 114}},
	{"sandybrown", [3]uint8{244, 164, 96}},
	{"seagreen", [3]uint8{46, 139, 87}},
	{"seashell", [3]uint8{255, 245, 238}},
	{"sienna", [3]uint8{160, 82, 45}},
	{"silver", [3]uint8{192, 192, 192}},
	{"skyblue", [3]uint8{135, 206, 235}},
	{"slateblue", [3]uint8{106, 90, 205}},
	{"slategray", [3]uint8{112, 128, 144}},
	{"slategrey", [3]uint8{112, 128, 144}},
	{"snow", [3]uint8{255, 250, 250}},
	{"springgreen", [3]uint8{0, 255, 127}},
	{"steelblue", [3]uint8{70, 130, 180}},
	{"tan", [3]uint8{210, 180, 140}},
	{"teal", [3]uint8{0, 128, 128}},
	{"thistle", [3]uint8{216, 191, 216}},
	{"tomato", [3]uint8{255, 99, 71}},
	{"turquoise", [3]uint8{64, 224, 208}},
	{"violet", [3]uint8{238, 130, 238}},
	{"wheat", [3]uint8{245, 222, 179}},
	{"white", [3]uint8{255, 255, 255}},
	{"whitesmoke", [3]uint8{245, 245, 245}},
	{"yellow", [3]uint8{255, 255, 0}},
	{"yellowgreen", [3]uint8{154, 205, 50}},
}
