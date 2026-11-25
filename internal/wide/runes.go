package wide

// RuneRanges is a set of up to 8 closed ranges of runes. IndexOutside scans
// a text for the first rune outside all of them, 8 runes at a time in the
// SIMD build (D92). The text code of typesetting uses it to skip the runes
// that cannot change a result.
type RuneRanges struct {
	n      int
	lo, hi [8]int32
}

// NewRuneRanges returns the set of the ranges. Each range is the first and
// the last rune of the range. It panics if it gets more than 8 ranges, or a
// range whose last rune is before its first rune, or a negative rune.
func NewRuneRanges(ranges ...[2]rune) RuneRanges {
	if len(ranges) > 8 {
		panic("wide: more than 8 rune ranges")
	}
	var rr RuneRanges
	for i, r := range ranges {
		if r[0] < 0 || r[1] < r[0] {
			panic("wide: invalid rune range")
		}
		rr.lo[i], rr.hi[i] = r[0], r[1]
	}
	rr.n = len(ranges)
	return rr
}

// Contains reports whether r is in one of the ranges.
func (rr *RuneRanges) Contains(r rune) bool {
	for k := range rr.n {
		if uint32(r-rr.lo[k]) <= uint32(rr.hi[k]-rr.lo[k]) {
			return true
		}
	}
	return false
}

// indexOutsideRuneRanges is the scalar code of RuneRanges.IndexOutside.
func indexOutsideRuneRanges(rr *RuneRanges, s []rune) int {
	for i, r := range s {
		if !rr.Contains(r) {
			return i
		}
	}
	return len(s)
}
