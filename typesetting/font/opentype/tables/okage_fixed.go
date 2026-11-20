// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

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
