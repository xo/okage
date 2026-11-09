// Copyright 2023 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/base64"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/internal/xmlwriter"
	"github.com/xo/okage/path"
)

// This file ports usvg src/writer.rs, of the feature writer.
//
// Port: the struct WriteOptions becomes the private writeOptions, with one
// functional option WriteOption for each field (D20). The trait XmlWriterExt
// becomes the type svgWriter, which embeds the *xmlwriter.XMLWriter. The
// trait IsDefault has no port, because Go compares a value with its default
// in place. A float prints with fmtF32, the text that the impl Display for
// f32 of Rust gives.

// ToString is Tree::to_string in usvg src/writer.rs. It writes `usvg::Tree`
// back to SVG.
func (t *Tree) ToString(opts ...WriteOption) string {
	o := defaultWriteOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return convertWriter(t, &o)
}

// WriteOption is an option of Tree.ToString. It sets a field of the struct
// WriteOptions in usvg src/writer.rs (D20).
type WriteOption func(*writeOptions)

// writeOptions is WriteOptions in usvg src/writer.rs. It holds the XML
// writing options.
type writeOptions struct {
	// Used to add a custom prefix to each element ID during writing.
	//
	// Port: the empty string is None.
	idPrefix string

	// Do not convert text into paths.
	//
	// Default: false
	preserveText bool

	// Set the coordinates numeric precision.
	//
	// Smaller precision can lead to a malformed output in some cases.
	//
	// Default: 8
	coordinatesPrecision uint8

	// Set the transform values numeric precision.
	//
	// Smaller precision can lead to a malformed output in some cases.
	//
	// Default: 8
	transformsPrecision uint8

	// Use single quote marks instead of double quote.
	//
	// Default: disabled
	useSingleQuote bool

	// Set XML nodes indention.
	//
	// Default: 4 spaces
	indent Indent

	// Set XML attributes indention.
	//
	// Default: `None`
	attributesIndent Indent
}

// defaultWriteOptions is the impl Default for WriteOptions in usvg
// src/writer.rs.
func defaultWriteOptions() writeOptions {
	return writeOptions{
		idPrefix:             "",
		preserveText:         false,
		coordinatesPrecision: 8,
		transformsPrecision:  8,
		useSingleQuote:       false,
		indent:               Indent{Kind: IndentSpaces, Spaces: 4},
		attributesIndent:     Indent{Kind: IndentNone},
	}
}

// WithIDPrefix sets WriteOptions::id_prefix in usvg src/writer.rs: a custom
// prefix that is added to each element ID during writing.
//
// Default: none. The empty string is none.
func WithIDPrefix(prefix string) WriteOption {
	return func(o *writeOptions) {
		o.idPrefix = prefix
	}
}

// WithPreserveText sets WriteOptions::preserve_text in usvg src/writer.rs:
// do not convert text into paths.
//
// Default: false.
func WithPreserveText(preserve bool) WriteOption {
	return func(o *writeOptions) {
		o.preserveText = preserve
	}
}

// WithCoordinatesPrecision sets WriteOptions::coordinates_precision in usvg
// src/writer.rs: the coordinates numeric precision.
//
// Smaller precision can lead to a malformed output in some cases.
//
// Default: 8. A precision above 12 panics in ToString, as an index out of
// range does in upstream.
func WithCoordinatesPrecision(precision uint8) WriteOption {
	return func(o *writeOptions) {
		o.coordinatesPrecision = precision
	}
}

// WithTransformsPrecision sets WriteOptions::transforms_precision in usvg
// src/writer.rs: the transform values numeric precision.
//
// Smaller precision can lead to a malformed output in some cases.
//
// Default: 8. A precision above 12 panics in ToString, as an index out of
// range does in upstream.
func WithTransformsPrecision(precision uint8) WriteOption {
	return func(o *writeOptions) {
		o.transformsPrecision = precision
	}
}

// WithUseSingleQuote sets WriteOptions::use_single_quote in usvg
// src/writer.rs: use single quote marks instead of double quote.
//
// Before:
//
//	<rect fill="red"/>
//
// After:
//
//	<rect fill='red'/>
//
// Default: disabled.
func WithUseSingleQuote(use bool) WriteOption {
	return func(o *writeOptions) {
		o.useSingleQuote = use
	}
}

// WithIndent sets WriteOptions::indent in usvg src/writer.rs: the XML nodes
// indention.
//
// With IndentNone, before:
//
//	<svg>
//	    <rect fill="red"/>
//	</svg>
//
// After:
//
//	<svg><rect fill="red"/></svg>
//
// Default: 4 spaces.
func WithIndent(indent Indent) WriteOption {
	return func(o *writeOptions) {
		o.indent = indent
	}
}

// WithAttributesIndent sets WriteOptions::attributes_indent in usvg
// src/writer.rs: the XML attributes indention.
//
// With 2 spaces, before:
//
//	<svg>
//	    <rect fill="red" stroke="black"/>
//	</svg>
//
// After:
//
//	<svg>
//	    <rect
//	      fill="red"
//	      stroke="black"/>
//	</svg>
//
// Default: none.
func WithAttributesIndent(indent Indent) WriteOption {
	return func(o *writeOptions) {
		o.attributesIndent = indent
	}
}

// convertWriter is convert in usvg src/writer.rs.
func convertWriter(tree *Tree, opt *writeOptions) string {
	xml := svgWriter{xmlwriter.New(
		xmlwriter.WithUseSingleQuote(opt.useSingleQuote),
		xmlwriter.WithIndent(opt.indent),
		xmlwriter.WithAttributesIndent(opt.attributesIndent),
	)}

	xml.startSvgElement(EIdSvg)
	xml.writeSvgAttributeF32(AIdWidth, tree.size.Width())
	xml.writeSvgAttributeF32(AIdHeight, tree.size.Height())
	xml.WriteAttribute("xmlns", "http://www.w3.org/2000/svg")
	if hasXlink(&tree.root) {
		xml.WriteAttribute("xmlns:xlink", "http://www.w3.org/1999/xlink")
	}

	hasTextPaths := hasTextPaths(&tree.root)
	if tree.HasDefsNodes() || hasTextPaths {
		writeDefs(tree, opt, xml, hasTextPaths)
	}

	writeElements(&tree.root, false, opt, xml)

	return xml.EndDocument()
}

