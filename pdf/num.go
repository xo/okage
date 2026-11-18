// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/path"

// NormalizedF32 is NormalizedF32 in krilla src/num.rs. krilla wraps the
// NormalizedF32 of tiny-skia-path, and the port takes the one of path.
//
// An immutable, finite float32 in a 0..=1 range.
type NormalizedF32 = path.NormalizedF32

// The constants of NormalizedF32 in krilla src/num.rs.
const (
	// NormalizedF32Zero is NormalizedF32::ZERO. A NormalizedF32 value
	// initialized with zero.
	NormalizedF32Zero = path.NormalizedF32Zero
	// NormalizedF32One is NormalizedF32::ONE. A NormalizedF32 value
	// initialized with one.
	NormalizedF32One = path.NormalizedF32One
)

// NewNormalizedF32 is NormalizedF32::new in krilla src/num.rs. It returns
// false if the number is not finite or not in the range 0..=1.
//
// Create a new normalized f32.
func NewNormalizedF32(num float32) (NormalizedF32, bool) {
	return path.NewNormalizedF32(num)
}
