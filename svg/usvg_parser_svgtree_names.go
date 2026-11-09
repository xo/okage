// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

// This file ports usvg src/parser/svgtree/names.rs. Upstream generates that
// file with crates/usvg/codegen, and it finds a name with a perfect hash table
// and the crate siphasher. okage keeps the two enums in the order of upstream,
// with a Go map from the name to the value and an array from the value to the
// name, because the hash is never visible (D7). The private Map, hash,
// get_index, split and displace of upstream have no port.

// EId is EId in usvg src/parser/svgtree/names.rs.
//
// An element ID.
type EId uint8

// The values of EId, in the order of upstream.
const (
	EIdA EId = iota
	EIdCircle
	EIdClipPath
	EIdDefs
	EIdEllipse
	EIdFeBlend
	EIdFeColorMatrix
	EIdFeComponentTransfer
	EIdFeComposite
	EIdFeConvolveMatrix
	EIdFeDiffuseLighting
	EIdFeDisplacementMap
	EIdFeDistantLight
	EIdFeDropShadow
	EIdFeFlood
	EIdFeFuncA
	EIdFeFuncB
	EIdFeFuncG
	EIdFeFuncR
	EIdFeGaussianBlur
	EIdFeImage
	EIdFeMerge
	EIdFeMergeNode
	EIdFeMorphology
	EIdFeOffset
	EIdFePointLight
	EIdFeSpecularLighting
	EIdFeSpotLight
	EIdFeTile
	EIdFeTurbulence
	EIdFilter
	EIdG
	EIdImage
	EIdLine
	EIdLinearGradient
	EIdMarker
	EIdMask
	EIdPath
	EIdPattern
	EIdPolygon
	EIdPolyline
	EIdRadialGradient
	EIdRect
	EIdStop
	EIdStyle
	EIdSvg
	EIdSwitch
	EIdSymbol
	EIdText
	EIdTextPath
	EIdTref
	EIdTspan
	EIdUse
)

// eidNames holds the SVG name of each value of EId. It takes the place
// of the static ELEMENTS in usvg src/parser/svgtree/names.rs, which is a
// perfect hash table (D7).
var eidNames = [...]string{
	EIdA:                   "a",
	EIdCircle:              "circle",
	EIdClipPath:            "clipPath",
	EIdDefs:                "defs",
	EIdEllipse:             "ellipse",
	EIdFeBlend:             "feBlend",
	EIdFeColorMatrix:       "feColorMatrix",
	EIdFeComponentTransfer: "feComponentTransfer",
	EIdFeComposite:         "feComposite",
	EIdFeConvolveMatrix:    "feConvolveMatrix",
	EIdFeDiffuseLighting:   "feDiffuseLighting",
	EIdFeDisplacementMap:   "feDisplacementMap",
	EIdFeDistantLight:      "feDistantLight",
	EIdFeDropShadow:        "feDropShadow",
	EIdFeFlood:             "feFlood",
	EIdFeFuncA:             "feFuncA",
	EIdFeFuncB:             "feFuncB",
	EIdFeFuncG:             "feFuncG",
	EIdFeFuncR:             "feFuncR",
	EIdFeGaussianBlur:      "feGaussianBlur",
	EIdFeImage:             "feImage",
	EIdFeMerge:             "feMerge",
	EIdFeMergeNode:         "feMergeNode",
	EIdFeMorphology:        "feMorphology",
	EIdFeOffset:            "feOffset",
	EIdFePointLight:        "fePointLight",
	EIdFeSpecularLighting:  "feSpecularLighting",
	EIdFeSpotLight:         "feSpotLight",
	EIdFeTile:              "feTile",
	EIdFeTurbulence:        "feTurbulence",
	EIdFilter:              "filter",
	EIdG:                   "g",
	EIdImage:               "image",
	EIdLine:                "line",
	EIdLinearGradient:      "linearGradient",
	EIdMarker:              "marker",
	EIdMask:                "mask",
	EIdPath:                "path",
	EIdPattern:             "pattern",
	EIdPolygon:             "polygon",
	EIdPolyline:            "polyline",
	EIdRadialGradient:      "radialGradient",
	EIdRect:                "rect",
	EIdStop:                "stop",
	EIdStyle:               "style",
	EIdSvg:                 "svg",
	EIdSwitch:              "switch",
	EIdSymbol:              "symbol",
	EIdText:                "text", //nolint:goconst // an SVG keyword, as upstream writes it
	EIdTextPath:            "textPath",
	EIdTref:                "tref",
	EIdTspan:               "tspan",
	EIdUse:                 "use",
}

