// Copyright 2022 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"strconv"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/filter.rs, a collection of SVG filters.

// fromValueColorInterpolationFilter is the impl FromValue for
// filter::ColorInterpolation in usvg src/parser/filter.rs.
func fromValueColorInterpolationFilter(_ svgNode, _ AId, value string) (ColorInterpolationFilter, bool) {
	switch value {
	case "sRGB":
		return ColorInterpolationFilterSRGB, true
	case "linearRGB": //nolint:goconst // an SVG keyword, as upstream writes it
		return ColorInterpolationFilterLinearRGB, true
	}
	return 0, false
}

// convertFilter is convert in usvg src/parser/filter.rs.
//
// Port: upstream returns Result<Vec<Arc<Filter>>, ()>. Go returns false for
// the error, which carries no data. A nil objectBBox is None.
func convertFilter(
	node svgNode,
	state *converterState,
	objectBBox *NonZeroRect,
	cache *converterCache,
) ([]*Filter, bool) {
	value, ok := node.attributeStr(AIdFilter)
	if !ok {
		return nil, true
	}

	hasInvalidUrls := false
	var filters []*Filter

	createBaseFilterFunc := func(kind KindFilter) {
		// Filter functions, unlike `filter` elements, do not have a filter region.
		// We're currently do not support an unlimited region, so we simply use a fairly large one.
		// This if far from ideal, but good for now.
		// TODO: Should be fixed eventually.
		var rect NonZeroRect
		switch kind.(type) {
		case *DropShadowFilter, *GaussianBlurFilter:
			rect = mustNonZeroRect(path.NonZeroRectFromXYWH(-0.5, -0.5, 2.0, 2.0))
		default:
			rect = mustNonZeroRect(path.NonZeroRectFromXYWH(-0.1, -0.1, 1.2, 1.2))
		}

		if objectBBox == nil {
			state.opt.warnf(
				"Filter '%s' has an invalid region. Skipped.",
				node.elementID(),
			)
			return
		}

		rect = rect.BBoxTransform(*objectBBox)

		filters = append(filters, &Filter{
			id:   cache.genFilterID(),
			rect: rect,
			primitives: []PrimitiveFilter{{
				rect: rect,
				// Unlike `filter` elements, filter functions use sRGB colors by default.
				colorInterpolation: ColorInterpolationFilterSRGB,
				result:             "result",
				kind:               kind,
			}},
		})
	}

	for fn, err := range svgtypes.NewFilterValueListParser(value).All() {
		if err != nil {
			// Skip the whole attribute list on error.
			state.opt.warnf("Failed to parse a filter value cause %s. Skipping.", err)
			return nil, true
		}

		switch fn.Kind {
		case svgtypes.FilterValueBlur:
			createBaseFilterFunc(convertBlurFunction(node, fn.Length, state))
		case svgtypes.FilterValueDropShadow:
			var color *svgtypes.Color
			if fn.HasColor {
				c := fn.Color
				color = &c
			}
			createBaseFilterFunc(convertDropShadowFunction(node, color, fn.Dx, fn.Dy, fn.StdDev, state))
		case svgtypes.FilterValueBrightness:
			createBaseFilterFunc(convertBrightnessFunction(fn.Number))
		case svgtypes.FilterValueContrast:
			createBaseFilterFunc(convertContrastFunction(fn.Number))
		case svgtypes.FilterValueGrayscale:
			createBaseFilterFunc(convertGrayscaleFunction(fn.Number))
		case svgtypes.FilterValueHueRotate:
			createBaseFilterFunc(convertHueRotateFunction(fn.Angle))
		case svgtypes.FilterValueInvert:
			createBaseFilterFunc(convertInvertFunction(fn.Number))
		case svgtypes.FilterValueOpacity:
			createBaseFilterFunc(convertOpacityFunction(fn.Number))
		case svgtypes.FilterValueSepia:
			createBaseFilterFunc(convertSepiaFunction(fn.Number))
		case svgtypes.FilterValueSaturate:
			createBaseFilterFunc(convertSaturateFunction(fn.Number))
		case svgtypes.FilterValueURL:
			if link, ok := node.document().elementByID(fn.URL); ok {
				if f, ok := convertURL(link, state, objectBBox, cache); ok {
					if f != nil {
						filters = append(filters, f)
					}
				} else {
					hasInvalidUrls = true
				}
			} else {
				hasInvalidUrls = true
			}
		}
	}

	// If a `filter` attribute had urls pointing to a missing elements
	// and there are no valid filters at all - this is an error.
	//
	// Note that an invalid url is not an error in general.
	if len(filters) == 0 && hasInvalidUrls {
		return nil, false
	}

	return filters, true
}

// convertURL is convert_url in usvg src/parser/filter.rs.
//
// Port: upstream returns Result<Option<Arc<Filter>>, ()>. Go returns false
// for the error, and a nil filter for None.
func convertURL(
	node svgNode,
	state *converterState,
	objectBBox *NonZeroRect,
	cache *converterCache,
) (*Filter, bool) {
	units := convertUnits(node, AIdFilterUnits, unitsObjectBoundingBox)
	primitiveUnits := convertUnits(node, AIdPrimitiveUnits, unitsUserSpaceOnUse)

	// Check if this element was already converted.
	//
	// Only `userSpaceOnUse` clipPaths can be shared,
	// because `objectBoundingBox` one will be converted into user one
	// and will become node-specific.
	cacheable := units == unitsUserSpaceOnUse && primitiveUnits == unitsUserSpaceOnUse
	if cacheable {
		if filter, ok := cache.filters[node.elementID()]; ok {
			return filter, true
		}
	}

	rect, ok := path.NonZeroRectFromXYWH(
		resolveNumber(
			node,
			AIdX,
			units,
			state,
			svgtypes.NewLength(-10.0, svgtypes.LengthUnitPercent),
		),
		resolveNumber(
			node,
			AIdY,
			units,
			state,
			svgtypes.NewLength(-10.0, svgtypes.LengthUnitPercent),
		),
		resolveNumber(
			node,
			AIdWidth,
			units,
			state,
			svgtypes.NewLength(120.0, svgtypes.LengthUnitPercent),
		),
		resolveNumber(
			node,
			AIdHeight,
			units,
			state,
			svgtypes.NewLength(120.0, svgtypes.LengthUnitPercent),
		),
	)

	rect, ok = logNone(rect, ok, func() {
		state.opt.warnf(
			"Filter '%s' has an invalid region. Skipped.",
			node.elementID(),
		)
	})
	if !ok {
		return nil, false
	}

	if units == unitsObjectBoundingBox {
		if objectBBox != nil {
			rect = rect.BBoxTransform(*objectBBox)
		} else {
			state.opt.warnf("Filters on zero-sized shapes are not allowed.")
			return nil, false
		}
	}

	nodeWithPrimitives, ok := findFilterWithPrimitives(node)
	if !ok {
		return nil, false
	}
	primitives := collectChildren(
		nodeWithPrimitives,
		primitiveUnits,
		state,
		objectBBox,
		rect,
		cache,
	)
	if len(primitives) == 0 {
		return nil, false
	}

	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return nil, false
	}
	// Generate ID only when we're parsing `objectBoundingBox` filter for the second time.
	if _, ok := cache.filters[id.get()]; !cacheable && ok {
		id = cache.genFilterID()
	}
	idCopy := id.get()

	filter := &Filter{
		id:         id,
		rect:       rect,
		primitives: primitives,
	}

	cache.filters[idCopy] = filter

	return filter, true
}

