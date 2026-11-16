// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import "strings"

// schemeLight is the default color scheme.
const schemeLight = "light"

// ColorSchemes is COLOR_SCHEMES in tinycss2 tinycss2/color5.py: the color
// schemes that light-dark() resolves.
var ColorSchemes = map[string]bool{"light": true, "dark": true}

// ParseColor5 is parse_color in tinycss2 tinycss2/color5.py. It parses a
// color value as CSS Color Level 5 defines it, from a component value. Parse
// a string first, as for ParseColor3.
//
// colorSchemes resolves the function light-dark(). The first item that is
// "light" or "dark" wins, and the default is "light". Upstream takes None,
// the string 'normal' or an iterable of color schemes, and each of them that
// holds no "light" and no "dark" gives "light" too.
//
// It returns nil and false for a value that is not a valid color, nil and
// true for the keyword currentcolor, and a Color for every other value.
//
// Upstream raises an exception for "color()" with no argument, for the
// keyword none in device-cmyk(), and for an invalid alpha value in
// device-cmyk() or in color() of a custom color space. The port returns nil
// and false for each of them (D50).
func ParseColor5(token Node, colorSchemes ...string) (color *Color, currentColor bool) {
	if c, cur := ParseColor4(token); c != nil || cur {
		return c, cur
	}

	colorScheme := schemeLight
	for _, scheme := range colorSchemes {
		if ColorSchemes[scheme] {
			colorScheme = scheme
			break
		}
	}

	t, ok := token.(*FunctionBlock)
	if !ok {
		return nil, false
	}
	var tokens []Node
	for _, token := range t.Arguments {
		if tt := token.Type(); tt != TypeWhitespace && tt != TypeComment {
			tokens = append(tokens, token)
		}
	}
	name := t.LowerName
	var alpha []Node

	var space Node
	if name == functionColor {
		if len(tokens) == 0 {
			return nil, false
		}
		space, tokens = tokens[0], tokens[1:]
	}

	oldSyntax := oddCommas(tokens)
	if oldSyntax {
		tokens = evenItems(tokens)
	} else {
		for index, token := range tokens {
			if IsLiteral(token, "/") {
				alpha = tokens[index+1:]
				tokens = tokens[:index]
				break
			}
		}
	}

	switch name {
	case "device-cmyk":
		a, ok := parseAlpha4(alpha)
		if !ok {
			return nil, false
		}
		return parseDeviceCMYK(tokens, a, oldSyntax), false
	case functionColor:
		a, ok := parseAlpha4(alpha)
		if !ok {
			return nil, false
		}
		return parseColorFunction5(space, tokens, a), false
	case "light-dark":
		return parseLightDark(tokens, colorScheme)
	}
	return nil, false
}

// parseDeviceCMYK is _parse_device_cmyk in tinycss2 tinycss2/color5.py. If
// args is a list of 4 NUMBER or PERCENTAGE tokens, it returns a device-cmyk
// Color. The ranges of C, M, Y and K are [0, 1] on input and on output.
func parseDeviceCMYK(args []Node, alpha float64, oldSyntax bool) *Color {
	if oldSyntax {
		if !types(args).is(TypeNumber) {
			return nil
		}
	} else if !types(args).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	if len(args) != 4 {
		return nil
	}
	cmyk := make([]float64, 0, 4)
	for _, v := range numberOrPercentage(args) {
		if v == nil {
			// Upstream raises TypeError on float(None).
			return nil
		}
		cmyk = append(cmyk, clipMaxMin(*v))
	}
	return newColorValues("device-cmyk", cmyk, alpha)
}

// parseLightDark is _parse_light_dark in tinycss2 tinycss2/color5.py. It
// returns true for currentcolor, as ParseColor5 does.
func parseLightDark(args []Node, colorScheme string) (*Color, bool) {
	type result struct {
		color        *Color
		currentColor bool
	}
	var colors []result
	for _, arg := range args {
		// Upstream passes the string colorScheme as the iterable of color
		// schemes, so each of its characters is a scheme, and the nested
		// call always resolves to light. No scheme gives the same result.
		if color, cur := ParseColor5(arg); color != nil || cur {
			colors = append(colors, result{color, cur})
		}
	}
	if len(colors) == 2 {
		if colorScheme == schemeLight {
			return colors[0].color, colors[0].currentColor
		}
		return colors[1].color, colors[1].currentColor
	}
	return nil, false
}

// parseColorFunction5 is _parse_color in tinycss2 tinycss2/color5.py. It
// parses the name of a custom color space and a list of coordinates in the
// range [0, 1].
func parseColorFunction5(space Node, args []Node, alpha float64) *Color {
	if !types(args).subsetOf(TypeNumber, TypePercentage) {
		return nil
	}
	ident, ok := space.(*IdentToken)
	if !ok || !strings.HasPrefix(ident.Value, "--") {
		return nil
	}
	return newColor(ident.Value, numberOrPercentage(args), alpha)
}
