// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/charset.rs.

// rewriteCharset is rewrite_charset in subsetter src/cff/charset.rs. It
// rewrites the charset of the font. We do not preserve the CID's from the
// original font. Instead, we assign each glyph it's original glyph as the
// CID. This makes it easier to reference them from the PDF, since we know
// the CID a glyph will have before it's subsetted.
func rewriteCharset(gidMapper *GlyphRemapper, w *writer) {
	if gidMapper.NumGids() == 1 {
		// We only have .notdef, so use format 0.
		w.writeU8(0)
	} else {
		// Use format 2.
		w.writeU8(2)

		w.writeU16(1)
		// -2 because -1 for not including .notdef and -1 since the first
		// glyph in the range is not counted.
		w.writeU16(gidMapper.NumGids() - 2)
	}
}
