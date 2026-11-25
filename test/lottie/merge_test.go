package lottie

import (
	"fmt"
	"strings"
	"testing"

	"github.com/xo/okage/internal/pathbool"
	"github.com/xo/okage/kurbo"
	okagelottie "github.com/xo/okage/lottie"
)

// mergeTransform is the transform item of a group that does not move it.
const mergeTransform = `{"ty": "tr", "a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}`

// mergeShapes are the operands of the tests of the merge paths, as shape
// items: a square, a circle that overlaps its corner, a rounded rectangle
// across both, an ellipse at the left, a bar across the square, a circle
// inside the square, a path in the shape of a U, and a bar across its arms.
var mergeShapes = []string{
	`{"ty": "rc", "p": {"a": 0, "k": [150, 130]}, "s": {"a": 0, "k": [140, 140]}, "r": {"a": 0, "k": 0}}`,
	`{"ty": "el", "p": {"a": 0, "k": [230, 200]}, "s": {"a": 0, "k": [150, 150]}}`,
	`{"ty": "rc", "p": {"a": 0, "k": [200, 220]}, "s": {"a": 0, "k": [260, 50]}, "r": {"a": 0, "k": 15}}`,
	`{"ty": "el", "p": {"a": 0, "k": [110, 170]}, "s": {"a": 0, "k": [90, 200]}}`,
	`{"ty": "rc", "p": {"a": 0, "k": [150, 130]}, "s": {"a": 0, "k": [200, 30]}, "r": {"a": 0, "k": 0}}`,
	`{"ty": "el", "p": {"a": 0, "k": [150, 130]}, "s": {"a": 0, "k": [60, 60]}}`,
	`{"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[60, 210], [120, 210], [120, 250], [180, 250], [180, 210], [240, 210], [240, 280], [60, 280]], "i": [[0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0]], "o": [[0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0], [0, 0]]}}}`,
	`{"ty": "rc", "p": {"a": 0, "k": [150, 230]}, "s": {"a": 0, "k": [240, 20]}, "r": {"a": 0, "k": 0}}`,
}

// mergeLayer returns a shape layer with one group of the shape items items.
func mergeLayer(items ...string) string {
	return fmt.Sprintf(`{
		"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0,
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
		"shapes": [{"ty": "gr", "it": [%s, %s]}]
	}`, strings.Join(items, ", "), mergeTransform)
}

// mergeDocument returns a document with the layer layer.
func mergeDocument(layer string) []byte {
	return textDocument(`{"list": []}`, "[]", layer)
}

// mergeOperands returns the paths of the geometries of the group of the
// document doc, as okage evaluates them at the frame 0.
func mergeOperands(t *testing.T, doc []byte) [][]kurbo.PathEl {
	t.Helper()
	c, err := okagelottie.Parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	var operands [][]kurbo.PathEl
	for _, s := range c.Layers[0].Content.Shapes[0].Group {
		if s.Kind == okagelottie.ShapeGeometry {
			operands = append(operands, s.Geometry.Evaluate(0, nil))
		}
	}
	return operands
}

// bezierJSON returns the value of each contour of path as a Lottie shape
// value, with its vertices and the tangents of its curves.
func bezierJSON(path []kurbo.PathEl) []string {
	var out []string
	var v, in, outT []kurbo.Point
	flush := func() {
		if len(v) > 1 && v[len(v)-1] == v[0] {
			in[0] = in[len(in)-1]
			v, in, outT = v[:len(v)-1], in[:len(in)-1], outT[:len(outT)-1]
		}
		if len(v) > 0 {
			list := func(ps []kurbo.Point) string {
				var parts []string
				for _, p := range ps {
					parts = append(parts, fmt.Sprintf("[%.6f, %.6f]", p.X, p.Y))
				}
				return "[" + strings.Join(parts, ", ") + "]"
			}
			out = append(out, fmt.Sprintf(`{"c": true, "v": %s, "i": %s, "o": %s}`, list(v), list(in), list(outT)))
		}
		v, in, outT = nil, nil, nil
	}
	for _, el := range path {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			flush()
			v, in, outT = append(v, el.P[0]), append(in, kurbo.Point{}), append(outT, kurbo.Point{})
		case kurbo.PathElLineTo:
			v, in, outT = append(v, el.P[0]), append(in, kurbo.Point{}), append(outT, kurbo.Point{})
		case kurbo.PathElCurveTo:
			last := v[len(v)-1]
			outT[len(outT)-1] = kurbo.Point(el.P[0].Sub(last))
			v, in, outT = append(v, el.P[2]), append(in, kurbo.Point(el.P[1].Sub(el.P[2]))), append(outT, kurbo.Point{})
		case kurbo.PathElClosePath:
			flush()
		}
	}
	flush()
	return out
}

// pathItems returns a shape item of a path for each contour of path.
func pathItems(path []kurbo.PathEl) []string {
	var items []string
	for _, b := range bezierJSON(path) {
		items = append(items, fmt.Sprintf(`{"ty": "sh", "ks": {"a": 0, "k": %s}}`, b))
	}
	return items
}

