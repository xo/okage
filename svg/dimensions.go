package svg

import (
	"bytes"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
	"github.com/xo/okage/roxmltree"
)

// This file ports no upstream file. It holds the API of D58 for a program
// that lays out SVG images, such as the layout of ivy: the size that an
// image declares, read before a parse, a tree from a node of roxmltree, and
// the option that accepts an image of size zero.

// Length is a length of SVG, a number and its unit. It is Length in
// svgtypes src/length.rs.
type Length = svgtypes.Length

// LengthUnit is the unit of a Length. It is LengthUnit in svgtypes
// src/length.rs.
type LengthUnit = svgtypes.LengthUnit

// The units of a Length.
const (
	// LengthUnitNone is a number with no unit, in user units.
	LengthUnitNone = svgtypes.LengthUnitNone
	// LengthUnitEm is a number of font sizes.
	LengthUnitEm = svgtypes.LengthUnitEm
	// LengthUnitEx is a number of x-heights, half a font size in usvg.
	LengthUnitEx = svgtypes.LengthUnitEx
	// LengthUnitPx is a number of pixels.
	LengthUnitPx = svgtypes.LengthUnitPx
	// LengthUnitIn is a number of inches.
	LengthUnitIn = svgtypes.LengthUnitIn
	// LengthUnitCm is a number of centimeters.
	LengthUnitCm = svgtypes.LengthUnitCm
	// LengthUnitMm is a number of millimeters.
	LengthUnitMm = svgtypes.LengthUnitMm
	// LengthUnitPt is a number of points.
	LengthUnitPt = svgtypes.LengthUnitPt
	// LengthUnitPc is a number of picas.
	LengthUnitPc = svgtypes.LengthUnitPc
	// LengthUnitPercent is a percentage.
	LengthUnitPercent = svgtypes.LengthUnitPercent
)

// Dimensions are the attributes width, height and viewBox of the root
// element of an SVG image, as the image declares them. A layout reads them
// to find the intrinsic size and the ratio of the image, as WeasyPrint does
// (D58). A Has field is false when the attribute is missing or not valid.
type Dimensions struct {
	// Width and Height are the attributes width and height, with their
	// units. The lengths are as the attributes give them, so a length can
	// be zero or negative.
	Width     Length
	HasWidth  bool
	Height    Length
	HasHeight bool

	// ViewBox is the attribute viewBox. A viewBox with a width or a height
	// that is not larger than zero is not valid, as in usvg.
	ViewBox    NonZeroRect
	HasViewBox bool
}

// ReadDimensions returns the Dimensions of SVG data, plain or compressed
// with gzip. It parses the XML only up to the start tag of the root element,
// with roxmltree.ParseRoot, so it costs much less than FromData for a large
// image. It reads the attributes of the root element alone, and no style
// sheet.
//
// It returns the errors of FromData for data that is not UTF-8, for XML
// text that it cannot parse up to the root element, and for a root element
// that is not svg. Data that it accepts can still fail in FromData.
func ReadDimensions(data []byte) (Dimensions, error) {
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		decoded, err := DecompressSVGZ(data)
		if err != nil {
			return Dimensions{}, err
		}
		data = decoded
	}
	if !utf8.Valid(data) {
		return Dimensions{}, okage.ErrSVGNotAnUTF8Str
	}
	xmlOpt := roxmltree.DefaultParsingOptions()
	xmlOpt.AllowDTD = true
	doc, err := roxmltree.ParseRoot(string(data), xmlOpt)
	if err != nil {
		return Dimensions{}, parsingFailed(err)
	}
	root := doc.RootElement()
	if eid, ok := parseTagName(root); !ok || eid != EIdSvg {
		return Dimensions{}, parsingFailed(okage.ErrXMLNoRootNode)
	}
	return NodeDimensions(root), nil
}

// NodeDimensions returns the Dimensions of an svg element that is already
// parsed, such as the root element of a document of roxmltree.
func NodeDimensions(node roxmltree.Node) Dimensions {
	var d Dimensions
	if v, ok := node.Attribute(roxmltree.ExpandedNameFromStr("width")); ok {
		length, err := svgtypes.ParseLength(v)
		d.Width, d.HasWidth = length, err == nil
	}
	if v, ok := node.Attribute(roxmltree.ExpandedNameFromStr("height")); ok {
		length, err := svgtypes.ParseLength(v)
		d.Height, d.HasHeight = length, err == nil
	}
	if v, ok := node.Attribute(roxmltree.ExpandedNameFromStr("viewBox")); ok {
		if vb, err := svgtypes.ParseViewBox(v); err == nil {
			d.ViewBox, d.HasViewBox = path.NonZeroRectFromXYWH(float32(vb.X), float32(vb.Y), float32(vb.W), float32(vb.H))
		}
	}
	return d
}

// FromXMLNode parses Tree from an svg element of a document of roxmltree,
// such as an inline svg element of an XHTML document. The element is the
// root of the image: the style sheets, the IDs and the links outside it do
// not apply. It copies the element and its descendants into a new document
// with roxmltree.Builder, and parses that document as FromXMLTree does.
func FromXMLNode(node roxmltree.Node, opts ...Option) (*Tree, error) {
	b := roxmltree.NewBuilder(roxmltree.DefaultParsingOptions())
	if !node.IsElement() {
		return nil, parsingFailed(okage.ErrXMLNoRootNode)
	}
	if err := b.Copy(node); err != nil {
		return nil, parsingFailed(err)
	}
	doc, err := b.Document()
	if err != nil {
		return nil, parsingFailed(err)
	}
	return fromXMLTree(doc, newOptions(opts...))
}

// WithZeroSize sets whether the parse accepts an image whose width or height
// is zero (D58). usvg returns okage.ErrSVGInvalidSize for such an image. With
// WithZeroSize(true), the parse returns a Tree with an empty root group, and
// Tree.Size returns the zero Size, so that a layout can lay the image out as
// an empty box. A size that is negative, or not finite, is still an error.
// The images that the image links to do not take the option.
//
// The Renderer returns okage.ErrSVGInvalidRenderSize for a Tree of the zero
// Size.
//
// Default: false, as in usvg.
func WithZeroSize(allow bool) Option {
	return func(o *options) {
		o.zeroSize = allow
	}
}

// isZeroSize reports whether a width and a height make the size of zero of
// WithZeroSize: both are finite and not negative, and one is zero.
func isZeroSize(width, height float32) bool {
	finite := func(v float32) bool { return v-v == 0 }
	return finite(width) && finite(height) && width >= 0 && height >= 0 && (width == 0 || height == 0)
}
