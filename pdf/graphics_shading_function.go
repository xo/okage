// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// gradientType is GradientType in krilla src/graphics/shading_function.rs.
type gradientType uint8

// The cases of GradientType.
const (
	gradientTypeSweep gradientType = iota
	gradientTypeLinear
)

// The cases of GradientProperties in krilla
// src/graphics/shading_function.rs, which the derived Hash writes.
const (
	gradientPropertiesRadialAxial = iota
	gradientPropertiesPostScript
)

// radialAxialGradient is RadialAxialGradient in krilla
// src/graphics/shading_function.rs.
type radialAxialGradient struct {
	coords      []float32
	shadingType pdfwriter.FunctionShadingType
	stops       []Stop
	antiAlias   bool
	// extend is the Extend of the shading. It is not in krilla, which
	// extends both ends (D68).
	extend ExtendMode
}

// hash is the derived Hash of GradientProperties::RadialAxialGradient in
// krilla src/graphics/shading_function.rs, with the impl Hash of
// RadialAxialGradient. It writes the coordinates without their length, as
// upstream does.
func (g radialAxialGradient) hash(h *hasher) {
	h.usize(gradientPropertiesRadialAxial)
	for _, el := range g.coords {
		hashF32Bits(h, el)
	}
	h.usize(int(g.shadingType))
	hashStops(h, g.stops)
	h.bool(g.antiAlias)
	g.extend.hash(h)
}

// singleStopColor is the case RadialAxialGradient of
// GradientProperties::single_stop_color in krilla
// src/graphics/shading_function.rs.
func (g radialAxialGradient) singleStopColor() (Color, NormalizedF32, bool) {
	if len(g.stops) == 1 {
		return g.stops[0].Color, g.stops[0].Opacity, true
	}
	return nil, 0, false
}

// postScriptGradient is PostScriptGradient in krilla
// src/graphics/shading_function.rs.
type postScriptGradient struct {
	min float32
	max float32
	// cx is only used for a sweep gradient.
	cx float32
	// cy is only used for a sweep gradient.
	cy           float32
	stops        []Stop
	domain       Rect
	spreadMethod SpreadMethod
	gradientType gradientType
	antiAlias    bool
}

// hash is the derived Hash of GradientProperties::PostScriptGradient in
// krilla src/graphics/shading_function.rs, with the impl Hash of
// PostScriptGradient. Upstream leaves out cx and cy, and the port keeps
// that.
func (g postScriptGradient) hash(h *hasher) {
	h.usize(gradientPropertiesPostScript)
	hashF32Bits(h, g.min)
	hashF32Bits(h, g.max)
	hashStops(h, g.stops)
	h.rect(g.domain)
	h.usize(int(g.spreadMethod))
	h.usize(int(g.gradientType))
	h.bool(g.antiAlias)
}

// singleStopColor is the case PostScriptGradient of
// GradientProperties::single_stop_color in krilla
// src/graphics/shading_function.rs.
func (g postScriptGradient) singleStopColor() (Color, NormalizedF32, bool) {
	if len(g.stops) == 1 {
		return g.stops[0].Color, g.stops[0].Opacity, true
	}
	return nil, 0, false
}

// gradientProperties is GradientProperties in krilla
// src/graphics/shading_function.rs. Its types are radialAxialGradient and
// postScriptGradient.
type gradientProperties interface {
	// singleStopColor is GradientProperties::single_stop_color. It checks
	// whether the gradient can be encoded as a solid fill instead, and
	// returns the color and the opacity of the single stop.
	singleStopColor() (Color, NormalizedF32, bool)
	// hash is the derived Hash of GradientProperties.
	hash(h *hasher)
}

// The trait GradientPropertiesExt of krilla src/graphics/shading_function.rs
// is the method gradientProperties of LinearGradient, RadialGradient and
// SweepGradient. krilla unwraps the inverse of the transform of a linear
// gradient with the spread method repeat or reflect, and of a sweep
// gradient, and panics when the transform cannot be inverted. The method
// returns false in that case, and contentBuilder records
// ErrPDFInvalidGradient.

