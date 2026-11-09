// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "slices"

// This file ports usvg src/tree/filter.rs, the module filter of usvg, which
// holds the SVG filter types.
//
// Port: the module filter becomes a part of the package svg. Its items keep
// their Rust names, except filter::Image, which collides with Image of usvg
// src/tree/mod.rs and becomes ImageFilter, with the name of its module as a
// suffix (D10).

// Filter is filter::Filter in usvg src/tree/filter.rs. It is a filter
// element.
//
// `filter` element in the SVG.
type Filter struct {
	id         nonEmptyString
	rect       NonZeroRect
	primitives []PrimitiveFilter
}

// ID is filter::Filter::id in usvg src/tree/filter.rs. It returns the
// element's ID.
//
// Taken from the SVG itself.
// Used only during SVG writing. `resvg` doesn't rely on this property.
func (f *Filter) ID() string {
	return f.id.get()
}

// Rect is filter::Filter::rect in usvg src/tree/filter.rs. It returns the
// filter region.
//
// `x`, `y`, `width` and `height` in the SVG.
func (f *Filter) Rect() NonZeroRect {
	return f.rect
}

// Primitives is filter::Filter::primitives in usvg src/tree/filter.rs. It
// returns a list of filter primitives.
func (f *Filter) Primitives() []PrimitiveFilter {
	return f.primitives
}

// PrimitiveFilter is filter::Primitive in usvg src/tree/filter.rs. It is a filter
// primitive element.
type PrimitiveFilter struct {
	rect               NonZeroRect
	colorInterpolation ColorInterpolationFilter
	result             string
	kind               KindFilter
}

// Rect is filter::Primitive::rect in usvg src/tree/filter.rs. It returns the
// filter subregion.
//
// `x`, `y`, `width` and `height` in the SVG.
func (p *PrimitiveFilter) Rect() NonZeroRect {
	return p.rect
}

// ColorInterpolation is filter::Primitive::color_interpolation in usvg
// src/tree/filter.rs. It returns the color interpolation mode.
//
// `color-interpolation-filters` in the SVG.
func (p *PrimitiveFilter) ColorInterpolation() ColorInterpolationFilter {
	return p.colorInterpolation
}

// Result is filter::Primitive::result in usvg src/tree/filter.rs. It returns
// the assigned name for this filter primitive.
//
// `result` in the SVG.
func (p *PrimitiveFilter) Result() string {
	return p.result
}

// Kind is filter::Primitive::kind in usvg src/tree/filter.rs. It returns the
// filter primitive kind.
func (p *PrimitiveFilter) Kind() KindFilter {
	return p.kind
}

// KindFilter is filter::Kind in usvg src/tree/filter.rs. It is a filter kind.
//
// Port: the cases hold different data, so the enum becomes an interface (D27).
// Its cases are the types *BlendFilter, *ColorMatrixFilter,
// *ComponentTransferFilter, *CompositeFilter, *ConvolveMatrixFilter,
// *DiffuseLightingFilter, *DisplacementMapFilter, *DropShadowFilter,
// *FloodFilter, *GaussianBlurFilter, *ImageFilter, *MergeFilter,
// *MorphologyFilter, *OffsetFilter, *SpecularLightingFilter, *TileFilter and
// *TurbulenceFilter, and a type switch takes the place of the Rust match.
type KindFilter interface {
	isKind()
}

// isKind makes *BlendFilter a case of KindFilter.
func (*BlendFilter) isKind() {}

// isKind makes *ColorMatrixFilter a case of KindFilter.
func (*ColorMatrixFilter) isKind() {}

// isKind makes *ComponentTransferFilter a case of KindFilter.
func (*ComponentTransferFilter) isKind() {}

// isKind makes *CompositeFilter a case of KindFilter.
func (*CompositeFilter) isKind() {}

// isKind makes *ConvolveMatrixFilter a case of KindFilter.
func (*ConvolveMatrixFilter) isKind() {}

// isKind makes *DiffuseLightingFilter a case of KindFilter.
func (*DiffuseLightingFilter) isKind() {}

// isKind makes *DisplacementMapFilter a case of KindFilter.
func (*DisplacementMapFilter) isKind() {}

// isKind makes *DropShadowFilter a case of KindFilter.
func (*DropShadowFilter) isKind() {}

// isKind makes *FloodFilter a case of KindFilter.
func (*FloodFilter) isKind() {}

// isKind makes *GaussianBlurFilter a case of KindFilter.
func (*GaussianBlurFilter) isKind() {}

// isKind makes *ImageFilter a case of KindFilter.
func (*ImageFilter) isKind() {}

// isKind makes *MergeFilter a case of KindFilter.
func (*MergeFilter) isKind() {}

// isKind makes *MorphologyFilter a case of KindFilter.
func (*MorphologyFilter) isKind() {}

// isKind makes *OffsetFilter a case of KindFilter.
func (*OffsetFilter) isKind() {}

// isKind makes *SpecularLightingFilter a case of KindFilter.
func (*SpecularLightingFilter) isKind() {}

// isKind makes *TileFilter a case of KindFilter.
func (*TileFilter) isKind() {}

// isKind makes *TurbulenceFilter a case of KindFilter.
func (*TurbulenceFilter) isKind() {}

