// Copyright 2026 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/evaluate.rs.

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/lottiecompat"
)

// OccurrencePath is OccurrencePath in velato src/runtime/evaluate.rs:
// nested layer indices, not Lottie IDs.
type OccurrencePath []int

// EvaluationErrorKind is the variant of EvaluationError in velato
// src/runtime/evaluate.rs.
type EvaluationErrorKind uint8

// The variants of EvaluationError.
const (
	EvalNonFiniteTime EvaluationErrorKind = iota
	EvalInvalidStretch
	EvalInvalidTransform
	EvalInvalidParent
	EvalUnresolvedParent
	EvalParentCycle
	EvalInvalidMatte
	EvalUnresolvedMatte
	EvalMatteCycle
	EvalMissingAsset
	EvalPrecompCycle
)

// EvaluationError is EvaluationError in velato src/runtime/evaluate.rs. It
// wraps okage.ErrLottieEvaluation.
type EvaluationError struct {
	Kind EvaluationErrorKind
	// Path of the layer, for the kinds about a layer.
	Path OccurrencePath
	// ID is the authored ind of an unresolved parent or matte.
	ID int
	// Name is the asset of MissingAsset and PrecompCycle.
	Name string
}

// Error is impl Display for EvaluationError in velato
// src/runtime/evaluate.rs.
func (e *EvaluationError) Error() string {
	switch e.Kind {
	case EvalUnresolvedParent:
		return fmt.Sprintf("%s: no parent layer with ind %d in the composition containing %v", okage.ErrLottieEvaluation, e.ID, []int(e.Path))
	case EvalUnresolvedMatte:
		return fmt.Sprintf("%s: no matte layer with ind %d in the composition containing %v", okage.ErrLottieEvaluation, e.ID, []int(e.Path))
	case EvalMissingAsset:
		return fmt.Sprintf("%s: missing asset %q", okage.ErrLottieEvaluation, e.Name)
	case EvalPrecompCycle:
		return fmt.Sprintf("%s: precomposition cycle at %q", okage.ErrLottieEvaluation, e.Name)
	}
	names := [...]string{"non-finite time", "invalid stretch", "invalid transform", "invalid parent", "", "parent cycle", "invalid matte", "", "matte cycle"}
	return fmt.Sprintf("%s: %s at %v", okage.ErrLottieEvaluation, names[e.Kind], []int(e.Path))
}

// Unwrap returns okage.ErrLottieEvaluation.
func (e *EvaluationError) Unwrap() error {
	return okage.ErrLottieEvaluation
}

// EvaluatedLayer is EvaluatedLayer in velato src/runtime/evaluate.rs: one
// layer instance at a frame. Coordinates are pixels, with y down.
type EvaluatedLayer struct {
	Path  OccurrencePath
	Layer *Layer
	// Transform-parent index in the containing composition, or -1.
	Parent int
	// Matte mode and source index in the containing composition, or nil.
	Matte *evaluatedMatte
	// Frame in the containing composition.
	Frame float64
	// Frame is at or after the layer's in-point and before its out-point.
	InRange bool
	// Active in the normal render tree.
	Visible bool
	// The authored components, and false for a matrix-only transform.
	AuthoredComponents   TransformComponents
	HasAuthoredComponent bool
	// Layer-to-parent transform, or layer-to-composition if parentless.
	LocalTransform kurbo.Affine
	// Places layer coordinates in the main composition.
	FullTransform kurbo.Affine
	// Layer's own opacity, from 0 to 1.
	Opacity float64
	// Layers inside this precomposition instance.
	Children []EvaluatedLayer
}

// evaluatedMatte is the matte of an EvaluatedLayer: its mode and the index
// of its source.
type evaluatedMatte struct {
	Mode   BlendMode
	Source int
}

