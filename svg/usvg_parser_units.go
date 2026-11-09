// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"slices"

	"github.com/xo/okage/internal/svgtypes"
)

// This file ports usvg src/parser/units.rs.

// convertLength is convert_length in usvg src/parser/units.rs.
//
// Port: the Go name has no suffix. The method SvgNode::convert_length of
// src/parser/converter.rs is the method svgNode.convertLength.
func convertLength(
	length svgtypes.Length,
	node svgNode,
	aid AId,
	objectUnits units,
	state *converterState,
) float32 {
	dpi := state.opt.dpi
	n := float32(length.Number)
	switch length.Unit {
	case svgtypes.LengthUnitNone, svgtypes.LengthUnitPx:
		return n
	case svgtypes.LengthUnitEm:
		return n * resolveFontSize(node, state)
	case svgtypes.LengthUnitEx:
		return float32(n*resolveFontSize(node, state)) / 2.0
	case svgtypes.LengthUnitIn:
		return n * dpi
	case svgtypes.LengthUnitCm:
		return float32(n*dpi) / 2.54
	case svgtypes.LengthUnitMm:
		return float32(n*dpi) / 25.4
	case svgtypes.LengthUnitPt:
		return float32(n*dpi) / 72.0
	case svgtypes.LengthUnitPc:
		return float32(n*dpi) / 6.0
	case svgtypes.LengthUnitPercent:
		if objectUnits == unitsObjectBoundingBox {
			return n / 100.0
		}
		viewBox := state.viewBox

		switch aid {
		case AIdCx, AIdDx, AIdFx, AIdMarkerWidth, AIdRefX, AIdRx, AIdWidth, AIdX, AIdX1, AIdX2:
			return convertPercent(length, viewBox.Width())
		case AIdCy, AIdDy, AIdFy, AIdHeight, AIdMarkerHeight, AIdRefY, AIdRy, AIdY, AIdY1, AIdY2:
			return convertPercent(length, viewBox.Height())
		}
		vbLen := float32(powi32(viewBox.Width(), 2)) + powi32(viewBox.Height(), 2)
		vbLen = float32(math.Sqrt(float64(vbLen / 2.0)))
		return convertPercent(length, vbLen)
	}
	return n
}

// convertUserLength is convert_user_length in usvg src/parser/units.rs.
//
// Port: the Go name has no suffix. The method SvgNode::convert_user_length
// of src/parser/converter.rs is the method svgNode.convertUserLength.
func convertUserLength(
	length svgtypes.Length,
	node svgNode,
	aid AId,
	state *converterState,
) float32 {
	return convertLength(length, node, aid, unitsUserSpaceOnUse, state)
}

// convertList is convert_list in usvg src/parser/units.rs.
func convertList(node svgNode, aid AId, state *converterState) ([]float32, bool) {
	text, ok := node.attributeStr(aid)
	if !ok {
		return nil, false
	}
	numList := []float32{}
	for length, err := range svgtypes.NewLengthListParser(text).All() {
		if err != nil {
			continue
		}
		numList = append(numList, convertUserLength(length, node, aid, state))
	}

	return numList, true
}

// convertPercent is convert_percent in usvg src/parser/units.rs.
func convertPercent(length svgtypes.Length, base float32) float32 {
	return float32(base*float32(length.Number)) / 100.0
}

// resolveFontSize is resolve_font_size in usvg src/parser/units.rs.
func resolveFontSize(node svgNode, state *converterState) float32 {
	nodes := slices.Collect(node.ancestors())
	fontSize := state.opt.fontSize
	// skip Root
	for i := len(nodes) - 2; i >= 0; i-- {
		n := nodes[i]
		if length, ok := nodeTryAttribute(n, AIdFontSize, fromValueLength); ok {
			dpi := state.opt.dpi
			n := float32(length.Number)
			switch length.Unit {
			case svgtypes.LengthUnitNone, svgtypes.LengthUnitPx:
				fontSize = n
			case svgtypes.LengthUnitEm:
				fontSize = n * fontSize
			case svgtypes.LengthUnitEx:
				fontSize = float32(n*fontSize) / 2.0
			case svgtypes.LengthUnitIn:
				fontSize = n * dpi
			case svgtypes.LengthUnitCm:
				fontSize = float32(n*dpi) / 2.54
			case svgtypes.LengthUnitMm:
				fontSize = float32(n*dpi) / 25.4
			case svgtypes.LengthUnitPt:
				fontSize = float32(n*dpi) / 72.0
			case svgtypes.LengthUnitPc:
				fontSize = float32(n*dpi) / 6.0
			case svgtypes.LengthUnitPercent:
				// If `font-size` has percent units that it's value
				// is relative to the parent node `font-size`.
				fontSize = float32(float32(length.Number)*fontSize) * 0.01
			}
		} else if name, ok := n.attributeStr(AIdFontSize); ok {
			fontSize = convertNamedFontSize(name, fontSize, state.opt)
		}
	}

	return fontSize
}

// convertNamedFontSize is convert_named_font_size in usvg
// src/parser/units.rs.
//
// Port: opt gives the logger (D20).
func convertNamedFontSize(name string, parentFontSize float32, opt *options) float32 {
	var factor int32
	switch name {
	case "xx-small":
		factor = -3
	case "x-small":
		factor = -2
	case "small":
		factor = -1
	case "medium":
		factor = 0
	case "large":
		factor = 1
	case "x-large":
		factor = 2
	case "xx-large":
		factor = 3
	case "smaller":
		factor = -1
	case "larger":
		factor = 1
	default:
		opt.warnf("Invalid 'font-size' value: '%s'.", name)
		factor = 0
	}

	// 'On a computer screen a scaling factor of 1.2 is suggested between adjacent indexes.'
	return parentFontSize * powi32(1.2, factor)
}

// powi32 is f32::powi of Rust. Rust calls the intrinsic llvm.powi, which
// compiler-rt computes as __powisf2: a product of squares in f32, and the
// reciprocal of the product for a negative exponent.
func powi32(a float32, b int32) float32 {
	recip := b < 0
	r := float32(1)
	for {
		if b&1 != 0 {
			r *= a
		}
		b /= 2
		if b == 0 {
			break
		}
		a *= a
	}
	if recip {
		return 1 / r
	}
	return r
}
