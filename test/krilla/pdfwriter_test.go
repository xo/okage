package krilla

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
)

// The Go side of the question "pdf_writer": goWriter makes the calls of a
// question with internal/pdfwriter, as rust/src/writer.rs makes them with
// pdf-writer. It reads the question from its JSON, as the oracle does, so
// that both sides see the same numbers.

// writerDicts holds the typed writers of a chunk that start a dictionary,
// by the name of the field "start". A unit of phase 12 that ports a typed
// writer adds it here from its own file, in a function init, such as:
//
//	writerDicts["pages"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
//		return c.Pages(id).Dict
//	}
var writerDicts = map[string]func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict{}

// writerStreams holds the typed writers of a chunk that start a stream, by
// the name of the field "start", as writerDicts does. "stream" is
// Chunk::stream.
var writerStreams = map[string]func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream{
	"stream": func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
		return c.Stream(id, data)
	},
}

// pdfDicts holds the typed writers of a Pdf, catalog and document_info, by
// the name of the field "start". structure.go adds them.
var pdfDicts = map[string]func(p *pdfwriter.Pdf, id pdfwriter.Ref) *pdfwriter.Dict{}

// goContent writes the operations of a content stream, as the function
// content of writer.rs does. content.go sets it.
var goContent func(ops []any, settings pdfwriter.Settings) ([]byte, error)

// goWriter answers a question "pdf_writer" with the Go writer. A panic of
// the writer is an error with the message of the panic, as in the oracle.
func goWriter(question []byte) (out []byte, err error) {
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

	var renumbered *pdfwriter.Chunk
	if pairs, ok := q["renumber"].([]any); ok {
		c := pdfwriter.NewChunkWithSettings(settings)
		for _, o := range objects {
			if err := writeObject(c, as[map[string]any](o), settings); err != nil {
				return nil, err
			}
		}
		m := map[pdfwriter.Ref]pdfwriter.Ref{}
		for _, p := range pairs {
			p := as[[]any](p)
			old, err := reference(p[0])
			if err != nil {
				return nil, err
			}
			n, err := reference(p[1])
			if err != nil {
				return nil, err
			}
			m[old] = n
		}
		renumbered = c.Renumber(func(old pdfwriter.Ref) pdfwriter.Ref {
			if n, ok := m[old]; ok {
				return n
			}
			return old
		})
	}
	if chunk && renumbered != nil {
		return renumbered.AsBytes(), nil
	}
	if chunk {
		c := pdfwriter.NewChunkWithSettings(settings)
		for _, o := range objects {
			if err := writeObject(c, as[map[string]any](o), settings); err != nil {
				return nil, err
			}
		}
		return c.AsBytes(), nil
	}

	pdf := pdfwriter.NewPdfWithSettings(settings)
	if v, ok := q["version"].([]any); ok {
		pdf.SetVersion(uint8(num(v[0])), uint8(num(v[1])))
	}
	if v, ok := q["binary_marker"].([]any); ok {
		var marker [4]byte
		for i := range marker {
			marker[i] = byte(num(v[i]))
		}
		pdf.SetBinaryMarker(marker)
	}
	if v, ok := q["file_id"].([]any); ok {
		a, err := hex.DecodeString(as[string](v[0]))
		if err != nil {
			return nil, err
		}
		b, err := hex.DecodeString(as[string](v[1]))
		if err != nil {
			return nil, err
		}
		pdf.SetFileID(a, b)
	}
	if renumbered != nil {
		pdf.Extend(renumbered)
	}
	for _, o := range objects {
		if renumbered != nil {
			break
		}
		o := as[map[string]any](o)
		start, _ := o["start"].(string)
		if f, ok := pdfDicts[start]; ok {
			id, err := reference(o["id"])
			if err != nil {
				return nil, err
			}
			if err := writePairs(f(pdf, id), o["dict"]); err != nil {
				return nil, err
			}
			continue
		}
		if err := writeObject(&pdf.Chunk, o, settings); err != nil {
			return nil, err
		}
	}

	var filter *pdfwriter.XRefFilter
	switch f := q["xref_filter"].(type) {
	case string:
		single, err := filterOf(f)
		if err != nil {
			return nil, err
		}
		x := pdfwriter.XRefFilterSingle(single)
		filter = &x
	case []any:
		var fs []pdfwriter.Filter
		for _, name := range f {
			x, err := filterOf(as[string](name))
			if err != nil {
				return nil, err
			}
			fs = append(fs, x)
		}
		x := pdfwriter.XRefFilterMultiple(fs...)
		filter = &x
	}
	xref, hasXref := q["xref_stream"]
	switch {
	case hasXref && filter != nil:
		id, err := reference(xref)
		if err != nil {
			return nil, err
		}
		return pdf.FinishWithXRefStreamAndFilter(id, func(data []byte) ([]byte, pdfwriter.XRefFilter) {
			return data, *filter
		}), nil
	case hasXref:
		id, err := reference(xref)
		if err != nil {
			return nil, err
		}
		return pdf.FinishWithXRefStream(id), nil
	case filter != nil:
		return nil, errors.New("xref_filter needs xref_stream")
	}
	return pdf.Finish(), nil
}

