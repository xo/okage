package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports no upstream file. It holds MeshGradient, a shader that
// tiny-skia does not have (D55). It fills a mesh of triangles with Gouraud
// shading: the color at a point is the interpolation of the colors at the
// three corners of its triangle.

// MeshVertex is a corner of a triangle of a MeshGradient. It holds the point
// of the corner and the color at that point.
type MeshVertex struct {
	Point Point
	Color Color
}

// MeshTriangle is a triangle of a MeshGradient. The order of its corners does
// not matter.
type MeshTriangle [3]MeshVertex

// MeshGradient is a shader that fills a mesh of triangles with a color for
// each corner (D55). It is the shading of the types 4 to 7 of PDF and
// PostScript, after a caller splits the patches of the types 6 and 7 into
// triangles. tiny-skia does not have it.
//
// The shader takes the center of each pixel, in the space of the mesh:
//
//   - If triangles cover the center, the last of them in the list gives the
//     color. The color is the interpolation of the colors of its corners.
//   - If no triangle covers the center, but the square of the pixel overlaps
//     triangles, the last of them gives the color of its point that is near
//     the center. An antialiased fill of the outline of the mesh then has no
//     dark border.
//   - Elsewhere, the color is transparent black.
//
// The shader interpolates the colors without premultiplied alpha, in the
// ColorSpace of the Paint, and then premultiplies them, as the gradients do.
//
// Two triangles that share an edge compute the same value for it, with the
// opposite sign. Each pixel thus takes its color from one triangle, and a
// fill of the mesh has no seam between its triangles. Fill a path that covers
// the mesh, such as its outline, with the shader. The path, the mask and the
// blend mode of the fill work as with every shader.
//
// A MeshGradient is never opaque. Its copies share the triangles, which do
// not change.
type MeshGradient struct {
	mesh      *meshData
	opacity   float32
	transform Transform
}

// NewMeshGradient makes a MeshGradient from triangles, with transform from
// the space of the triangles to the space of the user. The triangles paint in
// their order, so a later triangle covers an earlier one.
//
// It skips a triangle with no area or with a point that is not finite. It
// returns false when no triangle is left, and when transform is not
// invertible.
//
// The shader keeps a grid of the triangles, so that each pixel tests only the
// triangles near it. The grid holds at most about 16 entries for each
// triangle. The time to fill a pixel can still grow with the number of
// triangles that overlap at that pixel.
func NewMeshGradient(triangles []MeshTriangle, transform Transform) (Shader, bool) {
	if _, ok := transform.Invert(); !ok {
		return nil, false
	}
	mesh, ok := newMeshData(triangles)
	if !ok {
		return nil, false
	}
	return MeshGradient{
		mesh:      mesh,
		opacity:   1,
		transform: transform,
	}, true
}

// IsOpaque satisfies the Shader interface. A MeshGradient is never opaque,
// because it is transparent outside its triangles.
func (m MeshGradient) IsOpaque() bool {
	return false
}

// Transform satisfies the Shader interface. It returns the shader
// transformed by ts.
func (m MeshGradient) Transform(ts Transform) Shader {
	m.transform = m.transform.PostConcat(ts)
	return m
}

// ApplyOpacity satisfies the Shader interface. It returns the shader with
// the alpha of each color multiplied by opacity.
//
// opacity is clamped to the 0..=1 range.
func (m MeshGradient) ApplyOpacity(opacity float32) Shader {
	m.opacity = float32(m.opacity * path.Bound(opacity, 0.0, 1.0))
	return m
}

// pushStages satisfies the Shader interface. The pipeline maps each pixel to
// the space of the mesh, and stageMeshGradient gives its color.
func (m MeshGradient) pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool {
	if m.mesh == nil {
		return false
	}
	ts, ok := m.transform.Invert()
	if !ok || !ts.IsFinite() {
		warn(p.logger, "failed to invert a mesh gradient transform. Nothing will be rendered")
		return false
	}

	p.push(stageSeedShader)
	p.pushTransform(ts)

	colors := m.mesh.colors
	if cs != ColorSpaceLinear {
		colors = m.mesh.expandColors(cs)
	}

	// The half sides of a pixel in the space of the mesh.
	ux, uy := float32(0.5*ts.SX), float32(0.5*ts.KY)
	vx, vy := float32(0.5*ts.KX), float32(0.5*ts.SY)
	p.ctx.mesh = meshCtx{
		mesh:    m.mesh,
		colors:  colors,
		opacity: m.opacity,
		ux:      ux,
		uy:      uy,
		vx:      vx,
		vy:      vy,
		rx:      wide.LaneAbs(ux) + wide.LaneAbs(vx),
		ry:      wide.LaneAbs(uy) + wide.LaneAbs(vy),
	}
	p.push(stageMeshGradient)

	if !m.mesh.colorsAreOpaque || m.opacity < 1 {
		p.push(stagePremultiply)
	}
	return true
}

