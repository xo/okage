// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// IntRect is IntRect in tiny-skia-path path/src/rect.rs. It is an integer
// rectangle.
//
// The type guarantees these facts:
//   - The width and the height are in the range 1..=i32::MAX.
//   - x+width and y+height do not overflow.
type IntRect struct {
	x      int32
	y      int32
	width  lengthU32
	height lengthU32
}

// IntRectFromXYWH is IntRect::from_xywh in tiny-skia-path path/src/rect.rs.
// It creates a new IntRect.
func IntRectFromXYWH(x, y int32, width, height uint32) (IntRect, bool) {
	if _, ok := checkedAddI32(x, width); !ok {
		return IntRect{}, false
	}
	if _, ok := checkedAddI32(y, height); !ok {
		return IntRect{}, false
	}
	if width == 0 || height == 0 {
		return IntRect{}, false
	}
	return IntRect{x: x, y: y, width: width, height: height}, true
}

// checkedAddI32 ports no upstream function. It is
// x.checked_add(i32::try_from(n).ok()?) in IntRect::from_xywh in
// tiny-skia-path path/src/rect.rs.
func checkedAddI32(x int32, n uint32) (int32, bool) { //nolint:unparam // the signature of upstream is kept
	if n > math.MaxInt32 {
		return 0, false
	}
	sum := int64(x) + int64(n)
	if sum > math.MaxInt32 {
		return 0, false
	}
	return int32(sum), true
}

// checkedSubU32 ports no upstream function. It is
// u32::try_from(a.checked_sub(b)?).ok()? in tiny-skia-path path/src/rect.rs.
func checkedSubU32(a, b int32) (uint32, bool) {
	d := int64(a) - int64(b)
	if d < math.MinInt32 || d > math.MaxInt32 {
		return 0, false
	}
	if d < 0 {
		return 0, false
	}
	return uint32(d), true
}

// IntRectFromLTRB is IntRect::from_ltrb in tiny-skia-path path/src/rect.rs.
// It creates a new IntRect.
func IntRectFromLTRB(left, top, right, bottom int32) (IntRect, bool) {
	width, ok := checkedSubU32(right, left)
	if !ok {
		return IntRect{}, false
	}
	height, ok := checkedSubU32(bottom, top)
	if !ok {
		return IntRect{}, false
	}
	return IntRectFromXYWH(left, top, width, height)
}

// X is IntRect::x in tiny-skia-path path/src/rect.rs. It returns the X
// position of the rectangle.
func (r IntRect) X() int32 {
	return r.x
}

// Y is IntRect::y in tiny-skia-path path/src/rect.rs. It returns the Y
// position of the rectangle.
func (r IntRect) Y() int32 {
	return r.y
}

// Width is IntRect::width in tiny-skia-path path/src/rect.rs. It returns the
// width of the rectangle.
func (r IntRect) Width() uint32 {
	return r.width
}

// Height is IntRect::height in tiny-skia-path path/src/rect.rs. It returns the
// height of the rectangle.
func (r IntRect) Height() uint32 {
	return r.height
}

// Left is IntRect::left in tiny-skia-path path/src/rect.rs. It returns the
// left edge of the rectangle.
func (r IntRect) Left() int32 {
	return r.x
}

// Top is IntRect::top in tiny-skia-path path/src/rect.rs. It returns the top
// edge of the rectangle.
func (r IntRect) Top() int32 {
	return r.y
}

// Right is IntRect::right in tiny-skia-path path/src/rect.rs. It returns the
// right edge of the rectangle.
func (r IntRect) Right() int32 {
	// No overflow is guaranteed by constructors.
	return r.x + int32(r.width)
}

// Bottom is IntRect::bottom in tiny-skia-path path/src/rect.rs. It returns the
// bottom edge of the rectangle.
func (r IntRect) Bottom() int32 {
	// No overflow is guaranteed by constructors.
	return r.y + int32(r.height)
}

// Size is IntRect::size in tiny-skia-path path/src/rect.rs. It returns the
// size of the rectangle.
func (r IntRect) Size() IntSize {
	return intSizeFromWHSafe(r.width, r.height)
}

// Contains is IntRect::contains in tiny-skia-path path/src/rect.rs. It checks
// that the rectangle includes all of other.
func (r IntRect) Contains(other IntRect) bool {
	return r.x <= other.x &&
		r.y <= other.y &&
		r.Right() >= other.Right() &&
		r.Bottom() >= other.Bottom()
}

