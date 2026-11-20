// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package stixtwomath holds the font STIX Two Math, which has an OpenType
// MATH table, for the math layout of texmath (D78). It is the port of
// font_embed.go of github.com/go-tex/math.
//
// The font is a copy of the font of go-tex/math, byte for byte. It is under
// the SIL Open Font License 1.1, which is in OFL.txt beside this file. A
// program that does not import this package does not hold the font, which
// adds about 819 KiB to a binary.
package stixtwomath

import (
	_ "embed"
	"sync"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

//go:embed stixtwomath.otf
var data []byte

// Data returns the bytes of STIX Two Math. Every call returns the same
// slice, so the caller must not change it.
func Data() []byte { return data }

// parsed holds the result of Font.
var parsed struct {
	once sync.Once
	font *font.Font
	err  error
}

// Font returns STIX Two Math, parsed once. A *font.Font is safe for
// concurrent use, and each caller makes its faces with font.NewFace.
func Font() (*font.Font, error) {
	parsed.once.Do(func() {
		ld, err := ot.NewLoader(ot.NewSlicedResource(data))
		if err != nil {
			parsed.err = err
			return
		}
		parsed.font, parsed.err = font.NewFont(ld)
	})
	return parsed.font, parsed.err
}
