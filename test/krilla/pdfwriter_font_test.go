package krilla

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage/internal/pdfwriter"
)

// The Go side of the typed calls of font.rs, xobject.rs and files.rs, which
// rust/src/writer_font.rs makes with pdf-writer. That file describes the
// calls. goWriterFont answers a question "pdf_writer" whose objects are
// typed calls or the objects that goWriter writes.

func init() {
	writerDicts["type1_font"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Type1Font(id).Dict
	}
	writerDicts["type3_font"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Type3Font(id).Dict
	}
	writerDicts["type0_font"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Type0Font(id).Dict
	}
	writerDicts["cid_font"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.CIDFont(id).Dict
	}
	writerDicts["font_descriptor"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.FontDescriptor(id).Dict
	}
	writerStreams["cmap"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
		return c.Cmap(id, data).Stream
	}
	writerStreams["image_xobject"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
		return c.ImageXObject(id, data).Stream
	}
	writerStreams["form_xobject"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
		return c.FormXObject(id, data).Stream
	}
}

// goWriterFont answers a question "pdf_writer" with the Go writer, as
// goWriter does. It reads the fields pretty, chunk and objects. An object
// whose start begins with "calls." is a list of typed calls. Every other
// object goes to writeObject.
func goWriterFont(question []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("panic: %v", r)
		}
	}()
	dec := json.NewDecoder(bytes.NewReader(question))
	dec.UseNumber()
	var v struct {
		PdfWriter map[string]any `json:"pdf_writer"`
	}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	q := v.PdfWriter
	settings := pdfwriter.Settings{Pretty: true}
	if p, ok := q["pretty"].(bool); ok {
		settings.Pretty = p
	}
	objects, _ := q["objects"].([]any)
	chunk, _ := q["chunk"].(bool)

	write := func(c *pdfwriter.Chunk) error {
		for _, o := range objects {
			o := as[map[string]any](o)
			start, _ := o["start"].(string)
			if !strings.HasPrefix(start, "calls.") {
				if err := writeObject(c, o, settings); err != nil {
					return err
				}
				continue
			}
			if err := fwWriteCalls(c, o, start); err != nil {
				return err
			}
		}
		return nil
	}
	if chunk {
		c := pdfwriter.NewChunkWithSettings(settings)
		if err := write(c); err != nil {
			return nil, err
		}
		return c.AsBytes(), nil
	}
	pdf := pdfwriter.NewPdfWithSettings(settings)
	if err := write(&pdf.Chunk); err != nil {
		return nil, err
	}
	return pdf.Finish(), nil
}

// fwWriteCalls is write of writer_font.rs. A call that the oracle does not
// know panics, and goWriterFont gives the panic as an error.
func fwWriteCalls(c *pdfwriter.Chunk, o map[string]any, start string) error {
	id, err := reference(o["id"])
	if err != nil {
		return err
	}
	var data []byte
	hasData := o["data"] != nil
	if hasData {
		data, err = base64.StdEncoding.DecodeString(as[string](o["data"]))
		if err != nil {
			return fmt.Errorf("decoding the data of object %d: %w", id.Get(), err)
		}
	}
	var filter *pdfwriter.Filter
	if f, ok := o["filter"].(string); ok {
		x, err := filterOf(f)
		if err != nil {
			return err
		}
		filter = &x
	}
	calls, _ := o["dict"].([]any)
	stream := func(s *pdfwriter.Stream) {
		if filter != nil {
			s.Filter(*filter)
		}
	}
	switch {
	case start == "calls.type1_font" && !hasData:
		fwType1Font(c.Type1Font(id), calls)
	case start == "calls.type3_font" && !hasData:
		fwType3Font(c.Type3Font(id), calls)
	case start == "calls.type0_font" && !hasData:
		fwType0Font(c.Type0Font(id), calls)
	case start == "calls.cid_font" && !hasData:
		fwCIDFont(c.CIDFont(id), calls)
	case start == "calls.font_descriptor" && !hasData:
		fwFontDescriptor(c.FontDescriptor(id), calls)
	case start == "calls.file_spec" && !hasData:
		fwFileSpec(c.FileSpec(id), calls)
	case start == "calls.unicode_cmap" && !hasData:
		c.Cmap(id, fwUnicodeCmap(calls))
	case start == "calls.cmap" && hasData:
		w := c.Cmap(id, data)
		stream(w.Stream)
		fwCmap(w, calls)
	case start == "calls.image_xobject" && hasData:
		w := c.ImageXObject(id, data)
		stream(w.Stream)
		fwImageXObject(w, calls)
	case start == "calls.form_xobject" && hasData:
		w := c.FormXObject(id, data)
		stream(w.Stream)
		fwFormXObject(w, calls)
	default:
		return fmt.Errorf("object %d: the start %q needs data, or no data", id.Get(), start)
	}
	return nil
}

// one returns the content of a value of the kind, such as "name".
func fwOne(v any, kind string) any {
	m := as[map[string]any](v)
	x, ok := m[kind]
	if !ok || len(m) != 1 {
		panic(fmt.Sprintf("the value %v is not a %s", v, kind))
	}
	return x
}

// call splits a call into its method and its value.
func fwSplit(c any) (string, any) {
	p := as[[]any](c)
	return as[string](p[0]), p[1]
}

// fwArgs returns the arguments of a call, which must be n.
func fwArgs(m string, v any, n int) []any {
	a := fwItems(v)
	if len(a) != n {
		panic(fmt.Sprintf("the call %q takes %d arguments, not %d", m, n, len(a)))
	}
	return a
}

func fwItems(v any) []any         { return as[[]any](fwOne(v, "array")) }
func fwCallsOf(v any) []any       { return as[[]any](fwOne(v, "dict")) }
func fwName(v any) pdfwriter.Name { return pdfwriter.Name(as[string](fwOne(v, "name"))) }
func fwText(v any) string         { return as[string](fwOne(v, "text")) }
func fwBool(v any) bool           { return as[bool](fwOne(v, "bool")) }

func fwInt(v any) int32 {
	n, err := strconv.ParseInt(string(as[json.Number](fwOne(v, "int"))), 10, 32)
	if err != nil {
		panic(err)
	}
	return int32(n)
}

func fwRange(v any, lo, hi int32) int32 {
	n := fwInt(v)
	if n < lo || n > hi {
		panic(fmt.Sprintf("the int %d is out of range", n))
	}
	return n
}

func fwU8(v any) uint8   { return uint8(fwRange(v, 0, 255)) }
func fwU16(v any) uint16 { return uint16(fwRange(v, 0, 65535)) }

func fwReal(v any) float32 {
	f, err := strconv.ParseFloat(string(as[json.Number](fwOne(v, "real"))), 64)
	if err != nil {
		panic(err)
	}
	return float32(f)
}

func fwReals(vs []any) []float32 {
	out := make([]float32, len(vs))
	for i, v := range vs {
		out[i] = fwReal(v)
	}
	return out
}

func fwStr(v any) pdfwriter.Str {
	b, err := hex.DecodeString(as[string](fwOne(v, "str")))
	if err != nil {
		panic(err)
	}
	return pdfwriter.Str(b)
}

func fwRef(v any) pdfwriter.Ref {
	r, err := reference(fwOne(v, "ref"))
	if err != nil {
		panic(err)
	}
	return r
}

func fwRect(a []any) pdfwriter.Rect {
	return pdfwriter.NewRect(fwReal(a[0]), fwReal(a[1]), fwReal(a[2]), fwReal(a[3]))
}

