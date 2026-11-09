// Copyright 2016 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package simplecss is a port of the Rust crate simplecss 0.2.2, a simple
// CSS 2.1 parser and selector. usvg parses each style element with
// StyleSheet.ParseMore, matches each selector against the nodes of the XML
// tree through the interface Element, and reads each style attribute with
// DeclarationTokenizer (D8).
//
// This is not a browser-grade CSS parser.
//
// # Limitations
//
//   - At-rules are not supported. They will be skipped during parsing.
//   - Property values are not parsed. In CSS like `* { width: 5px }` you
//     will get a `width` property with a `5px` value as a string.
//   - CDO/CDC comments are not supported.
//   - Parser is case sensitive. All keywords must be lowercase.
//   - Unicode escape, like `\26`, is not supported.
//
// # Features
//
//   - Selector matching support.
//   - The rules are sorted by specificity.
//   - `!important` parsing support.
//   - Has a high-level parsers and low-level, zero-allocation tokenizers.
//
// Upstream logs its warnings with the crate log. The port logs them to the
// *slog.Logger that WithLogger gives, with the text of upstream, and logs
// nothing without it (D20). The errors of upstream wrap the sentinel errors
// okage.ErrCSS..., so that errors.Is finds them (D21).
package simplecss
