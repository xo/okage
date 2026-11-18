package krilla

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
)

// The writers of color.go and functions.go against pdf-writer. An object
// whose field "start" begins with "color:" holds a script in JSON after the
// colon. rust/src/writer_color.rs describes the scripts and makes the calls
// with pdf-writer, and goColor makes them with internal/pdfwriter.
//
// The function init makes each script of the tests, and registers its start
// in writerDicts or writerStreams. The tests then use the starts.

// colorPrefix is the prefix of the field "start" of a script.
const colorPrefix = "color:"

// colorScript is a script and the kind of its writer.
type colorScript struct {
	start string
	kind  string
}

// colorStreams holds the writers of a script that write a stream.
var colorStreams = map[string]bool{
	"stream_shading": true, "tiling_pattern": true, "icc_profile": true,
	"sampled_function": true, "post_script_function": true,
}

// colorDerefs holds the writers of a script that deref to a Dict or a
// Stream, so that an object can add the pairs of its field "dict".
var colorDerefs = map[string]bool{
	"function_shading": true, "shading_pattern": true, "exponential_function": true,
	"stitching_function": true, "stream_shading": true, "tiling_pattern": true,
	"icc_profile": true, "sampled_function": true, "post_script_function": true,
}

// colorFixed and colorRandom hold the scripts of TestPDFWriterGoColor and
// TestPDFWriterGoColorRandom.
var colorFixed, colorRandom []colorScript

func init() {
	colorFixed = registerColor(fixedColorScripts())
	r := rand.New(rand.NewPCG(62, 5))
	scripts := make([]any, 400)
	for i := range scripts {
		scripts[i] = randomColorScript(r)
	}
	colorRandom = registerColor(scripts)
}

// registerColor registers each script in writerDicts or writerStreams.
func registerColor(scripts []any) []colorScript {
	out := make([]colorScript, len(scripts))
	for i, s := range scripts {
		b, err := json.Marshal(s)
		if err != nil {
			panic(err)
		}
		start := colorPrefix + string(b)
		kind := as[string](as[[]any](s)[0])
		out[i] = colorScript{start: start, kind: kind}
		if colorStreams[kind] {
			writerStreams[start] = func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
				return as[*pdfwriter.Stream](goColor(c, id, start, data))
			}
		} else {
			writerDicts[start] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
				d, _ := goColor(c, id, start, nil).(*pdfwriter.Dict)
				return d
			}
		}
	}
	return out
}

// goColor makes the calls of a script, as write of writer_color.rs does. It
// returns the *Stream or the *Dict that the pairs of the object go into, or
// nil for a writer that does not deref. A bad script panics.
func goColor(c *pdfwriter.Chunk, id pdfwriter.Ref, start string, data []byte) any {
	dec := json.NewDecoder(bytes.NewReader([]byte(start[len(colorPrefix):])))
	dec.UseNumber()
	var script []any
	if err := dec.Decode(&script); err != nil {
		panic(err)
	}
	calls := script[1]
	switch as[string](script[0]) {
	case "color_space":
		goColorSpace(c.ColorSpace(id), calls)
		return nil
	case "function_shading":
		w := c.FunctionShading(id)
		goColorFunctionShading(w, calls)
		return w.Dict
	case "shading_pattern":
		w := c.ShadingPattern(id)
		goColorShadingPattern(w, calls)
		return w.Dict
	case "exponential_function":
		w := c.ExponentialFunction(id)
		goColorExponential(w, calls)
		return w.Dict
	case "stitching_function":
		w := c.StitchingFunction(id)
		goColorStitching(w, calls)
		return w.Dict
	case "separation_info":
		goColorSeparationInfo(pdfwriter.Start[pdfwriter.SeparationInfo](c.Indirect(id)), calls)
		return nil
	case "output_intent":
		goColorOutputIntent(pdfwriter.Start[pdfwriter.OutputIntent](c.Indirect(id)), calls)
		return nil
	case "stream_shading":
		w := c.StreamShading(id, data)
		goColorStreamShading(w, calls)
		return w.Stream
	case "tiling_pattern":
		w := c.TilingPattern(id, data)
		goColorTilingPattern(w, calls)
		return w.Stream
	case "icc_profile":
		w := c.ICCProfile(id, data)
		goColorICCProfile(w, calls)
		return w.Stream
	case "sampled_function":
		w := c.SampledFunction(id, data)
		goColorSampled(w, calls)
		return w.Stream
	case "post_script_function":
		if len(script) > 2 {
			data = pdfwriter.PostScriptOpEncode(goColorPostScriptOps(script[2])).IntoVec()
		}
		w := c.PostScriptFunction(id, data)
		goColorPostScript(w, calls)
		return w.Stream
	}
	panic(fmt.Sprintf("the script %s is not known", start))
}

// goColorSpace makes the one call of a color space.
func goColorSpace(cs *pdfwriter.ColorSpace, call any) {
	if call == nil {
		return
	}
	name, a := colorCall(call)
	switch name {
	case "cal_rgb":
		cs.CalRGB(colF3(a[0]), colOptional(a[1], colF3), colOptional(a[2], colF3), colOptional(a[3], colF9))
	case "cal_gray":
		cs.CalGray(colF3(a[0]), colOptional(a[1], colF3), colOptional(a[2], colF))
	case "lab":
		cs.Lab(colF3(a[0]), colOptional(a[1], colF3), colOptional(a[2], colF4))
	case "icc_based":
		cs.ICCBased(colRef(a[0]))
	case "srgb":
		cs.SRGB()
	case "adobe_rgb":
		cs.AdobeRGB()
	case "display_p3":
		cs.DisplayP3()
	case "pro_photo":
		cs.ProPhoto()
	case "eci_rgb":
		cs.ECIRGB()
	case "ntsc":
		cs.NTSC()
	case "pal":
		cs.PAL()
	case "d65_gray":
		cs.D65Gray()
	case "d50_gray":
		cs.D50Gray(colOptional(a[0], colF))
	case "c_gray":
		cs.CGray()
	case "e_gray":
		cs.EGray(colOptional(a[0], colF))
	case "device_rgb":
		cs.DeviceRGB()
	case "device_cmyk":
		cs.DeviceCMYK()
	case "device_gray":
		cs.DeviceGray()
	case "separation":
		w := cs.Separation(pdfwriter.Name(as[string](a[0])))
		goColorSeparation(w, a[1])
		w.Finish()
	case "device_n":
		w := cs.DeviceN(colNames(a[0])...)
		goColorDeviceN(w, a[1])
		w.Finish()
	case "indexed":
		lookup, err := colHex(as[string](a[2]))
		if err != nil {
			panic(err)
		}
		cs.Indexed(pdfwriter.Name(as[string](a[0])), colI(a[1]), lookup)
	case "pattern":
		cs.Pattern(pdfwriter.Name(as[string](a[0])))
	default:
		panic(fmt.Sprintf("the call %v of ColorSpace is not known", call))
	}
}

