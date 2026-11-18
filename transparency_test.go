package okage_test

// This file tests the transparency of PDF and XPS that D56 adds: soft masks
// of luminance, isolated groups, groups that are not isolated, and knockout
// groups. Each expected value is computed by hand in the comment next to it.

import (
	"bytes"
	"testing"

	"github.com/xo/okage"
)

// pixmapOf returns a pixmap of height 1 with the premultiplied pixels px.
func pixmapOf(t *testing.T, px ...[4]uint8) *okage.Pixmap {
	t.Helper()
	p := must(okage.NewPixmap(uint32(len(px)), 1))
	for i, c := range px {
		copy(p.DataMut()[i*4:], c[:])
	}
	return p
}

// maskOf returns a mask of height 1 with the values v.
func maskOf(t *testing.T, v ...uint8) *okage.Mask {
	t.Helper()
	m := must(okage.NewMask(uint32(len(v)), 1))
	copy(m.DataMut(), v)
	return m
}

// assertPixels fails t when the pixels of p are not want.
func assertPixels(t *testing.T, p *okage.Pixmap, want ...[4]uint8) {
	t.Helper()
	var w []byte
	for _, c := range want {
		w = append(w, c[:]...)
	}
	if !bytes.Equal(p.Data(), w) {
		t.Errorf("got pixels %v, want %v", p.Data(), w)
	}
}

// fillRect fills the rectangle x, y, w, h of p with the color c, which is
// not premultiplied, with the blend mode bm and no anti-aliasing.
func fillRect(p *okage.Pixmap, x, y, w, h float32, c [4]uint8, bm okage.BlendMode) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(c[0], c[1], c[2], c[3])
	paint.BlendMode = bm
	paint.AntiAlias = false
	p.FillRect(must(okage.RectFromXYWH(x, y, w, h)), &paint, okage.IdentityTransform(), nil)
}

func TestMaskFromPixmapLuminance(t *testing.T) {
	// Y = 0.2126 R + 0.7152 G + 0.0722 B of the demultiplied color, times
	// the alpha, times 255, rounded up.
	p := pixmapOf(t,
		[4]uint8{255, 255, 255, 255}, // 255
		[4]uint8{255, 0, 0, 255},     // 0.2126*255 = 54.2: 55
		[4]uint8{0, 0, 255, 255},     // 0.0722*255 = 18.4: 19
		[4]uint8{0, 128, 0, 128},     // 0.7152*128 = 91.5: 92
		[4]uint8{0, 0, 0, 0},         // 0
	)
	m := okage.MaskFromPixmap(p.AsRef(), okage.MaskTypeLuminance)
	if want := []byte{255, 55, 19, 92, 0}; !bytes.Equal(m.Data(), want) {
		t.Errorf("got %v, want %v", m.Data(), want)
	}
	m = okage.MaskFromPixmap(p.AsRef(), okage.MaskTypeAlpha)
	if want := []byte{255, 255, 255, 128, 0}; !bytes.Equal(m.Data(), want) {
		t.Errorf("got %v, want %v", m.Data(), want)
	}
}

func TestMaskFromLuminance(t *testing.T) {
	p := pixmapOf(t,
		[4]uint8{255, 0, 0, 255},
		[4]uint8{0, 128, 0, 128},
		[4]uint8{0, 0, 0, 0},
	)
	tests := []struct {
		name     string
		backdrop okage.Color
		want     []byte
	}{
		// A black backdrop adds nothing.
		{"black", okage.ColorBlack, []byte{55, 92, 0}},
		// White: the opaque pixel hides it, the half green pixel gives
		// 0.7152*128 + 255*(127/255) = 91.5 + 127 = 218.5: 219, and the
		// transparent pixel gives the backdrop, 255.
		{"white", okage.ColorWhite, []byte{55, 219, 255}},
		// The backdrop of PDF is opaque, so its alpha does not count.
		{"transparent white", okage.ColorFromRGBA8(255, 255, 255, 0), []byte{55, 219, 255}},
		// Red: 54.2 for the opaque pixel, 91.5 + 0.2126*127 = 118.5: 119, and
		// 54.2: 55.
		{"red", okage.ColorFromRGBA8(255, 0, 0, 255), []byte{55, 119, 55}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := okage.MaskFromLuminance(p.AsRef(), tt.backdrop)
			if !bytes.Equal(m.Data(), tt.want) {
				t.Errorf("got %v, want %v", m.Data(), tt.want)
			}
		})
	}
}