// The limits of the grid of a mesh.
const (
	// meshMaxGridSide is the largest number of columns, and of rows.
	meshMaxGridSide = 1024
	// meshEntriesPerTriangle is the number of entries of the grid for each
	// triangle, above which the grid gets coarser.
	meshEntriesPerTriangle = 16
)

// meshEdge is an edge of a triangle of a mesh. Its function is
// sign * ((x1 - x0) * (y - y0) - (y1 - y0) * (x - x0)), where (x0, y0) is the
// first point of the edge in the order of x and then y. Two triangles that
// share the edge thus compute the same value, with the opposite sign. The
// sign makes the function positive on the side of the triangle.
type meshEdge struct {
	x0, y0 float32
	dx, dy float32
	sign   float32
}

// newMeshEdge returns the edge from a to b, with the sign that makes the
// function positive at c. It returns false when the function is not finite
// or 0 at c.
func newMeshEdge(a, b, c Point) (meshEdge, bool) {
	if b.X < a.X || (b.X == a.X && b.Y < a.Y) {
		a, b = b, a
	}
	e := meshEdge{x0: a.X, y0: a.Y, dx: b.X - a.X, dy: b.Y - a.Y, sign: 1}
	f := e.eval(c.X, c.Y)
	switch {
	case !isFiniteF32(e.dx) || !isFiniteF32(e.dy) || !isFiniteF32(f) || f == 0:
		return meshEdge{}, false
	case f < 0:
		e.sign = -1
	}
	return e, true
}

// eval returns the value of the function of the edge at x and y.
func (e *meshEdge) eval(x, y float32) float32 {
	return e.sign * (float32(e.dx*(y-e.y0)) - float32(e.dy*(x-e.x0)))
}

// reach returns how far the function of the edge changes from the center of
// a pixel to its farthest corner. u and v are the half sides of the pixel.
func (e *meshEdge) reach(ux, uy, vx, vy float32) float32 {
	return wide.LaneAbs(float32(e.dx*uy)-float32(e.dy*ux)) +
		wide.LaneAbs(float32(e.dx*vy)-float32(e.dy*vx))
}

// meshTriangle is a triangle of a mesh, ready for the stage. edges[k] is the
// edge across from the corner k.
type meshTriangle struct {
	edges                  [3]meshEdge
	minX, minY, maxX, maxY float32
}

// meshData is the part of a MeshGradient that does not change: the
// triangles, their colors and the grid that finds them.
type meshData struct {
	triangles []meshTriangle
	// vertexColors holds the 3 colors of each triangle, as the caller gave
	// them.
	vertexColors []Color
	// colors holds 3 values for each triangle: the color of the corner 0, and
	// the colors of the corners 1 and 2 minus it. It is for ColorSpaceLinear.
	colors          []gradientColor
	colorsAreOpaque bool

	minX, minY, maxX, maxY float32
	cols, rows             int32
	invCellW, invCellH     float32
	// The triangles of the cell i are cellTriangles[cellStart[i]:cellStart[i+1]],
	// in the order of the triangles.
	cellStart     []int32
	cellTriangles []int32
}

// newMeshData prepares the triangles. It returns false when no triangle is
// left.
func newMeshData(triangles []MeshTriangle) (*meshData, bool) {
	m := &meshData{colorsAreOpaque: true}
	for i := range triangles {
		t := &triangles[i]
		p0, p1, p2 := t[0].Point, t[1].Point, t[2].Point
		e0, ok0 := newMeshEdge(p1, p2, p0)
		e1, ok1 := newMeshEdge(p2, p0, p1)
		e2, ok2 := newMeshEdge(p0, p1, p2)
		if !ok0 || !ok1 || !ok2 {
			continue
		}
		mt := meshTriangle{
			edges: [3]meshEdge{e0, e1, e2},
			minX:  min(p0.X, p1.X, p2.X),
			minY:  min(p0.Y, p1.Y, p2.Y),
			maxX:  max(p0.X, p1.X, p2.X),
			maxY:  max(p0.Y, p1.Y, p2.Y),
		}
		if len(m.triangles) == 0 {
			m.minX, m.minY, m.maxX, m.maxY = mt.minX, mt.minY, mt.maxX, mt.maxY
		} else {
			m.minX, m.minY = min(m.minX, mt.minX), min(m.minY, mt.minY)
			m.maxX, m.maxY = max(m.maxX, mt.maxX), max(m.maxY, mt.maxY)
		}
		m.triangles = append(m.triangles, mt)
		for _, v := range t {
			m.vertexColors = append(m.vertexColors, v.Color)
			m.colorsAreOpaque = m.colorsAreOpaque && v.Color.IsOpaque()
		}
	}
	if len(m.triangles) == 0 {
		return nil, false
	}
	m.colors = meshColors(m.vertexColors, ColorSpaceLinear)
	m.makeGrid()
	return m, true
}