func goColorSeparation(w *pdfwriter.Separation, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "alternate_color_space":
			goColorSpace(w.AlternateColorSpace(), a[0])
		case "alternate_color_space_ref":
			w.AlternateColorSpaceRef(colRef(a[0]))
		case "tint_exponential":
			f := w.TintExponential()
			goColorExponential(f, a[0])
			f.Finish()
		case "tint_stitching":
			f := w.TintStitching()
			goColorStitching(f, a[0])
			f.Finish()
		case "tint_ref":
			w.TintRef(colRef(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of Separation is not known", call))
		}
	}
}

func goColorDeviceN(w *pdfwriter.DeviceN, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "alternate_color_space":
			goColorSpace(w.AlternateColorSpace(), a[0])
		case "alternate_color_space_ref":
			w.AlternateColorSpaceRef(colRef(a[0]))
		case "tint_exponential":
			f := w.TintExponential()
			goColorExponential(f, a[0])
			f.Finish()
		case "tint_stitching":
			f := w.TintStitching()
			goColorStitching(f, a[0])
			f.Finish()
		case "tint_ref":
			w.TintRef(colRef(a[0]))
		case "attrs":
			at := w.Attrs()
			goColorAttrs(at, a[0])
			at.Finish()
		default:
			panic(fmt.Sprintf("the call %v of DeviceN is not known", call))
		}
	}
}

func goColorAttrs(w *pdfwriter.DeviceNAttrs, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "subtype":
			switch s := as[string](a[0]); s {
			case "DeviceN":
				w.Subtype(pdfwriter.DeviceNSubtypeDeviceN)
			case "NChannel":
				w.Subtype(pdfwriter.DeviceNSubtypeNChannel)
			default:
				panic(fmt.Sprintf("the subtype %q is not known", s))
			}
		case "colorants":
			d := w.Colorants()
			for _, entry := range as[[]any](a[0]) {
				e := as[[]any](entry)
				if err := writePairs(d.Insert(pdfwriter.Name(as[string](e[0]))), e[1]); err != nil {
					panic(err)
				}
			}
			d.Finish()
		case "process":
			p := w.Process()
			goColorProcess(p, a[0])
			p.Finish()
		case "mixing_hints":
			h := w.MixingHints()
			goColorMixingHints(h, a[0])
			h.Finish()
		default:
			panic(fmt.Sprintf("the call %v of DeviceNAttrs is not known", call))
		}
	}
}

func goColorProcess(w *pdfwriter.DeviceNProcess, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "color_space":
			w.ColorSpace(pdfwriter.Name(as[string](a[0])))
		case "color_space_array":
			goColorSpace(w.ColorSpaceArray(), a[0])
		case "components":
			w.Components(colNames(a[0])...)
		default:
			panic(fmt.Sprintf("the call %v of DeviceNProcess is not known", call))
		}
	}
}

func goColorMixingHints(w *pdfwriter.DeviceNMixingHints, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "solidities", "dot_gain":
			var d *pdfwriter.TypedDict[pdfwriter.Real]
			if name == "solidities" {
				d = w.Solidities()
			} else {
				d = w.DotGain()
			}
			for _, entry := range as[[]any](a[0]) {
				e := as[[]any](entry)
				d.Pair(pdfwriter.Name(as[string](e[0])), pdfwriter.Real(colF(e[1])))
			}
			d.Finish()
		case "printing_order":
			w.PrintingOrder(colNames(a[0])...)
		default:
			panic(fmt.Sprintf("the call %v of DeviceNMixingHints is not known", call))
		}
	}
}

func goColorTilingPattern(w *pdfwriter.TilingPattern, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "paint_type":
			switch s := as[string](a[0]); s {
			case "Colored":
				w.PaintType(pdfwriter.PaintTypeColored)
			case "Uncolored":
				w.PaintType(pdfwriter.PaintTypeUncolored)
			default:
				panic(fmt.Sprintf("the paint type %q is not known", s))
			}
		case "tiling_type":
			switch s := as[string](a[0]); s {
			case "ConstantSpacing":
				w.TilingType(pdfwriter.TilingTypeConstantSpacing)
			case "NoDistortion":
				w.TilingType(pdfwriter.TilingTypeNoDistortion)
			case "FastConstantSpacing":
				w.TilingType(pdfwriter.TilingTypeFastConstantSpacing)
			default:
				panic(fmt.Sprintf("the tiling type %q is not known", s))
			}
		case "bbox":
			w.BBox(colRect(a[0]))
		case "x_step":
			w.XStep(colF(a[0]))
		case "y_step":
			w.YStep(colF(a[0]))
		case "matrix":
			w.Matrix(colF6(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of TilingPattern is not known", call))
		}
	}
}

func goColorShadingPattern(w *pdfwriter.ShadingPattern, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "function_shading":
			s := w.FunctionShading()
			goColorFunctionShading(s, a[0])
			s.Finish()
		case "shading_ref":
			w.ShadingRef(colRef(a[0]))
		case "matrix":
			w.Matrix(colF6(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of ShadingPattern is not known", call))
		}
	}
}

func goColorFunctionShading(w *pdfwriter.FunctionShading, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "shading_type":
			switch s := as[string](a[0]); s {
			case "Function":
				w.ShadingType(pdfwriter.FunctionShadingTypeFunction)
			case "Axial":
				w.ShadingType(pdfwriter.FunctionShadingTypeAxial)
			case "Radial":
				w.ShadingType(pdfwriter.FunctionShadingTypeRadial)
			default:
				panic(fmt.Sprintf("the shading type %q is not known", s))
			}
		case "color_space":
			goColorSpace(w.ColorSpace(), a[0])
		case "background":
			w.Background(colFs(a[0])...)
		case "bbox":
			w.BBox(colRect(a[0]))
		case "anti_alias":
			w.AntiAlias(as[bool](a[0]))
		case "domain":
			w.Domain(colF4(a[0]))
		case "matrix":
			w.Matrix(colF6(a[0]))
		case "function":
			w.Function(colRef(a[0]))
		case "coords":
			w.Coords(colFs(a[0])...)
		case "extend":
			e := as[[]any](a[0])
			w.Extend([2]bool{as[bool](e[0]), as[bool](e[1])})
		default:
			panic(fmt.Sprintf("the call %v of FunctionShading is not known", call))
		}
	}
}