// getExpandedBBox is get_expanded_bbox in krilla
// src/graphics/shading_function.rs. It returns false when krilla panics:
// when the transform cannot be inverted, or the transformed box is not
// valid.
func getExpandedBBox(bbox Rect, shadingTransform Transform) (Rect, bool) {
	// We need to make sure the shading covers the whole bbox of the object
	// after the transform as been applied. In order to know that, we need
	// to calculate the resulting bbox from the inverted transform.
	inverted, ok := shadingTransform.Invert()
	if !ok {
		return Rect{}, false
	}
	transformed, ok := bbox.Transform(inverted)
	if !ok {
		return Rect{}, false
	}
	rectExpand(&bbox, transformed)
	return bbox, true
}

// toDegrees is f32::to_degrees of Rust.
func toDegrees(radians float32) float32 {
	// Use a constant for better precision.
	const pisIn180 float32 = 57.2957795130823208767981548141051703
	return radians * pisIn180
}

// getPointTs is get_point_ts in krilla src/graphics/shading_function.rs.
//
// When writing a PostScript shading, we assume that both points are on a
// horizontal line. Here, we calculate by how much we need to rotate the
// second point so that it is horizontal to the first point, as well as the
// position of the rotated point.
func getPointTs(start, end Point) (Transform, float32, float32) {
	dist := start.Distance(end)

	dx := end.X - start.X
	dy := end.Y - start.Y
	angle := toDegrees(crmath.Atan2_32(dy, dx))

	return path.TransformFromRotateAt(angle, start.X, start.Y),
		start.X,
		start.X + dist
}

// gradientProperties is GradientPropertiesExt::gradient_properties for
// LinearGradient in krilla src/graphics/shading_function.rs.
func (g LinearGradient) gradientProperties(bbox Rect) (gradientProperties, Transform, bool) {
	if g.SpreadMethod == SpreadMethodPad {
		return radialAxialGradient{
			coords:      []float32{g.X1, g.Y1, g.X2, g.Y2},
			shadingType: pdfwriter.FunctionShadingTypeAxial,
			stops:       slices.Clone(g.Stops),
			antiAlias:   g.AntiAlias,
			extend:      g.Extend,
		}, g.Transform, true
	}

	p1 := path.PointFromXY(g.X1, g.Y1)
	p2 := path.PointFromXY(g.X2, g.Y2)

	ts, min, max := getPointTs(p1, p2)
	domain, ok := getExpandedBBox(bbox, g.Transform.PreConcat(ts))
	if !ok {
		return nil, Transform{}, false
	}
	return postScriptGradient{
		min:          min,
		max:          max,
		cx:           0.0,
		cy:           0.0,
		stops:        slices.Clone(g.Stops),
		domain:       domain,
		spreadMethod: g.SpreadMethod,
		gradientType: gradientTypeLinear,
		antiAlias:    g.AntiAlias,
	}, g.Transform.PreConcat(ts), true
}

// gradientProperties is GradientPropertiesExt::gradient_properties for
// SweepGradient in krilla src/graphics/shading_function.rs.
func (g SweepGradient) gradientProperties(bbox Rect) (gradientProperties, Transform, bool) {
	min := g.StartAngle
	max := g.EndAngle

	transform := g.Transform

	domain, ok := getExpandedBBox(bbox, transform)
	if !ok {
		return nil, Transform{}, false
	}
	return postScriptGradient{
		min:          min,
		max:          max,
		cx:           g.Cx,
		cy:           g.Cy,
		stops:        slices.Clone(g.Stops),
		domain:       domain,
		spreadMethod: g.SpreadMethod,
		gradientType: gradientTypeSweep,
		antiAlias:    g.AntiAlias,
	}, transform, true
}

// gradientProperties is GradientPropertiesExt::gradient_properties for
// RadialGradient in krilla src/graphics/shading_function.rs. It cannot fail,
// and returns true.
func (g RadialGradient) gradientProperties(Rect) (gradientProperties, Transform, bool) {
	// TODO: Support other spread methods
	return radialAxialGradient{
		coords:      []float32{g.Fx, g.Fy, g.Fr, g.Cx, g.Cy, g.Cr},
		shadingType: pdfwriter.FunctionShadingTypeRadial,
		stops:       slices.Clone(g.Stops),
		antiAlias:   g.AntiAlias,
		extend:      g.Extend,
	}, g.Transform, true
}

