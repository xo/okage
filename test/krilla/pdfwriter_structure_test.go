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

// The Go side of the calls of structure.rs, annotations.rs and actions.rs of
// pdf-writer, unit 7 of phase 12. An object of a question "pdf_writer" with
// the field "structure" starts a typed writer and makes a list of calls on
// it, as the module writer_structure.rs of the oracle describes. goStructure
// makes the same calls with internal/pdfwriter.
//
// A call can have the field "finish", which the oracle ignores. When it is
// true, the Go side calls Finish on the writer that the call starts, after
// the calls of "then". Otherwise the writer finishes with the next call on
// its parent, as the port says. Both must give the bytes of Rust.

func init() {
	writerDicts["pages"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Pages(id).Dict
	}
	writerDicts["page"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Page(id).Dict
	}
	writerDicts["outline"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Outline(id).Dict
	}
	writerDicts["outline_item"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.OutlineItem(id).Dict
	}
	writerDicts["annotation"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref) *pdfwriter.Dict {
		return c.Annotation(id).Dict
	}
	writerStreams["metadata"] = func(c *pdfwriter.Chunk, id pdfwriter.Ref, data []byte) *pdfwriter.Stream {
		return c.Metadata(id, data).Stream
	}
	pdfDicts["catalog"] = func(p *pdfwriter.Pdf, id pdfwriter.Ref) *pdfwriter.Dict {
		return p.Catalog(id).Dict
	}
	pdfDicts["document_info"] = func(p *pdfwriter.Pdf, id pdfwriter.Ref) *pdfwriter.Dict {
		return p.DocumentInfo(id).Dict
	}
}

// structureQuestion asks for the bytes of objects that can have calls of
// typed writers.
type structureQuestion struct {
	PdfWriter structureSpec `json:"pdf_writer"`
}

type structureSpec struct {
	Pretty *bool `json:"pretty,omitempty"`
	Chunk  bool  `json:"chunk,omitempty"`
	// Objects holds values of writerObject and structureObject.
	Objects []any `json:"objects"`
}

// structureObject is an object with the calls of a typed writer.
type structureObject struct {
	ID        int             `json:"id"`
	Start     string          `json:"start"`
	Data      string          `json:"data,omitempty"`
	Structure []structureCall `json:"structure"`
}

// structureCall is a call of a method of a typed writer.
type structureCall struct {
	Call   string          `json:"call"`
	Args   []any           `json:"args,omitempty"`
	Then   []structureCall `json:"then,omitempty"`
	Finish bool            `json:"finish,omitempty"`
}

// stc makes a call.
func stc(name string, args ...any) structureCall {
	return structureCall{Call: name, Args: args}
}

// stThen makes a call that starts a writer and makes the calls on it.
func stThen(name string, calls ...structureCall) structureCall {
	return structureCall{Call: name, Then: calls}
}

// goStructure answers a question with objects that can have calls of typed
// writers. It knows the fields "pretty", "chunk" and "objects", and gives an
// object without calls to writeObject or to pdfDicts, as goWriter does. A
// field "structure" that is null is no calls, as serde reads an Option.
func goStructure(question []byte) (out []byte, err error) {
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
	if chunk, _ := q["chunk"].(bool); chunk {
		c := pdfwriter.NewChunkWithSettings(settings)
		for _, o := range objects {
			o := as[map[string]any](o)
			var err error
			if calls := o["structure"]; calls != nil {
				err = structureChunk(c, o, stParseCalls(calls))
			} else {
				err = writeObject(c, o, settings)
			}
			if err != nil {
				return nil, err
			}
		}
		return c.AsBytes(), nil
	}
	pdf := pdfwriter.NewPdfWithSettings(settings)
	for _, o := range objects {
		o := as[map[string]any](o)
		start, _ := o["start"].(string)
		var err error
		switch calls := o["structure"]; {
		case calls != nil:
			err = structurePdf(pdf, o, stParseCalls(calls))
		case pdfDicts[start] != nil:
			var id pdfwriter.Ref
			if id, err = reference(o["id"]); err == nil {
				err = writePairs(pdfDicts[start](pdf, id), o["dict"])
			}
		default:
			err = writeObject(&pdf.Chunk, o, settings)
		}
		if err != nil {
			return nil, err
		}
	}
	return pdf.Finish(), nil
}

// stCall is a call of a question, read from its JSON.
type stCall struct {
	name   string
	args   []any
	then   []stCall
	finish bool
}

// stParseCalls reads a list of calls.
func stParseCalls(v any) []stCall {
	var calls []stCall
	for _, c := range as[[]any](v) {
		m := as[map[string]any](c)
		x := stCall{name: as[string](m["call"])}
		x.args, _ = m["args"].([]any)
		if t, ok := m["then"]; ok {
			x.then = stParseCalls(t)
		}
		x.finish, _ = m["finish"].(bool)
		calls = append(calls, x)
	}
	return calls
}

// structurePdf is write_pdf of writer_structure.rs.
func structurePdf(pdf *pdfwriter.Pdf, o map[string]any, calls []stCall) error {
	id, err := reference(o["id"])
	if err != nil {
		return err
	}
	switch o["start"] {
	case "catalog":
		return stCatalog(pdf.Catalog(id), calls)
	case "document_info":
		return stDocumentInfo(pdf.DocumentInfo(id), calls)
	}
	return structureChunk(&pdf.Chunk, o, calls)
}