// KindFilterHasInput is filter::Kind::has_input in usvg src/tree/filter.rs. It
// checks that `FilterKind` has a specific input.
func KindFilterHasInput(kind KindFilter, input InputFilter) bool {
	switch fe := kind.(type) {
	case *BlendFilter:
		return fe.input1 == input || fe.input2 == input
	case *ColorMatrixFilter:
		return fe.input == input
	case *ComponentTransferFilter:
		return fe.input == input
	case *CompositeFilter:
		return fe.input1 == input || fe.input2 == input
	case *ConvolveMatrixFilter:
		return fe.input == input
	case *DiffuseLightingFilter:
		return fe.input == input
	case *DisplacementMapFilter:
		return fe.input1 == input || fe.input2 == input
	case *DropShadowFilter:
		return fe.input == input
	case *FloodFilter:
		return false
	case *GaussianBlurFilter:
		return fe.input == input
	case *ImageFilter:
		return false
	case *MergeFilter:
		return slices.Contains(fe.inputs, input)
	case *MorphologyFilter:
		return fe.input == input
	case *OffsetFilter:
		return fe.input == input
	case *SpecularLightingFilter:
		return fe.input == input
	case *TileFilter:
		return fe.input == input
	case *TurbulenceFilter:
		return false
	}
	return false
}

// InputFilterKind is the case of an InputFilter.
type InputFilterKind uint8

// The cases of the enum filter::Input in usvg src/tree/filter.rs.
const (
	// InputFilterSourceGraphic is filter::Input::SourceGraphic.
	InputFilterSourceGraphic InputFilterKind = iota
	// InputFilterSourceAlpha is filter::Input::SourceAlpha.
	InputFilterSourceAlpha
	// InputFilterReference is filter::Input::Reference.
	InputFilterReference
)

// String returns the Rust name of the case.
func (k InputFilterKind) String() string {
	switch k {
	case InputFilterSourceGraphic:
		return "SourceGraphic" //nolint:goconst // an SVG keyword, as upstream writes it
	case InputFilterSourceAlpha:
		return "SourceAlpha" //nolint:goconst // an SVG keyword, as upstream writes it
	case InputFilterReference:
		return "Reference"
	}
	return "InputKind(?)"
}

// InputFilter is filter::Input in usvg src/tree/filter.rs. It identifies input for
// a filter primitive.
//
// Port: only the case Reference holds data, so the enum becomes a struct with
// a kind (D27). The == operator is the derived PartialEq of upstream, as long
// as Reference is empty for the other cases. The zero value is
// InputFilterSourceGraphic.
type InputFilter struct {
	Kind InputFilterKind
	// Reference is the name of InputFilterReference.
	Reference string
}

// ColorInterpolationFilter is filter::ColorInterpolation in usvg
// src/tree/filter.rs. It is a color interpolation mode.
//
// The default is `ColorInterpolationFilter::LinearRGB`, which
// DefaultColorInterpolationFilter returns.
type ColorInterpolationFilter uint8

// The values of ColorInterpolationFilter.
const (
	ColorInterpolationFilterSRGB ColorInterpolationFilter = iota
	ColorInterpolationFilterLinearRGB
)

// String returns the Rust name of the value.
func (c ColorInterpolationFilter) String() string {
	switch c {
	case ColorInterpolationFilterSRGB:
		return "SRGB"
	case ColorInterpolationFilterLinearRGB:
		return "LinearRGB"
	}
	return "ColorInterpolation(?)"
}

// DefaultColorInterpolationFilter is the derived Default for
// filter::ColorInterpolation in usvg src/tree/filter.rs.
func DefaultColorInterpolationFilter() ColorInterpolationFilter {
	return ColorInterpolationFilterLinearRGB
}

// BlendFilter is filter::Blend in usvg src/tree/filter.rs. It is a blend filter
// primitive.
//
// `feBlend` element in the SVG.
type BlendFilter struct {
	input1 InputFilter
	input2 InputFilter
	mode   BlendMode
}

// Input1 is filter::Blend::input1 in usvg src/tree/filter.rs. It identifies
// input for the given filter primitive.
//
// `in` in the SVG.
func (b *BlendFilter) Input1() InputFilter {
	return b.input1
}

// Input2 is filter::Blend::input2 in usvg src/tree/filter.rs. It identifies
// input for the given filter primitive.
//
// `in2` in the SVG.
func (b *BlendFilter) Input2() InputFilter {
	return b.input2
}

// Mode is filter::Blend::mode in usvg src/tree/filter.rs. It returns a
// blending mode.
//
// `mode` in the SVG.
func (b *BlendFilter) Mode() BlendMode {
	return b.mode
}

// ColorMatrixFilter is filter::ColorMatrix in usvg src/tree/filter.rs. It is a
// color matrix filter primitive.
//
// `feColorMatrix` element in the SVG.
type ColorMatrixFilter struct {
	input InputFilter
	kind  ColorMatrixKindFilter
}

// Input is filter::ColorMatrix::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (c *ColorMatrixFilter) Input() InputFilter {
	return c.input
}

// Kind is filter::ColorMatrix::kind in usvg src/tree/filter.rs. It returns a
// matrix kind.
//
// `type` in the SVG.
func (c *ColorMatrixFilter) Kind() ColorMatrixKindFilter {
	return c.kind
}

// ColorMatrixKindFilter is filter::ColorMatrixKind in usvg src/tree/filter.rs. It
// is a color matrix filter primitive kind.
//
// Port: the cases hold different data, so the enum becomes an interface (D27).
// Its cases are ColorMatrixKindFilterMatrix, ColorMatrixKindFilterSaturate,
// ColorMatrixKindFilterHueRotate and ColorMatrixKindFilterLuminanceToAlpha.
type ColorMatrixKindFilter interface {
	isColorMatrixKind()
}