// meshColors returns the colors of the stage for colors, expanded with cs.
func meshColors(colors []Color, cs ColorSpace) []gradientColor {
	out := make([]gradientColor, len(colors))
	for i := 0; i+2 < len(colors); i += 3 {
		c0 := gradientColorFromColor(cs.expandColor(colors[i]))
		c1 := gradientColorFromColor(cs.expandColor(colors[i+1]))
		c2 := gradientColorFromColor(cs.expandColor(colors[i+2]))
		out[i] = c0
		out[i+1] = newGradientColor(c1.r-c0.r, c1.g-c0.g, c1.b-c0.b, c1.a-c0.a)
		out[i+2] = newGradientColor(c2.r-c0.r, c2.g-c0.g, c2.b-c0.b, c2.a-c0.a)
	}
	return out
}

// expandColors returns the colors of the stage in cs.
func (m *meshData) expandColors(cs ColorSpace) []gradientColor {
	return meshColors(m.vertexColors, cs)
}

// makeGrid puts the triangles in a grid of cells over the bounds of the
// mesh. A cell holds each triangle whose bounds overlap it. The grid has
// about one cell for each triangle, and it gets coarser until it holds at
// most meshEntriesPerTriangle entries for each triangle.
func (m *meshData) makeGrid() {
	n := len(m.triangles)
	w := float64(m.maxX) - float64(m.minX)
	h := float64(m.maxY) - float64(m.minY)
	// The area of each triangle is not 0, so w and h are not 0.
	cols := clampGridSide(math.Sqrt(math.Min(float64(n)*w/h, meshMaxGridSide*meshMaxGridSide)))
	rows := clampGridSide(math.Ceil(float64(n) / float64(cols)))
	limit := int64(n) * meshEntriesPerTriangle
	for {
		m.cols, m.rows = cols, rows
		m.invCellW = float32(float64(cols) / w)
		m.invCellH = float32(float64(rows) / h)
		var entries int64
		for i := range m.triangles {
			x0, y0, x1, y1 := m.cellRange(&m.triangles[i])
			entries += int64(x1-x0+1) * int64(y1-y0+1)
		}
		if entries <= limit || (cols == 1 && rows == 1) {
			break
		}
		cols, rows = max(cols/2, 1), max(rows/2, 1)
	}

	m.cellStart = make([]int32, int(m.cols)*int(m.rows)+1)
	for i := range m.triangles {
		x0, y0, x1, y1 := m.cellRange(&m.triangles[i])
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				m.cellStart[y*m.cols+x+1]++
			}
		}
	}
	for i := 1; i < len(m.cellStart); i++ {
		m.cellStart[i] += m.cellStart[i-1]
	}
	m.cellTriangles = make([]int32, m.cellStart[len(m.cellStart)-1])
	next := make([]int32, len(m.cellStart)-1)
	copy(next, m.cellStart)
	for i := range m.triangles {
		x0, y0, x1, y1 := m.cellRange(&m.triangles[i])
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				c := y*m.cols + x
				m.cellTriangles[next[c]] = int32(i)
				next[c]++
			}
		}
	}
}

// clampGridSide returns v as a number of columns or rows of a grid.
func clampGridSide(v float64) int32 {
	if !(v >= 1) {
		return 1
	}
	return int32(math.Min(math.Round(v), meshMaxGridSide))
}

// cellRange returns the first and the last column and row of the cells that
// the bounds of t overlap.
func (m *meshData) cellRange(t *meshTriangle) (x0, y0, x1, y1 int32) {
	return m.cellX(t.minX), m.cellY(t.minY), m.cellX(t.maxX), m.cellY(t.maxY)
}

// cellX returns the column of the cells at x. It clamps x to the grid.
func (m *meshData) cellX(x float32) int32 {
	return min(max(wide.TruncI32(float32((x-m.minX)*m.invCellW)), 0), m.cols-1)
}

// cellY returns the row of the cells at y. It clamps y to the grid.
func (m *meshData) cellY(y float32) int32 {
	return min(max(wide.TruncI32(float32((y-m.minY)*m.invCellH)), 0), m.rows-1)
}