// mergeOp returns the operation of pathbool for the merge mode mode.
func mergeOp(mode int) pathbool.Op {
	return [...]pathbool.Op{2: pathbool.Union, 3: pathbool.Difference, 4: pathbool.Intersection, 5: pathbool.Xor}[mode]
}

// mergeTolerance is the tolerance of the result of a merge path of okage at
// the scale 1 (D94).
const mergeTolerance = 0.1

const (
	mergeFill   = `{"ty": "fl", "c": {"a": 0, "k": [0.9, 0.3, 0.1, 1]}, "o": {"a": 0, "k": 100}, "r": 1}`
	mergeStroke = `{"ty": "st", "c": {"a": 0, "k": [0.1, 0.2, 0.6, 1]}, "o": {"a": 0, "k": 100}, "w": {"a": 0, "k": 6}, "lc": 2, "lj": 2}`
)

// mergeCases returns the sets of operands of the tests of the merge paths,
// by the indices of mergeShapes, with their names.
func mergeCases() []struct {
	name    string
	shapes  []int
	animate bool
} {
	return []struct {
		name    string
		shapes  []int
		animate bool
	}{
		{"square-circle", []int{0, 1}, false},
		{"three", []int{0, 1, 2}, false},
		{"ellipses", []int{1, 3}, false},
		{"four", []int{3, 0, 2, 1}, false},
		{"bar", []int{0, 4}, false},
		{"hole", []int{0, 5}, false},
		{"u", []int{6, 7}, false},
	}
}

// TestThorVGMergeArea compares the area of the geometric boolean operation
// of the merge paths of modes 2 to 5 with ThorVG (D94). ThorVG draws a merge
// path as a compound path (D43), so it cannot give the result itself. It
// draws the masks of Lottie by compositing, and masks with the modes add,
// subtract and intersect combine as the merge modes 2 to 4. So for these
// modes, ThorVG draws a filled layer with one mask for each operand. For the
// mode 5, ThorVG fills all operands as one path with the even-odd rule,
// which covers the area that an odd number of operands covers. Masks with
// the mode difference do not give that area for three operands or more in
// ThorVG. okage fills the contours of the result of pathbool as paths of
// the file.
func TestThorVGMergeArea(t *testing.T) {
	maskModes := map[int]string{2: "a", 3: "s", 4: "i"}
	var cases []compatCase
	for _, mc := range mergeCases() {
		var items []string
		for _, i := range mc.shapes {
			items = append(items, mergeShapes[i])
		}
		operands := mergeOperands(t, mergeDocument(mergeLayer(items...)))
		for mode := 2; mode <= 5; mode++ {
			result := pathbool.Combine(mergeOp(mode), operands, mergeTolerance, nil)
			okageDoc := mergeDocument(mergeLayer(append(pathItems(result), mergeFill)...))
			if mode == 5 {
				var all []kurbo.PathEl
				for _, operand := range operands {
					all = append(all, operand...)
				}
				evenOdd := strings.Replace(mergeFill, `"r": 1`, `"r": 2`, 1)
				cases = append(cases, compatCase{name: fmt.Sprintf("merge-area-%s-%d", mc.name, mode), doc: okageDoc,
					thorvg: mergeDocument(mergeLayer(append(pathItems(all), evenOdd)...))})
				continue
			}
			var masks []string
			for i, operand := range operands {
				m := maskModes[mode]
				if i == 0 {
					m = "a"
				}
				masks = append(masks, fmt.Sprintf(`{"mode": %q, "inv": false, "pt": {"a": 0, "k": %s}, "o": {"a": 0, "k": 100}, "x": {"a": 0, "k": 0}}`, m, bezierJSON(operand)[0]))
			}
			masked := fmt.Sprintf(`{
				"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "hasMask": true, "masksProperties": [%s],
				"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
				"shapes": [{"ty": "rc", "p": {"a": 0, "k": [200, 150]}, "s": {"a": 0, "k": [400, 300]}, "r": {"a": 0, "k": 0}}, %s]
			}`, strings.Join(masks, ", "), mergeFill)
			cases = append(cases, compatCase{name: fmt.Sprintf("merge-area-%s-%d", mc.name, mode), doc: okageDoc, thorvg: mergeDocument(masked)})
		}
	}
	compareCases(t, cases)
}