// ColorMatrixKindFilterMatrix is filter::ColorMatrixKind::Matrix in usvg
// src/tree/filter.rs.
type ColorMatrixKindFilterMatrix []float32 // Guarantee to have 20 numbers.

// ColorMatrixKindFilterSaturate is filter::ColorMatrixKind::Saturate in usvg
// src/tree/filter.rs.
type ColorMatrixKindFilterSaturate PositiveF32

// ColorMatrixKindFilterHueRotate is filter::ColorMatrixKind::HueRotate in usvg
// src/tree/filter.rs.
type ColorMatrixKindFilterHueRotate float32

// ColorMatrixKindFilterLuminanceToAlpha is filter::ColorMatrixKind::LuminanceToAlpha
// in usvg src/tree/filter.rs.
type ColorMatrixKindFilterLuminanceToAlpha struct{}

// isColorMatrixKind makes ColorMatrixKindFilterMatrix a case of
// ColorMatrixKindFilter.
func (ColorMatrixKindFilterMatrix) isColorMatrixKind() {}

// isColorMatrixKind makes ColorMatrixKindFilterSaturate a case of
// ColorMatrixKindFilter.
func (ColorMatrixKindFilterSaturate) isColorMatrixKind() {}

// isColorMatrixKind makes ColorMatrixKindFilterHueRotate a case of
// ColorMatrixKindFilter.
func (ColorMatrixKindFilterHueRotate) isColorMatrixKind() {}

// isColorMatrixKind makes ColorMatrixKindFilterLuminanceToAlpha a case of
// ColorMatrixKindFilter.
func (ColorMatrixKindFilterLuminanceToAlpha) isColorMatrixKind() {}

// DefaultColorMatrixKindFilter is the impl Default for filter::ColorMatrixKind in
// usvg src/tree/filter.rs.
func DefaultColorMatrixKindFilter() ColorMatrixKindFilter {
	return ColorMatrixKindFilterMatrix{
		1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
	}
}

// ComponentTransferFilter is filter::ComponentTransfer in usvg src/tree/filter.rs.
// It is a component-wise remapping filter primitive.
//
// `feComponentTransfer` element in the SVG.
type ComponentTransferFilter struct {
	input InputFilter
	funcR TransferFunctionFilter
	funcG TransferFunctionFilter
	funcB TransferFunctionFilter
	funcA TransferFunctionFilter
}

// Input is filter::ComponentTransfer::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (c *ComponentTransferFilter) Input() InputFilter {
	return c.input
}

// FuncR is filter::ComponentTransfer::func_r in usvg src/tree/filter.rs.
//
// `feFuncR` in the SVG.
func (c *ComponentTransferFilter) FuncR() TransferFunctionFilter {
	return c.funcR
}

// FuncG is filter::ComponentTransfer::func_g in usvg src/tree/filter.rs.
//
// `feFuncG` in the SVG.
func (c *ComponentTransferFilter) FuncG() TransferFunctionFilter {
	return c.funcG
}

// FuncB is filter::ComponentTransfer::func_b in usvg src/tree/filter.rs.
//
// `feFuncB` in the SVG.
func (c *ComponentTransferFilter) FuncB() TransferFunctionFilter {
	return c.funcB
}

// FuncA is filter::ComponentTransfer::func_a in usvg src/tree/filter.rs.
//
// `feFuncA` in the SVG.
func (c *ComponentTransferFilter) FuncA() TransferFunctionFilter {
	return c.funcA
}

// TransferFunctionFilter is filter::TransferFunction in usvg src/tree/filter.rs.
// It is a transfer function used by `FeComponentTransfer`.
//
// <https://www.w3.org/TR/SVG11/filters.html#transferFuncElements>
//
// Port: the cases hold different data, so the enum becomes an interface (D27).
// Its cases are TransferFunctionFilterIdentity, TransferFunctionFilterTable,
// TransferFunctionFilterDiscrete, TransferFunctionFilterLinear and
// TransferFunctionFilterGamma.
type TransferFunctionFilter interface {
	isTransferFunction()
}

// TransferFunctionFilterIdentity is filter::TransferFunction::Identity in usvg
// src/tree/filter.rs. It keeps a component as is.
type TransferFunctionFilterIdentity struct{}

// TransferFunctionFilterTable is filter::TransferFunction::Table in usvg
// src/tree/filter.rs. It applies a linear interpolation to a component.
//
// The number list can be empty.
type TransferFunctionFilterTable []float32

// TransferFunctionFilterDiscrete is filter::TransferFunction::Discrete in usvg
// src/tree/filter.rs. It applies a step function to a component.
//
// The number list can be empty.
type TransferFunctionFilterDiscrete []float32

// TransferFunctionFilterLinear is filter::TransferFunction::Linear in usvg
// src/tree/filter.rs. It applies a linear shift to a component.
type TransferFunctionFilterLinear struct {
	Slope     float32
	Intercept float32
}

// TransferFunctionFilterGamma is filter::TransferFunction::Gamma in usvg
// src/tree/filter.rs. It applies an exponential shift to a component.
type TransferFunctionFilterGamma struct {
	Amplitude float32
	Exponent  float32
	Offset    float32
}

// isTransferFunction makes TransferFunctionFilterIdentity a case of
// TransferFunctionFilter.
func (TransferFunctionFilterIdentity) isTransferFunction() {}

// isTransferFunction makes TransferFunctionFilterTable a case of
// TransferFunctionFilter.
func (TransferFunctionFilterTable) isTransferFunction() {}