func fwMatrix(a []any) [6]float32 {
	if len(a) != 6 {
		panic("a matrix has six numbers")
	}
	return [6]float32(fwReals(a))
}

func fwDate(a []any) pdfwriter.Date {
	if len(a) == 0 || len(a) > 8 {
		panic("a date has one to eight numbers")
	}
	d := pdfwriter.NewDate(fwU16(a[0]))
	for i, v := range a[1:] {
		switch i + 1 {
		case 1:
			d = d.Month(fwU8(v))
		case 2:
			d = d.Day(fwU8(v))
		case 3:
			d = d.Hour(fwU8(v))
		case 4:
			d = d.Minute(fwU8(v))
		case 5:
			d = d.Second(fwU8(v))
		case 6:
			d = d.UTCOffsetHour(int8(fwRange(v, -128, 127)))
		default:
			d = d.UTCOffsetMinute(fwU8(v))
		}
	}
	return d
}

func fwSystemInfo(a []any) pdfwriter.SystemInfo {
	return pdfwriter.SystemInfo{Registry: fwStr(a[0]), Ordering: fwStr(a[1]), Supplement: fwInt(a[2])}
}

// fwEnum finds a value of an enum by its name, with String.
func fwEnum[T fmt.Stringer](v any, values ...T) T {
	n := string(fwName(v))
	for _, x := range values {
		if x.String() == n {
			return x
		}
	}
	panic(fmt.Sprintf("the name %q is not known", n))
}

var (
	fwCIDFontTypes  = []pdfwriter.CIDFontType{pdfwriter.CIDFontType0, pdfwriter.CIDFontType2}
	fwFontStretches = []pdfwriter.FontStretch{
		pdfwriter.FontStretchUltraCondensed, pdfwriter.FontStretchExtraCondensed, pdfwriter.FontStretchCondensed,
		pdfwriter.FontStretchSemiCondensed, pdfwriter.FontStretchNormal, pdfwriter.FontStretchSemiExpanded,
		pdfwriter.FontStretchExpanded, pdfwriter.FontStretchExtraExpanded, pdfwriter.FontStretchUltraExpanded,
	}
	fwCJKClasses = []pdfwriter.CJKClass{
		pdfwriter.CJKClassAlphabetic, pdfwriter.CJKClassAlphaNum, pdfwriter.CJKClassDingbats,
		pdfwriter.CJKClassDingbatsRot, pdfwriter.CJKClassGeneric, pdfwriter.CJKClassGenericRot,
		pdfwriter.CJKClassHangul, pdfwriter.CJKClassHanja, pdfwriter.CJKClassHanzi, pdfwriter.CJKClassHRoman,
		pdfwriter.CJKClassHRomanRot, pdfwriter.CJKClassHKana, pdfwriter.CJKClassHKanaRot,
		pdfwriter.CJKClassHojoKanji, pdfwriter.CJKClassKana, pdfwriter.CJKClassKanji,
		pdfwriter.CJKClassProportional, pdfwriter.CJKClassProportionalRot, pdfwriter.CJKClassRuby,
	}
	fwAssociationKinds = []pdfwriter.AssociationKind{
		pdfwriter.AssociationKindSource, pdfwriter.AssociationKindData, pdfwriter.AssociationKindAlternative,
		pdfwriter.AssociationKindSupplement, pdfwriter.AssociationKindEncryptedPayload,
		pdfwriter.AssociationKindFormData, pdfwriter.AssociationKindSchema, pdfwriter.AssociationKindUnspecified,
	}
)

func fwWMode(v any) pdfwriter.WMode { return pdfwriter.WMode(fwRange(v, 0, 1)) }

// insert writes the call "insert" through the dictionary of a writer.
func fwInsert(d *pdfwriter.Dict, m string, v any) {
	a := fwArgs(m, v, 2)
	if err := writeValue(d.Insert(fwName(a[0])), a[1]); err != nil {
		panic(err)
	}
}

// item writes the call "item" through the array of a writer.
func fwItem(a *pdfwriter.Array, m string, v any) {
	if err := writeValue(a.Push(), fwArgs(m, v, 1)[0]); err != nil {
		panic(err)
	}
}

func fwUnknown(writer, m string) string {
	return fmt.Sprintf("the writer %s has no call %q", writer, m)
}

func fwType1Font(w *pdfwriter.Type1Font, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "name":
			w.Name(fwName(fwArgs(m, v, 1)[0]))
		case "base_font":
			w.BaseFont(fwName(fwArgs(m, v, 1)[0]))
		case "first_char":
			w.FirstChar(fwU8(fwArgs(m, v, 1)[0]))
		case "last_char":
			w.LastChar(fwU8(fwArgs(m, v, 1)[0]))
		case "widths":
			w.Widths(fwReals(fwItems(v))...)
		case "font_descriptor":
			w.FontDescriptor(fwRef(fwArgs(m, v, 1)[0]))
		case "encoding_predefined":
			w.EncodingPredefined(fwName(fwArgs(m, v, 1)[0]))
		case "encoding_custom":
			fwEncoding(w.EncodingCustom(), fwCallsOf(v))
		case "to_unicode":
			w.ToUnicode(fwRef(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Type1Font", m))
		}
	}
}

func fwType3Font(w *pdfwriter.Type3Font, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "name":
			w.Name(fwName(fwArgs(m, v, 1)[0]))
		case "bbox":
			w.BBox(fwRect(fwArgs(m, v, 4)))
		case "matrix":
			w.Matrix(fwMatrix(fwItems(v)))
		case "char_procs":
			a := fwItems(v)
			d := w.CharProcs()
			for i := 0; i < len(a); i += 2 {
				if i+1 == len(a) {
					panic("char_procs takes pairs")
				}
				d.Pair(fwName(a[i]), fwRef(a[i+1]))
			}
		case "encoding_predefined":
			w.EncodingPredefined(fwName(fwArgs(m, v, 1)[0]))
		case "encoding_custom":
			fwEncoding(w.EncodingCustom(), fwCallsOf(v))
		case "first_char":
			w.FirstChar(fwU8(fwArgs(m, v, 1)[0]))
		case "last_char":
			w.LastChar(fwU8(fwArgs(m, v, 1)[0]))
		case "widths":
			w.Widths(fwReals(fwItems(v))...)
		case "font_descriptor":
			w.FontDescriptor(fwRef(fwArgs(m, v, 1)[0]))
		case "to_unicode":
			w.ToUnicode(fwRef(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Type3Font", m))
		}
	}
}

func fwEncoding(w *pdfwriter.Encoding, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "base_encoding":
			w.BaseEncoding(fwName(fwArgs(m, v, 1)[0]))
		case "differences":
			fwDifferences(w.Differences(), fwCallsOf(v))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Encoding", m))
		}
	}
}

func fwDifferences(w *pdfwriter.Differences, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "consecutive":
			a := fwItems(v)
			if len(a) == 0 {
				panic("consecutive takes a start")
			}
			names := make([]pdfwriter.Name, len(a)-1)
			for i, n := range a[1:] {
				names[i] = fwName(n)
			}
			w.Consecutive(fwU8(a[0]), names...)
		case "item":
			fwItem(w.Array, m, v)
		default:
			panic(fwUnknown("Differences", m))
		}
	}
}

func fwType0Font(w *pdfwriter.Type0Font, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "base_font":
			w.BaseFont(fwName(fwArgs(m, v, 1)[0]))
		case "encoding_predefined":
			w.EncodingPredefined(fwName(fwArgs(m, v, 1)[0]))
		case "encoding_cmap":
			w.EncodingCmap(fwRef(fwArgs(m, v, 1)[0]))
		case "descendant_font":
			w.DescendantFont(fwRef(fwArgs(m, v, 1)[0]))
		case "to_unicode":
			w.ToUnicode(fwRef(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Type0Font", m))
		}
	}
}

