// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/head.rs.
//
// The `head` table mostly contains information that can be reused from the
// old table, except for the `loca` format, which depends on the size of the
// glyph data. The checksum will be recalculated in the very end.

// subsetHead is subset in subsetter src/head.rs.
func subsetHead(ctx *context) error {
	data, ok := ctx.expectTable(tagHEAD)
	if !ok || len(data) < 52 {
		return errMalformedFont
	}
	head := make([]byte, len(data))
	copy(head, data)
	head[50] = 0
	head[51] = 0
	if ctx.longLoca {
		head[51] = 1
	}
	ctx.push(tagHEAD, head)
	return nil
}