// isTransferFunction makes TransferFunctionFilterDiscrete a case of
// TransferFunctionFilter.
func (TransferFunctionFilterDiscrete) isTransferFunction() {}

// isTransferFunction makes TransferFunctionFilterLinear a case of
// TransferFunctionFilter.
func (TransferFunctionFilterLinear) isTransferFunction() {}

// isTransferFunction makes TransferFunctionFilterGamma a case of
// TransferFunctionFilter.
func (TransferFunctionFilterGamma) isTransferFunction() {}

// CompositeFilter is filter::Composite in usvg src/tree/filter.rs. It is a
// composite filter primitive.
//
// `feComposite` element in the SVG.
type CompositeFilter struct {
	input1   InputFilter
	input2   InputFilter
	operator CompositeOperatorFilter
}

// Input1 is filter::Composite::input1 in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (c *CompositeFilter) Input1() InputFilter {
	return c.input1
}

// Input2 is filter::Composite::input2 in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in2` in the SVG.
func (c *CompositeFilter) Input2() InputFilter {
	return c.input2
}

// Operator is filter::Composite::operator in usvg src/tree/filter.rs. It
// returns a compositing operation.
//
// `operator` in the SVG.
func (c *CompositeFilter) Operator() CompositeOperatorFilter {
	return c.operator
}

// CompositeOperatorFilterKind is the case of a CompositeOperatorFilter.
type CompositeOperatorFilterKind uint8

// The cases of the enum filter::CompositeOperator in usvg
// src/tree/filter.rs.
const (
	CompositeOperatorFilterOver CompositeOperatorFilterKind = iota
	CompositeOperatorFilterIn
	CompositeOperatorFilterOut
	CompositeOperatorFilterAtop
	CompositeOperatorFilterXor
	CompositeOperatorFilterLighter
	CompositeOperatorFilterArithmetic
)

// String returns the Rust name of the case.
func (k CompositeOperatorFilterKind) String() string {
	switch k {
	case CompositeOperatorFilterOver:
		return "Over"
	case CompositeOperatorFilterIn:
		return "In"
	case CompositeOperatorFilterOut:
		return "Out"
	case CompositeOperatorFilterAtop:
		return "Atop"
	case CompositeOperatorFilterXor:
		return "Xor"
	case CompositeOperatorFilterLighter:
		return "Lighter"
	case CompositeOperatorFilterArithmetic:
		return "Arithmetic"
	}
	return "CompositeOperatorKind(?)"
}

// CompositeOperatorFilter is filter::CompositeOperator in usvg src/tree/filter.rs.
// It is an images compositing operation.
//
// Port: only the case Arithmetic holds data, so the enum becomes a struct
// with a kind (D27). The == operator is the derived PartialEq of upstream, as
// long as K1 to K4 are zero for the other cases. The zero value is
// CompositeOperatorFilterOver.
type CompositeOperatorFilter struct {
	Kind CompositeOperatorFilterKind
	// K1, K2, K3 and K4 are the fields of CompositeOperatorFilterArithmetic.
	K1 float32
	K2 float32
	K3 float32
	K4 float32
}

// ConvolveMatrixFilter is filter::ConvolveMatrix in usvg src/tree/filter.rs. It is
// a matrix convolution filter primitive.
//
// `feConvolveMatrix` element in the SVG.
type ConvolveMatrixFilter struct {
	input         InputFilter
	matrix        ConvolveMatrixDataFilter
	divisor       NonZeroF32
	bias          float32
	edgeMode      EdgeModeFilter
	preserveAlpha bool
}

// Input is filter::ConvolveMatrix::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (c *ConvolveMatrixFilter) Input() InputFilter {
	return c.input
}

// Matrix is filter::ConvolveMatrix::matrix in usvg src/tree/filter.rs. It
// returns a convolve matrix.
func (c *ConvolveMatrixFilter) Matrix() *ConvolveMatrixDataFilter {
	return &c.matrix
}

// Divisor is filter::ConvolveMatrix::divisor in usvg src/tree/filter.rs. It
// returns a matrix divisor.
//
// `divisor` in the SVG.
func (c *ConvolveMatrixFilter) Divisor() NonZeroF32 {
	return c.divisor
}

// Bias is filter::ConvolveMatrix::bias in usvg src/tree/filter.rs. It returns
// a kernel matrix bias.
//
// `bias` in the SVG.
func (c *ConvolveMatrixFilter) Bias() float32 {
	return c.bias
}

// EdgeMode is filter::ConvolveMatrix::edge_mode in usvg src/tree/filter.rs.
// It returns an edges processing mode.
//
// `edgeMode` in the SVG.
func (c *ConvolveMatrixFilter) EdgeMode() EdgeModeFilter {
	return c.edgeMode
}

// PreserveAlpha is filter::ConvolveMatrix::preserve_alpha in usvg
// src/tree/filter.rs. It returns an alpha preserving flag.
//
// `preserveAlpha` in the SVG.
func (c *ConvolveMatrixFilter) PreserveAlpha() bool {
	return c.preserveAlpha
}

// ConvolveMatrixDataFilter is filter::ConvolveMatrixData in usvg
// src/tree/filter.rs. It is a convolve matrix representation.
//
// Used primarily by [`ConvolveMatrixFilter`].
type ConvolveMatrixDataFilter struct {
	targetX uint32
	targetY uint32
	columns uint32
	rows    uint32
	data    []float32
}

// TargetX is filter::ConvolveMatrixData::target_x in usvg
// src/tree/filter.rs. It returns a matrix's X target.
//
// `targetX` in the SVG.
func (d *ConvolveMatrixDataFilter) TargetX() uint32 {
	return d.targetX
}