func fwCIDFont(w *pdfwriter.CIDFont, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "subtype":
			w.Subtype(fwEnum(fwArgs(m, v, 1)[0], fwCIDFontTypes...))
		case "base_font":
			w.BaseFont(fwName(fwArgs(m, v, 1)[0]))
		case "system_info":
			w.SystemInfo(fwSystemInfo(fwArgs(m, v, 3)))
		case "font_descriptor":
			w.FontDescriptor(fwRef(fwArgs(m, v, 1)[0]))
		case "default_width":
			w.DefaultWidth(fwReal(fwArgs(m, v, 1)[0]))
		case "widths":
			fwWidths(w.Widths(), fwCallsOf(v))
		case "cid_to_gid_map_predefined":
			w.CIDToGIDMapPredefined(fwName(fwArgs(m, v, 1)[0]))
		case "cid_to_gid_map_stream":
			w.CIDToGIDMapStream(fwRef(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("CidFont", m))
		}
	}
}

func fwWidths(w *pdfwriter.Widths, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "consecutive":
			a := fwItems(v)
			if len(a) == 0 {
				panic("consecutive takes a start")
			}
			w.Consecutive(fwU16(a[0]), fwReals(a[1:])...)
		case "same":
			a := fwArgs(m, v, 3)
			w.Same(fwU16(a[0]), fwU16(a[1]), fwReal(a[2]))
		case "item":
			fwItem(w.Array, m, v)
		default:
			panic(fwUnknown("Widths", m))
		}
	}
}

func fwFontDescriptor(w *pdfwriter.FontDescriptor, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "name":
			w.Name(fwName(fwArgs(m, v, 1)[0]))
		case "family":
			w.Family(fwStr(fwArgs(m, v, 1)[0]))
		case "stretch":
			w.Stretch(fwEnum(fwArgs(m, v, 1)[0], fwFontStretches...))
		case "weight":
			w.Weight(fwU16(fwArgs(m, v, 1)[0]))
		case "flags":
			w.Flags(pdfwriter.FontFlags(uint32(fwInt(fwArgs(m, v, 1)[0]))))
		case "bbox":
			w.BBox(fwRect(fwArgs(m, v, 4)))
		case "italic_angle":
			w.ItalicAngle(fwReal(fwArgs(m, v, 1)[0]))
		case "ascent":
			w.Ascent(fwReal(fwArgs(m, v, 1)[0]))
		case "descent":
			w.Descent(fwReal(fwArgs(m, v, 1)[0]))
		case "leading":
			w.Leading(fwReal(fwArgs(m, v, 1)[0]))
		case "cap_height":
			w.CapHeight(fwReal(fwArgs(m, v, 1)[0]))
		case "x_height":
			w.XHeight(fwReal(fwArgs(m, v, 1)[0]))
		case "stem_v":
			w.StemV(fwReal(fwArgs(m, v, 1)[0]))
		case "stem_h":
			w.StemH(fwReal(fwArgs(m, v, 1)[0]))
		case "avg_width":
			w.AvgWidth(fwReal(fwArgs(m, v, 1)[0]))
		case "max_width":
			w.MaxWidth(fwReal(fwArgs(m, v, 1)[0]))
		case "missing_width":
			w.MissingWidth(fwReal(fwArgs(m, v, 1)[0]))
		case "font_file":
			w.FontFile(fwRef(fwArgs(m, v, 1)[0]))
		case "font_file2":
			w.FontFile2(fwRef(fwArgs(m, v, 1)[0]))
		case "font_file3":
			w.FontFile3(fwRef(fwArgs(m, v, 1)[0]))
		case "char_set":
			w.CharSet(fwStr(fwArgs(m, v, 1)[0]))
		case "style":
			a := fwArgs(m, v, 12)
			var panose [10]uint8
			for i := range panose {
				panose[i] = fwU8(a[2+i])
			}
			w.Style(fwU8(a[0]), fwU8(a[1]), panose)
		case "descriptor_override":
			fwFontDescriptorOverride(w.DescriptorOverride(), fwCallsOf(v))
		case "cid_set":
			w.CIDSet(fwRef(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("FontDescriptor", m))
		}
	}
}

func fwFontDescriptorOverride(w *pdfwriter.FontDescriptorOverride, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "custom_class":
			a := fwArgs(m, v, 2)
			fwFontDescriptor(w.CustomClass(fwName(a[0])), fwCallsOf(a[1]))
		case "custom_class_ref":
			a := fwArgs(m, v, 2)
			w.CustomClassRef(fwName(a[0]), fwRef(a[1]))
		case "cjk_class":
			a := fwArgs(m, v, 2)
			fwFontDescriptor(w.CJKClass(fwEnum(a[0], fwCJKClasses...)), fwCallsOf(a[1]))
		case "cjk_class_ref":
			a := fwArgs(m, v, 2)
			w.CJKClassRef(fwEnum(a[0], fwCJKClasses...), fwRef(a[1]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("FontDescriptorOverride", m))
		}
	}
}

func fwCmap(w *pdfwriter.Cmap, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "name":
			w.Name(fwName(fwArgs(m, v, 1)[0]))
		case "system_info":
			w.SystemInfo(fwSystemInfo(fwArgs(m, v, 3)))
		case "writing_mode":
			w.WritingMode(fwWMode(fwArgs(m, v, 1)[0]))
		case "use_cmap_stream":
			w.UseCmapStream(fwRef(fwArgs(m, v, 1)[0]))
		case "use_cmap_predefined":
			w.UseCmapPredefined(fwName(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Cmap", m))
		}
	}
}

func fwUnicodeCmap(calls []any) []byte {
	if len(calls) == 0 {
		panic("a UnicodeCmap needs the call new_u8 or new_u16")
	}
	m, v := fwSplit(calls[0])
	a := fwArgs(m, v, 5)
	info := fwSystemInfo(a[1:4])
	mode := fwWMode(a[4])
	switch m {
	case "new_u8":
		return fwCmapPairs(pdfwriter.NewUnicodeCmapWithWritingMode[uint8](fwName(a[0]), info, mode), calls[1:])
	case "new_u16":
		return fwCmapPairs(pdfwriter.NewUnicodeCmapWithWritingMode[uint16](fwName(a[0]), info, mode), calls[1:])
	}
	panic("a UnicodeCmap needs the call new_u8 or new_u16")
}

func fwCmapPairs[G pdfwriter.GlyphID](cmap *pdfwriter.UnicodeCmap[G], calls []any) []byte {
	for _, c := range calls {
		m, v := fwSplit(c)
		a := fwArgs(m, v, 2)
		g := G(fwRange(a[0], 0, int32(^G(0))))
		t := fwText(a[1])
		switch m {
		case "pair":
			if utf8.RuneCountInString(t) != 1 {
				panic("pair takes one character")
			}
			r, _ := utf8.DecodeRuneInString(t)
			cmap.Pair(g, r)
		case "pair_with_multiple":
			cmap.PairWithMultiple(g, []rune(t)...)
		default:
			panic(fwUnknown("UnicodeCmap", m))
		}
	}
	b := cmap.Finish()
	return b.IntoVec()
}