// shadingFunction is ShadingFunction in krilla
// src/graphics/shading_function.rs, with its Repr. Upstream holds the Repr
// in an Arc, and the port holds it as a value.
type shadingFunction struct {
	properties   gradientProperties
	useOpacities bool
}

// newShadingFunction is ShadingFunction::new in krilla
// src/graphics/shading_function.rs.
func newShadingFunction(properties gradientProperties, useOpacities bool) shadingFunction {
	return shadingFunction{properties: properties, useOpacities: useOpacities}
}

// typeID names the type for sipHash.
func (shadingFunction) typeID() string { return "shadingFunction" }

// hash is the derived Hash of ShadingFunction in krilla
// src/graphics/shading_function.rs.
func (f shadingFunction) hash(h *hasher) {
	f.properties.hash(h)
	h.bool(f.useOpacities)
}

// serialize is Cacheable::serialize for ShadingFunction in krilla
// src/graphics/shading_function.rs. A PostScript function records the error
// ContainsPostScript.
//
// krilla panics when two stops have colors with different numbers of
// components, such as an RGB color and a CMYK color. pdf records
// ErrPDFMixedColorSpaces, which Document.Finish returns, and writes nothing.
// A shading of the opacities has one component for each stop, so it has no
// such error.
func (f shadingFunction) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	if !f.useOpacities {
		if stops := f.stops(); len(stops) != 0 {
			n := len(stops[0].Color.toPDFColor())
			for _, s := range stops[1:] {
				if len(s.Color.toPDFColor()) != n {
					sc.setError(fmt.Errorf("%w: a stop of %d components and a stop of %d components", okage.ErrPDFMixedColorSpaces, n, len(s.Color.toPDFColor())))
					return
				}
			}
		}
	}

	streamChunk := sc.newChunk()

	switch p := f.properties.(type) {
	case radialAxialGradient:
		shadingCS := shadingColorSpace(sc, p.stops[0].Color, f.useOpacities)
		registeredCS := sc.registerColorSpace(cc, shadingCS)
		chunk := cc.nonStream.shadingFunctions
		serializeAxialRadialShading(sc, chunk, rootRef, p, f.useOpacities, registeredCS)
	case postScriptGradient:
		sc.registerValidationError(validationError{kind: validationContainsPostScript, location: sc.location})
		shadingCS := shadingColorSpace(sc, p.stops[0].Color, f.useOpacities)
		registeredCS := sc.registerColorSpace(cc, shadingCS)
		chunk := cc.nonStream.shadingFunctions
		serializePostScriptShading(sc, chunk, streamChunk, rootRef, p, f.useOpacities, registeredCS)
	}

	// Note: The stream chunk might be empty.
	cc.streams.shadingFunctions = append(cc.streams.shadingFunctions, streamChunk)
}

// stops gives the stops of the gradient of the shading function. It is not
// in krilla.
func (f shadingFunction) stops() []Stop {
	switch p := f.properties.(type) {
	case radialAxialGradient:
		return p.stops
	case postScriptGradient:
		return p.stops
	}
	return nil
}

// newResource is Resourceable for ShadingFunction in krilla
// src/graphics/shading_function.rs.
func (shadingFunction) newResource(ref pdfwriter.Ref) shadingResource {
	return shadingResource{ref}
}

// shadingColorSpace is shading_color_space in krilla
// src/graphics/shading_function.rs. The shading of the opacities is grey.
func shadingColorSpace(sc *serializeContext, color Color, useOpacities bool) colorSpace {
	if useOpacities {
		return colorSpaceLuma(sc.serializeSettings().noDeviceCS)
	}
	return color.colorSpace(sc)
}