// Intersect is IntRect::intersect in tiny-skia-path path/src/rect.rs. It
// returns the intersection of two rectangles.
//
// It returns false if the rectangles do not intersect.
func (r IntRect) Intersect(other IntRect) (IntRect, bool) {
	left := max(r.x, other.x)
	top := max(r.y, other.y)

	right := min(r.Right(), other.Right())
	bottom := min(r.Bottom(), other.Bottom())

	w, ok := checkedSubU32(right, left)
	if !ok {
		return IntRect{}, false
	}
	h, ok := checkedSubU32(bottom, top)
	if !ok {
		return IntRect{}, false
	}

	return IntRectFromXYWH(left, top, w, h)
}

// Inset is IntRect::inset in tiny-skia-path path/src/rect.rs. It insets the
// rectangle.
func (r IntRect) Inset(dx, dy int32) (IntRect, bool) {
	return IntRectFromLTRB(
		r.Left()+dx,
		r.Top()+dy,
		r.Right()-dx,
		r.Bottom()-dy,
	)
}

// MakeOutset is IntRect::make_outset in tiny-skia-path path/src/rect.rs. It
// outsets the rectangle.
func (r IntRect) MakeOutset(dx, dy int32) (IntRect, bool) {
	return IntRectFromLTRB(
		saturatingSubI32(r.Left(), dx),
		saturatingSubI32(r.Top(), dy),
		saturatingAddI32(r.Right(), dx),
		saturatingAddI32(r.Bottom(), dy),
	)
}

// Translate is IntRect::translate in tiny-skia-path path/src/rect.rs. It
// translates the rectangle by the given offset.
func (r IntRect) Translate(tx, ty int32) (IntRect, bool) {
	return IntRectFromXYWH(r.X()+tx, r.Y()+ty, r.Width(), r.Height())
}

// TranslateTo is IntRect::translate_to in tiny-skia-path path/src/rect.rs. It
// translates the rectangle to the given position.
func (r IntRect) TranslateTo(x, y int32) (IntRect, bool) {
	return IntRectFromXYWH(x, y, r.Width(), r.Height())
}

// ToRect is IntRect::to_rect in tiny-skia-path path/src/rect.rs. It converts
// the rectangle into a Rect.
func (r IntRect) ToRect() Rect {
	// Can't fail, because `IntRect` is always valid.
	rect, ok := RectFromLTRB(
		float32(r.x),
		float32(r.y),
		float32(r.x)+float32(r.width),
		float32(r.y)+float32(r.height),
	)
	if !ok {
		panic("path: IntRect.ToRect: Rect::from_ltrb returned None")
	}
	return rect
}

// saturatingAddI32 ports no upstream function. It is i32::saturating_add.
func saturatingAddI32(a, b int32) int32 {
	s := int64(a) + int64(b)
	switch {
	case s > math.MaxInt32:
		return math.MaxInt32
	case s < math.MinInt32:
		return math.MinInt32
	}
	return int32(s)
}

// saturatingSubI32 ports no upstream function. It is i32::saturating_sub.
func saturatingSubI32(a, b int32) int32 {
	s := int64(a) - int64(b)
	switch {
	case s > math.MaxInt32:
		return math.MaxInt32
	case s < math.MinInt32:
		return math.MinInt32
	}
	return int32(s)
}

// Rect is Rect in tiny-skia-path path/src/rect.rs. It is a rectangle defined
// by its left, top, right and bottom edges.
//
// It can have a zero width or height, or both. But not a negative one.
//
// The type guarantees these facts:
//   - All values are finite.
//   - The left edge is <= right.
//   - The top edge is <= bottom.
//   - The width and the height are <= f32::MAX.
type Rect struct {
	left   FiniteF32
	top    FiniteF32
	right  FiniteF32
	bottom FiniteF32
}