// TestThorVGMergeStroke compares a stroke and a trim path after the merge
// paths of modes 2 to 5 with Renderer.MergePaths with ThorVG (D94). ThorVG
// strokes and trims the contours of the result of pathbool as paths of the
// file, one path for each contour, in the order, from the start point and
// in the direction that pathbool gives them. okage draws the file with the
// merge path. The trim path takes 10 to 70 percent of the outline, once
// with the mode simultaneously and once with the mode individually. In the
// mode simultaneously, okage trims each contour of the result on its own,
// as velato trims each contour of a path, and ThorVG trims each path of the
// file on its own. In the mode individually, both trim the contours as one
// outline, each contour as a path of its own, in the order of ThorVG that
// TestThorVGTrimIndividually tests (D43). So the test pins the order, the
// start points and the directions of the contours of a result with several
// contours.
func TestThorVGMergeStroke(t *testing.T) {
	trims := []struct {
		name string
		item string
	}{
		{"trim", `{"ty": "tm", "s": {"a": 0, "k": 10}, "e": {"a": 0, "k": 70}, "o": {"a": 0, "k": 0}, "m": 1}`},
		{"trim-individually", `{"ty": "tm", "s": {"a": 0, "k": 10}, "e": {"a": 0, "k": 70}, "o": {"a": 0, "k": 0}, "m": 2}`},
	}
	var cases []compatCase
	for _, mc := range mergeCases() {
		var items []string
		for _, i := range mc.shapes {
			items = append(items, mergeShapes[i])
		}
		operands := mergeOperands(t, mergeDocument(mergeLayer(items...)))
		for mode := 2; mode <= 5; mode++ {
			result := pathbool.Combine(mergeOp(mode), operands, mergeTolerance, nil)
			merge := fmt.Sprintf(`{"ty": "mm", "mm": %d}`, mode)
			stroke := slicesConcat(items, []string{merge, mergeStroke})
			cases = append(cases, compatCase{
				name:       fmt.Sprintf("merge-stroke-%s-%d", mc.name, mode),
				doc:        mergeDocument(mergeLayer(stroke...)),
				thorvg:     mergeDocument(mergeLayer(append(pathItems(result), mergeStroke)...)),
				mergePaths: true,
			})
			for _, trim := range trims {
				trimmed := slicesConcat(items, []string{merge, trim.item, mergeStroke})
				cases = append(cases, compatCase{
					name:       fmt.Sprintf("merge-%s-%s-%d-contours-%d", trim.name, mc.name, mode, len(bezierJSON(result))),
					doc:        mergeDocument(mergeLayer(trimmed...)),
					thorvg:     mergeDocument(mergeLayer(append(pathItems(result), trim.item, mergeStroke)...)),
					mergePaths: true,
				})
			}
		}
	}
	compareCases(t, cases)
}

// trimSquare returns a shape item of the square i of a row of squares of 50
// pixels.
func trimSquare(i int) string {
	return fmt.Sprintf(`{"ty": "rc", "p": {"a": 0, "k": [%d, 60]}, "s": {"a": 0, "k": [50, 50]}, "r": {"a": 0, "k": 0}}`, 50+70*i)
}

// trimGroup returns a shape item of a group of the shape items items.
func trimGroup(items ...string) string {
	return fmt.Sprintf(`{"ty": "gr", "it": [%s]}`, strings.Join(slicesConcat(items, []string{mergeTransform}), ", "))
}

// TestThorVGTrimIndividually compares the order in which a trim path with
// the mode individually trims the paths of its group with ThorVG (D43).
// ThorVG trims the paths of the first item that holds a path backwards, and
// then the paths of the other items backwards. That item is a path or a
// group with a path. Each file has five squares of the same length, in
// groups or not, and the trim path takes 0 to 20.5, 40.5, 60.5 and 80.5
// percent of the outline, so each image shows which squares come first.
// velato trims the paths in their order. The outline of a rectangle of
// ThorVG starts at another point than the one of velato, so the start of
// the next square differs in a few pixels.
func TestThorVGTrimIndividually(t *testing.T) {
	sq := trimSquare
	layouts := []struct {
		name  string
		items []string
	}{
		{"flat", []string{sq(0), sq(1), sq(2), sq(3), sq(4)}},
		{"group-second", []string{sq(0), trimGroup(sq(1), sq(2)), sq(3), sq(4)}},
		{"groups", []string{trimGroup(sq(0), sq(1)), trimGroup(sq(2), sq(3)), sq(4)}},
		{"group-first", []string{trimGroup(sq(0), sq(1), sq(2)), sq(3), sq(4)}},
		{"group-nested", []string{trimGroup(sq(0), trimGroup(sq(1), sq(2))), sq(3), sq(4)}},
		{"group-no-path", []string{trimGroup(mergeFill), sq(0), sq(1), sq(2), sq(3), sq(4)}},
		{"fill-first", []string{mergeFill, sq(0), sq(1), sq(2), sq(3), sq(4)}},
	}
	var cases []compatCase
	for _, l := range layouts {
		for _, end := range []string{"20.5", "40.5", "60.5", "80.5"} {
			trim := `{"ty": "tm", "s": {"a": 0, "k": 0}, "e": {"a": 0, "k": ` + end + `}, "o": {"a": 0, "k": 0}, "m": 2}`
			cases = append(cases, compatCase{
				name: fmt.Sprintf("trim-individually-%s-%s", l.name, end),
				doc:  mergeDocument(mergeLayer(slicesConcat(l.items, []string{trim, mergeStroke})...)),
			})
		}
	}
	compareCases(t, cases)
}

// slicesConcat returns the items of a followed by the items of b.
func slicesConcat(a, b []string) []string {
	return append(append([]string(nil), a...), b...)
}
