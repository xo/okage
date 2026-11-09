// Copyright (c) 2019 Reizner Evgeniy
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package xmlwriter is a port of the Rust crate xmlwriter 0.1.0, a simple,
// streaming, partially-validating XML writer that writes XML data into an
// internal buffer. usvg writes its tree back to SVG text with it.
//
// # Features
//
//   - A simple, bare-minimum, panic-based API.
//   - Nodes auto-closing.
//
// Upstream takes a struct Options. The port takes the functional options
// WithUseSingleQuote, WithIndent and WithAttributesIndent, with the defaults
// of upstream (D20). Upstream formats values with fmt::Display and
// fmt::Arguments. The port formats them with the package fmt, which gives the
// same text for a string and an integer.
package xmlwriter