// RectFromLTRB is Rect::from_ltrb in tiny-skia-path path/src/rect.rs. It
// creates a new Rect.
func RectFromLTRB(left, top, right, bottom float32) (Rect, bool) { //nolint:dupl // upstream has the same code for NonZeroRect
	l, ok := NewFiniteF32(left)
	if !ok {
		return Rect{}, false
	}
	t, ok := NewFiniteF32(top)
	if !ok {
		return Rect{}, false
	}
	r, ok := NewFiniteF32(right)
	if !ok {
		return Rect{}, false
	}
	b, ok := NewFiniteF32(bottom)
	if !ok {
		return Rect{}, false
	}

	if l.Get() <= r.Get() && t.Get() <= b.Get() {
		// Width and height must not overflow.
		if _, ok := checkedF32Sub(r.Get(), l.Get()); !ok {
			return Rect{}, false
		}
		if _, ok := checkedF32Sub(b.Get(), t.Get()); !ok {
			return Rect{}, false
		}

		return Rect{left: l, top: t, right: r, bottom: b}, true
	}
	return Rect{}, false
}

// RectFromXYWH is Rect::from_xywh in tiny-skia-path path/src/rect.rs. It
// creates a new Rect.
func RectFromXYWH(x, y, w, h float32) (Rect, bool) {
	return RectFromLTRB(x, y, w+x, h+y)
}

// String is the Debug impl of Rect in tiny-skia-path path/src/rect.rs. It
// satisfies the fmt.Stringer interface.
func (r Rect) String() string {
	return "Rect { left: " + debugF32(r.left.Get()) + ", top: " + debugF32(r.top.Get()) +
		", right: " + debugF32(r.right.Get()) + ", bottom: " + debugF32(r.bottom.Get()) + " }"
}

// Left is Rect::left in tiny-skia-path path/src/rect.rs. It returns the left
// edge.
func (r Rect) Left() float32 {
	return r.left.Get()
}

// Top is Rect::top in tiny-skia-path path/src/rect.rs. It returns the top
// edge.
func (r Rect) Top() float32 {
	return r.top.Get()
}

// Right is Rect::right in tiny-skia-path path/src/rect.rs. It returns the
// right edge.
func (r Rect) Right() float32 {
	return r.right.Get()
}

// Bottom is Rect::bottom in tiny-skia-path path/src/rect.rs. It returns the
// bottom edge.
func (r Rect) Bottom() float32 {
	return r.bottom.Get()
}

// X is Rect::x in tiny-skia-path path/src/rect.rs. It returns the X position
// of the rectangle.
func (r Rect) X() float32 {
	return r.left.Get()
}

// Y is Rect::y in tiny-skia-path path/src/rect.rs. It returns the Y position
// of the rectangle.
func (r Rect) Y() float32 {
	return r.top.Get()
}

// Width is Rect::width in tiny-skia-path path/src/rect.rs. It returns the
// width of the rectangle.
func (r Rect) Width() float32 {
	return r.right.Get() - r.left.Get()
}

// Height is Rect::height in tiny-skia-path path/src/rect.rs. It returns the
// height of the rectangle.
func (r Rect) Height() float32 {
	return r.bottom.Get() - r.top.Get()
}

// IsEmpty is Rect::is_empty in tiny-skia-path path/src/rect.rs. It returns
// true if the rectangle is empty.
func (r Rect) IsEmpty() bool {
	return r.left == r.right || r.top == r.bottom
}

// Round is Rect::round in tiny-skia-path path/src/rect.rs. It converts the
// rectangle into an IntRect: it adds 0.5 and discards the fraction.
//
// The width and the height are guaranteed to be >= 1.
func (r Rect) Round() (IntRect, bool) {
	left := SaturateRound(r.Left())
	top := SaturateRound(r.Top())
	right := SaturateRound(r.Right())
	bottom := SaturateRound(r.Bottom())
	return IntRectFromXYWH(
		left,
		top,
		uint32(max(1, saturatingSubI32(right, left))),
		uint32(max(1, saturatingSubI32(bottom, top))),
	)
}

// RoundOut is Rect::round_out in tiny-skia-path path/src/rect.rs. It converts
// the rectangle into an IntRect and rounds outwards.
//
// The width and the height are guaranteed to be >= 1.
func (r Rect) RoundOut() (IntRect, bool) {
	left := SaturateFloor(r.Left())
	top := SaturateFloor(r.Top())
	right := SaturateCeil(r.Right())
	bottom := SaturateCeil(r.Bottom())
	return IntRectFromXYWH(
		left,
		top,
		uint32(max(1, saturatingSubI32(right, left))),
		uint32(max(1, saturatingSubI32(bottom, top))),
	)
}