// writeObject is write_object of writer.rs.
func writeObject(c *pdfwriter.Chunk, o map[string]any, settings pdfwriter.Settings) error {
	id, err := reference(o["id"])
	if err != nil {
		return err
	}
	var data []byte
	hasData := false
	switch {
	case o["data"] != nil:
		data, err = base64.StdEncoding.DecodeString(as[string](o["data"]))
		if err != nil {
			return fmt.Errorf("decoding the data of object %d: %w", id.Get(), err)
		}
		hasData = true
	case o["content"] != nil:
		if goContent == nil {
			return errors.New("content.rs is not ported yet")
		}
		data, err = goContent(as[[]any](o["content"]), settings)
		if err != nil {
			return err
		}
		hasData = true
	}
	var filter *pdfwriter.Filter
	if f, ok := o["filter"].(string); ok {
		x, err := filterOf(f)
		if err != nil {
			return err
		}
		filter = &x
	}
	start, hasStart := o["start"].(string)

	stream := func(s *pdfwriter.Stream) error {
		if filter != nil {
			s.Filter(*filter)
		}
		return writePairs(s.Dict, o["dict"])
	}
	switch {
	case !hasStart && !hasData:
		if v, ok := o["value"]; ok {
			return writeValue(c.Indirect(id), v)
		}
		return writePairs(c.Indirect(id).Dict(), o["dict"])
	case !hasStart && hasData:
		return stream(c.Stream(id, data))
	case hasData && writerStreams[start] != nil:
		return stream(writerStreams[start](c, id, data))
	case !hasData && writerDicts[start] != nil:
		return writePairs(writerDicts[start](c, id), o["dict"])
	}
	return fmt.Errorf("object %d: the start %q is not known", id.Get(), start)
}

// writePairs writes the pairs of a field "dict" into a dictionary.
func writePairs(d *pdfwriter.Dict, pairs any) error {
	list, _ := pairs.([]any)
	for _, p := range list {
		p := as[[]any](p)
		if err := writeValue(d.Insert(pdfwriter.Name(as[string](p[0]))), p[1]); err != nil {
			return err
		}
	}
	return nil
}

