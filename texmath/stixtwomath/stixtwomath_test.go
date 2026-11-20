// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package stixtwomath_test

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"

	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
)

// TestData checks that the font is the font of go-tex/math at 6a6d2729,
// byte for byte.
func TestData(t *testing.T) {
	data := stixtwomath.Data()
	sum := sha256.Sum256(data)
	const want = "95bc2729e41faf93b0bcae9e96c4dc4da45855067fd0581e621e30734fe8d90b"
	if len(data) != 838508 || hex.EncodeToString(sum[:]) != want {
		t.Errorf("the font has %d bytes and the SHA-256 %x, want 838508 bytes and %s", len(data), sum, want)
	}
}

// TestFont checks that Font parses the font once, with its MATH table.
func TestFont(t *testing.T) {
	var wg sync.WaitGroup
	fonts := make([]*font.Font, 4)
	for i := range fonts {
		wg.Go(func() {
			f, err := stixtwomath.Font()
			if err != nil {
				t.Error(err)
			}
			fonts[i] = f
		})
	}
	wg.Wait()
	f := fonts[0]
	for _, g := range fonts[1:] {
		if g != f {
			t.Fatal("Font parses the font more than once")
		}
	}
	if !f.HasMath() || f.Upem() != 1000 {
		t.Fatalf("the font has a MATH table %v, and %d units per em", f.HasMath(), f.Upem())
	}
	face := font.NewFace(f)
	if got := face.MathConstant(font.AxisHeight); got != 258 {
		t.Errorf("AxisHeight is %v, want 258", got)
	}
	if _, ok := face.NominalGlyph('∑'); !ok {
		t.Error("the font has no glyph for ∑")
	}
}