// Intersect is Rect::intersect in tiny-skia-path path/src/rect.rs. It returns
// the intersection of two rectangles.
//
// It returns false if the rectangles do not intersect.
func (r Rect) Intersect(other Rect) (Rect, bool) {
	left := maxF32(r.X(), other.X())
	top := maxF32(r.Y(), other.Y())

	right := minF32(r.Right(), other.Right())
	bottom := minF32(r.Bottom(), other.Bottom())

	return RectFromLTRB(left, top, right, bottom)
}

// Join is Rect::join in tiny-skia-path path/src/rect.rs. It returns the union
// of two rectangles.
//
// It returns false if the width or the height overflows.
func (r Rect) Join(other Rect) (Rect, bool) {
	if other.IsEmpty() {
		return r, true
	}
	if r.IsEmpty() {
		return other, true
	}

	left := minF32(r.X(), other.X())
	top := minF32(r.Y(), other.Y())

	right := maxF32(r.Right(), other.Right())
	bottom := maxF32(r.Bottom(), other.Bottom())

	return RectFromLTRB(left, top, right, bottom)
}

// RectFromPoints is Rect::from_points in tiny-skia-path path/src/rect.rs. It
// creates a Rect from a slice of points.
//
// It returns false if the slice is empty or if a point holds an infinity or
// NaN.
func RectFromPoints(points []Point) (Rect, bool) {
	var offset int
	var min, max f32x4

	// Handle trivial-size lists without creating vectors, since they won't
	// actually use the vector min/max code
	switch i := len(points); {
	case i == 0:
		return Rect{}, false
	case i == 1:
		x, y := points[0].X, points[0].Y
		return RectFromXYWH(x, y, 0, 0)
	case i == 2:
		x0, y0 := points[0].X, points[0].Y
		x1, y1 := points[1].X, points[1].Y
		// Avoid min/max to preserve a NaN for rejection by from_ltrb
		var l, r, t, b float32
		if x0 < x1 {
			l, r = x0, x1
		} else {
			l, r = x1, x0
		}
		if y0 < y1 {
			t, b = y0, y1
		} else {
			t, b = y1, y0
		}
		return RectFromLTRB(l, t, r, b)
	case i&1 != 0:
		pt := points[0]
		min = f32x4{pt.X, pt.Y, pt.X, pt.Y}
		max = min
		offset = 1
	default:
		pt0 := points[0]
		pt1 := points[1]
		min = f32x4{pt0.X, pt0.Y, pt1.X, pt1.Y}
		max = min
		offset = 2
	}

	var accum f32x4
	for offset != len(points) {
		pt0 := points[offset+0]
		pt1 := points[offset+1]
		xy := f32x4{pt0.X, pt0.Y, pt1.X, pt1.Y}

		accum.mulAssign(xy)
		min = min.min(xy)
		max = max.max(xy)
		offset += 2
	}

	allFinite := accum.mul(f32x4{}) == f32x4{}
	if allFinite {
		return RectFromLTRB(
			minF32(min[0], min[2]),
			minF32(min[1], min[3]),
			maxF32(max[0], max[2]),
			maxF32(max[1], max[3]),
		)
	}
	return Rect{}, false
}

// Inset is Rect::inset in tiny-skia-path path/src/rect.rs. It insets the
// rectangle by the given offset.
func (r Rect) Inset(dx, dy float32) (Rect, bool) {
	return RectFromLTRB(
		r.Left()+dx,
		r.Top()+dy,
		r.Right()-dx,
		r.Bottom()-dy,
	)
}

// Outset is Rect::outset in tiny-skia-path path/src/rect.rs. It outsets the
// rectangle by the given offset.
func (r Rect) Outset(dx, dy float32) (Rect, bool) {
	return r.Inset(-dx, -dy)
}

