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

// FuzzDocument builds a document from random settings, page sizes, boxes and
// labels, which a program can take from a hostile file. Finish must not
// panic, and a file must start with the header of PDF. The surface comes
// with unit 5 of phase 14, and its calls join this test then.
func FuzzDocument(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x1f, 2, 0, 0, 0x48, 0x43, 0, 0, 0x48, 0x43, 1, 1, 2, 0, 0, 0, 1, 0x80})
	f.Add([]byte{0x07, 3, 1, 0, 0, 0x80, 0x7f, 0, 0, 0x80, 0x7f})
	f.Add(bytes.Repeat([]byte{0xff}, 64))
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