// findFilterWithPrimitives is find_filter_with_primitives in usvg
// src/parser/filter.rs.
func findFilterWithPrimitives(node svgNode) (svgNode, bool) {
	for link := range node.hrefIter() {
		if !link.hasTagName(EIdFilter) {
			tagName, ok := link.tagName()
			if !ok {
				panic("svg: called Option::unwrap() on a None value")
			}
			node.doc.opt.warnf(
				"Filter '%s' cannot reference '%s' via 'xlink:href'.",
				node.elementID(),
				tagName,
			)
			return svgNode{}, false
		}

		if link.hasChildren() {
			return link, true
		}
	}

	return svgNode{}, false
}

// filterResults is FilterResults in usvg src/parser/filter.rs.
type filterResults struct {
	names map[string]struct{}
	idx   int
}

// collectChildren is collect_children in usvg src/parser/filter.rs.
func collectChildren(
	filter svgNode,
	units units,
	state *converterState,
	objectBBox *NonZeroRect,
	filterRegion NonZeroRect,
	cache *converterCache,
) []PrimitiveFilter {
	var primitives []PrimitiveFilter

	results := &filterResults{
		names: map[string]struct{}{},
		idx:   1,
	}

	var scale Size
	if units == unitsObjectBoundingBox {
		if objectBBox == nil {
			// No need to warn. Already checked.
			return nil
		}
		scale = objectBBox.Size()
	} else {
		scale = mustSize(path.SizeFromWH(1.0, 1.0))
	}

	for child := range filter.children() {
		tagName, ok := child.tagName()
		if !ok {
			continue
		}

		filterSubregion, ok := resolvePrimitiveRegion(child, units, state, objectBBox, filterRegion)
		if !ok {
			break
		}

		var kind KindFilter
		switch tagName {
		case EIdFeDropShadow:
			kind = convertDropShadow(child, scale, primitives)
		case EIdFeGaussianBlur:
			kind = convertGaussianBlur(child, scale, primitives)
		case EIdFeOffset:
			kind = convertOffset(child, scale, primitives)
		case EIdFeBlend:
			kind = convertBlend(child, primitives)
		case EIdFeFlood:
			kind = convertFlood(child)
		case EIdFeComposite:
			kind = convertComposite(child, primitives)
		case EIdFeMerge:
			kind = convertMerge(child, primitives)
		case EIdFeTile:
			kind = convertTile(child, primitives)
		case EIdFeImage:
			kind = convertImageFilter(child, filterSubregion, state, cache)
		case EIdFeComponentTransfer:
			kind = convertComponentTransfer(child, primitives)
		case EIdFeColorMatrix:
			kind = convertColorMatrix(child, primitives)
		case EIdFeConvolveMatrix:
			if kind, ok = convertConvolveMatrix(child, primitives); !ok {
				kind = createDummyPrimitive()
			}
		case EIdFeMorphology:
			kind = convertMorphology(child, scale, primitives)
		case EIdFeDisplacementMap:
			kind = convertDisplacementMap(child, scale, primitives)
		case EIdFeTurbulence:
			kind = convertTurbulence(child)
		case EIdFeDiffuseLighting:
			if kind, ok = convertDiffuseLighting(child, primitives); !ok {
				kind = createDummyPrimitive()
			}
		case EIdFeSpecularLighting:
			if kind, ok = convertSpecularLighting(child, primitives); !ok {
				kind = createDummyPrimitive()
			}
		default:
			state.opt.warnf("'%s' is not a valid filter primitive. Skipped.", tagName)
			continue
		}

		colorInterpolation, ok := nodeFindAttribute(child, AIdColorInterpolationFilters, fromValueColorInterpolationFilter)
		if !ok {
			colorInterpolation = DefaultColorInterpolationFilter()
		}

		primitives = append(primitives, PrimitiveFilter{
			rect:               filterSubregion,
			colorInterpolation: colorInterpolation,
			result:             genResult(child, results),
			kind:               kind,
		})
	}

	// TODO: remove primitives which results are not used

	return primitives
}

// resolvePrimitiveRegion is resolve_primitive_region in usvg
// src/parser/filter.rs.
func resolvePrimitiveRegion(
	fe svgNode,
	units units,
	state *converterState,
	bbox *NonZeroRect,
	filterRegion NonZeroRect,
) (NonZeroRect, bool) {
	x, hasX := fe.tryConvertLength(AIdX, units, state)
	y, hasY := fe.tryConvertLength(AIdY, units, state)
	width, hasWidth := fe.tryConvertLength(AIdWidth, units, state)
	height, hasHeight := fe.tryConvertLength(AIdHeight, units, state)

	if units == unitsObjectBoundingBox {
		if bbox == nil {
			return NonZeroRect{}, false
		}
		if hasX {
			x = float32(x*bbox.Width()) + bbox.X()
		}
		if hasY {
			y = float32(y*bbox.Height()) + bbox.Y()
		}
		if hasWidth {
			width *= bbox.Width()
		}
		if hasHeight {
			height *= bbox.Height()
		}
	}

	or := func(v float32, ok bool, def float32) float32 {
		if ok {
			return v
		}
		return def
	}
	return path.NonZeroRectFromXYWH(
		or(x, hasX, filterRegion.X()),
		or(y, hasY, filterRegion.Y()),
		or(width, hasWidth, filterRegion.Width()),
		or(height, hasHeight, filterRegion.Height()),
	)
}

