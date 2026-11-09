// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/roxmltree"
)

// This file ports usvg src/parser/mod.rs, without the items of the feature
// text.
//
// Port: the enum Error becomes the sentinel errors ErrSVG* of the root
// package (D21). The case SvgzFeatureNotEnabled has no port, because okage
// always reads SVGZ data with compress/gzip (D7). The case
// ParsingFailed(roxmltree::Error) wraps okage.ErrSVGParsingFailed with the
// error of the XML parser. The impl From<roxmltree::Error> for Error is the
// function parsingFailed, and the impl Display for Error is the message of
// each sentinel error.

// parsingFailed is the impl From<roxmltree::Error> for Error in usvg
// src/parser/mod.rs. It wraps okage.ErrSVGParsingFailed with err.
func parsingFailed(err error) error {
	return fmt.Errorf("%w: %w", okage.ErrSVGParsingFailed, err)
}

// logNone is OptionLog::log_none for Option<T> in usvg src/parser/mod.rs. It
// calls f when ok is false, and returns v and ok as they are.
func logNone[T any](v T, ok bool, f func()) (T, bool) {
	if !ok {
		f()
	}
	return v, ok
}

// FromData is Tree::from_data in usvg src/parser/mod.rs. It parses `Tree`
// from an SVG data.
//
// Can contain an SVG string or a gzip compressed data.
func FromData(data []byte, opts ...Option) (*Tree, error) {
	return fromData(data, newOptions(opts...))
}

// fromData is FromData with the options already made.
func fromData(data []byte, opt *options) (*Tree, error) {
	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		decoded, err := DecompressSVGZ(data)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(decoded) {
			return nil, okage.ErrSVGNotAnUTF8Str
		}
		return fromStr(string(decoded), opt)
	}
	if !utf8.Valid(data) {
		return nil, okage.ErrSVGNotAnUTF8Str
	}
	return fromStr(string(data), opt)
}

// FromDataNested is Tree::from_data_nested in usvg src/parser/mod.rs.
//
// Similar to the `from_data` method, except that it ignores all `image` elements linking to
// external files, as required by the SVG specification when SVG files are loaded
// for `<image href="..." />` tags.
func FromDataNested(data []byte, opts ...Option) (*Tree, error) {
	return fromDataNested(data, newOptions(opts...))
}

// fromDataNested is FromDataNested with the options already made.
func fromDataNested(data []byte, opt *options) (*Tree, error) {
	nestedOpt := defaultOptions()
	nestedOpt.resourcesDir = ""
	nestedOpt.dpi = opt.dpi
	nestedOpt.fontSize = opt.fontSize
	nestedOpt.languages = opt.languages
	nestedOpt.shapeRendering = opt.shapeRendering
	nestedOpt.textRendering = opt.textRendering
	nestedOpt.imageRendering = opt.imageRendering
	nestedOpt.defaultSize = opt.defaultSize
	nestedOpt.imageHrefResolver = ImageHrefResolver{
		ResolveData: opt.imageHrefResolver.ResolveData,
		// External images should be ignored.
		ResolveString: func(string) (ImageKind, bool) { return nil, false },
	}
	// Port: the logger is not a field of upstream. The nested tree logs to
	// the same logger (D20).
	nestedOpt.logger = opt.logger

	return fromData(data, &nestedOpt)
}

// FromStr is Tree::from_str in usvg src/parser/mod.rs. It parses `Tree` from
// an SVG string.
func FromStr(text string, opts ...Option) (*Tree, error) {
	return fromStr(text, newOptions(opts...))
}

// fromStr is FromStr with the options already made.
//
// Port: Rust has a &str, which is always UTF-8. A Go string can hold other
// bytes, and the XML parser reads a byte of invalid UTF-8 as U+FFFD.
func fromStr(text string, opt *options) (*Tree, error) {
	xmlOpt := roxmltree.DefaultParsingOptions()
	xmlOpt.AllowDTD = true

	doc, err := roxmltree.ParseWithOptions(text, xmlOpt)
	if err != nil {
		return nil, parsingFailed(err)
	}

	return fromXMLTree(doc, opt)
}

// fromXMLTree is Tree::from_xmltree in usvg src/parser/mod.rs. It parses
// `Tree` from `roxmltree::Document`.
//
// Port: the function is unexported, because internal/roxmltree is internal
// and a user cannot make its document (D5).
func fromXMLTree(doc *roxmltree.Document, opt *options) (*Tree, error) {
	svgDoc, err := parseTree(doc, opt.styleSheet, opt)
	if err != nil {
		return nil, parsingFailed(err)
	}
	return convertDoc(svgDoc, opt)
}

// DecompressSVGZ is decompress_svgz in usvg src/parser/mod.rs. It
// decompresses an SVGZ file.
//
// Port: compress/gzip takes the place of the crate flate2 (D7). Like the
// GzDecoder of flate2, it reads only the first member of the gzip data.
func DecompressSVGZ(data []byte) ([]byte, error) {
	decoder, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, okage.ErrSVGMalformedGZip
	}
	decoder.Multistream(false)
	decoded := bytes.NewBuffer(make([]byte, 0, len(data)*2))
	if _, err := io.Copy(decoded, decoder); err != nil { //nolint:gosec // upstream reads the whole stream too
		return nil, okage.ErrSVGMalformedGZip
	}
	return decoded.Bytes(), nil
}

// f32Bound is f32_bound in usvg src/parser/mod.rs.
func f32Bound(minimum, val, maximum float32) float32 {
	switch {
	case val > maximum:
		return maximum
	case val >= minimum:
		return val
	}
	// Catches `val < min` as well as a NaN `val`.
	return minimum
}
