// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package svgtypes is a port of the Rust crate svgtypes 0.16.1, a collection
// of parsers for SVG types, at the commit fa62387a.
//
// Supported SVG types:
//
//   - [`<color>`](https://www.w3.org/TR/css-color-3/)
//   - [`<number>`](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGNumber)
//   - [`<length>`](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGLength)
//   - [`<angle>`](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGAngle)
//   - [`<viewBox>`](https://www.w3.org/TR/SVG2/coords.html#ViewBoxAttribute)
//   - [`<path>`](https://www.w3.org/TR/SVG2/paths.html#PathData)
//   - [`<transform>`](https://www.w3.org/TR/SVG11/types.html#DataTypeTransformList)
//   - [`<list-of-numbers>`](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGNumberList)
//   - [`<list-of-lengths>`](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGLengthList)
//   - [`<list-of-points>`](https://www.w3.org/TR/SVG11/shapes.html#PointsBNF)
//   - [`<filter-value-list>`](https://www.w3.org/TR/filter-effects-1/#typedef-filter-value-list)
//   - [`<paint>`](https://www.w3.org/TR/SVG2/painting.html#SpecifyingPaint)
//   - [`<preserveAspectRatio>`](https://www.w3.org/TR/SVG11/coords.html#PreserveAspectRatioAttribute)
//   - [`<enable-background>`](https://www.w3.org/TR/SVG11/filters.html#EnableBackgroundProperty)
//   - [`<IRI>`](https://www.w3.org/TR/SVG11/types.html#DataTypeIRI)
//   - [`<FuncIRI>`](https://www.w3.org/TR/SVG11/types.html#DataTypeFuncIRI)
//   - [`paint-order`](https://www.w3.org/TR/SVG2/painting.html#PaintOrder)
//
// Features:
//
//   - Complete support of paths, so data like `M10-20A5.5.3-4 110-.1` will be parsed correctly.
//   - Implicit path commands will be automatically converted into explicit one.
//   - Some SVG2 data types support.
//
// Limitations:
//
//   - Accepts only [normalized](https://www.w3.org/TR/REC-xml/#AVNormalize) values,
//     e.g. an input text should not contain `&#x20;` or `&data;`.
//   - All keywords must be lowercase.
//     Case-insensitive parsing is supported only for colors (requires allocation for named colors).
//   - The `<color>` followed by the `<icccolor>` is not supported. As the `<icccolor>` itself.
//   - [System colors](https://www.w3.org/TR/css3-color/#css2-system), like `fill="AppWorkspace"`,
//     are not supported. They were deprecated anyway.
//
// The port follows the Go form of docs/UPSTREAM.md. The impl FromStr of a
// type T becomes the function ParseT, and the impl From<&str> of a parser
// becomes NewT. A parser has the method Next, the port of Iterator::next,
// and the method All, which gives the same values as an iterator. A Rust
// enum with data becomes a struct with a kind (D27). The errors are the
// sentinel errors ErrSVGTypes* of the root package (D21).
//
// A number gives the same float64 as upstream: the scanner passes only the
// text that upstream scans to strconv.ParseFloat, which rounds correctly as
// Rust does. The arcs of a path go through internal/kurbo, and sin, cos and
// tan are the correctly rounded functions of internal/crmath (D26).
package svgtypes
