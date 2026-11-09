// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package okage is a port of the Rust crate tiny-skia 0.12.0, a tiny subset
// of Skia (https://skia.org/).
//
// The API of okage is a bit unconventional. It does not look like cairo,
// QPainter (Qt), HTML Canvas or even Skia itself. Instead, okage gives a set
// of low-level drawing APIs, and a user manages the world transform, the
// clipping mask and the style manually.
//
// The package path holds the paths and the geometry types. okage declares a
// type alias for each type of path that tiny-skia exports again, so that a
// user needs one import.
//
// The renderer gives the same bytes as the scalar build of tiny-skia (D16). A
// multiply that is followed by an add is written as float32(x*y) + z, so that
// the Go compiler cannot fuse the two operations.
package okage