// TargetY is filter::ConvolveMatrixData::target_y in usvg
// src/tree/filter.rs. It returns a matrix's Y target.
//
// `targetY` in the SVG.
func (d *ConvolveMatrixDataFilter) TargetY() uint32 {
	return d.targetY
}

// Columns is filter::ConvolveMatrixData::columns in usvg src/tree/filter.rs.
// It returns a number of columns in the matrix.
//
// Part of the `order` attribute in the SVG.
func (d *ConvolveMatrixDataFilter) Columns() uint32 {
	return d.columns
}

// Rows is filter::ConvolveMatrixData::rows in usvg src/tree/filter.rs. It
// returns a number of rows in the matrix.
//
// Part of the `order` attribute in the SVG.
func (d *ConvolveMatrixDataFilter) Rows() uint32 {
	return d.rows
}

// Data is filter::ConvolveMatrixData::data in usvg src/tree/filter.rs. It
// returns the actual matrix.
func (d *ConvolveMatrixDataFilter) Data() []float32 {
	return d.data
}

// newConvolveMatrixDataFilter is filter::ConvolveMatrixData::new in usvg
// src/tree/filter.rs. It creates a new `ConvolveMatrixDataFilter`.
//
// Returns `None` when:
//
// - `columns` * `rows` != `data.len()`
// - `target_x` >= `columns`
// - `target_y` >= `rows`.
func newConvolveMatrixDataFilter(
	targetX uint32,
	targetY uint32,
	columns uint32,
	rows uint32,
	data []float32,
) (ConvolveMatrixDataFilter, bool) {
	if uint(columns*rows) != uint(len(data)) || targetX >= columns || targetY >= rows {
		return ConvolveMatrixDataFilter{}, false
	}

	return ConvolveMatrixDataFilter{
		targetX: targetX,
		targetY: targetY,
		columns: columns,
		rows:    rows,
		data:    data,
	}, true
}

// Get is filter::ConvolveMatrixData::get in usvg src/tree/filter.rs. It
// returns a matrix value at the specified position.
//
// # Panics
//
// - When position is out of bounds.
func (d *ConvolveMatrixDataFilter) Get(x, y uint32) float32 {
	return d.data[uint(y*d.columns+x)]
}

// EdgeModeFilter is filter::EdgeMode in usvg src/tree/filter.rs. It is an edges
// processing mode.
type EdgeModeFilter uint8

// The values of EdgeModeFilter.
const (
	EdgeModeFilterNone EdgeModeFilter = iota
	EdgeModeFilterDuplicate
	EdgeModeFilterWrap
)

// String returns the Rust name of the value.
func (m EdgeModeFilter) String() string {
	switch m {
	case EdgeModeFilterNone:
		return "None"
	case EdgeModeFilterDuplicate:
		return "Duplicate"
	case EdgeModeFilterWrap:
		return "Wrap"
	}
	return "EdgeMode(?)"
}

// DisplacementMapFilter is filter::DisplacementMap in usvg src/tree/filter.rs. It
// is a displacement map filter primitive.
//
// `feDisplacementMap` element in the SVG.
type DisplacementMapFilter struct {
	input1           InputFilter
	input2           InputFilter
	scale            float32
	xChannelSelector ColorChannelFilter
	yChannelSelector ColorChannelFilter
}

// Input1 is filter::DisplacementMap::input1 in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (d *DisplacementMapFilter) Input1() InputFilter {
	return d.input1
}

// Input2 is filter::DisplacementMap::input2 in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in2` in the SVG.
func (d *DisplacementMapFilter) Input2() InputFilter {
	return d.input2
}

// Scale is filter::DisplacementMap::scale in usvg src/tree/filter.rs. It
// returns the scale factor.
//
// `scale` in the SVG.
func (d *DisplacementMapFilter) Scale() float32 {
	return d.scale
}

// XChannelSelector is filter::DisplacementMap::x_channel_selector in usvg
// src/tree/filter.rs. It indicates a source color channel along the X-axis.
//
// `xChannelSelector` in the SVG.
func (d *DisplacementMapFilter) XChannelSelector() ColorChannelFilter {
	return d.xChannelSelector
}

// YChannelSelector is filter::DisplacementMap::y_channel_selector in usvg
// src/tree/filter.rs. It indicates a source color channel along the Y-axis.
//
// `yChannelSelector` in the SVG.
func (d *DisplacementMapFilter) YChannelSelector() ColorChannelFilter {
	return d.yChannelSelector
}

// ColorChannelFilter is filter::ColorChannel in usvg src/tree/filter.rs. It is a
// color channel.
type ColorChannelFilter uint8

// The values of ColorChannelFilter.
const (
	ColorChannelFilterR ColorChannelFilter = iota
	ColorChannelFilterG
	ColorChannelFilterB
	ColorChannelFilterA
)

// String returns the Rust name of the value.
func (c ColorChannelFilter) String() string {
	switch c {
	case ColorChannelFilterR:
		return "R"
	case ColorChannelFilterG:
		return "G"
	case ColorChannelFilterB:
		return "B"
	case ColorChannelFilterA:
		return "A"
	}
	return "ColorChannel(?)"
}

// DropShadowFilter is filter::DropShadow in usvg src/tree/filter.rs. It is a drop
// shadow filter primitive.
//
// This is essentially `feGaussianBlur`, `feOffset` and `feFlood` joined together.
//
// `feDropShadow` element in the SVG.
type DropShadowFilter struct {
	input   InputFilter
	dx      float32
	dy      float32
	stdDevX PositiveF32
	stdDevY PositiveF32
	color   Color
	opacity Opacity
}

