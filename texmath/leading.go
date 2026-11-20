// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

// leading says how the rows of a grid are separated. TeX has three models,
// and they differ in kind, not by a constant, so each one is a type of its
// own. An environment then cannot take \arraystretch and \lineskip at once,
// and the choice of the model is visible where the grid is made.
type leading interface {
	// strut is the smallest height and depth of each row, before a gap is
	// computed. \array and its kin take theirs from \@arstrut. The other two
	// models have none.
	strut() (h, d float64)
	// gap is the space between a row of the depth prevDepth and the next row
	// of the height height. Only the model of the interline glue reads its
	// arguments.
	gap(prevDepth, height float64) float64
}

// flatLeading separates the rows by a fixed gap. It is not a model of TeX.
// The source keeps it only where no measurement was made, for \substack.
type flatLeading struct{ rowGap float64 }

func (l flatLeading) strut() (float64, float64) { return 0, 0 }
func (l flatLeading) gap(_, _ float64) float64  { return l.rowGap }

// strutLeading is the model of \array and of everything on it: matrix,
// pmatrix and its kin, cases, aligned and gathered. \@arstrut puts a strut of
// \arraystretch times .7 and .3 of \baselineskip in each row (latex.ltx:12103),
// and the rows touch: the struts alone set the pitch. \jot, where the
// environment has one, is the only space between them. So the pitch grows
// with \arraystretch in proportion, which the model of the glue does not
// give.
type strutLeading struct {
	baselineskip float64
	jot          float64 // added to the pitch, never to the strut
	stretch      float64 // \arraystretch, 0 for 1
}

func (l strutLeading) strut() (float64, float64) {
	st := l.stretch
	if st <= 0 {
		st = 1
	}
	return float64(st * 0.7 * l.baselineskip), float64(st * 0.3 * l.baselineskip)
}

func (l strutLeading) gap(_, _ float64) float64 { return l.jot }

// glueLeading is the interline glue of TeX, tex.web §679: between a box of
// the depth prev_depth and the next one of the height h, it adds
// \baselineskip - prev_depth - h, or \lineskip when that is below
// \lineskiplimit. There is no strut, so the extents of the rows choose the
// branch. smallmatrix of amsmath takes it: \baselineskip6\ex@
// \lineskip1.5\ex@ \lineskiplimit\lineskip, with no strut
// (amsmath.sty:1045-1047).
type glueLeading struct {
	baselineskip  float64
	lineskip      float64
	lineskiplimit float64
}

func (l glueLeading) strut() (float64, float64) { return 0, 0 }

func (l glueLeading) gap(prevDepth, height float64) float64 {
	if g := l.baselineskip - prevDepth - height; g >= l.lineskiplimit {
		return g
	}
	return l.lineskip
}