// writeValue is write_value of writer.rs.
func writeValue(obj pdfwriter.Obj, v any) error {
	if v == "null" {
		obj.Primitive(pdfwriter.Null{})
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok || len(m) != 1 {
		return fmt.Errorf("the value %v is not known", v)
	}
	for k, x := range m {
		switch k {
		case "bool":
			obj.Primitive(pdfwriter.Bool(as[bool](x)))
		case "int":
			n, err := strconv.ParseInt(string(as[json.Number](x)), 10, 32)
			if err != nil {
				return err
			}
			obj.Primitive(pdfwriter.Int(n))
		case "real":
			// serde reads an f32 as an f64, and then rounds it to an f32.
			f, err := strconv.ParseFloat(string(as[json.Number](x)), 64)
			if err != nil {
				return err
			}
			obj.Primitive(pdfwriter.Real(float32(f)))
		case "name":
			obj.Primitive(pdfwriter.Name(as[string](x)))
		case "str":
			b, err := hex.DecodeString(as[string](x))
			if err != nil {
				return err
			}
			obj.Primitive(pdfwriter.Str(b))
		case "text":
			obj.Primitive(pdfwriter.TextStr(as[string](x)))
		case "ref":
			r, err := reference(x)
			if err != nil {
				return err
			}
			obj.Primitive(r)
		case "array":
			a := obj.Array()
			for _, item := range as[[]any](x) {
				if err := writeValue(a.Push(), item); err != nil {
					return err
				}
			}
		case "dict":
			if err := writePairs(obj.Dict(), x); err != nil {
				return err
			}
		default:
			return fmt.Errorf("the value %v is not known", v)
		}
	}
	return nil
}

// as returns v as a T. It panics if v is not a T, and goWriter gives the
// panic as an error.
func as[T any](v any) T {
	t, ok := v.(T)
	if !ok {
		panic(fmt.Sprintf("the value %v is not a %T", v, t))
	}
	return t
}

// num reads a JSON number as an int64.
func num(v any) int64 {
	n, err := as[json.Number](v).Int64()
	if err != nil {
		panic(err)
	}
	return n
}

// reference is reference of writer.rs.
func reference(v any) (pdfwriter.Ref, error) {
	id := num(v)
	if id < 1 || id > math.MaxInt32 {
		return pdfwriter.Ref{}, fmt.Errorf("the object number %d is not positive", id)
	}
	return pdfwriter.NewRef(int32(id)), nil
}

// filterOf is filter of writer.rs.
func filterOf(name string) (pdfwriter.Filter, error) {
	for _, f := range []pdfwriter.Filter{
		pdfwriter.FilterASCIIHexDecode, pdfwriter.FilterASCII85Decode, pdfwriter.FilterLZWDecode,
		pdfwriter.FilterFlateDecode, pdfwriter.FilterRunLengthDecode, pdfwriter.FilterCCITTFaxDecode,
		pdfwriter.FilterJBIG2Decode, pdfwriter.FilterDCTDecode, pdfwriter.FilterJPXDecode, pdfwriter.FilterCrypt,
	} {
		if f.String() == name {
			return f, nil
		}
	}
	return 0, fmt.Errorf("the filter %q is not known", name)
}

// compareWriter asks the oracle each question, answers it with goWriter,
// and compares the bytes. When the oracle gives an error, goWriter must
// give one too, and a panic must have the same message.
func compareWriter(t *testing.T, questions []writerQuestion) {
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
		got, gerr := goWriter(b)
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

// TestPDFWriterGo compares the Go writer with pdf-writer for lists of calls
// of buf.rs, object.rs, chunk.rs, lib.rs and renumber.rs: values of each
// kind, streams, chunks, renumbering, the header, the cross-reference table
// and stream, and the trailer.
func TestPDFWriterGo(t *testing.T) {
	no, yes := false, true
	all := []any{
		pw.Null, pw.Bool(true), pw.Bool(false), pw.Int(0), pw.Int(-7), pw.Int(math.MaxInt32), pw.Int(math.MinInt32),
		pw.Real(0), pw.Real(-0.5), pw.Real(4.22), pw.Real(1.184e-7), pw.Real(4.2e13), pw.Real(3e9), pw.Real(2147483648),
		pw.Name("Type"), pw.Name("A B#/(x)"), pw.Name("é\x01"), pw.Name(""),
		pw.Str([]byte("a(b)\n")), pw.Str([]byte(")(")), pw.Str([]byte("\xff\x00")), pw.Str(nil), pw.Str([]byte("\\\r\t\b\f\x7f")),
		pw.Text("Hallo"), pw.Text("é😀"), pw.Text("tab\there"), pw.Text(""),
		pw.Ref(3),
		pw.Array(), pw.Array(pw.Int(1), pw.Name("N"), pw.Array(pw.Real(1.5)), pw.Dict(pw.Pair("K", pw.Null))),
		pw.Dict(), pw.Dict(pw.Pair("A", pw.Dict(pw.Pair("B", pw.Dict(pw.Pair("C", pw.Array(pw.Dict()))))))),
	}
	var pairs [][2]any
	for i, v := range all {
		pairs = append(pairs, pw.Pair(fmt.Sprintf("K%d", i), v))
	}
	objects := []writerObject{
		{ID: 1, Value: pw.Array(all...)},
		{ID: 2, Dict: pairs},
		{ID: 4, Data: base64.StdEncoding.EncodeToString([]byte("BT 1 0 R endstream ET")), Filter: "FlateDecode",
			Dict: [][2]any{pw.Pair("Ref", pw.Ref(1)), pw.Pair("DecodeParms", pw.Dict(pw.Pair("Predictor", pw.Int(15))))}},
		{ID: 3, Start: "stream", Data: base64.StdEncoding.EncodeToString([]byte("q Q"))},
		{ID: 9, Value: pw.Ref(2)},
	}
	questions := []writerQuestion{
		{PdfWriter: writerSpec{Objects: []writerObject{}}},
		{PdfWriter: writerSpec{Objects: objects}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: objects}},
		{PdfWriter: writerSpec{Chunk: true, Objects: objects}},
		{PdfWriter: writerSpec{Chunk: true, Pretty: &no, Objects: objects}},
		{PdfWriter: writerSpec{
			Version: &[2]int{2, 0}, BinaryMarker: &[4]int{'A', 'B', 'C', 'D'},
			FileID: &[2]string{"00ff", ""}, Objects: objects,
		}},
		{PdfWriter: writerSpec{Version: &[2]int{12, 4}, Objects: objects, XrefStream: 7}},
		{PdfWriter: writerSpec{Objects: objects, XrefStream: 70000, XrefFilter: "FlateDecode"}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: objects, XrefStream: 8, XrefFilter: []string{"ASCIIHexDecode", "FlateDecode"}}},
		{PdfWriter: writerSpec{Pretty: &yes, Objects: objects, Renumber: [][2]int{{1, 5}, {2, 6}, {3, 1}, {9, 2}}}},
		{PdfWriter: writerSpec{Chunk: true, Objects: objects, Renumber: [][2]int{{4, 400}, {1, 1}}}},
		{PdfWriter: writerSpec{Pretty: &no, Objects: objects, Renumber: [][2]int{}, XrefStream: 1000}},
		// A duplicate number panics in both writers.
		{PdfWriter: writerSpec{Objects: []writerObject{{ID: 3, Value: pw.Int(1)}, {ID: 3, Value: pw.Int(1)}}}},
		{PdfWriter: writerSpec{Objects: objects, XrefStream: 4}},
	}
	compareWriter(t, questions)
}

