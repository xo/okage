// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"encoding/binary"
	"image"
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// fuzzReader reads the values of a fuzz input. It gives zeros after the end
// of the input.
type fuzzReader struct {
	data []byte
}

func (r *fuzzReader) u8() uint8 {
	if len(r.data) == 0 {
		return 0
	}
	v := r.data[0]
	r.data = r.data[1:]
	return v
}

func (r *fuzzReader) u32() uint32 {
	var b [4]byte
	for i := range b {
		b[i] = r.u8()
	}
	return binary.LittleEndian.Uint32(b[:])
}

// f32 reads a float32 from its bits, so that the input can give NaN, the
// infinities and the largest floats.
func (r *fuzzReader) f32() float32 {
	return math.Float32frombits(r.u32())
}

// rect reads a rectangle, or nil.
func (r *fuzzReader) rect() *Rect {
	if r.u8()%2 == 0 {
		return nil
	}
	rect, ok := path.RectFromXYWH(r.f32(), r.f32(), r.f32(), r.f32())
	if !ok {
		return nil
	}
	return &rect
}

// color reads a solid color.
func (r *fuzzReader) color() Color {
	switch r.u8() % 3 {
	case 0:
		return NewColorRGB(r.u8(), r.u8(), r.u8())
	case 1:
		return NewColorLuma(r.u8())
	default:
		return NewColorCMYK(r.u8(), r.u8(), r.u8(), r.u8())
	}
}

// stops reads at most 7 stops of a gradient, whose colors can be in
// different color spaces.
func (r *fuzzReader) stops() []Stop {
	n := int(r.u8() % 8)
	if n == 7 {
		// More than 257 stops make a tree of stitching functions (D69).
		n = 250 + int(r.u8())
	}
	stops := make([]Stop, 0, n)
	for range n {
		stops = append(stops, Stop{Offset: r.opacity(), Color: r.color(), Opacity: r.opacity()})
	}
	return stops
}

// transform reads a transform from six floats.
func (r *fuzzReader) transform() Transform {
	return path.TransformFromRow(r.f32(), r.f32(), r.f32(), r.f32(), r.f32(), r.f32())
}

// paint reads a paint: a color, a gradient, or a pattern whose stream draws
// its own calls, up to a depth of 3.
func (r *fuzzReader) paint(s *Surface, depth int) Paint {
	switch r.u8() % 8 {
	case 0:
		return LinearGradient{
			X1: r.f32(), Y1: r.f32(), X2: r.f32(), Y2: r.f32(), Transform: r.transform(),
			SpreadMethod: SpreadMethod(r.u8() % 3), Stops: r.stops(), AntiAlias: r.u8()%2 == 0,
			Extend: ExtendMode(r.u8() % 4),
		}
	case 1:
		return RadialGradient{
			Fx: r.f32(), Fy: r.f32(), Fr: r.f32(), Cx: r.f32(), Cy: r.f32(), Cr: r.f32(), Transform: r.transform(),
			SpreadMethod: SpreadMethod(r.u8() % 3), Stops: r.stops(), AntiAlias: r.u8()%2 == 0,
			Extend: ExtendMode(r.u8() % 4),
		}
	case 2:
		return SweepGradient{
			Cx: r.f32(), Cy: r.f32(), StartAngle: r.f32(), EndAngle: r.f32(), Transform: r.transform(),
			SpreadMethod: SpreadMethod(r.u8() % 3), Stops: r.stops(), AntiAlias: r.u8()%2 == 0,
		}
	case 3:
		if depth < 3 {
			b := s.StreamBuilder()
			fuzzSurface(r, b.Surface(), depth+1)
			return Pattern{Stream: b.Finish(), Transform: r.transform(), Width: r.f32(), Height: r.f32()}
		}
	case 4:
		return r.mesh()
	}
	return r.color()
}

// mesh reads a mesh of at most 3 triangles, whose vertices can have no
// color, colors in different color spaces, or points that are not finite.
func (r *fuzzReader) mesh() MeshGradient {
	m := MeshGradient{Triangles: make([]MeshTriangle, r.u8()%4), Transform: r.transform(), AntiAlias: r.u8()%2 == 0}
	for i := range m.Triangles {
		for k := range 3 {
			v := MeshVertex{Point: path.PointFromXY(r.coord(), r.coord()), Opacity: r.opacity()}
			if r.u8()%16 != 0 {
				v.Color = r.color()
			}
			m.Triangles[i][k] = v
		}
	}
	return m
}