// eidByName maps the SVG name of each value of EId to the value. It
// takes the place of the static ELEMENTS in usvg
// src/parser/svgtree/names.rs (D7).
var eidByName = func() map[string]EId {
	m := make(map[string]EId, len(eidNames))
	for id, name := range eidNames {
		m[name] = EId(id)
	}
	return m
}()

// eidFromStr is EId::from_str in usvg src/parser/svgtree/names.rs. It
// returns the value with the SVG name text, and false for an unknown name.
func eidFromStr(text string) (EId, bool) {
	id, ok := eidByName[text]
	return id, ok
}

// ToStr is EId::to_str in usvg src/parser/svgtree/names.rs. It returns
// the original string.
//
// Port: upstream panics for a value that is not in the table. Go returns
// "EId(?)".
func (id EId) ToStr() string {
	if int(id) < len(eidNames) {
		return eidNames[id]
	}
	return "EId(?)"
}

// String is the impl Display and the impl Debug for EId in usvg
// src/parser/svgtree/names.rs. It returns the SVG name.
func (id EId) String() string {
	return id.ToStr()
}

// AId is AId in usvg src/parser/svgtree/names.rs.
//
// An attribute ID.
type AId uint8

// The values of AId, in the order of upstream.
const (
	AIdAlignmentBaseline AId = iota
	AIdAmplitude
	AIdAzimuth
	AIdBackgroundColor
	AIdBaseFrequency
	AIdBaselineShift
	AIdBias
	AIdClass
	AIdClip
	AIdClipPath
	AIdClipRule
	AIdClipPathUnits
	AIdColor
	AIdColorInterpolation
	AIdColorInterpolationFilters
	AIdColorProfile
	AIdColorRendering
	AIdCx
	AIdCy
	AIdD
	AIdDiffuseConstant
	AIdDirection
	AIdDisplay
	AIdDivisor
	AIdDominantBaseline
	AIdDx
	AIdDy
	AIdEdgeMode
	AIdElevation
	AIdEnableBackground
	AIdExponent
	AIdFill
	AIdFillOpacity
	AIdFillRule
	AIdFilter
	AIdFilterUnits
	AIdFloodColor
	AIdFloodOpacity
	AIdFont
	AIdFontFamily
	AIdFontFeatureSettings
	AIdFontKerning
	AIdFontOpticalSizing
	AIdFontSize
	AIdFontSizeAdjust
	AIdFontStretch
	AIdFontStyle
	AIdFontSynthesis
	AIdFontVariant
	AIdFontVariantCaps
	AIdFontVariantEastAsian
	AIdFontVariantLigatures
	AIdFontVariantNumeric
	AIdFontVariantPosition
	AIdFontVariationSettings
	AIdFontWeight
	AIdFr
	AIdFx
	AIdFy
	AIdGlyphOrientationHorizontal
	AIdGlyphOrientationVertical
	AIdGradientTransform
	AIdGradientUnits
	AIdHeight
	AIdHref
	AIdId //nolint:revive // AId::Id of upstream, named Type and Variant as the other values
	AIdImageRendering
	AIdIn
	AIdIn2
	AIdInlineSize
	AIdIntercept
	AIdIsolation
	AIdK1
	AIdK2
	AIdK3
	AIdK4
	AIdKernelMatrix
	AIdKernelUnitLength
	AIdKerning
	AIdLengthAdjust
	AIdLetterSpacing
	AIdLightingColor
	AIdLimitingConeAngle
	AIdLineHeight
	AIdMarkerEnd
	AIdMarkerMid
	AIdMarkerStart
	AIdMarkerHeight
	AIdMarkerUnits
	AIdMarkerWidth
	AIdMask
	AIdMaskBorder
	AIdMaskBorderMode
	AIdMaskBorderOutset
	AIdMaskBorderRepeat
	AIdMaskBorderSlice
	AIdMaskBorderSource
	AIdMaskBorderWidth
	AIdMaskClip
	AIdMaskComposite
	AIdMaskImage
	AIdMaskMode
	AIdMaskOrigin
	AIdMaskPosition
	AIdMaskSize
	AIdMaskType
	AIdMaskContentUnits
	AIdMaskUnits
	AIdMixBlendMode
	AIdMode
	AIdNumOctaves
	AIdOffset
	AIdOpacity
	AIdOperator
	AIdOrder
	AIdOrient
	AIdOverflow
	AIdPaintOrder
	AIdPath
	AIdPathLength
	AIdPatternContentUnits
	AIdPatternTransform
	AIdPatternUnits
	AIdPoints
	AIdPointsAtX
	AIdPointsAtY
	AIdPointsAtZ
	AIdPreserveAlpha
	AIdPreserveAspectRatio
	AIdPrimitiveUnits
	AIdR
	AIdRadius
	AIdRefX
	AIdRefY
	AIdRequiredExtensions
	AIdRequiredFeatures
	AIdResult
	AIdRotate
	AIdRx
	AIdRy
	AIdScale
	AIdSeed
	AIdShapeImageThreshold
	AIdShapeInside
	AIdShapeMargin
	AIdShapePadding
	AIdShapeRendering
	AIdShapeSubtract
	AIdSide
	AIdSlope
	AIdSpace
	AIdSpecularConstant
	AIdSpecularExponent
	AIdSpreadMethod
	AIdStartOffset
	AIdStdDeviation
	AIdStitchTiles
	AIdStopColor
	AIdStopOpacity
	AIdStroke
	AIdStrokeDasharray
	AIdStrokeDashoffset
	AIdStrokeLinecap
	AIdStrokeLinejoin
	AIdStrokeMiterlimit
	AIdStrokeOpacity
	AIdStrokeWidth
	AIdStyle
	AIdSurfaceScale
	AIdSystemLanguage
	AIdTableValues
	AIdTargetX
	AIdTargetY
	AIdTextAlign
	AIdTextAlignLast
	AIdTextAnchor
	AIdTextDecoration
	AIdTextDecorationColor
	AIdTextDecorationFill
	AIdTextDecorationLine
	AIdTextDecorationStroke
	AIdTextDecorationStyle
	AIdTextIndent
	AIdTextOrientation
	AIdTextOverflow
	AIdTextRendering
	AIdTextUnderlinePosition
	AIdTextLength
	AIdTransform
	AIdTransformBox
	AIdTransformOrigin
	AIdType
	AIdUnicodeBidi
	AIdUnicodeRange
	AIdValues
	AIdVectorEffect
	AIdViewBox
	AIdVisibility
	AIdWhiteSpace
	AIdWidth
	AIdWordSpacing
	AIdWritingMode
	AIdX
	AIdX1
	AIdX2
	AIdXChannelSelector
	AIdY
	AIdY1
	AIdY2
	AIdYChannelSelector
	AIdZ
)