// structureChunk is write of writer_structure.rs.
func structureChunk(c *pdfwriter.Chunk, o map[string]any, calls []stCall) error {
	id, err := reference(o["id"])
	if err != nil {
		return err
	}
	start, ok := o["start"].(string)
	if !ok {
		return fmt.Errorf("object %d: calls need a start", id.Get())
	}
	switch start {
	case "pages":
		return stPages(c.Pages(id), calls)
	case "page":
		return stPage(c.Page(id), calls)
	case "outline":
		return stOutline(c.Outline(id), calls)
	case "outline_item":
		return stOutlineItem(c.OutlineItem(id), calls)
	case "destination":
		return stDestination(c.Destination(id), calls)
	case "destinations":
		return stDestinations(c.Destinations(id), calls)
	case "annotation":
		return stAnnotation(c.Annotation(id), calls)
	case "action":
		return stAction(pdfwriter.Start[pdfwriter.Action](c.Indirect(id)), calls)
	case "page_label":
		return stPageLabel(pdfwriter.Start[pdfwriter.PageLabel](c.Indirect(id)), calls)
	case "page_labels":
		return stNumberTree(pdfwriter.Start[pdfwriter.NumberTree[pdfwriter.Ref]](c.Indirect(id)), calls)
	case "names":
		return stNames(pdfwriter.Start[pdfwriter.Names](c.Indirect(id)), calls)
	case "viewer_preferences":
		return stViewerPreferences(pdfwriter.Start[pdfwriter.ViewerPreferences](c.Indirect(id)), calls)
	case "metadata":
		d, ok := o["data"].(string)
		if !ok {
			return fmt.Errorf("object %d: metadata needs data", id.Get())
		}
		data, err := base64.StdEncoding.DecodeString(d)
		if err != nil {
			return fmt.Errorf("decoding the data of object %d: %w", id.Get(), err)
		}
		m := c.Metadata(id, data)
		for _, x := range calls {
			if x.name == "filter" {
				f, err := filterOf(x.str(0))
				if err != nil {
					return err
				}
				m.Filter(f)
				continue
			}
			if err := stDict(m.Dict, x); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("object %d: the start %q has no calls", id.Get(), start)
}

// stUnknown is the error of a call that is not known.
func stUnknown(c stCall) error {
	return fmt.Errorf("the call %q is not known", c.name)
}

// stDict makes the call "pair" on a dictionary.
func stDict(d *pdfwriter.Dict, c stCall) error {
	if c.name != "pair" {
		return stUnknown(c)
	}
	return writeValue(d.Insert(pdfwriter.Name(c.str(0))), c.arg(1))
}

func stDictCalls(d *pdfwriter.Dict, calls []stCall) error {
	for _, c := range calls {
		if err := stDict(d, c); err != nil {
			return err
		}
	}
	return nil
}

func stCatalog(w *pdfwriter.Catalog, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "pages":
			w.Pages(c.ref(0))
		case "page_layout":
			w.PageLayout(stEnum(stPageLayouts, c.str(0)))
		case "page_labels":
			t := w.PageLabels()
			err = stNumberTree(t, c.then)
			if c.finish {
				t.Finish()
			}
		case "page_mode":
			w.PageMode(stEnum(stPageModes, c.str(0)))
		case "viewer_preferences":
			v := w.ViewerPreferences()
			err = stViewerPreferences(v, c.then)
			if c.finish {
				v.Finish()
			}
		case "names":
			n := w.Names()
			err = stNames(n, c.then)
			if c.finish {
				n.Finish()
			}
		case "destinations":
			w.Destinations(c.ref(0))
		case "outlines":
			w.Outlines(c.ref(0))
		case "lang":
			w.Lang(pdfwriter.TextStr(c.str(0)))
		case "version":
			w.Version(uint8(c.int(0)), uint8(c.int(1)))
		case "metadata":
			w.Metadata(c.ref(0))
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func stNumberTree(w *pdfwriter.NumberTree[pdfwriter.Ref], calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "nums":
			nums := w.Nums()
			for _, e := range c.then {
				if e.name != "insert" {
					return stUnknown(e)
				}
				nums.Insert(int32(e.int(0)), e.ref(1))
			}
			if c.finish {
				nums.Finish()
			}
		case "kids":
			kids := w.Kids()
			for _, e := range c.then {
				if e.name != "item" {
					return stUnknown(e)
				}
				kids.Item(e.ref(0))
			}
			if c.finish {
				kids.Finish()
			}
		case "limits":
			w.Limits(int32(c.int(0)), int32(c.int(1)))
		default:
			return stUnknown(c)
		}
	}
	return nil
}

func stNameTree(w *pdfwriter.NameTree[pdfwriter.Ref], calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "names":
			names := w.Names()
			for _, e := range c.then {
				if e.name != "insert" {
					return stUnknown(e)
				}
				names.Insert(pdfwriter.Str(e.hex(0)), e.ref(1))
			}
			if c.finish {
				names.Finish()
			}
		case "kids":
			kids := w.Kids()
			for _, e := range c.then {
				if e.name != "item" {
					return stUnknown(e)
				}
				kids.Item(e.ref(0))
			}
			if c.finish {
				kids.Finish()
			}
		case "limits":
			w.Limits(pdfwriter.Name(c.str(0)), pdfwriter.Name(c.str(1)))
		default:
			return stUnknown(c)
		}
	}
	return nil
}