// Transform is Rect::transform in tiny-skia-path path/src/rect.rs. It
// transforms the rectangle with the given Transform.
//
// If the transform is a skew, the result is a bounding box around the skewed
// rectangle.
func (r Rect) Transform(ts Transform) (Rect, bool) {
	if ts.IsIdentity() {
		return r, true
	} else if ts.HasSkew() {
		// we need to transform all 4 corners
		lt := PointFromXY(r.Left(), r.Top())
		rt := PointFromXY(r.Right(), r.Top())
		lb := PointFromXY(r.Left(), r.Bottom())
		rb := PointFromXY(r.Right(), r.Bottom())
		pts := [4]Point{lt, rt, lb, rb}
		ts.MapPoints(pts[:])
		return RectFromPoints(pts[:])
	}
	// Faster (more common) case
	lt := PointFromXY(r.Left(), r.Top())
	rb := PointFromXY(r.Right(), r.Bottom())
	pts := [2]Point{lt, rb}
	ts.MapPoints(pts[:])
	return RectFromPoints(pts[:])
}

// BBoxTransform is Rect::bbox_transform in tiny-skia-path path/src/rect.rs.
// It applies a bounding box transform.
func (r Rect) BBoxTransform(bbox NonZeroRect) Rect {
	x := float32(r.X()*bbox.Width()) + bbox.X()
	y := float32(r.Y()*bbox.Height()) + bbox.Y()
	w := float32(r.Width() * bbox.Width())
	h := float32(r.Height() * bbox.Height())
	rect, ok := RectFromXYWH(x, y, w, h)
	if !ok {
		panic("path: Rect.BBoxTransform: Rect::from_xywh returned None")
	}
	return rect
}

// ToNonZeroRect is Rect::to_non_zero_rect in tiny-skia-path path/src/rect.rs.
// It converts the rectangle into a NonZeroRect.
func (r Rect) ToNonZeroRect() (NonZeroRect, bool) {
	return NonZeroRectFromXYWH(r.X(), r.Y(), r.Width(), r.Height())
}

// checkedF32Sub is checked_f32_sub in tiny-skia-path path/src/rect.rs.
func checkedF32Sub(a, b float32) (float32, bool) { //nolint:unparam // the signature of upstream is kept
	n := float64(a) - float64(b)
	// Not sure if this is perfectly correct.
	if n > float64(-math.MaxFloat32) && n < float64(math.MaxFloat32) {
		return float32(n), true
	}
	return 0, false
}

// NonZeroRect is NonZeroRect in tiny-skia-path path/src/rect.rs. It is a
// rectangle defined by its left, top, right and bottom edges.
//
// It is like Rect, but its width and its height are guaranteed to be positive
// and not zero.
//
// The type guarantees these facts:
//   - All values are finite.
//   - The left edge is < right.
//   - The top edge is < bottom.
//   - The width and the height are <= f32::MAX.
//   - The width and the height are > 0.0.
type NonZeroRect struct {
	left   FiniteF32
	top    FiniteF32
	right  FiniteF32
	bottom FiniteF32
}

// NonZeroRectFromLTRB is NonZeroRect::from_ltrb in tiny-skia-path
// path/src/rect.rs. It creates a new NonZeroRect.
func NonZeroRectFromLTRB(left, top, right, bottom float32) (NonZeroRect, bool) { //nolint:dupl // upstream has the same code for Rect
	l, ok := NewFiniteF32(left)
	if !ok {
		return NonZeroRect{}, false
	}
	t, ok := NewFiniteF32(top)
	if !ok {
		return NonZeroRect{}, false
	}
	r, ok := NewFiniteF32(right)
	if !ok {
		return NonZeroRect{}, false
	}
	b, ok := NewFiniteF32(bottom)
	if !ok {
		return NonZeroRect{}, false
	}

	if l.Get() < r.Get() && t.Get() < b.Get() {
		// Width and height must not overflow.
		if _, ok := checkedF32Sub(r.Get(), l.Get()); !ok {
			return NonZeroRect{}, false
		}
		if _, ok := checkedF32Sub(b.Get(), t.Get()); !ok {
			return NonZeroRect{}, false
		}

		return NonZeroRect{left: l, top: t, right: r, bottom: b}, true
	}
	return NonZeroRect{}, false
}

// NonZeroRectFromXYWH is NonZeroRect::from_xywh in tiny-skia-path
// path/src/rect.rs. It creates a new NonZeroRect.
func NonZeroRectFromXYWH(x, y, w, h float32) (NonZeroRect, bool) {
	return NonZeroRectFromLTRB(x, y, w+x, h+y)
}

// Left is NonZeroRect::left in tiny-skia-path path/src/rect.rs. It returns the
// left edge.
func (r NonZeroRect) Left() float32 {
	return r.left.Get()
}

