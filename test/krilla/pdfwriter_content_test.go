package krilla

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
)

// The Go side of the typed calls of content.rs, which
// rust/src/writer_content.rs makes with pdf-writer. An operation of a
// content stream whose "op" is the name of a method of Content, such as
// "set_line_width", calls that method with its operands as the arguments.
// The comment of writer_content.rs gives the forms of the arguments and of
// the calls on the writers that a method starts.

func init() {
	goContent = contentOps
	writerDicts["ext_graphics"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.ExtGraphics(id).Dict
	}
}

// contentOps writes the operations of a content stream, as the function
// content of writer.rs does.
func contentOps(ops []any, settings pdfwriter.Settings) ([]byte, error) {
	c := pdfwriter.NewContentWithSettings(settings)
	for _, o := range ops {
		o := as[map[string]any](o)
		name := as[string](o["op"])
		operands, _ := o["operands"].([]any)
		ok, err := contentCall(c, name, operands)
		if err != nil {
			return nil, err
		}
		if ok {
			continue
		}
		w := c.Op(name)
		for _, v := range operands {
			if err := writeValue(w.Obj(), v); err != nil {
				return nil, err
			}
		}
	}
	return c.Finish().IntoVec(), nil
}

// contentCall is call of writer_content.rs. It returns false when the name
// is not a typed call. An argument of the wrong form panics, and goWriter
// gives the panic as an error.
func contentCall(c *pdfwriter.Content, name string, a []any) (bool, error) {
	switch name {
	case "set_line_width":
		c.SetLineWidth(argReal(a, 0))
	case "set_line_cap":
		c.SetLineCap(lookup(argName(a, 0), pdfwriter.LineCapStyleButtCap,
			pdfwriter.LineCapStyleRoundCap, pdfwriter.LineCapStyleProjectingSquareCap))
	case "set_line_join":
		c.SetLineJoin(lookup(argName(a, 0), pdfwriter.LineJoinStyleMiterJoin,
			pdfwriter.LineJoinStyleRoundJoin, pdfwriter.LineJoinStyleBevelJoin))
	case "set_miter_limit":
		c.SetMiterLimit(argReal(a, 0))
	case "set_dash_pattern":
		c.SetDashPattern(argReals(a, 0), argReal(a, 1))
	case "set_rendering_intent":
		c.SetRenderingIntent(renderingIntent(argName(a, 0)))
	case "set_flatness":
		c.SetFlatness(argInt(a, 0))
	case "set_parameters":
		c.SetParameters(pdfwriter.Name(argName(a, 0)))
	case "save_state":
		c.SaveState()
	case "restore_state":
		c.RestoreState()
	case "transform":
		c.Transform(argMatrix(a, 0))
	case "move_to":
		c.MoveTo(argReal(a, 0), argReal(a, 1))
	case "line_to":
		c.LineTo(argReal(a, 0), argReal(a, 1))
	case "cubic_to":
		c.CubicTo(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3), argReal(a, 4), argReal(a, 5))
	case "cubic_to_initial":
		c.CubicToInitial(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3))
	case "cubic_to_final":
		c.CubicToFinal(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3))
	case "close_path":
		c.ClosePath()
	case "rect":
		c.Rect(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3))
	case "stroke":
		c.Stroke()
	case "close_and_stroke":
		c.CloseAndStroke()
	case "fill_nonzero":
		c.FillNonzero()
	case "fill_even_odd":
		c.FillEvenOdd()
	case "fill_nonzero_and_stroke":
		c.FillNonzeroAndStroke()
	case "fill_even_odd_and_stroke":
		c.FillEvenOddAndStroke()
	case "close_fill_nonzero_and_stroke":
		c.CloseFillNonzeroAndStroke()
	case "close_fill_even_odd_and_stroke":
		c.CloseFillEvenOddAndStroke()
	case "end_path":
		c.EndPath()
	case "clip_nonzero":
		c.ClipNonzero()
	case "clip_even_odd":
		c.ClipEvenOdd()
	case "begin_text":
		c.BeginText()
	case "end_text":
		c.EndText()
	case "set_char_spacing":
		c.SetCharSpacing(argReal(a, 0))
	case "set_word_spacing":
		c.SetWordSpacing(argReal(a, 0))
	case "set_horizontal_scaling":
		c.SetHorizontalScaling(argReal(a, 0))
	case "set_leading":
		c.SetLeading(argReal(a, 0))
	case "set_font":
		c.SetFont(pdfwriter.Name(argName(a, 0)), argReal(a, 1))
	case "set_text_rendering_mode":
		c.SetTextRenderingMode(lookup(argName(a, 0), textRenderingModes...))
	case "set_rise":
		c.SetRise(argReal(a, 0))
	case "next_line":
		c.NextLine(argReal(a, 0), argReal(a, 1))
	case "next_line_and_set_leading":
		c.NextLineAndSetLeading(argReal(a, 0), argReal(a, 1))
	case "set_text_matrix":
		c.SetTextMatrix(argMatrix(a, 0))
	case "next_line_using_leading":
		c.NextLineUsingLeading()
	case "show":
		c.Show(argStr(a, 0))
	case "next_line_show":
		c.NextLineShow(argStr(a, 0))
	case "next_line_show_and_set_word_and_char_spacing":
		c.NextLineShowAndSetWordAndCharSpacing(argReal(a, 0), argReal(a, 1), argStr(a, 2))
	case "show_positioned":
		return true, showPositioned(c.ShowPositioned(), a)
	case "start_color_glyph":
		c.StartColorGlyph(argReal(a, 0))
	case "start_shape_glyph":
		c.StartShapeGlyph(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3), argReal(a, 4))
	case "set_stroke_color_space":
		c.SetStrokeColorSpace(colorSpace(arg(a, 0)))
	case "set_fill_color_space":
		c.SetFillColorSpace(colorSpace(arg(a, 0)))
	case "set_stroke_color":
		c.SetStrokeColor(argReals(a, 0)...)
	case "set_stroke_pattern":
		c.SetStrokePattern(argReals(a, 0), pdfwriter.Name(argName(a, 1)))
	case "set_fill_color":
		c.SetFillColor(argReals(a, 0)...)
	case "set_fill_pattern":
		c.SetFillPattern(argReals(a, 0), pdfwriter.Name(argName(a, 1)))
	case "set_stroke_gray":
		c.SetStrokeGray(argReal(a, 0))
	case "set_fill_gray":
		c.SetFillGray(argReal(a, 0))
	case "set_stroke_rgb":
		c.SetStrokeRGB(argReal(a, 0), argReal(a, 1), argReal(a, 2))
	case "set_fill_rgb":
		c.SetFillRGB(argReal(a, 0), argReal(a, 1), argReal(a, 2))
	case "set_stroke_cmyk":
		c.SetStrokeCMYK(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3))
	case "set_fill_cmyk":
		c.SetFillCMYK(argReal(a, 0), argReal(a, 1), argReal(a, 2), argReal(a, 3))
	case "shading":
		c.Shading(pdfwriter.Name(argName(a, 0)))
	case "x_object":
		c.XObject(pdfwriter.Name(argName(a, 0)))
	case "marked_content_point":
		c.MarkedContentPoint(pdfwriter.Name(argName(a, 0)))
	case "marked_content_point_with_properties":
		return true, markContent(c.MarkedContentPointWithProperties(pdfwriter.Name(argName(a, 0))), a[1:])
	case "begin_marked_content":
		c.BeginMarkedContent(pdfwriter.Name(argName(a, 0)))
	case "begin_marked_content_with_properties":
		return true, markContent(c.BeginMarkedContentWithProperties(pdfwriter.Name(argName(a, 0))), a[1:])
	case "end_marked_content":
		c.EndMarkedContent()
	case "begin_compat":
		c.BeginCompat()
	case "end_compat":
		c.EndCompat()
	case "ext_graphics":
		op := c.Op(argName(a, 0))
		return true, extGraphics(pdfwriter.Start[pdfwriter.ExtGraphicsState](op.Obj()), a[1:])
	case "resources":
		op := c.Op(argName(a, 0))
		return true, resources(pdfwriter.Start[pdfwriter.Resources](op.Obj()), a[1:])
	default:
		return false, nil
	}
	return true, nil
}