func goColorStreamShading(w *pdfwriter.StreamShading, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "shading_type":
			switch s := as[string](a[0]); s {
			case "FreeformGouraud":
				w.ShadingType(pdfwriter.StreamShadingTypeFreeformGouraud)
			case "LatticeGouraud":
				w.ShadingType(pdfwriter.StreamShadingTypeLatticeGouraud)
			case "CoonsPatch":
				w.ShadingType(pdfwriter.StreamShadingTypeCoonsPatch)
			case "TensorProductPatch":
				w.ShadingType(pdfwriter.StreamShadingTypeTensorProductPatch)
			default:
				panic(fmt.Sprintf("the shading type %q is not known", s))
			}
		case "color_space":
			goColorSpace(w.ColorSpace(), a[0])
		case "background":
			w.Background(colFs(a[0])...)
		case "bbox":
			w.BBox(colRect(a[0]))
		case "anti_alias":
			w.AntiAlias(as[bool](a[0]))
		case "function":
			w.Function(colRef(a[0]))
		case "bits_per_coordinate":
			w.BitsPerCoordinate(colI(a[0]))
		case "bits_per_component":
			w.BitsPerComponent(colI(a[0]))
		case "bits_per_flag":
			w.BitsPerFlag(colI(a[0]))
		case "decode":
			w.Decode(colFs(a[0])...)
		case "vertices_per_row":
			w.VerticesPerRow(colI(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of StreamShading is not known", call))
		}
	}
}

func goColorICCProfile(w *pdfwriter.ICCProfile, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "n":
			w.N(colI(a[0]))
		case "alternate":
			goColorSpace(w.Alternate(), a[0])
		case "alternate_name":
			w.AlternateName(pdfwriter.Name(as[string](a[0])))
		case "range":
			w.Range(colFs(a[0])...)
		case "metadata":
			w.Metadata(colRef(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of ICCProfile is not known", call))
		}
	}
}

func goColorSeparationInfo(w *pdfwriter.SeparationInfo, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "pages":
			w.Pages(colRefs(a[0])...)
		case "device_colorant":
			w.DeviceColorant(pdfwriter.Name(as[string](a[0])))
		case "device_colorant_str":
			w.DeviceColorantStr(as[string](a[0]))
		case "color_space":
			goColorSpace(w.ColorSpace(), a[0])
		default:
			panic(fmt.Sprintf("the call %v of SeparationInfo is not known", call))
		}
	}
}

func goColorOutputIntent(w *pdfwriter.OutputIntent, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "subtype":
			switch s := as[string](a[0]); s {
			case "PDFX":
				w.Subtype(pdfwriter.OutputIntentSubtypePDFX)
			case "PDFA":
				w.Subtype(pdfwriter.OutputIntentSubtypePDFA)
			case "PDFE":
				w.Subtype(pdfwriter.OutputIntentSubtypePDFE)
			case "Custom":
				w.Subtype(pdfwriter.OutputIntentSubtypeCustom(pdfwriter.Name(as[string](a[1]))))
			default:
				panic(fmt.Sprintf("the subtype %q is not known", s))
			}
		case "output_condition":
			w.OutputCondition(pdfwriter.TextStr(as[string](a[0])))
		case "output_condition_identifier":
			w.OutputConditionIdentifier(pdfwriter.TextStr(as[string](a[0])))
		case "registry_name":
			w.RegistryName(pdfwriter.TextStr(as[string](a[0])))
		case "info":
			w.Info(pdfwriter.TextStr(as[string](a[0])))
		case "dest_output_profile":
			w.DestOutputProfile(colRef(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of OutputIntent is not known", call))
		}
	}
}

func goColorSampled(w *pdfwriter.SampledFunction, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "domain":
			w.Domain(colFs(a[0])...)
		case "range":
			w.Range(colFs(a[0])...)
		case "size":
			var size []int32
			for _, v := range as[[]any](a[0]) {
				size = append(size, colI(v))
			}
			w.Size(size...)
		case "bits_per_sample":
			w.BitsPerSample(colI(a[0]))
		case "order":
			switch s := as[string](a[0]); s {
			case "Linear":
				w.Order(pdfwriter.InterpolationOrderLinear)
			case "Cubic":
				w.Order(pdfwriter.InterpolationOrderCubic)
			default:
				panic(fmt.Sprintf("the order %q is not known", s))
			}
		case "encode":
			w.Encode(colFs(a[0])...)
		case "decode":
			w.Decode(colFs(a[0])...)
		default:
			panic(fmt.Sprintf("the call %v of SampledFunction is not known", call))
		}
	}
}

func goColorExponential(w *pdfwriter.ExponentialFunction, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "domain":
			w.Domain(colFs(a[0])...)
		case "range":
			w.Range(colFs(a[0])...)
		case "c0":
			w.C0(colFs(a[0])...)
		case "c1":
			w.C1(colFs(a[0])...)
		case "n":
			w.N(colF(a[0]))
		default:
			panic(fmt.Sprintf("the call %v of ExponentialFunction is not known", call))
		}
	}
}

func goColorStitching(w *pdfwriter.StitchingFunction, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "domain":
			w.Domain(colFs(a[0])...)
		case "range":
			w.Range(colFs(a[0])...)
		case "functions":
			w.Functions(colRefs(a[0])...)
		case "bounds":
			w.Bounds(colFs(a[0])...)
		case "encode":
			w.Encode(colFs(a[0])...)
		default:
			panic(fmt.Sprintf("the call %v of StitchingFunction is not known", call))
		}
	}
}

func goColorPostScript(w *pdfwriter.PostScriptFunction, calls any) {
	for _, call := range as[[]any](calls) {
		name, a := colorCall(call)
		switch name {
		case "domain":
			w.Domain(colFs(a[0])...)
		case "range":
			w.Range(colFs(a[0])...)
		default:
			panic(fmt.Sprintf("the call %v of PostScriptFunction is not known", call))
		}
	}
}

// colorPostScriptOps holds the PostScript operators without data, by name.
var colorPostScriptOps = map[string]pdfwriter.PostScriptOp{
	"abs": pdfwriter.PostScriptOpAbs, "add": pdfwriter.PostScriptOpAdd, "atan": pdfwriter.PostScriptOpAtan,
	"ceiling": pdfwriter.PostScriptOpCeiling, "cos": pdfwriter.PostScriptOpCos, "cvi": pdfwriter.PostScriptOpCvi,
	"cvr": pdfwriter.PostScriptOpCvr, "div": pdfwriter.PostScriptOpDiv, "exp": pdfwriter.PostScriptOpExp,
	"floor": pdfwriter.PostScriptOpFloor, "idiv": pdfwriter.PostScriptOpIdiv, "ln": pdfwriter.PostScriptOpLn,
	"log": pdfwriter.PostScriptOpLog, "mod": pdfwriter.PostScriptOpMod, "mul": pdfwriter.PostScriptOpMul,
	"neg": pdfwriter.PostScriptOpNeg, "round": pdfwriter.PostScriptOpRound, "sin": pdfwriter.PostScriptOpSin,
	"sqrt": pdfwriter.PostScriptOpSqrt, "sub": pdfwriter.PostScriptOpSub, "truncate": pdfwriter.PostScriptOpTruncate,
	"and": pdfwriter.PostScriptOpAnd, "bitshift": pdfwriter.PostScriptOpBitshift, "eq": pdfwriter.PostScriptOpEq,
	"false": pdfwriter.PostScriptOpFalse, "ge": pdfwriter.PostScriptOpGe, "gt": pdfwriter.PostScriptOpGt,
	"le": pdfwriter.PostScriptOpLe, "lt": pdfwriter.PostScriptOpLt, "ne": pdfwriter.PostScriptOpNe,
	"not": pdfwriter.PostScriptOpNot, "or": pdfwriter.PostScriptOpOr, "true": pdfwriter.PostScriptOpTrue,
	"xor": pdfwriter.PostScriptOpXor, "copy": pdfwriter.PostScriptOpCopy, "dup": pdfwriter.PostScriptOpDup,
	"exch": pdfwriter.PostScriptOpExch, "index": pdfwriter.PostScriptOpIndex, "pop": pdfwriter.PostScriptOpPop,
	"roll": pdfwriter.PostScriptOpRoll,
}

