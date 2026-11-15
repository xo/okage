// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package brotli

import _ "embed"

// These constants are defined in RFC section 8.
const (
	minDictLen = 4
	maxDictLen = 24
)

// Defined in Appendix A of the RFC as NDBITS.
// This maps a word length to the log2 of the number of words of that length.
var dictBitSizes = [maxDictLen + 1]int{
	0, 0, 0, 0, 10, 10, 11, 11, 10, 10, 10, 10, 10, 9, 9, 8, 7, 7, 8, 7, 7, 6, 6, 5, 5,
}

// Defined in RFC section 8 as NWORDS.
// This maps a word length to the number of words of that length.
var dictSizes [maxDictLen + 1]int

// Defined in RFC section 8 as DOFFSET.
// This maps a word length to the starting offset in dictLUT for those words.
var dictOffsets [maxDictLen + 1]int

// We dynamically compute NWORDS and DOFFSET according to the algorithm
// provided in RFC section 8.
func initDictLUTs() {
	// Sweep from minDictLen to maxDictLen, inclusive.
	for i := minDictLen; i <= maxDictLen; i++ {
		dictSizes[i] = 1 << uint(dictBitSizes[i])
		dictOffsets[i] = dictOffsets[i-1] + (i-1)*dictSizes[i-1]
	}
}

// dictLUT is DICT of Appendix A of the RFC: the static dictionary of words
// and phrases that are common in web content.
//
//go:embed dictionary.bin
var dictLUT []byte