// showPositioned makes the calls on a ShowPositioned.
func showPositioned(w *pdfwriter.ShowPositioned, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		switch n {
		case "items":
			if err := positionedItems(w.Items(), a); err != nil {
				return err
			}
		case "obj":
			if err := writeValue(w.Obj(), arg(a, 0)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("ShowPositioned has no call %q", n)
		}
	}
	return nil
}

// positionedItems makes the calls on a PositionedItems.
func positionedItems(w *pdfwriter.PositionedItems, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		switch n {
		case "show":
			w.Show(argStr(a, 0))
		case "adjust":
			w.Adjust(argReal(a, 0))
		case "item":
			if err := writeValue(w.Push(), arg(a, 0)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("PositionedItems has no call %q", n)
		}
	}
	return nil
}

// markContent makes the calls on a MarkContent. The call
// "properties_named" must be the last one.
func markContent(w *pdfwriter.MarkContent, calls []any) error {
	for i, v := range calls {
		n, a := subCall(v)
		switch {
		case n == "properties":
			if err := propertyList(w.Properties(), a); err != nil {
				return err
			}
		case n == "properties_named" && i == len(calls)-1:
			w.PropertiesNamed(pdfwriter.Name(argName(a, 0)))
			return nil
		case n == "obj":
			if err := writeValue(w.Obj(), arg(a, 0)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("MarkContent has no call %q at %d", n, i)
		}
	}
	return nil
}

// propertyList makes the calls on a PropertyList. The call "artifact" must
// be the last one.
func propertyList(w *pdfwriter.PropertyList, calls []any) error {
	for i, v := range calls {
		n, a := subCall(v)
		switch {
		case n == "identify":
			w.Identify(argInt(a, 0))
		case n == "actual_text":
			w.ActualText(pdfwriter.TextStr(argText(a, 0)))
		case n == "artifact" && i == len(calls)-1:
			return artifact(w.Artifact(), a)
		case n == "pair":
			if err := pair(w.Dict, a); err != nil {
				return err
			}
		default:
			return fmt.Errorf("PropertyList has no call %q at %d", n, i)
		}
	}
	return nil
}

// artifact makes the calls on an Artifact.
func artifact(w *pdfwriter.Artifact, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		switch n {
		case "kind":
			w.Kind(lookup(argName(a, 0), pdfwriter.ArtifactTypePagination, pdfwriter.ArtifactTypeLayout,
				pdfwriter.ArtifactTypePage, pdfwriter.ArtifactTypeBackground, pdfwriter.ArtifactTypeInline))
		case "subtype":
			w.Subtype(artifactSubtype(arg(a, 0)))
		case "bounding_box":
			r := argReals(a, 0)
			if len(r) != 4 {
				return errors.New("a Rect has four numbers")
			}
			w.BoundingBox(pdfwriter.NewRect(r[0], r[1], r[2], r[3]))
		case "attached":
			var list []pdfwriter.ArtifactAttachment
			for i := range a {
				list = append(list, lookup(argName(a, i), pdfwriter.ArtifactAttachmentLeft,
					pdfwriter.ArtifactAttachmentTop, pdfwriter.ArtifactAttachmentRight, pdfwriter.ArtifactAttachmentBottom))
			}
			w.Attached(list...)
		case "pair":
			if err := pair(w.Dict, a); err != nil {
				return err
			}
		default:
			return fmt.Errorf("Artifact has no call %q", n)
		}
	}
	return nil
}

