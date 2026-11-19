// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// This file ports no file of krilla. It holds the stitching functions of a
// gradient with many stops, which the device of ivy needs and krilla does not
// have (D69).

// maxStitchedFunctions is the largest number of functions that pdf writes in
// one stitching function. MuPDF 1.28 reads at most 256 functions of a
// stitching function, and draws nothing for a shading with more.
const maxStitchedFunctions = 256

// nestStitching puts the functions of a stitching function into a tree of
// stitching functions, with at most maxStitchedFunctions functions in each.
// The function i of functions draws the interval from the offset of stops[i]
// to the offset of stops[i+1], with the encode values encode[2*i] and
// encode[2*i+1]. count is the number of components of the colors.
//
// Each stitching function of the tree has the domain of its first and its
// last offset, and its parent encodes its interval to that same domain. So
// each leaf gets the same input as in a single stitching function, and the
// tree draws the same gradient. nestStitching returns the functions, the
// bounds and the encode values of the root of the tree.
func nestStitching(stops []Stop, functions []pdfwriter.Ref, encode []float32, count int, chunk *pdfwriter.Chunk, sc *serializeContext) ([]pdfwriter.Ref, []float32, []float32) {
	// The offsets of the ends of the intervals of the functions.
	edges := make([]float32, len(functions)+1)
	for i := range edges {
		edges[i] = stops[i].Offset.Get()
	}
	for len(functions) > maxStitchedFunctions {
		n := len(functions)
		groups := (n + maxStitchedFunctions - 1) / maxStitchedFunctions
		nextFunctions := make([]pdfwriter.Ref, 0, groups)
		nextEdges := make([]float32, 0, groups+1)
		nextEncode := make([]float32, 0, 2*groups)
		for start := 0; start < n; start += maxStitchedFunctions {
			end := min(start+maxStitchedFunctions, n)
			lo, hi := edges[start], edges[end]
			ref := sc.newRef()
			f := chunk.StitchingFunction(ref)
			f.Domain(lo, hi)
			f.Range(repeatUnitRange(count)...)
			f.Functions(functions[start:end]...)
			f.Bounds(edges[start+1 : end]...)
			f.Encode(encode[2*start : 2*end]...)
			f.Finish()
			nextFunctions = append(nextFunctions, ref)
			nextEdges = append(nextEdges, lo)
			nextEncode = append(nextEncode, lo, hi)
		}
		nextEdges = append(nextEdges, edges[n])
		functions, edges, encode = nextFunctions, nextEdges, nextEncode
	}
	return functions, edges[1 : len(edges)-1], encode
}
