package svg

import (
	"testing"

	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests that the metrics of a font follow the font model of
// read-fonts 0.46: each metric keeps the delta of MVAR in the 48.16 format
// (D100).

// TestMVARDeltaF48Dot16 checks a metric with a large MVAR delta that has a
// fraction. The MVAR table is the one of the font that D100 builds from
// SourceSans-VF.ttf with fontTools: the delta 32767 for "undo" in a region
// that peaks at 1, and an underline position of -16285 in post. At the
// coordinate 8193, read-fonts 0.46 gives the raw value 6586364, which is
// 100.49994, and usvg rounds it to 100. A delta that is first converted to
// a float32 is 16385.5, so the sum is 100.5, which rounds to 101.
func TestMVARDeltaF48Dot16(t *testing.T) {
	undo := ot.MustNewTag("undo")
	mvar := tables.MVAR{
		ItemVariationStore: tables.ItemVarStore{
			VariationRegionList: tables.VariationRegionList{
				VariationRegions: []tables.VariationRegion{{
					RegionAxes: []tables.RegionAxisCoordinates{{StartCoord: 0, PeakCoord: 1 << 14, EndCoord: 1 << 14}},
				}},
			},
			ItemVariationDatas: []tables.ItemVariationData{{
				RegionIndexes: []uint16{0},
				DeltaSets:     [][]int32{{13}, {22}, {32767}},
			}},
		},
		ValueRecords: []tables.VarValueRecord{
			{ValueTag: ot.MustNewTag("stro"), Index: tables.VariationStoreIndex{DeltaSetInner: 0}},
			{ValueTag: undo, Index: tables.VariationStoreIndex{DeltaSetInner: 2}},
			{ValueTag: ot.MustNewTag("xhgt"), Index: tables.VariationStoreIndex{DeltaSetInner: 1}},
		},
	}
	coords := []tables.Coord{8193}

	got := f48Dot16FromI32(-16285) + mvarMetricDelta(mvar, "undo", coords)
	if got != 6586364 {
		t.Errorf("underline position is %d, want 6586364", got)
	}
	if v := got.toF32(); v != 100.49994 {
		t.Errorf("toF32 is %v, want 100.49994", v)
	}
	if v := asI16(round32(got.toF32())); v != 100 {
		t.Errorf("rounded underline position is %d, want 100", v)
	}

	// Before D100, Go converted the delta to a float32 and then added it.
	old := float32(-16285) + mvar.ItemVariationStore.GetDelta(tables.VariationStoreIndex{DeltaSetInner: 2}, coords)
	if old != 100.5 {
		t.Errorf("the float32 sum is %v, want 100.5", old)
	}

	if got := mvarMetricDelta(mvar, "xhgt", coords); got != 22*4*8193 {
		t.Errorf("delta of xhgt is %d, want %d", got, 22*4*8193)
	}
	if got := mvarMetricDelta(mvar, "hasc", coords); got != 0 {
		t.Errorf("delta of hasc, which the font does not vary, is %d", got)
	}
}

// TestMetricsReadFonts checks the metrics of RobotoFlex at a location
// against the raw values of read-fonts 0.46, which the Rust program of D100
// printed with the same normalized coordinates.
func TestMetricsReadFonts(t *testing.T) {
	db, ids := testFontDB(t, "RobotoFlex.subset.ttf")
	pf, ok := db.withFaceData(ids[0])
	if !ok {
		t.Fatal("no face")
	}
	coords := []tables.Coord{15691, 12861, 0, -13763, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	m := newFontMetrics(pf, coords)
	line, _ := m.hLine()
	s := newStyleMetrics(pf, coords)
	for _, c := range []struct {
		name      string
		got, want f48Dot16
	}{
		{"ascender", line.ascender, 124518400},
		{"descender", line.descender, -32768000},
		{"x height", *m.xHeight, 60295496},
		{"strikeout position", s.strikeThrough.position, 33554432},
		{"strikeout size", s.strikeThrough.thickness, 6684672},
		{"underline position", s.underline.position, -11993088},
		{"underline thickness", s.underline.thickness, 10878976},
		{"subscript offset", *s.subscriptYOffset, 18808832},
		{"superscript offset", *s.superscriptYOffset, 64028672},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d, want %d", c.name, c.got, c.want)
		}
	}
}
