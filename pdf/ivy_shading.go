// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports no file of krilla. It holds the Extend of the linear and
// radial gradients, and the mesh gradients, which the device of ivy needs and
// krilla does not have (D62 item 11, D68).

// ExtendMode says which ends of a linear or a radial gradient extend past the
// stops. It is the entry Extend of a shading of type 2 or 3. The zero value
// extends both ends, as krilla does. It is not in krilla (D68).
//
// A linear gradient takes it only with SpreadMethodPad. Past an end that does
// not extend, the gradient paints nothing. This is the decal spread mode of
// okage (D55), and the rule of NewRadialGradientExtend of okage (D64).
type ExtendMode uint8

// The values of ExtendMode.
const (
	// ExtendBoth extends both ends.
	ExtendBoth ExtendMode = iota
	// ExtendStart extends the start only.
	ExtendStart
	// ExtendEnd extends the end only.
	ExtendEnd
	// ExtendNone extends no end.
	ExtendNone
)

// flags gives the two flags of the entry Extend.
func (e ExtendMode) flags() [2]bool {
	switch e {
	case ExtendStart:
		return [2]bool{true, false}
	case ExtendEnd:
		return [2]bool{false, true}
	case ExtendNone:
		return [2]bool{false, false}
	}
	return [2]bool{true, true}
}

// hash writes the mode when it is not ExtendBoth, so that the hash of a
// gradient of krilla stays the hash of upstream.
func (e ExtendMode) hash(h *hasher) {
	if e != ExtendBoth {
		h.usize(int(e))
	}
}

// extendModeOf gives the ExtendMode of two flags.
func extendModeOf(start, end bool) ExtendMode {
	switch {
	case start && end:
		return ExtendBoth
	case start:
		return ExtendStart
	case end:
		return ExtendEnd
	}
	return ExtendNone
}

// MeshVertex is a corner of a triangle of a MeshGradient. It is not in krilla
// (D68).
type MeshVertex struct {
	// Point is the point of the corner.
	Point Point
	// Color is the color at the corner. Each vertex of a mesh must have a
	// color of the same color space.
	Color Color
	// Opacity is the opacity at the corner.
	Opacity NormalizedF32
}

// MeshTriangle is a triangle of a MeshGradient.
type MeshTriangle [3]MeshVertex

// MeshGradient is a paint that fills a mesh of triangles with a color at each
// corner, which the viewer interpolates. It is the shading of type 4 of PDF,
// the free-form Gouraud-shaded triangle mesh, and the MeshGradient of okage
// (D55). A later triangle paints over an earlier one. Outside the triangles,
// the paint paints nothing. It is not in krilla (D68).
//
// When a vertex has an opacity below 1, a soft mask holds the opacities, as a
// second shading of type 4 in the gray color space, as krilla does for the
// opacities of the stops of a gradient.
type MeshGradient struct {
	// Triangles are the triangles of the mesh.
	Triangles []MeshTriangle
	// Transform maps the space of the triangles to the space of the user.
	// The zero Transform of Go is not the identity, so set it.
	Transform Transform
	// AntiAlias is whether the viewer anti-aliases the shading.
	AntiAlias bool
}

// innerPaintMesh is the case of a MeshGradient in hashPaint. It is not a case
// of InnerPaint of krilla.
const innerPaintMesh = 100

// hashPaint writes the case and the mesh.
func (m MeshGradient) hashPaint(h *hasher) {
	h.usize(innerPaintMesh)
	hashMesh(h, m.Triangles)
	h.transform(m.Transform)
	h.bool(m.AntiAlias)
}

// hashMesh writes the triangles of a mesh.
func hashMesh(h *hasher, triangles []MeshTriangle) {
	h.usize(len(triangles))
	for _, t := range triangles {
		for _, v := range t {
			hashF32Bits(h, v.Point.X)
			hashF32Bits(h, v.Point.Y)
			if v.Color == nil {
				h.usize(0)
			} else {
				h.usize(1)
				v.Color.hash(h)
			}
			hashF32Bits(h, v.Opacity.Get())
		}
	}
}

// hasOpacity reports whether a vertex of the mesh has an opacity below 1.
func (m MeshGradient) hasOpacity() bool {
	for _, t := range m.Triangles {
		for _, v := range t {
			if v.Opacity != NormalizedF32One {
				return true
			}
		}
	}
	return false
}