// resources makes the calls on a Resources.
func resources(w *pdfwriter.Resources, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		var err error
		switch n {
		case "x_objects":
			err = dictCalls(w.XObjects(), a)
		case "fonts":
			err = dictCalls(w.Fonts(), a)
		case "color_spaces":
			err = dictCalls(w.ColorSpaces(), a)
		case "patterns":
			err = dictCalls(w.Patterns(), a)
		case "shadings":
			err = dictCalls(w.Shadings(), a)
		case "ext_g_states":
			err = dictCalls(w.ExtGStates(), a)
		case "proc_sets":
			var list []pdfwriter.ProcSet
			for i := range a {
				list = append(list, lookup(argName(a, i), pdfwriter.ProcSetPDF, pdfwriter.ProcSetText,
					pdfwriter.ProcSetImageGrayscale, pdfwriter.ProcSetImageColor, pdfwriter.ProcSetImageIndexed))
			}
			w.ProcSets(list...)
		case "proc_sets_all":
			w.ProcSetsAll()
		case "properties":
			d := w.Properties()
			for _, v := range a {
				n, a := subCall(v)
				if n != "insert" {
					return fmt.Errorf("TypedDict<PropertyList> has no call %q", n)
				}
				if err := propertyList(d.Insert(pdfwriter.Name(argName(a, 0))), a[1:]); err != nil {
					return err
				}
			}
		case "pair":
			err = pair(w.Dict, a)
		default:
			return fmt.Errorf("Resources has no call %q", n)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// dictCalls makes the calls "pair" on a Dict.
func dictCalls(w *pdfwriter.Dict, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		if n != "pair" {
			return fmt.Errorf("Dict has no call %q", n)
		}
		if err := pair(w, a); err != nil {
			return err
		}
	}
	return nil
}

// extGraphics makes the calls on an ExtGraphicsState.
func extGraphics(w *pdfwriter.ExtGraphicsState, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		switch n {
		case "line_width":
			w.LineWidth(argReal(a, 0))
		case "line_cap":
			w.LineCap(lookup(argName(a, 0), pdfwriter.LineCapStyleButtCap,
				pdfwriter.LineCapStyleRoundCap, pdfwriter.LineCapStyleProjectingSquareCap))
		case "line_join":
			w.LineJoin(lookup(argName(a, 0), pdfwriter.LineJoinStyleMiterJoin,
				pdfwriter.LineJoinStyleRoundJoin, pdfwriter.LineJoinStyleBevelJoin))
		case "miter_limit":
			w.MiterLimit(argReal(a, 0))
		case "dash_pattern":
			w.DashPattern(argReals(a, 0), argReal(a, 1))
		case "rendering_intent":
			w.RenderingIntent(renderingIntent(argName(a, 0)))
		case "overprint":
			w.Overprint(argBool(a, 0))
		case "overprint_fill":
			w.OverprintFill(argBool(a, 0))
		case "overprint_mode":
			w.OverprintMode(lookup(argName(a, 0), pdfwriter.OverprintModeOverrideAllColorants,
				pdfwriter.OverprintModeIgnoreZeroChannel))
		case "font":
			w.Font(pdfwriter.Name(argName(a, 0)), argReal(a, 1))
		case "black_generation":
			w.BlackGeneration(argRef(a, 0))
		case "black_generation_default":
			w.BlackGenerationDefault()
		case "undercolor_removal":
			w.UndercolorRemoval(argRef(a, 0))
		case "undercolor_removal_default":
			w.UndercolorRemovalDefault()
		case "transfer":
			w.Transfer(argRef(a, 0))
		case "transfer_default":
			w.TransferDefault()
		case "halftone":
			w.Halftone(argRef(a, 0))
		case "halftone_default":
			w.HalftoneDefault()
		case "flatness":
			w.Flatness(argReal(a, 0))
		case "smoothness":
			w.Smoothness(argReal(a, 0))
		case "stroke_adjustment":
			w.StrokeAdjustment(argBool(a, 0))
		case "blend_mode":
			w.BlendMode(lookup(argName(a, 0), blendModes...))
		case "soft_mask":
			if err := softMask(w.SoftMask(), a); err != nil {
				return err
			}
		case "soft_mask_name":
			w.SoftMaskName(pdfwriter.Name(argName(a, 0)))
		case "stroking_alpha":
			w.StrokingAlpha(argReal(a, 0))
		case "non_stroking_alpha":
			w.NonStrokingAlpha(argReal(a, 0))
		case "alpha_source":
			w.AlphaSource(argBool(a, 0))
		case "text_knockout":
			w.TextKnockout(argBool(a, 0))
		case "pair":
			if err := pair(w.Dict, a); err != nil {
				return err
			}
		default:
			return fmt.Errorf("ExtGraphicsState has no call %q", n)
		}
	}
	return nil
}

// softMask makes the calls on a SoftMask.
func softMask(w *pdfwriter.SoftMask, calls []any) error {
	for _, v := range calls {
		n, a := subCall(v)
		switch n {
		case "subtype":
			w.Subtype(lookup(argName(a, 0), pdfwriter.MaskTypeAlpha, pdfwriter.MaskTypeLuminosity))
		case "group":
			w.Group(argRef(a, 0))
		case "backdrop":
			w.Backdrop(argReals(a, 0)...)
		case "transfer_function":
			w.TransferFunction(argRef(a, 0))
		case "pair":
			if err := pair(w.Dict, a); err != nil {
				return err
			}
		default:
			return fmt.Errorf("SoftMask has no call %q", n)
		}
	}
	return nil
}

// pair writes the call "pair" of a writer that embeds a Dict.
func pair(d *pdfwriter.Dict, a []any) error {
	return writeValue(d.Insert(pdfwriter.Name(argName(a, 0))), arg(a, 1))
}

// subCall splits a call on a writer into its name and its arguments.
func subCall(v any) (string, []any) {
	items := as[[]any](as[map[string]any](v)["array"])
	return argName(items, 0), items[1:]
}

func arg(a []any, i int) any {
	if i >= len(a) {
		panic(fmt.Sprintf("the argument %d is missing", i))
	}
	return a[i]
}

// field returns the field of a value with one key, such as {"real": 1}.
func field(a []any, i int, key string) any {
	m := as[map[string]any](arg(a, i))
	v, ok := m[key]
	if !ok {
		panic(fmt.Sprintf("the argument %d is not a %s", i, key))
	}
	return v
}

func argReal(a []any, i int) float32 {
	// serde reads an f32 as an f64, and then rounds it to an f32.
	f, err := strconv.ParseFloat(string(as[json.Number](field(a, i, "real"))), 64)
	if err != nil {
		panic(err)
	}
	return float32(f)
}

func argInt(a []any, i int) int32 {
	n, err := strconv.ParseInt(string(as[json.Number](field(a, i, "int"))), 10, 32)
	if err != nil {
		panic(err)
	}
	return int32(n)
}

func argBool(a []any, i int) bool { return as[bool](field(a, i, "bool")) }

func argName(a []any, i int) string { return as[string](field(a, i, "name")) }

func argText(a []any, i int) string { return as[string](field(a, i, "text")) }