// serializePostScriptShading is serialize_postscript_shading in krilla
// src/graphics/shading_function.rs.
func serializePostScriptShading(sc *serializeContext, chunk, streamChunk *pdfwriter.Chunk, rootRef pdfwriter.Ref, postScriptGradient postScriptGradient, useOpacities bool, cs maybeDeviceColorSpace) {
	domain := postScriptGradient.domain

	functionRef := selectPostScriptFunction(postScriptGradient, streamChunk, sc, useOpacities)
	shading := chunk.FunctionShading(rootRef)
	shading.ShadingType(pdfwriter.FunctionShadingTypeFunction)

	cs.write(shading.Insert(pdfwriter.Name("ColorSpace")))

	// Write the identity matrix, because ghostscript has a bug where it
	// thinks the entry is mandatory.
	shading.Matrix([6]float32{1.0, 0.0, 0.0, 1.0, 0.0, 0.0})
	shading.AntiAlias(postScriptGradient.antiAlias)
	shading.Function(functionRef)

	shading.Domain([4]float32{domain.Left(), domain.Right(), domain.Top(), domain.Bottom()})
	shading.Finish()
}

// serializeAxialRadialShading is serialize_axial_radial_shading in krilla
// src/graphics/shading_function.rs.
func serializeAxialRadialShading(sc *serializeContext, chunk *pdfwriter.Chunk, rootRef pdfwriter.Ref, radialAxialGradient radialAxialGradient, useOpacities bool, cs maybeDeviceColorSpace) {
	functionRef := selectAxialRadialFunction(radialAxialGradient, chunk, sc, useOpacities)
	shading := chunk.FunctionShading(rootRef)
	if radialAxialGradient.shadingType == pdfwriter.FunctionShadingTypeRadial {
		shading.ShadingType(pdfwriter.FunctionShadingTypeRadial)
	} else {
		shading.ShadingType(pdfwriter.FunctionShadingTypeAxial)
	}

	cs.write(shading.Insert(pdfwriter.Name("ColorSpace")))

	shading.AntiAlias(radialAxialGradient.antiAlias)
	shading.Function(functionRef)
	shading.Coords(radialAxialGradient.coords...)
	// okage: krilla writes true for both ends (D68).
	shading.Extend(radialAxialGradient.extend.flags())
	shading.Finish()
}

// selectAxialRadialFunction is select_axial_radial_function in krilla
// src/graphics/shading_function.rs. It pads the stops at 0 and at 1, and
// writes one exponential function for two stops, or a stitching function
// for more.
func selectAxialRadialFunction(properties radialAxialGradient, chunk *pdfwriter.Chunk, sc *serializeContext, useOpacities bool) pdfwriter.Ref {
	stops := slices.Clone(properties.stops)

	if len(stops) > 0 {
		first := stops[0]
		if first.Offset.Get() != 0.0 {
			newStop := first
			newStop.Offset = NormalizedF32Zero
			stops = slices.Insert(stops, 0, newStop)
		}
	}

	if len(stops) > 0 {
		last := stops[len(stops)-1]
		if last.Offset.Get() != 1.0 {
			newStop := last
			newStop.Offset = NormalizedF32One
			stops = append(stops, newStop)
		}
	}

	if len(stops) == 2 {
		if useOpacities {
			return serializeExponential(
				[]float32{stops[0].Opacity.Get()},
				[]float32{stops[1].Opacity.Get()},
				chunk,
				sc,
			)
		}
		return serializeExponential(
			stops[0].Color.toPDFColor(),
			stops[1].Color.toPDFColor(),
			chunk,
			sc,
		)
	}
	return serializeStitching(stops, chunk, sc, useOpacities)
}

// selectPostScriptFunction is select_postscript_function in krilla
// src/graphics/shading_function.rs.
func selectPostScriptFunction(properties postScriptGradient, chunk *pdfwriter.Chunk, sc *serializeContext, useOpacities bool) pdfwriter.Ref {
	if properties.gradientType == gradientTypeLinear {
		return serializeLinearPostScript(properties, chunk, sc, useOpacities)
	}
	return serializeSweepPostScript(properties, chunk, sc, useOpacities)
}