// aidNames holds the SVG name of each value of AId. It takes the place
// of the static ATTRIBUTES in usvg src/parser/svgtree/names.rs, which is a
// perfect hash table (D7).
var aidNames = [...]string{
	AIdAlignmentBaseline:          "alignment-baseline",
	AIdAmplitude:                  "amplitude",
	AIdAzimuth:                    "azimuth",
	AIdBackgroundColor:            "background-color",
	AIdBaseFrequency:              "baseFrequency",
	AIdBaselineShift:              "baseline-shift",
	AIdBias:                       "bias",
	AIdClass:                      "class",
	AIdClip:                       "clip",
	AIdClipPath:                   "clip-path",
	AIdClipRule:                   "clip-rule",
	AIdClipPathUnits:              "clipPathUnits",
	AIdColor:                      "color", //nolint:goconst // an SVG keyword, as upstream writes it
	AIdColorInterpolation:         "color-interpolation",
	AIdColorInterpolationFilters:  "color-interpolation-filters",
	AIdColorProfile:               "color-profile",
	AIdColorRendering:             "color-rendering",
	AIdCx:                         "cx",
	AIdCy:                         "cy",
	AIdD:                          "d",
	AIdDiffuseConstant:            "diffuseConstant",
	AIdDirection:                  "direction",
	AIdDisplay:                    "display",
	AIdDivisor:                    "divisor",
	AIdDominantBaseline:           "dominant-baseline",
	AIdDx:                         "dx",
	AIdDy:                         "dy",
	AIdEdgeMode:                   "edgeMode",
	AIdElevation:                  "elevation",
	AIdEnableBackground:           "enable-background",
	AIdExponent:                   "exponent",
	AIdFill:                       "fill",
	AIdFillOpacity:                "fill-opacity",
	AIdFillRule:                   "fill-rule",
	AIdFilter:                     "filter",
	AIdFilterUnits:                "filterUnits",
	AIdFloodColor:                 "flood-color",
	AIdFloodOpacity:               "flood-opacity",
	AIdFont:                       "font",
	AIdFontFamily:                 "font-family",
	AIdFontFeatureSettings:        "font-feature-settings",
	AIdFontKerning:                "font-kerning",
	AIdFontOpticalSizing:          "font-optical-sizing",
	AIdFontSize:                   "font-size",
	AIdFontSizeAdjust:             "font-size-adjust",
	AIdFontStretch:                "font-stretch",
	AIdFontStyle:                  "font-style",
	AIdFontSynthesis:              "font-synthesis",
	AIdFontVariant:                "font-variant",
	AIdFontVariantCaps:            "font-variant-caps",
	AIdFontVariantEastAsian:       "font-variant-east-asian",
	AIdFontVariantLigatures:       "font-variant-ligatures",
	AIdFontVariantNumeric:         "font-variant-numeric",
	AIdFontVariantPosition:        "font-variant-position",
	AIdFontVariationSettings:      "font-variation-settings",
	AIdFontWeight:                 "font-weight",
	AIdFr:                         "fr",
	AIdFx:                         "fx",
	AIdFy:                         "fy",
	AIdGlyphOrientationHorizontal: "glyph-orientation-horizontal",
	AIdGlyphOrientationVertical:   "glyph-orientation-vertical",
	AIdGradientTransform:          "gradientTransform",
	AIdGradientUnits:              "gradientUnits",
	AIdHeight:                     "height",
	AIdHref:                       "href", //nolint:goconst // an SVG keyword, as upstream writes it
	AIdId:                         "id",
	AIdImageRendering:             "image-rendering",
	AIdIn:                         "in",
	AIdIn2:                        "in2",
	AIdInlineSize:                 "inline-size",
	AIdIntercept:                  "intercept",
	AIdIsolation:                  "isolation",
	AIdK1:                         "k1",
	AIdK2:                         "k2",
	AIdK3:                         "k3",
	AIdK4:                         "k4",
	AIdKernelMatrix:               "kernelMatrix",
	AIdKernelUnitLength:           "kernelUnitLength",
	AIdKerning:                    "kerning",
	AIdLengthAdjust:               "lengthAdjust",
	AIdLetterSpacing:              "letter-spacing",
	AIdLightingColor:              "lighting-color",
	AIdLimitingConeAngle:          "limitingConeAngle",
	AIdLineHeight:                 "line-height",
	AIdMarkerEnd:                  "marker-end",
	AIdMarkerMid:                  "marker-mid",
	AIdMarkerStart:                "marker-start",
	AIdMarkerHeight:               "markerHeight",
	AIdMarkerUnits:                "markerUnits",
	AIdMarkerWidth:                "markerWidth",
	AIdMask:                       "mask",
	AIdMaskBorder:                 "mask-border",
	AIdMaskBorderMode:             "mask-border-mode",
	AIdMaskBorderOutset:           "mask-border-outset",
	AIdMaskBorderRepeat:           "mask-border-repeat",
	AIdMaskBorderSlice:            "mask-border-slice",
	AIdMaskBorderSource:           "mask-border-source",
	AIdMaskBorderWidth:            "mask-border-width",
	AIdMaskClip:                   "mask-clip",
	AIdMaskComposite:              "mask-composite",
	AIdMaskImage:                  "mask-image",
	AIdMaskMode:                   "mask-mode",
	AIdMaskOrigin:                 "mask-origin",
	AIdMaskPosition:               "mask-position",
	AIdMaskSize:                   "mask-size",
	AIdMaskType:                   "mask-type",
	AIdMaskContentUnits:           "maskContentUnits",
	AIdMaskUnits:                  "maskUnits",
	AIdMixBlendMode:               "mix-blend-mode",
	AIdMode:                       "mode",
	AIdNumOctaves:                 "numOctaves",
	AIdOffset:                     "offset",
	AIdOpacity:                    "opacity",
	AIdOperator:                   "operator",
	AIdOrder:                      "order",
	AIdOrient:                     "orient",
	AIdOverflow:                   "overflow",
	AIdPaintOrder:                 "paint-order",
	AIdPath:                       "path",
	AIdPathLength:                 "pathLength",
	AIdPatternContentUnits:        "patternContentUnits",
	AIdPatternTransform:           "patternTransform",
	AIdPatternUnits:               "patternUnits",
	AIdPoints:                     "points",
	AIdPointsAtX:                  "pointsAtX",
	AIdPointsAtY:                  "pointsAtY",
	AIdPointsAtZ:                  "pointsAtZ",
	AIdPreserveAlpha:              "preserveAlpha",
	AIdPreserveAspectRatio:        "preserveAspectRatio",
	AIdPrimitiveUnits:             "primitiveUnits",
	AIdR:                          "r",
	AIdRadius:                     "radius",
	AIdRefX:                       "refX",
	AIdRefY:                       "refY",
	AIdRequiredExtensions:         "requiredExtensions",
	AIdRequiredFeatures:           "requiredFeatures",
	AIdResult:                     "result",
	AIdRotate:                     "rotate",
	AIdRx:                         "rx",
	AIdRy:                         "ry",
	AIdScale:                      "scale",
	AIdSeed:                       "seed",
	AIdShapeImageThreshold:        "shape-image-threshold",
	AIdShapeInside:                "shape-inside",
	AIdShapeMargin:                "shape-margin",
	AIdShapePadding:               "shape-padding",
	AIdShapeRendering:             "shape-rendering",
	AIdShapeSubtract:              "shape-subtract",
	AIdSide:                       "side",
	AIdSlope:                      "slope",
	AIdSpace:                      "space",
	AIdSpecularConstant:           "specularConstant",
	AIdSpecularExponent:           "specularExponent",
	AIdSpreadMethod:               "spreadMethod",
	AIdStartOffset:                "startOffset",
	AIdStdDeviation:               "stdDeviation",
	AIdStitchTiles:                "stitchTiles",
	AIdStopColor:                  "stop-color",
	AIdStopOpacity:                "stop-opacity",
	AIdStroke:                     "stroke",
	AIdStrokeDasharray:            "stroke-dasharray",
	AIdStrokeDashoffset:           "stroke-dashoffset",
	AIdStrokeLinecap:              "stroke-linecap",
	AIdStrokeLinejoin:             "stroke-linejoin",
	AIdStrokeMiterlimit:           "stroke-miterlimit",
	AIdStrokeOpacity:              "stroke-opacity",
	AIdStrokeWidth:                "stroke-width",
	AIdStyle:                      "style",
	AIdSurfaceScale:               "surfaceScale",
	AIdSystemLanguage:             "systemLanguage",
	AIdTableValues:                "tableValues",
	AIdTargetX:                    "targetX",
	AIdTargetY:                    "targetY",
	AIdTextAlign:                  "text-align",
	AIdTextAlignLast:              "text-align-last",
	AIdTextAnchor:                 "text-anchor",
	AIdTextDecoration:             "text-decoration",
	AIdTextDecorationColor:        "text-decoration-color",
	AIdTextDecorationFill:         "text-decoration-fill",
	AIdTextDecorationLine:         "text-decoration-line",
	AIdTextDecorationStroke:       "text-decoration-stroke",
	AIdTextDecorationStyle:        "text-decoration-style",
	AIdTextIndent:                 "text-indent",
	AIdTextOrientation:            "text-orientation",
	AIdTextOverflow:               "text-overflow",
	AIdTextRendering:              "text-rendering",
	AIdTextUnderlinePosition:      "text-underline-position",
	AIdTextLength:                 "textLength",
	AIdTransform:                  "transform",
	AIdTransformBox:               "transform-box",
	AIdTransformOrigin:            "transform-origin",
	AIdType:                       "type",
	AIdUnicodeBidi:                "unicode-bidi",
	AIdUnicodeRange:               "unicode-range",
	AIdValues:                     "values",
	AIdVectorEffect:               "vector-effect",
	AIdViewBox:                    "viewBox",
	AIdVisibility:                 "visibility",
	AIdWhiteSpace:                 "white-space",
	AIdWidth:                      "width",
	AIdWordSpacing:                "word-spacing",
	AIdWritingMode:                "writing-mode",
	AIdX:                          "x",
	AIdX1:                         "x1",
	AIdX2:                         "x2",
	AIdXChannelSelector:           "xChannelSelector",
	AIdY:                          "y",
	AIdY1:                         "y1",
	AIdY2:                         "y2",
	AIdYChannelSelector:           "yChannelSelector",
	AIdZ:                          "z",
}

// aidByName maps the SVG name of each value of AId to the value. It
// takes the place of the static ATTRIBUTES in usvg
// src/parser/svgtree/names.rs (D7).
var aidByName = func() map[string]AId {
	m := make(map[string]AId, len(aidNames))
	for id, name := range aidNames {
		m[name] = AId(id)
	}
	return m
}()

// aidFromStr is AId::from_str in usvg src/parser/svgtree/names.rs. It
// returns the value with the SVG name text, and false for an unknown name.
func aidFromStr(text string) (AId, bool) {
	id, ok := aidByName[text]
	return id, ok
}

// ToStr is AId::to_str in usvg src/parser/svgtree/names.rs. It returns
// the original string.
//
// Port: upstream panics for a value that is not in the table. Go returns
// "AId(?)".
func (id AId) ToStr() string {
	if int(id) < len(aidNames) {
		return aidNames[id]
	}
	return "AId(?)"
}

// String is the impl Display and the impl Debug for AId in usvg
// src/parser/svgtree/names.rs. It returns the SVG name.
func (id AId) String() string {
	return id.ToStr()
}
