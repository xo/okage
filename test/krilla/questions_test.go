package krilla

import (
	"encoding/base64"
	"encoding/json"
	"image"
)

// The types and the functions of this file write the questions of doc.go.
// A value of a question is a map where a missing field and a zero value
// differ, such as an opacity of 0.

// subsetQuestion asks for the subset of a font.
type subsetQuestion struct {
	Subset subsetSpec `json:"subset"`
}

type subsetSpec struct {
	Font   string   `json:"font"`
	Index  uint32   `json:"index"`
	Glyphs []uint16 `json:"glyphs"`
	// Coords is nil to call subset, and not nil, even empty, to call
	// subset_with_variations.
	Coords *[]coord `json:"coords,omitempty"`
}

// coord is the coordinate of one axis of a variable font. It is written as
// ["wght", 700].
type coord struct {
	Tag   string
	Value float32
}

func (c coord) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{c.Tag, c.Value})
}

// writerQuestion asks for the bytes of a list of calls of pdf-writer.
type writerQuestion struct {
	PdfWriter writerSpec `json:"pdf_writer"`
}

type writerSpec struct {
	Pretty       *bool          `json:"pretty,omitempty"`
	Chunk        bool           `json:"chunk,omitempty"`
	Version      *[2]int        `json:"version,omitempty"`
	BinaryMarker *[4]int        `json:"binary_marker,omitempty"`
	FileID       *[2]string     `json:"file_id,omitempty"`
	Objects      []writerObject `json:"objects"`
	XrefStream   int            `json:"xref_stream,omitempty"`
	// XrefFilter is a string for one filter, or a []string for a list.
	XrefFilter any      `json:"xref_filter,omitempty"`
	Renumber   [][2]int `json:"renumber,omitempty"`
}

// writerObject is an indirect object of pdf-writer. Value, Dict, Data and
// Content are maps and slices of the values that pw makes.
type writerObject struct {
	ID      int       `json:"id"`
	Value   any       `json:"value,omitempty"`
	Start   string    `json:"start,omitempty"`
	Dict    [][2]any  `json:"dict,omitempty"`
	Data    string    `json:"data,omitempty"`
	Content []writeOp `json:"content,omitempty"`
	Filter  string    `json:"filter,omitempty"`
}

// writeOp is an operation of a content stream.
type writeOp struct {
	Op       string `json:"op"`
	Operands []any  `json:"operands,omitempty"`
}

// pw makes the values of pdf-writer.
var pw = struct {
	Null  any
	Bool  func(bool) any
	Int   func(int32) any
	Real  func(float32) any
	Name  func(string) any
	Str   func([]byte) any
	Text  func(string) any
	Ref   func(int) any
	Array func(...any) any
	Dict  func(...[2]any) any
	Pair  func(string, any) [2]any
}{
	Null:  "null",
	Bool:  func(b bool) any { return map[string]any{"bool": b} },
	Int:   func(i int32) any { return map[string]any{"int": i} },
	Real:  func(f float32) any { return map[string]any{"real": f} },
	Name:  func(n string) any { return map[string]any{"name": n} },
	Str:   func(b []byte) any { return map[string]any{"str": hexString(b)} },
	Text:  func(s string) any { return map[string]any{"text": s} },
	Ref:   func(r int) any { return map[string]any{"ref": r} },
	Array: func(v ...any) any { return map[string]any{"array": append([]any{}, v...)} },
	Dict:  func(p ...[2]any) any { return map[string]any{"dict": append([][2]any{}, p...)} },
	Pair:  func(k string, v any) [2]any { return [2]any{k, v} },
}

func hexString(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(b))
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

// svgQuestion asks for the PDF file of an SVG file.
type svgQuestion struct {
	Svg svgSpec `json:"svg"`
}

