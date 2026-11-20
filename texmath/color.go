package texmath

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// This file ports no file of go-tex/math. It holds the color commands that
// phase 26 of D78 adds: \color, \textcolor, \colorbox, \fcolorbox and \fbox.
// The source has none of them. The layout half of \color and \textcolor is
// layoutColor in layout.go, and that of the boxes is boxed in math.go.

// parseColor reads the color argument of a color command at the start of
// toks: a group or one token, as KaTeX reads it. The color is a name of CSS,
// such as red or SteelBlue, in any case, or a number in hexadecimal: #rgb,
// #rgba, #rrggbb, #rrggbbaa, or rrggbb without the #. KaTeX gives the name to
// the browser, so these are the colors of Mermaid. A color that this cannot
// read is a syntax error.
func (p *parser) parseColor(cmd *token, toks []token) (okage.Color, []token, error) {
	if len(toks) == 0 {
		return okage.Color{}, nil, p.fail(toks, fmt.Sprintf(`\%s needs a color`, cmd.text))
	}
	s, rest := readArgText(toks)
	c, ok := colorOf(strings.TrimSpace(s))
	if !ok {
		return okage.Color{}, nil, p.fail(toks, fmt.Sprintf(`\%s: unknown color %q`, cmd.text, s))
	}
	return c, rest, nil
}

// colorOf returns the color of the name or the number s.
func colorOf(s string) (okage.Color, bool) {
	if v, ok := cssColors[strings.ToLower(s)]; ok {
		return okage.ColorFromRGBA8(uint8(v>>16), uint8(v>>8), uint8(v), 255), true
	}
	hex, sharp := strings.CutPrefix(s, "#")
	switch {
	case sharp && (len(hex) == 3 || len(hex) == 4):
		// Each digit is doubled: #f80 is #ff8800.
		var b strings.Builder
		for _, c := range hex {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		hex = b.String()
	case len(hex) == 6, sharp && len(hex) == 8:
	default:
		return okage.Color{}, false
	}
	if len(hex) == 6 {
		hex += "ff"
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return okage.Color{}, false
	}
	return okage.ColorFromRGBA8(uint8(v>>24), uint8(v>>16), uint8(v>>8), uint8(v)), true
}

// parseTextcolor parses \textcolor{color}{body}: body in the color. The
// layout half is layoutColor.
func (p *parser) parseTextcolor(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	c, rest, err := p.parseColor(cmd, toks)
	if err != nil {
		return nil, nil, err
	}
	b, r, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Color{Color: c, Body: b}), r, nil
}

// startColor reads the color of \color at the start of toks, and the rest of
// the list as its body: \color sets the color of the rest of its group, as
// in LaTeX and KaTeX.
//
//go:noinline
func (b *listBuilder) startColor(toks []token) error {
	p := b.p
	c, rest, err := p.parseColor(&toks[0], toks[1:])
	if err != nil {
		return err
	}
	b.flush()
	body, rest, err := p.parseList(rest, b.st, b.stop)
	if err != nil {
		return err
	}
	b.items = append(b.items, p.add(Color{Color: c, Body: body}))
	b.toks = rest
	return nil
}

// parseColorbox parses \colorbox{color}{text}, \fcolorbox{frame}{color}{text}
// and \fbox{text}. The argument is text, as in LaTeX. The layout half is
// boxed: \colorbox has a background and no frame, \fcolorbox both, and
// \fbox a frame only, as \boxed.
func (p *parser) parseColorbox(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	var n Boxed
	rest := toks
	if cmd.text == "fcolorbox" {
		c, r, err := p.parseColor(cmd, rest)
		if err != nil {
			return nil, nil, err
		}
		n.Border, rest = &c, r
	}
	if cmd.text != "fbox" {
		c, r, err := p.parseColor(cmd, rest)
		if err != nil {
			return nil, nil, err
		}
		n.Background, rest = &c, r
		n.NoFrame = cmd.text == "colorbox"
	}
	body, r, err := p.parseGroupArg(rest, textAlphabet(st))
	if err != nil {
		return nil, nil, err
	}
	n.Body = body
	return p.add(n), r, nil
}