func fwImageXObject(w *pdfwriter.ImageXObject, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "width":
			w.Width(fwInt(fwArgs(m, v, 1)[0]))
		case "height":
			w.Height(fwInt(fwArgs(m, v, 1)[0]))
		case "color_space_name":
			w.ColorSpaceName(fwName(fwArgs(m, v, 1)[0]))
		case "bits_per_component":
			w.BitsPerComponent(fwInt(fwArgs(m, v, 1)[0]))
		case "image_mask":
			w.ImageMask(fwBool(fwArgs(m, v, 1)[0]))
		case "color_mask":
			var colors []int32
			for _, x := range fwItems(v) {
				colors = append(colors, fwInt(x))
			}
			w.ColorMask(colors...)
		case "stencil_mask":
			w.StencilMask(fwRef(fwArgs(m, v, 1)[0]))
		case "decode":
			w.Decode(fwReals(fwItems(v))...)
		case "interpolate":
			w.Interpolate(fwBool(fwArgs(m, v, 1)[0]))
		case "alternates":
			var refs []pdfwriter.Ref
			for _, x := range fwItems(v) {
				refs = append(refs, fwRef(x))
			}
			w.Alternates(refs...)
		case "s_mask":
			w.SMask(fwRef(fwArgs(m, v, 1)[0]))
		case "s_mask_in_data":
			w.SMaskInData(pdfwriter.SMaskInData(fwRange(fwArgs(m, v, 1)[0], 0, 2)))
		case "struct_parent":
			w.StructParent(fwInt(fwArgs(m, v, 1)[0]))
		case "matte":
			w.Matte(fwReals(fwItems(v))...)
		case "metadata":
			w.Metadata(fwRef(fwArgs(m, v, 1)[0]))
		case "associated_files":
			a := w.AssociatedFiles()
			for _, d := range fwItems(v) {
				fwFileSpec(a.Push(), fwCallsOf(d))
			}
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("ImageXObject", m))
		}
	}
}

func fwFormXObject(w *pdfwriter.FormXObject, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "bbox":
			w.BBox(fwRect(fwArgs(m, v, 4)))
		case "matrix":
			w.Matrix(fwMatrix(fwItems(v)))
		case "group":
			fwGroup(w.Group(), fwCallsOf(v))
		case "struct_parent":
			w.StructParent(fwInt(fwArgs(m, v, 1)[0]))
		case "struct_parents":
			w.StructParents(fwInt(fwArgs(m, v, 1)[0]))
		case "reference":
			fwReference(w.Reference(), fwCallsOf(v))
		case "metadata":
			w.Metadata(fwRef(fwArgs(m, v, 1)[0]))
		case "last_modified":
			w.LastModified(fwDate(fwItems(v)))
		case "associated_files":
			a := w.AssociatedFiles()
			for _, d := range fwItems(v) {
				fwFileSpec(a.Push(), fwCallsOf(d))
			}
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("FormXObject", m))
		}
	}
}

func fwGroup(w *pdfwriter.Group, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "transparency":
			fwArgs(m, v, 0)
			w.Transparency()
		case "isolated":
			w.Isolated(fwBool(fwArgs(m, v, 1)[0]))
		case "knockout":
			w.Knockout(fwBool(fwArgs(m, v, 1)[0]))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Group", m))
		}
	}
}

func fwReference(w *pdfwriter.Reference, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "file_spec":
			fwFileSpec(w.FileSpec(), fwCallsOf(v))
		case "page_number":
			w.PageNumber(fwInt(fwArgs(m, v, 1)[0]))
		case "page_label":
			w.PageLabel(pdfwriter.TextStr(fwText(fwArgs(m, v, 1)[0])))
		case "id":
			a := fwArgs(m, v, 2)
			w.ID([2]pdfwriter.Str{fwStr(a[0]), fwStr(a[1])})
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("Reference", m))
		}
	}
}

func fwFileSpec(w *pdfwriter.FileSpec, calls []any) {
	for _, c := range calls {
		switch m, v := fwSplit(c); m {
		case "file_system":
			w.FileSystem(fwName(fwArgs(m, v, 1)[0]))
		case "path":
			w.Path(fwStr(fwArgs(m, v, 1)[0]))
		case "unic_file":
			w.UnicFile(pdfwriter.TextStr(fwText(fwArgs(m, v, 1)[0])))
		case "volatile":
			w.Volatile(fwBool(fwArgs(m, v, 1)[0]))
		case "description":
			w.Description(pdfwriter.TextStr(fwText(fwArgs(m, v, 1)[0])))
		case "embedded_file":
			w.EmbeddedFile(fwRef(fwArgs(m, v, 1)[0]))
		case "embedded_file_with_unicode":
			w.EmbeddedFileWithUnicode(fwRef(fwArgs(m, v, 1)[0]))
		case "association_kind":
			w.AssociationKind(fwEnum(fwArgs(m, v, 1)[0], fwAssociationKinds...))
		case "insert":
			fwInsert(w.Dict, m, v)
		default:
			panic(fwUnknown("FileSpec", m))
		}
	}
}

// compareWriterFont is compareWriter with goWriterFont.
func compareWriterFont(t *testing.T, questions []writerQuestion) {
	t.Helper()
	qs := make([]any, len(questions))
	for i := range questions {
		qs[i] = questions[i]
	}
	answers := ask(t, qs...)
	for i, a := range answers {
		b, err := json.Marshal(questions[i])
		if err != nil {
			t.Fatal(err)
		}
		got, gerr := goWriterFont(b)
		switch {
		case a.Err != "" && gerr == nil:
			t.Errorf("question %d: the oracle gives the error %q, and Go gives no error\n%s", i, a.Err, b)
		case a.Err != "" && strings.HasPrefix(a.Err, "panic: ") && a.Err != gerr.Error():
			t.Errorf("question %d: the oracle panics with %q, and Go with %q\n%s", i, a.Err, gerr, b)
		case a.Err == "" && gerr != nil:
			t.Errorf("question %d: Go gives the error %q\n%s", i, gerr, b)
		case a.Err == "" && !bytes.Equal(got, a.Data):
			t.Errorf("question %d: the bytes differ\n%s\ngot  %q\nwant %q", i, b, got, a.Data)
		}
	}
}

// c makes a call of writer_font.rs with its arguments.
func fwCall(method string, args ...any) [2]any {
	return pw.Pair(method, pw.Array(args...))
}

// sub makes a call that starts another writer, with the calls on it.
func fwSub(method string, calls ...[2]any) [2]any {
	return pw.Pair(method, pw.Dict(calls...))
}

// fwSubArgs makes a call that takes arguments and starts another writer.
func fwSubArgs(method string, args []any, calls ...[2]any) [2]any {
	return pw.Pair(method, pw.Array(append(args, pw.Dict(calls...))...))
}

func fwReals32(fs ...float32) []any {
	out := make([]any, len(fs))
	for i, f := range fs {
		out[i] = pw.Real(f)
	}
	return out
}

func fwData(b string) string { return base64.StdEncoding.EncodeToString([]byte(b)) }

// TestPDFWriterGoFontStarts compares the fixed keys of the typed writers of
// font.rs and xobject.rs, through the starts of writer.rs and the hooks
// that this file registers.
func TestPDFWriterGoFontStarts(t *testing.T) {
	no := false
	pairs := [][2]any{pw.Pair("A", pw.Int(1)), pw.Pair("B", pw.Array(pw.Real(0.5)))}
	objects := []writerObject{
		{ID: 1, Start: "type1_font", Dict: pairs},
		{ID: 2, Start: "type3_font"},
		{ID: 3, Start: "type0_font", Dict: pairs},
		{ID: 4, Start: "cid_font", Dict: pairs},
		{ID: 5, Start: "font_descriptor", Dict: pairs},
		{ID: 6, Start: "cmap", Data: fwData("cmap"), Filter: "FlateDecode", Dict: pairs},
		{ID: 7, Start: "image_xobject", Data: fwData("\x00\xff"), Dict: pairs},
		{ID: 8, Start: "form_xobject", Data: fwData("q Q")},
	}
	compareWriter(t, []writerQuestion{
		{PdfWriter: writerSpec{Objects: objects}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: objects}},
		{PdfWriter: writerSpec{Chunk: true, Objects: objects}},
	})
}