// goColorPostScriptOps reads a list of PostScript operators, as ps_ops of
// writer_color.rs does.
func goColorPostScriptOps(v any) []pdfwriter.PostScriptOp {
	var ops []pdfwriter.PostScriptOp
	for _, op := range as[[]any](v) {
		switch op := op.(type) {
		case string:
			o, ok := colorPostScriptOps[op]
			if !ok {
				panic(fmt.Sprintf("the PostScript operator %q is not known", op))
			}
			ops = append(ops, o)
		case map[string]any:
			for k, x := range op {
				switch k {
				case "real":
					ops = append(ops, pdfwriter.PostScriptOpReal(colF(x)))
				case "int":
					ops = append(ops, pdfwriter.PostScriptOpInteger(colI(x)))
				case "if":
					ops = append(ops, pdfwriter.PostScriptOpIf(goColorPostScriptOps(x)))
				case "ifelse":
					both := as[[]any](x)
					ops = append(ops, pdfwriter.PostScriptOpIfElse(goColorPostScriptOps(both[0]), goColorPostScriptOps(both[1])))
				default:
					panic(fmt.Sprintf("the PostScript operator %v is not known", op))
				}
			}
		default:
			panic(fmt.Sprintf("the PostScript operator %v is not known", op))
		}
	}
	return ops
}

// colorCall splits a call into its name and its arguments.
func colorCall(call any) (string, []any) {
	c := as[[]any](call)
	return as[string](c[0]), c[1:]
}

// colF reads an f32, as f of writer_color.rs does.
func colF(v any) float32 {
	f, err := strconv.ParseFloat(string(as[json.Number](v)), 64)
	if err != nil {
		panic(err)
	}
	return float32(f)
}

// colI reads an i32, as i of writer_color.rs does.
func colI(v any) int32 {
	return int32(num(v))
}

func colFs(v any) []float32 {
	var out []float32
	for _, x := range as[[]any](v) {
		out = append(out, colF(x))
	}
	return out
}

func colF3(v any) [3]float32 { return [3]float32(colFs(v)) }
func colF4(v any) [4]float32 { return [4]float32(colFs(v)) }
func colF6(v any) [6]float32 { return [6]float32(colFs(v)) }
func colF9(v any) [9]float32 { return [9]float32(colFs(v)) }

func colRect(v any) pdfwriter.Rect {
	r := colF4(v)
	return pdfwriter.NewRect(r[0], r[1], r[2], r[3])
}

func colRef(v any) pdfwriter.Ref {
	r, err := reference(v)
	if err != nil {
		panic(err)
	}
	return r
}

func colRefs(v any) []pdfwriter.Ref {
	var out []pdfwriter.Ref
	for _, x := range as[[]any](v) {
		out = append(out, colRef(x))
	}
	return out
}

func colNames(v any) []pdfwriter.Name {
	var out []pdfwriter.Name
	for _, x := range as[[]any](v) {
		out = append(out, pdfwriter.Name(as[string](x)))
	}
	return out
}

// colOptional reads a value that can be null, as opt of writer_color.rs does.
func colOptional[T any](v any, read func(any) T) *T {
	if v == nil {
		return nil
	}
	x := read(v)
	return &x
}

// colHex decodes a string of hex, as hex of writer.rs does.
func colHex(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := range b {
		n, err := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		if err != nil {
			return nil, err
		}
		b[i] = byte(n)
	}
	return b, nil
}

// The scripts.

// cCall makes a call of a script.
func cCall(name string, args ...any) []any {
	return append([]any{name}, args...)
}

// cCalls makes the list of calls of a writer.
func cCalls(cs ...[]any) []any {
	out := make([]any, len(cs))
	for i, c := range cs {
		out[i] = c
	}
	return out
}

// cFl makes a list of numbers. Each float32 goes into the JSON as a float64,
// which serde reads exactly and rounds back to the float32.
func cFl(xs ...float32) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = float64(x)
	}
	return out
}