// Input is filter::DropShadow::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (d *DropShadowFilter) Input() InputFilter {
	return d.input
}

// Dx is filter::DropShadow::dx in usvg src/tree/filter.rs. It returns the
// amount to offset the input graphic along the X-axis.
func (d *DropShadowFilter) Dx() float32 {
	return d.dx
}

// Dy is filter::DropShadow::dy in usvg src/tree/filter.rs. It returns the
// amount to offset the input graphic along the Y-axis.
func (d *DropShadowFilter) Dy() float32 {
	return d.dy
}

// StdDevX is filter::DropShadow::std_dev_x in usvg src/tree/filter.rs. It
// returns a standard deviation along the X-axis.
//
// `stdDeviation` in the SVG.
func (d *DropShadowFilter) StdDevX() PositiveF32 {
	return d.stdDevX
}

// StdDevY is filter::DropShadow::std_dev_y in usvg src/tree/filter.rs. It
// returns a standard deviation along the Y-axis.
//
// `stdDeviation` in the SVG.
func (d *DropShadowFilter) StdDevY() PositiveF32 {
	return d.stdDevY
}

// Color is filter::DropShadow::color in usvg src/tree/filter.rs. It returns
// a flood color.
//
// `flood-color` in the SVG.
func (d *DropShadowFilter) Color() Color {
	return d.color
}

// Opacity is filter::DropShadow::opacity in usvg src/tree/filter.rs. It
// returns a flood opacity.
//
// `flood-opacity` in the SVG.
func (d *DropShadowFilter) Opacity() Opacity {
	return d.opacity
}

// FloodFilter is filter::Flood in usvg src/tree/filter.rs. It is a flood filter
// primitive.
//
// `feFlood` element in the SVG.
type FloodFilter struct {
	color   Color
	opacity Opacity
}

// Color is filter::Flood::color in usvg src/tree/filter.rs. It returns a
// flood color.
//
// `flood-color` in the SVG.
func (f *FloodFilter) Color() Color {
	return f.color
}

// Opacity is filter::Flood::opacity in usvg src/tree/filter.rs. It returns a
// flood opacity.
//
// `flood-opacity` in the SVG.
func (f *FloodFilter) Opacity() Opacity {
	return f.opacity
}

// GaussianBlurFilter is filter::GaussianBlur in usvg src/tree/filter.rs. It is a
// Gaussian blur filter primitive.
//
// `feGaussianBlur` element in the SVG.
type GaussianBlurFilter struct {
	input   InputFilter
	stdDevX PositiveF32
	stdDevY PositiveF32
}

// Input is filter::GaussianBlur::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (g *GaussianBlurFilter) Input() InputFilter {
	return g.input
}

// StdDevX is filter::GaussianBlur::std_dev_x in usvg src/tree/filter.rs. It
// returns a standard deviation along the X-axis.
//
// `stdDeviation` in the SVG.
func (g *GaussianBlurFilter) StdDevX() PositiveF32 {
	return g.stdDevX
}

// StdDevY is filter::GaussianBlur::std_dev_y in usvg src/tree/filter.rs. It
// returns a standard deviation along the Y-axis.
//
// `stdDeviation` in the SVG.
func (g *GaussianBlurFilter) StdDevY() PositiveF32 {
	return g.stdDevY
}

// ImageFilter is filter::Image in usvg src/tree/filter.rs. It is an image
// filter primitive.
//
// `feImage` element in the SVG.
type ImageFilter struct {
	root Group
}

// Root is filter::Image::root in usvg src/tree/filter.rs. It returns the
// `feImage` children.
func (i *ImageFilter) Root() *Group {
	return &i.root
}

// DiffuseLightingFilter is filter::DiffuseLighting in usvg src/tree/filter.rs. It
// is a diffuse lighting filter primitive.
//
// `feDiffuseLighting` element in the SVG.
type DiffuseLightingFilter struct {
	input           InputFilter
	surfaceScale    float32
	diffuseConstant float32
	lightingColor   Color
	lightSource     LightSourceFilter
}

// Input is filter::DiffuseLighting::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (d *DiffuseLightingFilter) Input() InputFilter {
	return d.input
}

// SurfaceScale is filter::DiffuseLighting::surface_scale in usvg
// src/tree/filter.rs. It returns a surface scale.
//
// `surfaceScale` in the SVG.
func (d *DiffuseLightingFilter) SurfaceScale() float32 {
	return d.surfaceScale
}

// DiffuseConstant is filter::DiffuseLighting::diffuse_constant in usvg
// src/tree/filter.rs. It returns a diffuse constant.
//
// `diffuseConstant` in the SVG.
func (d *DiffuseLightingFilter) DiffuseConstant() float32 {
	return d.diffuseConstant
}

// LightingColor is filter::DiffuseLighting::lighting_color in usvg
// src/tree/filter.rs. It returns a lighting color.
//
// `lighting-color` in the SVG.
func (d *DiffuseLightingFilter) LightingColor() Color {
	return d.lightingColor
}

// LightSource is filter::DiffuseLighting::light_source in usvg
// src/tree/filter.rs. It returns a light source.
func (d *DiffuseLightingFilter) LightSource() LightSourceFilter {
	return d.lightSource
}

