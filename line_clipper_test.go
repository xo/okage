// Copyright 2011 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// lineClipperCases holds the cases of TestLineClipperGolden. It ports no
// upstream test. The expected output came from clip and intersect of
// tiny-skia 0.12.0. A CLIP line has the two points, the flag
// can_cull_to_the_right and the points that clip returns. An ISECT line has
// the two points, the result of intersect and the two points of dst. Each
// number is the bits of a float32 in hexadecimal. The clip rect is (0, 0,
// 200, 150).
const lineClipperCases = `
CLIP c2476a56 c23fddba c285b5a2 42b4341a 1 00000000 00000000 00000000 42b4341a
ISECT 42fff4b2 c16735c0 434d555c 434f519e 1 430504b9 00000000 4339570c 43160000
CLIP be85b100 4383cb3c c143a720 435e9a28 1
ISECT c2454bc8 4320525e c1193d38 438a12e5 0 00000000 00000000 00000000 00000000
CLIP 4323dc94 4388d90a 42cfe5c8 4271deec 0 43011b09 43160000 42cfe5c8 4271deec
ISECT 42d3c63e 438ce96b 43283646 42020998 1 430ad753 43160000 43283646 42020998
CLIP 4387ead7 43767d3e 40a12020 42fcdd70 0 4265a28b 43160000 40a12020 42fcdd70
ISECT 435d0b52 43808557 42d9bc1c 434386c2 0 00000000 00000000 00000000 00000000
CLIP 42f3afdc c2948e86 43913a8f 4371b102 0 43217b3a 00000000 43480000 42905cad 43480000 43160000
ISECT 42072de4 43872290 c2c0b49d 438568b3 0 00000000 00000000 00000000 00000000
CLIP 433c8408 c2b77410 4363d698 436b9084 0 4347894a 00000000 43480000 407703f3 43480000 43160000
ISECT 42490c54 42a69f4c 438c3d2e 4268a3e0 1 42490c54 42a69f4c 43480000 4285e78e
CLIP 42fec518 418ece90 426fcf28 4348d15c 1 42fec518 418ece90 429d5ccd 43160000
ISECT 43395b14 4387a0bb 438c0dc5 42734ee8 0 00000000 00000000 00000000 00000000
CLIP 430ea14b c2a72546 439349cf 43303e7c 0 433f8158 00000000 43480000 416867e7 43480000 43160000
ISECT 4392fb1d c09b4900 4289b36a 43199b41 1 43480000 42752753 4293f29d 43160000
CLIP 43863162 42a7ff30 4349eebe 42d4eec4 1
ISECT 40b5bd30 438babd8 4353de9a 436c208e 0 00000000 00000000 00000000 00000000
CLIP 4191d5a0 3eae9a00 42bc1644 c292f575 0 4191d5a0 3eae9a00 4194a2e5 00000000
ISECT 438b3484 c113ef20 c2bf9410 41981d48 1 431bfbc5 00000000 00000000 413c7b70
CLIP 42410a6c 40f94220 4336acc4 c2876a96 1 42410a6c 40f94220 42788341 00000000
ISECT 43404c7e c29b95b2 c26fe6c7 435370ca 1 42f8e4cf 00000000 00000000 430eadbe
CLIP c2c61a15 42cc9c7c 4256e60c 42a75f76 1 00000000 42cc9c7c 00000000 42b477d1 4256e60c 42a75f76
ISECT 427e1980 42ebbae4 c2aee94c 42e7c88a 1 427e1980 42ebbae4 00000000 42ea11c9
`

// TestLineClipperGolden runs each case of lineClipperCases.
func TestLineClipperGolden(t *testing.T) {
	h32 := func(v float32) string { return fmt.Sprintf("%08x", math.Float32bits(v)) }
	r, _ := path.RectFromLTRB(0, 0, 200, 150)
	for line := range strings.SplitSeq(strings.TrimSpace(lineClipperCases), "\n") {
		f := strings.Fields(line)
		p0 := path.PointFromXY(f32FromHex(t, f[1]), f32FromHex(t, f[2]))
		p1 := path.PointFromXY(f32FromHex(t, f[3]), f32FromHex(t, f[4]))
		var got string
		switch f[0] {
		case "CLIP":
			var pts [maxPoints]Point
			res := clip(&[2]Point{p0, p1}, &r, f[5] == "1", &pts)
			var sb strings.Builder
			sb.WriteString(strings.Join(f[:6], " "))
			for _, p := range res {
				sb.WriteString(" " + h32(p.X) + " " + h32(p.Y))
			}
			got = sb.String()
		case "ISECT":
			var dst [2]Point
			ok := 0
			if intersect(&[2]Point{p0, p1}, &r, &dst) {
				ok = 1
			}
			got = fmt.Sprintf("%s %d %s %s %s %s", strings.Join(f[:5], " "), ok,
				h32(dst[0].X), h32(dst[0].Y), h32(dst[1].X), h32(dst[1].Y))
		}
		if got != line {
			t.Errorf("got  %s\nwant %s", got, line)
		}
	}
}