func argStr(a []any, i int) pdfwriter.Str {
	b, err := hex.DecodeString(as[string](field(a, i, "str")))
	if err != nil {
		panic(err)
	}
	return pdfwriter.Str(b)
}

func argRef(a []any, i int) pdfwriter.Ref {
	r, err := reference(field(a, i, "ref"))
	if err != nil {
		panic(err)
	}
	return r
}

func argReals(a []any, i int) []float32 {
	items := as[[]any](field(a, i, "array"))
	out := make([]float32, len(items))
	for j := range items {
		out[j] = argReal(items, j)
	}
	return out
}

func argMatrix(a []any, i int) [6]float32 {
	r := argReals(a, i)
	if len(r) != 6 {
		panic("a matrix has six numbers")
	}
	return [6]float32(r)
}

// lookup returns the value whose String is the name.
func lookup[T fmt.Stringer](name string, values ...T) T {
	for _, v := range values {
		if v.String() == name {
			return v
		}
	}
	panic(fmt.Sprintf("the name %q is not known", name))
}

func renderingIntent(name string) pdfwriter.RenderingIntent {
	return lookup(name, pdfwriter.RenderingIntentAbsoluteColorimetric, pdfwriter.RenderingIntentRelativeColorimetric,
		pdfwriter.RenderingIntentSaturation, pdfwriter.RenderingIntentPerceptual)
}

var textRenderingModes = []pdfwriter.TextRenderingMode{
	pdfwriter.TextRenderingModeFill, pdfwriter.TextRenderingModeStroke, pdfwriter.TextRenderingModeFillStroke,
	pdfwriter.TextRenderingModeInvisible, pdfwriter.TextRenderingModeFillClip, pdfwriter.TextRenderingModeStrokeClip,
	pdfwriter.TextRenderingModeFillStrokeClip, pdfwriter.TextRenderingModeClip,
}

var blendModes = []pdfwriter.BlendMode{
	pdfwriter.BlendModeNormal, pdfwriter.BlendModeMultiply, pdfwriter.BlendModeScreen, pdfwriter.BlendModeOverlay,
	pdfwriter.BlendModeDarken, pdfwriter.BlendModeLighten, pdfwriter.BlendModeColorDodge, pdfwriter.BlendModeColorBurn,
	pdfwriter.BlendModeHardLight, pdfwriter.BlendModeSoftLight, pdfwriter.BlendModeDifference,
	pdfwriter.BlendModeExclusion, pdfwriter.BlendModeHue, pdfwriter.BlendModeSaturation, pdfwriter.BlendModeColor,
	pdfwriter.BlendModeLuminosity,
}

// colorSpace reads a ColorSpaceOperand: a name for the case Named, and a
// text with the Rust name for the other cases.
func colorSpace(v any) pdfwriter.ColorSpaceOperand {
	m := as[map[string]any](v)
	if n, ok := m["name"]; ok {
		return pdfwriter.ColorSpaceOperandFromName(pdfwriter.Name(as[string](n)))
	}
	kind := lookup(as[string](m["text"]), pdfwriter.ColorSpaceOperandDeviceGray, pdfwriter.ColorSpaceOperandDeviceRGB,
		pdfwriter.ColorSpaceOperandDeviceCMYK, pdfwriter.ColorSpaceOperandPattern)
	return pdfwriter.ColorSpaceOperand{Kind: kind}
}

// artifactSubtype reads an ArtifactSubtype: a name for the case Custom,
// and a text with the Rust name for the other cases.
func artifactSubtype(v any) pdfwriter.ArtifactSubtype {
	m := as[map[string]any](v)
	if n, ok := m["name"]; ok {
		return pdfwriter.ArtifactSubtype{Kind: pdfwriter.ArtifactSubtypeCustom, Name: pdfwriter.Name(as[string](n))}
	}
	kind := lookup(as[string](m["text"]), pdfwriter.ArtifactSubtypeHeader, pdfwriter.ArtifactSubtypeFooter,
		pdfwriter.ArtifactSubtypeWatermark, pdfwriter.ArtifactSubtypePageNumber, pdfwriter.ArtifactSubtypeBates,
		pdfwriter.ArtifactSubtypeLineNumber, pdfwriter.ArtifactSubtypeRedaction)
	return pdfwriter.ArtifactSubtype{Kind: kind}
}

// call makes a call on a writer, as an operand of a typed call.
func call(name string, args ...any) any {
	return pw.Array(append([]any{pw.Name(name)}, args...)...)
}

// reals makes an array of reals.
func reals(fs ...float32) any {
	items := make([]any, len(fs))
	for i, f := range fs {
		items[i] = pw.Real(f)
	}
	return pw.Array(items...)
}