// fwObjects holds a list of objects that make each call of
// writer_font.rs at least once.
func fwObjects() []writerObject {
	info := []any{pw.Str([]byte("Adobe")), pw.Str([]byte("Identity")), pw.Int(0)}
	fileSpec := []([2]any){
		fwCall("file_system", pw.Name("URL")), fwCall("path", pw.Str([]byte("a/b (1).pdf"))),
		fwCall("unic_file", pw.Text("é/ü.pdf")), fwCall("volatile", pw.Bool(true)), fwCall("description", pw.Text("A file")),
		fwCall("embedded_file", pw.Ref(20)), fwCall("embedded_file_with_unicode", pw.Ref(21)),
		fwCall("association_kind", pw.Name("Supplement")), fwCall("insert", pw.Name("X"), pw.Null),
	}
	var cmapCalls16, cmapCalls8 [][2]any
	cmapCalls16 = append(cmapCalls16, fwCall("new_u16", append([]any{pw.Name("Adobe-Identity-UCS")}, append(info, pw.Int(0))...)...))
	cmapCalls8 = append(cmapCalls8, fwCall("new_u8", append([]any{pw.Name("Custom #1")}, append(info, pw.Int(1))...)...))
	for g := range 130 {
		cmapCalls16 = append(cmapCalls16, fwCall("pair", pw.Int(int32(g*500)), pw.Text(string(rune(0x20+g)))))
	}
	cmapCalls16 = append(cmapCalls16,
		fwCall("pair_with_multiple", pw.Int(65535), pw.Text("ﬃ😀x")),
		fwCall("pair_with_multiple", pw.Int(7), pw.Text("")),
		fwCall("pair", pw.Int(0), pw.Text("\U0010FFFF")))
	for g := range 256 {
		cmapCalls8 = append(cmapCalls8, fwCall("pair", pw.Int(int32(g)), pw.Text(string(rune(0x400+g)))))
	}
	return []writerObject{
		{ID: 1, Start: "calls.type1_font", Dict: [][2]any{
			fwCall("name", pw.Name("F1")), fwCall("base_font", pw.Name("Helvetica")), fwCall("first_char", pw.Int(32)),
			fwCall("last_char", pw.Int(255)), fwCall("widths", fwReals32(278, 0.5, 1e-7, -3, 4.22)...),
			fwCall("font_descriptor", pw.Ref(5)), fwCall("encoding_predefined", pw.Name("WinAnsiEncoding")),
			fwSub("encoding_custom",
				fwCall("base_encoding", pw.Name("StandardEncoding")),
				fwSub("differences",
					fwCall("consecutive", pw.Int(0), pw.Name("g0"), pw.Name("g 1"), pw.Name("é")),
					fwCall("item", pw.Int(39)), fwCall("consecutive", pw.Int(255))),
				fwCall("insert", pw.Name("Extra"), pw.Dict(pw.Pair("K", pw.Int(1))))),
			fwCall("to_unicode", pw.Ref(9)), fwCall("insert", pw.Name("Last"), pw.Bool(false)),
		}},
		{ID: 2, Start: "calls.type3_font", Dict: [][2]any{
			fwCall("name", pw.Name("T3")), fwCall("bbox", fwReals32(0, -200, 1000, 800.5)...),
			fwCall("matrix", fwReals32(0.001, 0, 0, 0.001, 0, 0)...),
			fwCall("char_procs", pw.Name("g0"), pw.Ref(30), pw.Name("g1"), pw.Ref(31)),
			fwCall("encoding_predefined", pw.Name("MacRomanEncoding")),
			fwSub("encoding_custom", fwSub("differences", fwCall("consecutive", pw.Int(0), pw.Name("g0"), pw.Name("g1")))),
			fwCall("first_char", pw.Int(0)), fwCall("last_char", pw.Int(1)), fwCall("widths", fwReals32(500, 600)...),
			fwCall("font_descriptor", pw.Ref(5)), fwCall("to_unicode", pw.Ref(9)),
		}},
		{ID: 3, Start: "calls.type0_font", Dict: [][2]any{
			fwCall("base_font", pw.Name("ABCDEF+Noto-Identity-H")), fwCall("encoding_predefined", pw.Name("Identity-H")),
			fwCall("encoding_cmap", pw.Ref(10)), fwCall("descendant_font", pw.Ref(4)), fwCall("to_unicode", pw.Ref(9)),
			fwCall("insert", pw.Name("I"), pw.Int(-1)),
		}},
		{ID: 4, Start: "calls.cid_font", Dict: [][2]any{
			fwCall("subtype", pw.Name("CIDFontType2")), fwCall("subtype", pw.Name("CIDFontType0")),
			fwCall("base_font", pw.Name("ABCDEF+Noto")), fwCall("system_info", info...), fwCall("font_descriptor", pw.Ref(5)),
			fwCall("default_width", pw.Real(1000)),
			fwSub("widths",
				fwCall("consecutive", append([]any{pw.Int(0)}, fwReals32(500, 0.25, 600)...)...),
				fwCall("same", pw.Int(10), pw.Int(65535), pw.Real(-12.5)),
				fwCall("consecutive", pw.Int(65535)), fwCall("item", pw.Name("odd"))),
			fwCall("cid_to_gid_map_predefined", pw.Name("Identity")), fwCall("cid_to_gid_map_stream", pw.Ref(11)),
		}},
		{ID: 5, Start: "calls.font_descriptor", Dict: [][2]any{
			fwCall("name", pw.Name("ABCDEF+Noto")), fwCall("family", pw.Str([]byte("Noto (Sans)"))),
			fwCall("stretch", pw.Name("SemiExpanded")), fwCall("weight", pw.Int(700)), fwCall("flags", pw.Int(1<<5|1<<6|1<<18)),
			fwCall("flags", pw.Int(-1)), fwCall("bbox", fwReals32(-100, -250.5, 1200, 900)...),
			fwCall("italic_angle", pw.Real(-12.5)), fwCall("ascent", pw.Real(800)), fwCall("descent", pw.Real(-200)),
			fwCall("leading", pw.Real(0)), fwCall("cap_height", pw.Real(700)), fwCall("x_height", pw.Real(500)),
			fwCall("stem_v", pw.Real(80)), fwCall("stem_h", pw.Real(70)), fwCall("avg_width", pw.Real(510)),
			fwCall("max_width", pw.Real(1500)), fwCall("missing_width", pw.Real(250)), fwCall("font_file", pw.Ref(12)),
			fwCall("font_file2", pw.Ref(13)), fwCall("font_file3", pw.Ref(14)), fwCall("char_set", pw.Str([]byte("/a/b"))),
			fwCall("style", pw.Int(8), pw.Int(1), pw.Int(2), pw.Int(11), pw.Int(6), pw.Int(3), pw.Int(0), pw.Int(255),
				pw.Int(40), pw.Int(41), pw.Int(92), pw.Int(10)),
			fwSub("descriptor_override",
				fwSubArgs("custom_class", []any{pw.Name("My Class")}, fwCall("ascent", pw.Real(1)), fwCall("flags", pw.Int(4))),
				fwCall("custom_class_ref", pw.Name("Other"), pw.Ref(15)),
				fwSubArgs("cjk_class", []any{pw.Name("HKanaRot")}, fwCall("descent", pw.Real(-2))),
				fwCall("cjk_class_ref", pw.Name("Ruby"), pw.Ref(16)), fwCall("cjk_class_ref", pw.Name("Alphabetic"), pw.Ref(17)),
				fwCall("insert", pw.Name("Z"), pw.Text("z"))),
			fwCall("cid_set", pw.Ref(18)),
		}},
		{ID: 6, Start: "calls.file_spec", Dict: fileSpec},
		{ID: 7, Start: "calls.cmap", Data: fwData("begincmap"), Filter: "FlateDecode", Dict: [][2]any{
			fwCall("name", pw.Name("Custom")), fwCall("system_info", info...), fwCall("writing_mode", pw.Int(1)),
			fwCall("writing_mode", pw.Int(0)), fwCall("use_cmap_stream", pw.Ref(10)), fwCall("use_cmap_predefined", pw.Name("Identity-H")),
		}},
		{ID: 8, Start: "calls.unicode_cmap", Dict: cmapCalls16},
		{ID: 9, Start: "calls.unicode_cmap", Dict: cmapCalls8},
		{ID: 10, Start: "calls.image_xobject", Data: fwData("\x00\x01\x02"), Filter: "DCTDecode", Dict: [][2]any{
			fwCall("width", pw.Int(3)), fwCall("height", pw.Int(1)), fwCall("color_space_name", pw.Name("DeviceGray")),
			fwCall("bits_per_component", pw.Int(8)), fwCall("image_mask", pw.Bool(false)),
			fwCall("color_mask", pw.Int(0), pw.Int(10), pw.Int(-3)), fwCall("stencil_mask", pw.Ref(19)),
			fwCall("decode", fwReals32(1, 0)...), fwCall("interpolate", pw.Bool(true)), fwCall("alternates", pw.Ref(20), pw.Ref(21)),
			fwCall("alternates"), fwCall("s_mask", pw.Ref(22)), fwCall("s_mask_in_data", pw.Int(2)), fwCall("s_mask_in_data", pw.Int(0)),
			fwCall("struct_parent", pw.Int(4)), fwCall("matte", fwReals32(0.25, 0.5, 1)...), fwCall("metadata", pw.Ref(23)),
			pw.Pair("associated_files", pw.Array(pw.Dict(fileSpec...), pw.Dict())),
		}},
		{ID: 11, Start: "calls.form_xobject", Data: fwData("q 1 0 0 1 0 0 cm Q"), Dict: [][2]any{
			fwCall("bbox", fwReals32(0, 0, 595.5, 842)...), fwCall("matrix", fwReals32(1, 0, 0, 1, 10.5, -3)...),
			fwSub("group", fwCall("transparency"), fwCall("isolated", pw.Bool(true)), fwCall("knockout", pw.Bool(false)),
				fwCall("insert", pw.Name("CS"), pw.Name("DeviceRGB"))),
			fwCall("struct_parent", pw.Int(1)), fwCall("struct_parents", pw.Int(2)),
			fwSub("reference", fwSub("file_spec", fileSpec...), fwCall("page_number", pw.Int(0)),
				fwCall("page_label", pw.Text("iv")), fwCall("id", pw.Str([]byte{0, 1}), pw.Str(nil))),
			fwCall("metadata", pw.Ref(23)), fwCall("last_modified", pw.Int(2026)),
			fwCall("last_modified", pw.Int(2026), pw.Int(0), pw.Int(40), pw.Int(25), pw.Int(70), pw.Int(59), pw.Int(-30), pw.Int(99)),
			fwCall("last_modified", pw.Int(12000), pw.Int(11), pw.Int(18), pw.Int(9), pw.Int(5), pw.Int(1), pw.Int(0)),
			pw.Pair("associated_files", pw.Array(pw.Dict(fwCall("path", pw.Str([]byte("x")))))),
		}},
	}
}

