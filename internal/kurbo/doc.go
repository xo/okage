// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package kurbo is a port of a subset of the Rust crate kurbo 0.13.1, a 2D
// curves library, at the commit 838b69ef.
//
// okage ports only the types and functions that svgtypes and usvg call, and
// what they need (D7): Point, Vec2, Line, QuadBez, CubicBez, SvgArc, Arc, the
// traits ParamCurve, ParamCurveDeriv and ParamCurveArclen, and the parts of
// common.rs that they use. SUBSET.md lists each item, with the file of
// svgtypes or usvg that uses it.
//
// The arithmetic gives the same bits as the Rust code. A multiply that is
// followed by an add is written as float64(x*y) + z, so that the Go compiler
// cannot fuse the two operations (D16). The functions that Rust takes from
// the C library, such as sin and atan2, are the correctly rounded functions
// of internal/crmath (D26). The C library of the Rust oracle can differ from
// them in the last bit.
package kurbo
