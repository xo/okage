// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"strings"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
)

// testGraphicsState is a cacheable object with a resource, in the way of the
// ExtGState of krilla src/graphics/graphics_state.rs, for the tests of the
// serializer and of the resources.
type testGraphicsState struct {
	alpha float32
	// serialized counts the calls of serialize.
	serialized *int
}

func (g testGraphicsState) typeID() string { return "testGraphicsState" }

func (g testGraphicsState) hash(h *hasher) {
	h.u32(uint32(g.alpha * 1000))
}

func (g testGraphicsState) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	*g.serialized++
	cc.nonStream.extGStates.Indirect(rootRef).Dict().
		Pair(pdfwriter.Name("ca"), pdfwriter.Real(g.alpha)).
		Finish()
}

func (testGraphicsState) newResource(ref pdfwriter.Ref) extGStateResource {
	return extGStateResource{ref}
}

// TestRegisterResourceable checks that the serializer writes an object once
// for each value, as register_cacheable of krilla src/serialize.rs does.
func TestRegisterResourceable(t *testing.T) {
	d := NewDocument()
	n := 0
	a := registerResourceable(d.sc, d.cc, testGraphicsState{alpha: 0.5, serialized: &n})
	b := registerResourceable(d.sc, d.cc, testGraphicsState{alpha: 0.5, serialized: &n})
	c := registerResourceable(d.sc, d.cc, testGraphicsState{alpha: 0.25, serialized: &n})
	if a != b || a == c || n != 2 {
		t.Errorf("the references are %v, %v and %v, and serialize ran %d times", a, b, c, n)
	}
	// Another type with the same hash gets another reference.
	other := d.sc.registerCached(hashedInt{500}, func(*serializeContext, pdfwriter.Ref) {})
	if other == a.getRef() {
		t.Error("two types share a reference")
	}
}

// TestResourceMapper checks the names of ResourceMapper in krilla
// src/resource.rs: each kind numbers its references in the order of their
// first use.
func TestResourceMapper(t *testing.T) {
	b := newResourceDictionaryBuilder()
	ref := pdfwriter.NewRef
	names := []string{
		b.registerResource(fontResource{ref(10)}),
		b.registerResource(fontResource{ref(11)}),
		b.registerResource(fontResource{ref(10)}),
		b.registerResource(extGStateResource{ref(10)}),
		b.registerResource(colorSpaceResource{ref(12)}),
		b.registerResource(patternResource{ref(13)}),
		b.registerResource(xObjectResource{ref(14)}),
		b.registerResource(shadingResource{ref(15)}),
	}
	want := []string{"f0", "f1", "f0", "g0", "c0", "p0", "x0", "s0"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("name %d is %q, want %q", i, names[i], want[i])
		}
	}
	p := newResourceDictionaryBuilderWithPrefix("xo")
	if got := p.registerResource(fontResource{ref(3)}); got != "xof0" {
		t.Errorf("a prefixed name is %q", got)
	}
	rd := b.finish()
	if rd.fonts.len() != 2 || rd.extGStates.len() != 1 {
		t.Errorf("the dictionary has %d fonts and %d graphics states", rd.fonts.len(), rd.extGStates.len())
	}
	for range rd.fonts.getEntries(resourceFont, "") {
		break
	}
}

// TestToPDFResources checks ResourceDictionary::to_pdf_resources of krilla
// src/resource.rs, for a page.
func TestToPDFResources(t *testing.T) {
	write := func(version PDFVersion, rd resourceDictionary) (string, string) {
		d := NewDocument(WithPDFVersion(version), WithPretty(false))
		pages := d.sc.newChunk()
		resources := d.sc.newChunk()
		page := pages.Page(pdfwriter.NewRef(100))
		rd.toPDFResources(page, d.sc, resources)
		page.Finish()
		return string(pages.AsBytes()), string(resources.AsBytes())
	}

	page, resources := write(PDF17, resourceDictionary{})
	if !strings.Contains(page, "/Resources 2 0 R") || !strings.Contains(resources, "<</ProcSet[/PDF/Text/ImageC/ImageB]>>") {
		t.Errorf("an empty dictionary of PDF 1.7 gives %q and %q", page, resources)
	}
	page, resources = write(PDF20, resourceDictionary{})
	if !strings.Contains(page, "/Resources<<>>") || resources != "" {
		t.Errorf("an empty dictionary of PDF 2.0 gives %q and %q", page, resources)
	}

	b := newResourceDictionaryBuilderWithPrefix("a")
	for _, r := range []resource{
		fontResource{pdfwriter.NewRef(20)},
		shadingResource{pdfwriter.NewRef(21)},
		xObjectResource{pdfwriter.NewRef(22)},
		patternResource{pdfwriter.NewRef(23)},
		extGStateResource{pdfwriter.NewRef(24)},
		colorSpaceResource{pdfwriter.NewRef(25)},
		fontResource{pdfwriter.NewRef(26)},
	} {
		b.registerResource(r)
	}
	page, resources = write(PDF20, b.finish())
	want := "<</ColorSpace<</ac0 25 0 R>>/ExtGState<</ag0 24 0 R>>/Pattern<</ap0 23 0 R>>/XObject<</ax0 22 0 R>>/Shading<</as0 21 0 R>>/Font<</af0 20 0 R/af1 26 0 R>>>>"
	if !strings.Contains(page, "/Resources 2 0 R") || !strings.Contains(resources, want) {
		t.Errorf("a full dictionary gives %q and %q", page, resources)
	}

	h1, h2 := newHasher(), newHasher()
	rd := b.finish()
	rd.hash(h1)
	(&resourceDictionary{}).hash(h2)
	if h1.finish128() == h2.finish128() {
		t.Error("two dictionaries get the same hash")
	}
}
