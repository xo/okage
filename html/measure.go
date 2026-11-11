package html

// This file ports no upstream file. It holds the options of Layout and the
// default measure of text, which calls svg (D38).

import (
	"strconv"
	"strings"
	"sync"

	"github.com/xo/okage/svg"
)

// TextStyle is the font of a run of text.
type TextStyle struct {
	// Family is the family of the font, as the face of a <font> gives it, or
	// empty for the default family of the measure.
	Family string

	// Size is the size of the font in points.
	Size float64

	Bold   bool
	Italic bool
}

// Metrics is the size of a run of text, in points.
type Metrics struct {
	// Width is the advance width of the run.
	Width float64

	// Height is the height of a line of the run.
	Height float64

	// Ascent is the distance from the top of the line to the baseline.
	Ascent float64
}

// Measure returns the metrics of a run of text in a style.
type Measure func(text string, style TextStyle) (Metrics, error)

// Option is an option of Label.Layout.
type Option func(*options)

// options holds the options of Label.Layout.
type options struct {
	measure  Measure
	renderer *svg.Renderer
	style    TextStyle
}

// WithMeasure sets the measure of text. It overrides WithRenderer.
func WithMeasure(m Measure) Option {
	return func(o *options) {
		o.measure = m
	}
}

// WithRenderer sets the renderer that the default measure shapes text with,
// with its fonts and its generic families (D36).
func WithRenderer(r *svg.Renderer) Option {
	return func(o *options) {
		o.renderer = r
	}
}

// WithTextStyle sets the font of text with no <font> around it. An empty
// family is the default family of the measure, and a size of 0 is 14 points,
// as in Graphviz. Bold and Italic are ignored: the tags b and i set them.
func WithTextStyle(style TextStyle) Option {
	return func(o *options) {
		o.style = style
	}
}

// defaultSize is the size of the font of Graphviz.
const defaultSize = 14

// defaultRenderer is the renderer of the default measure when no option
// gives one. It loads the fonts of the system on its first call (D36).
var defaultRenderer = sync.OnceValue(func() *svg.Renderer { return svg.New() })

// newOptions applies the options.
func newOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	if o.style.Size <= 0 {
		o.style.Size = defaultSize
	}
	o.style.Bold, o.style.Italic = false, false
	if o.measure == nil {
		r := o.renderer
		if r == nil {
			r = defaultRenderer()
		}
		o.measure = rendererMeasure(r)
	}
	return o
}

// rendererMeasure returns a measure that shapes text with r. The height and
// the ascent come from the metrics of the glyphs, as svg.TextRun.Bounds gives
// them. The fallback of svg finds a font for each character.
func rendererMeasure(r *svg.Renderer) Measure {
	return func(text string, style TextStyle) (Metrics, error) {
		st := svg.TextStyle{Family: cssFamily(style.Family), Size: float32(style.Size)}
		if style.Bold {
			st.Weight = 700
		}
		if style.Italic {
			st.Style = svg.FontStyleItalic
		}
		run, err := r.ShapeText(text, st)
		if err != nil {
			return Metrics{}, err //nolint:wrapcheck // Layout wraps it.
		}
		b := run.Bounds()
		return Metrics{Width: float64(run.Width()), Height: float64(b.Height()), Ascent: float64(-b.Y())}, nil
	}
}

// cssFamily quotes a face as a family of CSS. An empty face is the default
// family.
func cssFamily(face string) string {
	if face == "" {
		return ""
	}
	if strings.ContainsAny(face, `,"'`) {
		return face
	}
	return strconv.Quote(face)
}