// writeFilters is write_filters in usvg src/writer.rs.
func writeFilters(tree *Tree, opt *writeOptions, xml svgWriter) {
	var writtenFeImageNodes []string
	for _, filter := range tree.Filters() {
		for i := range filter.primitives {
			fe := &filter.primitives[i]
			if img, ok := fe.kind.(*ImageFilter); ok {
				if len(img.Root().children) != 0 {
					child := img.Root().children[0]
					seen := false
					for _, id := range writtenFeImageNodes {
						if id == child.ID() {
							seen = true
							break
						}
					}
					if !seen {
						writeElement(child, false, opt, xml)
						writtenFeImageNodes = append(writtenFeImageNodes, child.ID())
					}
				}
			}
		}

		xml.startSvgElement(EIdFilter)
		xml.writeIDAttribute(filter.ID(), opt)
		xml.writeRectAttrs(filter.rect)
		xml.writeUnits(
			AIdFilterUnits,
			unitsUserSpaceOnUse,
			unitsObjectBoundingBox,
		)

		for i := range filter.primitives {
			fe := &filter.primitives[i]
			switch kind := fe.kind.(type) {
			case *DropShadowFilter:
				xml.startSvgElement(EIdFeDropShadow)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.WriteAttribute(
					AIdStdDeviation.ToStr(),
					fmtF32(kind.stdDevX.Get())+" "+fmtF32(kind.stdDevY.Get()),
				)
				xml.writeSvgAttributeF32(AIdDx, kind.dx)
				xml.writeSvgAttributeF32(AIdDy, kind.dy)
				xml.writeColor(AIdFloodColor, kind.color)
				xml.writeSvgAttributeF32(AIdFloodOpacity, kind.opacity.Get())
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *GaussianBlurFilter:
				xml.startSvgElement(EIdFeGaussianBlur)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.WriteAttribute(
					AIdStdDeviation.ToStr(),
					fmtF32(kind.stdDevX.Get())+" "+fmtF32(kind.stdDevY.Get()),
				)
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *OffsetFilter:
				xml.startSvgElement(EIdFeOffset)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttributeF32(AIdDx, kind.dx)
				xml.writeSvgAttributeF32(AIdDy, kind.dy)
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *BlendFilter:
				xml.startSvgElement(EIdFeBlend)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input1)
				xml.writeFilterInput(AIdIn2, kind.input2)
				xml.writeSvgAttribute(AIdMode, kind.mode.String())
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *FloodFilter:
				xml.startSvgElement(EIdFeFlood)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeColor(AIdFloodColor, kind.color)
				xml.writeSvgAttributeF32(AIdFloodOpacity, kind.opacity.Get())
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *CompositeFilter:
				xml.startSvgElement(EIdFeComposite)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input1)
				xml.writeFilterInput(AIdIn2, kind.input2)
				var operator string
				switch kind.operator.Kind {
				case CompositeOperatorFilterOver:
					operator = "over"
				case CompositeOperatorFilterIn:
					operator = "in"
				case CompositeOperatorFilterOut:
					operator = "out"
				case CompositeOperatorFilterAtop:
					operator = "atop"
				case CompositeOperatorFilterXor:
					operator = "xor"
				case CompositeOperatorFilterLighter:
					operator = "lighter"
				case CompositeOperatorFilterArithmetic:
					operator = "arithmetic"
				}
				xml.writeSvgAttribute(AIdOperator, operator)

				if kind.operator.Kind == CompositeOperatorFilterArithmetic {
					xml.writeSvgAttributeF32(AIdK1, kind.operator.K1)
					xml.writeSvgAttributeF32(AIdK2, kind.operator.K2)
					xml.writeSvgAttributeF32(AIdK3, kind.operator.K3)
					xml.writeSvgAttributeF32(AIdK4, kind.operator.K4)
				}

				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *MergeFilter:
				xml.startSvgElement(EIdFeMerge)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeSvgAttribute(AIdResult, fe.result)
				for _, input := range kind.inputs {
					xml.startSvgElement(EIdFeMergeNode)
					xml.writeFilterInput(AIdIn, input)
					xml.EndElement()
				}

				xml.EndElement()
			case *TileFilter:
				xml.startSvgElement(EIdFeTile)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *ImageFilter:
				xml.startSvgElement(EIdFeImage)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				if len(kind.root.children) != 0 {
					child := kind.root.children[0]
					xml.WriteAttribute("xlink:href", "#"+opt.idPrefix+child.ID())
				}

				xml.writeSvgAttribute(AIdResult, fe.result)
				xml.EndElement()
			case *ComponentTransferFilter:
				xml.startSvgElement(EIdFeComponentTransfer)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.writeFilterTransferFunction(EIdFeFuncR, kind.funcR)
				xml.writeFilterTransferFunction(EIdFeFuncG, kind.funcG)
				xml.writeFilterTransferFunction(EIdFeFuncB, kind.funcB)
				xml.writeFilterTransferFunction(EIdFeFuncA, kind.funcA)

				xml.EndElement()
			case *ColorMatrixFilter:
				xml.startSvgElement(EIdFeColorMatrix)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttribute(AIdResult, fe.result)

				switch k := kind.kind.(type) {
				case ColorMatrixKindFilterMatrix:
					xml.writeSvgAttribute(AIdType, "matrix")
					xml.writeNumbers(AIdValues, k)
				case ColorMatrixKindFilterSaturate:
					xml.writeSvgAttribute(AIdType, "saturate")
					xml.writeSvgAttributeF32(AIdValues, PositiveF32(k).Get())
				case ColorMatrixKindFilterHueRotate:
					xml.writeSvgAttribute(AIdType, "hueRotate")
					xml.writeSvgAttributeF32(AIdValues, float32(k))
				case ColorMatrixKindFilterLuminanceToAlpha:
					xml.writeSvgAttribute(AIdType, "luminanceToAlpha")
				}

				xml.EndElement()
			case *ConvolveMatrixFilter:
				xml.startSvgElement(EIdFeConvolveMatrix)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.WriteAttribute(
					AIdOrder.ToStr(),
					strconv.FormatUint(uint64(kind.matrix.columns), 10)+" "+strconv.FormatUint(uint64(kind.matrix.rows), 10),
				)
				xml.writeNumbers(AIdKernelMatrix, kind.matrix.data)
				xml.writeSvgAttributeF32(AIdDivisor, kind.divisor.Get())
				xml.writeSvgAttributeF32(AIdBias, kind.bias)
				xml.writeSvgAttribute(AIdTargetX, strconv.FormatUint(uint64(kind.matrix.targetX), 10))
				xml.writeSvgAttribute(AIdTargetY, strconv.FormatUint(uint64(kind.matrix.targetY), 10))
				var edgeMode string
				switch kind.edgeMode {
				case EdgeModeFilterNone:
					edgeMode = "none" //nolint:goconst // an SVG keyword, as upstream writes it
				case EdgeModeFilterDuplicate:
					edgeMode = "duplicate"
				case EdgeModeFilterWrap:
					edgeMode = "wrap"
				}
				xml.writeSvgAttribute(AIdEdgeMode, edgeMode)
				preserveAlpha := "false" //nolint:goconst // an SVG keyword, as upstream writes it
				if kind.preserveAlpha {
					preserveAlpha = "true" //nolint:goconst // an SVG keyword, as upstream writes it
				}
				xml.writeSvgAttribute(AIdPreserveAlpha, preserveAlpha)

				xml.EndElement()
			case *MorphologyFilter:
				xml.startSvgElement(EIdFeMorphology)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input)
				xml.writeSvgAttribute(AIdResult, fe.result)

				operator := "erode"
				if kind.operator == MorphologyOperatorFilterDilate {
					operator = "dilate"
				}
				xml.writeSvgAttribute(AIdOperator, operator)
				xml.WriteAttribute(
					AIdRadius.ToStr(),
					fmtF32(kind.radiusX.Get())+" "+fmtF32(kind.radiusY.Get()),
				)

				xml.EndElement()
			case *DisplacementMapFilter:
				xml.startSvgElement(EIdFeDisplacementMap)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeFilterInput(AIdIn, kind.input1)
				xml.writeFilterInput(AIdIn2, kind.input2)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.writeSvgAttributeF32(AIdScale, kind.scale)

				writeChannel := func(c ColorChannelFilter, aid AId) {
					var name string
					switch c {
					case ColorChannelFilterR:
						name = "R"
					case ColorChannelFilterG:
						name = "G"
					case ColorChannelFilterB:
						name = "B"
					case ColorChannelFilterA:
						name = "A"
					}
					xml.writeSvgAttribute(aid, name)
				}
				writeChannel(kind.xChannelSelector, AIdXChannelSelector)
				writeChannel(kind.yChannelSelector, AIdYChannelSelector)

				xml.EndElement()
			case *TurbulenceFilter:
				xml.startSvgElement(EIdFeTurbulence)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.WriteAttribute(
					AIdBaseFrequency.ToStr(),
					fmtF32(kind.baseFrequencyX.Get())+" "+fmtF32(kind.baseFrequencyY.Get()),
				)
				xml.writeSvgAttribute(AIdNumOctaves, strconv.FormatUint(uint64(kind.numOctaves), 10))
				xml.writeSvgAttribute(AIdSeed, strconv.FormatInt(int64(kind.seed), 10))
				stitchTiles := "noStitch"
				if kind.stitchTiles {
					stitchTiles = "stitch"
				}
				xml.writeSvgAttribute(AIdStitchTiles, stitchTiles)
				typ := "fractalNoise"
				if kind.kind == TurbulenceKindFilterTurbulence {
					typ = "turbulence"
				}
				xml.writeSvgAttribute(AIdType, typ)

				xml.EndElement()
			case *DiffuseLightingFilter:
				xml.startSvgElement(EIdFeDiffuseLighting)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.writeSvgAttributeF32(AIdSurfaceScale, kind.surfaceScale)
				xml.writeSvgAttributeF32(AIdDiffuseConstant, kind.diffuseConstant)
				xml.writeColor(AIdLightingColor, kind.lightingColor)
				writeLightSource(kind.lightSource, xml)

				xml.EndElement()
			case *SpecularLightingFilter:
				xml.startSvgElement(EIdFeSpecularLighting)
				xml.writeFilterPrimitiveAttrs(filter.Rect(), fe)
				xml.writeSvgAttribute(AIdResult, fe.result)

				xml.writeSvgAttributeF32(AIdSurfaceScale, kind.surfaceScale)
				xml.writeSvgAttributeF32(AIdSpecularConstant, kind.specularConstant)
				xml.writeSvgAttributeF32(AIdSpecularExponent, kind.specularExponent)
				xml.writeColor(AIdLightingColor, kind.lightingColor)
				writeLightSource(kind.lightSource, xml)

				xml.EndElement()
			}
		}

		xml.EndElement()
	}
}