// writePostScriptFunction is the end of serialize_sweep_postscript and of
// serialize_linear_postscript in krilla src/graphics/shading_function.rs,
// which is the same in both: it encodes the code and writes the function.
func writePostScriptFunction(properties postScriptGradient, code []pdfwriter.PostScriptOp, chunk *pdfwriter.Chunk, sc *serializeContext, rootRef pdfwriter.Ref, useOpacities bool) {
	encodedBuf := pdfwriter.PostScriptOpEncode(code)
	sc.registerLimits(encodedBuf.Limits())
	encoded := newFilterStreamFromContentStream(encodedBuf.AsSlice(), sc.serializeSettings()).
		finish(sc.serializeSettings())
	postScriptFunction := chunk.PostScriptFunction(rootRef, encoded.encodedData())
	encoded.writeFilters(postScriptFunction.Dict)
	postScriptFunction.Domain(
		properties.domain.Left(),
		properties.domain.Right(),
		properties.domain.Top(),
		properties.domain.Bottom(),
	)

	if useOpacities {
		postScriptFunction.Range(0.0, 1.0)
	} else {
		postScriptFunction.Range(0.0, 1.0, 0.0, 1.0, 0.0, 1.0)
	}
	postScriptFunction.Finish()
}

// serializeSweepPostScript is serialize_sweep_postscript in krilla
// src/graphics/shading_function.rs. The function of radial gradients of
// upstream is a comment that does not work yet, and has no port.
func serializeSweepPostScript(properties postScriptGradient, chunk *pdfwriter.Chunk, sc *serializeContext, useOpacities bool) pdfwriter.Ref {
	rootRef := sc.newRef()

	min := properties.min
	max := properties.max

	code := []pdfwriter.PostScriptOp{
		// Stack: x y
		// Shift by cy, so gradient actually starts from the center
		pdfwriter.PostScriptOpReal(properties.cy),
		pdfwriter.PostScriptOpSub,
		pdfwriter.PostScriptOpExch,
		// y x
		// Shift by cx, so gradient actually starts from the center
		pdfwriter.PostScriptOpReal(properties.cx),
		pdfwriter.PostScriptOpSub,
		// Make sure x is never 0.
		pdfwriter.PostScriptOpDup,
		pdfwriter.PostScriptOpDup,
		pdfwriter.PostScriptOpReal(0.0001),
		pdfwriter.PostScriptOpLt,
		pdfwriter.PostScriptOpExch,
		pdfwriter.PostScriptOpReal(-0.0001),
		pdfwriter.PostScriptOpGt,
		pdfwriter.PostScriptOpAnd,
		pdfwriter.PostScriptOpIf([]pdfwriter.PostScriptOp{pdfwriter.PostScriptOpPop, pdfwriter.PostScriptOpReal(0.0001)}),
		// Get the angle
		pdfwriter.PostScriptOpAtan,
	}

	encodeSpreadMethod(min, max, &code, properties.spreadMethod)
	encodePostScriptStops(properties.stops, min, max, &code, useOpacities)

	writePostScriptFunction(properties, code, chunk, sc, rootRef, useOpacities)
	return rootRef
}

// maxPostScriptStops is MAX_POSTSCRIPT_STOPS in krilla
// src/graphics/shading_function.rs.
const maxPostScriptStops = 97

// trimStops is trim_stops in krilla src/graphics/shading_function.rs. It
// samples maxPostScriptStops stops of a longer list.
func trimStops(stops []Stop) []Stop {
	n := len(stops)
	factor := float32(n) / float32(maxPostScriptStops)
	var curIndex float32

	getIndex := func(i float32) int { return int(math.Round(float64(i))) }

	var newStops []Stop

	for getIndex(curIndex) < n {
		newStops = append(newStops, stops[getIndex(curIndex)])
		curIndex += factor
	}

	return newStops
}