func TestMaskFromLuminanceBlackIsUpstream(t *testing.T) {
	// Each premultiplied pixel with the colors in steps of 17 gives the same
	// bytes as MaskFromPixmap with a black backdrop.
	var px [][4]uint8
	for a := 0; a < 256; a += 17 {
		for c := 0; c <= a; c += 17 {
			px = append(px, [4]uint8{uint8(c), uint8(a - c), uint8(c / 2), uint8(a)})
		}
	}
	p := pixmapOf(t, px...)
	want := okage.MaskFromPixmap(p.AsRef(), okage.MaskTypeLuminance)
	got := okage.MaskFromLuminance(p.AsRef(), okage.ColorBlack)
	if !bytes.Equal(got.Data(), want.Data()) {
		t.Errorf("got %v, want %v", got.Data(), want.Data())
	}
}

func TestMaskApplyTransfer(t *testing.T) {
	m := maskOf(t, 0, 1, 128, 255)
	var inv [256]uint8
	for i := range inv {
		inv[i] = 255 - uint8(i)
	}
	m.ApplyTransfer(&inv)
	if want := []byte{255, 254, 127, 0}; !bytes.Equal(m.Data(), want) {
		t.Errorf("got %v, want %v", m.Data(), want)
	}
	// A step function at 0.5, as a transfer function of type 2 or 4 can
	// give.
	var step [256]uint8
	for i := 128; i < 256; i++ {
		step[i] = 255
	}
	m.ApplyTransfer(&step)
	if want := []byte{255, 255, 0, 0}; !bytes.Equal(m.Data(), want) {
		t.Errorf("got %v, want %v", m.Data(), want)
	}
}

func TestIsolatedGroup(t *testing.T) {
	// An isolated group draws into a transparent pixmap. Its blue element
	// with Multiply sees no backdrop, so it stays blue. The group then draws
	// onto the backdrop with Multiply, an opacity of 0.5 and a mask of 255
	// and 128.
	backdrop := pixmapOf(t, [4]uint8{200, 100, 50, 255}, [4]uint8{200, 100, 50, 255})
	layer := must(okage.NewPixmap(2, 1))
	fillRect(layer, 0, 0, 2, 1, [4]uint8{0, 0, 255, 255}, okage.BlendModeMultiply)
	assertPixels(t, layer, [4]uint8{0, 0, 255, 255}, [4]uint8{0, 0, 255, 255})

	pp := okage.DefaultPixmapPaint()
	pp.Opacity = 0.5
	pp.BlendMode = okage.BlendModeMultiply
	backdrop.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), maskOf(t, 255, 128))
	// The source is (0, 0, 0.5, 0.5). Multiply gives s(1-da) + d(1-sa) + sd:
	// red 200*0.5 = 100, green 100*0.5 = 50, blue 50*0.5 + 0.5*50 = 50.
	// The mask of 128 moves the backdrop half way to that result: red 150,
	// green 75, blue 50.
	assertPixels(t, backdrop, [4]uint8{100, 50, 50, 255}, [4]uint8{150, 75, 50, 255})
}

func TestRemoveBackdrop(t *testing.T) {
	tests := []struct {
		name     string
		result   [4]uint8
		backdrop [4]uint8
		alpha    uint8
		want     [4]uint8
	}{
		// The element (0, 0, 128, 128) over the backdrop gave (100, 50, 153,
		// 255). c = cn - c0*(1-ag): red 100 - 200*127/255 = 100 - 100,
		// green 50 - 100*127/255 = 50 - 50, blue 153 - 50*127/255 = 153 -
		// 25. The result is the element again.
		{"half", [4]uint8{100, 50, 153, 255}, [4]uint8{200, 100, 50, 255}, 128, [4]uint8{0, 0, 128, 128}},
		// No element: the backdrop goes, and nothing is left.
		{"empty", [4]uint8{200, 100, 50, 255}, [4]uint8{200, 100, 50, 255}, 0, [4]uint8{0, 0, 0, 0}},
		// An opaque group hides the backdrop, so the result stays.
		{"opaque", [4]uint8{78, 78, 49, 255}, [4]uint8{200, 100, 50, 255}, 255, [4]uint8{78, 78, 49, 255}},
		// A backdrop that is not opaque: the element (60, 0, 0, 60) over (0,
		// 0, 100, 100) gives blue 100*195/255 = 76.5 and alpha 60 + 76.5 =
		// 136.5, so (60, 0, 76, 136). Blue 76 - 76 = 0, and the alpha is 60.
		{"translucent", [4]uint8{60, 0, 76, 136}, [4]uint8{0, 0, 100, 100}, 60, [4]uint8{60, 0, 0, 60}},
		// Input that no group gives: blue 50 - 76 is below 0, and clamps to
		// 0. Red 100 is above the alpha 60, and clamps to 60.
		{"clamp", [4]uint8{100, 0, 50, 136}, [4]uint8{0, 0, 100, 100}, 60, [4]uint8{60, 0, 0, 60}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pixmapOf(t, tt.result)
			p.RemoveBackdrop(pixmapOf(t, tt.backdrop).AsRef(), pixmapOf(t, [4]uint8{9, 9, 9, tt.alpha}).AsRef())
			assertPixels(t, p, tt.want)
		})
	}
}

