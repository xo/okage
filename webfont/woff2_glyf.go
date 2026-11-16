package webfont

import (
	"encoding/binary"
	"math"
)

// The flags of a simple glyph of the table glyf.
const (
	glyfOnCurve       = 0x01
	glyfXShort        = 0x02
	glyfYShort        = 0x04
	glyfRepeat        = 0x08
	glyfXSameOrPos    = 0x10
	glyfYSameOrPos    = 0x20
	glyfOverlapSimple = 0x40
)

// The flags of a component of a composite glyph of the table glyf.
const (
	compArgsAreWords     = 0x0001
	compHaveScale        = 0x0008
	compMoreComponents   = 0x0020
	compHaveXYScale      = 0x0040
	compHaveTwoByTwo     = 0x0080
	compHaveInstructions = 0x0100
)

// glyfPoint is a point of a simple glyph, in absolute coordinates.
type glyfPoint struct {
	x, y    int32
	onCurve bool
}

// reconstructGlyf reverses the transform of glyf of section 5 of WOFF2. It
// returns the tables glyf and loca, and the xMin of each glyph.
func reconstructGlyf(data []byte) ([]byte, []byte, []int16, error) {
	r := &woff2Reader{b: data}
	r.u16() // reserved
	optionFlags := r.u16()
	numGlyphs := int(r.u16())
	indexFormat := r.u16()
	var sizes [7]uint32
	for i := range sizes {
		sizes[i] = r.u32()
	}
	streams := make([]woff2Reader, len(sizes))
	for i, n := range sizes {
		streams[i] = woff2Reader{b: r.bytes(int(n))}
	}
	if r.err != nil {
		return nil, nil, nil, r.err
	}
	nContours, nPoints, flags, glyphs, composites, bboxes, instructions :=
		&streams[0], &streams[1], &streams[2], &streams[3], &streams[4], &streams[5], &streams[6]
	if indexFormat > 1 {
		return nil, nil, nil, errorf("woff2", "the table glyf has the index format %d", indexFormat)
	}
	var overlap []byte
	if optionFlags&1 != 0 {
		overlap = r.bytes((numGlyphs + 7) / 8)
		if r.err != nil {
			return nil, nil, nil, r.err
		}
	}
	bboxBitmap := bboxes.bytes(4 * ((numGlyphs + 31) / 32))
	if bboxes.err != nil {
		return nil, nil, nil, bboxes.err
	}
	hasBit := func(bitmap []byte, i int) bool {
		return bitmap != nil && bitmap[i>>3]&(0x80>>(i&7)) != 0
	}

	var glyf []byte
	offsets := make([]int, numGlyphs+1)
	xMins := make([]int16, numGlyphs)
	var points []glyfPoint
	var endPoints []uint16
	for i := range numGlyphs {
		offsets[i] = len(glyf)
		n := int16(nContours.u16())
		explicitBBox := hasBit(bboxBitmap, i)
		var bbox [4]int16
		if explicitBBox {
			for j := range bbox {
				bbox[j] = int16(bboxes.u16())
			}
		}
		switch {
		case n == 0:
			if explicitBBox {
				return nil, nil, nil, errorf("woff2", "the empty glyph %d has a bounding box", i)
			}
		case n < 0:
			if n != -1 {
				return nil, nil, nil, errorf("woff2", "glyph %d has %d contours", i, n)
			}
			if !explicitBBox {
				return nil, nil, nil, errorf("woff2", "the composite glyph %d has no bounding box", i)
			}
			start := composites.b
			haveInstructions := false
			for {
				f := composites.u16()
				composites.u16() // glyphIndex
				size := 2
				if f&compArgsAreWords != 0 {
					size = 4
				}
				switch {
				case f&compHaveScale != 0:
					size += 2
				case f&compHaveXYScale != 0:
					size += 4
				case f&compHaveTwoByTwo != 0:
					size += 8
				}
				composites.bytes(size)
				haveInstructions = haveInstructions || f&compHaveInstructions != 0
				if composites.err != nil || f&compMoreComponents == 0 {
					break
				}
			}
			if composites.err != nil {
				return nil, nil, nil, composites.err
			}
			glyf = appendGlyphHeader(glyf, -1, bbox)
			glyf = append(glyf, start[:len(start)-len(composites.b)]...)
			if haveInstructions {
				size := glyphs.u255()
				glyf = binary.BigEndian.AppendUint16(glyf, size)
				glyf = append(glyf, instructions.bytes(int(size))...)
				if glyphs.err != nil || instructions.err != nil {
					return nil, nil, nil, errorf("woff2", "the instructions of glyph %d are cut short", i)
				}
			}
			xMins[i] = bbox[0]
		default:
			endPoints = endPoints[:0]
			total := 0
			for range n {
				total += int(nPoints.u255())
				if total > math.MaxUint16 {
					return nil, nil, nil, errorf("woff2", "glyph %d has too many points", i)
				}
				endPoints = append(endPoints, uint16(total-1))
			}
			if nPoints.err != nil {
				return nil, nil, nil, nPoints.err
			}
			flagBytes := flags.bytes(total)
			if flags.err != nil {
				return nil, nil, nil, flags.err
			}
			points = points[:0]
			var x, y int32
			for _, flag := range flagBytes {
				dx, dy, ok := decodeTriplet(flag, glyphs)
				if !ok {
					return nil, nil, nil, errorf("woff2", "the coordinates of glyph %d are cut short", i)
				}
				x += dx
				y += dy
				points = append(points, glyfPoint{x: x, y: y, onCurve: flag&0x80 == 0})
			}
			size := glyphs.u255()
			instr := instructions.bytes(int(size))
			if glyphs.err != nil || instructions.err != nil {
				return nil, nil, nil, errorf("woff2", "the instructions of glyph %d are cut short", i)
			}
			if !explicitBBox {
				bbox = pointsBBox(points)
			}
			glyf = appendGlyphHeader(glyf, n, bbox)
			for _, e := range endPoints {
				glyf = binary.BigEndian.AppendUint16(glyf, e)
			}
			glyf = binary.BigEndian.AppendUint16(glyf, size)
			glyf = append(glyf, instr...)
			glyf = appendGlyphPoints(glyf, points, hasBit(overlap, i))
			xMins[i] = bbox[0]
		}
		if nContours.err != nil || bboxes.err != nil {
			return nil, nil, nil, errorf("woff2", "the table glyf is cut short at glyph %d", i)
		}
		// Each glyph starts at a multiple of 4.
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
	}
	offsets[numGlyphs] = len(glyf)

	var loca []byte
	if indexFormat == 0 {
		if len(glyf)/2 > math.MaxUint16 {
			return nil, nil, nil, errorf("woff2", "the table glyf is too large for a short loca")
		}
		loca = make([]byte, 0, 2*(numGlyphs+1))
		for _, o := range offsets {
			loca = binary.BigEndian.AppendUint16(loca, uint16(o/2))
		}
	} else {
		loca = make([]byte, 0, 4*(numGlyphs+1))
		for _, o := range offsets {
			loca = binary.BigEndian.AppendUint32(loca, uint32(o))
		}
	}
	return glyf, loca, xMins, nil
}

