// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
)

// This file ports krilla-svg src/util.rs. The traits KrillaTransformExt and
// UsvgTransformExt convert between the transforms of krilla and of usvg.
// pdf and svg both take the Transform of the package path, so the port has
// no conversion.

// convertSpreadMethod is convert_spread_method in krilla-svg src/util.rs.
//
// Convert a usvg SpreadMethod into a krilla SpreadMethod.
func convertSpreadMethod(spreadMethod svg.SpreadMethod) pdf.SpreadMethod {
	switch spreadMethod {
	case svg.SpreadMethodReflect:
		return pdf.SpreadMethodReflect
	case svg.SpreadMethodRepeat:
		return pdf.SpreadMethodRepeat
	default:
		return pdf.SpreadMethodPad
	}
}

// convertStop is convert_stop in krilla-svg src/util.rs. The offset and the
// opacity of usvg are the NormalizedF32 of path, as in krilla, so the port
// takes them as they are.
//
// Convert a usvg Stop into a krilla Stop.
func convertStop(stop svg.Stop) pdf.Stop {
	return pdf.Stop{
		Offset:  stop.Offset(),
		Color:   pdf.NewColorRGB(stop.Color().Red, stop.Color().Green, stop.Color().Blue),
		Opacity: stop.Opacity(),
	}
}

// convertStops converts the stops of a gradient with convertStop, as the
// iterator of upstream does.
func convertStops(stops []svg.Stop) []pdf.Stop {
	out := make([]pdf.Stop, len(stops))
	for i, s := range stops {
		out[i] = convertStop(s)
	}
	return out
}

// convertPaint is convert_paint in krilla-svg src/util.rs. Upstream takes a
// StreamBuilder that the caller makes. The port takes the surface and makes
// the StreamBuilder for a pattern only, because a StreamBuilder that is not
// finished writes nothing.
//
// Convert a usvg Paint into a krilla Paint.
//
// The additional transform is needed because in krilla, a transform to a
// shape will also apply the transform to the paint server. However, in the
// case of SVG glyphs, we don't want the transform to be shifted for each
// glyph we draw (since we draw them separately instead of in a glyph run),
// so we need to apply an additional inverse transform to counter that
// effect.
func convertPaint(paint svg.Paint, surface *pdf.Surface, pc *processContext, additionalTransform path.Transform) pdf.Paint {
	switch p := paint.(type) {
	case svg.Color:
		if p.Red == p.Green && p.Green == p.Blue {
			// Use gray-scale color if possible.
			return pdf.NewColorLuma(p.Red)
		}
		return pdf.NewColorRGB(p.Red, p.Green, p.Blue)
	case *svg.LinearGradient:
		return pdf.LinearGradient{
			X1:           p.X1(),
			Y1:           p.Y1(),
			X2:           p.X2(),
			Y2:           p.Y2(),
			Transform:    additionalTransform.PreConcat(p.Transform()),
			SpreadMethod: convertSpreadMethod(p.SpreadMethod()),
			Stops:        convertStops(p.Stops()),
			AntiAlias:    false,
		}
	case *svg.RadialGradient:
		return pdf.RadialGradient{
			Cx:           p.Cx(),
			Cy:           p.Cy(),
			Cr:           p.R().Get(),
			Fx:           p.Fx(),
			Fy:           p.Fy(),
			Fr:           0.0,
			Transform:    additionalTransform.PreConcat(p.Transform()),
			SpreadMethod: convertSpreadMethod(p.SpreadMethod()),
			Stops:        convertStops(p.Stops()),
			AntiAlias:    false,
		}
	case *svg.Pattern:
		streamBuilder := surface.StreamBuilder()
		sub := streamBuilder.Surface()
		renderRoot(p.Root(), sub, pc)
		sub.Finish()
		stream := streamBuilder.Finish()

		return pdf.Pattern{
			Stream: stream,
			Transform: additionalTransform.
				PreConcat(p.Transform()).
				PreConcat(path.TransformFromTranslate(p.Rect().X(), p.Rect().Y())),
			Width:  p.Rect().Width(),
			Height: p.Rect().Height(),
		}
	}
	return nil
}

// convertLineCap is convert_line_cap in krilla-svg src/util.rs.
//
// Convert a usvg LineCap into a krilla LineCap.
func convertLineCap(lineCap svg.LineCap) pdf.LineCap {
	switch lineCap {
	case svg.LineCapRound:
		return pdf.LineCapRound
	case svg.LineCapSquare:
		return pdf.LineCapSquare
	default:
		return pdf.LineCapButt
	}
}

// convertLineJoin is convert_line_join in krilla-svg src/util.rs.
//
// Convert a usvg LineJoin into a krilla LineJoin.
func convertLineJoin(lineJoin svg.LineJoin) pdf.LineJoin {
	switch lineJoin {
	case svg.LineJoinRound:
		return pdf.LineJoinRound
	case svg.LineJoinBevel:
		return pdf.LineJoinBevel
	default:
		// Miter and MiterClip.
		return pdf.LineJoinMiter
	}
}

// convertFillRule is convert_fill_rule in krilla-svg src/util.rs.
//
// Convert a usvg FillRule into a krilla FillRule.
func convertFillRule(fillRule svg.FillRule) pdf.FillRule {
	if fillRule == svg.FillRuleEvenOdd {
		return pdf.FillRuleEvenOdd
	}
	return pdf.FillRuleNonZero
}