// okageColor reads a color of okage.
func (r *fuzzReader) okageColor() okage.Color {
	return okage.ColorFromRGBA8(r.u8(), r.u8(), r.u8(), r.u8())
}

// shader reads a shader of okage for Surface.PaintFromShader, or nil.
func (r *fuzzReader) shader() okage.Shader {
	// The positions can be in any order, and NaN (D68).
	var stops []okage.GradientStop
	for range r.u8() % 4 {
		stops = append(stops, okage.NewGradientStop(r.f32(), r.okageColor()))
	}
	pt := func() okage.Point { return path.PointFromXY(r.coord(), r.coord()) }
	mode := okage.SpreadMode(r.u8() % 4)
	var s okage.Shader
	switch r.u8() % 7 {
	case 0:
		return okage.SolidColor(r.okageColor())
	case 1:
		s, _ = okage.NewLinearGradient(pt(), pt(), stops, mode, r.transform())
	case 2:
		s, _ = okage.NewRadialGradient(pt(), r.coord(), pt(), r.coord(), stops, mode, r.transform())
	case 3:
		s, _ = okage.NewRadialGradientExtend(pt(), r.coord(), pt(), r.coord(), stops, r.u8()%2 == 0, r.u8()%2 == 0, r.transform())
	case 4:
		s, _ = okage.NewSweepGradient(pt(), r.coord(), r.coord(), stops, mode, r.transform())
	case 5:
		tris := make([]okage.MeshTriangle, r.u8()%4)
		for i := range tris {
			for k := range 3 {
				tris[i][k] = okage.MeshVertex{Point: pt(), Color: r.okageColor()}
			}
		}
		s, _ = okage.NewMeshGradient(tris, r.transform())
	default:
		pm, ok := okage.NewPixmap(uint32(r.u8()%4), uint32(r.u8()%4))
		if !ok {
			return nil
		}
		s = okage.NewPattern(pm.AsRef(), mode, okage.FilterQuality(r.u8()%3), r.f32(), r.transform())
	}
	return s
}

// stencil reads a stencil of at most 3 by 3 pixels.
func (r *fuzzReader) stencil() *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, int(r.u8()%4), int(r.u8()%4)))
	// A stencil of 0 and 255 is an image mask with a color (D69).
	binary := r.u8()%2 == 0
	for i := range m.Pix {
		m.Pix[i] = r.u8()
		if binary {
			m.Pix[i] = 0xff * (m.Pix[i] & 1)
		}
	}
	return m
}

// mask reads a mask whose stream draws its own calls, with a backdrop and a
// transfer function or not.
func (r *fuzzReader) mask(s *Surface, depth int) *Mask {
	b := s.StreamBuilder()
	fuzzSurface(r, b.Surface(), depth+1)
	m := NewMask(b.Finish(), MaskType(r.u8()%2))
	flags := r.u8()
	if flags&1 != 0 {
		m = m.WithBackdrop(NewColorRGB(r.u8(), r.u8(), r.u8()))
	}
	if flags&2 != 0 {
		var table [256]uint8
		for i := range table {
			table[i] = uint8(i) ^ flags
		}
		m = m.WithTransfer(&table)
	}
	return m
}

// opacity reads a NormalizedF32.
func (r *fuzzReader) opacity() NormalizedF32 {
	return NormalizedF32(float32(r.u8()) / 255)
}

// coord reads a coordinate of a path. Half of the coordinates are small
// integers, so that the curves of a path are often in a line, and the other
// half are any float32.
func (r *fuzzReader) coord() float32 {
	if r.u8()%2 == 0 {
		return float32(int8(r.u8()))
	}
	return r.f32()
}

// path reads a path of at most 8 segments, or nil.
func (r *fuzzReader) path() *Path {
	b := path.NewPathBuilder()
	for range r.u8() % 8 {
		switch r.u8() % 5 {
		case 0:
			b.MoveTo(r.coord(), r.coord())
		case 1:
			b.LineTo(r.coord(), r.coord())
		case 2:
			b.QuadTo(r.coord(), r.coord(), r.coord(), r.coord())
		case 3:
			b.CubicTo(r.coord(), r.coord(), r.coord(), r.coord(), r.coord(), r.coord())
		default:
			b.Close()
		}
	}
	p, _ := b.Finish()
	return p
}

