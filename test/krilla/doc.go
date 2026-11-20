// Package krilla compares the PDF writer of okage with krilla, its upstream
// (D22, D62). The package has no code of its own. Its tests hold the PDF
// oracle in the folder rust, a reader that decodes the streams of a PDF file
// and compares two files by their objects, and the helpers that render a
// page with MuPDF `mutool draw` and Poppler `pdftoppm` and run `qpdf
// --check` on a file.
//
// # The oracle
//
// The oracle is a Rust program that links krilla and krilla-svg at the
// matched commit of the row krilla of docs/upstream/upstreams.tsv, or at its
// base commit when the column matched is empty, pdf-writer at the
// commit that krilla pins, and subsetter at the version that krilla
// requires. TestMain builds it with cargo into .cache/krilla in the root of
// the repository, with the scalar code of tiny-skia, as test/oracle builds
// its oracle. The tests of the oracle skip when cargo cannot build it. A
// helper of a program skips its test when the program is missing.
//
// The oracle reads one question as JSON on each line of its input. It writes
// one answer for each question: four bytes with the length of the answer,
// little-endian, then the answer. The first byte of an answer is 0 for a
// result and 1 for an error, and the rest of an error is its message. A
// panic of a crate is an error. The oracle then answers the next question.
//
// The Go tests write each number of a float32 as the shortest decimal that
// gives back the float32, as encoding/json does. A field that the text below
// calls optional takes the default of the crate when it is missing.
//
// # The question "subset"
//
// The question gives the subset of a font, as krilla makes it for a CID font:
//
//	{"subset": {"font": "/abs/Font.ttf", "index": 0, "glyphs": [36, 37],
//	            "coords": [["wght", 700]]}}
//
// "glyphs" holds the glyph IDs in the order of their first use. The oracle
// remaps them in this order after the glyph 0, as the GlyphRemapper of
// subsetter does. When "coords" is there, even empty, the oracle calls
// subset_with_variations, as krilla does. Without it, the oracle calls
// subset. The answer is the tag of six capital letters that krilla puts in
// front of the name of the font, which is a hash of the remapper by
// siphasher, then the bytes of the subset.
//
// The tests of subsetter_test.go make the same subsets with
// internal/subsetter, and compare the tag and the bytes with the answer. They
// read the TrueType fonts of upstream/subsetter/fonts and of the tests of
// okage, and fonts with bytes changed at random. A case that needs a unit of
// phase 13 that is not ported yet is skipped.
//
// The tests of subsetter_vf_test.go make the instances of the variable fonts
// of subsetter and of the tests of okage at many locations. A subset at the
// default location must be the same, byte for byte. In an instance, each
// coordinate, advance and left side bearing must be within 1 unit of the
// instance of subsetter, because typesetting computes the deltas in float32
// and skrifa in fixed point. The tests count the values that differ, and log
// the counts for each font.
//
// # The question "pdf_writer"
//
// The question gives the bytes of a list of calls of pdf-writer:
//
//	{"pdf_writer": {"pretty": true, "chunk": false, "version": [1, 7],
//	                "binary_marker": [37, 37, 37, 37], "file_id": ["0a", "0b"],
//	                "objects": [...], "xref_stream": 9}}
//
// Each field except "objects" is optional. "pretty" is the setting of
// pdf-writer, and its default is true. When "chunk" is true, the oracle
// writes the objects into a Chunk and answers its bytes. Otherwise it writes
// them into a Pdf with the version, the binary marker and the file
// identifier, and answers the bytes that Pdf::finish gives. With
// "xref_stream", the file ends with a cross-reference stream of that object
// number, from Pdf::finish_with_xref_stream. The strings of "file_id" are in
// hex.
//
// Two more fields are optional:
//
//	{"pdf_writer": {..., "xref_stream": 9, "xref_filter": ["ASCIIHexDecode", "FlateDecode"],
//	                "renumber": [[1, 7], [2, 8]]}}
//
// "xref_filter" needs "xref_stream". The oracle then calls
// Pdf::finish_with_xref_stream_and_filter. A name gives XRefFilter::Single,
// a list of names gives XRefFilter::Multiple, and the data of the stream
// stays as it is. With "renumber", the oracle writes the objects into a
// Chunk and renumbers it with Chunk::renumber. Each pair is an old number
// and a new number, and a number that is not in a pair stays as it is. The
// answer is the renumbered chunk when "chunk" is true. Otherwise the oracle
// adds the renumbered chunk to a Pdf with Pdf::extend and answers the file.
// The typed writers catalog and document_info cannot go into a chunk.
//
// Each object of "objects" is one of these forms:
//
//	{"id": 1, "value": V}
//	{"id": 2, "dict": [["Key", V], ...]}
//	{"id": 3, "start": "pages", "dict": [["Count", {"int": 1}], ...]}
//	{"id": 4, "data": "<base64>", "filter": "FlateDecode", "dict": [...]}
//	{"id": 5, "start": "form_xobject", "content": [{"op": "re", "operands": [V, ...]}, ...]}
//
// The first form writes a value with Chunk::indirect, and the second a
// dictionary. The third starts a typed writer of pdf-writer, which writes its
// fixed keys, such as /Type /Pages, and then the pairs of "dict". The typed
// writers are catalog and document_info, which also go into the trailer,
// pages, page, outline, outline_item, ext_graphics, type1_font, type3_font,
// type0_font, cid_font, font_descriptor, function_shading, shading_pattern,
// exponential_function, stitching_function and annotation. An object with
// "data" or "content" is a stream. "data" holds its bytes in base64, and
// "content" is a content stream that the oracle writes with Content::op and
// its operands. "filter" is written as it is, and the data is not encoded.
// The typed writers of a stream are stream, form_xobject, image_xobject,
// cmap, icc_profile, tiling_pattern, stream_shading, post_script_function,
// sampled_function, metadata and embedded_file.
//
// The units of phase 12 add typed calls, each in a module of the oracle that
// documents its format:
//
//   - An operation of "content" whose "op" is the name of a method of
//     Content calls that method (writer_content.rs).
//   - An object whose "start" is "calls.<writer>" makes the typed calls of a
//     font, a font descriptor, a CMap or an XObject (writer_font.rs).
//   - An object whose "start" begins with "color:" runs a script of the
//     writers of color spaces, patterns, shadings and functions
//     (writer_color.rs).
//   - An object with "structure" makes the calls of the catalog, the pages,
//     the outline, the page labels, the destinations, the annotations and the
//     actions (writer_structure.rs).
//
// The test TestPDFWriterGo makes the same calls with the Go writer of
// internal/pdfwriter, and compares the bytes with the answer of the oracle.
// The Go side of each typed writer is in the maps writerDicts, writerStreams
// and pdfDicts of pdfwriter_test.go. A unit that ports a typed writer adds
// its entries to them from a file of its own, in a function init.
//
// A value V is the string "null", or an object with one key: {"bool": true},
// {"int": 3}, {"real": 1.5}, {"name": "Type"}, {"str": "<hex>"}, {"text":
// "UTF-8 text"}, {"ref": 4}, {"array": [V, ...]} or {"dict": [["Key", V],
// ...]}. A "str" is a byte string, and a "text" is a text string of
// pdf-writer.
//
// # The question "svg"
//
// The question gives the PDF file of an SVG file, as the test svg_impl of
// krilla writes it: one page with the size of the tree, and the tree drawn
// with draw_svg of krilla-svg:
//
//	{"svg": {"file": "/abs/file.svg", "width": 100, "height": 50,
//	         "embed_text": true, "filter_scale": 2,
//	         "fonts": {"dirs": [...], "files": [...], "serif": "Noto Serif",
//	                   "sans_serif": "Noto Sans", "cursive": "...",
//	                   "fantasy": "...", "monospace": "..."},
//	         "settings": S}}
//
// Each field except "file" is optional. "width" and "height" give the size
// of the page, and the default is the size of the tree. "embed_text" and
// "filter_scale" are the fields of SvgSettings. "fonts" fills the font
// database of usvg, which is empty by default. The folder of the file is the
// folder of its resources.
//
// S holds the fields of SerializeSettings: "pretty", "ascii_compatible",
// "compress_content_streams", "no_device_cs", "xmp_metadata" and
// "enable_tagging" are booleans, and "version" is "1.4", "1.5", "1.6", "1.7"
// or "2.0". Each field is optional. A missing field takes the value of
// SerializeSettings::default. The settings draw the glyphs of SVG fonts with
// render_svg_glyph of krilla-svg, as the tests of krilla do, unless
// "svg_glyphs" is false. The Go side draws them with pdfsvg.RenderGlyph.
//
// The test TestSVGFiles of svg_test.go asks this question for each SVG file
// of the tests of resvg in svg/testdata/tests, with the fonts of
// svg/testdata/fonts and the filter scale 2, as the test svg_impl of krilla
// does. goSVG draws the same file with svg and pdfsvg. When the objects of
// the two files are the same, the file passes. Otherwise MuPDF and Poppler
// render both files at 72 dots for each inch, and the metric of D31
// compares the pixels with the gate of item 4 of "Testing" in docs/PLAN.md.
// A file that differs because svg follows a later usvg than krilla-svg is
// in the map svgDifferences, with the reason. The test logs the number of
// files of each outcome. With -short, it compares one file in 20. goRun
// makes the call draw_svg of a scene with pdfsvg, in goDrawSVG.
//
// # The question "krilla"
//
// The question gives the PDF file of a scene: a document, its fonts and
// images, and the calls of the surface of each page:
//
//	{"krilla": {"settings": S,
//	            "fonts": [{"file": "/abs/Font.ttf", "index": 0, "coords": [["wght", 700]]}],
//	            "images": [{"file": "/abs/image.png", "kind": "png", "interpolate": false},
//	                       {"rgba": "<base64>", "width": 4, "height": 4}],
//	            "pages": [{"width": 200, "height": 200, "ops": [OP, ...]}]}}
//
// A font is a font file with Font::new_variable. An image is a PNG, JPEG, GIF
// or WebP file, with the kind from the extension unless "kind" gives it, or
// the RGBA bytes of Image::from_rgba8, not premultiplied. The module image
// of the oracle adds two forms of an image:
//
//	{"file": "/abs/image.png", "kind": "png_native"}
//	{"custom": {"color": "<base64>", "alpha": "<base64>", "bits": 8,
//	            "width": 4, "height": 4, "color_space": "rgb", "icc": "/abs/p.icc"}}
//
// The kind "png_native" is Image::from_png_native. "custom" is
// Image::from_custom, with the samples of the colors and of the alpha, 8 or
// 16 bits each in big-endian order. "color_space" is "rgb", "luma" or
// "cmyk". "alpha" and "icc" are optional. Each page is
// PageSettings::from_wh, and its calls make its content.
//
// A page has more fields, which are optional:
//
//	{"width": 200, "height": 200, "media_box": [x, y, w, h],
//	 "crop_box": [x, y, w, h], "bleed_box": [...], "trim_box": [...], "art_box": [...],
//	 "label": {"style": "r", "prefix": "A-", "offset": 2}, "surface": false}
//
// With "media_box", the page is PageSettings::new with that rectangle, and
// "width" and "height" are not read. The other boxes and the label are the
// methods with_crop_box, with_bleed_box, with_trim_box, with_art_box and
// with_page_label. The style of a label is the key /S of PDF: "D", "r",
// "R", "a" or "A". A missing field of a label is None. When "surface" is
// false, the oracle finishes the page without a call of Page::surface, so the
// page has the empty stream of upstream. A scene without pages gets the
// default page of Document::finish.
//
// Each call OP is an object with the field "op". These are the calls, with
// the fields of each one:
//
//	set_fill        "fill": F, or no field for no fill
//	set_stroke      "stroke": K, or no field for no stroke
//	draw_path       "path": P
//	push_transform  "transform": [sx, ky, kx, sy, tx, ty]
//	push_blend_mode "mode": "multiply" and the other modes of krilla in snake case
//	push_clip_path  "path": P, "rule": "nonzero" or "evenodd"
//	push_mask       "mask_type": "luminosity" or "alpha", "ops": [OP, ...],
//	                and the optional "bbox": [x, y, w, h]
//	push_opacity    "opacity": 0.5
//	push_isolated
//	pop
//	draw_image      "image": the index of the image, "width", "height"
//	draw_glyphs     "start": [x, y], "font": the index of the font, "size",
//	                "text", "outlined", "glyphs": [G, ...]
//	draw_text       "start": [x, y], "font", "size", "text", "outlined",
//	                "direction": "auto", "ltr", "rtl", "ttb" or "btt"
//	draw_svg        the fields of the question "svg" without "settings"
//
// A push and its pop must be in the same list of calls. A mask draws its
// calls into a stream of the surface, as StreamBuilder does, and a pattern
// does too. The surface draws a path with the fill and the stroke that the
// calls set.
//
// A path P is a list of segments: [0, x, y] moves, [1, x, y] draws a line,
// [2, x1, y1, x, y] a quadratic curve, [3, x1, y1, x2, y2, x, y] a cubic
// curve, and [4] closes. This is the form of the paths of test/oracle.
//
// A fill F is {"paint": PAINT, "opacity": 1, "rule": "nonzero"}. A stroke K
// is {"paint": PAINT, "width": 1, "miter_limit": 10, "line_cap": "butt",
// "line_join": "miter", "opacity": 1, "dash": {"array": [3, 1], "offset":
// 0}}. Each field is optional, and the defaults are the ones shown. The
// default paint is black, and a stroke has no dash by default. The cap is
// "butt", "round" or "square", and the join is "miter", "round" or "bevel".
//
// A paint PAINT is one of these objects:
//
//	{"rgb": [255, 0, 0]}, {"luma": 128}, {"cmyk": [0, 0, 0, 255]}
//	{"linear": {"x1", "y1", "x2", "y2", "transform", "spread", "stops", "anti_alias"}}
//	{"radial": {"fx", "fy", "fr", "cx", "cy", "cr", "transform", "spread", "stops", "anti_alias"}}
//	{"sweep": {"cx", "cy", "start_angle", "end_angle", "transform", "spread", "stops", "anti_alias"}}
//	{"pattern": {"ops": [OP, ...], "width", "height", "transform"}}
//
// A transform has the order of push_transform, and the identity is the
// default. "spread" is "pad", "reflect" or "repeat", and "pad" is the
// default. A stop is {"offset": 0.5, "color": C, "opacity": 1}, where C is
// one of the three colors above.
//
// A glyph G is {"id": 36, "range": [0, 1], "x_advance": 0.6, "x_offset": 0,
// "y_offset": 0, "y_advance": 0}. The range is the byte range of the cluster
// of the glyph in the text. The advances and the offsets are in units of the
// size of the font, as in KrillaGlyph. The fields other than "id" are
// optional, and their default is 0.
//
// A scene and a page have more fields for the metadata, the outline and
// the links. The module interchange.rs of the oracle reads them. Each field
// is optional:
//
//	{"krilla": {..., "metadata": M, "outline": [N, ...],
//	            "named_destinations": [{"name": "a", "page": 0, "x": 1, "y": 2}, ...],
//	            "pages": [{..., "annotations": [A, ...]}]}}
//
// The oracle registers the named destinations with
// Document::register_named_destination before the pages, and ignores the
// result. It adds the annotations of a page with Page::add_annotation before
// the surface of the page. After the pages, it sets the metadata with
// Document::set_metadata and the outline with Document::set_outline. An
// outline that is there, even empty, calls set_outline.
//
// The metadata M has the fields "title", "description", "creator",
// "producer", "document_id", "version_id" and "language", which are
// strings, "keywords" and "authors", which are lists of strings, and
// "display_doc_title" and "trapped", which are booleans. Each one calls the
// method of Metadata of the same name. "text_direction" is "ltr" or "rtl". "page_layout" is the
// name of a case of PageLayout, such as "TwoColumnRight". "creation_date" is
// {"year": 2024, "month": 11, "day": 8, "hour": 22, "minute": 23, "second":
// 18, "utc_offset_hour": 1, "utc_offset_minute": 12}, where each field
// except "year" is optional. "custom_fields" is a list of pairs [name,
// value] for Metadata::custom_field, in their order. The oracle leaves out
// a field that krilla does not accept.
//
// A node N of the outline is {"text": "Heading", "dest": {"page": 0, "x":
// 0, "y": 0}, "open": false, "children": [N, ...]}. The destination is an
// XyzDestination.
//
// An annotation A is a LinkAnnotation, with "rect": [x, y, w, h] for
// LinkAnnotation::new, or "quad_points": [[x1, y1, x2, y2, x3, y3, x4, y4],
// ...] for LinkAnnotation::new_with_quad_points. "target" is one of
// {"xyz": {"page", "x", "y"}}, {"named": {"name", "page", "x", "y"}}, which
// are destinations, {"link": "https://..."}, a LinkAction, and {"goto": D},
// the action Action::Goto, where D is {"xyz": ...} or {"named": ...}. The
// optional "border" is {"width": 1, "color": C}, with a color C of the
// paints, and the optional "alt" is the alt text of Annotation::new_link.
//
// The tests of document_test.go build the scenes of the pages without a
// surface with the package pdf, and compare the files with the answers by
// their objects. When no stream is compressed, the bytes must be the same,
// with the ID of the document. The tests of surface_test.go make the calls
// of the surface in the same way, with goRun, for scenes of every paint and
// for random scenes. The tests of image_test.go draw images with pdf, from
// the image files of the tests of pdf, of Go and of golang.org/x/image, from
// PNG files with faults in their image data, and from the images of Go with
// ImageFromImage, and compare them with the files of krilla in the same way.
// The tests of text_test.go draw the glyphs and the texts of the fonts of
// pdf/testdata/fonts and of the checkout of krilla, with and without the SVG
// glyphs:
// the snapshots of text and fonts of krilla, the COLR glyphs of Noto Color
// Emoji, the bitmap glyphs of the tables sbix and CBDT, and random runs of
// glyphs. The font files of their subsets must be the bytes of krilla,
// except for an instance of a variable font (D65). The tests of
// interchange_test.go do the same for the metadata, the outlines and the
// links, for fixed and for random scenes, and compare the errors where
// krilla panics. The test of fallback_test.go draws text with a font whose
// table post subsetter refuses, for the fallback of D80. krilla returns an
// error for it. pdf returns the error with the fallback off. With the
// fallback on, the reader reads the file of pdf, MuPDF and Poppler render
// it as the file of krilla with the font that does not fail, and Poppler
// `pdftotext` extracts its text.
//
// # The question "point_ts"
//
// The question gives the rotation of get_point_ts of krilla, which rotates a
// linear gradient with the spread method reflect or repeat:
//
//	{"point_ts": [[x1, y1, x2, y2], ...]}
//
// For each line, the oracle computes the angle of the line with atan2 and
// the transform of Transform::from_rotate_at, which calls sin and cos. The
// answer holds six float32 for each line, little-endian: sx, ky, kx, sy, tx
// and ty. Rust calls the C library for these functions, and glibc misses the
// correctly rounded result of sinf and cosf for about 1 percent of the
// angles, where pdf computes it with internal/crmath (D26). When the file of
// a scene differs from the file of krilla, the tests of surface_test.go ask
// this question. The difference is accepted only when a rotation of the
// scene differs.
package krilla
