// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import "math"

// okage: this file is not in go-text (D84).
//
// go-text computes the deltas of gvar, of an item variation store and of
// the blends of CFF2 in float32, as HarfBuzz does. skrifa and FreeType
// compute them in the 16.16 fixed-point numbers of OpenType, and round each
// product and each quotient. The copy computes them as skrifa does, so that
// an instance of a variable font gives the same points as skrifa. Fixed and
// its methods port Fixed of font-types src/fixed.rs, and EvaluateFixed ports
// VariationRegion::compute_scalar of read-fonts src/tables/variations.rs.

// Fixed is a number in the 16.16 fixed-point format: an int32 with 16 bits
// of fraction.
type Fixed int32

// FixedOne is the number 1 in the 16.16 format.
const FixedOne Fixed = 1 << 16

// FixedFromInt returns the integer i in the 16.16 format. As in font-types,
// an integer out of the range of the format wraps.
func FixedFromInt(i int32) Fixed { return Fixed(i << 16) }

// Fixed returns the coordinate in the 16.16 format, as F2Dot14::to_fixed of
// font-types does.
func (c Coord) Fixed() Fixed { return Fixed(int32(c) * 4) }

// Mul returns the product of a and b, rounded to the nearest number, with
// ties away from zero, as impl Mul for Fixed of font-types does.
func (a Fixed) Mul(b Fixed) Fixed {
	ab := int64(a) * int64(b)
	if ab < 0 {
		ab--
	}
	return Fixed((ab + 0x8000) >> 16)
}

// Div returns the quotient of a and b, rounded to the nearest number, as
// impl Div for Fixed of font-types does. A division by zero gives the
// largest number of the format, with the sign of a.
func (a Fixed) Div(b Fixed) Fixed {
	neg := (a < 0) != (b < 0)
	au, bu := absFixed(a), absFixed(b)
	q := uint32(0x7FFFFFFF)
	if bu != 0 {
		q = uint32(((au << 16) + (bu >> 1)) / bu)
	}
	if neg {
		return -Fixed(q)
	}
	return Fixed(q)
}

// MulDiv returns a times b divided by c, with one rounding, as
// Fixed::mul_div of font-types does. A division by zero gives the largest
// number of the format, with the sign of the operands.
func (a Fixed) MulDiv(b, c Fixed) Fixed {
	neg := (a < 0) != (b < 0) != (c < 0)
	au, bu, cu := absFixed(a), absFixed(b), absFixed(c)
	r := uint64(0x7FFFFFFF)
	if cu > 0 {
		r = (au*bu + cu>>1) / cu
	}
	if neg {
		return -Fixed(uint32(r))
	}
	return Fixed(uint32(r))
}

// absFixed returns the absolute value of a as a uint64, as the wrapping
// negation of font-types does.
func absFixed(a Fixed) uint64 {
	if a < 0 {
		return uint64(-int64(a))
	}
	return uint64(a)
}

// EvaluateFixed returns the scalar factor of the region in the 16.16
// format, as VariationRegion::compute_scalar of read-fonts does. An axis
// without a coordinate is at its default, 0, as in Evaluate.
func (vr VariationRegion) EvaluateFixed(coords []Coord) Fixed {
	scalar := FixedOne
	for axis, reg := range vr.RegionAxes {
		if reg.PeakCoord == 0 {
			continue
		}
		peak, start, end := reg.PeakCoord.Fixed(), reg.StartCoord.Fixed(), reg.EndCoord.Fixed()
		if start > peak || peak > end || start < 0 && end > 0 {
			continue
		}
		var coord Fixed
		if axis < len(coords) {
			coord = coords[axis].Fixed()
		}
		switch {
		case coord < start || coord > end:
			return 0
		case coord == peak:
			continue
		case coord < peak:
			scalar = scalar.MulDiv(coord-start, peak-start)
		default:
			scalar = scalar.MulDiv(end-coord, end-peak)
		}
	}
	return scalar
}