// TestPDFWriterGoRandom compares the Go writer with pdf-writer for random
// lists of calls.
func TestPDFWriterGoRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 12))
	questions := make([]writerQuestion, 300)
	for i := range questions {
		questions[i] = randomQuestion(r)
	}
	compareWriter(t, questions)
}

// randomQuestion makes a random question "pdf_writer".
func randomQuestion(r *rand.Rand) writerQuestion {
	q := writerSpec{Objects: []writerObject{}}
	if r.IntN(2) == 0 {
		p := r.IntN(2) == 0
		q.Pretty = &p
	}
	q.Chunk = r.IntN(5) == 0
	if r.IntN(4) == 0 {
		q.Version = &[2]int{r.IntN(12), r.IntN(12)}
	}
	if r.IntN(4) == 0 {
		q.BinaryMarker = &[4]int{r.IntN(256), r.IntN(256), r.IntN(256), r.IntN(256)}
	}
	if r.IntN(4) == 0 {
		q.FileID = &[2]string{hex.EncodeToString(randomBytes(r)), hex.EncodeToString(randomBytes(r))}
	}
	used := map[int]bool{}
	for range r.IntN(8) {
		id := 1 + r.IntN(30)
		used[id] = true
		o := writerObject{ID: id}
		switch r.IntN(4) {
		case 0:
			o.Value = randomValue(r, 3)
		case 1:
			o.Dict = randomPairs(r, 3)
		default:
			o.Data = base64.StdEncoding.EncodeToString(randomBytes(r))
			if r.IntN(2) == 0 {
				o.Start = "stream"
			}
			if r.IntN(2) == 0 {
				o.Filter = []string{"FlateDecode", "DCTDecode", "Crypt", "LZWDecode"}[r.IntN(4)]
			}
			o.Dict = randomPairs(r, 2)
		}
		q.Objects = append(q.Objects, o)
	}
	if !q.Chunk && r.IntN(3) == 0 {
		q.XrefStream = 1 + r.IntN(40)
		switch r.IntN(3) {
		case 0:
			q.XrefFilter = "FlateDecode"
		case 1:
			q.XrefFilter = []string{"ASCII85Decode", "FlateDecode"}
		}
	}
	if r.IntN(3) == 0 {
		q.Renumber = [][2]int{}
		for id := range used {
			if r.IntN(2) == 0 {
				q.Renumber = append(q.Renumber, [2]int{id, 1 + r.IntN(60)})
			}
		}
	}
	return writerQuestion{PdfWriter: q}
}