// svgSpec is an SVG file, with the fields of SvgSettings and the settings of
// the document. A nil field takes the default of krilla.
type svgSpec struct {
	File        string         `json:"file"`
	Width       *float32       `json:"width,omitempty"`
	Height      *float32       `json:"height,omitempty"`
	EmbedText   *bool          `json:"embed_text,omitempty"`
	FilterScale *float32       `json:"filter_scale,omitempty"`
	Fonts       *fontsSpec     `json:"fonts,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
}

// fontsSpec holds the fonts of the font database of usvg.
type fontsSpec struct {
	Dirs      []string `json:"dirs,omitempty"`
	Files     []string `json:"files,omitempty"`
	Serif     string   `json:"serif,omitempty"`
	SansSerif string   `json:"sans_serif,omitempty"`
	Cursive   string   `json:"cursive,omitempty"`
	Fantasy   string   `json:"fantasy,omitempty"`
	Monospace string   `json:"monospace,omitempty"`
}

// sceneQuestion asks for the PDF file of a scene of calls of the surface.
type sceneQuestion struct {
	Krilla scene `json:"krilla"`
}

type scene struct {
	Settings map[string]any `json:"settings,omitempty"`
	Fonts    []fontSpec     `json:"fonts,omitempty"`
	Images   []imageSpec    `json:"images,omitempty"`
	Pages    []pageSpec     `json:"pages"`
}

type fontSpec struct {
	File   string  `json:"file"`
	Index  uint32  `json:"index"`
	Coords []coord `json:"coords,omitempty"`
}

type imageSpec struct {
	File        string `json:"file,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Interpolate bool   `json:"interpolate,omitempty"`
	RGBA        string `json:"rgba,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// rgbaImage makes an image of a scene from an NRGBA image.
func rgbaImage(img *image.NRGBA) imageSpec {
	b := img.Bounds()
	pix := make([]byte, 0, 4*b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		i := img.PixOffset(b.Min.X, y)
		pix = append(pix, img.Pix[i:i+4*b.Dx()]...)
	}
	return imageSpec{RGBA: base64.StdEncoding.EncodeToString(pix), Width: b.Dx(), Height: b.Dy()}
}

type pageSpec struct {
	Width  float32 `json:"width"`
	Height float32 `json:"height"`
	Ops    []op    `json:"ops"`
}

// op is a call of the surface, such as {"op": "draw_path", "path": ...}.
type op map[string]any

// seg is a segment of a path: [0, x, y] moves, [1, x, y] draws a line, [2,
// x1, y1, x, y] a quadratic curve, [3, x1, y1, x2, y2, x, y] a cubic curve,
// and [4] closes.
type seg []float64

// rect returns the segments of a rectangle.
func rect(x, y, w, h float64) []seg {
	return []seg{{0, x, y}, {1, x + w, y}, {1, x + w, y + h}, {1, x, y + h}, {4}}
}

// The paints and the colors of a scene.
func rgb(r, g, b int) map[string]any     { return map[string]any{"rgb": []int{r, g, b}} }
func luma(l int) map[string]any          { return map[string]any{"luma": l} }
func cmyk(c, m, y, k int) map[string]any { return map[string]any{"cmyk": []int{c, m, y, k}} }

// stop is a stop of a gradient.
func stop(offset float32, color map[string]any, opacity float32) map[string]any {
	return map[string]any{"offset": offset, "color": color, "opacity": opacity}
}

// The calls of the surface.
func setFill(paint map[string]any, opacity float32, rule string) op {
	return op{"op": "set_fill", "fill": map[string]any{"paint": paint, "opacity": opacity, "rule": rule}}
}

func noFill() op { return op{"op": "set_fill"} }

func setStroke(stroke map[string]any) op { return op{"op": "set_stroke", "stroke": stroke} }

func noStroke() op { return op{"op": "set_stroke"} }

func drawPath(p []seg) op { return op{"op": "draw_path", "path": p} }

func pushTransform(sx, ky, kx, sy, tx, ty float32) op {
	return op{"op": "push_transform", "transform": []float32{sx, ky, kx, sy, tx, ty}}
}

func pushBlendMode(mode string) op { return op{"op": "push_blend_mode", "mode": mode} }

func pushClipPath(p []seg, rule string) op {
	return op{"op": "push_clip_path", "path": p, "rule": rule}
}

func pushMask(maskType string, ops ...op) op {
	return op{"op": "push_mask", "mask_type": maskType, "ops": ops}
}

func pushOpacity(opacity float32) op { return op{"op": "push_opacity", "opacity": opacity} }

func pushIsolated() op { return op{"op": "push_isolated"} }

func pop() op { return op{"op": "pop"} }

func drawImage(index int, w, h float32) op {
	return op{"op": "draw_image", "image": index, "width": w, "height": h}
}

func drawText(x, y float32, font int, size float32, text string) op {
	return op{"op": "draw_text", "start": []float32{x, y}, "font": font, "size": size, "text": text}
}

// glyph is a glyph of draw_glyphs. The advances and the offsets are in
// units of the size of the font, and the range is the byte range of its
// cluster in the text.
func glyph(id uint32, start, end int, xAdvance float32) map[string]any {
	return map[string]any{"id": id, "range": []int{start, end}, "x_advance": xAdvance}
}

func drawGlyphs(x, y float32, font int, size float32, text string, glyphs ...map[string]any) op {
	return op{"op": "draw_glyphs", "start": []float32{x, y}, "font": font, "size": size, "text": text, "glyphs": glyphs}
}

// drawSVG draws an SVG file with krilla-svg at a size.
func drawSVG(file string, w, h float32) op {
	return op{"op": "draw_svg", "file": file, "width": w, "height": h}
}