// convertFill is convert_fill in krilla-svg src/util.rs.
//
// Convert a usvg Fill into a krilla Fill.
func convertFill(fill *svg.Fill, surface *pdf.Surface, pc *processContext, additionalTransform path.Transform) *pdf.Fill {
	return &pdf.Fill{
		Paint:   convertPaint(fill.Paint(), surface, pc, additionalTransform),
		Opacity: fill.Opacity(),
		Rule:    convertFillRule(fill.Rule()),
	}
}

// convertStroke is convert_stroke in krilla-svg src/util.rs.
//
// Convert a usvg Stroke into a krilla Stroke.
func convertStroke(stroke *svg.Stroke, surface *pdf.Surface, pc *processContext, additionalTransform path.Transform) *pdf.Stroke {
	var dash *pdf.StrokeDash
	if dashArray, ok := stroke.Dasharray(); ok {
		dash = &pdf.StrokeDash{
			Offset: stroke.Dashoffset(),
			Array:  append([]float32(nil), dashArray...),
		}
	}

	return &pdf.Stroke{
		Paint:      convertPaint(stroke.Paint(), surface, pc, additionalTransform),
		Width:      stroke.Width().Get(),
		MiterLimit: stroke.Miterlimit().Get(),
		LineCap:    convertLineCap(stroke.Linecap()),
		LineJoin:   convertLineJoin(stroke.Linejoin()),
		Opacity:    stroke.Opacity(),
		Dash:       dash,
	}
}

// convertBlendMode is convert_blend_mode in krilla-svg src/util.rs.
//
// Convert a usvg BlendMode into a krilla BlendMode.
func convertBlendMode(blendMode svg.BlendMode) pdf.BlendMode {
	switch blendMode {
	case svg.BlendModeNormal:
		return pdf.BlendModeNormal
	case svg.BlendModeMultiply:
		return pdf.BlendModeMultiply
	case svg.BlendModeScreen:
		return pdf.BlendModeScreen
	case svg.BlendModeOverlay:
		return pdf.BlendModeOverlay
	case svg.BlendModeDarken:
		return pdf.BlendModeDarken
	case svg.BlendModeLighten:
		return pdf.BlendModeLighten
	case svg.BlendModeColorDodge:
		return pdf.BlendModeColorDodge
	case svg.BlendModeColorBurn:
		return pdf.BlendModeColorBurn
	case svg.BlendModeHardLight:
		return pdf.BlendModeHardLight
	case svg.BlendModeSoftLight:
		return pdf.BlendModeSoftLight
	case svg.BlendModeDifference:
		return pdf.BlendModeDifference
	case svg.BlendModeExclusion:
		return pdf.BlendModeExclusion
	case svg.BlendModeHue:
		return pdf.BlendModeHue
	case svg.BlendModeSaturation:
		return pdf.BlendModeSaturation
	case svg.BlendModeColor:
		return pdf.BlendModeColor
	case svg.BlendModeLuminosity:
		return pdf.BlendModeLuminosity
	}
	// okage: usvg 0.47, which krilla-svg 0.8.1 requires, has no PlusLighter.
	// PDF has no such blend mode, so group.go draws the group with the
	// raster fallback, and this case is not reached for it (D62).
	return pdf.BlendModeNormal
}

// convertMaskType is convert_mask_type in krilla-svg src/util.rs.
//
// Convert a usvg MaskType into a krilla MaskType.
func convertMaskType(maskType svg.MaskType) pdf.MaskType {
	if maskType == svg.MaskTypeAlpha {
		return pdf.MaskTypeAlpha
	}
	return pdf.MaskTypeLuminosity
}

// rectToClipPath is RectExt::to_clip_path in krilla-svg src/util.rs.
func rectToClipPath(r path.Rect) *path.Path {
	pathBuilder := path.NewPathBuilder()
	pathBuilder.MoveTo(r.Left(), r.Top())
	pathBuilder.LineTo(r.Right(), r.Top())
	pathBuilder.LineTo(r.Right(), r.Bottom())
	pathBuilder.LineTo(r.Left(), r.Bottom())
	pathBuilder.Close()
	p, _ := pathBuilder.Finish()
	return p
}

// pathToKrilla is PathExt::to_krilla in krilla-svg src/util.rs. It builds
// the path again from its segments, as upstream does. It returns nil where
// upstream panics, for a path that the builder does not finish. The paths
// of svg always finish.
func pathToKrilla(p *svg.Path) *path.Path {
	pb := path.NewPathBuilder()

	for seg := range p.Data().Segments().All() {
		pts := seg.Points
		switch seg.Kind {
		case path.PathSegmentMoveTo:
			pb.MoveTo(pts[0].X, pts[0].Y)
		case path.PathSegmentLineTo:
			pb.LineTo(pts[0].X, pts[0].Y)
		case path.PathSegmentQuadTo:
			pb.QuadTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y)
		case path.PathSegmentCubicTo:
			pb.CubicTo(pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
		case path.PathSegmentClose:
			pb.Close()
		}
	}

	out, ok := pb.Finish()
	if !ok {
		return nil
	}
	return out
}