// EvaluatedPaths is EvaluatedLayer::evaluated_paths in velato
// src/runtime/evaluate.rs: the paths of a shape layer after its transforms,
// trims and repeaters, in pixels of the main animation.
func (l *EvaluatedLayer) EvaluatedPaths() [][]kurbo.PathEl {
	if l.Layer.Content.Kind != ContentShape {
		return nil
	}
	return evaluatePaths(l.Layer.Content.Shapes, l.FullTransform, l.Frame)
}

// EvaluatedComposition is EvaluatedComposition in velato
// src/runtime/evaluate.rs.
type EvaluatedComposition struct {
	composition *Composition
	Layers      []EvaluatedLayer
}

// Get is EvaluatedComposition::get in velato src/runtime/evaluate.rs.
func (c *EvaluatedComposition) Get(path OccurrencePath) *EvaluatedLayer {
	layers := c.Layers
	var result *EvaluatedLayer
	for _, index := range path {
		if index < 0 || index >= len(layers) {
			return nil
		}
		result = &layers[index]
		layers = result.Children
	}
	return result
}

// Evaluate is Composition::evaluate in velato src/runtime/evaluate.rs: the
// layers at a frame, without drawing.
func (c *Composition) Evaluate(frame float64) (*EvaluatedComposition, error) {
	layers, err := evaluateLayers(c, c.Layers, frame, kurbo.IdentityAffine, true, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EvaluatedComposition{composition: c, Layers: layers}, nil
}

// dependencyOrder is dependency_order in velato src/runtime/evaluate.rs. It
// returns the indices in an order where each one comes after its
// dependency, or the index that starts a cycle.
func dependencyOrder(count int, dependency func(int) int) ([]int, int, bool) {
	const (
		unvisited = iota
		visiting
		visited
	)
	states := make([]uint8, count)
	var stack []int
	order := make([]int, 0, count)
	for start := range count {
		current := start
		for current >= 0 {
			if states[current] == visited {
				break
			}
			if states[current] == visiting {
				return nil, current, false
			}
			states[current] = visiting
			stack = append(stack, current)
			current = dependency(current)
		}
		for len(stack) > 0 {
			index := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			states[index] = visited
			order = append(order, index)
		}
	}
	return order, 0, true
}

// isFiniteF64 is f64::is_finite.
func isFiniteF64(x float64) bool {
	return !math.IsInf(x, 0) && x == x
}

// evaluateLayers is evaluate_layers in velato src/runtime/evaluate.rs.
func evaluateLayers(animation *Composition, layers []Layer, frame float64, outer kurbo.Affine, visible bool, prefix []int, assets []string) ([]EvaluatedLayer, error) {
	if !isFiniteF64(frame) {
		return nil, &EvaluationError{Kind: EvalNonFiniteTime}
	}
	result := make([]EvaluatedLayer, 0, len(layers))
	for index := range layers {
		layer := &layers[index]
		path := append(slices.Clone(prefix), index)
		parent := -1
		if layer.Parent != nil {
			if !layer.Parent.Resolved {
				return nil, &EvaluationError{Kind: EvalUnresolvedParent, Path: path, ID: layer.Parent.Index}
			}
			if layer.Parent.Index >= len(layers) {
				return nil, &EvaluationError{Kind: EvalInvalidParent, Path: path}
			}
			parent = layer.Parent.Index
		}
		var matte *evaluatedMatte
		if layer.Matte != nil {
			if !layer.Matte.Source.Resolved {
				return nil, &EvaluationError{Kind: EvalUnresolvedMatte, Path: path, ID: layer.Matte.Source.Index}
			}
			if layer.Matte.Source.Index >= len(layers) {
				return nil, &EvaluationError{Kind: EvalInvalidMatte, Path: path}
			}
			matte = &evaluatedMatte{Mode: layer.Matte.Mode, Source: layer.Matte.Source.Index}
		}
		if layer.Content.Kind == ContentInstance && (!isFiniteF64(layer.Stretch) || layer.Stretch == 0.0) {
			return nil, &EvaluationError{Kind: EvalInvalidStretch, Path: path}
		}
		components, hasComponents := layer.Transform.Components(frame)
		var local kurbo.Affine
		if hasComponents {
			local = components.Matrix()
		} else {
			local = layer.Transform.Evaluate(frame)
		}
		if !local.IsFinite() {
			return nil, &EvaluationError{Kind: EvalInvalidTransform, Path: path}
		}
		inRange := frame >= layer.Frames[0] && frame < layer.Frames[1]
		result = append(result, EvaluatedLayer{
			Path:                 path,
			Layer:                layer,
			Parent:               parent,
			Matte:                matte,
			Frame:                frame,
			InRange:              inRange,
			Visible:              visible && inRange && !layer.Hidden && !layer.IsMatteSource,
			AuthoredComponents:   components,
			HasAuthoredComponent: hasComponents,
			LocalTransform:       local,
			FullTransform:        local,
			Opacity:              layer.Opacity.Evaluate(frame) / 100.0,
		})
	}
	parentOrder, cycle, ok := dependencyOrder(len(result), func(i int) int { return result[i].Parent })
	if !ok {
		return nil, &EvaluationError{Kind: EvalParentCycle, Path: result[cycle].Path}
	}
	for _, index := range parentOrder {
		parentTransform := outer
		if p := result[index].Parent; p >= 0 {
			parentTransform = result[p].FullTransform
		}
		transform := parentTransform.Mul(result[index].LocalTransform)
		if !transform.IsFinite() {
			return nil, &EvaluationError{Kind: EvalInvalidTransform, Path: result[index].Path}
		}
		result[index].FullTransform = transform
	}
	if _, cycle, ok := dependencyOrder(len(result), func(i int) int {
		if m := result[i].Matte; m != nil {
			return m.Source
		}
		return -1
	}); !ok {
		return nil, &EvaluationError{Kind: EvalMatteCycle, Path: result[cycle].Path}
	}
	for i := range result {
		node := &result[i]
		content := &node.Layer.Content
		switch content.Kind {
		case ContentInstance:
			var childFrame float64
			if tm := content.TimeRemap; tm != nil {
				// Port: velato samples tm at frame/stretch - start_frame.
				// The keyframes of tm are on the timeline of the parent, so
				// okage samples tm at the frame and turns the seconds into
				// frames with 0 as the least, as ThorVG does. The stretch
				// does not apply, because tm gives the time itself (D43).
				// ThorVG computes the frame in float32, which decides a frame
				// that falls on the out point of a layer.
				childFrame = float64(float32(math.Max(tm.Evaluate(frame), 0)) * float32(animation.FrameRate))
				if lottiecompat.Velato {
					childFrame = tm.Evaluate(frame/node.Layer.Stretch-node.Layer.StartFrame) * animation.FrameRate
				}
			} else {
				childFrame = frame/node.Layer.Stretch - node.Layer.StartFrame/node.Layer.Stretch
			}
			if !isFiniteF64(childFrame) {
				return nil, &EvaluationError{Kind: EvalNonFiniteTime}
			}
			if slices.Contains(assets, content.Name) {
				return nil, &EvaluationError{Kind: EvalPrecompCycle, Name: content.Name}
			}
			children, ok := animation.Assets[content.Name]
			if !ok {
				return nil, &EvaluationError{Kind: EvalMissingAsset, Name: content.Name}
			}
			childPrefix := append(slices.Clone(prefix), node.Path[len(node.Path)-1])
			kids, err := evaluateLayers(animation, children, childFrame, node.FullTransform, node.Visible, childPrefix, append(slices.Clone(assets), content.Name))
			if err != nil {
				return nil, err
			}
			node.Children = kids
		case ContentImage:
			if _, ok := animation.Images[content.Name]; !ok {
				return nil, &EvaluationError{Kind: EvalMissingAsset, Name: content.Name}
			}
		}
	}
	return result, nil
}
