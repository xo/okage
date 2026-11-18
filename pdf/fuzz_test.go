// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

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
	n := r.u8() % 8
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
		}
	case 1:
		return RadialGradient{
			Fx: r.f32(), Fy: r.f32(), Fr: r.f32(), Cx: r.f32(), Cy: r.f32(), Cr: r.f32(), Transform: r.transform(),
			SpreadMethod: SpreadMethod(r.u8() % 3), Stops: r.stops(), AntiAlias: r.u8()%2 == 0,
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
	}
	return r.color()
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
// depth of 3.
func fuzzSurface(r *fuzzReader, s *Surface, depth int) {
	for range r.u8() % 32 {
		switch r.u8() % 12 {
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
				b := s.StreamBuilder()
				fuzzSurface(r, b.Surface(), depth+1)
				s.PushMask(NewMask(b.Finish(), MaskType(r.u8()%2)))
			}
		case 10:
			if depth < 3 {
				b := s.StreamBuilder()
				fuzzSurface(r, b.Surface(), depth+1)
				s.DrawGraphic(NewGraphic(b.Finish(), r.u8()%2 == 0))
			}
		default:
			s.Pop()
		}
	}
	s.Finish()
}

// FuzzDocument builds a document from random settings, page sizes, boxes and
// labels, which a program can take from a hostile file. Finish must not
// panic, and a file must start with the header of PDF. A page that is not
// finished at once gets random calls of its surface, with every paint.
func FuzzDocument(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x1f, 2, 0, 0, 0x48, 0x43, 0, 0, 0x48, 0x43, 1, 1, 2, 0, 0, 0, 1, 0x80})
	f.Add([]byte{0x07, 3, 1, 0, 0, 0x80, 0x7f, 0, 0, 0x80, 0x7f})
	f.Add(bytes.Repeat([]byte{0xff}, 64))
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
			if kind&0x80 != 0 {
				p.Finish()
			} else if kind&0x40 != 0 {
				fuzzSurface(r, p.Surface(), 0)
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