// TestPDFWriterGoFont compares the Go writer with pdf-writer for lists of
// calls that make each call of the typed writers of font.rs, xobject.rs and
// files.rs.
func TestPDFWriterGoFont(t *testing.T) {
	no := false
	objects := fwObjects()
	compareWriterFont(t, []writerQuestion{
		{PdfWriter: writerSpec{Objects: objects}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: objects}},
		{PdfWriter: writerSpec{Chunk: true, Objects: objects}},
		{PdfWriter: writerSpec{Chunk: true, Pretty: &no, Objects: objects}},
		// test_chunk of src/chunk.rs, written into one chunk.
		{PdfWriter: writerSpec{Objects: []writerObject{
			{ID: 1, Start: "calls.type3_font", Dict: [][2]any{fwCall("font_descriptor", pw.Ref(2))}},
			{ID: 2, Start: "calls.font_descriptor", Dict: [][2]any{fwCall("name", pw.Name("MyFont"))}},
		}}},
		// A UnicodeCmap with no pair, and one with exactly 100 pairs.
		{PdfWriter: writerSpec{Chunk: true, Objects: []writerObject{
			{ID: 1, Start: "calls.unicode_cmap", Dict: [][2]any{
				fwCall("new_u16", pw.Name(""), pw.Str(nil), pw.Str([]byte("a(b)")), pw.Int(-7), pw.Int(1)),
			}},
			{ID: 2, Start: "calls.unicode_cmap", Dict: fwHundredPairs()},
		}}},
		// A form XObject without data is an error in both writers.
		{PdfWriter: writerSpec{Objects: []writerObject{{ID: 1, Start: "calls.form_xobject"}}}},
		// A duplicate number panics in both writers.
		{PdfWriter: writerSpec{Objects: []writerObject{
			{ID: 3, Start: "calls.type0_font"}, {ID: 3, Start: "calls.cid_font"},
		}}},
	})
}

func fwHundredPairs() [][2]any {
	calls := [][2]any{fwCall("new_u8", pw.Name("H"), pw.Str([]byte("R")), pw.Str([]byte("O")), pw.Int(2), pw.Int(0))}
	for g := range 100 {
		calls = append(calls, fwCall("pair_with_multiple", pw.Int(int32(g)), pw.Text("ab")))
	}
	return calls
}

// TestPDFWriterGoFontRandom compares the Go writer with pdf-writer for
// random lists of typed calls, mixed with the random objects of
// TestPDFWriterGoRandom.
func TestPDFWriterGoFontRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 6))
	questions := make([]writerQuestion, 300)
	for i := range questions {
		questions[i] = randomFontQuestion(r)
	}
	compareWriterFont(t, questions)
}