// serializeLinearPostScript is serialize_linear_postscript in krilla
// src/graphics/shading_function.rs.
func serializeLinearPostScript(properties postScriptGradient, chunk *pdfwriter.Chunk, sc *serializeContext, useOpacities bool) pdfwriter.Ref {
	rootRef := sc.newRef()

	min := properties.min
	max := properties.max

	code := []pdfwriter.PostScriptOp{
		// Stack: x y
		// Ignore the y coordinate. We account for it in the gradient
		// transform.
		pdfwriter.PostScriptOpPop,
		// x
	}

	encodeSpreadMethod(min, max, &code, properties.spreadMethod)
	encodePostScriptStops(properties.stops, min, max, &code, useOpacities)

	writePostScriptFunction(properties, code, chunk, sc, rootRef, useOpacities)
	return rootRef
}

// encodeSpreadMethod is encode_spread_method in krilla
// src/graphics/shading_function.rs.
//
// PostScript code that, given an arbitrary x coordinate, normalizes it to an
// x coordinate between min and max that yields the correct color, depending
// on the spread mode. In the case of the Pad spread methods, the coordinate
// will not be normalized since the PostScript functions assign the correct
// value by default.
//
//nolint:dupword // The comments of upstream show the stack, which repeats a value.
func encodeSpreadMethod(min, max float32, code *[]pdfwriter.PostScriptOp, spreadMethod SpreadMethod) {
	if spreadMethod == SpreadMethodPad {
		return
	}

	length := max - min

	reflectOp := pdfwriter.PostScriptOpPop
	if spreadMethod == SpreadMethodReflect {
		reflectOp = pdfwriter.PostScriptOpIf([]pdfwriter.PostScriptOp{
			pdfwriter.PostScriptOpInteger(2),
			pdfwriter.PostScriptOpIndex,
			pdfwriter.PostScriptOpExch,
			pdfwriter.PostScriptOpSub,
		})
	}

	*code = append(*code,
		// We do the following:
		// 1. Normalize by doing n = x - min.
		// 2. Calculate the "interval" we are in by doing
		//    i = floor(n / length)
		// 3. Calculate the offset by doing o = n - i * length
		// 4. If the spread method is repeat, we additionally calculate
		//    o = length - 0 if i % 2 == 1
		// 5. Calculate the final value with x_new = min + o.

		// Current stack:
		// x
		pdfwriter.PostScriptOpReal(length),
		pdfwriter.PostScriptOpReal(min),
		// x length min
		pdfwriter.PostScriptOpInteger(2),
		pdfwriter.PostScriptOpIndex,
		// x length min x
		pdfwriter.PostScriptOpInteger(1),
		pdfwriter.PostScriptOpIndex,
		// x length min x min
		pdfwriter.PostScriptOpSub,
		// x length min n
		pdfwriter.PostScriptOpDup,
		// x length min n n
		pdfwriter.PostScriptOpInteger(3),
		pdfwriter.PostScriptOpIndex,
		// x length min n n length
		pdfwriter.PostScriptOpDiv,
		// x length min n {n/length}
		pdfwriter.PostScriptOpFloor,
		// x length min n i
		pdfwriter.PostScriptOpExch,
		// x length min i n
		pdfwriter.PostScriptOpInteger(1),
		pdfwriter.PostScriptOpIndex,
		// x length min i n i
		pdfwriter.PostScriptOpInteger(4),
		pdfwriter.PostScriptOpIndex,
		// x length min i n i length
		pdfwriter.PostScriptOpMul,
		// x length min i n {i * length}
		pdfwriter.PostScriptOpSub,
		// x length min i o
		pdfwriter.PostScriptOpExch,
		// x length min o i
		pdfwriter.PostScriptOpCvi,
		pdfwriter.PostScriptOpAbs,
		// x length min o abs(i)
		pdfwriter.PostScriptOpInteger(2),
		pdfwriter.PostScriptOpMod,
		// x length min o {abs(i) % 2}
		// See https://github.com/google/skia/blob/645b77ce61449951cb9f3cf754b47d4977b68e1a/src/pdf/SkPDFGradientShader.cpp#L402-L408
		// for why we check > 0 instead of == 1.
		pdfwriter.PostScriptOpInteger(0),
		pdfwriter.PostScriptOpGt,
		// x length min o {(abs(i) % 2) > 0}
		reflectOp,
		// x length min o
		pdfwriter.PostScriptOpAdd,
		// x length x_new
		pdfwriter.PostScriptOpInteger(3),
		pdfwriter.PostScriptOpInteger(1),
		pdfwriter.PostScriptOpRoll,
		// x_new x length
		pdfwriter.PostScriptOpPop,
		pdfwriter.PostScriptOpPop,
		// x_new
	)
}