func TestRemoveBackdropSize(t *testing.T) {
	p := pixmapOf(t, [4]uint8{1, 2, 3, 4})
	p.RemoveBackdrop(pixmapOf(t, [4]uint8{}, [4]uint8{}).AsRef(), pixmapOf(t, [4]uint8{}).AsRef())
	assertPixels(t, p, [4]uint8{1, 2, 3, 4})
}

// drawNonIsolated draws the elements of a group that is not isolated onto
// a copy of backdrop and onto a transparent pixmap, and returns the result
// of the group with RemoveBackdrop.
func drawNonIsolated(t *testing.T, backdrop *okage.Pixmap, draw func(p *okage.Pixmap)) *okage.Pixmap {
	t.Helper()
	group := backdrop.Clone()
	alpha := must(okage.NewPixmap(backdrop.Width(), backdrop.Height()))
	draw(group)
	draw(alpha)
	group.RemoveBackdrop(backdrop.AsRef(), alpha.AsRef())
	return group
}

func TestNonIsolatedGroupMultiply(t *testing.T) {
	// The element (100, 200, 250, 255) with Multiply over the backdrop (200,
	// 100, 50, 255) gives red 100*200/255 = 78.4, green 78.4 and blue
	// 250*50/255 = 49.0. The lowp pipeline of tiny-skia divides by 255 as
	// (v + 255) / 256, so red and green are 79. The group is opaque, so
	// RemoveBackdrop keeps them.
	backdrop := pixmapOf(t, [4]uint8{200, 100, 50, 255})
	group := drawNonIsolated(t, backdrop, func(p *okage.Pixmap) {
		fillRect(p, 0, 0, 1, 1, [4]uint8{100, 200, 250, 255}, okage.BlendModeMultiply)
	})
	assertPixels(t, group, [4]uint8{79, 79, 49, 255})

	// The group draws with an opacity of 0.5: red 79*0.5 + 200*0.5 = 139.5,
	// green 39.5 + 50 = 89.5, blue 24.5 + 25 = 49.5. Each rounds up.
	pp := okage.DefaultPixmapPaint()
	pp.Opacity = 0.5
	backdrop.DrawPixmap(0, 0, group.AsRef(), &pp, okage.IdentityTransform(), nil)
	assertPixels(t, backdrop, [4]uint8{140, 90, 50, 255})

	// An isolated group gives another result: Multiply onto a transparent
	// pixmap keeps the element, and half of it over the backdrop gives 150
	// for each color.
	isolated := pixmapOf(t, [4]uint8{200, 100, 50, 255})
	layer := must(okage.NewPixmap(1, 1))
	fillRect(layer, 0, 0, 1, 1, [4]uint8{100, 200, 250, 255}, okage.BlendModeMultiply)
	isolated.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), nil)
	assertPixels(t, isolated, [4]uint8{150, 150, 150, 255})
}

