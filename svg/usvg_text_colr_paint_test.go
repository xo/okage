package svg

import (
	"encoding/binary"
	"testing"

	"github.com/go-text/typesetting/font/opentype/tables"
)

// countFillPainter is a colorPainter that counts the fills.
type countFillPainter struct {
	fills int
}

func (p *countFillPainter) pushTransform(colrTransform)                  {}
func (p *countFillPainter) popTransform()                                {}
func (p *countFillPainter) fillGlyph(GlyphID, *colrTransform, colrBrush) {}
func (p *countFillPainter) pushClipGlyph(GlyphID)                        {}
func (p *countFillPainter) pushClipBox(colrBoundingBox)                  {}
func (p *countFillPainter) popClip()                                     {}
func (p *countFillPainter) fill(colrBrush)                               { p.fills++ }
func (p *countFillPainter) pushLayer(tables.CompositeMode)               {}
func (p *countFillPainter) popLayer()                                    {}

// nestedLayersCOLR returns a COLRv1 table where the base glyph 5 is a
// PaintColrLayers of the layer 0, and the layer 0 is a PaintColrLayers of the
// layer 1, a PaintSolid. A ZWJ sequence of Noto Color Emoji nests its layers
// in the same way.
func nestedLayersCOLR() []byte {
	b := binary.BigEndian
	var t []byte
	u8 := func(v uint8) { t = append(t, v) }
	u16 := func(v uint16) { t = b.AppendUint16(t, v) }
	u32 := func(v uint32) { t = b.AppendUint32(t, v) }

	// The header of version 1, 34 bytes.
	u16(1)  // version
	u16(0)  // numBaseGlyphRecords
	u32(0)  // baseGlyphRecordsOffset
	u32(0)  // layerRecordsOffset
	u16(0)  // numLayerRecords
	u32(34) // baseGlyphListOffset
	u32(50) // layerListOffset
	u32(0)  // clipListOffset
	u32(0)  // varIndexMapOffset
	u32(0)  // itemVariationStoreOffset

	// The BaseGlyphList at 34, with its paint at 44.
	u32(1)  // numBaseGlyphPaintRecords
	u16(5)  // glyphID
	u32(10) // paintOffset
	u8(1)   // PaintColrLayers
	u8(1)   // numLayers
	u32(0)  // firstLayerIndex

	// The LayerList at 50, with its paints at 62 and 68.
	u32(2)      // numLayers
	u32(12)     // the offset of the layer 0
	u32(18)     // the offset of the layer 1
	u8(1)       // PaintColrLayers
	u8(1)       // numLayers
	u32(1)      // firstLayerIndex
	u8(2)       // PaintSolid
	u16(0)      // paletteIndex
	u16(0x4000) // alpha 1.0
	return t
}

// TestTraverseNestedColrLayers checks that a PaintColrLayers inside another
// PaintColrLayers is not taken as a cycle.
func TestTraverseNestedColrLayers(t *testing.T) {
	colr, err := tables.ParseCOLR(nestedLayersCOLR())
	if err != nil {
		t.Fatal(err)
	}
	paint, ok := colr.Search(5)
	if !ok {
		t.Fatal("no paint for the glyph 5")
	}
	inst := &colrInstance{colr: &colr}
	rp, ok := resolvePaint(inst, paint)
	if !ok {
		t.Fatal("resolvePaint failed")
	}
	var decycler paintDecycler
	painter := &countFillPainter{}
	if !traverseWithCallbacks(&rp, inst, painter, &decycler, 0) {
		t.Fatal("the traversal found a cycle")
	}
	if painter.fills != 1 {
		t.Errorf("got %d fills, want 1", painter.fills)
	}
}
