// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/error.rs. The enum Error becomes the sentinel
// errors ErrSVGTypes* of the root package (D21). A case with data wraps its
// sentinel error, and the message ends with the Display text of upstream, so
// that errors.Is finds the case and the message keeps the data.

import (
	"fmt"
	"strings"

	"github.com/xo/okage"
)

// errUnexpectedData is Error::UnexpectedData in svgtypes src/error.rs.
//
// An input text contains unknown data.
func errUnexpectedData(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesUnexpectedData, pos)
}

// errInvalidChar is Error::InvalidChar in svgtypes src/error.rs.
//
// An invalid/unexpected character.
//
// The first byte is an actual one, others - expected.
func errInvalidChar(chars []byte, pos int) error {
	list := make([]string, 0, len(chars)-1)
	for _, c := range chars[1:] {
		list = append(list, string([]byte{c}))
	}
	// Rust writes chars[0] as a char, which maps a byte to the code point
	// with the same value.
	return fmt.Errorf("%w: expected '%s' not '%s' at position %d",
		okage.ErrSVGTypesInvalidChar, strings.Join(list, "', '"), string(rune(chars[0])), pos)
}

// errInvalidString is Error::InvalidString in svgtypes src/error.rs.
//
// An unexpected character instead of an XML space.
//
// The first string is an actual one, others - expected.
func errInvalidString(strs []string, pos int) error {
	return fmt.Errorf("%w: expected '%s' not '%s' at position %d",
		okage.ErrSVGTypesInvalidString, strings.Join(strs[1:], "', '"), strs[0], pos)
}

// errInvalidNumber is Error::InvalidNumber in svgtypes src/error.rs.
//
// An invalid number.
func errInvalidNumber(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesInvalidNumber, pos)
}
