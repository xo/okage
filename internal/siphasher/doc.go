// Copyright 2012-2016 The Rust Project Developers
// Copyright 2016-2026 Frank Denis
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package siphasher is a port of the module sip128 of the Rust crate
// siphasher 1.0.2, at the commit db817204. It holds SipHash-2-4 and
// SipHash-1-3 with a 128-bit output.
//
// krilla hashes the glyphs of a font subset with SipHasher13 to make the tag
// of six letters in front of the name of the font. The tag is in the PDF
// file, so the hash must give the bytes of upstream (D62). The 64-bit mode of
// the module sip is not ported, because krilla does not call it.
//
// Upstream reads the bytes of an integer in the byte order of the machine.
// The port reads them in little-endian order on every machine, as upstream
// does on a little-endian machine, so that each build gives the same hash
// (D16). The maintainers accepted this difference on 2026-11-18.
//
// A Rust value of the type Default is the hasher with the keys 0 and 0. The
// zero value of a Go hasher is not ready for use. Make one with a
// constructor, such as NewSipHasher13.
package siphasher