// cssColors are the 147 color names of CSS, from the table of svgtypes that
// internal/svgtypes ports, as 0xrrggbb.
var cssColors = map[string]uint32{
	"aliceblue": 0xf0f8ff, "antiquewhite": 0xfaebd7, "aqua": 0x00ffff,
	"aquamarine": 0x7fffd4, "azure": 0xf0ffff, "beige": 0xf5f5dc,
	"bisque": 0xffe4c4, "black": 0x000000, "blanchedalmond": 0xffebcd,
	"blue": 0x0000ff, "blueviolet": 0x8a2be2, "brown": 0xa52a2a,
	"burlywood": 0xdeb887, "cadetblue": 0x5f9ea0, "chartreuse": 0x7fff00,
	"chocolate": 0xd2691e, "coral": 0xff7f50, "cornflowerblue": 0x6495ed,
	"cornsilk": 0xfff8dc, "crimson": 0xdc143c, "cyan": 0x00ffff,
	"darkblue": 0x00008b, "darkcyan": 0x008b8b,
	"darkgoldenrod": 0xb8860b, "darkgray": 0xa9a9a9,
	"darkgreen": 0x006400, "darkgrey": 0xa9a9a9, "darkkhaki": 0xbdb76b,
	"darkmagenta": 0x8b008b, "darkolivegreen": 0x556b2f,
	"darkorange": 0xff8c00, "darkorchid": 0x9932cc, "darkred": 0x8b0000,
	"darksalmon": 0xe9967a, "darkseagreen": 0x8fbc8f,
	"darkslateblue": 0x483d8b, "darkslategray": 0x2f4f4f,
	"darkslategrey": 0x2f4f4f, "darkturquoise": 0x00ced1,
	"darkviolet": 0x9400d3, "deeppink": 0xff1493,
	"deepskyblue": 0x00bfff, "dimgray": 0x696969, "dimgrey": 0x696969,
	"dodgerblue": 0x1e90ff, "firebrick": 0xb22222,
	"floralwhite": 0xfffaf0, "forestgreen": 0x228b22,
	"fuchsia": 0xff00ff, "gainsboro": 0xdcdcdc, "ghostwhite": 0xf8f8ff,
	"gold": 0xffd700, "goldenrod": 0xdaa520, "gray": 0x808080,
	"green": 0x008000, "greenyellow": 0xadff2f, "grey": 0x808080,
	"honeydew": 0xf0fff0, "hotpink": 0xff69b4, "indianred": 0xcd5c5c,
	"indigo": 0x4b0082, "ivory": 0xfffff0, "khaki": 0xf0e68c,
	"lavender": 0xe6e6fa, "lavenderblush": 0xfff0f5,
	"lawngreen": 0x7cfc00, "lemonchiffon": 0xfffacd,
	"lightblue": 0xadd8e6, "lightcoral": 0xf08080, "lightcyan": 0xe0ffff,
	"lightgoldenrodyellow": 0xfafad2, "lightgray": 0xd3d3d3,
	"lightgreen": 0x90ee90, "lightgrey": 0xd3d3d3, "lightpink": 0xffb6c1,
	"lightsalmon": 0xffa07a, "lightseagreen": 0x20b2aa,
	"lightskyblue": 0x87cefa, "lightslategray": 0x778899,
	"lightslategrey": 0x778899, "lightsteelblue": 0xb0c4de,
	"lightyellow": 0xffffe0, "lime": 0x00ff00, "limegreen": 0x32cd32,
	"linen": 0xfaf0e6, "magenta": 0xff00ff, "maroon": 0x800000,
	"mediumaquamarine": 0x66cdaa, "mediumblue": 0x0000cd,
	"mediumorchid": 0xba55d3, "mediumpurple": 0x9370db,
	"mediumseagreen": 0x3cb371, "mediumslateblue": 0x7b68ee,
	"mediumspringgreen": 0x00fa9a, "mediumturquoise": 0x48d1cc,
	"mediumvioletred": 0xc71585, "midnightblue": 0x191970,
	"mintcream": 0xf5fffa, "mistyrose": 0xffe4e1, "moccasin": 0xffe4b5,
	"navajowhite": 0xffdead, "navy": 0x000080, "oldlace": 0xfdf5e6,
	"olive": 0x808000, "olivedrab": 0x6b8e23, "orange": 0xffa500,
	"orangered": 0xff4500, "orchid": 0xda70d6, "palegoldenrod": 0xeee8aa,
	"palegreen": 0x98fb98, "paleturquoise": 0xafeeee,
	"palevioletred": 0xdb7093, "papayawhip": 0xffefd5,
	"peachpuff": 0xffdab9, "peru": 0xcd853f, "pink": 0xffc0cb,
	"plum": 0xdda0dd, "powderblue": 0xb0e0e6, "purple": 0x800080,
	"red": 0xff0000, "rosybrown": 0xbc8f8f, "royalblue": 0x4169e1,
	"saddlebrown": 0x8b4513, "salmon": 0xfa8072, "sandybrown": 0xf4a460,
	"seagreen": 0x2e8b57, "seashell": 0xfff5ee, "sienna": 0xa0522d,
	"silver": 0xc0c0c0, "skyblue": 0x87ceeb, "slateblue": 0x6a5acd,
	"slategray": 0x708090, "slategrey": 0x708090, "snow": 0xfffafa,
	"springgreen": 0x00ff7f, "steelblue": 0x4682b4, "tan": 0xd2b48c,
	"teal": 0x008080, "thistle": 0xd8bfd8, "tomato": 0xff6347,
	"turquoise": 0x40e0d0, "violet": 0xee82ee, "wheat": 0xf5deb3,
	"white": 0xffffff, "whitesmoke": 0xf5f5f5, "yellow": 0xffff00,
	"yellowgreen": 0x9acd32,
}