// encodePostScriptStops is encode_postscript_stops in krilla
// src/graphics/shading_function.rs.
//
// PostScript code that, given an x coordinate between the min and max of a
// gradient, returns the interpolated color value depending on where it lies
// within the stops.
func encodePostScriptStops(stops []Stop, min, max float32, code *[]pdfwriter.PostScriptOp, useOpacities bool) {
	// Our algorithm requires the stops to be padded.
	stops = slices.Clone(stops)

	// Most viewers have a nesting depth on how many `elseif` can be nested
	// (100 for mupdf, 128 for Chrome and 256 for Acrobat), so we trim the
	// stops if they are too large. this is of course a bit unfortunate, but
	// it should be pretty rare to have that many stops, and if we do, the
	// stops are most likely going to be very similar, so it's safe to just
	// sample from in-between.
	if len(stops) > maxPostScriptStops {
		stops = trimStops(stops)
	}

	if len(stops) > 0 {
		first := stops[0]
		first.Offset = NormalizedF32Zero
		stops = slices.Insert(stops, 0, first)
	}

	if len(stops) > 0 {
		last := stops[len(stops)-1]
		last.Offset = NormalizedF32One
		stops = append(stops, last)
	}

	encodeStopsImpl(stops, min, max, code, useOpacities)
}

// encodeTwoStops is the closure encode_two_stops of encode_stops_impl in
// krilla src/graphics/shading_function.rs.
func encodeTwoStops(c0, c1 []float32, min, max float32, code *[]pdfwriter.PostScriptOp) {
	if min == max {
		*code = append(*code, pdfwriter.PostScriptOpPop)
		for _, n := range c0 {
			*code = append(*code, pdfwriter.PostScriptOpReal(n))
		}
		return
	}

	// Sanity check that both stops have the same number of components.
	// krilla panics here. shadingFunction.serialize checks the stops first.

	// Normalize the x coordinate to be between 0 and 1.
	*code = append(*code,
		pdfwriter.PostScriptOpReal(min),
		pdfwriter.PostScriptOpSub,
		pdfwriter.PostScriptOpReal(max),
		pdfwriter.PostScriptOpReal(min),
		pdfwriter.PostScriptOpSub,
		pdfwriter.PostScriptOpDiv,
	)

	for i := range c0 {
		// Interpolate each color component c0 + x_norm * (x1 - c0).
		*code = append(*code,
			pdfwriter.PostScriptOpInteger(int32(i)),
			pdfwriter.PostScriptOpIndex,
			pdfwriter.PostScriptOpReal(c0[i]),
			pdfwriter.PostScriptOpExch,
			pdfwriter.PostScriptOpReal(c1[i]),
			pdfwriter.PostScriptOpReal(c0[i]),
			pdfwriter.PostScriptOpSub,
			pdfwriter.PostScriptOpMul,
			pdfwriter.PostScriptOpAdd,
		)
		// x_norm, c0, c1, ...
	}
	// Remove x_norm from the stack.
	*code = append(*code,
		pdfwriter.PostScriptOpInteger(int32(len(c0)+1)),
		pdfwriter.PostScriptOpInteger(-1),
		pdfwriter.PostScriptOpRoll,
		pdfwriter.PostScriptOpPop,
	)
	// c0, c1, c2, ...
}