// everyContentCall calls each method of Content, and each method of the
// writers that it starts, once.
func everyContentCall() []writeOp {
	return []writeOp{
		{Op: "set_line_width", Operands: []any{pw.Real(0.5)}},
		{Op: "set_line_cap", Operands: []any{pw.Name("RoundCap")}},
		{Op: "set_line_join", Operands: []any{pw.Name("BevelJoin")}},
		{Op: "set_miter_limit", Operands: []any{pw.Real(4)}},
		{Op: "set_dash_pattern", Operands: []any{reals(3, 1.5), pw.Real(0)}},
		{Op: "set_dash_pattern", Operands: []any{reals(), pw.Real(-2)}},
		{Op: "set_rendering_intent", Operands: []any{pw.Name("Saturation")}},
		{Op: "set_flatness", Operands: []any{pw.Int(0)}},
		{Op: "set_parameters", Operands: []any{pw.Name("GS1")}},
		{Op: "save_state"},
		{Op: "transform", Operands: []any{reals(1, 0, 0, -1, 0.25, 1e-7)}},
		{Op: "move_to", Operands: []any{pw.Real(1), pw.Real(2)}},
		{Op: "line_to", Operands: []any{pw.Real(3e12), pw.Real(-4)}},
		{Op: "cubic_to", Operands: []any{pw.Real(1), pw.Real(2), pw.Real(3), pw.Real(4), pw.Real(5), pw.Real(6)}},
		{Op: "cubic_to_initial", Operands: []any{pw.Real(1), pw.Real(2), pw.Real(3), pw.Real(4)}},
		{Op: "cubic_to_final", Operands: []any{pw.Real(1), pw.Real(2), pw.Real(3), pw.Real(4)}},
		{Op: "close_path"},
		{Op: "rect", Operands: []any{pw.Real(0), pw.Real(0), pw.Real(10.5), pw.Real(20)}},
		{Op: "stroke"},
		{Op: "close_and_stroke"},
		{Op: "fill_nonzero"},
		{Op: "fill_even_odd"},
		{Op: "fill_nonzero_and_stroke"},
		{Op: "fill_even_odd_and_stroke"},
		{Op: "close_fill_nonzero_and_stroke"},
		{Op: "close_fill_even_odd_and_stroke"},
		{Op: "end_path"},
		{Op: "clip_nonzero"},
		{Op: "clip_even_odd"},
		{Op: "begin_text"},
		{Op: "set_char_spacing", Operands: []any{pw.Real(0.1)}},
		{Op: "set_word_spacing", Operands: []any{pw.Real(-0.2)}},
		{Op: "set_horizontal_scaling", Operands: []any{pw.Real(90)}},
		{Op: "set_leading", Operands: []any{pw.Real(14)}},
		{Op: "set_font", Operands: []any{pw.Name("F1"), pw.Real(12)}},
		{Op: "set_text_rendering_mode", Operands: []any{pw.Name("FillStrokeClip")}},
		{Op: "set_rise", Operands: []any{pw.Real(-3)}},
		{Op: "next_line", Operands: []any{pw.Real(1), pw.Real(2)}},
		{Op: "next_line_and_set_leading", Operands: []any{pw.Real(1), pw.Real(-12)}},
		{Op: "set_text_matrix", Operands: []any{reals(1, 0, 0, 1, 72, 720)}},
		{Op: "next_line_using_leading"},
		{Op: "show", Operands: []any{pw.Str([]byte("Hi (there)\n"))}},
		{Op: "next_line_show", Operands: []any{pw.Str([]byte{0xff, 0})}},
		{Op: "next_line_show_and_set_word_and_char_spacing", Operands: []any{pw.Real(1), pw.Real(2), pw.Str([]byte("x"))}},
		{Op: "show_positioned"},
		{Op: "show_positioned", Operands: []any{call("items")}},
		{Op: "show_positioned", Operands: []any{call("items",
			call("show", pw.Str([]byte("AB"))), call("adjust", pw.Real(-250)), call("item", pw.Int(3)),
			call("show", pw.Str([]byte("C"))))}},
		{Op: "show_positioned", Operands: []any{call("obj", pw.Name("N")), call("items", call("adjust", pw.Real(1)))}},
		{Op: "end_text"},
		{Op: "start_color_glyph", Operands: []any{pw.Real(500)}},
		{Op: "start_shape_glyph", Operands: []any{pw.Real(500), pw.Real(-10), pw.Real(-20), pw.Real(510), pw.Real(700)}},
		{Op: "set_stroke_color_space", Operands: []any{pw.Text("DeviceGray")}},
		{Op: "set_fill_color_space", Operands: []any{pw.Text("DeviceRgb")}},
		{Op: "set_stroke_color_space", Operands: []any{pw.Text("DeviceCmyk")}},
		{Op: "set_fill_color_space", Operands: []any{pw.Text("Pattern")}},
		{Op: "set_fill_color_space", Operands: []any{pw.Name("Cs0")}},
		{Op: "set_stroke_color", Operands: []any{reals(0.1, 0.2, 0.3)}},
		{Op: "set_stroke_pattern", Operands: []any{reals(), pw.Name("P0")}},
		{Op: "set_fill_color", Operands: []any{reals()}},
		{Op: "set_fill_pattern", Operands: []any{reals(0.5), pw.Name("P1")}},
		{Op: "set_stroke_gray", Operands: []any{pw.Real(0.5)}},
		{Op: "set_fill_gray", Operands: []any{pw.Real(1)}},
		{Op: "set_stroke_rgb", Operands: []any{pw.Real(1), pw.Real(0), pw.Real(0.5)}},
		{Op: "set_fill_rgb", Operands: []any{pw.Real(0), pw.Real(1), pw.Real(0.25)}},
		{Op: "set_stroke_cmyk", Operands: []any{pw.Real(0), pw.Real(0.1), pw.Real(0.2), pw.Real(0.3)}},
		{Op: "set_fill_cmyk", Operands: []any{pw.Real(1), pw.Real(0.9), pw.Real(0.8), pw.Real(0.7)}},
		{Op: "shading", Operands: []any{pw.Name("Sh0")}},
		{Op: "x_object", Operands: []any{pw.Name("Im0")}},
		{Op: "marked_content_point", Operands: []any{pw.Name("Mp")}},
		{Op: "marked_content_point_with_properties", Operands: []any{pw.Name("Dp")}},
		{Op: "marked_content_point_with_properties", Operands: []any{pw.Name("Dp"), call("properties_named", pw.Name("Pr0"))}},
		{Op: "begin_marked_content", Operands: []any{pw.Name("Span")}},
		{Op: "end_marked_content"},
		{Op: "begin_marked_content_with_properties", Operands: []any{pw.Name("Span"), call("properties",
			call("identify", pw.Int(7)), call("actual_text", pw.Text("é😀")), call("pair", pw.Name("Lang"), pw.Str([]byte("en"))))}},
		{Op: "end_marked_content"},
		{Op: "begin_marked_content_with_properties", Operands: []any{pw.Name("Artifact"), call("properties",
			call("artifact", call("kind", pw.Name("Pagination")), call("subtype", pw.Text("PageNumber")),
				call("subtype", pw.Name("Mine")), call("bounding_box", reals(0, 0, 612, 792.5)),
				call("attached", pw.Name("Left"), pw.Name("Top"), pw.Name("Right"), pw.Name("Bottom")),
				call("attached"), call("pair", pw.Name("X"), pw.Null)))}},
		{Op: "end_marked_content"},
		{Op: "begin_marked_content_with_properties", Operands: []any{pw.Name("Q"), call("obj", pw.Array())}},
		{Op: "begin_compat"},
		{Op: "end_compat"},
		{Op: "restore_state"},
		{Op: "ext_graphics", Operands: []any{pw.Name("Gs"),
			call("line_width", pw.Real(2)), call("line_cap", pw.Name("ProjectingSquareCap")),
			call("line_join", pw.Name("MiterJoin")), call("miter_limit", pw.Real(10)),
			call("dash_pattern", reals(1, 2, 3), pw.Real(0.5)), call("rendering_intent", pw.Name("AbsoluteColorimetric")),
			call("overprint", pw.Bool(true)), call("overprint_fill", pw.Bool(false)),
			call("overprint_mode", pw.Name("IgnoreZeroChannel")), call("font", pw.Name("F2"), pw.Real(8)),
			call("black_generation", pw.Ref(4)), call("black_generation_default"),
			call("undercolor_removal", pw.Ref(5)), call("undercolor_removal_default"),
			call("transfer", pw.Ref(6)), call("transfer_default"), call("halftone", pw.Ref(7)), call("halftone_default"),
			call("flatness", pw.Real(1.5)), call("smoothness", pw.Real(0.02)), call("stroke_adjustment", pw.Bool(true)),
			call("blend_mode", pw.Name("Luminosity")),
			call("soft_mask", call("subtype", pw.Name("Alpha")), call("group", pw.Ref(9)),
				call("backdrop", reals(0, 0.5, 1)), call("transfer_function", pw.Ref(10)), call("pair", pw.Name("K"), pw.Int(1))),
			call("soft_mask_name", pw.Name("None")), call("stroking_alpha", pw.Real(0.3)),
			call("non_stroking_alpha", pw.Real(0.6)), call("alpha_source", pw.Bool(false)),
			call("text_knockout", pw.Bool(true)), call("pair", pw.Name("Extra"), pw.Dict()),
		}},
		{Op: "resources", Operands: []any{pw.Name("Rs"),
			call("x_objects", call("pair", pw.Name("Im0"), pw.Ref(11))), call("fonts", call("pair", pw.Name("F1"), pw.Ref(12))),
			call("color_spaces"), call("patterns", call("pair", pw.Name("P0"), pw.Ref(13))),
			call("shadings", call("pair", pw.Name("Sh0"), pw.Ref(14))), call("ext_g_states", call("pair", pw.Name("Gs0"), pw.Ref(15))),
			call("proc_sets", pw.Name("PDF"), pw.Name("ImageB")), call("proc_sets_all"),
			call("properties", call("insert", pw.Name("Pr0"), call("identify", pw.Int(1))),
				call("insert", pw.Name("Pr1"), call("artifact", call("kind", pw.Name("Inline"))))),
			call("pair", pw.Name("Extra"), pw.Int(1)),
		}},
		{Op: "Tj", Operands: []any{pw.Str([]byte("generic"))}},
		{Op: "fill_nonzero"},
	}
}

