// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

// Package harfbuzz gives the files of the folder harfbuzz of the module
// github.com/go-text/typesetting-utils, which okage copies into
// typesetting/testdata/harfbuzz (D63, D76). Only the tests of typesetting use
// it.
package harfbuzz

import "github.com/xo/okage/typesetting/internal/testfonts"

// Files provides font samples used for the harfbuzz test suite.
//
// okage: the files are in typesetting/testdata/harfbuzz, a module of its
// own, and they are not embedded (D76).
var Files = testfonts.NewDir("harfbuzz")