// cell returns the triangles of the cell at the column x and the row y.
func (m *meshData) cell(x, y int32) []int32 {
	i := y*m.cols + x
	return m.cellTriangles[m.cellStart[i]:m.cellStart[i+1]]
}

// meshCtx is the context of stageMeshGradient.
type meshCtx struct {
	mesh    *meshData
	colors  []gradientColor
	opacity float32
	// u and v are the half sides of a pixel in the space of the mesh. rx and
	// ry are the half sizes of the bounds of the pixel.
	ux, uy, vx, vy float32
	rx, ry         float32
}

// shade returns the color at x and y in the space of the mesh, without
// premultiplied alpha.
func (c *meshCtx) shade(x, y float32) (r, g, b, a float32) {
	m := c.mesh
	// The comparisons are false for NaN.
	if !(x >= m.minX-c.rx && x <= m.maxX+c.rx && y >= m.minY-c.ry && y <= m.maxY+c.ry) {
		return 0, 0, 0, 0
	}

	// The last triangle comes first. The loop has no iterator, so that the
	// stage uses no closure (D25).
	cell := m.cell(m.cellX(x), m.cellY(y))
	for j := range cell {
		i := cell[len(cell)-1-j]
		t := &m.triangles[i]
		f0 := t.edges[0].eval(x, y)
		if !(f0 >= 0) {
			continue
		}
		f1 := t.edges[1].eval(x, y)
		if !(f1 >= 0) {
			continue
		}
		f2 := t.edges[2].eval(x, y)
		if !(f2 >= 0) {
			continue
		}
		if s := f0 + f1 + f2; s > 0 {
			return c.color(i, f1/s, f2/s)
		}
	}

	// No triangle covers the center. Take the last triangle that overlaps the
	// pixel, at its point near the center.
	i, ok := c.overlap(x, y)
	if !ok {
		return 0, 0, 0, 0
	}
	t := &m.triangles[i]
	f0 := max(t.edges[0].eval(x, y), 0)
	f1 := max(t.edges[1].eval(x, y), 0)
	f2 := max(t.edges[2].eval(x, y), 0)
	s := f0 + f1 + f2
	if !(s > 0) {
		return 0, 0, 0, 0
	}
	return c.color(i, f1/s, f2/s)
}

// overlap returns the last triangle that overlaps the pixel with the center
// x and y. The pixel and the triangle overlap when no axis separates them:
// not x, not y, and not the normal of an edge of the triangle.
func (c *meshCtx) overlap(x, y float32) (int32, bool) {
	m := c.mesh
	left, right := x-c.rx, x+c.rx
	top, bottom := y-c.ry, y+c.ry
	best := int32(-1)
	for cy := m.cellY(top); cy <= m.cellY(bottom); cy++ {
		for cx := m.cellX(left); cx <= m.cellX(right); cx++ {
			cell := m.cell(cx, cy)
			for j := len(cell) - 1; j >= 0 && cell[j] > best; j-- {
				i := cell[j]
				if c.overlaps(&m.triangles[i], x, y, left, top, right, bottom) {
					best = i
					break
				}
			}
		}
	}
	return best, best >= 0
}

// overlaps reports whether the pixel with the center x and y and the bounds
// left, top, right and bottom overlaps t with an area that is not 0.
func (c *meshCtx) overlaps(t *meshTriangle, x, y, left, top, right, bottom float32) bool {
	if !(t.maxX > left && t.minX < right && t.maxY > top && t.minY < bottom) {
		return false
	}
	for k := range t.edges {
		e := &t.edges[k]
		if !(e.eval(x, y)+e.reach(c.ux, c.uy, c.vx, c.vy) > 0) {
			return false
		}
	}
	return true
}

// color returns the color of the triangle i at the weights t1 and t2 of its
// corners 1 and 2, with the opacity of the shader.
func (c *meshCtx) color(i int32, t1, t2 float32) (r, g, b, a float32) {
	k := 3 * int(i)
	c0, d1, d2 := &c.colors[k], &c.colors[k+1], &c.colors[k+2]
	r = unitClamp(c0.r + float32(t1*d1.r) + float32(t2*d2.r))
	g = unitClamp(c0.g + float32(t1*d1.g) + float32(t2*d2.g))
	b = unitClamp(c0.b + float32(t1*d1.b) + float32(t2*d2.b))
	a = unitClamp(c0.a + float32(t1*d1.a) + float32(t2*d2.a))
	return r, g, b, float32(a * c.opacity)
}

// unitClamp clamps v to the 0..=1 range.
func unitClamp(v float32) float32 {
	return wide.LaneMin(wide.LaneMax(v, 0), 1.0)
}