// Top is NonZeroRect::top in tiny-skia-path path/src/rect.rs. It returns the
// top edge.
func (r NonZeroRect) Top() float32 {
	return r.top.Get()
}

// Right is NonZeroRect::right in tiny-skia-path path/src/rect.rs. It returns
// the right edge.
func (r NonZeroRect) Right() float32 {
	return r.right.Get()
}

// Bottom is NonZeroRect::bottom in tiny-skia-path path/src/rect.rs. It returns
// the bottom edge.
func (r NonZeroRect) Bottom() float32 {
	return r.bottom.Get()
}

// X is NonZeroRect::x in tiny-skia-path path/src/rect.rs. It returns the X
// position of the rectangle.
func (r NonZeroRect) X() float32 {
	return r.left.Get()
}

// Y is NonZeroRect::y in tiny-skia-path path/src/rect.rs. It returns the Y
// position of the rectangle.
func (r NonZeroRect) Y() float32 {
	return r.top.Get()
}

// Width is NonZeroRect::width in tiny-skia-path path/src/rect.rs. It returns
// the width of the rectangle.
func (r NonZeroRect) Width() float32 {
	return r.right.Get() - r.left.Get()
}

// Height is NonZeroRect::height in tiny-skia-path path/src/rect.rs. It returns
// the height of the rectangle.
func (r NonZeroRect) Height() float32 {
	return r.bottom.Get() - r.top.Get()
}

// Size is NonZeroRect::size in tiny-skia-path path/src/rect.rs. It returns the
// size of the rectangle.
func (r NonZeroRect) Size() Size {
	s, ok := SizeFromWH(r.Width(), r.Height())
	if !ok {
		panic("path: NonZeroRect.Size: Size::from_wh returned None")
	}
	return s
}

// TranslateTo is NonZeroRect::translate_to in tiny-skia-path
// path/src/rect.rs. It translates the rectangle to the given position.
func (r NonZeroRect) TranslateTo(x, y float32) (NonZeroRect, bool) {
	return NonZeroRectFromXYWH(x, y, r.Width(), r.Height())
}

// Transform is NonZeroRect::transform in tiny-skia-path path/src/rect.rs. It
// transforms the rectangle with the given Transform.
//
// If the transform is a skew, the result is a bounding box around the skewed
// rectangle.
func (r NonZeroRect) Transform(ts Transform) (NonZeroRect, bool) {
	if ts.IsIdentity() {
		return r, true
	}
	rect, ok := r.ToRect().Transform(ts)
	if !ok {
		return NonZeroRect{}, false
	}
	return rect.ToNonZeroRect()
}

// BBoxTransform is NonZeroRect::bbox_transform in tiny-skia-path
// path/src/rect.rs. It applies a bounding box transform.
func (r NonZeroRect) BBoxTransform(bbox NonZeroRect) NonZeroRect {
	x := float32(r.X()*bbox.Width()) + bbox.X()
	y := float32(r.Y()*bbox.Height()) + bbox.Y()
	w := float32(r.Width() * bbox.Width())
	h := float32(r.Height() * bbox.Height())
	rect, ok := NonZeroRectFromXYWH(x, y, w, h)
	if !ok {
		panic("path: NonZeroRect.BBoxTransform: NonZeroRect::from_xywh returned None")
	}
	return rect
}

// ToRect is NonZeroRect::to_rect in tiny-skia-path path/src/rect.rs. It
// converts the rectangle into a Rect.
func (r NonZeroRect) ToRect() Rect {
	rect, ok := RectFromXYWH(r.X(), r.Y(), r.Width(), r.Height())
	if !ok {
		panic("path: NonZeroRect.ToRect: Rect::from_xywh returned None")
	}
	return rect
}

// ToIntRect is NonZeroRect::to_int_rect in tiny-skia-path path/src/rect.rs.
// It converts the rectangle into an IntRect.
func (r NonZeroRect) ToIntRect() IntRect {
	rect, ok := IntRectFromXYWH(
		wide.TruncI32(floorF32(r.X())),
		wide.TruncI32(floorF32(r.Y())),
		max(1, wide.TruncU32(ceilF32(r.Width()))),
		max(1, wide.TruncU32(ceilF32(r.Height()))),
	)
	if !ok {
		panic("path: NonZeroRect.ToIntRect: IntRect::from_xywh returned None")
	}
	return rect
}
