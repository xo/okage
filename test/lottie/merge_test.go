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
// across both, and an ellipse at the left.
var mergeShapes = []string{
	`{"ty": "rc", "p": {"a": 0, "k": [150, 130]}, "s": {"a": 0, "k": [140, 140]}, "r": {"a": 0, "k": 0}}`,
	`{"ty": "el", "p": {"a": 0, "k": [230, 200]}, "s": {"a": 0, "k": [150, 150]}}`,
	`{"ty": "rc", "p": {"a": 0, "k": [200, 220]}, "s": {"a": 0, "k": [260, 50]}, "r": {"a": 0, "k": 15}}`,
	`{"ty": "el", "p": {"a": 0, "k": [110, 170]}, "s": {"a": 0, "k": [90, 200]}}`,
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
// file, from the start point and in the direction that pathbool gives them.
// okage draws the file with the merge path. The trim path takes 10 to 70
// percent of the outline. okage trims the result as one path, as velato
// trims a path with several contours, and ThorVG trims each path of the
// file. So the test compares a trim only for a result with one contour.
func TestThorVGMergeStroke(t *testing.T) {
	trim := `{"ty": "tm", "s": {"a": 0, "k": 10}, "e": {"a": 0, "k": 70}, "o": {"a": 0, "k": 0}, "m": 1}`
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
			if len(bezierJSON(result)) != 1 {
				continue
			}
			trimmed := slicesConcat(items, []string{merge, trim, mergeStroke})
			cases = append(cases, compatCase{
				name:       fmt.Sprintf("merge-trim-%s-%d", mc.name, mode),
				doc:        mergeDocument(mergeLayer(trimmed...)),
				thorvg:     mergeDocument(mergeLayer(append(pathItems(result), trim, mergeStroke)...)),
				mergePaths: true,
			})
		}
	}
	compareCases(t, cases)
}

// slicesConcat returns the items of a followed by the items of b.
func slicesConcat(a, b []string) []string {
	return append(append([]string(nil), a...), b...)
}