// MapFixed maps a normalized coordinate in the 16.16 format with the
// segment map, as SegmentMaps::apply of read-fonts src/tables/avar.rs
// does. skrifa normalizes a coordinate of the user in the 16.16 format,
// maps it with avar, and only then rounds it to 2.14. Map takes and gives a
// coordinate in 2.14, as HarfBuzz does.
func (sm SegmentMaps) MapFixed(coord Fixed) Fixed {
	maps := sm.AxisValueMaps
	from := func(i int) Fixed { return maps[i].FromCoordinate.Fixed() }
	to := func(i int) Fixed { return maps[i].ToCoordinate.Fixed() }
	n := len(maps)
	if n < 2 {
		if n == 0 {
			return coord
		}
		return coord - from(0) + to(0)
	}
	neg1, pos1 := FixedFromInt(-1), FixedFromInt(1)
	start, end := 0, n
	if from(start) == neg1 && to(start) == neg1 && from(start+1) == neg1 {
		start++
	}
	if from(end-1) == pos1 && to(end-1) == pos1 && from(end-2) == pos1 {
		end--
	}
	i := start
	for i < end && coord != from(i) {
		i++
	}
	if i < end {
		j := i
		for j+1 < end && coord == from(j+1) {
			j++
		}
		switch {
		case i == j:
			return to(i)
		case i+2 == j:
			return to(i + 1)
		case coord < 0:
			return to(j)
		case coord > 0:
			return to(i)
		}
		ti, tj := to(i), to(j)
		if absFixed(ti) < absFixed(tj) {
			return ti
		}
		return tj
	}
	k := start
	for k < end && coord >= from(k) {
		k++
	}
	if k == 0 {
		return coord - from(0) + to(0)
	}
	if k == end {
		return coord - from(end-1) + to(end-1)
	}
	bf, bt, af, at := from(k-1), to(k-1), from(k), to(k)
	return bt + (at-bt).MulDiv(coord-bf, af-bf)
}

// ToCoord rounds a coordinate in the 16.16 format to 2.14, as
// Fixed::to_f2dot14 of font-types does.
func (a Fixed) ToCoord() Coord { return Coord((a + 2) >> 2) }

// okage: FixedFromFloat64, SaturatingSub and NormalizeFixed are not in
// go-text (D101). They normalize a coordinate of the user as skrifa does,
// for Font.NormalizeVariations and for internal/subsetter.

// FixedFromFloat64 returns x in the 16.16 format, as Fixed::from_f64 of
// font-types src/fixed.rs does. It rounds half away from zero, and
// saturates as the conversion "as i32" of Rust does. NaN gives 0.
func FixedFromFloat64(x float64) Fixed {
	frac := -0.5
	if !math.Signbit(x) {
		frac = 0.5
	}
	v := float64(x*65536) + frac
	switch {
	case v != v:
		return 0
	case v <= math.MinInt32:
		return math.MinInt32
	case v >= math.MaxInt32:
		return math.MaxInt32
	}
	return Fixed(int32(v))
}

// SaturatingSub returns a minus b, clamped to the range of the format, as
// Fixed::saturating_sub of font-types src/fixed.rs does.
func (a Fixed) SaturatingSub(b Fixed) Fixed {
	d := int64(a) - int64(b)
	return Fixed(min(max(d, math.MinInt32), math.MaxInt32))
}

// fixedOfFloat1616 returns the bits of a number of the table fvar, which
// go-text reads as a Float1616.
func fixedOfFloat1616(v Float1616) Fixed {
	return Fixed(int32(Float1616ToUint(v)))
}

// NormalizeFixed maps a coordinate of the user to the range -1 to 1 of the
// axis, in the 16.16 format, as VariationAxisRecord::normalize of
// read-fonts src/tables/fvar.rs does. The quotient is rounded.
func (va VariationAxisRecord) NormalizeFixed(value Fixed) Fixed {
	minValue, defaultValue := fixedOfFloat1616(va.Minimum), fixedOfFloat1616(va.Default)
	// Make sure max is >= min to avoid potential panic in clamp.
	maxValue := max(fixedOfFloat1616(va.Maximum), minValue)
	value = min(max(value, minValue), maxValue)
	switch {
	case value < defaultValue:
		value = -defaultValue.SaturatingSub(value).Div(defaultValue.SaturatingSub(minValue))
	case value > defaultValue:
		value = value.SaturatingSub(defaultValue).Div(maxValue.SaturatingSub(defaultValue))
	default:
		value = 0
	}
	return min(max(value, -FixedOne), FixedOne)
}