// TestPDFWriterGoContent compares the Go writer with pdf-writer for content
// streams that call each method of Content and of its writers, and for the
// checks of the operands, which panic.
func TestPDFWriterGoContent(t *testing.T) {
	no := false
	every := everyContentCall()
	// A name that neither side knows is a generic operation on both sides,
	// so make sure that the Go side knows each typed call.
	for _, op := range every[:len(every)-2] {
		b, err := json.Marshal(op.Operands)
		if err != nil {
			t.Fatal(err)
		}
		var operands []any
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(&operands); err != nil {
			t.Fatal(err)
		}
		if ok, err := contentCall(pdfwriter.NewContent(), op.Op, operands); !ok || err != nil {
			t.Errorf("the typed call %q gives %v, %v", op.Op, ok, err)
		}
	}
	panics := []writeOp{
		{Op: "set_line_width", Operands: []any{pw.Real(-1)}},
		{Op: "set_flatness", Operands: []any{pw.Int(101)}},
		{Op: "set_flatness", Operands: []any{pw.Int(-1)}},
	}
	questions := []writerQuestion{
		{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{{ID: 1, Content: every}}}},
		{PdfWriter: writerSpec{Chunk: true, Pretty: &no, Objects: []writerObject{{ID: 1, Content: every}}}},
		{PdfWriter: writerSpec{Objects: []writerObject{
			{ID: 1, Content: every},
			{ID: 2, Start: "ext_graphics", Dict: [][2]any{pw.Pair("CA", pw.Real(0.5))}},
		}}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: []writerObject{{ID: 2, Start: "ext_graphics"}}}},
		{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{{ID: 1, Content: []writeOp{{Op: "set_flatness", Operands: []any{pw.Int(100)}}}}}}},
		{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{{ID: 1, Content: []writeOp{{Op: "set_line_width", Operands: []any{pw.Real(0)}}}}}}},
	}
	for _, p := range panics {
		questions = append(questions, writerQuestion{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{
			{ID: 1, Content: []writeOp{{Op: "save_state"}, p}},
		}}})
	}
	compareWriter(t, questions)

	// compareWriter accepts an error of both writers, so make sure that the
	// questions give bytes or panic as expected.
	want := []string{"panic: line width must be positive", "panic: flatness tolerance must be between 0 and 100",
		"panic: flatness tolerance must be between 0 and 100"}
	for i, q := range questions {
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		_, gerr := goWriter(b)
		switch j := i - (len(questions) - len(panics)); {
		case j < 0 && gerr != nil:
			t.Errorf("question %d gives the error %q", i, gerr)
		case j >= 0 && (gerr == nil || gerr.Error() != want[j]):
			t.Errorf("question %d gives the error %v, want %q", i, gerr, want[j])
		}
	}
}

// TestPDFWriterGoContentRandom compares the Go writer with pdf-writer for
// random content streams of typed calls and generic operations.
func TestPDFWriterGoContentRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 4))
	questions := make([]writerQuestion, 300)
	for i := range questions {
		q := writerSpec{Chunk: r.IntN(2) == 0}
		if r.IntN(3) != 0 {
			p := r.IntN(2) == 0
			q.Pretty = &p
		}
		for id := range 1 + r.IntN(3) {
			q.Objects = append(q.Objects, writerObject{ID: id + 1, Content: randomContent(r)})
		}
		if r.IntN(4) == 0 {
			q.Objects = append(q.Objects, writerObject{ID: 9, Start: "ext_graphics", Dict: randomPairs(r, 2)})
		}
		questions[i] = writerQuestion{PdfWriter: q}
	}
	compareWriter(t, questions)

	// compareWriter accepts an error of both writers, so make sure that only
	// the few calls with operands out of range give one.
	errs := 0
	for _, q := range questions {
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := goWriter(b); err != nil {
			if !strings.Contains(err.Error(), "line width") && !strings.Contains(err.Error(), "flatness") {
				t.Errorf("a random question gives the error %q", err)
			}
			errs++
		}
	}
	if errs > len(questions)/10 {
		t.Errorf("%d of %d random questions give an error", errs, len(questions))
	}
}