// createDummyPrimitive is create_dummy_primitive in usvg
// src/parser/filter.rs.
//
// A malformed filter primitive usually should produce a transparent image.
// But since `FilterKind` structs are designed to always be valid,
// we are using `FeFlood` as fallback.
func createDummyPrimitive() KindFilter {
	return &FloodFilter{
		color:   ColorBlack(),
		opacity: 0,
	}
}

// resolveInput is resolve_input in usvg src/parser/filter.rs.
func resolveInput(node svgNode, aid AId, primitives []PrimitiveFilter) InputFilter {
	if s, ok := node.attributeStr(aid); ok {
		input := parseIn(s, node.doc.opt)

		// If `in` references an unknown `result` than fallback
		// to previous result or `SourceGraphic`.
		if input.Kind == InputFilterReference {
			found := false
			for i := range primitives {
				if primitives[i].result == input.Reference {
					found = true
					break
				}
			}
			if !found {
				if len(primitives) > 0 {
					return InputFilter{Kind: InputFilterReference, Reference: primitives[len(primitives)-1].result}
				}
				return InputFilter{Kind: InputFilterSourceGraphic}
			}
		}

		return input
	}
	if len(primitives) > 0 {
		// If `in` is not set and this is not the first primitive
		// than the input is a result of the previous primitive.
		return InputFilter{Kind: InputFilterReference, Reference: primitives[len(primitives)-1].result}
	}
	// If `in` is not set and this is the first primitive
	// than the input is `SourceGraphic`.
	return InputFilter{Kind: InputFilterSourceGraphic}
}

// parseIn is parse_in in usvg src/parser/filter.rs.
//
// Port: Go takes the options for the logger (D20).
func parseIn(s string, opt *options) InputFilter {
	switch s {
	case "SourceGraphic": //nolint:goconst // an SVG keyword, as upstream writes it
		return InputFilter{Kind: InputFilterSourceGraphic}
	case "SourceAlpha": //nolint:goconst // an SVG keyword, as upstream writes it
		return InputFilter{Kind: InputFilterSourceAlpha}
	case "BackgroundImage", "BackgroundAlpha", "FillPaint", "StrokePaint":
		opt.warnf("%s filter input isn't supported and not planed.", s)
		return InputFilter{Kind: InputFilterSourceGraphic}
	}
	return InputFilter{Kind: InputFilterReference, Reference: s}
}

// genResult is gen_result in usvg src/parser/filter.rs.
func genResult(node svgNode, results *filterResults) string {
	if s, ok := node.attributeStr(AIdResult); ok {
		// Remember predefined result.
		results.names[s] = struct{}{}
		results.idx++

		return s
	}
	// Generate an unique name for `result`.
	for {
		name := "result" + strconv.Itoa(results.idx)
		results.idx++

		if _, ok := results.names[name]; !ok {
			return name
		}
	}
}

// convertBlend is convert_blend in usvg src/parser/filter.rs.
func convertBlend(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	mode, _ := nodeAttribute(fe, AIdMode, fromValueBlendMode)
	input1 := resolveInput(fe, AIdIn, primitives)
	input2 := resolveInput(fe, AIdIn2, primitives)
	return &BlendFilter{
		mode:   mode,
		input1: input1,
		input2: input2,
	}
}

// convertColorMatrix is convert_color_matrix in usvg src/parser/filter.rs.
func convertColorMatrix(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	kind, ok := convertColorMatrixKind(fe)
	if !ok {
		kind = DefaultColorMatrixKindFilter()
	}
	return &ColorMatrixFilter{
		input: resolveInput(fe, AIdIn, primitives),
		kind:  kind,
	}
}

// convertColorMatrixKind is convert_color_matrix_kind in usvg
// src/parser/filter.rs.
func convertColorMatrixKind(fe svgNode) (ColorMatrixKindFilter, bool) {
	typ, hasType := fe.attributeStr(AIdType)
	switch {
	case hasType && typ == "saturate":
		if list, ok := nodeAttribute(fe, AIdValues, fromValueNumberList); ok {
			if len(list) != 0 {
				n := f32Bound(0.0, list[0], 1.0)
				return ColorMatrixKindFilterSaturate(mustPositiveF32(path.NewPositiveF32(n))), true
			}
			return ColorMatrixKindFilterSaturate(mustPositiveF32(path.NewPositiveF32(1.0))), true
		}
	case hasType && typ == "hueRotate":
		if list, ok := nodeAttribute(fe, AIdValues, fromValueNumberList); ok {
			if len(list) != 0 {
				return ColorMatrixKindFilterHueRotate(list[0]), true
			}
			return ColorMatrixKindFilterHueRotate(0.0), true
		}
	case hasType && typ == "luminanceToAlpha":
		return ColorMatrixKindFilterLuminanceToAlpha{}, true
	default:
		// Fallback to `matrix`.
		if list, ok := nodeAttribute(fe, AIdValues, fromValueNumberList); ok {
			if len(list) == 20 {
				return ColorMatrixKindFilterMatrix(list), true
			}
		}
	}

	return nil, false
}

// mustPositiveF32 is the unwrap of an Option<PositiveF32> that upstream
// relies on. It panics when ok is false.
func mustPositiveF32(n PositiveF32, ok bool) PositiveF32 {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return n
}

