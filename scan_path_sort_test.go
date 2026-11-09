package okage

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"
)

// TestSortEdges checks that sortEdges gives the order of the stable sort of
// fill_path_impl in tiny-skia src/scan/path.rs, on random edges with many
// equal keys.
func TestSortEdges(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for n := range 2000 {
		count := r.IntN(300)
		edges := make([]edge, count)
		for i := range edges {
			edges[i] = edge{kind: edgeKindLine, line: lineEdge{
				firstY:  int32(r.IntN(8)),
				x:       int32(r.IntN(4)),
				lastY:   int32(i),
				winding: int8(r.IntN(3) - 1),
			}}
		}
		want := slices.Clone(edges)
		slices.SortStableFunc(want, func(a, b edge) int {
			valueA := a.asLine().firstY
			valueB := b.asLine().firstY
			if valueA == valueB {
				valueA = a.asLine().x
				valueB = b.asLine().x
			}
			return cmp.Compare(valueA, valueB)
		})
		got := sortEdges(slices.Clone(edges))
		if !slices.Equal(got, want) {
			t.Fatalf("case %d: the order differs", n)
		}
	}
}
