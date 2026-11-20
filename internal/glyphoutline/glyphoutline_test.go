package glyphoutline

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// recordPen is a Pen that records each command as text.
type recordPen struct {
	cmds []string
}

func (p *recordPen) MoveTo(x, y float32) { p.cmds = append(p.cmds, fmt.Sprintf("M%v,%v", x, y)) }
func (p *recordPen) LineTo(x, y float32) { p.cmds = append(p.cmds, fmt.Sprintf("L%v,%v", x, y)) }
func (p *recordPen) QuadTo(cx0, cy0, x, y float32) {
	p.cmds = append(p.cmds, fmt.Sprintf("Q%v,%v,%v,%v", cx0, cy0, x, y))
}

func (p *recordPen) CurveTo(cx0, cy0, cx1, cy1, x, y float32) {
	p.cmds = append(p.cmds, fmt.Sprintf("C%v,%v,%v,%v,%v,%v", cx0, cy0, cx1, cy1, x, y))
}
func (p *recordPen) Close() { p.cmds = append(p.cmds, "Z") }

// TestNopFilterPen checks the commands that NopFilterSink drops.
func TestNopFilterPen(t *testing.T) {
	tests := []struct {
		name string
		draw func(n *nopFilterPen)
		want string
	}{
		{
			"a move that a close follows",
			func(n *nopFilterPen) { n.moveTo(1, 2); n.close() },
			"",
		},
		{
			"a line to the current point",
			func(n *nopFilterPen) { n.moveTo(0, 0); n.lineTo(0, 0); n.lineTo(5, 0); n.lineTo(5, 0); n.close() },
			"M0,0 L5,0 Z",
		},
		{
			"a closing line to the start",
			func(n *nopFilterPen) { n.moveTo(0, 0); n.lineTo(5, 0); n.lineTo(5, 5); n.lineTo(0, 0); n.close() },
			"M0,0 L5,0 L5,5 Z",
		},
		{
			"a curve and a second contour",
			func(n *nopFilterPen) {
				n.moveTo(0, 0)
				n.curveTo(1, 1, 2, 2, 3, 0)
				n.close()
				n.moveTo(9, 9)
				n.lineTo(8, 8)
				n.close()
			},
			"M0,0 C1,1,2,2,3,0 Z M9,9 L8,8 Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p recordPen
			tt.draw(&nopFilterPen{pen: &p})
			if got := strings.Join(p.cmds, " "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// seg makes a segment of typesetting.
func seg(op ot.SegmentOp, xy ...float32) font.Segment {
	s := font.Segment{Op: op}
	for i := 0; i+1 < len(xy); i += 2 {
		s.Args[i/2] = font.SegmentPoint{X: xy[i], Y: xy[i+1]}
	}
	return s
}

// TestSkrifaSegments checks the start of each kind of contour.
func TestSkrifaSegments(t *testing.T) {
	move, line, quad := ot.SegmentOpMoveTo, ot.SegmentOpLineTo, ot.SegmentOpQuadTo
	tests := []struct {
		name  string
		in    []font.Segment
		flags [][2]bool
		want  []font.Segment
	}{
		{
			"the first and the last points on the curve",
			[]font.Segment{seg(move, 0, 0), seg(line, 5, 0), seg(line, 5, 5), seg(line, 0, 0)},
			[][2]bool{{true, true}},
			[]font.Segment{seg(move, 0, 0), seg(line, 5, 0), seg(line, 5, 5)},
		},
		{
			"the first point off the curve and the last on it",
			// typesetting starts at the point after the first, and ends
			// with a line back to that start.
			[]font.Segment{seg(move, 5, 0), seg(line, 5, 5), seg(line, 0, 5), seg(quad, 0, 0, 5, 0)},
			[][2]bool{{false, true}},
			[]font.Segment{seg(move, 0, 5), seg(quad, 0, 0, 5, 0), seg(line, 5, 5)},
		},
		{
			"contours that do not match the flags",
			[]font.Segment{seg(move, 0, 0), seg(line, 5, 0)},
			[][2]bool{{true, true}, {true, true}},
			[]font.Segment{seg(move, 0, 0), seg(line, 5, 0)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skrifaSegments(tt.in, tt.flags); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