// convertComponentTransfer is convert_component_transfer in usvg
// src/parser/filter.rs.
func convertComponentTransfer(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	kind := &ComponentTransferFilter{
		input: resolveInput(fe, AIdIn, primitives),
		funcR: TransferFunctionFilterIdentity{},
		funcG: TransferFunctionFilterIdentity{},
		funcB: TransferFunctionFilterIdentity{},
		funcA: TransferFunctionFilterIdentity{},
	}

	for child := range fe.children() {
		if !child.isElement() {
			continue
		}
		if fn, ok := convertTransferFunction(child); ok {
			tagName, ok := child.tagName()
			if !ok {
				panic("svg: called Option::unwrap() on a None value")
			}
			switch tagName {
			case EIdFeFuncR:
				kind.funcR = fn
			case EIdFeFuncG:
				kind.funcG = fn
			case EIdFeFuncB:
				kind.funcB = fn
			case EIdFeFuncA:
				kind.funcA = fn
			}
		}
	}

	return kind
}

// convertTransferFunction is convert_transfer_function in usvg
// src/parser/filter.rs.
func convertTransferFunction(node svgNode) (TransferFunctionFilter, bool) {
	typ, ok := node.attributeStr(AIdType)
	if !ok {
		return nil, false
	}
	switch typ {
	case "identity":
		return TransferFunctionFilterIdentity{}, true
	case "table":
		if values, ok := nodeAttribute(node, AIdTableValues, fromValueNumberList); ok {
			return TransferFunctionFilterTable(values), true
		}
		return TransferFunctionFilterTable{}, true
	case "discrete":
		if values, ok := nodeAttribute(node, AIdTableValues, fromValueNumberList); ok {
			return TransferFunctionFilterDiscrete(values), true
		}
		return TransferFunctionFilterDiscrete{}, true
	case "linear":
		return TransferFunctionFilterLinear{
			Slope:     attrF32Or(node, AIdSlope, 1.0),
			Intercept: attrF32Or(node, AIdIntercept, 0.0),
		}, true
	case "gamma":
		return TransferFunctionFilterGamma{
			Amplitude: attrF32Or(node, AIdAmplitude, 1.0),
			Exponent:  attrF32Or(node, AIdExponent, 1.0),
			Offset:    attrF32Or(node, AIdOffset, 0.0),
		}, true
	}
	return nil, false
}

// attrF32Or is the Rust expression `node.attribute::<f32>(aid).unwrap_or(def)`.
func attrF32Or(node svgNode, aid AId, def float32) float32 {
	if v, ok := node.attributeF32(aid); ok {
		return v
	}
	return def
}

// convertComposite is convert_composite in usvg src/parser/filter.rs.
func convertComposite(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	op, ok := fe.attributeStr(AIdOperator)
	if !ok {
		op = "over"
	}
	var operator CompositeOperatorFilter
	switch op {
	case "in":
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterIn}
	case "out":
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterOut}
	case "atop":
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterAtop}
	case "xor":
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterXor}
	case "lighter":
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterLighter}
	case "arithmetic":
		operator = CompositeOperatorFilter{
			Kind: CompositeOperatorFilterArithmetic,
			K1:   attrF32Or(fe, AIdK1, 0.0),
			K2:   attrF32Or(fe, AIdK2, 0.0),
			K3:   attrF32Or(fe, AIdK3, 0.0),
			K4:   attrF32Or(fe, AIdK4, 0.0),
		}
	default:
		operator = CompositeOperatorFilter{Kind: CompositeOperatorFilterOver}
	}

	input1 := resolveInput(fe, AIdIn, primitives)
	input2 := resolveInput(fe, AIdIn2, primitives)

	return &CompositeFilter{
		operator: operator,
		input1:   input1,
		input2:   input2,
	}
}

// asI32 is the Rust cast `x as i32` of an f32: it rounds toward zero,
// saturates, and gives 0 for NaN.
func asI32(x float32) int32 {
	switch {
	case x != x:
		return 0
	case x >= 2147483648.0:
		return math.MaxInt32
	case x <= -2147483648.0:
		return math.MinInt32
	}
	return int32(x)
}

// asI32F64 is the Rust cast `x as i32` of an f64: it rounds toward zero,
// saturates, and gives 0 for NaN.
func asI32F64(x float64) int32 {
	switch {
	case x != x:
		return 0
	case x >= 2147483648.0:
		return math.MaxInt32
	case x <= -2147483648.0:
		return math.MinInt32
	}
	return int32(x)
}

// asU32 is the Rust cast `x as u32` of an f32: it rounds toward zero,
// saturates, and gives 0 for NaN.
func asU32(x float32) uint32 {
	switch {
	case x != x || x <= 0:
		return 0
	case x >= 4294967296.0:
		return math.MaxUint32
	}
	return uint32(x)
}