// randomContent gives a random list of up to 24 operations.
func randomContent(r *rand.Rand) []writeOp {
	var ops []writeOp
	for range r.IntN(25) {
		ops = append(ops, randomContentOp(r))
	}
	return ops
}

// randomContentOp gives a random typed call or generic operation.
func randomContentOp(r *rand.Rand) writeOp {
	re := func() any { return pw.Real(randomReal(r)) }
	nm := func() any { return pw.Name(randomText(r)) }
	st := func() any { return pw.Str(randomBytes(r)) }
	rs := func(n int) any {
		fs := make([]float32, n)
		for i := range fs {
			fs[i] = randomReal(r)
		}
		return reals(fs...)
	}
	pick := func(names ...string) any { return pw.Name(names[r.IntN(len(names))]) }
	op := func(name string, args ...any) writeOp { return writeOp{Op: name, Operands: args} }
	switch r.IntN(30) {
	case 0:
		w := randomReal(r)
		if w < 0 && r.IntN(40) != 0 {
			w = -w
		}
		return op("set_line_width", pw.Real(w))
	case 1:
		n := int32(r.IntN(101))
		if r.IntN(40) == 0 {
			n = int32(r.IntN(300)) - 100
		}
		return op("set_flatness", pw.Int(n))
	case 2:
		return op("set_line_cap", pick("ButtCap", "RoundCap", "ProjectingSquareCap"))
	case 3:
		return op("set_line_join", pick("MiterJoin", "RoundJoin", "BevelJoin"))
	case 4:
		return op("set_text_rendering_mode", pick("Fill", "Stroke", "FillStroke", "Invisible", "FillClip",
			"StrokeClip", "FillStrokeClip", "Clip"))
	case 5:
		return op("set_rendering_intent", pick("AbsoluteColorimetric", "RelativeColorimetric", "Saturation", "Perceptual"))
	case 6:
		return op("set_dash_pattern", rs(r.IntN(4)), re())
	case 7:
		return op([]string{"transform", "set_text_matrix"}[r.IntN(2)], rs(6))
	case 8:
		return op([]string{"move_to", "line_to", "next_line", "next_line_and_set_leading"}[r.IntN(4)], re(), re())
	case 9:
		return op([]string{"cubic_to_initial", "cubic_to_final", "rect", "set_stroke_cmyk", "set_fill_cmyk"}[r.IntN(5)],
			re(), re(), re(), re())
	case 10:
		return op("cubic_to", re(), re(), re(), re(), re(), re())
	case 11:
		return op([]string{"set_stroke_rgb", "set_fill_rgb"}[r.IntN(2)], re(), re(), re())
	case 12:
		return op([]string{"set_miter_limit", "set_char_spacing", "set_word_spacing", "set_horizontal_scaling",
			"set_leading", "set_rise", "set_stroke_gray", "set_fill_gray", "start_color_glyph"}[r.IntN(9)], re())
	case 13:
		return op("start_shape_glyph", re(), re(), re(), re(), re())
	case 14:
		return op([]string{"save_state", "restore_state", "close_path", "stroke", "close_and_stroke", "fill_nonzero",
			"fill_even_odd", "fill_nonzero_and_stroke", "fill_even_odd_and_stroke", "close_fill_nonzero_and_stroke",
			"close_fill_even_odd_and_stroke", "end_path", "clip_nonzero", "clip_even_odd", "begin_text", "end_text",
			"next_line_using_leading", "end_marked_content", "begin_compat", "end_compat"}[r.IntN(20)])
	case 15:
		return op([]string{"set_parameters", "shading", "x_object", "marked_content_point", "begin_marked_content"}[r.IntN(5)], nm())
	case 16:
		return op("set_font", nm(), re())
	case 17:
		return op([]string{"show", "next_line_show"}[r.IntN(2)], st())
	case 18:
		return op("next_line_show_and_set_word_and_char_spacing", re(), re(), st())
	case 19:
		var calls []any
		for range r.IntN(3) {
			if r.IntN(3) == 0 {
				calls = append(calls, call("obj", randomValue(r, 1)))
				continue
			}
			var items []any
			for range r.IntN(6) {
				switch r.IntN(3) {
				case 0:
					items = append(items, call("show", st()))
				case 1:
					items = append(items, call("adjust", re()))
				default:
					items = append(items, call("item", randomValue(r, 1)))
				}
			}
			calls = append(calls, call("items", items...))
		}
		return op("show_positioned", calls...)
	case 20:
		kind := []string{"Gray", "Rgb", "Cmyk"}[r.IntN(3)]
		space := pw.Text("Device" + kind)
		switch r.IntN(3) {
		case 0:
			space = pw.Text("Pattern")
		case 1:
			space = nm()
		}
		return op([]string{"set_stroke_color_space", "set_fill_color_space"}[r.IntN(2)], space)
	case 21:
		return op([]string{"set_stroke_color", "set_fill_color"}[r.IntN(2)], rs(r.IntN(5)))
	case 22:
		return op([]string{"set_stroke_pattern", "set_fill_pattern"}[r.IntN(2)], rs(r.IntN(4)), nm())
	case 23:
		name := []string{"marked_content_point_with_properties", "begin_marked_content_with_properties"}[r.IntN(2)]
		args := []any{nm()}
		for range r.IntN(3) {
			if r.IntN(2) == 0 {
				args = append(args, call("obj", randomValue(r, 1)))
			} else {
				args = append(args, call("properties", randomPropertyCalls(r)...))
			}
		}
		if r.IntN(3) == 0 {
			args = append(args, call("properties_named", nm()))
		}
		return op(name, args...)
	case 24, 25:
		return op("ext_graphics", append([]any{nm()}, randomExtGraphicsCalls(r)...)...)
	case 26:
		return op("resources", append([]any{nm()}, randomResourcesCalls(r)...)...)
	}
	// A generic operation with random operands.
	var operands []any
	for range r.IntN(4) {
		operands = append(operands, randomValue(r, 2))
	}
	return op([]string{"re", "BT", "Tj", "", "x y", "q"}[r.IntN(6)], operands...)
}