// decodeTriplet reads the coordinates of a point from the glyph stream, for
// its flag byte, as section 5.2 of WOFF2 gives. It returns the change of x
// and of y from the last point.
func decodeTriplet(flag byte, glyphs *woff2Reader) (int32, int32, bool) {
	withSign := func(flag byte, v int32) int32 {
		if flag&1 != 0 {
			return v
		}
		return -v
	}
	f := flag & 0x7f
	var n int
	switch {
	case f < 84:
		n = 1
	case f < 120:
		n = 2
	case f < 124:
		n = 3
	default:
		n = 4
	}
	b := glyphs.bytes(n)
	if b == nil {
		return 0, 0, false
	}
	var dx, dy int32
	switch {
	case f < 10:
		dy = withSign(f, int32(f&14)<<7+int32(b[0]))
	case f < 20:
		dx = withSign(f, int32((f-10)&14)<<7+int32(b[0]))
	case f < 84:
		b0 := int32(f - 20)
		b1 := int32(b[0])
		dx = withSign(f, 1+(b0&0x30)+b1>>4)
		dy = withSign(f>>1, 1+(b0&0x0c)<<2+b1&0x0f)
	case f < 120:
		b0 := int32(f - 84)
		dx = withSign(f, 1+(b0/12)<<8+int32(b[0]))
		dy = withSign(f>>1, 1+((b0%12)>>2)<<8+int32(b[1]))
	case f < 124:
		b2 := int32(b[1])
		dx = withSign(f, int32(b[0])<<4+b2>>4)
		dy = withSign(f>>1, (b2&0x0f)<<8+int32(b[2]))
	default:
		dx = withSign(f, int32(b[0])<<8+int32(b[1]))
		dy = withSign(f>>1, int32(b[2])<<8+int32(b[3]))
	}
	return dx, dy, true
}

