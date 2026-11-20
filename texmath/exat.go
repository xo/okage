// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

// exAt is the \ex@ of amsmath at the font size p, in points: the unit of the
// leading of smallmatrix (amsmath.sty:1047, \baselineskip6\ex@
// \lineskip1.5\ex@). finishEnv uses it for smallmatrix.
//
// It is not a fixed 1pt and not proportional to the size. \compute@ex@ of
// amsgen.sty multiplies \vfuzz by .97 once for each point of distance from
// 10pt, doubled, and adds the shortfall to 1pt or takes it away, by the sign
// of the distance. It reaches 1.5pt above 20pt. It is written in the scaled
// points of TeX, because .97 is a fraction of 16 bits and a float drifts.
// TestExAtMatchesTheReference holds fifteen sizes measured with tectonic.
func exAt(p float64) float64 {
	const pt = 1 << 16 // scaled points per point
	// okage: a size of 21pt or more, an infinity or NaN gives 1.5 before the
	// conversion to an integer, which cannot hold them. The source converts
	// first.
	if !(p < 21) {
		return 1.5
	}
	// okage: a size below -1e12 gives the result of -1e12, where \vfuzz is
	// already 0, so that the conversion holds it.
	p = max(p, -1e12)
	// A font size is positive, so this rounds one way only. The conversion
	// to float64 stops a fused multiply and add (D16).
	d := -int64(float64(p*pt) + 0.5)
	if d < -20*pt {
		return 1.5
	}
	d += 10 * pt
	d *= 2
	// \edef\@tempa{\ifdim\dimen@>\z@ -\fi}: the sign is taken before the
	// absolute value. It makes a size below 10pt subtract where a size above
	// adds.
	neg := d > 0
	if d < 0 {
		d = -d
	}
	d -= 10000 // "fudge factor", \@m sp
	vfuzz := int64(pt)
	// okage: the loop stops when \vfuzz reaches 0, where it stays. The
	// source runs once for each point, which does not end in time for a size
	// such as -1e15.
	for d > 0 && vfuzz > 0 {
		vfuzz = texScale(vfuzz, 63570) // \vfuzz=.97\vfuzz
		d -= pt
	}
	d = pt - vfuzz
	ex := int64(pt)
	if neg {
		ex -= d
	} else {
		ex += d
	}
	return float64(ex) / pt
}

// texScale is <factor><dimen> as scan_dimen of TeX computes it: the dimen
// times the numerator of 16 bits of the factor, over 2^16, truncated. .97 is
// 63570/65536.
func texScale(x, num int64) int64 {
	return x * num / (1 << 16)
}