// SpecularLightingFilter is filter::SpecularLighting in usvg src/tree/filter.rs. It
// is a specular lighting filter primitive.
//
// `feSpecularLighting` element in the SVG.
type SpecularLightingFilter struct {
	input            InputFilter
	surfaceScale     float32
	specularConstant float32
	specularExponent float32
	lightingColor    Color
	lightSource      LightSourceFilter
}

// Input is filter::SpecularLighting::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (s *SpecularLightingFilter) Input() InputFilter {
	return s.input
}

// SurfaceScale is filter::SpecularLighting::surface_scale in usvg
// src/tree/filter.rs. It returns a surface scale.
//
// `surfaceScale` in the SVG.
func (s *SpecularLightingFilter) SurfaceScale() float32 {
	return s.surfaceScale
}

// SpecularConstant is filter::SpecularLighting::specular_constant in usvg
// src/tree/filter.rs. It returns a specular constant.
//
// `specularConstant` in the SVG.
func (s *SpecularLightingFilter) SpecularConstant() float32 {
	return s.specularConstant
}

// SpecularExponent is filter::SpecularLighting::specular_exponent in usvg
// src/tree/filter.rs. It returns a specular exponent.
//
// Should be in 1..128 range.
//
// `specularExponent` in the SVG.
func (s *SpecularLightingFilter) SpecularExponent() float32 {
	return s.specularExponent
}

// LightingColor is filter::SpecularLighting::lighting_color in usvg
// src/tree/filter.rs. It returns a lighting color.
//
// `lighting-color` in the SVG.
func (s *SpecularLightingFilter) LightingColor() Color {
	return s.lightingColor
}

// LightSource is filter::SpecularLighting::light_source in usvg
// src/tree/filter.rs. It returns a light source.
func (s *SpecularLightingFilter) LightSource() LightSourceFilter {
	return s.lightSource
}

// LightSourceFilter is filter::LightSource in usvg src/tree/filter.rs. It is a
// light source kind.
//
// Port: the cases hold different data, so the enum becomes an interface (D27).
// Its cases are the values DistantLightFilter, PointLightFilter and
// SpotLightFilter, as in upstream, where the enum is Copy.
type LightSourceFilter interface {
	isLightSource()
}

// isLightSource makes DistantLightFilter a case of LightSourceFilter.
func (DistantLightFilter) isLightSource() {}

// isLightSource makes PointLightFilter a case of LightSourceFilter.
func (PointLightFilter) isLightSource() {}

// isLightSource makes SpotLightFilter a case of LightSourceFilter.
func (SpotLightFilter) isLightSource() {}

// DistantLightFilter is filter::DistantLight in usvg src/tree/filter.rs. It is a
// distant light source.
//
// `feDistantLight` element in the SVG.
type DistantLightFilter struct {
	// Direction angle for the light source on the XY plane (clockwise),
	// in degrees from the x axis.
	//
	// `azimuth` in the SVG.
	Azimuth float32

	// Direction angle for the light source from the XY plane towards the z axis, in degrees.
	//
	// `elevation` in the SVG.
	Elevation float32
}

// PointLightFilter is filter::PointLight in usvg src/tree/filter.rs. It is a point
// light source.
//
// `fePointLight` element in the SVG.
type PointLightFilter struct {
	// X location for the light source.
	//
	// `x` in the SVG.
	X float32

	// Y location for the light source.
	//
	// `y` in the SVG.
	Y float32

	// Z location for the light source.
	//
	// `z` in the SVG.
	Z float32
}

// SpotLightFilter is filter::SpotLight in usvg src/tree/filter.rs. It is a spot
// light source.
//
// `feSpotLight` element in the SVG.
type SpotLightFilter struct {
	// X location for the light source.
	//
	// `x` in the SVG.
	X float32

	// Y location for the light source.
	//
	// `y` in the SVG.
	Y float32

	// Z location for the light source.
	//
	// `z` in the SVG.
	Z float32

	// X point at which the light source is pointing.
	//
	// `pointsAtX` in the SVG.
	PointsAtX float32

	// Y point at which the light source is pointing.
	//
	// `pointsAtY` in the SVG.
	PointsAtY float32

	// Z point at which the light source is pointing.
	//
	// `pointsAtZ` in the SVG.
	PointsAtZ float32

	// Exponent value controlling the focus for the light source.
	//
	// `specularExponent` in the SVG.
	SpecularExponent PositiveF32

	// A limiting cone which restricts the region where the light is projected.
	//
	// `limitingConeAngle` in the SVG.
	//
	// Port: nil is the Rust None.
	LimitingConeAngle *float32
}

// MergeFilter is filter::Merge in usvg src/tree/filter.rs. It is a merge filter
// primitive.
//
// `feMerge` element in the SVG.
type MergeFilter struct {
	inputs []InputFilter
}

// Inputs is filter::Merge::inputs in usvg src/tree/filter.rs. It returns the
// list of input layers that should be merged.
//
// List of `feMergeNode`'s in the SVG.
func (m *MergeFilter) Inputs() []InputFilter {
	return m.inputs
}

// MorphologyFilter is filter::Morphology in usvg src/tree/filter.rs. It is a
// morphology filter primitive.
//
// `feMorphology` element in the SVG.
type MorphologyFilter struct {
	input    InputFilter
	operator MorphologyOperatorFilter
	radiusX  PositiveF32
	radiusY  PositiveF32
}

// Input is filter::Morphology::input in usvg src/tree/filter.rs. It
// identifies input for the given filter primitive.
//
// `in` in the SVG.
func (m *MorphologyFilter) Input() InputFilter {
	return m.input
}