func TestNonIsolatedGroupRoundTrip(t *testing.T) {
	// A group that is not isolated, with SourceOver, an opacity of 1 and no
	// mask, gives the colors of its elements drawn onto the backdrop. Over
	// an opaque backdrop it gives the same bytes. Over a backdrop that is
	// not opaque, the alpha can differ by 1.
	for _, opaque := range []bool{true, false} {
		backdrop := must(okage.NewPixmap(64, 48))
		for y := range uint32(48) {
			for x := range uint32(64) {
				a := uint8(255)
				if !opaque {
					a = uint8(x * 4)
				}
				c := min(uint8(y*5), a)
				backdrop.PixelsMut().Set(int(y*64+x), must(okage.PremultipliedColorU8FromRGBA(c, a/2, a-c, a)))
			}
		}
		draw := func(p *okage.Pixmap) {
			paint := okage.DefaultPaint()
			paint.AntiAlias = true
			paint.SetColorRGBA8(200, 30, 90, 150)
			circle := must(okage.PathFromCircle(30, 20, 17))
			p.FillPath(circle, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
			paint.SetColorRGBA8(20, 180, 60, 90)
			paint.BlendMode = okage.BlendModeScreen
			p.FillRect(must(okage.RectFromXYWH(10.5, 8.25, 40, 30)), &paint, okage.IdentityTransform(), nil)
		}
		want := backdrop.Clone()
		draw(want)

		group := drawNonIsolated(t, backdrop, draw)
		pp := okage.DefaultPixmapPaint()
		backdrop.DrawPixmap(0, 0, group.AsRef(), &pp, okage.IdentityTransform(), nil)
		if opaque {
			assertPixmapEqual(t, backdrop, want)
			continue
		}
		for i, v := range backdrop.Data() {
			w := want.Data()[i]
			if i%4 != 3 && v != w || i%4 == 3 && max(v, w)-min(v, w) > 1 {
				t.Fatalf("byte %d: got %d, want %d", i, v, w)
			}
		}
	}
}

func TestKnockout(t *testing.T) {
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}
	tests := []struct {
		name     string
		prev     [4]uint8
		element  [4]uint8
		backdrop *[4]uint8
		shape    uint8
		want     [4]uint8
	}{
		// Isolated: the half blue element (0, 0, 128, 128) replaces red.
		{"isolated full", red, [4]uint8{0, 0, 128, 128}, nil, 255, [4]uint8{0, 0, 128, 128}},
		// Isolated, at a coverage of 128: the element drawn with that
		// coverage is (0, 0, 64, 64), and red*127/255 = 127 stays.
		{"isolated edge", red, [4]uint8{0, 0, 64, 64}, nil, 128, [4]uint8{127, 0, 64, 191}},
		// Isolated, outside the element: red stays.
		{"isolated outside", red, [4]uint8{}, nil, 0, red},
		// Not isolated, over blue: the earlier half red element gave (128, 0,
		// 127, 255). The half green element over the backdrop gives (0, 128,
		// 127, 255), and replaces it.
		{"backdrop full", [4]uint8{128, 0, 127, 255}, [4]uint8{0, 128, 127, 255}, &blue, 255, [4]uint8{0, 128, 127, 255}},
		// At a coverage of 128, the element over the backdrop is (0, 64, 191,
		// 255). Red 0 + 128*127/255 = 64, blue 191 + (127-255)*127/255 = 191
		// - 64 = 127.
		{"backdrop edge", [4]uint8{128, 0, 127, 255}, [4]uint8{0, 64, 191, 255}, &blue, 128, [4]uint8{64, 64, 127, 255}},
		{"backdrop outside", [4]uint8{128, 0, 127, 255}, blue, &blue, 0, [4]uint8{128, 0, 127, 255}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pixmapOf(t, tt.prev)
			var backdrop okage.PixmapRef
			if tt.backdrop != nil {
				backdrop = pixmapOf(t, *tt.backdrop).AsRef()
			}
			p.Knockout(pixmapOf(t, tt.element).AsRef(), backdrop, maskOf(t, tt.shape))
			assertPixels(t, p, tt.want)
		})
	}
}

func TestKnockoutNilShape(t *testing.T) {
	p := pixmapOf(t, [4]uint8{255, 0, 0, 255})
	p.Knockout(pixmapOf(t, [4]uint8{0, 0, 10, 20}).AsRef(), okage.PixmapRef{}, nil)
	assertPixels(t, p, [4]uint8{0, 0, 10, 20})
}

func TestKnockoutSize(t *testing.T) {
	p := pixmapOf(t, [4]uint8{1, 2, 3, 4})
	p.Knockout(pixmapOf(t, [4]uint8{}).AsRef(), okage.PixmapRef{}, maskOf(t, 1, 2))
	assertPixels(t, p, [4]uint8{1, 2, 3, 4})
	p.Knockout(pixmapOf(t, [4]uint8{}, [4]uint8{}).AsRef(), okage.PixmapRef{}, nil)
	assertPixels(t, p, [4]uint8{1, 2, 3, 4})
}

func TestKnockoutGroup(t *testing.T) {
	// An isolated knockout group of two rectangles: opaque red on x 0 to 3,
	// then half blue on x 1 to 4. The blue knocks out the red where they
	// overlap. A group that is not knockout draws blue over red there:
	// (127, 0, 128, 255).
	elements := []struct {
		x, w float32
		c    [4]uint8
	}{
		{0, 3, [4]uint8{255, 0, 0, 255}},
		{1, 3, [4]uint8{0, 0, 255, 128}},
	}
	group := must(okage.NewPixmap(4, 1))
	normal := must(okage.NewPixmap(4, 1))
	for _, e := range elements {
		element := must(okage.NewPixmap(4, 1))
		fillRect(element, e.x, 0, e.w, 1, e.c, okage.BlendModeSourceOver)
		shape := must(okage.NewMask(4, 1))
		rect := okage.PathFromRect(must(okage.RectFromXYWH(e.x, 0, e.w, 1)))
		shape.FillPath(rect, okage.FillRuleWinding, false, okage.IdentityTransform())
		group.Knockout(element.AsRef(), okage.PixmapRef{}, shape)
		fillRect(normal, e.x, 0, e.w, 1, e.c, okage.BlendModeSourceOver)
	}
	half := [4]uint8{0, 0, 128, 128}
	assertPixels(t, group, [4]uint8{255, 0, 0, 255}, half, half, half)
	assertPixels(t, normal, [4]uint8{255, 0, 0, 255}, [4]uint8{127, 0, 128, 255}, [4]uint8{127, 0, 128, 255}, half)
}