// writeMesh sets the mesh as the paint of a fill or a stroke, as writeGradient
// of contentSetFillStrokeProperties does for a gradient. A mesh with no
// triangle, or with a vertex that has no color, records
// ErrPDFInvalidGradient and sets no paint.
func (b *contentBuilder) writeMesh(m MeshGradient, bounds Rect, sc *serializeContext, cc *chunkContainer, setPatternFn func(content *pdfwriter.Content, colorSpace string)) {
	if len(m.Triangles) == 0 {
		sc.setError(fmt.Errorf("%w: the mesh has no triangle", okage.ErrPDFInvalidGradient))
		return
	}
	for _, t := range m.Triangles {
		for _, v := range t {
			if v.Color == nil {
				sc.setError(fmt.Errorf("%w: a vertex of the mesh has no color", okage.ErrPDFInvalidGradient))
				return
			}
			if !v.Point.IsFinite() {
				sc.setError(fmt.Errorf("%w: the vertex %v of the mesh is not finite", okage.ErrPDFInvalidGradient, v.Point))
				return
			}
		}
	}

	pattern := meshPattern{
		shading:   meshShading{triangles: m.Triangles, antiAlias: m.AntiAlias},
		transform: b.curTransformWithRootTransform().PreConcat(m.Transform),
	}
	colorSpace := b.rdBuilder.registerResource(registerResourceable(sc, cc, pattern))

	if m.hasOpacity() {
		opacities := meshShading{triangles: m.Triangles, antiAlias: m.AntiAlias, useOpacities: true}
		mask := &Mask{
			stream:     meshMaskStream(sc, cc, m.Transform, opacities),
			maskType:   MaskTypeLuminosity,
			customBBox: &bounds,
		}
		state := newExtGState().mask(mask, sc, cc)
		b.usesMask = true
		ext := b.rdBuilder.registerResource(registerResourceable(sc, cc, state))
		b.content.SetParameters(pdfwriter.Name(ext))
	}

	setPatternFn(b.content, colorSpace)
}

// meshMaskStream draws the shading of the opacities of a mesh onto the
// surface of a new stream, as shadingMaskStream does for a gradient.
func meshMaskStream(sc *serializeContext, cc *chunkContainer, transform Transform, shading meshShading) *Stream {
	builder := newStreamBuilder(sc, cc, path.IdentityTransform())
	surface := builder.Surface()
	surface.PushTransform(transform)
	surface.bd.getMut().applyIsolatedOp(
		func(*contentBuilder) {},
		func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
			sh := sb.rdBuilder.registerResource(registerResourceable(sc, cc, shading))
			sb.content.Shading(pdfwriter.Name(sh))
		},
		sc,
		cc,
	)
	surface.Pop()
	surface.Finish()
	return builder.Finish()
}

// meshPattern is a shading pattern of a mesh, as shadingPattern is for a
// gradient.
type meshPattern struct {
	shading   meshShading
	transform Transform
}

// typeID names the type for sipHash.
func (meshPattern) typeID() string { return "meshPattern" }

// hash writes the shading and the transform.
func (p meshPattern) hash(h *hasher) {
	p.shading.hash(h)
	h.transform(p.transform)
}

// serialize writes the pattern, with the shading as its own object.
func (p meshPattern) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	shadingRef := sc.registerCacheable(cc, p.shading)
	sp := cc.nonStream.patterns.ShadingPattern(rootRef)
	sp.Pair(pdfwriter.Name("Shading"), shadingRef)
	sp.Matrix(transformToPDF(p.transform))
	sp.Finish()
}

// newResource gives the resource of the pattern.
func (meshPattern) newResource(ref pdfwriter.Ref) patternResource {
	return patternResource{ref}
}

// meshShading is a shading of type 4 of a mesh. With useOpacities, the colors
// of the shading are the opacities of the vertices, in the gray color space.
type meshShading struct {
	triangles    []MeshTriangle
	antiAlias    bool
	useOpacities bool
}

// typeID names the type for sipHash.
func (meshShading) typeID() string { return "meshShading" }

// hash writes the triangles and the flags.
func (s meshShading) hash(h *hasher) {
	hashMesh(h, s.triangles)
	h.bool(s.antiAlias)
	h.bool(s.useOpacities)
}

// The sizes of a vertex of a shading of type 4: a flag of 8 bits, two
// coordinates of 32 bits, and components of 16 bits.
const (
	meshBitsPerFlag       = 8
	meshBitsPerCoordinate = 32
	meshBitsPerComponent  = 16
)