// writeDefs is write_defs in usvg src/writer.rs.
func writeDefs(tree *Tree, opt *writeOptions, xml svgWriter, writeTextPaths bool) {
	xml.startSvgElement(EIdDefs)
	for _, lg := range tree.LinearGradients() {
		xml.startSvgElement(EIdLinearGradient)
		xml.writeIDAttribute(lg.ID(), opt)
		xml.writeSvgAttributeF32(AIdX1, lg.x1)
		xml.writeSvgAttributeF32(AIdY1, lg.y1)
		xml.writeSvgAttributeF32(AIdX2, lg.x2)
		xml.writeSvgAttributeF32(AIdY2, lg.y2)
		writeBaseGrad(&lg.BaseGradient, opt, xml)
		xml.EndElement()
	}

	for _, rg := range tree.RadialGradients() {
		xml.startSvgElement(EIdRadialGradient)
		xml.writeIDAttribute(rg.ID(), opt)
		xml.writeSvgAttributeF32(AIdCx, rg.cx)
		xml.writeSvgAttributeF32(AIdCy, rg.cy)
		xml.writeSvgAttributeF32(AIdR, rg.r.Get())
		xml.writeSvgAttributeF32(AIdFx, rg.fx)
		xml.writeSvgAttributeF32(AIdFy, rg.fy)
		writeBaseGrad(&rg.BaseGradient, opt, xml)
		xml.EndElement()
	}

	for _, pattern := range tree.Patterns() {
		xml.startSvgElement(EIdPattern)
		xml.writeIDAttribute(pattern.ID(), opt)
		xml.writeRectAttrs(pattern.rect)
		xml.writeUnits(AIdPatternUnits, pattern.units, unitsObjectBoundingBox)
		xml.writeUnits(
			AIdPatternContentUnits,
			pattern.contentUnits,
			unitsUserSpaceOnUse,
		)
		xml.writeTransform(AIdPatternTransform, pattern.transform, opt)

		writeElements(&pattern.root, false, opt, xml)

		xml.EndElement()
	}

	if writeTextPaths {
		writeTextPathPaths(&tree.root, opt, xml)
	}

	writeFilters(tree, opt, xml)

	for _, clip := range tree.ClipPaths() {
		xml.startSvgElement(EIdClipPath)
		xml.writeIDAttribute(clip.ID(), opt)
		xml.writeTransform(AIdTransform, clip.transform, opt)

		if clip.clipPath != nil {
			xml.writeFuncIRI(AIdClipPath, clip.clipPath.ID(), opt)
		}

		writeElements(&clip.root, true, opt, xml)

		xml.EndElement()
	}

	for _, mask := range tree.Masks() {
		xml.startSvgElement(EIdMask)
		xml.writeIDAttribute(mask.ID(), opt)
		if mask.kind == MaskTypeAlpha {
			xml.writeSvgAttribute(AIdMaskType, "alpha")
		}
		xml.writeUnits(
			AIdMaskUnits,
			unitsUserSpaceOnUse,
			unitsObjectBoundingBox,
		)
		xml.writeRectAttrs(mask.rect)

		if mask.mask != nil {
			xml.writeFuncIRI(AIdMask, mask.mask.ID(), opt)
		}

		writeElements(&mask.root, false, opt, xml)

		xml.EndElement()
	}
	xml.EndElement() // end EId::Defs
}

// textPathAsPath is the Path::new of a text path that has_text_paths and
// write_text_path_paths in usvg src/writer.rs make.
func textPathAsPath(textPath *TextPath) (*Path, bool) {
	return newPath(
		textPath.ID(),
		true,
		nil,
		nil,
		PaintOrderFillAndStroke,
		DefaultShapeRendering(),
		textPath.path,
		path.DefaultTransform(),
	)
}

// hasTextPaths is has_text_paths in usvg src/writer.rs.
func hasTextPaths(parent *Group) bool {
	for _, node := range parent.children {
		switch n := node.(type) {
		case *Group:
			if hasTextPaths(n) {
				return true
			}
		case *Text:
			for i := range n.chunks {
				chunk := &n.chunks[i]
				if chunk.textFlow.Kind == TextFlowPath {
					if _, ok := textPathAsPath(chunk.textFlow.Path); ok {
						return true
					}
				}
			}
		}
		needPath := false
		node.Subroots(func(subroot *Group) {
			if !needPath && hasTextPaths(subroot) {
				needPath = true
			}
		})
		if needPath {
			return true
		}
	}
	return false
}