// fuzzSurface makes random calls of a surface, with pushes and pops that do
// not have to match, and finishes it. A mask draws its own calls, up to a
// depth of 3. The calls include the additions for ivy of D68: the groups,
// the stroke adjustment, the stencils, the raster fallback, the masks with a
// backdrop and a transfer function, the meshes, the Extend of a gradient and
// the conversion of a shader of okage.
func fuzzSurface(r *fuzzReader, s *Surface, depth int) {
	for range r.u8() % 32 {
		switch r.u8() % 18 {
		case 0:
			if r.u8()%4 == 0 {
				s.SetFill(nil)
			} else {
				s.SetFill(&Fill{Paint: r.paint(s, depth), Opacity: r.opacity(), Rule: FillRule(r.u8() % 2)})
			}
		case 1:
			if r.u8()%4 == 0 {
				s.SetStroke(nil)
			} else {
				k := DefaultStroke()
				k.Paint = r.paint(s, depth)
				k.Width = r.f32()
				k.MiterLimit = r.f32()
				k.LineCap = LineCap(r.u8() % 3)
				k.LineJoin = LineJoin(r.u8() % 3)
				k.Opacity = r.opacity()
				if r.u8()%2 == 0 {
					k.Dash = &StrokeDash{Array: []float32{r.f32(), r.f32()}, Offset: r.f32()}
				}
				s.SetStroke(&k)
			}
		case 2, 3:
			if p := r.path(); p != nil {
				s.DrawPath(p)
			}
		case 4:
			s.PushTransform(r.transform())
		case 5:
			s.PushBlendMode(BlendMode(r.u8() % 16))
		case 6:
			if p := r.path(); p != nil {
				s.PushClipPath(p, FillRule(r.u8()%2))
			}
		case 7:
			s.PushOpacity(r.opacity())
		case 8:
			s.PushIsolated()
		case 9:
			if depth < 3 {
				s.PushMask(r.mask(s, depth))
			}
		case 10:
			if depth < 3 {
				b := s.StreamBuilder()
				fuzzSurface(r, b.Surface(), depth+1)
				s.DrawGraphic(NewGraphic(b.Finish(), r.u8()%2 == 0))
			}
		case 11:
			g := Group{Opacity: r.opacity(), BlendMode: BlendMode(r.u8() % 16), Isolated: r.u8()%2 == 0, Knockout: r.u8()%2 == 0}
			if depth < 3 && r.u8()%2 == 0 {
				g.Mask = r.mask(s, depth)
			}
			s.PushGroup(g)
		case 12:
			s.PushStrokeAdjustment(r.u8()%2 == 0)
		case 13:
			if size, ok := path.SizeFromWH(r.coord(), r.coord()); ok {
				s.FillStencil(r.stencil(), size, r.u8()%2 == 0)
			}
		case 14:
			// The pixmap stays small: the transform of the surface scales
			// by at most 4, and the rectangle is at most 255 units wide.
			ctm := s.CTM()
			if rect, ok := path.RectFromXYWH(r.coord(), r.coord(), r.coord(), r.coord()); ok &&
				max(abs32(ctm.SX), abs32(ctm.KX), abs32(ctm.KY), abs32(ctm.SY)) <= 4 {
				c := r.okageColor()
				s.DrawRaster(rect, float32(r.u8()%4)/2, func(pm *okage.Pixmap, _ Transform) { pm.Fill(c) })
			}
		case 15:
			if p, o, ok := s.PaintFromShader(r.shader()); ok {
				s.SetFill(&Fill{Paint: p, Opacity: o, Rule: FillRule(r.u8() % 2)})
			}
		default:
			s.Pop()
		}
	}
	s.Finish()
}

// abs32 returns the magnitude of v, and NaN for NaN.
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// point reads a point of a destination or of a quadrilateral.
func (r *fuzzReader) point() Point {
	return path.PointFromXY(r.coord(), r.coord())
}