// convertConvolveMatrix is convert_convolve_matrix in usvg
// src/parser/filter.rs.
func convertConvolveMatrix(fe svgNode, primitives []PrimitiveFilter) (KindFilter, bool) {
	parseTarget := func(target float32, hasTarget bool, order uint32) (uint32, bool) {
		defaultTarget := asU32(float32(math.Floor(float64(float32(order) / 2.0))))
		if !hasTarget {
			target = float32(defaultTarget)
		}
		t := asI32(target)
		if t < 0 || t >= int32(order) {
			return 0, false
		}
		return uint32(t), true
	}

	var orderX uint32 = 3
	var orderY uint32 = 3
	if value, ok := fe.attributeStr(AIdOrder); ok {
		s := svgtypes.NewNumberListParser(value)
		var x int32 = 3
		if n, ok, err := s.Next(); ok && err == nil {
			x = asI32F64(n)
		}
		y := x
		if n, ok, err := s.Next(); ok && err == nil {
			y = asI32F64(n)
		}
		if x > 0 && y > 0 {
			orderX = uint32(x)
			orderY = uint32(y)
		}
	}

	var matrix []float32
	if list, ok := nodeAttribute(fe, AIdKernelMatrix, fromValueNumberList); ok {
		if uint(len(list)) == uint(orderX*orderY) {
			matrix = list
		}
	}

	var kernelSum float32
	for _, v := range matrix {
		kernelSum += v
	}
	// Round up to prevent float precision issues.
	kernelSum = round32(kernelSum*1_000_000.0) / 1_000_000.0
	if ApproxZeroUlps32(kernelSum, 4) {
		kernelSum = 1.0
	}

	divisor := attrF32Or(fe, AIdDivisor, kernelSum)
	if ApproxZeroUlps32(divisor, 4) {
		return nil, false
	}

	bias := attrF32Or(fe, AIdBias, 0.0)

	tx, hasTx := fe.attributeF32(AIdTargetX)
	targetX, ok := parseTarget(tx, hasTx, orderX)
	if !ok {
		return nil, false
	}
	ty, hasTy := fe.attributeF32(AIdTargetY)
	targetY, ok := parseTarget(ty, hasTy, orderY)
	if !ok {
		return nil, false
	}

	kernelMatrix, ok := newConvolveMatrixDataFilter(targetX, targetY, orderX, orderY, matrix)
	if !ok {
		return nil, false
	}

	em, ok := fe.attributeStr(AIdEdgeMode)
	if !ok {
		em = "duplicate"
	}
	var edgeMode EdgeModeFilter
	switch em {
	case "none": //nolint:goconst // an SVG keyword, as upstream writes it
		edgeMode = EdgeModeFilterNone
	case "wrap":
		edgeMode = EdgeModeFilterWrap
	default:
		edgeMode = EdgeModeFilterDuplicate
	}

	pa, ok := fe.attributeStr(AIdPreserveAlpha)
	if !ok {
		pa = "false" //nolint:goconst // an SVG keyword, as upstream writes it
	}
	preserveAlpha := pa == "true" //nolint:goconst // an SVG keyword, as upstream writes it

	d, ok := NewNonZeroF32(divisor)
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return &ConvolveMatrixFilter{
		input:         resolveInput(fe, AIdIn, primitives),
		matrix:        kernelMatrix,
		divisor:       d,
		bias:          bias,
		edgeMode:      edgeMode,
		preserveAlpha: preserveAlpha,
	}, true
}

// convertDisplacementMap is convert_displacement_map in usvg
// src/parser/filter.rs.
func convertDisplacementMap(fe svgNode, scale Size, primitives []PrimitiveFilter) KindFilter {
	parseChannel := func(aid AId) ColorChannelFilter {
		v, ok := fe.attributeStr(aid)
		if !ok {
			v = "A"
		}
		switch v {
		case "R":
			return ColorChannelFilterR
		case "G":
			return ColorChannelFilterG
		case "B":
			return ColorChannelFilterB
		}
		return ColorChannelFilterA
	}

	// TODO: should probably split scale to scale_x and scale_y,
	//       but resvg doesn't support displacement map anyway...
	s := (scale.Width() + scale.Height()) / 2.0

	return &DisplacementMapFilter{
		input1:           resolveInput(fe, AIdIn, primitives),
		input2:           resolveInput(fe, AIdIn2, primitives),
		scale:            attrF32Or(fe, AIdScale, 0.0) * s,
		xChannelSelector: parseChannel(AIdXChannelSelector),
		yChannelSelector: parseChannel(AIdYChannelSelector),
	}
}

// convertDropShadow is convert_drop_shadow in usvg src/parser/filter.rs.
func convertDropShadow(fe svgNode, scale Size, primitives []PrimitiveFilter) KindFilter {
	stdDevX, stdDevY := convertStdDevAttr(fe, scale, "2 2")

	svgColor, ok := nodeAttribute(fe, AIdFloodColor, fromValueColor)
	if !ok {
		svgColor = svgtypes.ColorBlack()
	}
	color, opacity := splitAlpha(svgColor)

	floodOpacity, ok := fe.attributeOpacity(AIdFloodOpacity)
	if !ok {
		floodOpacity = path.NormalizedF32One
	}

	return &DropShadowFilter{
		input:   resolveInput(fe, AIdIn, primitives),
		dx:      attrF32Or(fe, AIdDx, 2.0) * scale.Width(),
		dy:      attrF32Or(fe, AIdDy, 2.0) * scale.Height(),
		stdDevX: stdDevX,
		stdDevY: stdDevY,
		color:   color,
		opacity: opacity.Mul(floodOpacity),
	}
}

// convertFlood is convert_flood in usvg src/parser/filter.rs.
func convertFlood(fe svgNode) KindFilter {
	svgColor, ok := nodeAttribute(fe, AIdFloodColor, fromValueColor)
	if !ok {
		svgColor = svgtypes.ColorBlack()
	}
	color, opacity := splitAlpha(svgColor)

	floodOpacity, ok := fe.attributeOpacity(AIdFloodOpacity)
	if !ok {
		floodOpacity = path.NormalizedF32One
	}

	return &FloodFilter{
		color:   color,
		opacity: opacity.Mul(floodOpacity),
	}
}

// convertGaussianBlur is convert_gaussian_blur in usvg src/parser/filter.rs.
func convertGaussianBlur(fe svgNode, scale Size, primitives []PrimitiveFilter) KindFilter {
	stdDevX, stdDevY := convertStdDevAttr(fe, scale, "0 0")
	return &GaussianBlurFilter{
		input:   resolveInput(fe, AIdIn, primitives),
		stdDevX: stdDevX,
		stdDevY: stdDevY,
	}
}

// convertStdDevAttr is convert_std_dev_attr in usvg src/parser/filter.rs.
func convertStdDevAttr(fe svgNode, scale Size, def string) (PositiveF32, PositiveF32) {
	text, ok := fe.attributeStr(AIdStdDeviation)
	if !ok {
		text = def
	}
	parser := svgtypes.NewNumberListParser(text)

	next := func() (float64, bool) {
		n, ok, err := parser.Next()
		return n, ok && err == nil
	}
	n1, has1 := next()
	n2, has2 := next()
	// `stdDeviation` must have no more than two values.
	// Otherwise we should fallback to `0 0`.
	_, has3 := next()

	var stdDevX, stdDevY float64
	switch {
	case has1 && has2 && !has3:
		stdDevX, stdDevY = n1, n2
	case has1 && !has2 && !has3:
		stdDevX, stdDevY = n1, n1
	default:
		stdDevX, stdDevY = 0.0, 0.0
	}

	x := float32(stdDevX) * scale.Width()
	y := float32(stdDevY) * scale.Height()

	px, ok := path.NewPositiveF32(x)
	if !ok {
		px = 0
	}
	py, ok := path.NewPositiveF32(y)
	if !ok {
		py = 0
	}

	return px, py
}