// writeTextPathPaths is write_text_path_paths in usvg src/writer.rs. It
// writes the `path` elements for text paths.
func writeTextPathPaths(parent *Group, opt *writeOptions, xml svgWriter) {
	for _, node := range parent.children {
		switch n := node.(type) {
		case *Group:
			writeTextPathPaths(n, opt, xml)
		case *Text:
			for i := range n.chunks {
				chunk := &n.chunks[i]
				if chunk.textFlow.Kind == TextFlowPath {
					if p, ok := textPathAsPath(chunk.textFlow.Path); ok {
						writePath(p, false, path.DefaultTransform(), "", false, opt, xml)
					}
				}
			}
		}

		node.Subroots(func(subroot *Group) { writeTextPathPaths(subroot, opt, xml) })
	}
}

// writeElements is write_elements in usvg src/writer.rs.
func writeElements(parent *Group, isClipPath bool, opt *writeOptions, xml svgWriter) {
	for _, n := range parent.children {
		writeElement(n, isClipPath, opt, xml)
	}
}

// writeElement is write_element in usvg src/writer.rs.
func writeElement(node Node, isClipPath bool, opt *writeOptions, xml svgWriter) {
	switch n := node.(type) {
	case *Path:
		writePath(n, isClipPath, path.DefaultTransform(), "", false, opt, xml)
	case *Image:
		xml.startSvgElement(EIdImage)
		if n.id != "" {
			xml.writeIDAttribute(n.id, opt)
		}

		xml.writeSvgAttributeF32(AIdWidth, n.Size().Width())
		xml.writeSvgAttributeF32(AIdHeight, n.Size().Height())

		xml.writeVisibility(n.visible)

		switch n.renderingMode {
		case ImageRenderingOptimizeQuality:
		case ImageRenderingOptimizeSpeed:
			xml.writeSvgAttribute(AIdImageRendering, "optimizeSpeed")
		case ImageRenderingSmooth:
			xml.WriteAttribute(AIdStyle.ToStr(), "image-rendering:smooth")
		case ImageRenderingHighQuality:
			xml.WriteAttribute(AIdStyle.ToStr(), "image-rendering:high-quality")
		case ImageRenderingCrispEdges:
			xml.WriteAttribute(AIdStyle.ToStr(), "image-rendering:crisp-edges")
		case ImageRenderingPixelated:
			xml.WriteAttribute(AIdStyle.ToStr(), "image-rendering:pixelated")
		}

		xml.writeImageData(n.kind)

		xml.EndElement()
	case *Group:
		writeGroupElement(n, isClipPath, opt, xml)
	case *Text:
		if !opt.preserveText {
			writeGroupElement(n.Flattened(), isClipPath, opt, xml)
			return
		}
		xml.startSvgElement(EIdText)

		if n.id != "" {
			xml.writeIDAttribute(n.id, opt)
		}

		xml.WriteAttribute("xml:space", "preserve")

		switch n.writingMode {
		case WritingModeLeftToRight:
		case WritingModeTopToBottom:
			xml.writeSvgAttribute(AIdWritingMode, "tb")
		}

		switch n.renderingMode {
		case TextRenderingOptimizeSpeed:
			xml.writeSvgAttribute(AIdTextRendering, "optimizeSpeed")
		case TextRenderingGeometricPrecision:
			xml.writeSvgAttribute(AIdTextRendering, "geometricPrecision")
		case TextRenderingOptimizeLegibility:
		}

		if anyNonZero(n.rotate) {
			xml.writeNumbers(AIdRotate, n.rotate)
		}

		if anyNonZero(n.dx) {
			xml.writeNumbers(AIdDx, n.dx)
		}

		if anyNonZero(n.dy) {
			xml.writeNumbers(AIdDy, n.dy)
		}

		xml.SetPreserveWhitespaces(true)

		for i := range n.chunks {
			chunk := &n.chunks[i]
			if chunk.textFlow.Kind == TextFlowPath {
				textPath := chunk.textFlow.Path
				xml.startSvgElement(EIdTextPath)

				xml.WriteAttribute("xlink:href", "#"+opt.idPrefix+textPath.ID())

				if textPath.startOffset != 0.0 {
					xml.writeSvgAttributeF32(AIdStartOffset, textPath.startOffset)
				}
			}

			xml.startSvgElement(EIdTspan)

			if x, ok := chunk.X(); ok {
				xml.writeSvgAttributeF32(AIdX, x)
			}

			if y, ok := chunk.Y(); ok {
				xml.writeSvgAttributeF32(AIdY, y)
			}

			switch chunk.anchor {
			case TextAnchorStart:
			case TextAnchorMiddle:
				xml.writeSvgAttribute(AIdTextAnchor, "middle")
			case TextAnchorEnd:
				xml.writeSvgAttribute(AIdTextAnchor, "end")
			}

			for j := range chunk.spans {
				span := &chunk.spans[j]
				type decoration struct {
					name  string
					style *TextDecorationStyle
				}
				var decorations []decoration
				for _, d := range []decoration{
					{"underline", span.decoration.underline},
					{"line-through", span.decoration.lineThrough},
					{"overline", span.decoration.overline},
				} {
					if d.style != nil {
						decorations = append(decorations, d)
					}
				}

				// Decorations need to be dumped BEFORE we write the actual span data
				// (so that for example stroke color of span doesn't affect the text
				// itself while baseline shifts need to be written after (since they are
				// affected by the font size)
				for _, d := range decorations {
					xml.startSvgElement(EIdTspan)
					xml.writeSvgAttribute(AIdTextDecoration, d.name)
					writeFill(d.style.fill, false, opt, xml)
					writeStroke(d.style.stroke, opt, xml)
				}

				writeSpan(isClipPath, opt, xml, chunk, span)

				// End for each tspan we needed to create for decorations
				for range decorations {
					xml.EndElement()
				}
			}
			xml.EndElement()

			// End textPath element
			if chunk.textFlow.Kind == TextFlowPath {
				xml.EndElement()
			}
		}

		xml.EndElement()
		xml.SetPreserveWhitespaces(false)
	}
}

// anyNonZero is the Rust expression `list.iter().any(|n| *n != 0.0)`.
func anyNonZero(list []float32) bool {
	for _, n := range list {
		if n != 0.0 {
			return true
		}
	}
	return false
}