// text reads a string of at most 7 bytes, which need not be UTF-8.
func (r *fuzzReader) text() string {
	n := int(r.u8() % 8)
	b := make([]byte, 0, n)
	for range n {
		b = append(b, r.u8())
	}
	return string(b)
}

// xyz reads an XYZ destination, whose page index can be out of range.
func (r *fuzzReader) xyz() XYZDestination {
	return NewXYZDestination(int(int8(r.u8())), r.point())
}

// named reads a named destination, with one of a few names, so that names
// repeat.
func (r *fuzzReader) named() NamedDestination {
	return NewNamedDestination(string(rune('a'+r.u8()%4)), r.xyz())
}

// destination reads a destination of either kind, or nil.
func (r *fuzzReader) destination() Destination {
	switch r.u8() % 5 {
	case 0:
		return nil
	case 1, 2:
		return r.xyz()
	}
	return r.named()
}

// target reads the target of a link: a destination, a link or a go-to
// action, or nil.
func (r *fuzzReader) target() Target {
	switch r.u8() % 4 {
	case 0:
		return NewLinkAction(r.text())
	case 1:
		return NewGotoAction(r.destination())
	case 2:
		if d := r.destination(); d != nil {
			return d
		}
	}
	return nil
}

// annotation reads a link annotation, with a rectangle or with at most 3
// quadrilaterals.
func (r *fuzzReader) annotation() Annotation {
	var l LinkAnnotation
	if r.u8()%2 == 0 {
		rect, ok := path.RectFromXYWH(r.f32(), r.f32(), r.f32(), r.f32())
		if !ok {
			rect, _ = path.RectFromXYWH(0, 0, 1, 1)
		}
		l = NewLinkAnnotation(rect, r.target())
	} else {
		quads := make([]Quadrilateral, r.u8()%4)
		for i := range quads {
			quads[i] = Quadrilateral{r.point(), r.point(), r.point(), r.point()}
		}
		l = NewLinkAnnotationWithQuadPoints(quads, r.target())
	}
	if r.u8()%2 == 0 {
		l = l.WithBorder(NewLinkBorder(r.f32(), r.color()))
	}
	if r.u8()%2 == 0 {
		alt := r.text()
		return NewAnnotationLink(l, &alt).WithLocation(Location(r.u8()))
	}
	return AnnotationFromLink(l)
}

