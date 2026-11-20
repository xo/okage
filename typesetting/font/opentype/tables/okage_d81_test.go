// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import (
	"math/rand/v2"
	"testing"
)

// okage: the tests of D81.

// TestPairPosRecordXAdvance compares the records of a PairPos of format 2
// that gives only an advance for the first glyph with parseValueRecord,
// for each pair of classes, and for classes past the end of the data.
func TestPairPosRecordXAdvance(t *testing.T) {
	rng := rand.New(rand.NewPCG(81, 1120))
	data := make([]byte, 16+2*5*7)
	for i := range data {
		data[i] = byte(rng.Uint32())
	}
	pp := PairPosData2{ValueFormat1: XAdvance, class1Count: 5, class2Count: 7, classData: data}
	for c1 := uint16(0); c1 < 7; c1++ {
		for c2 := uint16(0); c2 < 9; c2++ {
			got := pp.Record(c1, c2)
			v1, _, _ := parseValueRecord(XAdvance, data, 16+2*7*int(c1)+2*int(c2))
			if want := (Class2Record{ValueRecord1: v1}); got != want {
				t.Fatalf("classes %d and %d: got %+v, want %+v", c1, c2, got, want)
			}
		}
	}
}
