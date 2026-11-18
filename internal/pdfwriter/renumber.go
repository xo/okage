// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"bytes"
	"strconv"
)

// This file ports pdf-writer src/renumber.rs.

// renumber is renumber in pdf-writer src/renumber.rs.
//
// Renumbers a chunk of objects.
//
// See Chunk.Renumber for more details.
func renumber(source, target *Chunk, mapping func(Ref) Ref) {
	target.MergeLimits(source.Limits())

	for i, o := range source.offsets {
		id, offset := o.id, o.offset
		n := mapping(id)
		end := len(source.buf.inner)
		if i+1 < len(source.offsets) {
			end = source.offsets[i+1].offset
		}
		slice := source.buf.inner[offset:end]
		gen, slice, ok := extractObject(slice)
		if !ok {
			continue
		}
		target.offsets = append(target.offsets, refOffset{n, len(target.buf.inner)})
		target.buf.pushInt(n.Get())
		target.buf.push(' ')
		target.buf.pushInt(gen)
		target.buf.extendString(" obj\n")
		patchObject(slice, &target.buf, mapping)
		target.buf.extendString("\nendobj\n")

		if target.settings.Pretty {
			target.buf.extendString("\n")
		}
	}
}

// extractObject is extract_object in pdf-writer src/renumber.rs.
//
// Extract the generation number and interior of an indirect object.
func extractObject(slice []byte) (int32, []byte, bool) {
	offset := bytes.Index(slice, []byte("obj"))
	if offset < 0 {
		return 0, nil, false
	}
	prefix := slice[:offset]
	requireWhitespaceRev(&prefix)
	gen, ok := eatNumberRev(&prefix)
	if !ok {
		return 0, nil, false
	}

	head := offset + 3
	for head < len(slice) && isWhitespace(slice[head]) {
		head++
	}

	tail := bytes.LastIndex(slice, []byte("endobj"))
	if tail < 0 {
		return 0, nil, false
	}
	for tail > 0 && isWhitespace(slice[tail-1]) {
		tail--
	}

	if head > tail {
		return 0, nil, false
	}
	return gen, slice[head:tail], true
}

// patchObject is patch_object in pdf-writer src/renumber.rs.
//
// Processes the interior of an indirect object and patches all indirect
// references.
func patchObject(slice []byte, buf *Buf, mapping func(Ref) Ref) {
	// Find the next point of interest:
	// - 'R' is interesting because it could be an indirect reference
	// - Anything that could contain indirect-reference-like things that are not
	//   actually indirect references is interesting
	//   - 's' could start a stream
	//   - '(' starts a string
	//   - Names are not a problem because they can't contain literal whitespace
	//   - Hexadecimal strings are not a problem because they can't contain R
	//   - There are no other collection of arbitrary bytes
	written := 0
	seen := 0
loop:
	for seen < len(slice) {
		switch slice[seen] {
		// Validate whether this is an indirect reference and if it is,
		// patch it!
		case 'R':
			if head, id, gen, ok := validateRef(slice[:seen]); ok {
				n := mapping(id)
				buf.extend(slice[written:head])
				buf.pushInt(n.Get())
				buf.push(' ')
				buf.pushInt(gen)
				buf.push(' ')
				buf.push('R')
				written = seen + 1
			}

		// Skip comments.
		case '%':
			for seen < len(slice) {
				if c := slice[seen]; c == '\n' || c == '\r' {
					break
				}
				seen++
			}

		// Skip strings.
		case '(':
			depth := 0
		str:
			for seen < len(slice) {
				switch c := slice[seen]; {
				case c == '(':
					depth++
				case c == ')' && depth == 1:
					break str
				case c == ')':
					depth--
				case c == '\\':
					seen++
				}
				seen++
			}

		// Check whether this is the start of a stream. If yes, we can bail
		// and copy the rest verbatim.
		case 's':
			if bytes.HasPrefix(slice[seen:], []byte("stream")) && validateStream(slice[:seen]) {
				break loop
			}
		}

		seen++
	}

	buf.extend(slice[written:])
}

// validateRef is validate_ref in pdf-writer src/renumber.rs.
//
// Validate a match for an indirect reference.
func validateRef(prefix []byte) (int, Ref, int32, bool) {
	if !requireWhitespaceRev(&prefix) {
		return 0, Ref{}, 0, false
	}
	gen, ok := eatNumberRev(&prefix)
	if !ok {
		return 0, Ref{}, 0, false
	}
	if !requireWhitespaceRev(&prefix) {
		return 0, Ref{}, 0, false
	}
	id, ok := eatNumberRev(&prefix)
	if !ok || id <= 0 {
		return 0, Ref{}, 0, false
	}
	return len(prefix), NewRef(id), gen, true
}

// validateStream is validate_stream in pdf-writer src/renumber.rs.
//
// Validate a match for a stream.
func validateStream(prefix []byte) bool {
	eatSuffix(&prefix, isWhitespace)
	return bytes.HasSuffix(prefix, []byte(">>"))
}

// requireWhitespaceRev is require_whitespace_rev in pdf-writer
// src/renumber.rs.
//
// Require at least one byte of whitespace, in reverse.
func requireWhitespaceRev(slice *[]byte) bool {
	return len(eatSuffix(slice, isWhitespace)) != 0
}

// eatNumberRev is eat_number_rev in pdf-writer src/renumber.rs.
//
// Eat an ASCII number, in reverse.
func eatNumberRev(slice *[]byte) (int32, bool) {
	tail := eatSuffix(slice, func(c byte) bool { return '0' <= c && c <= '9' })
	n, err := strconv.ParseInt(string(tail), 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(n), true
}

// eatSuffix is eat_suffix in pdf-writer src/renumber.rs.
//
// Eat a suffix that fulfills a predicate.
func eatSuffix(slice *[]byte, predicate func(byte) bool) []byte {
	s := *slice
	i := len(s)
	for i > 0 && predicate(s[i-1]) {
		i--
	}
	head, tail := s[:i], s[i:]
	*slice = head
	return tail
}

// isWhitespace is is_whitespace in pdf-writer src/renumber.rs.
//
// Whether a character is whitespace according to PDF syntax conventions.
func isWhitespace(c byte) bool {
	switch c {
	case '\x00', '\t', '\n', '\x0c', '\r', ' ':
		return true
	}
	return false
}