// encodeStopsImpl is encode_stops_impl in krilla
// src/graphics/shading_function.rs. It nests one condition for each pair of
// stops.
func encodeStopsImpl(stops []Stop, min, max float32, code *[]pdfwriter.PostScriptOp, useOpacities bool) {
	if len(stops) == 1 {
		if useOpacities {
			*code = append(*code, pdfwriter.PostScriptOpReal(stops[0].Opacity.Get()))
		} else {
			for _, n := range stops[0].Color.toPDFColor() {
				*code = append(*code, pdfwriter.PostScriptOpReal(n))
			}
		}
		return
	}

	length := max - min
	stopsMin := min + float32(length*stops[0].Offset.Get())
	stopsMax := min + float32(length*stops[1].Offset.Get())
	// Write the if conditions to find the corresponding set of two stops.

	var ifStops []pdfwriter.PostScriptOp
	if useOpacities {
		encodeTwoStops(
			[]float32{stops[0].Opacity.Get()},
			[]float32{stops[1].Opacity.Get()},
			stopsMin,
			stopsMax,
			&ifStops,
		)
	} else {
		encodeTwoStops(
			stops[0].Color.toPDFColor(),
			stops[1].Color.toPDFColor(),
			stopsMin,
			stopsMax,
			&ifStops,
		)
	}
	var elseStops []pdfwriter.PostScriptOp
	encodeStopsImpl(stops[1:], min, max, &elseStops, useOpacities)

	*code = append(*code,
		pdfwriter.PostScriptOpDup,
		pdfwriter.PostScriptOpReal(stopsMax),
		pdfwriter.PostScriptOpLe,
		pdfwriter.PostScriptOpIfElse(ifStops, elseStops),
	)
}

// serializeStitching is serialize_stitching in krilla
// src/graphics/shading_function.rs. It writes one exponential function for
// each pair of stops, and the stitching function of them.
func serializeStitching(stops []Stop, chunk *pdfwriter.Chunk, sc *serializeContext, useOpacities bool) pdfwriter.Ref {
	rootRef := sc.newRef()
	var functions []pdfwriter.Ref
	var bounds []float32
	var encode []float32
	count := 0

	for i := 0; i+1 < len(stops); i++ {
		first, second := stops[i], stops[i+1]
		bounds = append(bounds, second.Offset.Get())

		var c0Components, c1Components []float32
		if useOpacities {
			c0Components = []float32{first.Opacity.Get()}
			c1Components = []float32{second.Opacity.Get()}
		} else {
			c0Components = first.Color.toPDFColor()
			c1Components = second.Color.toPDFColor()
		}

		count = len(c0Components)

		expRef := serializeExponential(c0Components, c1Components, chunk, sc)

		functions = append(functions, expRef)
		encode = append(encode, 0.0, 1.0)
	}

	if len(bounds) > 0 {
		bounds = bounds[:len(bounds)-1]
	}
	// okage: MuPDF 1.28 reads at most 256 functions of a stitching function,
	// so more functions go into a tree of stitching functions (D69).
	if len(functions) > maxStitchedFunctions {
		functions, bounds, encode = nestStitching(stops, functions, encode, count, chunk, sc)
	}
	stitchingFunction := chunk.StitchingFunction(rootRef)
	stitchingFunction.Domain(0.0, 1.0)
	stitchingFunction.Range(repeatUnitRange(count)...)
	stitchingFunction.Functions(functions...)
	stitchingFunction.Bounds(bounds...)
	stitchingFunction.Encode(encode...)
	stitchingFunction.Finish()

	return rootRef
}

// serializeExponential is serialize_exponential in krilla
// src/graphics/shading_function.rs.
func serializeExponential(c0, c1 []float32, chunk *pdfwriter.Chunk, sc *serializeContext) pdfwriter.Ref {
	rootRef := sc.newRef()
	// krilla panics when the two colors have different numbers of
	// components. shadingFunction.serialize checks the stops first.
	numComponents := len(c0)

	exp := chunk.ExponentialFunction(rootRef)

	exp.Range(repeatUnitRange(numComponents)...)
	exp.C0(c0...)
	exp.C1(c1...)
	exp.Domain(0.0, 1.0)
	exp.N(1.0)
	exp.Finish()
	return rootRef
}

// repeatUnitRange is [0.0, 1.0].repeat(n) of Rust: the range 0 to 1 for
// each of n components.
func repeatUnitRange(n int) []float32 {
	r := make([]float32, 0, 2*n)
	for range n {
		r = append(r, 0.0, 1.0)
	}
	return r
}