// Operator is filter::Morphology::operator in usvg src/tree/filter.rs. It
// returns a filter operator.
//
// `operator` in the SVG.
func (m *MorphologyFilter) Operator() MorphologyOperatorFilter {
	return m.operator
}

// RadiusX is filter::Morphology::radius_x in usvg src/tree/filter.rs. It
// returns a filter radius along the X-axis.
//
// A value of zero disables the effect of the given filter primitive.
//
// `radius` in the SVG.
func (m *MorphologyFilter) RadiusX() PositiveF32 {
	return m.radiusX
}

// RadiusY is filter::Morphology::radius_y in usvg src/tree/filter.rs. It
// returns a filter radius along the Y-axis.
//
// A value of zero disables the effect of the given filter primitive.
//
// `radius` in the SVG.
func (m *MorphologyFilter) RadiusY() PositiveF32 {
	return m.radiusY
}

// MorphologyOperatorFilter is filter::MorphologyOperator in usvg
// src/tree/filter.rs. It is a morphology operation.
type MorphologyOperatorFilter uint8

// The values of MorphologyOperatorFilter.
const (
	MorphologyOperatorFilterErode MorphologyOperatorFilter = iota
	MorphologyOperatorFilterDilate
)

// String returns the Rust name of the value.
func (o MorphologyOperatorFilter) String() string {
	switch o {
	case MorphologyOperatorFilterErode:
		return "Erode"
	case MorphologyOperatorFilterDilate:
		return "Dilate"
	}
	return "MorphologyOperator(?)"
}

// OffsetFilter is filter::Offset in usvg src/tree/filter.rs. It is an offset
// filter primitive.
//
// `feOffset` element in the SVG.
type OffsetFilter struct {
	input InputFilter
	dx    float32
	dy    float32
}

// Input is filter::Offset::input in usvg src/tree/filter.rs. It identifies
// input for the given filter primitive.
//
// `in` in the SVG.
func (o *OffsetFilter) Input() InputFilter {
	return o.input
}

// Dx is filter::Offset::dx in usvg src/tree/filter.rs. It returns the amount
// to offset the input graphic along the X-axis.
func (o *OffsetFilter) Dx() float32 {
	return o.dx
}

// Dy is filter::Offset::dy in usvg src/tree/filter.rs. It returns the amount
// to offset the input graphic along the Y-axis.
func (o *OffsetFilter) Dy() float32 {
	return o.dy
}

// TileFilter is filter::Tile in usvg src/tree/filter.rs. It is a tile filter
// primitive.
//
// `feTile` element in the SVG.
type TileFilter struct {
	input InputFilter
}

// Input is filter::Tile::input in usvg src/tree/filter.rs. It identifies
// input for the given filter primitive.
//
// `in` in the SVG.
func (t *TileFilter) Input() InputFilter {
	return t.input
}

// TurbulenceFilter is filter::Turbulence in usvg src/tree/filter.rs. It is a
// turbulence generation filter primitive.
//
// `feTurbulence` element in the SVG.
type TurbulenceFilter struct {
	baseFrequencyX PositiveF32
	baseFrequencyY PositiveF32
	numOctaves     uint32
	seed           int32
	stitchTiles    bool
	kind           TurbulenceKindFilter
}

// BaseFrequencyX is filter::Turbulence::base_frequency_x in usvg
// src/tree/filter.rs. It identifies the base frequency for the noise
// function.
//
// `baseFrequency` in the SVG.
func (t *TurbulenceFilter) BaseFrequencyX() PositiveF32 {
	return t.baseFrequencyX
}

// BaseFrequencyY is filter::Turbulence::base_frequency_y in usvg
// src/tree/filter.rs. It identifies the base frequency for the noise
// function.
//
// `baseFrequency` in the SVG.
func (t *TurbulenceFilter) BaseFrequencyY() PositiveF32 {
	return t.baseFrequencyY
}

// NumOctaves is filter::Turbulence::num_octaves in usvg src/tree/filter.rs.
// It identifies the number of octaves for the noise function.
//
// `numOctaves` in the SVG.
func (t *TurbulenceFilter) NumOctaves() uint32 {
	return t.numOctaves
}

// Seed is filter::Turbulence::seed in usvg src/tree/filter.rs. It returns the
// starting number for the pseudo random number generator.
//
// `seed` in the SVG.
func (t *TurbulenceFilter) Seed() int32 {
	return t.seed
}

// StitchTiles is filter::Turbulence::stitch_tiles in usvg src/tree/filter.rs.
// It reports smooth transitions at the border of tiles.
//
// `stitchTiles` in the SVG.
func (t *TurbulenceFilter) StitchTiles() bool {
	return t.stitchTiles
}

// Kind is filter::Turbulence::kind in usvg src/tree/filter.rs. It indicates
// whether the filter primitive should perform a noise or turbulence function.
//
// `type` in the SVG.
func (t *TurbulenceFilter) Kind() TurbulenceKindFilter {
	return t.kind
}

// TurbulenceKindFilter is filter::TurbulenceKind in usvg src/tree/filter.rs. It is
// a turbulence kind for the `feTurbulence` filter.
type TurbulenceKindFilter uint8

// The values of TurbulenceKindFilter.
const (
	TurbulenceKindFilterFractalNoise TurbulenceKindFilter = iota
	TurbulenceKindFilterTurbulence
)

// String returns the Rust name of the value.
func (k TurbulenceKindFilter) String() string {
	switch k {
	case TurbulenceKindFilterFractalNoise:
		return "FractalNoise"
	case TurbulenceKindFilterTurbulence:
		return "Turbulence"
	}
	return "TurbulenceKind(?)"
}