// writeGroupElement is write_group_element in usvg src/writer.rs.
func writeGroupElement(g *Group, isClipPath bool, opt *writeOptions, xml svgWriter) {
	if isClipPath {
		// The `clipPath` element in SVG doesn't allow groups, only shapes and text.
		// The problem is that in `usvg` we can set a `clip-path` only on groups.
		// So in cases when a `clipPath` child has a `clip-path` as well,
		// it would be inside a group. And we have to skip this group during writing.
		//
		// Basically, the following SVG:
		//
		// <clipPath id="clip">
		//   <path clip-path="url(#clip-nested)"/>
		// </clipPath>
		//
		// will be represented in usvg as:
		//
		// <clipPath id="clip">
		//   <g clip-path="url(#clip-nested)">
		//      <path/>
		//   </g>
		// </clipPath>
		//
		//
		// Same with text. Text elements will be converted into groups,
		// but only the group's children should be written.
		for _, child := range g.children {
			switch c := child.(type) {
			case *Group:
				writeGroupElement(c, isClipPath, opt, xml)
			case *Path:
				clipID, hasClipID := "", false
				if g.clipPath != nil {
					clipID, hasClipID = g.clipPath.ID(), true
				}
				writePath(
					c,
					isClipPath,
					g.transform,
					clipID,
					hasClipID,
					opt,
					xml,
				)
			}
		}
		return
	}

	xml.startSvgElement(EIdG)
	if g.id != "" {
		xml.writeIDAttribute(g.id, opt)
	}

	if g.clipPath != nil {
		xml.writeFuncIRI(AIdClipPath, g.clipPath.ID(), opt)
	}

	if g.mask != nil {
		xml.writeFuncIRI(AIdMask, g.mask.ID(), opt)
	}

	if len(g.filters) != 0 {
		ids := make([]string, 0, len(g.filters))
		for _, filter := range g.filters {
			ids = append(ids, "url(#"+opt.idPrefix+filter.ID()+")")
		}
		xml.writeSvgAttribute(AIdFilter, strings.Join(ids, " "))
	}

	if g.opacity != path.NormalizedF32One {
		xml.writeSvgAttributeF32(AIdOpacity, g.opacity.Get())
	}

	xml.writeTransform(AIdTransform, g.transform, opt)

	if g.blendMode != BlendModeNormal || g.isolate {
		// For reasons unknown, `mix-blend-mode` and `isolation` must be written
		// as `style` attribute.
		isolation := "auto" //nolint:goconst // an SVG keyword, as upstream writes it
		if g.isolate {
			isolation = "isolate"
		}
		xml.WriteAttribute(
			AIdStyle.ToStr(),
			"mix-blend-mode:"+g.blendMode.String()+";isolation:"+isolation,
		)
	}

	writeElements(g, false, opt, xml)

	xml.EndElement()
}

// svgWriter is the trait XmlWriterExt for XmlWriter in usvg src/writer.rs.
//
// Port: the trait becomes a type that embeds the writer, with one method for
// each method of the trait. XmlWriterExt::write_svg_attribute takes any
// Display value. Go has writeSvgAttribute for a string and
// writeSvgAttributeF32 for a float.
type svgWriter struct {
	*xmlwriter.XMLWriter
}

// startSvgElement is XmlWriterExt::start_svg_element in usvg src/writer.rs.
func (w svgWriter) startSvgElement(id EId) {
	w.StartElement(id.ToStr())
}

// writeSvgAttribute is XmlWriterExt::write_svg_attribute in usvg
// src/writer.rs, for a string value.
func (w svgWriter) writeSvgAttribute(id AId, value string) {
	w.WriteAttribute(id.ToStr(), value)
}

// writeSvgAttributeF32 is XmlWriterExt::write_svg_attribute in usvg
// src/writer.rs, for an f32 value.
func (w svgWriter) writeSvgAttributeF32(id AId, value float32) {
	w.WriteAttribute(id.ToStr(), fmtF32(value))
}

// writeIDAttribute is XmlWriterExt::write_id_attribute in usvg
// src/writer.rs.
func (w svgWriter) writeIDAttribute(id string, opt *writeOptions) {
	if opt.idPrefix != "" {
		fullID := opt.idPrefix + id
		w.WriteAttribute("id", fullID)
	} else {
		w.WriteAttribute("id", id)
	}
}

// writeColor is XmlWriterExt::write_color in usvg src/writer.rs.
func (w svgWriter) writeColor(id AId, c Color) {
	const chars = "0123456789abcdef"

	int2hex := func(n uint8) (byte, byte) {
		return chars[n>>4], chars[n&0xf]
	}

	r1, r2 := int2hex(c.Red)
	g1, g2 := int2hex(c.Green)
	b1, b2 := int2hex(c.Blue)

	w.WriteAttributeRaw(id.ToStr(), func(buf []byte) []byte {
		return append(buf, '#', r1, r2, g1, g2, b1, b2)
	})
}

// writeUnits is XmlWriterExt::write_units in usvg src/writer.rs.
func (w svgWriter) writeUnits(id AId, u units, def units) {
	if u != def {
		w.WriteAttribute(id.ToStr(), u.String())
	}
}

// writeTransform is XmlWriterExt::write_transform in usvg src/writer.rs.
func (w svgWriter) writeTransform(id AId, ts Transform, opt *writeOptions) {
	if ts != path.DefaultTransform() {
		w.WriteAttributeRaw(id.ToStr(), func(buf []byte) []byte {
			buf = append(buf, "matrix("...)
			buf = writeNum(ts.SX, buf, opt.transformsPrecision)
			buf = append(buf, ' ')
			buf = writeNum(ts.KY, buf, opt.transformsPrecision)
			buf = append(buf, ' ')
			buf = writeNum(ts.KX, buf, opt.transformsPrecision)
			buf = append(buf, ' ')
			buf = writeNum(ts.SY, buf, opt.transformsPrecision)
			buf = append(buf, ' ')
			buf = writeNum(ts.TX, buf, opt.transformsPrecision)
			buf = append(buf, ' ')
			buf = writeNum(ts.TY, buf, opt.transformsPrecision)
			buf = append(buf, ")"...)
			return buf
		})
	}
}

// writeVisibility is XmlWriterExt::write_visibility in usvg src/writer.rs.
func (w svgWriter) writeVisibility(value bool) {
	if !value {
		w.WriteAttribute(AIdVisibility.ToStr(), "hidden")
	}
}

// writeFuncIRI is XmlWriterExt::write_func_iri in usvg src/writer.rs.
func (w svgWriter) writeFuncIRI(aid AId, id string, opt *writeOptions) {
	w.WriteAttribute(aid.ToStr(), "url(#"+opt.idPrefix+id+")")
}

// writeRectAttrs is XmlWriterExt::write_rect_attrs in usvg src/writer.rs.
func (w svgWriter) writeRectAttrs(r NonZeroRect) {
	w.writeSvgAttributeF32(AIdX, r.X())
	w.writeSvgAttributeF32(AIdY, r.Y())
	w.writeSvgAttributeF32(AIdWidth, r.Width())
	w.writeSvgAttributeF32(AIdHeight, r.Height())
}

// writeNumbers is XmlWriterExt::write_numbers in usvg src/writer.rs.
func (w svgWriter) writeNumbers(aid AId, list []float32) {
	w.WriteAttributeRaw(aid.ToStr(), func(buf []byte) []byte {
		for _, n := range list {
			buf = appendF32(buf, n)
			buf = append(buf, ' ')
		}

		if len(list) != 0 {
			buf = buf[:len(buf)-1]
		}
		return buf
	})
}

// writeFilterInput is XmlWriterExt::write_filter_input in usvg
// src/writer.rs.
func (w svgWriter) writeFilterInput(id AId, input InputFilter) {
	var value string
	switch input.Kind {
	case InputFilterSourceGraphic:
		value = "SourceGraphic" //nolint:goconst // an SVG keyword, as upstream writes it
	case InputFilterSourceAlpha:
		value = "SourceAlpha" //nolint:goconst // an SVG keyword, as upstream writes it
	case InputFilterReference:
		value = input.Reference
	}
	w.WriteAttribute(id.ToStr(), value)
}

