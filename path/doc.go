// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package path is a port of the Rust crate tiny-skia-path 0.12.0, a Bezier
// path of tiny-skia, and of the crate strict-num 0.1.1, which holds the checked
// float types.
//
// The package holds a memory-efficient container for a Bezier path, a path
// builder, a path stroker and a path dasher.
//
// It also holds some basic geometry types. Upstream says that they will move
// to an external crate eventually.
//
// All types use single precision floats (float32), as Skia does.
//
// The arithmetic gives the same bits as the Rust code. A multiply that is
// followed by an add is written as float32(x*y) + z, so that the Go compiler
// cannot fuse the two operations (D16).
package path