// convertImageFilter is convert_image in usvg src/parser/filter.rs.
func convertImageFilter(
	fe svgNode,
	filterSubregion NonZeroRect,
	state *converterState,
	cache *converterCache,
) KindFilter {
	if kind, ok := convertImageInner(fe, filterSubregion, state, cache); ok {
		return kind
	}
	return createDummyPrimitive()
}

// convertImageInner is convert_image_inner in usvg src/parser/filter.rs.
func convertImageInner(
	fe svgNode,
	filterSubregion NonZeroRect,
	state *converterState,
	cache *converterCache,
) (KindFilter, bool) {
	renderingMode, ok := nodeFindAttribute(fe, AIdImageRendering, fromValueImageRendering)
	if !ok {
		renderingMode = state.opt.imageRendering
	}

	if node, ok := nodeTryAttribute(fe, AIdHref, fromValueSvgNode); ok {
		state := state.clone()
		state.feImageLink = true
		root := emptyGroup()
		convertElement(node, state, cache, root)
		if !root.HasChildren() {
			return nil, false
		}
		root.calculateBoundingBoxes()
		updatePaintServers(
			root,
			path.DefaultTransform(),
			nil,
			nil,
			cache,
		)
		// Transfer node id from group's child to the group itself if needed.
		if len(root.children) > 0 {
			if g, ok := root.children[0].(*Group); ok {
				if len(g.children) > 0 {
					child2 := g.children[0]
					g.id = child2.ID()
					switch c := child2.(type) {
					case *Group:
						c.id = ""
					case *Path:
						c.id = ""
					case *Image:
						c.id = ""
					case *Text:
						c.id = ""
					}
				}
			}
		}

		return &ImageFilter{root: *root}, true
	}

	href, ok := nodeTryAttribute(fe, AIdHref, fromValueStr)
	href, ok = logNone(href, ok, func() {
		state.opt.warnf("The 'feImage' element lacks the 'xlink:href' attribute. Skipped.")
	})
	if !ok {
		return nil, false
	}
	imgData, ok := getHrefData(href, state)
	if !ok {
		return nil, false
	}
	actualSize, ok := imageKindActualSize(imgData, state.opt)
	if !ok {
		return nil, false
	}

	aspect, ok := nodeAttribute(fe, AIdPreserveAspectRatio, fromValueAspectRatio)
	if !ok {
		aspect = svgtypes.DefaultAspectRatio()
	}

	rect, ok := filterSubregion.TranslateTo(0.0, 0.0)
	if !ok {
		return nil, false
	}
	root := emptyGroup()
	convertInner(
		imgData,
		cache.genImageID().take(),
		true,
		renderingMode,
		aspect,
		actualSize,
		rect,
		cache,
		root,
	)
	root.calculateBoundingBoxes()

	return &ImageFilter{root: *root}, true
}

// convertDiffuseLighting is convert_diffuse_lighting in usvg
// src/parser/filter.rs.
func convertDiffuseLighting(fe svgNode, primitives []PrimitiveFilter) (KindFilter, bool) {
	lightSource, ok := convertLightSource(fe)
	if !ok {
		return nil, false
	}
	return &DiffuseLightingFilter{
		input:           resolveInput(fe, AIdIn, primitives),
		surfaceScale:    attrF32Or(fe, AIdSurfaceScale, 1.0),
		diffuseConstant: attrF32Or(fe, AIdDiffuseConstant, 1.0),
		lightingColor:   convertLightingColor(fe),
		lightSource:     lightSource,
	}, true
}

// convertSpecularLighting is convert_specular_lighting in usvg
// src/parser/filter.rs.
func convertSpecularLighting(fe svgNode, primitives []PrimitiveFilter) (KindFilter, bool) {
	lightSource, ok := convertLightSource(fe)
	if !ok {
		return nil, false
	}

	specularExponent := attrF32Or(fe, AIdSpecularExponent, 1.0)
	if !(1.0 <= specularExponent && specularExponent <= 128.0) {
		// When exponent is out of range, the whole filter primitive should be ignored.
		return nil, false
	}

	specularExponent = f32Bound(1.0, specularExponent, 128.0)

	return &SpecularLightingFilter{
		input:            resolveInput(fe, AIdIn, primitives),
		surfaceScale:     attrF32Or(fe, AIdSurfaceScale, 1.0),
		specularConstant: attrF32Or(fe, AIdSpecularConstant, 1.0),
		specularExponent: specularExponent,
		lightingColor:    convertLightingColor(fe),
		lightSource:      lightSource,
	}, true
}

// convertLightingColor is convert_lighting_color in usvg
// src/parser/filter.rs.
func convertLightingColor(node svgNode) Color {
	// Color's alpha doesn't affect lighting-color. Simply skip it.
	value, ok := node.attributeStr(AIdLightingColor)
	switch {
	case ok && value == "currentColor":
		c, ok := nodeFindAttribute(node, AIdColor, fromValueColor)
		if !ok {
			// Yes, a missing `currentColor` resolves to black and not white.
			c = svgtypes.ColorBlack()
		}
		color, _ := splitAlpha(c)
		return color
	case ok:
		if c, err := svgtypes.ParseColor(value); err == nil {
			color, _ := splitAlpha(c)
			return color
		}
		node.doc.opt.warnf("Failed to parse lighting-color value: '%s'.", value)
		return ColorWhite()
	}
	return ColorWhite()
}