// outlineNodes reads at most 3 nodes of an outline, with children up to a
// depth of 3.
func (r *fuzzReader) outlineNodes(depth int) []OutlineNode {
	var nodes []OutlineNode
	for range r.u8() % 4 {
		n := NewOutlineNode(r.text(), r.xyz()).WithOpen(r.u8()%2 == 0)
		if depth < 3 {
			for _, c := range r.outlineNodes(depth + 1) {
				n.PushChild(c)
			}
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// metadata reads metadata with random fields.
func (r *fuzzReader) metadata() Metadata {
	flags := r.u32()
	m := NewMetadata()
	setters := []func(Metadata, string) Metadata{
		Metadata.Title, Metadata.Description, Metadata.Creator, Metadata.Producer,
		Metadata.DocumentID, Metadata.VersionID, Metadata.Language,
	}
	for i, set := range setters {
		if flags&(1<<i) != 0 {
			m = set(m, r.text())
		}
	}
	if flags&(1<<7) != 0 {
		m = m.Keywords([]string{r.text(), r.text()})
	}
	if flags&(1<<8) != 0 {
		m = m.Authors([]string{r.text()})
	}
	if flags&(1<<9) != 0 {
		m = m.CreationDate(NewDateTime(uint16(r.u32())).Month(r.u8()).Day(r.u8()).Hour(r.u8()).
			Minute(r.u8()).Second(r.u8()).UTCOffsetHour(int8(r.u8())).UTCOffsetMinute(r.u8()))
	}
	if flags&(1<<10) != 0 {
		m = m.TextDirection(TextDirectionMetadata(r.u8() % 3))
	}
	if flags&(1<<11) != 0 {
		m = m.PageLayout(PageLayout(r.u8() % 7))
	}
	m = m.DisplayDocTitle(flags&(1<<12) != 0)
	for range r.u8() % 4 {
		m, _ = m.CustomField(r.text(), r.text())
	}
	return m
}

// FuzzDocument builds a document from random settings, page sizes, boxes and
// labels, which a program can take from a hostile file. Finish must not
// panic, and a file must start with the header of PDF. A page that is not
// finished at once gets random calls of its surface, with every paint. A
// page can get link annotations, and the document named destinations,
// metadata and an outline, whose destinations can link to pages that do not
// exist.
func FuzzDocument(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x1f, 2, 0, 0, 0x48, 0x43, 0, 0, 0x48, 0x43, 1, 1, 2, 0, 0, 0, 1, 0x80})
	f.Add([]byte{0x07, 3, 1, 0, 0, 0x80, 0x7f, 0, 0, 0x80, 0x7f})
	f.Add(bytes.Repeat([]byte{0xff}, 64))
	// Two pages with links, then named destinations, metadata and an
	// outline.
	f.Add([]byte{0x07, 2, 0x20, 0, 0, 0, 0, 0, 2, 0, 0x40, 0x40, 0x40, 0x40, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0x20, 0, 0, 0, 0, 0, 2, 1, 1, 2, 0, 1, 0, 1, 1, 0, 0x0e, 1, 0, 1, 0, 2, 0, 0xff, 0x1f, 0, 0,
		2, 1, 'x', 0, 1, 'y', 0, 0, 2, 1, 'a', 1, 1, 0, 1, 0, 0, 0})
	f.Add([]byte{0x04, 1, 0x40, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0x10, 0, 1, 255, 0, 0, 200, 0, 2, 4, 0, 0, 0, 0, 1, 0, 0, 0x20, 0x41, 0, 0, 0x20, 0x41,
		1, 0, 0, 0, 0, 0, 0, 0x20, 0x41, 7, 128, 8, 2, 4, 0, 0, 0, 0, 1, 0, 0, 0x20, 0x41, 0, 0, 0x20, 0x41, 11, 11})
	f.Fuzz(func(t *testing.T, data []byte) {
		r := &fuzzReader{data: data}
		flags := r.u8()
		d := NewDocument(
			WithPretty(flags&1 != 0),
			WithASCIICompatible(flags&2 != 0),
			WithCompression(flags&4 != 0),
			WithNoDeviceCS(flags&8 != 0),
			WithPDFVersion(PDFVersion(flags>>4)%6),
		)
		pages := int(r.u8() % 16)
		for range pages {
			var ps PageSettings
			kind := r.u8()
			switch kind % 3 {
			case 0:
				ps = DefaultPageSettings()
			case 1:
				var ok bool
				if ps, ok = PageSettingsFromWH(r.f32(), r.f32()); !ok {
					ps = DefaultPageSettings()
				}
			default:
				if box := r.rect(); box != nil {
					ps = NewPageSettings(*box)
				}
			}
			ps = ps.WithCropBox(r.rect()).WithBleedBox(r.rect()).WithTrimBox(r.rect()).WithArtBox(r.rect())
			if l := r.u8(); l%2 == 1 {
				var style *NumberingStyle
				if l&2 != 0 {
					s := NumberingStyle(r.u8() % 5)
					style = &s
				}
				var prefix *string
				if l&4 != 0 {
					p := string(data[:min(len(data), int(r.u8()%8))])
					prefix = &p
				}
				ps = ps.WithPageLabel(NewPageLabel(style, prefix, r.u32()))
			}
			p := d.StartPage(ps)
			if kind&0x20 != 0 {
				for range r.u8() % 4 {
					p.AddAnnotation(r.annotation())
				}
			}
			if kind&0x80 != 0 {
				p.Finish()
			} else if kind&0x40 != 0 {
				fuzzSurface(r, p.Surface(), 0)
			}
		}
		if extra := r.u8(); extra != 0 {
			for range extra % 4 {
				d.RegisterNamedDestination(r.named())
			}
			if extra&4 != 0 {
				d.SetMetadata(r.metadata())
			}
			if extra&8 != 0 {
				outline := NewOutline()
				for _, n := range r.outlineNodes(0) {
					outline.PushChild(n)
				}
				d.SetOutline(outline)
			}
		}
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			return
		}
		if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
			t.Fatalf("the file starts with %q", buf.Bytes()[:min(buf.Len(), 8)])
		}
	})
}