// pointsBBox returns the bounding box of points, as xMin, yMin, xMax and yMax.
func pointsBBox(points []glyfPoint) [4]int16 {
	if len(points) == 0 {
		return [4]int16{}
	}
	xMin, yMin, xMax, yMax := points[0].x, points[0].y, points[0].x, points[0].y
	for _, p := range points[1:] {
		xMin, xMax = min(xMin, p.x), max(xMax, p.x)
		yMin, yMax = min(yMin, p.y), max(yMax, p.y)
	}
	return [4]int16{int16(xMin), int16(yMin), int16(xMax), int16(yMax)}
}

// appendGlyphHeader appends the header of a glyph: its number of contours
// and its bounding box.
func appendGlyphHeader(b []byte, n int16, bbox [4]int16) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(n))
	for _, v := range bbox {
		b = binary.BigEndian.AppendUint16(b, uint16(v))
	}
	return b
}

// appendGlyphPoints appends the flags and the coordinates of the points of a
// simple glyph. It writes a coordinate in one byte when it can, and repeats
// a flag that is the same as the one before. overlap sets the flag
// OVERLAP_SIMPLE on the first point.
func appendGlyphPoints(b []byte, points []glyfPoint, overlap bool) []byte {
	flags := make([]byte, len(points))
	var xs, ys []byte
	var lastX, lastY int32
	for i, p := range points {
		var flag byte
		if p.onCurve {
			flag |= glyfOnCurve
		}
		if i == 0 && overlap {
			flag |= glyfOverlapSimple
		}
		dx, dy := p.x-lastX, p.y-lastY
		lastX, lastY = p.x, p.y
		switch {
		case dx == 0:
			flag |= glyfXSameOrPos
		case dx >= -255 && dx <= 255:
			flag |= glyfXShort
			if dx > 0 {
				flag |= glyfXSameOrPos
			}
			xs = append(xs, byte(abs(dx)))
		default:
			xs = binary.BigEndian.AppendUint16(xs, uint16(int16(dx)))
		}
		switch {
		case dy == 0:
			flag |= glyfYSameOrPos
		case dy >= -255 && dy <= 255:
			flag |= glyfYShort
			if dy > 0 {
				flag |= glyfYSameOrPos
			}
			ys = append(ys, byte(abs(dy)))
		default:
			ys = binary.BigEndian.AppendUint16(ys, uint16(int16(dy)))
		}
		flags[i] = flag
	}
	for i := 0; i < len(flags); {
		j := i + 1
		for j < len(flags) && flags[j] == flags[i] && j-i <= 255 {
			j++
		}
		if repeat := j - i - 1; repeat > 0 {
			b = append(b, flags[i]|glyfRepeat, byte(repeat))
		} else {
			b = append(b, flags[i])
		}
		i = j
	}
	b = append(b, xs...)
	return append(b, ys...)
}

// abs returns the absolute value of v.
func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