// serialize writes the shading as a stream of vertices. Each vertex has the
// flag 0, so that each three vertices make a triangle of their own. Two
// colors with different numbers of components record ErrPDFMixedColorSpaces,
// as for a gradient (D66), and write nothing.
func (s meshShading) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	first := s.triangles[0][0].Color
	n := 1
	if !s.useOpacities {
		n = len(first.toPDFColor())
		for _, t := range s.triangles {
			for _, v := range t {
				if m := len(v.Color.toPDFColor()); m != n {
					sc.setError(fmt.Errorf("%w: a vertex of %d components and a vertex of %d components", okage.ErrPDFMixedColorSpaces, n, m))
					return
				}
			}
		}
	}

	// The range of the coordinates, which the entry Decode gives.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, t := range s.triangles {
		for _, v := range t {
			x, y := float64(v.Point.X), float64(v.Point.Y)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	minX, maxX = meshRange(minX, maxX)
	minY, maxY = meshRange(minY, maxY)

	data := make([]byte, 0, len(s.triangles)*3*(1+8+2*n))
	for _, t := range s.triangles {
		for _, v := range t {
			data = append(data, 0)
			data = binary.BigEndian.AppendUint32(data, meshCoordinate(float64(v.Point.X), minX, maxX))
			data = binary.BigEndian.AppendUint32(data, meshCoordinate(float64(v.Point.Y), minY, maxY))
			if s.useOpacities {
				data = binary.BigEndian.AppendUint16(data, meshComponent(v.Opacity.Get()))
				continue
			}
			for _, c := range v.Color.toPDFColor() {
				data = binary.BigEndian.AppendUint16(data, meshComponent(c))
			}
		}
	}

	shadingCS := shadingColorSpace(sc, first, s.useOpacities)
	registeredCS := sc.registerColorSpace(cc, shadingCS)

	decode := []float32{float32(minX), float32(maxX), float32(minY), float32(maxY)}
	for range n {
		decode = append(decode, 0, 1)
	}

	chunk := sc.newChunk()
	stream := newFilterStreamFromBinaryData(data).finish(sc.serializeSettings())
	shading := chunk.StreamShading(rootRef, stream.encodedData())
	stream.writeFilters(shading.Dict)
	shading.ShadingType(pdfwriter.StreamShadingTypeFreeformGouraud)
	registeredCS.write(shading.Insert(pdfwriter.Name("ColorSpace")))
	shading.BitsPerCoordinate(meshBitsPerCoordinate).
		BitsPerComponent(meshBitsPerComponent).
		BitsPerFlag(meshBitsPerFlag).
		Decode(decode...).
		AntiAlias(s.antiAlias)
	shading.Finish()
	cc.streams.shadingFunctions = append(cc.streams.shadingFunctions, chunk)
}

// newResource gives the resource of the shading.
func (meshShading) newResource(ref pdfwriter.Ref) shadingResource {
	return shadingResource{ref}
}

// meshRange gives the range of the entry Decode for the coordinates from lo
// to hi. The range is not empty, and its ends are float32 values that hold
// lo and hi.
func meshRange(lo, hi float64) (float64, float64) {
	l, h := float64(float32(lo)), float64(float32(hi))
	if l > lo {
		l = float64(math.Nextafter32(float32(l), float32(math.Inf(-1))))
	}
	if h < hi {
		h = float64(math.Nextafter32(float32(h), float32(math.Inf(1))))
	}
	if !(h > l) {
		h = l + 1
	}
	return l, h
}

// meshCoordinate encodes the coordinate v in the range from lo to hi as a
// number of 32 bits.
func meshCoordinate(v, lo, hi float64) uint32 {
	const top = math.MaxUint32
	q := math.Round((v - lo) / (hi - lo) * top)
	switch {
	case !(q > 0):
		return 0
	case q >= top:
		return top
	}
	return uint32(q)
}

// meshComponent encodes a component from 0 to 1 as a number of 16 bits.
func meshComponent(v float32) uint16 {
	q := math.Round(float64(v) * math.MaxUint16)
	switch {
	case !(q > 0):
		return 0
	case q >= math.MaxUint16:
		return math.MaxUint16
	}
	return uint16(q)
}

// drawMeshFill draws a path whose fill is a mesh with opacities, and returns
// false for any other fill. The path fills with the opaque mesh in a group,
// whose soft mask is the shading of the opacities. Poppler 26.11 does not
// apply a soft mask of the graphics state of a fill to a shading of type 4,
// as writeMesh writes it, but it applies the soft mask of a group. The stroke
// of the surface then draws on its own.
func (s *Surface) drawMeshFill(p *Path) bool {
	if s.fill == nil {
		return false
	}
	m, ok := s.fill.Paint.(MeshGradient)
	if !ok || !m.hasOpacity() || len(m.Triangles) == 0 {
		return false
	}
	opaque := m
	opaque.Triangles = make([]MeshTriangle, len(m.Triangles))
	for i, t := range m.Triangles {
		for k, v := range t {
			v.Opacity = NormalizedF32One
			opaque.Triangles[i][k] = v
		}
	}
	bbox := p.Bounds()
	opacities := meshShading{triangles: m.Triangles, antiAlias: m.AntiAlias, useOpacities: true}
	s.PushMask(&Mask{
		stream:     meshMaskStream(s.sc, s.cc, m.Transform, opacities),
		maskType:   MaskTypeLuminosity,
		customBBox: &bbox,
	})
	fill := *s.fill
	fill.Paint = opaque
	s.bd.getMut().drawPath(p, &fill, nil, s.sc, s.cc)
	s.Pop()
	if s.stroke != nil {
		s.bd.getMut().drawPath(p, nil, s.stroke, s.sc, s.cc)
	}
	return true
}