// randomFontQuestion makes a random question of typed calls.
func randomFontQuestion(r *rand.Rand) writerQuestion {
	q := writerSpec{Objects: []writerObject{}}
	if r.IntN(2) == 0 {
		p := r.IntN(2) == 0
		q.Pretty = &p
	}
	q.Chunk = r.IntN(4) == 0
	used := map[int]bool{}
	for range r.IntN(6) {
		id := 1 + r.IntN(30)
		if used[id] {
			continue
		}
		used[id] = true
		o := writerObject{ID: id}
		switch r.IntN(12) {
		case 0:
			o.Value = randomValue(r, 2)
		case 1:
			o.Start, o.Dict = "calls.type1_font", fwRandomCalls(r, fwType1Calls, 2)
		case 2:
			o.Start, o.Dict = "calls.type3_font", fwRandomCalls(r, fwType3Calls, 2)
		case 3:
			o.Start, o.Dict = "calls.type0_font", fwRandomCalls(r, fwType0Calls, 2)
		case 4:
			o.Start, o.Dict = "calls.cid_font", fwRandomCalls(r, fwCidFontCalls, 2)
		case 5:
			o.Start, o.Dict = "calls.font_descriptor", fwRandomCalls(r, fwFontDescriptorCalls, 2)
		case 6:
			o.Start, o.Dict = "calls.file_spec", fwRandomCalls(r, fwFileSpecCalls, 2)
		case 7:
			o.Start, o.Dict = "calls.unicode_cmap", fwRandomUnicodeCmap(r)
		case 8:
			o.Start, o.Dict = "calls.cmap", fwRandomCalls(r, fwCmapCalls, 2)
		case 9:
			o.Start, o.Dict = "calls.image_xobject", fwRandomCalls(r, fwImageCalls, 2)
		default:
			o.Start, o.Dict = "calls.form_xobject", fwRandomCalls(r, fwFormCalls, 2)
		}
		if strings.HasSuffix(o.Start, "_xobject") || o.Start == "calls.cmap" {
			// The data is never empty, because the field data of the
			// question leaves out an empty string.
			o.Data = base64.StdEncoding.EncodeToString(append([]byte{'d'}, randomBytes(r)...))
			if r.IntN(2) == 0 {
				o.Filter = []string{"FlateDecode", "DCTDecode", "JPXDecode"}[r.IntN(3)]
			}
		}
		q.Objects = append(q.Objects, o)
	}
	return writerQuestion{PdfWriter: q}
}

// fwMaker makes one random call. depth limits the writers inside it.
type fwMaker func(r *rand.Rand, depth int) [2]any

// fwRandomCalls makes up to seven random calls from the makers.
func fwRandomCalls(r *rand.Rand, makers []fwMaker, depth int) [][2]any {
	var calls [][2]any
	for range r.IntN(8) {
		calls = append(calls, makers[r.IntN(len(makers))](r, depth))
	}
	return calls
}

func fwRName(r *rand.Rand) any { return pw.Name(randomText(r)) }
func fwRReal(r *rand.Rand) any { return pw.Real(randomReal(r)) }
func fwRRef(r *rand.Rand) any  { return pw.Ref(1 + r.IntN(30)) }
func fwRStr(r *rand.Rand) any  { return pw.Str(randomBytes(r)) }
func fwRText(r *rand.Rand) any { return pw.Text(randomText(r)) }
func fwRBool(r *rand.Rand) any { return pw.Bool(r.IntN(2) == 0) }
func fwRInt(r *rand.Rand) any  { return pw.Int(int32(r.Uint32()) >> r.IntN(32)) }
func fwRU8(r *rand.Rand) any   { return pw.Int(int32(r.IntN(256))) }
func fwRU16(r *rand.Rand) any  { return pw.Int(int32(r.IntN(65536))) }

func fwRReals(r *rand.Rand, n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = fwRReal(r)
	}
	return out
}

// fwOne1 makes a maker of a call with one argument.
func fwOne1(method string, arg func(*rand.Rand) any) fwMaker {
	return func(r *rand.Rand, _ int) [2]any { return fwCall(method, arg(r)) }
}

// fwRealsN makes a maker of a call with up to n reals, or exactly n when
// exact is true.
func fwRealsN(method string, n int, exact bool) fwMaker {
	return func(r *rand.Rand, _ int) [2]any {
		k := n
		if !exact {
			k = r.IntN(n + 1)
		}
		return fwCall(method, fwRReals(r, k)...)
	}
}

// fwSubCalls makes a maker of a call that starts another writer. At depth 0,
// the writer gets no call.
func fwSubCalls(method string, makers *[]fwMaker) fwMaker {
	return func(r *rand.Rand, depth int) [2]any {
		if depth <= 0 {
			return fwSub(method)
		}
		return fwSub(method, fwRandomCalls(r, *makers, depth-1)...)
	}
}

func fwInsertCall(r *rand.Rand, depth int) [2]any {
	return fwCall("insert", fwRName(r), randomValue(r, max(depth, 0)))
}

func fwItemCall(r *rand.Rand, depth int) [2]any {
	return fwCall("item", randomValue(r, max(depth, 0)))
}

func fwPick(r *rand.Rand, names ...string) any { return pw.Name(names[r.IntN(len(names))]) }

func fwEnumNames[T fmt.Stringer](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = v.String()
	}
	return out
}

func fwRInfo(r *rand.Rand) []any { return []any{fwRStr(r), fwRStr(r), fwRInt(r)} }

var (
	fwType1Calls, fwType3Calls, fwEncodingCalls, fwDifferencesCalls, fwType0Calls, fwCidFontCalls, fwWidthsCalls,
	fwFontDescriptorCalls, fwOverrideCalls, fwCmapCalls, fwImageCalls, fwFormCalls, fwGroupCalls, fwReferenceCalls,
	fwFileSpecCalls []fwMaker
)