// cStrs makes a list of strings.
func cStrs(xs ...string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// fixedColorScripts holds a script for each call of each writer.
func fixedColorScripts() []any {
	wp := cFl(0.9505, 1, 1.0888)
	exp := cCalls(cCall("domain", cFl(0, 1)), cCall("range", cFl(0, 1, 0, 1)), cCall("c0", cFl(1, 1, 1)),
		cCall("c1", cFl(0, 0.5, 0)), cCall("n", 1.5))
	stitch := cCalls(cCall("domain", cFl(0, 1)), cCall("functions", []any{3, 4}), cCall("bounds", cFl(0.5)),
		cCall("encode", cFl(0, 1, 0, 1)), cCall("range", cFl(0, 1)))
	var scripts []any
	for _, cs := range []any{
		nil,
		cCall("cal_rgb", wp, nil, nil, nil),
		cCall("cal_rgb", wp, cFl(0, 0, 0), cFl(2.2, 2.2, 2.2), cFl(1, 0, 0, 0, 1, 0, 0, 0, 1)),
		cCall("cal_gray", wp, nil, nil),
		cCall("cal_gray", wp, cFl(0.1, 0.2, 0.3), 1.8),
		cCall("lab", wp, nil, nil),
		cCall("lab", wp, cFl(0, 0, 0), cFl(-100, 100, -100, 100)),
		cCall("icc_based", 7),
		cCall("srgb"), cCall("adobe_rgb"), cCall("display_p3"), cCall("pro_photo"), cCall("eci_rgb"),
		cCall("ntsc"), cCall("pal"), cCall("d65_gray"), cCall("d50_gray", nil), cCall("d50_gray", 1.8),
		cCall("c_gray"), cCall("e_gray", nil), cCall("e_gray", 2.2),
		cCall("device_rgb"), cCall("device_cmyk"), cCall("device_gray"),
		cCall("separation", "Spot Color", cCalls(
			cCall("alternate_color_space", cCall("srgb")),
			cCall("tint_exponential", exp))),
		cCall("separation", "PANTONE", cCalls(
			cCall("alternate_color_space_ref", 4),
			cCall("tint_stitching", stitch))),
		cCall("separation", "All", cCalls(cCall("alternate_color_space_ref", 4), cCall("tint_ref", 9))),
		cCall("device_n", cStrs("Cyan", "Spot#1"), cCalls(
			cCall("alternate_color_space", cCall("device_cmyk")),
			cCall("tint_ref", 5),
			cCall("attrs", cCalls(
				cCall("subtype", "NChannel"),
				cCall("colorants", []any{
					[]any{"Spot#1", [][2]any{pw.Pair("A", pw.Array(pw.Name("Separation"), pw.Name("Spot#1")))}},
					[]any{"Empty", [][2]any{}},
				}),
				cCall("process", cCalls(
					cCall("color_space", "DeviceCMYK"),
					cCall("color_space_array", cCall("cal_gray", wp, nil, 2.2)),
					cCall("components", cStrs("Cyan", "Magenta")))),
				cCall("mixing_hints", cCalls(
					cCall("solidities", []any{[]any{"Cyan", 0.5}, []any{"Spot#1", 1}}),
					cCall("printing_order", cStrs("Cyan", "Spot#1")),
					cCall("dot_gain", []any{[]any{"Cyan", 0.25}}))),
				cCall("subtype", "DeviceN"))))),
		cCall("device_n", cStrs(), cCalls(
			cCall("alternate_color_space_ref", 4),
			cCall("tint_exponential", exp),
			cCall("attrs", cCalls()))),
		cCall("device_n", cStrs("A"), cCalls(cCall("alternate_color_space_ref", 4), cCall("tint_stitching", stitch))),
		cCall("indexed", "DeviceRGB", 1, "ff000000ff00"),
		cCall("pattern", "DeviceRGB"),
		// Errors of order panic with the messages of upstream.
		cCall("separation", "S", cCalls(cCall("tint_ref", 1))),
		cCall("separation", "S", cCalls(cCall("tint_exponential", exp))),
		cCall("separation", "S", cCalls(cCall("alternate_color_space_ref", 1), cCall("alternate_color_space", nil))),
		cCall("device_n", cStrs("A"), cCalls(cCall("attrs", cCalls()))),
		cCall("device_n", cStrs("A"), cCalls(cCall("alternate_color_space_ref", 1), cCall("attrs", cCalls()))),
		cCall("device_n", cStrs("A"), cCalls(cCall("alternate_color_space_ref", 1), cCall("tint_ref", 2), cCall("tint_stitching", stitch))),
	} {
		scripts = append(scripts, cCall("color_space", cs))
	}
	fs := cCalls(
		cCall("shading_type", "Axial"),
		cCall("color_space", cCall("srgb")),
		cCall("background", cFl(1, 0.5, 0)),
		cCall("bbox", cFl(0, 0, 100, 200.5)),
		cCall("anti_alias", true),
		cCall("domain", cFl(0, 1, 0, 1)),
		cCall("matrix", cFl(1, 0, 0, 1, 10, 20)),
		cCall("function", 3),
		cCall("coords", cFl(0, 0, 1, 1)),
		cCall("extend", []any{true, false}),
	)
	scripts = append(scripts,
		cCall("function_shading", fs),
		cCall("function_shading", cCalls(cCall("shading_type", "Function"), cCall("shading_type", "Radial"))),
		cCall("shading_pattern", cCalls(cCall("function_shading", fs), cCall("matrix", cFl(2, 0, 0, 2, 0, 0)))),
		cCall("shading_pattern", cCalls(cCall("shading_ref", 8))),
		cCall("exponential_function", exp),
		cCall("stitching_function", stitch),
		cCall("separation_info", cCalls(
			cCall("pages", []any{1, 2}),
			cCall("device_colorant", "Cyan"),
			cCall("device_colorant_str", "Cyän"),
			cCall("color_space", cCall("separation", "Cyan", cCalls(
				cCall("alternate_color_space", cCall("device_cmyk")), cCall("tint_ref", 3)))))),
		cCall("output_intent", cCalls(
			cCall("subtype", "PDFA"),
			cCall("output_condition", "sRGB"),
			cCall("output_condition_identifier", "sRGB IEC61966-2.1"),
			cCall("registry_name", "http://www.color.org"),
			cCall("info", "Info ü"),
			cCall("dest_output_profile", 5))),
		cCall("output_intent", cCalls(cCall("subtype", "PDFX"), cCall("subtype", "PDFE"), cCall("subtype", "Custom", "My Name"))),
		cCall("stream_shading", cCalls(
			cCall("shading_type", "FreeformGouraud"), cCall("shading_type", "LatticeGouraud"),
			cCall("shading_type", "CoonsPatch"), cCall("shading_type", "TensorProductPatch"),
			cCall("color_space", cCall("device_rgb")),
			cCall("background", cFl(0.25)),
			cCall("bbox", cFl(-1, -2, 3, 4)),
			cCall("anti_alias", false),
			cCall("function", 2),
			cCall("bits_per_coordinate", 32),
			cCall("bits_per_component", 8),
			cCall("bits_per_flag", 2),
			cCall("decode", cFl(0, 100, 0, 100, 0, 1)),
			cCall("vertices_per_row", 3))),
		cCall("tiling_pattern", cCalls(
			cCall("tiling_type", "ConstantSpacing"),
			cCall("paint_type", "Colored"),
			cCall("bbox", cFl(0, 0, 10, 10)),
			cCall("matrix", cFl(1, 0, 0, 1, 0, 0)),
			cCall("x_step", 10),
			cCall("y_step", 10.5))),
		cCall("tiling_pattern", cCalls(
			cCall("tiling_type", "NoDistortion"), cCall("tiling_type", "FastConstantSpacing"),
			cCall("paint_type", "Uncolored"))),
		cCall("tiling_pattern", cCalls(cCall("x_step", 0))),
		cCall("tiling_pattern", cCalls(cCall("x_step", 1), cCall("y_step", 0))),
		cCall("icc_profile", cCalls(
			cCall("n", 3),
			cCall("alternate", cCall("device_rgb")),
			cCall("alternate_name", "CS0"),
			cCall("range", cFl(0, 1, 0, 1, 0, 1)),
			cCall("metadata", 6))),
		cCall("icc_profile", cCalls(cCall("n", 1), cCall("n", 4))),
		cCall("icc_profile", cCalls(cCall("n", 2))),
		cCall("sampled_function", cCalls(
			cCall("domain", cFl(0, 1)),
			cCall("range", cFl(0, 1, 0, 1, 0, 1)),
			cCall("size", []any{2, 4}),
			cCall("bits_per_sample", 8),
			cCall("order", "Linear"),
			cCall("order", "Cubic"),
			cCall("encode", cFl(0, 1)),
			cCall("decode", cFl(0, 1, 0, 1, 0, 1)))),
		cCall("post_script_function", cCalls(cCall("domain", cFl(0, 1)), cCall("range", cFl(0, 1, 0, 1))),
			[]any{map[string]any{"real": 3.0}, map[string]any{"real": 2.0}, "mul", "exch", "dup",
				map[string]any{"real": 0.0}, "ge",
				map[string]any{"ifelse": []any{[]any{map[string]any{"real": 1.0}, "add"}, []any{"neg"}}}, "add"}),
		cCall("post_script_function", cCalls(),
			append(cStrs("abs", "add", "atan", "ceiling", "cos", "cvi", "cvr", "div", "exp", "floor", "idiv",
				"ln", "log", "mod", "mul", "neg", "round", "sin", "sqrt", "sub", "truncate", "and", "bitshift",
				"eq", "false", "ge", "gt", "le", "lt", "ne", "not", "or", "true", "xor", "copy", "dup", "exch",
				"index", "pop", "roll"),
				map[string]any{"int": -7}, map[string]any{"real": 1e-7}, map[string]any{"real": 4.2e13},
				map[string]any{"if": []any{}}, map[string]any{"if": []any{"dup"}},
				map[string]any{"ifelse": []any{[]any{}, []any{map[string]any{"if": []any{"pop", "pop"}}}}})),
		cCall("post_script_function", cCalls(), []any{}),
		cCall("post_script_function", cCalls(), []any{"dup"}),
		cCall("post_script_function", cCalls(cCall("domain", cFl(0, 1)))),
	)
	return scripts
}

// randomColorScript makes a random script.
func randomColorScript(r *rand.Rand) any {
	switch r.IntN(14) {
	case 0, 1:
		return cCall("color_space", randomColorSpace(r, 2))
	case 2:
		return cCall("function_shading", randomColorFunctionShading(r, 2))
	case 3:
		var cs []any
		for range r.IntN(4) {
			switch r.IntN(3) {
			case 0:
				cs = append(cs, cCall("function_shading", randomColorFunctionShading(r, 1)))
			case 1:
				cs = append(cs, cCall("shading_ref", 1+r.IntN(30)))
			default:
				cs = append(cs, cCall("matrix", randomColorFloats(r, 6)))
			}
		}
		return cCall("shading_pattern", cList(cs))
	case 4:
		return cCall("exponential_function", randomColorExponential(r))
	case 5:
		return cCall("stitching_function", randomColorStitching(r))
	case 6:
		var cs []any
		for range r.IntN(5) {
			switch r.IntN(4) {
			case 0:
				cs = append(cs, cCall("pages", randomColorRefs(r)))
			case 1:
				cs = append(cs, cCall("device_colorant", randomText(r)))
			case 2:
				cs = append(cs, cCall("device_colorant_str", randomText(r)))
			default:
				cs = append(cs, cCall("color_space", randomColorSpace(r, 1)))
			}
		}
		return cCall("separation_info", cList(cs))
	case 7:
		var cs []any
		for range r.IntN(6) {
			switch r.IntN(6) {
			case 0:
				kinds := []string{"PDFX", "PDFA", "PDFE", "Custom"}
				cs = append(cs, cCall("subtype", kinds[r.IntN(4)], randomText(r)))
			case 1:
				cs = append(cs, cCall("output_condition", randomText(r)))
			case 2:
				cs = append(cs, cCall("output_condition_identifier", randomText(r)))
			case 3:
				cs = append(cs, cCall("registry_name", randomText(r)))
			case 4:
				cs = append(cs, cCall("info", randomText(r)))
			default:
				cs = append(cs, cCall("dest_output_profile", 1+r.IntN(30)))
			}
		}
		return cCall("output_intent", cList(cs))
	case 8:
		var cs []any
		for range r.IntN(8) {
			switch r.IntN(11) {
			case 0:
				kinds := []string{"FreeformGouraud", "LatticeGouraud", "CoonsPatch", "TensorProductPatch"}
				cs = append(cs, cCall("shading_type", kinds[r.IntN(4)]))
			case 1:
				cs = append(cs, cCall("color_space", randomColorSpace(r, 1)))
			case 2:
				cs = append(cs, cCall("background", randomColorFloats(r, r.IntN(5))))
			case 3:
				cs = append(cs, cCall("bbox", randomColorFloats(r, 4)))
			case 4:
				cs = append(cs, cCall("anti_alias", r.IntN(2) == 0))
			case 5:
				cs = append(cs, cCall("function", 1+r.IntN(30)))
			case 6:
				cs = append(cs, cCall("bits_per_coordinate", randomColorInt(r)))
			case 7:
				cs = append(cs, cCall("bits_per_component", randomColorInt(r)))
			case 8:
				cs = append(cs, cCall("bits_per_flag", randomColorInt(r)))
			case 9:
				cs = append(cs, cCall("decode", randomColorFloats(r, r.IntN(7))))
			default:
				cs = append(cs, cCall("vertices_per_row", randomColorInt(r)))
			}
		}
		return cCall("stream_shading", cList(cs))
	case 9:
		var cs []any
		for range r.IntN(7) {
			switch r.IntN(6) {
			case 0:
				cs = append(cs, cCall("paint_type", []string{"Colored", "Uncolored"}[r.IntN(2)]))
			case 1:
				cs = append(cs, cCall("tiling_type", []string{"ConstantSpacing", "NoDistortion", "FastConstantSpacing"}[r.IntN(3)]))
			case 2:
				cs = append(cs, cCall("bbox", randomColorFloats(r, 4)))
			case 3:
				cs = append(cs, cCall("x_step", randomColorStep(r)))
			case 4:
				cs = append(cs, cCall("y_step", randomColorStep(r)))
			default:
				cs = append(cs, cCall("matrix", randomColorFloats(r, 6)))
			}
		}
		return cCall("tiling_pattern", cList(cs))
	case 10:
		var cs []any
		for range r.IntN(6) {
			switch r.IntN(5) {
			case 0:
				cs = append(cs, cCall("n", []int{1, 3, 4, 1, 3, 4, 0, 2}[r.IntN(8)]))
			case 1:
				cs = append(cs, cCall("alternate", randomColorSpace(r, 1)))
			case 2:
				cs = append(cs, cCall("alternate_name", randomText(r)))
			case 3:
				cs = append(cs, cCall("range", randomColorFloats(r, r.IntN(9))))
			default:
				cs = append(cs, cCall("metadata", 1+r.IntN(30)))
			}
		}
		return cCall("icc_profile", cList(cs))
	case 11:
		var cs []any
		for range r.IntN(8) {
			switch r.IntN(7) {
			case 0:
				cs = append(cs, cCall("domain", randomColorFloats(r, r.IntN(5))))
			case 1:
				cs = append(cs, cCall("range", randomColorFloats(r, r.IntN(5))))
			case 2:
				size := []any{}
				for range r.IntN(4) {
					size = append(size, randomColorInt(r))
				}
				cs = append(cs, cCall("size", size))
			case 3:
				cs = append(cs, cCall("bits_per_sample", randomColorInt(r)))
			case 4:
				cs = append(cs, cCall("order", []string{"Linear", "Cubic"}[r.IntN(2)]))
			case 5:
				cs = append(cs, cCall("encode", randomColorFloats(r, r.IntN(5))))
			default:
				cs = append(cs, cCall("decode", randomColorFloats(r, r.IntN(5))))
			}
		}
		return cCall("sampled_function", cList(cs))
	}
	var cs []any
	for range r.IntN(3) {
		cs = append(cs, cCall([]string{"domain", "range"}[r.IntN(2)], randomColorFloats(r, r.IntN(5))))
	}
	if r.IntN(4) == 0 {
		return cCall("post_script_function", cList(cs))
	}
	return cCall("post_script_function", cList(cs), randomColorPostScript(r, 2))
}

// cList gives an empty list for nil, so that the JSON holds [] and not
// null.
func cList(cs []any) []any {
	if cs == nil {
		return []any{}
	}
	return cs
}

// randomColorFloats gives n random numbers.
func randomColorFloats(r *rand.Rand, n int) []any {
	xs := make([]float32, n)
	for i := range xs {
		xs[i] = randomReal(r)
	}
	return cFl(xs...)
}

// randomColorInt gives a random i32, often a small one.
func randomColorInt(r *rand.Rand) int32 {
	if r.IntN(2) == 0 {
		return int32(r.IntN(33))
	}
	return int32(r.Uint32()) >> r.IntN(32)
}

// randomColorStep gives a random step of a tiling pattern, sometimes zero.
func randomColorStep(r *rand.Rand) float64 {
	if r.IntN(8) == 0 {
		return 0
	}
	return float64(randomReal(r))
}

// randomColorRefs gives up to four random object numbers.
func randomColorRefs(r *rand.Rand) []any {
	refs := []any{}
	for range r.IntN(5) {
		refs = append(refs, 1+r.IntN(30))
	}
	return refs
}

// randomColorNames gives up to four random names.
func randomColorNames(r *rand.Rand) []any {
	names := []any{}
	for range r.IntN(5) {
		names = append(names, randomText(r))
	}
	return names
}

// randomColorSpace gives the call of a random color space, or nil for an
// unused one. A separation and a DeviceN go down to the depth.
func randomColorSpace(r *rand.Rand, depth int) any {
	n := 23
	if depth > 0 {
		n = 26
	}
	opt3 := func() any {
		if r.IntN(2) == 0 {
			return nil
		}
		return randomColorFloats(r, 3)
	}
	optF := func() any {
		if r.IntN(2) == 0 {
			return nil
		}
		return float64(randomReal(r))
	}
	switch r.IntN(n) {
	case 0:
		return nil
	case 1:
		var m any
		if r.IntN(2) == 0 {
			m = randomColorFloats(r, 9)
		}
		return cCall("cal_rgb", randomColorFloats(r, 3), opt3(), opt3(), m)
	case 2:
		return cCall("cal_gray", randomColorFloats(r, 3), opt3(), optF())
	case 3:
		var rng any
		if r.IntN(2) == 0 {
			rng = randomColorFloats(r, 4)
		}
		return cCall("lab", randomColorFloats(r, 3), opt3(), rng)
	case 4:
		return cCall("icc_based", 1+r.IntN(30))
	case 5:
		return cCall("d50_gray", optF())
	case 6:
		return cCall("e_gray", optF())
	case 7:
		return cCall("indexed", randomText(r), randomColorInt(r), hexString(randomBytes(r)))
	case 8:
		return cCall("pattern", randomText(r))
	case 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22:
		return cCall([]string{"srgb", "adobe_rgb", "display_p3", "pro_photo", "eci_rgb", "ntsc", "pal",
			"d65_gray", "c_gray", "device_rgb", "device_cmyk", "device_gray", "device_rgb", "device_gray"}[r.IntN(14)])
	case 23, 24:
		return cCall("separation", randomText(r), randomColorTints(r, depth-1, false))
	}
	return cCall("device_n", randomColorNames(r), randomColorTints(r, depth-1, true))
}

// randomColorTints gives the calls of a separation or a DeviceN. They are
// mostly in the order that pdf-writer wants, and sometimes not, so that
// both writers panic.
func randomColorTints(r *rand.Rand, depth int, deviceN bool) []any {
	alternate := func() []any {
		if r.IntN(2) == 0 {
			return cCall("alternate_color_space", randomColorSpace(r, depth))
		}
		return cCall("alternate_color_space_ref", 1+r.IntN(30))
	}
	tint := func() []any {
		switch r.IntN(3) {
		case 0:
			return cCall("tint_exponential", randomColorExponential(r))
		case 1:
			return cCall("tint_stitching", randomColorStitching(r))
		}
		return cCall("tint_ref", 1+r.IntN(30))
	}
	var cs []any
	if r.IntN(10) != 0 {
		cs = append(cs, alternate())
	}
	if r.IntN(10) != 0 {
		cs = append(cs, tint())
	}
	if deviceN && r.IntN(2) == 0 {
		cs = append(cs, cCall("attrs", randomColorDeviceNAttrs(r, depth)))
	}
	if r.IntN(10) == 0 {
		cs = append(cs, []any{alternate(), tint()}[r.IntN(2)])
	}
	r.Shuffle(len(cs), func(i, j int) {
		if r.IntN(8) == 0 {
			cs[i], cs[j] = cs[j], cs[i]
		}
	})
	return cList(cs)
}

// randomColorDeviceNAttrs gives the calls of the attributes of a DeviceN.
func randomColorDeviceNAttrs(r *rand.Rand, depth int) []any {
	var cs []any
	for range r.IntN(5) {
		switch r.IntN(4) {
		case 0:
			cs = append(cs, cCall("subtype", []string{"DeviceN", "NChannel"}[r.IntN(2)]))
		case 1:
			entries := []any{}
			for range r.IntN(3) {
				entries = append(entries, []any{randomText(r), cPairs(randomPairs(r, 1))})
			}
			cs = append(cs, cCall("colorants", entries))
		case 2:
			var ps []any
			for range r.IntN(4) {
				switch r.IntN(3) {
				case 0:
					ps = append(ps, cCall("color_space", randomText(r)))
				case 1:
					ps = append(ps, cCall("color_space_array", randomColorSpace(r, depth)))
				default:
					ps = append(ps, cCall("components", randomColorNames(r)))
				}
			}
			cs = append(cs, cCall("process", cList(ps)))
		default:
			var hs []any
			for range r.IntN(4) {
				switch r.IntN(3) {
				case 0, 1:
					entries := []any{}
					for range r.IntN(3) {
						entries = append(entries, []any{randomText(r), float64(randomReal(r))})
					}
					hs = append(hs, cCall([]string{"solidities", "dot_gain"}[r.IntN(2)], entries))
				default:
					hs = append(hs, cCall("printing_order", randomColorNames(r)))
				}
			}
			cs = append(cs, cCall("mixing_hints", cList(hs)))
		}
	}
	return cList(cs)
}

// cPairs gives the pairs of a dictionary as a list that is never
// null in the JSON.
func cPairs(pairs [][2]any) [][2]any {
	if pairs == nil {
		return [][2]any{}
	}
	return pairs
}

// randomColorFunctionShading gives the calls of a function shading.
func randomColorFunctionShading(r *rand.Rand, depth int) []any {
	var cs []any
	for range r.IntN(8) {
		switch r.IntN(10) {
		case 0:
			cs = append(cs, cCall("shading_type", []string{"Function", "Axial", "Radial"}[r.IntN(3)]))
		case 1:
			cs = append(cs, cCall("color_space", randomColorSpace(r, depth)))
		case 2:
			cs = append(cs, cCall("background", randomColorFloats(r, r.IntN(5))))
		case 3:
			cs = append(cs, cCall("bbox", randomColorFloats(r, 4)))
		case 4:
			cs = append(cs, cCall("anti_alias", r.IntN(2) == 0))
		case 5:
			cs = append(cs, cCall("domain", randomColorFloats(r, 4)))
		case 6:
			cs = append(cs, cCall("matrix", randomColorFloats(r, 6)))
		case 7:
			cs = append(cs, cCall("function", 1+r.IntN(30)))
		case 8:
			cs = append(cs, cCall("coords", randomColorFloats(r, r.IntN(7))))
		default:
			cs = append(cs, cCall("extend", []any{r.IntN(2) == 0, r.IntN(2) == 0}))
		}
	}
	return cList(cs)
}

// randomColorExponential gives the calls of an exponential function.
func randomColorExponential(r *rand.Rand) []any {
	var cs []any
	for range r.IntN(6) {
		switch r.IntN(5) {
		case 0:
			cs = append(cs, cCall("domain", randomColorFloats(r, r.IntN(5))))
		case 1:
			cs = append(cs, cCall("range", randomColorFloats(r, r.IntN(5))))
		case 2:
			cs = append(cs, cCall("c0", randomColorFloats(r, r.IntN(5))))
		case 3:
			cs = append(cs, cCall("c1", randomColorFloats(r, r.IntN(5))))
		default:
			cs = append(cs, cCall("n", float64(randomReal(r))))
		}
	}
	return cList(cs)
}

// randomColorStitching gives the calls of a stitching function.
func randomColorStitching(r *rand.Rand) []any {
	var cs []any
	for range r.IntN(6) {
		switch r.IntN(5) {
		case 0:
			cs = append(cs, cCall("domain", randomColorFloats(r, r.IntN(5))))
		case 1:
			cs = append(cs, cCall("range", randomColorFloats(r, r.IntN(5))))
		case 2:
			cs = append(cs, cCall("functions", randomColorRefs(r)))
		case 3:
			cs = append(cs, cCall("bounds", randomColorFloats(r, r.IntN(5))))
		default:
			cs = append(cs, cCall("encode", randomColorFloats(r, r.IntN(5))))
		}
	}
	return cList(cs)
}

// randomColorPostScript gives a random list of PostScript operators, with
// conditionals down to the depth.
func randomColorPostScript(r *rand.Rand, depth int) []any {
	names := make([]string, 0, len(colorPostScriptOps))
	for name := range colorPostScriptOps {
		names = append(names, name)
	}
	// The order of a map changes, so the names are sorted.
	slices.Sort(names)
	ops := []any{}
	for range r.IntN(6) {
		n := 3
		if depth > 0 {
			n = 5
		}
		switch r.IntN(n) {
		case 0:
			ops = append(ops, names[r.IntN(len(names))])
		case 1:
			ops = append(ops, map[string]any{"real": float64(randomReal(r))})
		case 2:
			ops = append(ops, map[string]any{"int": randomColorInt(r)})
		case 3:
			ops = append(ops, map[string]any{"if": randomColorPostScript(r, depth-1)})
		default:
			ops = append(ops, map[string]any{"ifelse": []any{randomColorPostScript(r, depth-1), randomColorPostScript(r, depth-1)}})
		}
	}
	return ops
}

// colorObject makes an object of a question for a script. A stream gets
// random data and sometimes a filter. A writer that derefs sometimes gets
// random pairs.
func colorObject(r *rand.Rand, id int, s colorScript) writerObject {
	o := writerObject{ID: id, Start: s.start}
	if colorStreams[s.kind] {
		data := randomBytes(r)
		if len(data) == 0 {
			data = []byte{'x'}
		}
		o.Data = base64.StdEncoding.EncodeToString(data)
		if r.IntN(3) == 0 {
			o.Filter = []string{"FlateDecode", "DCTDecode", "Crypt", "LZWDecode"}[r.IntN(4)]
		}
	}
	if colorDerefs[s.kind] && r.IntN(3) == 0 {
		o.Dict = randomPairs(r, 2)
	}
	return o
}

// TestPDFWriterGoColor compares the writers of color.go and functions.go
// with pdf-writer, for a script of each call.
func TestPDFWriterGoColor(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 51))
	no := false
	var questions []writerQuestion
	for _, s := range colorFixed {
		questions = append(questions,
			writerQuestion{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{{ID: 1, Start: s.start, Data: colorData(s)}}}},
			writerQuestion{PdfWriter: writerSpec{Chunk: true, Pretty: &no, Objects: []writerObject{colorObject(r, 2, s)}}},
		)
	}
	compareWriter(t, questions)
}

// colorData gives the data of a stream of a script, or nothing.
func colorData(s colorScript) string {
	if colorStreams[s.kind] {
		return base64.StdEncoding.EncodeToString([]byte("q Q"))
	}
	return ""
}

// TestPDFWriterGoColorRandom compares the writers of color.go and
// functions.go with pdf-writer, for random scripts in random files.
func TestPDFWriterGoColorRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 52))
	var questions []writerQuestion
	for i := 0; i < len(colorRandom); {
		q := writerSpec{Objects: []writerObject{}}
		if r.IntN(2) == 0 {
			p := r.IntN(2) == 0
			q.Pretty = &p
		}
		q.Chunk = r.IntN(3) == 0
		for range 1 + r.IntN(4) {
			if i == len(colorRandom) {
				break
			}
			q.Objects = append(q.Objects, colorObject(r, 1+r.IntN(30), colorRandom[i]))
			i++
		}
		if r.IntN(4) == 0 {
			q.Objects = append(q.Objects, writerObject{ID: 31 + r.IntN(10), Value: randomValue(r, 2)})
		}
		questions = append(questions, writerQuestion{PdfWriter: q})
	}
	compareWriter(t, questions)
}