// writeFilterPrimitiveAttrs is XmlWriterExt::write_filter_primitive_attrs in
// usvg src/writer.rs.
func (w svgWriter) writeFilterPrimitiveAttrs(parentRect NonZeroRect, fe *PrimitiveFilter) {
	if parentRect.X() != fe.Rect().X() {
		w.writeSvgAttributeF32(AIdX, fe.Rect().X())
	}
	if parentRect.Y() != fe.Rect().Y() {
		w.writeSvgAttributeF32(AIdY, fe.Rect().Y())
	}
	if parentRect.Width() != fe.Rect().Width() {
		w.writeSvgAttributeF32(AIdWidth, fe.Rect().Width())
	}
	if parentRect.Height() != fe.Rect().Height() {
		w.writeSvgAttributeF32(AIdHeight, fe.Rect().Height())
	}

	value := "linearRGB" //nolint:goconst // an SVG keyword, as upstream writes it
	if fe.colorInterpolation == ColorInterpolationFilterSRGB {
		value = "sRGB"
	}
	w.WriteAttribute(AIdColorInterpolationFilters.ToStr(), value)
}

// writeFilterTransferFunction is XmlWriterExt::write_filter_transfer_function
// in usvg src/writer.rs.
func (w svgWriter) writeFilterTransferFunction(eid EId, fe TransferFunctionFilter) {
	w.startSvgElement(eid)

	switch f := fe.(type) {
	case TransferFunctionFilterIdentity:
		w.writeSvgAttribute(AIdType, "identity")
	case TransferFunctionFilterTable:
		w.writeSvgAttribute(AIdType, "table")
		w.writeNumbers(AIdTableValues, f)
	case TransferFunctionFilterDiscrete:
		w.writeSvgAttribute(AIdType, "discrete")
		w.writeNumbers(AIdTableValues, f)
	case TransferFunctionFilterLinear:
		w.writeSvgAttribute(AIdType, "linear")
		w.writeSvgAttributeF32(AIdSlope, f.Slope)
		w.writeSvgAttributeF32(AIdIntercept, f.Intercept)
	case TransferFunctionFilterGamma:
		w.writeSvgAttribute(AIdType, "gamma")
		w.writeSvgAttributeF32(AIdAmplitude, f.Amplitude)
		w.writeSvgAttributeF32(AIdExponent, f.Exponent)
		w.writeSvgAttributeF32(AIdOffset, f.Offset)
	}

	w.EndElement()
}

// writeImageData is XmlWriterExt::write_image_data in usvg src/writer.rs.
func (w svgWriter) writeImageData(kind ImageKind) {
	var mime string
	var data []byte
	switch k := kind.(type) {
	case ImageKindJPEG:
		mime, data = "jpeg", k
	case ImageKindPNG:
		mime, data = "png", k
	case ImageKindGIF:
		mime, data = "gif", k
	case ImageKindWEBP:
		mime, data = "webp", k
	case ImageKindSVG:
		mime, data = "svg+xml", []byte(k.Tree.ToString())
	}

	w.WriteAttributeRaw("xlink:href", func(buf []byte) []byte {
		buf = append(buf, "data:image/"...)
		buf = append(buf, mime...)
		buf = append(buf, ";base64, "...)
		return base64.StdEncoding.AppendEncode(buf, data)
	})
}

// hasXlink is has_xlink in usvg src/writer.rs.
func hasXlink(parent *Group) bool {
	for _, node := range parent.children {
		switch n := node.(type) {
		case *Group:
			for _, filter := range n.filters {
				for i := range filter.primitives {
					if _, ok := filter.primitives[i].kind.(*ImageFilter); ok {
						return true
					}
				}
			}

			if mask := n.mask; mask != nil {
				if hasXlink(mask.Root()) {
					return true
				}

				if subMask := mask.mask; subMask != nil {
					if hasXlink(&subMask.root) {
						return true
					}
				}
			}

			if hasXlink(n) {
				return true
			}
		case *Image:
			return true
		case *Text:
			for i := range n.chunks {
				if n.chunks[i].textFlow.Kind == TextFlowPath {
					return true
				}
			}
		}

		present := false
		node.Subroots(func(root *Group) { present = present || hasXlink(root) })
		if present {
			return true
		}
	}

	return false
}

// writeBaseGrad is write_base_grad in usvg src/writer.rs.
func writeBaseGrad(g *BaseGradient, opt *writeOptions, xml svgWriter) {
	xml.writeUnits(AIdGradientUnits, g.units, unitsObjectBoundingBox)
	xml.writeTransform(AIdGradientTransform, g.transform, opt)

	switch g.spreadMethod {
	case SpreadMethodPad:
	case SpreadMethodReflect:
		xml.writeSvgAttribute(AIdSpreadMethod, "reflect")
	case SpreadMethodRepeat:
		xml.writeSvgAttribute(AIdSpreadMethod, "repeat")
	}

	for _, s := range g.stops {
		xml.startSvgElement(EIdStop)
		xml.writeSvgAttributeF32(AIdOffset, s.offset.Get())
		xml.writeColor(AIdStopColor, s.color)
		if s.opacity != path.NormalizedF32One {
			xml.writeSvgAttributeF32(AIdStopOpacity, s.opacity.Get())
		}

		xml.EndElement()
	}
}