// convertLightSource is convert_light_source in usvg src/parser/filter.rs.
func convertLightSource(parent svgNode) (LightSourceFilter, bool) {
	var child svgNode
	found := false
	for n := range parent.children() {
		if t, ok := n.tagName(); ok && (t == EIdFeDistantLight || t == EIdFePointLight || t == EIdFeSpotLight) {
			child, found = n, true
			break
		}
	}
	if !found {
		return nil, false
	}

	tagName, _ := child.tagName()
	switch tagName {
	case EIdFeDistantLight:
		return DistantLightFilter{
			Azimuth:   attrF32Or(child, AIdAzimuth, 0.0),
			Elevation: attrF32Or(child, AIdElevation, 0.0),
		}, true
	case EIdFePointLight:
		return PointLightFilter{
			X: attrF32Or(child, AIdX, 0.0),
			Y: attrF32Or(child, AIdY, 0.0),
			Z: attrF32Or(child, AIdZ, 0.0),
		}, true
	case EIdFeSpotLight:
		specularExponent := attrF32Or(child, AIdSpecularExponent, 1.0)
		se, ok := path.NewPositiveF32(specularExponent)
		if !ok {
			se = mustPositiveF32(path.NewPositiveF32(1.0))
		}

		var limitingConeAngle *float32
		if v, ok := child.attributeF32(AIdLimitingConeAngle); ok {
			limitingConeAngle = &v
		}

		return SpotLightFilter{
			X:                 attrF32Or(child, AIdX, 0.0),
			Y:                 attrF32Or(child, AIdY, 0.0),
			Z:                 attrF32Or(child, AIdZ, 0.0),
			PointsAtX:         attrF32Or(child, AIdPointsAtX, 0.0),
			PointsAtY:         attrF32Or(child, AIdPointsAtY, 0.0),
			PointsAtZ:         attrF32Or(child, AIdPointsAtZ, 0.0),
			SpecularExponent:  se,
			LimitingConeAngle: limitingConeAngle,
		}, true
	}
	return nil, false
}

// convertMerge is convert_merge in usvg src/parser/filter.rs.
func convertMerge(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	var inputs []InputFilter
	for child := range fe.children() {
		inputs = append(inputs, resolveInput(child, AIdIn, primitives))
	}

	return &MergeFilter{inputs: inputs}
}

// convertMorphology is convert_morphology in usvg src/parser/filter.rs.
func convertMorphology(fe svgNode, scale Size, primitives []PrimitiveFilter) KindFilter {
	op, ok := fe.attributeStr(AIdOperator)
	if !ok {
		op = "erode"
	}
	operator := MorphologyOperatorFilterErode
	if op == "dilate" {
		operator = MorphologyOperatorFilterDilate
	}

	radiusX := mustPositiveF32(path.NewPositiveF32(scale.Width()))
	radiusY := mustPositiveF32(path.NewPositiveF32(scale.Height()))
	if list, ok := nodeAttribute(fe, AIdRadius, fromValueNumberList); ok {
		var rx, ry float32
		if len(list) == 2 {
			rx = list[0]
			ry = list[1]
		} else if len(list) == 1 {
			rx = list[0]
			ry = list[0] // The same as `rx`.
		}

		if ApproxZeroUlps32(rx, 4) && ApproxZeroUlps32(ry, 4) {
			rx = 1.0
			ry = 1.0
		}

		// If only one of the values is zero, reset it to 1.0
		// This is not specified in the spec, but this is how Chrome and Safari work.
		if ApproxZeroUlps32(rx, 4) && !ApproxZeroUlps32(ry, 4) {
			rx = 1.0
		}
		if !ApproxZeroUlps32(rx, 4) && ApproxZeroUlps32(ry, 4) {
			ry = 1.0
		}

		// Both values must be positive.
		if !math.Signbit(float64(rx)) && !math.Signbit(float64(ry)) {
			radiusX = mustPositiveF32(path.NewPositiveF32(rx * scale.Width()))
			radiusY = mustPositiveF32(path.NewPositiveF32(ry * scale.Height()))
		}
	}

	return &MorphologyFilter{
		input:    resolveInput(fe, AIdIn, primitives),
		operator: operator,
		radiusX:  radiusX,
		radiusY:  radiusY,
	}
}

// convertOffset is convert_offset in usvg src/parser/filter.rs.
func convertOffset(fe svgNode, scale Size, primitives []PrimitiveFilter) KindFilter {
	return &OffsetFilter{
		input: resolveInput(fe, AIdIn, primitives),
		dx:    attrF32Or(fe, AIdDx, 0.0) * scale.Width(),
		dy:    attrF32Or(fe, AIdDy, 0.0) * scale.Height(),
	}
}

// convertTile is convert_tile in usvg src/parser/filter.rs.
func convertTile(fe svgNode, primitives []PrimitiveFilter) KindFilter {
	return &TileFilter{
		input: resolveInput(fe, AIdIn, primitives),
	}
}

// convertTurbulence is convert_turbulence in usvg src/parser/filter.rs.
func convertTurbulence(fe svgNode) KindFilter {
	var baseFrequencyX, baseFrequencyY PositiveF32
	if list, ok := nodeAttribute(fe, AIdBaseFrequency, fromValueNumberList); ok {
		var x, y float32
		if len(list) == 2 {
			x = list[0]
			y = list[1]
		} else if len(list) == 1 {
			x = list[0]
			y = list[0] // The same as `x`.
		}

		if !math.Signbit(float64(x)) && !math.Signbit(float64(y)) {
			baseFrequencyX = mustPositiveF32(path.NewPositiveF32(x))
			baseFrequencyY = mustPositiveF32(path.NewPositiveF32(y))
		}
	}

	numOctaves := attrF32Or(fe, AIdNumOctaves, 1.0)
	if math.Signbit(float64(numOctaves)) {
		numOctaves = 0.0
	}

	t, ok := fe.attributeStr(AIdType)
	if !ok {
		t = "turbulence"
	}
	kind := TurbulenceKindFilterTurbulence
	if t == "fractalNoise" {
		kind = TurbulenceKindFilterFractalNoise
	}

	st, hasSt := fe.attributeStr(AIdStitchTiles)
	return &TurbulenceFilter{
		baseFrequencyX: baseFrequencyX,
		baseFrequencyY: baseFrequencyY,
		numOctaves:     asU32(round32(numOctaves)),
		seed:           asI32(float32(math.Trunc(float64(attrF32Or(fe, AIdSeed, 0.0))))),
		stitchTiles:    hasSt && st == "stitch",
		kind:           kind,
	}
}