// randomPropertyCalls gives random calls on a PropertyList, which can end
// with the call "artifact".
func randomPropertyCalls(r *rand.Rand) []any {
	var calls []any
	for range r.IntN(4) {
		switch r.IntN(3) {
		case 0:
			calls = append(calls, call("identify", pw.Int(int32(r.Uint32())>>r.IntN(32))))
		case 1:
			calls = append(calls, call("actual_text", pw.Text(randomText(r))))
		default:
			calls = append(calls, call("pair", pw.Name(randomText(r)), randomValue(r, 1)))
		}
	}
	if r.IntN(2) == 0 {
		var a []any
		for range r.IntN(5) {
			switch r.IntN(5) {
			case 0:
				a = append(a, call("kind", pw.Name([]string{"Pagination", "Layout", "Page", "Background", "Inline"}[r.IntN(5)])))
			case 1:
				sub := pw.Name(randomText(r))
				if r.IntN(2) == 0 {
					sub = pw.Text([]string{"Header", "Footer", "Watermark", "PageNumber", "Bates", "LineNumber", "Redaction"}[r.IntN(7)])
				}
				a = append(a, call("subtype", sub))
			case 2:
				a = append(a, call("bounding_box", reals(randomReal(r), randomReal(r), randomReal(r), randomReal(r))))
			case 3:
				var names []any
				for range r.IntN(4) {
					names = append(names, pw.Name([]string{"Left", "Top", "Right", "Bottom"}[r.IntN(4)]))
				}
				a = append(a, call("attached", names...))
			default:
				a = append(a, call("pair", pw.Name(randomText(r)), randomValue(r, 1)))
			}
		}
		calls = append(calls, call("artifact", a...))
	}
	return calls
}

// randomExtGraphicsCalls gives random calls on an ExtGraphicsState.
func randomExtGraphicsCalls(r *rand.Rand) []any {
	re := func() any { return pw.Real(randomReal(r)) }
	ref := func() any { return pw.Ref(1 + r.IntN(30)) }
	bo := func() any { return pw.Bool(r.IntN(2) == 0) }
	var calls []any
	for range r.IntN(8) {
		switch r.IntN(14) {
		case 0:
			calls = append(calls, call([]string{"line_width", "miter_limit", "flatness", "smoothness",
				"stroking_alpha", "non_stroking_alpha"}[r.IntN(6)], re()))
		case 1:
			calls = append(calls, call("line_cap", pw.Name([]string{"ButtCap", "RoundCap", "ProjectingSquareCap"}[r.IntN(3)])))
		case 2:
			calls = append(calls, call("line_join", pw.Name([]string{"MiterJoin", "RoundJoin", "BevelJoin"}[r.IntN(3)])))
		case 3:
			fs := make([]float32, r.IntN(4))
			for i := range fs {
				fs[i] = randomReal(r)
			}
			calls = append(calls, call("dash_pattern", reals(fs...), re()))
		case 4:
			calls = append(calls, call("rendering_intent", pw.Name([]string{"AbsoluteColorimetric",
				"RelativeColorimetric", "Saturation", "Perceptual"}[r.IntN(4)])))
		case 5:
			calls = append(calls, call([]string{"overprint", "overprint_fill", "stroke_adjustment", "alpha_source",
				"text_knockout"}[r.IntN(5)], bo()))
		case 6:
			calls = append(calls, call("overprint_mode", pw.Name([]string{"OverrideAllColorants", "IgnoreZeroChannel"}[r.IntN(2)])))
		case 7:
			calls = append(calls, call("font", pw.Name(randomText(r)), re()))
		case 8:
			calls = append(calls, call([]string{"black_generation", "undercolor_removal", "transfer", "halftone"}[r.IntN(4)], ref()))
		case 9:
			calls = append(calls, call([]string{"black_generation_default", "undercolor_removal_default",
				"transfer_default", "halftone_default"}[r.IntN(4)]))
		case 10:
			calls = append(calls, call("blend_mode", pw.Name(blendModes[r.IntN(len(blendModes))].String())))
		case 11:
			var a []any
			for range r.IntN(5) {
				switch r.IntN(5) {
				case 0:
					a = append(a, call("subtype", pw.Name([]string{"Alpha", "Luminosity"}[r.IntN(2)])))
				case 1:
					a = append(a, call([]string{"group", "transfer_function"}[r.IntN(2)], ref()))
				case 2:
					fs := make([]float32, r.IntN(4))
					for i := range fs {
						fs[i] = randomReal(r)
					}
					a = append(a, call("backdrop", reals(fs...)))
				default:
					a = append(a, call("pair", pw.Name(randomText(r)), randomValue(r, 1)))
				}
			}
			calls = append(calls, call("soft_mask", a...))
		case 12:
			calls = append(calls, call("soft_mask_name", pw.Name(randomText(r))))
		default:
			calls = append(calls, call("pair", pw.Name(randomText(r)), randomValue(r, 2)))
		}
	}
	return calls
}

// randomResourcesCalls gives random calls on a Resources.
func randomResourcesCalls(r *rand.Rand) []any {
	var calls []any
	for range r.IntN(6) {
		switch r.IntN(5) {
		case 0:
			var pairs []any
			for range r.IntN(3) {
				pairs = append(pairs, call("pair", pw.Name(randomText(r)), randomValue(r, 1)))
			}
			name := []string{"x_objects", "fonts", "color_spaces", "patterns", "shadings", "ext_g_states"}[r.IntN(6)]
			calls = append(calls, call(name, pairs...))
		case 1:
			var names []any
			for range r.IntN(6) {
				names = append(names, pw.Name([]string{"PDF", "Text", "ImageB", "ImageC", "ImageI"}[r.IntN(5)]))
			}
			calls = append(calls, call("proc_sets", names...))
		case 2:
			calls = append(calls, call("proc_sets_all"))
		case 3:
			var inserts []any
			for range r.IntN(3) {
				inserts = append(inserts, call("insert", append([]any{pw.Name(randomText(r))}, randomPropertyCalls(r)...)...))
			}
			calls = append(calls, call("properties", inserts...))
		default:
			calls = append(calls, call("pair", pw.Name(randomText(r)), randomValue(r, 2)))
		}
	}
	return calls
}