// randomBytes gives up to 15 random bytes, often with the bytes that PDF
// treats in a special way.
func randomBytes(r *rand.Rand) []byte {
	const special = "()\\\n\r\t\b\f /#%<>[]{}R0 objendstream"
	b := make([]byte, r.IntN(16))
	for i := range b {
		if r.IntN(2) == 0 {
			b[i] = special[r.IntN(len(special))]
		} else {
			b[i] = byte(r.IntN(256))
		}
	}
	return b
}

// randomText gives a random string of UTF-8 text.
func randomText(r *rand.Rand) string {
	runes := []rune{'a', 'Z', '0', ' ', '(', ')', '\\', '\n', '\t', '#', '/', '~', 0x7f, 0x1b, 'é', '€', '😀', 0x10ffff}
	var b strings.Builder
	for range r.IntN(10) {
		b.WriteRune(runes[r.IntN(len(runes))])
	}
	return b.String()
}

// randomReal gives a random float32 from one of a few ranges, so that each
// branch of push_float and push_decimal runs.
func randomReal(r *rand.Rand) float32 {
	switch r.IntN(6) {
	case 0:
		return float32(r.IntN(2001)-1000) / 8
	case 1:
		return float32(r.NormFloat64())
	case 2:
		return float32(r.NormFloat64() * 1e-6)
	case 3:
		return float32(r.NormFloat64() * 1e12)
	case 4:
		return float32(r.IntN(1<<20)) * 4096
	}
	for {
		f := math.Float32frombits(r.Uint32())
		if !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0) {
			return f
		}
	}
}

// randomValue gives a random value of the question, with arrays and
// dictionaries down to the depth.
func randomValue(r *rand.Rand, depth int) any {
	n := 9
	if depth > 0 {
		n = 11
	}
	switch r.IntN(n) {
	case 0:
		return pw.Null
	case 1:
		return pw.Bool(r.IntN(2) == 0)
	case 2:
		return pw.Int(int32(r.Uint32()) >> r.IntN(32))
	case 3:
		return pw.Real(randomReal(r))
	case 4:
		return pw.Name(randomText(r))
	case 5:
		return pw.Str(randomBytes(r))
	case 6:
		return pw.Text(randomText(r))
	case 7, 8:
		return pw.Ref(1 + r.IntN(30))
	case 9:
		var items []any
		for range r.IntN(5) {
			items = append(items, randomValue(r, depth-1))
		}
		return pw.Array(items...)
	}
	return pw.Dict(randomPairs(r, depth-1)...)
}

// randomPairs gives up to four random pairs of a dictionary.
func randomPairs(r *rand.Rand, depth int) [][2]any {
	var pairs [][2]any
	for range r.IntN(5) {
		pairs = append(pairs, pw.Pair(randomText(r), randomValue(r, depth)))
	}
	return pairs
}