func stViewerPreferences(w *pdfwriter.ViewerPreferences, calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "hide_toolbar":
			w.HideToolbar(c.bool(0))
		case "hide_menubar":
			w.HideMenubar(c.bool(0))
		case "fit_window":
			w.FitWindow(c.bool(0))
		case "center_window":
			w.CenterWindow(c.bool(0))
		case "display_doc_title":
			w.DisplayDocTitle(c.bool(0))
		case "non_full_screen_page_mode":
			w.NonFullScreenPageMode(stEnum(stPageModes, c.str(0)))
		case "direction":
			w.Direction(stEnum(stDirections, c.str(0)))
		default:
			if err := stDict(w.Dict, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func stNames(w *pdfwriter.Names, calls []stCall) error {
	trees := map[string]func() *pdfwriter.NameTree[pdfwriter.Ref]{
		"destinations":            w.Destinations,
		"appearances":             w.Appearances,
		"javascript":              w.JavaScript,
		"pages":                   w.Pages,
		"templates":               w.Templates,
		"capture_ids":             w.CaptureIDs,
		"capture_urls":            w.CaptureURLs,
		"embedded_files":          w.EmbeddedFiles,
		"alternate_presentations": w.AlternatePresentations,
		"renditions":              w.Renditions,
	}
	for _, c := range calls {
		f, ok := trees[c.name]
		if !ok {
			if err := stDict(w.Dict, c); err != nil {
				return err
			}
			continue
		}
		t := f()
		if err := stNameTree(t, c.then); err != nil {
			return err
		}
		if c.finish {
			t.Finish()
		}
	}
	return nil
}

func stDocumentInfo(w *pdfwriter.DocumentInfo, calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "title":
			w.Title(pdfwriter.TextStr(c.str(0)))
		case "author":
			w.Author(pdfwriter.TextStr(c.str(0)))
		case "subject":
			w.Subject(pdfwriter.TextStr(c.str(0)))
		case "keywords":
			w.Keywords(pdfwriter.TextStr(c.str(0)))
		case "creator":
			w.Creator(pdfwriter.TextStr(c.str(0)))
		case "producer":
			w.Producer(pdfwriter.TextStr(c.str(0)))
		case "creation_date":
			w.CreationDate(c.date(0))
		case "modified_date":
			w.ModifiedDate(c.date(0))
		case "trapped":
			w.Trapped(stEnum(stTrappingStatuses, c.str(0)))
		default:
			if err := stDict(w.Dict, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func stPages(w *pdfwriter.Pages, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "parent":
			w.Parent(c.ref(0))
		case "kids":
			w.Kids(c.refs(0)...)
		case "count":
			w.Count(int32(c.int(0)))
		case "media_box":
			w.MediaBox(c.rect(0))
		case "resources":
			r := w.Resources()
			err = stDictCalls(r.Dict, c.then)
			if c.finish {
				r.Finish()
			}
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func stPage(w *pdfwriter.Page, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "parent":
			w.Parent(c.ref(0))
		case "last_modified":
			w.LastModified(c.date(0))
		case "media_box":
			w.MediaBox(c.rect(0))
		case "crop_box":
			w.CropBox(c.rect(0))
		case "bleed_box":
			w.BleedBox(c.rect(0))
		case "trim_box":
			w.TrimBox(c.rect(0))
		case "art_box":
			w.ArtBox(c.rect(0))
		case "resources":
			r := w.Resources()
			err = stDictCalls(r.Dict, c.then)
			if c.finish {
				r.Finish()
			}
		case "contents":
			w.Contents(c.ref(0))
		case "contents_array":
			w.ContentsArray(c.refs(0)...)
		case "rotate":
			w.Rotate(int32(c.int(0)))
		case "thumbnail":
			w.Thumbnail(c.ref(0))
		case "duration":
			w.Duration(c.float(0))
		case "annotations":
			w.Annotations(c.refs(0)...)
		case "struct_parents":
			w.StructParents(int32(c.int(0)))
		case "tab_order":
			w.TabOrder(stEnum(stTabOrders, c.str(0)))
		case "user_unit":
			w.UserUnit(c.float(0))
		case "metadata":
			w.Metadata(c.ref(0))
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func stOutline(w *pdfwriter.Outline, calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "first":
			w.First(c.ref(0))
		case "last":
			w.Last(c.ref(0))
		case "count":
			w.Count(int32(c.int(0)))
		default:
			if err := stDict(w.Dict, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func stOutlineItem(w *pdfwriter.OutlineItem, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "title":
			w.Title(pdfwriter.TextStr(c.str(0)))
		case "parent":
			w.Parent(c.ref(0))
		case "prev":
			w.Prev(c.ref(0))
		case "next":
			w.Next(c.ref(0))
		case "first":
			w.First(c.ref(0))
		case "last":
			w.Last(c.ref(0))
		case "count":
			w.Count(int32(c.int(0)))
		case "dest":
			d := w.Dest()
			err = stDestination(d, c.then)
			if c.finish {
				d.Finish()
			}
		case "dest_name":
			w.DestName(pdfwriter.Name(c.str(0)))
		case "color_rgb":
			w.ColorRGB(c.float(0), c.float(1), c.float(2))
		case "flags":
			w.Flags(pdfwriter.OutlineItemFlags(c.int(0)))
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// stDestination is destination of writer_structure.rs. A call after one
// that ends the destination is an error.
func stDestination(d *pdfwriter.Destination, calls []stCall) error {
	done := false
	for _, c := range calls {
		if done {
			return errors.New("the destination is finished")
		}
		done = true
		switch c.name {
		case "page":
			d.Page(c.ref(0))
			done = false
		case "xyz":
			var zoom *float32
			if len(c.args) > 2 && c.args[2] != nil {
				z := c.float(2)
				zoom = &z
			}
			d.XYZ(c.float(0), c.float(1), zoom)
		case "fit":
			d.Fit()
		case "fit_horizontal":
			d.FitHorizontal(c.float(0))
		case "fit_vertical":
			d.FitVertical(c.float(0))
		case "fit_rect":
			d.FitRect(c.rect(0))
		case "fit_bounding_box":
			d.FitBoundingBox()
		case "fit_bounding_box_horizontal":
			d.FitBoundingBoxHorizontal(c.float(0))
		case "fit_bounding_box_vertical":
			d.FitBoundingBoxVertical(c.float(0))
		case "item":
			if err := writeValue(d.Push(), c.arg(0)); err != nil {
				return err
			}
			done = false
		default:
			return stUnknown(c)
		}
	}
	return nil
}

func stDestinations(w *pdfwriter.WriterDict[pdfwriter.Destination, *pdfwriter.Destination], calls []stCall) error {
	for _, c := range calls {
		if c.name != "insert" {
			return stUnknown(c)
		}
		d := w.Insert(pdfwriter.Name(c.str(0)))
		if err := stDestination(d, c.then); err != nil {
			return err
		}
		if c.finish {
			d.Finish()
		}
	}
	return nil
}

func stPageLabel(w *pdfwriter.PageLabel, calls []stCall) error {
	for _, c := range calls {
		switch c.name {
		case "style":
			w.Style(stEnum(stNumberingStyles, c.str(0)))
		case "prefix":
			w.Prefix(pdfwriter.TextStr(c.str(0)))
		case "offset":
			w.Offset(int32(c.int(0)))
		default:
			if err := stDict(w.Dict, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func stAnnotation(w *pdfwriter.Annotation, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "subtype":
			w.Subtype(stEnum(stAnnotationTypes, c.str(0)))
		case "rect":
			w.Rect(c.rect(0))
		case "contents":
			w.Contents(pdfwriter.TextStr(c.str(0)))
		case "page":
			w.Page(c.ref(0))
		case "name":
			w.Name(pdfwriter.TextStr(c.str(0)))
		case "modified":
			w.Modified(c.date(0))
		case "flags":
			w.Flags(pdfwriter.AnnotationFlags(c.int(0)))
		case "appearance_state":
			w.AppearanceState(pdfwriter.Name(c.str(0)))
		case "border":
			var dash []float32
			if len(c.args) > 3 && c.args[3] != nil {
				dash = c.floats(3)
			}
			w.Border(c.float(0), c.float(1), c.float(2), dash)
		case "color_transparent":
			w.ColorTransparent()
		case "color_gray":
			w.ColorGray(c.float(0))
		case "color_rgb":
			w.ColorRGB(c.float(0), c.float(1), c.float(2))
		case "color_cmyk":
			w.ColorCMYK(c.float(0), c.float(1), c.float(2), c.float(3))
		case "struct_parent":
			w.StructParent(int32(c.int(0)))
		case "action":
			a := w.Action()
			err = stAction(a, c.then)
			if c.finish {
				a.Finish()
			}
		case "highlight":
			w.Highlight(stEnum(stHighlightEffects, c.str(0)))
		case "author":
			w.Author(pdfwriter.TextStr(c.str(0)))
		case "subject":
			w.Subject(pdfwriter.TextStr(c.str(0)))
		case "quad_points":
			w.QuadPoints(c.floats(0)...)
		case "line_to":
			w.LineTo(c.float(0), c.float(1), c.float(2), c.float(3))
		case "parent":
			w.Parent(c.ref(0))
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func stAction(w *pdfwriter.Action, calls []stCall) error {
	for _, c := range calls {
		var err error
		switch c.name {
		case "action_type":
			w.ActionType(stEnum(stActionTypes, c.str(0)))
		case "destination":
			d := w.Destination()
			err = stDestination(d, c.then)
			if c.finish {
				d.Finish()
			}
		case "destination_named":
			w.DestinationNamed(pdfwriter.Name(c.str(0)))
		case "new_window":
			w.NewWindow(c.bool(0))
		case "uri":
			w.URI(pdfwriter.Str(c.hex(0)))
		case "is_map":
			w.IsMap(c.bool(0))
		case "js_string":
			w.JSString(pdfwriter.TextStr(c.str(0)))
		case "js_stream":
			w.JSStream(c.ref(0))
		case "annotation":
			w.Annotation(c.ref(0))
		default:
			err = stDict(w.Dict, c)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// The arguments of a call. A wrong argument panics, and goStructure gives
// the panic as an error.

func (c stCall) arg(i int) any {
	if i >= len(c.args) {
		panic(fmt.Sprintf("the call %q has no argument %d", c.name, i))
	}
	return c.args[i]
}

func (c stCall) str(i int) string { return as[string](c.arg(i)) }

func (c stCall) bool(i int) bool { return as[bool](c.arg(i)) }

func (c stCall) int(i int) int64 { return num(c.arg(i)) }

func (c stCall) hex(i int) []byte {
	b, err := hex.DecodeString(c.str(i))
	if err != nil {
		panic(err)
	}
	return b
}

// float reads a number as a float64 and rounds it to a float32, as serde
// and the oracle do.
func (c stCall) float(i int) float32 { return stFloat(c.arg(i)) }

func stFloat(v any) float32 {
	f, err := strconv.ParseFloat(string(as[json.Number](v)), 64)
	if err != nil {
		panic(err)
	}
	return float32(f)
}

func (c stCall) floats(i int) []float32 {
	fs := []float32{}
	for _, v := range as[[]any](c.arg(i)) {
		fs = append(fs, stFloat(v))
	}
	return fs
}

func (c stCall) ref(i int) pdfwriter.Ref {
	r, err := reference(c.arg(i))
	if err != nil {
		panic(err)
	}
	return r
}

func (c stCall) refs(i int) []pdfwriter.Ref {
	var rs []pdfwriter.Ref
	for _, v := range as[[]any](c.arg(i)) {
		r, err := reference(v)
		if err != nil {
			panic(err)
		}
		rs = append(rs, r)
	}
	return rs
}

func (c stCall) rect(i int) pdfwriter.Rect {
	f := c.floats(i)
	if len(f) != 4 {
		panic(fmt.Sprintf("the argument %d of the call %q is not a rectangle", i, c.name))
	}
	return pdfwriter.NewRect(f[0], f[1], f[2], f[3])
}

// date is date of writer_structure.rs. The casts wrap as the casts of Rust
// do.
func (c stCall) date(i int) pdfwriter.Date {
	var n []int64
	for _, v := range as[[]any](c.arg(i)) {
		n = append(n, num(v))
	}
	if len(n) == 0 {
		panic(fmt.Sprintf("the argument %d of the call %q is not a date", i, c.name))
	}
	d := pdfwriter.NewDate(uint16(n[0]))
	setters := []func(int64){
		func(v int64) { d = d.Month(uint8(v)) },
		func(v int64) { d = d.Day(uint8(v)) },
		func(v int64) { d = d.Hour(uint8(v)) },
		func(v int64) { d = d.Minute(uint8(v)) },
		func(v int64) { d = d.Second(uint8(v)) },
		func(v int64) { d = d.UTCOffsetHour(int8(v)) },
		func(v int64) { d = d.UTCOffsetMinute(uint8(v)) },
	}
	for j, v := range n[1:min(len(n), 8)] {
		setters[j](v)
	}
	return d
}

// The enums, by the Rust names of their variants.

var (
	stPageLayouts = []pdfwriter.PageLayout{
		pdfwriter.PageLayoutSinglePage, pdfwriter.PageLayoutOneColumn, pdfwriter.PageLayoutTwoColumnLeft,
		pdfwriter.PageLayoutTwoColumnRight, pdfwriter.PageLayoutTwoPageLeft, pdfwriter.PageLayoutTwoPageRight,
	}
	stPageModes = []pdfwriter.PageMode{
		pdfwriter.PageModeUseNone, pdfwriter.PageModeUseOutlines, pdfwriter.PageModeUseThumbs,
		pdfwriter.PageModeFullScreen, pdfwriter.PageModeUseOC, pdfwriter.PageModeUseAttachments,
	}
	stDirections      = []pdfwriter.Direction{pdfwriter.DirectionL2R, pdfwriter.DirectionR2L}
	stNumberingStyles = []pdfwriter.NumberingStyle{
		pdfwriter.NumberingStyleArabic, pdfwriter.NumberingStyleLowerRoman, pdfwriter.NumberingStyleUpperRoman,
		pdfwriter.NumberingStyleLowerAlpha, pdfwriter.NumberingStyleUpperAlpha,
	}
	stTrappingStatuses = []pdfwriter.TrappingStatus{
		pdfwriter.TrappingStatusTrapped, pdfwriter.TrappingStatusNotTrapped, pdfwriter.TrappingStatusUnknown,
	}
	stTabOrders = []pdfwriter.TabOrder{
		pdfwriter.TabOrderRowOrder, pdfwriter.TabOrderColumnOrder, pdfwriter.TabOrderStructureOrder,
	}
	stAnnotationTypes = []pdfwriter.AnnotationType{
		pdfwriter.AnnotationTypeText, pdfwriter.AnnotationTypeLink, pdfwriter.AnnotationTypeLine,
		pdfwriter.AnnotationTypeSquare, pdfwriter.AnnotationTypeCircle, pdfwriter.AnnotationTypeHighlight,
		pdfwriter.AnnotationTypeUnderline, pdfwriter.AnnotationTypeSquiggly, pdfwriter.AnnotationTypeStrikeOut,
		pdfwriter.AnnotationTypeFileAttachment, pdfwriter.AnnotationTypeWidget, pdfwriter.AnnotationTypeScreen,
	}
	stHighlightEffects = []pdfwriter.HighlightEffect{
		pdfwriter.HighlightEffectNone, pdfwriter.HighlightEffectInvert, pdfwriter.HighlightEffectOutline,
		pdfwriter.HighlightEffectPush,
	}
	stActionTypes = []pdfwriter.ActionType{
		pdfwriter.ActionTypeGoTo, pdfwriter.ActionTypeRemoteGoTo, pdfwriter.ActionTypeLaunch,
		pdfwriter.ActionTypeURI, pdfwriter.ActionTypeSubmitForm, pdfwriter.ActionTypeResetForm,
		pdfwriter.ActionTypeImportData, pdfwriter.ActionTypeJavaScript, pdfwriter.ActionTypeRendition,
	}
)

// stEnum finds the value of an enum by the Rust name of its variant, which
// its method String gives.
func stEnum[E fmt.Stringer](values []E, name string) E {
	for _, v := range values {
		if v.String() == name {
			return v
		}
	}
	panic(fmt.Sprintf("the variant %q is not known", name))
}

// stVariants gives the Rust names of the values of an enum.
func stVariants[E fmt.Stringer](values []E) []string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = v.String()
	}
	return s
}

// compareStructure asks the oracle each question, answers it with
// goStructure, and compares the bytes, as compareWriter does. It returns
// the number of answers of the oracle that are errors.
func compareStructure(t *testing.T, questions []structureQuestion) int {
	t.Helper()
	errs := 0
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
		got, gerr := goStructure(b)
		if a.Err != "" {
			errs++
		}
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
	return errs
}

// stKrillaDocument gives the objects that krilla writes for a document with
// two pages, page labels, an outline, links and named destinations, with the
// calls of krilla in their order.
func stKrillaDocument() []any {
	rect := []any{float32(0), float32(0), float32(595), float32(842)}
	return []any{
		structureObject{ID: 1, Start: "page_label", Structure: []structureCall{
			stc("style", "LowerRoman"), stc("prefix", "p-"), stc("offset", 3),
		}},
		structureObject{ID: 2, Start: "page_labels", Structure: []structureCall{
			stThen("nums", stc("insert", 0, 1), stc("insert", 1, 3)),
		}},
		structureObject{ID: 3, Start: "page_label", Structure: []structureCall{stc("style", "Arabic")}},
		structureObject{ID: 4, Start: "outline", Structure: []structureCall{
			stc("first", 5), stc("last", 6), stc("count", 2),
		}},
		structureObject{ID: 5, Start: "outline_item", Structure: []structureCall{
			stc("parent", 4), stc("next", 6), stc("title", "Intro é"), stc("pair", "Dest", pw.Ref(20)),
		}},
		structureObject{ID: 6, Start: "outline_item", Structure: []structureCall{
			stc("parent", 4), stc("prev", 5), stc("first", 7), stc("last", 7), stc("count", -1),
			stc("title", "Body"), stc("pair", "Dest", pw.Ref(21)),
		}},
		structureObject{ID: 7, Start: "outline_item", Structure: []structureCall{
			stc("parent", 6), stc("title", "Part"), stc("pair", "Dest", pw.Ref(21)),
		}},
		structureObject{ID: 8, Start: "annotation", Structure: []structureCall{
			stc("subtype", "Link"), stc("rect", []any{float32(10), float32(20.5), float32(110), float32(40)}),
			stc("border", 0, 0, 0, nil),
			stc("quad_points", []any{float32(10), float32(20.5), float32(110), float32(20.5), float32(110), float32(40), float32(10), float32(40)}),
			stThen("action", stc("action_type", "Uri"), stc("uri", hexString([]byte("https://example.com/a b")))),
			stc("flags", 4), stc("contents", "A link"),
		}},
		structureObject{ID: 9, Start: "annotation", Structure: []structureCall{
			stc("subtype", "Link"), stc("rect", rect), stc("border", 0, 0, float32(1.5), nil),
			stc("color_rgb", float32(1), float32(0), float32(0.25)),
			stc("pair", "Dest", pw.Str([]byte("chapter"))),
		}},
		structureObject{ID: 10, Start: "annotation", Structure: []structureCall{
			stc("subtype", "Link"), stc("rect", rect), stc("border", 0, 0, 0, []any{float32(3), float32(1)}),
			stc("color_cmyk", float32(0), float32(1), float32(0.5), float32(0)),
			{Call: "action", Then: []structureCall{stc("action_type", "GoTo"), stc("pair", "D", pw.Ref(20))}, Finish: true},
			stc("flags", 4),
		}},
		structureObject{ID: 11, Start: "annotation", Structure: []structureCall{
			stc("subtype", "Link"), stc("rect", rect), stc("color_gray", float32(0.5)),
			stThen("action", stc("action_type", "GoTo"), stThen("destination", stc("page", 13), stc("fit"))),
		}},
		structureObject{ID: 12, Start: "pages", Structure: []structureCall{stc("count", 2), stc("kids", []any{13, 14})}},
		structureObject{ID: 13, Start: "page", Structure: []structureCall{
			stThen("resources"), stc("media_box", rect), stc("crop_box", rect),
			stc("bleed_box", rect), stc("trim_box", rect), stc("art_box", rect),
			stc("tab_order", "StructureOrder"), stc("parent", 12), stc("contents", 15),
			stc("annotations", []any{8, 9, 10, 11}),
		}},
		structureObject{ID: 14, Start: "page", Structure: []structureCall{
			stc("pair", "Resources", pw.Ref(16)), stc("media_box", rect), stc("parent", 12), stc("contents", 17),
		}},
		writerObject{ID: 15, Data: base64.StdEncoding.EncodeToString([]byte("q Q"))},
		writerObject{ID: 16, Dict: [][2]any{pw.Pair("ProcSet", pw.Array(pw.Name("PDF"), pw.Name("Text")))}},
		writerObject{ID: 17, Data: base64.StdEncoding.EncodeToString([]byte("q Q"))},
		structureObject{ID: 20, Start: "destination", Structure: []structureCall{
			stc("page", 13), stc("xyz", float32(0), float32(842), nil),
		}},
		structureObject{ID: 21, Start: "destination", Structure: []structureCall{
			stc("page", 14), stc("xyz", float32(72.25), float32(700), float32(2)),
		}},
		structureObject{ID: 22, Start: "document_info", Structure: []structureCall{
			stc("title", "A (title)"), stc("subject", "Things"), stc("keywords", "a, b"),
			stc("author", "Somebody"), stc("creator", "okage"), stc("producer", "krilla"),
			stc("modified_date", []any{2026, 11, 18, 9, 30, 5, -5, 30}),
			stc("creation_date", []any{2026, 11, 18, 9, 30, 5, 0, 0}),
			stc("pair", "Custom", pw.Text("value")),
		}},
		structureObject{ID: 23, Start: "metadata", Data: base64.StdEncoding.EncodeToString([]byte("<x:xmpmeta/>"))},
		structureObject{ID: 24, Start: "catalog", Structure: []structureCall{
			stc("pages", 12), stc("metadata", 23), stc("pair", "PageLabels", pw.Ref(2)), stc("lang", "en-US"),
			stThen("viewer_preferences", stc("display_doc_title", true), stc("direction", "R2L")),
			stc("page_layout", "TwoPageRight"), stc("outlines", 4),
			stThen("names", stThen("destinations", stThen("names",
				stc("insert", hexString([]byte("chapter")), 21),
				stc("insert", hexString([]byte("intro")), 20),
			))),
		}},
	}
}

// TestPDFWriterStructure compares the Go writer with pdf-writer for the
// calls of structure.rs, annotations.rs and actions.rs that krilla makes,
// and for each other ported method once.
func TestPDFWriterStructure(t *testing.T) {
	no := false
	doc := stKrillaDocument()
	others := []any{
		structureObject{ID: 1, Start: "catalog", Structure: []structureCall{
			stc("pages", 2), stc("page_mode", "UseOutlines"), stc("destinations", 9), stc("version", 1, 7),
			stThen("page_labels", stThen("kids", stc("item", 30), stc("item", 31)), stc("limits", 0, 9)),
			{Call: "viewer_preferences", Finish: true, Then: []structureCall{
				stc("hide_toolbar", true), stc("hide_menubar", false), stc("fit_window", true),
				stc("center_window", false), stc("non_full_screen_page_mode", "UseThumbs"),
				stc("pair", "PrintScaling", pw.Name("None")),
			}},
			stThen("names",
				stThen("appearances", stThen("kids", stc("item", 3)), stc("limits", "a", "z")),
				stThen("javascript"), stThen("pages"), stThen("templates"), stThen("capture_ids"),
				stThen("capture_urls"), stThen("embedded_files"), stThen("alternate_presentations"),
				structureCall{Call: "renditions", Finish: true, Then: []structureCall{
					{Call: "names", Finish: true, Then: []structureCall{stc("insert", "00ff", 5)}},
				}},
			),
		}},
		structureObject{ID: 2, Start: "pages", Structure: []structureCall{
			stc("parent", 3), stc("kids", []any{}), stc("count", 0), stc("media_box", []any{float32(-1), float32(0.5), float32(1e6), float32(3)}),
			stThen("resources", stc("pair", "Font", pw.Dict(pw.Pair("F1", pw.Ref(4))))),
		}},
		structureObject{ID: 3, Start: "page", Structure: []structureCall{
			stc("last_modified", []any{1999}), stc("contents_array", []any{4, 5}), stc("rotate", 90),
			stc("thumbnail", 6), stc("duration", float32(2.5)), stc("struct_parents", 7),
			stc("tab_order", "RowOrder"), stc("user_unit", float32(1.25)), stc("metadata", 8),
			{Call: "resources", Finish: true}, stc("tab_order", "ColumnOrder"),
		}},
		structureObject{ID: 4, Start: "outline_item", Structure: []structureCall{
			stThen("dest", stc("page", 3), stc("fit_horizontal", float32(10))),
			stc("dest_name", "Named"), stc("color_rgb", float32(0.1), float32(0.2), float32(0.3)),
			stc("flags", 3), stc("count", 5),
		}},
		structureObject{ID: 5, Start: "destinations", Structure: []structureCall{
			stThen("insert", stc("page", 3), stc("fit_vertical", float32(1))),
			stThen("insert", stc("page", 3), stc("fit_rect", []any{float32(1), float32(2), float32(3), float32(4)})),
			{Call: "insert", Finish: true, Then: []structureCall{stc("page", 3), stc("fit_bounding_box")}},
			stThen("insert", stc("page", 3), stc("fit_bounding_box_horizontal", float32(5))),
			stThen("insert", stc("item", pw.Ref(3)), stc("fit_bounding_box_vertical", float32(6))),
			stThen("insert"),
		}},
		structureObject{ID: 6, Start: "annotation", Structure: []structureCall{
			stc("subtype", "Text"), stc("page", 3), stc("name", "n"), stc("modified", []any{2020, 1}),
			stc("appearance_state", "On"), stc("color_transparent"), stc("struct_parent", 2),
			stc("highlight", "Push"), stc("author", "me"), stc("subject", "it"),
			stc("line_to", float32(1), float32(2), float32(3), float32(4)), stc("parent", 7),
			stc("border", float32(1), float32(2), float32(3), []any{}), stc("flags", 1023),
		}},
		structureObject{ID: 7, Start: "action", Structure: []structureCall{
			stc("action_type", "JavaScript"), stc("destination_named", "D"), stc("new_window", true),
			stc("is_map", false), stc("js_string", "app.alert(1)"), stc("js_stream", 8), stc("annotation", 6),
		}},
		structureObject{ID: 8, Start: "names", Structure: []structureCall{stThen("destinations", stc("limits", "a", "b"))}},
		structureObject{ID: 9, Start: "viewer_preferences", Structure: []structureCall{stc("direction", "L2R")}},
		structureObject{ID: 10, Start: "metadata", Data: base64.StdEncoding.EncodeToString([]byte("x")),
			Structure: []structureCall{stc("filter", "FlateDecode"), stc("pair", "N", pw.Int(1))}},
		structureObject{ID: 11, Start: "document_info", Structure: []structureCall{stc("trapped", "Unknown")}},
	}
	var chunkObjects []any
	for _, o := range others {
		if s, ok := o.(structureObject); ok && s.Start != "catalog" && s.Start != "document_info" {
			chunkObjects = append(chunkObjects, o)
		}
	}
	// The example of the package comment of pdf-writer, as calls.
	example := []any{
		structureObject{ID: 1, Start: "catalog", Structure: []structureCall{stc("pages", 2)}},
		structureObject{ID: 2, Start: "pages", Structure: []structureCall{stc("kids", []any{3}), stc("count", 1)}},
		structureObject{ID: 3, Start: "page", Structure: []structureCall{
			stc("parent", 2), stc("media_box", []any{float32(0), float32(0), float32(595), float32(842)}), stThen("resources"),
		}},
	}
	questions := []structureQuestion{
		{PdfWriter: structureSpec{Objects: example}},
		{PdfWriter: structureSpec{Objects: doc}},
		{PdfWriter: structureSpec{Pretty: &no, Objects: doc}},
		{PdfWriter: structureSpec{Objects: others}},
		{PdfWriter: structureSpec{Pretty: &no, Objects: others}},
		{PdfWriter: structureSpec{Chunk: true, Objects: chunkObjects}},
		{PdfWriter: structureSpec{Chunk: true, Pretty: &no, Objects: chunkObjects}},
		// The fixed keys of the typed writers, with pairs from "dict".
		{PdfWriter: structureSpec{Objects: []any{
			writerObject{ID: 1, Start: "catalog", Dict: [][2]any{pw.Pair("Pages", pw.Ref(2))}},
			writerObject{ID: 2, Start: "pages"}, writerObject{ID: 3, Start: "page"},
			writerObject{ID: 4, Start: "outline"}, writerObject{ID: 5, Start: "outline_item"},
			writerObject{ID: 6, Start: "annotation", Dict: [][2]any{pw.Pair("Subtype", pw.Name("Link"))}},
			writerObject{ID: 7, Start: "metadata", Data: base64.StdEncoding.EncodeToString([]byte("m"))},
			writerObject{ID: 8, Start: "document_info"},
		}}},
		// The asserts of upstream panic with the same message.
		{PdfWriter: structureSpec{Objects: []any{
			structureObject{ID: 1, Start: "outline", Structure: []structureCall{stc("count", -1)}},
		}}},
		{PdfWriter: structureSpec{Objects: []any{
			structureObject{ID: 1, Start: "catalog", Structure: []structureCall{
				stThen("viewer_preferences", stc("non_full_screen_page_mode", "FullScreen")),
			}},
		}}},
		// Errors: an unknown call, a call after the end of a destination, a
		// catalog in a chunk, and a start without calls.
		{PdfWriter: structureSpec{Objects: []any{
			structureObject{ID: 1, Start: "page", Structure: []structureCall{stc("nothing")}},
		}}},
		{PdfWriter: structureSpec{Objects: []any{
			structureObject{ID: 1, Start: "destination", Structure: []structureCall{stc("fit"), stc("fit")}},
		}}},
		{PdfWriter: structureSpec{Chunk: true, Objects: []any{
			structureObject{ID: 1, Start: "catalog", Structure: []structureCall{}},
		}}},
		{PdfWriter: structureSpec{Objects: []any{
			structureObject{ID: 1, Start: "form_xobject", Structure: []structureCall{}},
		}}},
	}
	compareStructure(t, questions)

	// qpdf reads the document that krilla writes.
	b, err := json.Marshal(questions[1])
	if err != nil {
		t.Fatal(err)
	}
	data, err := goStructure(b)
	if err != nil {
		t.Fatal(err)
	}
	qpdfCheck(t, writeTemp(t, "structure.pdf", data))
}

// TestPDFWriterStructureRandom compares the Go writer with pdf-writer for
// random lists of calls of the typed writers of structure.rs,
// annotations.rs and actions.rs.
func TestPDFWriterStructureRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 7))
	questions := make([]structureQuestion, 300)
	for i := range questions {
		questions[i] = randomStructureQuestion(r)
	}
	// Most questions must give bytes, so that the test compares bytes and
	// not only the panics of the asserts.
	if errs := compareStructure(t, questions); errs > len(questions)/5 {
		t.Errorf("%d of %d questions give an error", errs, len(questions))
	} else {
		t.Logf("%d of %d questions give an error", errs, len(questions))
	}
}

// randomStructureQuestion makes a random question with typed writers.
func randomStructureQuestion(r *rand.Rand) structureQuestion {
	q := structureSpec{Objects: []any{}}
	if r.IntN(2) == 0 {
		p := r.IntN(2) == 0
		q.Pretty = &p
	}
	q.Chunk = r.IntN(4) == 0
	starts := []string{
		"pages", "page", "outline", "outline_item", "destination", "destinations", "annotation",
		"action", "page_label", "page_labels", "names", "viewer_preferences", "metadata",
	}
	if !q.Chunk {
		starts = append(starts, "catalog", "document_info")
	}
	ids := r.Perm(30)
	for i := range r.IntN(6) {
		o := structureObject{ID: 1 + ids[i], Start: starts[r.IntN(len(starts))]}
		if o.Start == "metadata" {
			o.Data = base64.StdEncoding.EncodeToString(randomBytes(r))
		}
		o.Structure = stRandomCalls(r, o.Start, 2)
		q.Objects = append(q.Objects, o)
	}
	return structureQuestion{PdfWriter: q}
}

// stRandomCalls gives up to six random calls on a typed writer of the kind
// that a start names, with nested writers down to the depth.
func stRandomCalls(r *rand.Rand, kind string, depth int) []structureCall {
	calls := []structureCall{}
	n := r.IntN(7)
	for range n {
		c, ok := stRandomCall(r, kind, depth)
		if !ok {
			continue
		}
		if len(c.Then) > 0 || r.IntN(3) == 0 {
			c.Finish = r.IntN(2) == 0
		}
		calls = append(calls, c)
		if kind == "destination" && stDestinationEnds[c.Call] {
			break
		}
	}
	return calls
}

// stDestinationEnds holds the calls that end a destination.
var stDestinationEnds = map[string]bool{
	"xyz": true, "fit": true, "fit_horizontal": true, "fit_vertical": true, "fit_rect": true,
	"fit_bounding_box": true, "fit_bounding_box_horizontal": true, "fit_bounding_box_vertical": true,
}

// stRandomCall gives one random call on a writer of the kind. It gives false
// when the depth does not allow a nested writer.
func stRandomCall(r *rand.Rand, kind string, depth int) (structureCall, bool) {
	ref := func() any { return 1 + r.IntN(30) }
	refList := func() any {
		var l []any
		for range r.IntN(4) {
			l = append(l, ref())
		}
		return append([]any{}, l...)
	}
	flt := func() any { return randomReal(r) }
	rect := func() any { return []any{flt(), flt(), flt(), flt()} }
	text := func() any { return randomText(r) }
	boolean := func() any { return r.IntN(2) == 0 }
	i32 := func() any { return int32(r.Uint32()) >> r.IntN(32) }
	date := func() any {
		d := []any{r.IntN(10001)}
		for _, hi := range []int{13, 32, 24, 60, 60} {
			if r.IntN(4) == 0 {
				return d
			}
			d = append(d, r.IntN(hi))
		}
		if r.IntN(2) == 0 {
			d = append(d, r.IntN(47)-23)
			if r.IntN(2) == 0 {
				d = append(d, r.IntN(60))
			}
		}
		return d
	}
	pick := func(values []string) any { return values[r.IntN(len(values))] }
	pair := func() structureCall { return stc("pair", randomText(r), randomValue(r, 1)) }
	nested := func(name, kind string) (structureCall, bool) {
		if depth == 0 {
			return structureCall{}, false
		}
		return stThen(name, stRandomCalls(r, kind, depth-1)...), true
	}
	switch kind {
	case "catalog":
		switch r.IntN(12) {
		case 0:
			return stc("pages", ref()), true
		case 1:
			return stc("page_layout", pick(stVariants(stPageLayouts))), true
		case 2:
			return nested("page_labels", "page_labels")
		case 3:
			return stc("page_mode", pick(stVariants(stPageModes))), true
		case 4:
			return nested("viewer_preferences", "viewer_preferences")
		case 5:
			return nested("names", "names")
		case 6:
			return stc("destinations", ref()), true
		case 7:
			return stc("outlines", ref()), true
		case 8:
			return stc("lang", text()), true
		case 9:
			return stc("version", r.IntN(20), r.IntN(20)), true
		case 10:
			return stc("metadata", ref()), true
		}
		return pair(), true
	case "page_labels":
		switch r.IntN(3) {
		case 0:
			var e []structureCall
			for range r.IntN(4) {
				e = append(e, stc("insert", i32(), ref()))
			}
			return stThen("nums", e...), true
		case 1:
			var e []structureCall
			for range r.IntN(4) {
				e = append(e, stc("item", ref()))
			}
			return stThen("kids", e...), true
		}
		return stc("limits", i32(), i32()), true
	case "name_tree":
		switch r.IntN(3) {
		case 0:
			var e []structureCall
			for range r.IntN(4) {
				e = append(e, stc("insert", hexString(randomBytes(r)), ref()))
			}
			return stThen("names", e...), true
		case 1:
			var e []structureCall
			for range r.IntN(4) {
				e = append(e, stc("item", ref()))
			}
			return stThen("kids", e...), true
		}
		return stc("limits", text(), text()), true
	case "viewer_preferences":
		calls := []string{"hide_toolbar", "hide_menubar", "fit_window", "center_window", "display_doc_title"}
		switch i := r.IntN(8); {
		case i < len(calls):
			return stc(calls[i], boolean()), true
		case i == 5:
			// FullScreen panics in both writers.
			return stc("non_full_screen_page_mode", pick(stVariants(stPageModes))), true
		case i == 6:
			return stc("direction", pick(stVariants(stDirections))), true
		}
		return pair(), true
	case "names":
		trees := []string{
			"destinations", "appearances", "javascript", "pages", "templates", "capture_ids",
			"capture_urls", "embedded_files", "alternate_presentations", "renditions",
		}
		if r.IntN(6) == 0 {
			return pair(), true
		}
		return nested(trees[r.IntN(len(trees))], "name_tree")
	case "document_info":
		texts := []string{"title", "author", "subject", "keywords", "creator", "producer"}
		switch i := r.IntN(10); {
		case i < len(texts):
			return stc(texts[i], text()), true
		case i == 6:
			return stc("creation_date", date()), true
		case i == 7:
			return stc("modified_date", date()), true
		case i == 8:
			return stc("trapped", pick(stVariants(stTrappingStatuses))), true
		}
		return pair(), true
	case "pages":
		switch r.IntN(6) {
		case 0:
			return stc("parent", ref()), true
		case 1:
			return stc("kids", refList()), true
		case 2:
			return stc("count", i32()), true
		case 3:
			return stc("media_box", rect()), true
		case 4:
			return nested("resources", "dict")
		}
		return pair(), true
	case "page":
		boxes := []string{"media_box", "crop_box", "bleed_box", "trim_box", "art_box"}
		switch i := r.IntN(19); {
		case i < len(boxes):
			return stc(boxes[i], rect()), true
		case i == 5:
			return stc("parent", ref()), true
		case i == 6:
			return stc("last_modified", date()), true
		case i == 7:
			return nested("resources", "dict")
		case i == 8:
			return stc("contents", ref()), true
		case i == 9:
			return stc("contents_array", refList()), true
		case i == 10:
			return stc("rotate", i32()), true
		case i == 11:
			return stc("thumbnail", ref()), true
		case i == 12:
			return stc("duration", flt()), true
		case i == 13:
			return stc("annotations", refList()), true
		case i == 14:
			return stc("struct_parents", i32()), true
		case i == 15:
			return stc("tab_order", pick(stVariants(stTabOrders))), true
		case i == 16:
			return stc("user_unit", flt()), true
		case i == 17:
			return stc("metadata", ref()), true
		}
		return pair(), true
	case "dict":
		return pair(), true
	case "outline":
		switch r.IntN(4) {
		case 0:
			return stc("first", ref()), true
		case 1:
			return stc("last", ref()), true
		case 2:
			// A negative count panics in both writers.
			return stc("count", r.IntN(40)-1), true
		}
		return pair(), true
	case "outline_item":
		refs := []string{"parent", "prev", "next", "first", "last"}
		switch i := r.IntN(12); {
		case i < len(refs):
			return stc(refs[i], ref()), true
		case i == 5:
			return stc("title", text()), true
		case i == 6:
			return stc("count", i32()), true
		case i == 7:
			return nested("dest", "destination")
		case i == 8:
			return stc("dest_name", text()), true
		case i == 9:
			return stc("color_rgb", flt(), flt(), flt()), true
		case i == 10:
			return stc("flags", r.Uint32()), true
		}
		return pair(), true
	case "destination":
		switch r.IntN(11) {
		case 0, 1:
			return stc("page", ref()), true
		case 2:
			if r.IntN(2) == 0 {
				return stc("xyz", flt(), flt(), nil), true
			}
			return stc("xyz", flt(), flt(), flt()), true
		case 3:
			return stc("fit"), true
		case 4:
			return stc("fit_horizontal", flt()), true
		case 5:
			return stc("fit_vertical", flt()), true
		case 6:
			return stc("fit_rect", rect()), true
		case 7:
			return stc("fit_bounding_box"), true
		case 8:
			return stc("fit_bounding_box_horizontal", flt()), true
		case 9:
			return stc("fit_bounding_box_vertical", flt()), true
		}
		return stc("item", randomValue(r, 1)), true
	case "destinations":
		c, ok := nested("insert", "destination")
		c.Args = []any{text()}
		return c, ok
	case "page_label":
		switch r.IntN(4) {
		case 0:
			return stc("style", pick(stVariants(stNumberingStyles))), true
		case 1:
			return stc("prefix", text()), true
		case 2:
			return stc("offset", i32()), true
		}
		return pair(), true
	case "annotation":
		switch r.IntN(23) {
		case 0:
			return stc("subtype", pick(stVariants(stAnnotationTypes))), true
		case 1:
			return stc("rect", rect()), true
		case 2:
			return stc("contents", text()), true
		case 3:
			return stc("page", ref()), true
		case 4:
			return stc("name", text()), true
		case 5:
			return stc("modified", date()), true
		case 6:
			return stc("flags", r.Uint32()), true
		case 7:
			return stc("appearance_state", text()), true
		case 8:
			var dash any
			if r.IntN(2) == 0 {
				var d []any
				for range r.IntN(4) {
					d = append(d, flt())
				}
				dash = append([]any{}, d...)
			}
			return stc("border", flt(), flt(), flt(), dash), true
		case 9:
			return stc("color_transparent"), true
		case 10:
			return stc("color_gray", flt()), true
		case 11:
			return stc("color_rgb", flt(), flt(), flt()), true
		case 12:
			return stc("color_cmyk", flt(), flt(), flt(), flt()), true
		case 13:
			return stc("struct_parent", i32()), true
		case 14:
			return nested("action", "action")
		case 15:
			return stc("highlight", pick(stVariants(stHighlightEffects))), true
		case 16:
			return stc("author", text()), true
		case 17:
			return stc("subject", text()), true
		case 18:
			var q []any
			for range 4 * r.IntN(3) {
				q = append(q, flt())
			}
			return stc("quad_points", append([]any{}, q...)), true
		case 19:
			return stc("line_to", flt(), flt(), flt(), flt()), true
		case 20:
			return stc("parent", ref()), true
		}
		return pair(), true
	case "action":
		switch r.IntN(11) {
		case 0:
			return stc("action_type", pick(stVariants(stActionTypes))), true
		case 1:
			return nested("destination", "destination")
		case 2:
			return stc("destination_named", text()), true
		case 3:
			return stc("new_window", boolean()), true
		case 4:
			return stc("uri", hexString(randomBytes(r))), true
		case 5:
			return stc("is_map", boolean()), true
		case 6:
			return stc("js_string", text()), true
		case 7:
			return stc("js_stream", ref()), true
		case 8:
			return stc("annotation", ref()), true
		}
		return pair(), true
	case "metadata":
		if r.IntN(3) == 0 {
			return stc("filter", pick([]string{"FlateDecode", "DCTDecode", "Crypt"})), true
		}
		return pair(), true
	}
	panic("the kind " + kind + " is not known")
}

// TestStructureNumbers makes sure that a float32 of a question goes through
// JSON unchanged, so that both writers see the same numbers.
func TestStructureNumbers(t *testing.T) {
	r := rand.New(rand.NewPCG(62, 8))
	for range 1000 {
		f := randomReal(r)
		b, err := json.Marshal([]any{f})
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var v []any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		if got := stFloat(v[0]); math.Float32bits(got) != math.Float32bits(f) {
			t.Errorf("%v goes through JSON as %v", f, got)
		}
	}
}