// writePath is write_path in usvg src/writer.rs.
//
// Port: hasClipPath is false for a clip path of None.
func writePath(
	p *Path,
	isClipPath bool,
	pathTransform Transform,
	clipPath string,
	hasClipPath bool,
	opt *writeOptions,
	xml svgWriter,
) {
	xml.startSvgElement(EIdPath)
	if p.id != "" {
		xml.writeIDAttribute(p.id, opt)
	}

	writeFill(p.fill, isClipPath, opt, xml)
	writeStroke(p.stroke, opt, xml)

	xml.writeVisibility(p.visible)

	if p.paintOrder == PaintOrderStrokeAndFill {
		xml.writeSvgAttribute(AIdPaintOrder, "stroke")
	}

	switch p.renderingMode {
	case ShapeRenderingOptimizeSpeed:
		xml.writeSvgAttribute(AIdShapeRendering, "optimizeSpeed")
	case ShapeRenderingCrispEdges:
		xml.writeSvgAttribute(AIdShapeRendering, "crispEdges")
	case ShapeRenderingGeometricPrecision:
	}

	if hasClipPath {
		xml.writeFuncIRI(AIdClipPath, clipPath, opt)
	}

	xml.writeTransform(AIdTransform, pathTransform, opt)

	xml.WriteAttributeRaw("d", func(buf []byte) []byte {
		prec := opt.coordinatesPrecision
		segments := p.data.Segments()
		for {
			seg, ok := segments.Next()
			if !ok {
				break
			}
			switch seg.Kind {
			case path.PathSegmentMoveTo:
				pt := seg.Points[0]
				buf = append(buf, "M "...)
				buf = writeNum(pt.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.Y, buf, prec)
				buf = append(buf, ' ')
			case path.PathSegmentLineTo:
				pt := seg.Points[0]
				buf = append(buf, "L "...)
				buf = writeNum(pt.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.Y, buf, prec)
				buf = append(buf, ' ')
			case path.PathSegmentQuadTo:
				p1, pt := seg.Points[0], seg.Points[1]
				buf = append(buf, "Q "...)
				buf = writeNum(p1.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(p1.Y, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.Y, buf, prec)
				buf = append(buf, ' ')
			case path.PathSegmentCubicTo:
				p1, p2, pt := seg.Points[0], seg.Points[1], seg.Points[2]
				buf = append(buf, "C "...)
				buf = writeNum(p1.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(p1.Y, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(p2.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(p2.Y, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.X, buf, prec)
				buf = append(buf, ' ')
				buf = writeNum(pt.Y, buf, prec)
				buf = append(buf, ' ')
			case path.PathSegmentClose:
				buf = append(buf, "Z "...)
			}
		}

		// Port: Vec::pop does nothing on an empty buffer.
		if len(buf) != 0 {
			buf = buf[:len(buf)-1]
		}
		return buf
	})

	xml.EndElement()
}

// writeFill is write_fill in usvg src/writer.rs.
func writeFill(fill *Fill, isClipPath bool, opt *writeOptions, xml svgWriter) {
	if fill == nil {
		xml.writeSvgAttribute(AIdFill, "none")
		return
	}
	writePaint(AIdFill, fill.paint, opt, xml)

	if fill.opacity != path.NormalizedF32One {
		xml.writeSvgAttributeF32(AIdFillOpacity, fill.opacity.Get())
	}

	if fill.rule != FillRuleNonZero {
		name := AIdFillRule
		if isClipPath {
			name = AIdClipRule
		}

		xml.writeSvgAttribute(name, "evenodd")
	}
}

// writeStroke is write_stroke in usvg src/writer.rs.
func writeStroke(stroke *Stroke, opt *writeOptions, xml svgWriter) {
	if stroke == nil {
		// Always set `stroke` to `none` to override the parent value.
		// In 99.9% of the cases it's redundant, but a group with `filter` with `StrokePaint`
		// will set `stroke`, which will interfere with children nodes.
		xml.writeSvgAttribute(AIdStroke, "none")
		return
	}
	writePaint(AIdStroke, stroke.paint, opt, xml)

	if stroke.opacity != path.NormalizedF32One {
		xml.writeSvgAttributeF32(AIdStrokeOpacity, stroke.opacity.Get())
	}

	if !ApproxZeroUlps32(stroke.dashoffset, 4) {
		xml.writeSvgAttributeF32(AIdStrokeDashoffset, stroke.dashoffset)
	}

	if stroke.miterlimit != DefaultStrokeMiterlimit() {
		xml.writeSvgAttributeF32(AIdStrokeMiterlimit, stroke.miterlimit.Get())
	}

	if stroke.width.Get() != 1.0 {
		xml.writeSvgAttributeF32(AIdStrokeWidth, stroke.width.Get())
	}

	switch stroke.linecap {
	case LineCapButt:
	case LineCapRound:
		xml.writeSvgAttribute(AIdStrokeLinecap, "round")
	case LineCapSquare:
		xml.writeSvgAttribute(AIdStrokeLinecap, "square")
	}

	switch stroke.linejoin {
	case LineJoinMiter:
	case LineJoinMiterClip:
		xml.writeSvgAttribute(AIdStrokeLinejoin, "miter-clip")
	case LineJoinRound:
		xml.writeSvgAttribute(AIdStrokeLinejoin, "round")
	case LineJoinBevel:
		xml.writeSvgAttribute(AIdStrokeLinejoin, "bevel")
	}

	if stroke.dasharray != nil {
		xml.writeNumbers(AIdStrokeDasharray, stroke.dasharray)
	}
}

// writePaint is write_paint in usvg src/writer.rs.
func writePaint(aid AId, paint Paint, opt *writeOptions, xml svgWriter) {
	switch p := paint.(type) {
	case Color:
		xml.writeColor(aid, p)
	case *LinearGradient:
		xml.writeFuncIRI(aid, p.ID(), opt)
	case *RadialGradient:
		xml.writeFuncIRI(aid, p.ID(), opt)
	case *Pattern:
		xml.writeFuncIRI(aid, p.ID(), opt)
	}
}

// writeLightSource is write_light_source in usvg src/writer.rs.
func writeLightSource(light LightSourceFilter, xml svgWriter) {
	switch l := light.(type) {
	case DistantLightFilter:
		xml.startSvgElement(EIdFeDistantLight)
		xml.writeSvgAttributeF32(AIdAzimuth, l.Azimuth)
		xml.writeSvgAttributeF32(AIdElevation, l.Elevation)
	case PointLightFilter:
		xml.startSvgElement(EIdFePointLight)
		xml.writeSvgAttributeF32(AIdX, l.X)
		xml.writeSvgAttributeF32(AIdY, l.Y)
		xml.writeSvgAttributeF32(AIdZ, l.Z)
	case SpotLightFilter:
		xml.startSvgElement(EIdFeSpotLight)
		xml.writeSvgAttributeF32(AIdX, l.X)
		xml.writeSvgAttributeF32(AIdY, l.Y)
		xml.writeSvgAttributeF32(AIdZ, l.Z)
		xml.writeSvgAttributeF32(AIdPointsAtX, l.PointsAtX)
		xml.writeSvgAttributeF32(AIdPointsAtY, l.PointsAtY)
		xml.writeSvgAttributeF32(AIdPointsAtZ, l.PointsAtZ)
		xml.writeSvgAttributeF32(AIdSpecularExponent, l.SpecularExponent.Get())
		if l.LimitingConeAngle != nil {
			xml.writeSvgAttributeF32(AIdLimitingConeAngle, *l.LimitingConeAngle)
		}
	}

	xml.EndElement()
}

// powVec is POW_VEC in usvg src/writer.rs.
var powVec = [...]float32{
	1.0,
	10.0,
	100.0,
	1_000.0,
	10_000.0,
	100_000.0,
	1_000_000.0,
	10_000_000.0,
	100_000_000.0,
	1_000_000_000.0,
	10_000_000_000.0,
	100_000_000_000.0,
	1_000_000_000_000.0,
}

// writeNum is write_num in usvg src/writer.rs. It appends num to buf and
// returns buf.
func writeNum(num float32, buf []byte, precision uint8) []byte {
	// If number is an integer, it's faster to write it as i32.
	if ApproxZeroUlps32(fract32(num), 4) {
		return strconv.AppendInt(buf, int64(wide.TruncI32(num)), 10)
	}

	// Round numbers up to the specified precision to prevent writing
	// ugly numbers like 29.999999999999996.
	// It's not 100% correct, but differences are insignificant.
	//
	// Note that at least in Rust 1.64 the number formatting in debug and release modes
	// can be slightly different. So having a lower precision makes
	// our output and tests reproducible.
	v := round32(num*powVec[precision]) / powVec[precision]

	return appendF32(buf, v)
}

// fract32 is f32::fract of Rust: x minus its integer part, with the sign of
// x.
func fract32(x float32) float32 {
	return x - float32(math.Trunc(float64(x)))
}

// round32 is f32::round of Rust: the nearest integer, with a half rounded
// away from zero.
func round32(x float32) float32 {
	return float32(math.Round(float64(x)))
}

// fmtF32 is the impl Display for f32 of Rust, which `{}` uses: the shortest
// decimal text that reads back as the same f32, with no exponent. Negative
// zero is "-0", an infinity is "inf" or "-inf", and NaN is "NaN".
func fmtF32(x float32) string {
	return string(appendF32(nil, x))
}

// appendF32 appends the text of fmtF32 to buf and returns buf.
func appendF32(buf []byte, x float32) []byte {
	f := float64(x)
	switch {
	case math.IsNaN(f):
		return append(buf, "NaN"...)
	case math.IsInf(f, 1):
		return append(buf, "inf"...)
	case math.IsInf(f, -1):
		return append(buf, "-inf"...)
	}
	return strconv.AppendFloat(buf, f, 'f', -1, 32)
}

// writeSpan is write_span in usvg src/writer.rs. It writes all of the tspan
// attributes except for decorations.
func writeSpan(
	isClipPath bool,
	opt *writeOptions,
	xml svgWriter,
	chunk *TextChunk,
	span *TextSpan,
) {
	xml.startSvgElement(EIdTspan)

	fontFamilyToStr := func(fontFamily FontFamily) string {
		switch fontFamily.Kind {
		case svgtypes.FontFamilyMonospace:
			return "monospace"
		case svgtypes.FontFamilySerif:
			return "serif"
		case svgtypes.FontFamilySansSerif:
			return "sans-serif"
		case svgtypes.FontFamilyCursive:
			return "cursive"
		case svgtypes.FontFamilyFantasy:
			return "fantasy"
		}
		s := fontFamily.Name
		// Only quote if absolutely necessary
		if _, err := svgtypes.ParseFontFamilies(s); err == nil {
			return s
		}
		if opt.useSingleQuote {
			return "\"" + s + "\""
		}
		return "'" + s + "'"
	}

	if len(span.font.families) != 0 {
		families := make([]string, 0, len(span.font.families))
		for _, f := range span.font.families {
			families = append(families, fontFamilyToStr(f))
		}
		xml.writeSvgAttribute(AIdFontFamily, strings.Join(families, ", "))
	}

	switch span.font.style {
	case FontStyleNormal:
	case FontStyleItalic:
		xml.writeSvgAttribute(AIdFontStyle, "italic")
	case FontStyleOblique:
		xml.writeSvgAttribute(AIdFontStyle, "oblique")
	}

	if span.font.weight != 400 {
		xml.writeSvgAttribute(AIdFontWeight, strconv.FormatUint(uint64(span.font.weight), 10))
	}

	if span.font.stretch != FontStretchNormal {
		var name string
		switch span.font.stretch {
		case FontStretchCondensed:
			name = "condensed"
		case FontStretchExtraCondensed:
			name = "extra-condensed"
		case FontStretchUltraCondensed:
			name = "ultra-condensed"
		case FontStretchSemiCondensed:
			name = "semi-condensed"
		case FontStretchExpanded:
			name = "expanded"
		case FontStretchSemiExpanded:
			name = "semi-expanded"
		case FontStretchExtraExpanded:
			name = "extra-expanded"
		case FontStretchUltraExpanded:
			name = "ultra-expanded"
		default:
			panic("internal error: entered unreachable code")
		}
		xml.writeSvgAttribute(AIdFontStretch, name)
	}

	xml.writeSvgAttributeF32(AIdFontSize, span.fontSize.Get())

	xml.writeVisibility(span.visible)

	if span.letterSpacing != 0.0 {
		xml.writeSvgAttributeF32(AIdLetterSpacing, span.letterSpacing)
	}

	if span.wordSpacing != 0.0 {
		xml.writeSvgAttributeF32(AIdWordSpacing, span.wordSpacing)
	}

	if textLength, ok := span.TextLength(); ok {
		xml.writeSvgAttributeF32(AIdTextLength, textLength)
	}

	if span.lengthAdjust == LengthAdjustSpacingAndGlyphs {
		xml.writeSvgAttribute(AIdLengthAdjust, "spacingAndGlyphs")
	}

	if span.smallCaps {
		xml.writeSvgAttribute(AIdFontVariant, "small-caps")
	}

	if span.paintOrder == PaintOrderStrokeAndFill {
		xml.writeSvgAttribute(AIdPaintOrder, "stroke fill")
	}

	if !span.applyKerning {
		xml.WriteAttribute("style", "font-kerning:none")
	}

	if span.dominantBaseline != DominantBaselineAuto {
		var name string
		switch span.dominantBaseline {
		case DominantBaselineUseScript:
			name = "use-script"
		case DominantBaselineNoChange:
			name = "no-change"
		case DominantBaselineResetSize:
			name = "reset-size"
		case DominantBaselineTextBeforeEdge:
			name = "text-before-edge"
		case DominantBaselineMiddle:
			name = "middle"
		case DominantBaselineCentral:
			name = "central"
		case DominantBaselineTextAfterEdge:
			name = "text-after-edge"
		case DominantBaselineIdeographic:
			name = "ideographic"
		case DominantBaselineAlphabetic:
			name = "alphabetic"
		case DominantBaselineHanging:
			name = "hanging"
		case DominantBaselineMathematical:
			name = "mathematical"
		default:
			panic("internal error: entered unreachable code")
		}
		xml.writeSvgAttribute(AIdDominantBaseline, name)
	}

	if span.alignmentBaseline != AlignmentBaselineAuto {
		var name string
		switch span.alignmentBaseline {
		case AlignmentBaselineBaseline:
			name = "baseline"
		case AlignmentBaselineBeforeEdge:
			name = "before-edge"
		case AlignmentBaselineTextBeforeEdge:
			name = "text-before-edge"
		case AlignmentBaselineMiddle:
			name = "middle"
		case AlignmentBaselineCentral:
			name = "central"
		case AlignmentBaselineAfterEdge:
			name = "after-edge"
		case AlignmentBaselineTextAfterEdge:
			name = "text-after-edge"
		case AlignmentBaselineIdeographic:
			name = "ideographic"
		case AlignmentBaselineAlphabetic:
			name = "alphabetic"
		case AlignmentBaselineHanging:
			name = "hanging"
		case AlignmentBaselineMathematical:
			name = "mathematical"
		default:
			panic("internal error: entered unreachable code")
		}
		xml.writeSvgAttribute(AIdAlignmentBaseline, name)
	}

	writeFill(span.fill, isClipPath, opt, xml)
	writeStroke(span.stroke, opt, xml)

	for _, baselineShift := range span.baselineShift {
		xml.startSvgElement(EIdTspan)
		switch baselineShift.Kind {
		case BaselineShiftBaseline:
		case BaselineShiftNumber:
			xml.writeSvgAttributeF32(AIdBaselineShift, baselineShift.Number)
		case BaselineShiftSubscript:
			xml.writeSvgAttribute(AIdBaselineShift, "sub")
		case BaselineShiftSuperscript:
			xml.writeSvgAttribute(AIdBaselineShift, "super")
		}
	}

	curText := chunk.text[span.start:span.end]

	xml.WriteText(strings.ReplaceAll(curText, "&", "&amp;"))

	// End for each tspan we needed to create for baseline_shift
	for range span.baselineShift {
		xml.EndElement()
	}

	xml.EndElement()
}