// convertGrayscaleFunction is convert_grayscale_function in usvg
// src/parser/filter.rs.
func convertGrayscaleFunction(amount float64) KindFilter {
	a := float32(math.Min(amount, 1.0))
	return &ColorMatrixFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		kind: ColorMatrixKindFilterMatrix{
			0.2126 + float32(0.7874*(1.0-a)),
			0.7152 - float32(0.7152*(1.0-a)),
			0.0722 - float32(0.0722*(1.0-a)),
			0.0,
			0.0,
			0.2126 - float32(0.2126*(1.0-a)),
			0.7152 + float32(0.2848*(1.0-a)),
			0.0722 - float32(0.0722*(1.0-a)),
			0.0,
			0.0,
			0.2126 - float32(0.2126*(1.0-a)),
			0.7152 - float32(0.7152*(1.0-a)),
			0.0722 + float32(0.9278*(1.0-a)),
			0.0,
			0.0,
			0.0,
			0.0,
			0.0,
			1.0,
			0.0,
		},
	}
}

// convertSepiaFunction is convert_sepia_function in usvg
// src/parser/filter.rs.
func convertSepiaFunction(amount float64) KindFilter {
	a := float32(math.Min(amount, 1.0))
	return &ColorMatrixFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		kind: ColorMatrixKindFilterMatrix{
			0.393 + float32(0.607*(1.0-a)),
			0.769 - float32(0.769*(1.0-a)),
			0.189 - float32(0.189*(1.0-a)),
			0.0,
			0.0,
			0.349 - float32(0.349*(1.0-a)),
			0.686 + float32(0.314*(1.0-a)),
			0.168 - float32(0.168*(1.0-a)),
			0.0,
			0.0,
			0.272 - float32(0.272*(1.0-a)),
			0.534 - float32(0.534*(1.0-a)),
			0.131 + float32(0.869*(1.0-a)),
			0.0,
			0.0,
			0.0,
			0.0,
			0.0,
			1.0,
			0.0,
		},
	}
}

// convertSaturateFunction is convert_saturate_function in usvg
// src/parser/filter.rs.
func convertSaturateFunction(amount float64) KindFilter {
	a, ok := path.NewPositiveF32(float32(amount))
	if !ok {
		a = 0
	}
	return &ColorMatrixFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		kind:  ColorMatrixKindFilterSaturate(a),
	}
}

// convertHueRotateFunction is convert_hue_rotate_function in usvg
// src/parser/filter.rs.
func convertHueRotateFunction(amount svgtypes.Angle) KindFilter {
	return &ColorMatrixFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		kind:  ColorMatrixKindFilterHueRotate(float32(amount.ToDegrees())),
	}
}

// convertInvertFunction is convert_invert_function in usvg
// src/parser/filter.rs.
func convertInvertFunction(amount float64) KindFilter {
	a := float32(math.Min(amount, 1.0))
	return &ComponentTransferFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		funcR: TransferFunctionFilterTable{a, 1.0 - a},
		funcG: TransferFunctionFilterTable{a, 1.0 - a},
		funcB: TransferFunctionFilterTable{a, 1.0 - a},
		funcA: TransferFunctionFilterIdentity{},
	}
}

// convertOpacityFunction is convert_opacity_function in usvg
// src/parser/filter.rs.
func convertOpacityFunction(amount float64) KindFilter {
	a := float32(math.Min(amount, 1.0))
	return &ComponentTransferFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		funcR: TransferFunctionFilterIdentity{},
		funcG: TransferFunctionFilterIdentity{},
		funcB: TransferFunctionFilterIdentity{},
		funcA: TransferFunctionFilterTable{0.0, a},
	}
}

// convertBrightnessFunction is convert_brightness_function in usvg
// src/parser/filter.rs.
func convertBrightnessFunction(amount float64) KindFilter {
	a := float32(amount)
	return &ComponentTransferFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		funcR: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: 0.0,
		},
		funcG: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: 0.0,
		},
		funcB: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: 0.0,
		},
		funcA: TransferFunctionFilterIdentity{},
	}
}

// convertContrastFunction is convert_contrast_function in usvg
// src/parser/filter.rs.
func convertContrastFunction(amount float64) KindFilter {
	a := float32(amount)
	return &ComponentTransferFilter{
		input: InputFilter{Kind: InputFilterSourceGraphic},
		funcR: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: -float32(0.5*a) + 0.5,
		},
		funcG: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: -float32(0.5*a) + 0.5,
		},
		funcB: TransferFunctionFilterLinear{
			Slope:     a,
			Intercept: -float32(0.5*a) + 0.5,
		},
		funcA: TransferFunctionFilterIdentity{},
	}
}

// convertBlurFunction is convert_blur_function in usvg src/parser/filter.rs.
func convertBlurFunction(node svgNode, stdDev svgtypes.Length, state *converterState) KindFilter {
	sd, ok := path.NewPositiveF32(convertUserLength(
		stdDev,
		node,
		AIdDx,
		state,
	))
	if !ok {
		sd = 0
	}
	return &GaussianBlurFilter{
		input:   InputFilter{Kind: InputFilterSourceGraphic},
		stdDevX: sd,
		stdDevY: sd,
	}
}

// convertDropShadowFunction is convert_drop_shadow_function in usvg
// src/parser/filter.rs.
//
// Port: a nil color is None.
func convertDropShadowFunction(
	node svgNode,
	color *svgtypes.Color,
	dx svgtypes.Length,
	dy svgtypes.Length,
	stdDev svgtypes.Length,
	state *converterState,
) KindFilter {
	sd, ok := path.NewPositiveF32(convertUserLength(
		stdDev,
		node,
		AIdDx,
		state,
	))
	if !ok {
		sd = 0
	}

	var svgColor svgtypes.Color
	if color != nil {
		svgColor = *color
	} else if c, ok := nodeFindAttribute(node, AIdColor, fromValueColor); ok {
		svgColor = c
	} else {
		svgColor = svgtypes.ColorBlack()
	}
	c, opacity := splitAlpha(svgColor)

	return &DropShadowFilter{
		input:   InputFilter{Kind: InputFilterSourceGraphic},
		dx:      convertUserLength(dx, node, AIdDx, state),
		dy:      convertUserLength(dy, node, AIdDy, state),
		stdDevX: sd,
		stdDevY: sd,
		color:   c,
		opacity: opacity,
	}
}