func init() {
	fwType1Calls = []fwMaker{
		fwOne1("name", fwRName), fwOne1("base_font", fwRName), fwOne1("first_char", fwRU8), fwOne1("last_char", fwRU8),
		fwRealsN("widths", 6, false), fwOne1("font_descriptor", fwRRef), fwOne1("encoding_predefined", fwRName),
		fwSubCalls("encoding_custom", &fwEncodingCalls), fwOne1("to_unicode", fwRRef), fwInsertCall,
	}
	fwType3Calls = []fwMaker{
		fwOne1("name", fwRName), fwRealsN("bbox", 4, true), fwRealsN("matrix", 6, true),
		func(r *rand.Rand, _ int) [2]any {
			var a []any
			for range r.IntN(4) {
				a = append(a, fwRName(r), fwRRef(r))
			}
			return fwCall("char_procs", a...)
		},
		fwOne1("encoding_predefined", fwRName), fwSubCalls("encoding_custom", &fwEncodingCalls),
		fwOne1("first_char", fwRU8), fwOne1("last_char", fwRU8), fwRealsN("widths", 6, false),
		fwOne1("font_descriptor", fwRRef), fwOne1("to_unicode", fwRRef), fwInsertCall,
	}
	fwEncodingCalls = []fwMaker{fwOne1("base_encoding", fwRName), fwSubCalls("differences", &fwDifferencesCalls), fwInsertCall}
	fwDifferencesCalls = []fwMaker{
		func(r *rand.Rand, _ int) [2]any {
			a := []any{fwRU8(r)}
			for range r.IntN(4) {
				a = append(a, fwRName(r))
			}
			return fwCall("consecutive", a...)
		},
		fwItemCall,
	}
	fwType0Calls = []fwMaker{
		fwOne1("base_font", fwRName), fwOne1("encoding_predefined", fwRName), fwOne1("encoding_cmap", fwRRef),
		fwOne1("descendant_font", fwRRef), fwOne1("to_unicode", fwRRef), fwInsertCall,
	}
	fwCidFontCalls = []fwMaker{
		fwOne1("subtype", func(r *rand.Rand) any { return fwPick(r, fwEnumNames(fwCIDFontTypes)...) }),
		fwOne1("base_font", fwRName),
		func(r *rand.Rand, _ int) [2]any { return fwCall("system_info", fwRInfo(r)...) },
		fwOne1("font_descriptor", fwRRef), fwOne1("default_width", fwRReal), fwSubCalls("widths", &fwWidthsCalls),
		fwOne1("cid_to_gid_map_predefined", fwRName), fwOne1("cid_to_gid_map_stream", fwRRef), fwInsertCall,
	}
	fwWidthsCalls = []fwMaker{
		func(r *rand.Rand, _ int) [2]any {
			return fwCall("consecutive", append([]any{fwRU16(r)}, fwRReals(r, r.IntN(5))...)...)
		},
		func(r *rand.Rand, _ int) [2]any { return fwCall("same", fwRU16(r), fwRU16(r), fwRReal(r)) },
		fwItemCall,
	}
	fwFontDescriptorCalls = []fwMaker{
		fwOne1("name", fwRName), fwOne1("family", fwRStr),
		fwOne1("stretch", func(r *rand.Rand) any { return fwPick(r, fwEnumNames(fwFontStretches)...) }),
		fwOne1("weight", fwRU16), fwOne1("flags", fwRInt), fwRealsN("bbox", 4, true),
		fwOne1("italic_angle", fwRReal), fwOne1("ascent", fwRReal), fwOne1("descent", fwRReal), fwOne1("leading", fwRReal),
		fwOne1("cap_height", fwRReal), fwOne1("x_height", fwRReal), fwOne1("stem_v", fwRReal), fwOne1("stem_h", fwRReal),
		fwOne1("avg_width", fwRReal), fwOne1("max_width", fwRReal), fwOne1("missing_width", fwRReal),
		fwOne1("font_file", fwRRef), fwOne1("font_file2", fwRRef), fwOne1("font_file3", fwRRef), fwOne1("char_set", fwRStr),
		func(r *rand.Rand, _ int) [2]any {
			a := make([]any, 12)
			for i := range a {
				a[i] = fwRU8(r)
			}
			return fwCall("style", a...)
		},
		fwSubCalls("descriptor_override", &fwOverrideCalls), fwOne1("cid_set", fwRRef), fwInsertCall,
	}
	fwOverrideCalls = []fwMaker{
		func(r *rand.Rand, depth int) [2]any {
			return fwSubArgs("custom_class", []any{fwRName(r)}, fwRandomCalls(r, fwFontDescriptorCalls, depth-1)...)
		},
		func(r *rand.Rand, _ int) [2]any { return fwCall("custom_class_ref", fwRName(r), fwRRef(r)) },
		func(r *rand.Rand, depth int) [2]any {
			return fwSubArgs("cjk_class", []any{fwPick(r, fwEnumNames(fwCJKClasses)...)},
				fwRandomCalls(r, fwFontDescriptorCalls, depth-1)...)
		},
		func(r *rand.Rand, _ int) [2]any {
			return fwCall("cjk_class_ref", fwPick(r, fwEnumNames(fwCJKClasses)...), fwRRef(r))
		},
		fwInsertCall,
	}
	fwCmapCalls = []fwMaker{
		fwOne1("name", fwRName),
		func(r *rand.Rand, _ int) [2]any { return fwCall("system_info", fwRInfo(r)...) },
		fwOne1("writing_mode", func(r *rand.Rand) any { return pw.Int(int32(r.IntN(2))) }),
		fwOne1("use_cmap_stream", fwRRef), fwOne1("use_cmap_predefined", fwRName), fwInsertCall,
	}
	fwImageCalls = []fwMaker{
		fwOne1("width", fwRInt), fwOne1("height", fwRInt), fwOne1("color_space_name", fwRName),
		fwOne1("bits_per_component", fwRInt), fwOne1("image_mask", fwRBool),
		func(r *rand.Rand, _ int) [2]any {
			var a []any
			for range r.IntN(6) {
				a = append(a, fwRInt(r))
			}
			return fwCall("color_mask", a...)
		},
		fwOne1("stencil_mask", fwRRef), fwRealsN("decode", 6, false), fwOne1("interpolate", fwRBool),
		func(r *rand.Rand, _ int) [2]any {
			var a []any
			for range r.IntN(4) {
				a = append(a, fwRRef(r))
			}
			return fwCall("alternates", a...)
		},
		fwOne1("s_mask", fwRRef), fwOne1("s_mask_in_data", func(r *rand.Rand) any { return pw.Int(int32(r.IntN(3))) }),
		fwOne1("struct_parent", fwRInt), fwRealsN("matte", 4, false), fwOne1("metadata", fwRRef),
		fwAssociatedFiles, fwInsertCall,
	}
	fwFormCalls = []fwMaker{
		fwRealsN("bbox", 4, true), fwRealsN("matrix", 6, true), fwSubCalls("group", &fwGroupCalls),
		fwOne1("struct_parent", fwRInt), fwOne1("struct_parents", fwRInt), fwSubCalls("reference", &fwReferenceCalls),
		fwOne1("metadata", fwRRef),
		func(r *rand.Rand, _ int) [2]any {
			a := []any{pw.Int(int32(r.IntN(12000)))}
			for i := range r.IntN(8) {
				if i == 5 {
					a = append(a, pw.Int(int32(r.IntN(256)-128)))
				} else {
					a = append(a, fwRU8(r))
				}
			}
			return fwCall("last_modified", a...)
		},
		fwAssociatedFiles, fwInsertCall,
	}
	fwGroupCalls = []fwMaker{
		func(*rand.Rand, int) [2]any { return fwCall("transparency") },
		fwOne1("isolated", fwRBool), fwOne1("knockout", fwRBool), fwInsertCall,
	}
	fwReferenceCalls = []fwMaker{
		fwSubCalls("file_spec", &fwFileSpecCalls), fwOne1("page_number", fwRInt), fwOne1("page_label", fwRText),
		func(r *rand.Rand, _ int) [2]any { return fwCall("id", fwRStr(r), fwRStr(r)) },
		fwInsertCall,
	}
	fwFileSpecCalls = []fwMaker{
		fwOne1("file_system", fwRName), fwOne1("path", fwRStr), fwOne1("unic_file", fwRText), fwOne1("volatile", fwRBool),
		fwOne1("description", fwRText), fwOne1("embedded_file", fwRRef), fwOne1("embedded_file_with_unicode", fwRRef),
		fwOne1("association_kind", func(r *rand.Rand) any { return fwPick(r, fwEnumNames(fwAssociationKinds)...) }),
		fwInsertCall,
	}
}

func fwAssociatedFiles(r *rand.Rand, depth int) [2]any {
	var files []any
	for range r.IntN(3) {
		if depth <= 0 {
			files = append(files, pw.Dict())
			continue
		}
		files = append(files, pw.Dict(fwRandomCalls(r, fwFileSpecCalls, depth-1)...))
	}
	return pw.Pair("associated_files", pw.Array(files...))
}

// fwRandomUnicodeCmap makes the calls of a random UnicodeCmap, sometimes with
// more than 100 pairs.
func fwRandomUnicodeCmap(r *rand.Rand) [][2]any {
	bits := r.IntN(2)
	method, top := "new_u8", 256
	if bits == 1 {
		method, top = "new_u16", 65536
	}
	calls := [][2]any{fwCall(method, fwRName(r), fwRStr(r), fwRStr(r), fwRInt(r), pw.Int(int32(r.IntN(2))))}
	n := r.IntN(20)
	if r.IntN(4) == 0 {
		n = 90 + r.IntN(130)
	}
	for range n {
		g := pw.Int(int32(r.IntN(top)))
		t := randomText(r)
		if r.IntN(2) == 0 && utf8.RuneCountInString(t) > 0 {
			first, _ := utf8.DecodeRuneInString(t)
			calls = append(calls, fwCall("pair", g, pw.Text(string(first))))
			continue
		}
		calls = append(calls, fwCall("pair_with_multiple", g, pw.Text(t)))
	}
	return calls
}
