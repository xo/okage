// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"

	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/options.rs.
//
// Port: the struct Options of upstream becomes the private options, with one
// exported functional option for each field that a user sets (D20). The
// logger of WithLogger takes the place of the crate log.
//
// The field fontdb holds a FontDatabase of usvg_text_fontdb.go, which takes
// the place of fontdb::Database (D9). The fonts come from the caller, with
// WithFontDatabase, WithFontFile or WithFontFS. The method fontdb_mut is
// fontdbMut, which the options that change the database call.
//
// The options also hold the options of the Renderer of renderer.go (D36).
// WithFontDatabase, WithFontFamily and the options of the generic families
// record that they were given, so that the renderer does not change what
// they set.

// ImageHrefDataResolverFn is ImageHrefDataResolverFn in usvg
// src/parser/image.rs. It is a shorthand for [ImageHrefResolver]'s data
// function.
//
// The function is called with the mime type and the decoded base64 data. It
// returns false where upstream returns None.
//
// Port: upstream also passes the parsing options. Go does not, because the
// options are private (D20).
type ImageHrefDataResolverFn func(mime string, data []byte) (ImageKind, bool)

// ImageHrefStringResolverFn is ImageHrefStringResolverFn in usvg
// src/parser/image.rs. It is a shorthand for [ImageHrefResolver]'s string
// function.
//
// The function is called with the value of `xlink:href`. It returns false
// where upstream returns None.
//
// Port: upstream also passes the parsing options. Go does not, because the
// options are private (D20).
type ImageHrefStringResolverFn func(href string) (ImageKind, bool)

// ImageHrefResolver is ImageHrefResolver in usvg src/parser/image.rs. It is
// an `xlink:href` resolver for `<image>` elements.
//
// This type can be useful if you want to have an alternative `xlink:href` handling
// to the default one. For example, you can forbid access to local files (which is allowed by default)
// or add support for resolving actual URLs (usvg doesn't do any network requests).
//
// Port: a nil function is the default resolver of upstream,
// DefaultDataResolver or DefaultStringResolver in usvg_parser_image.go, with
// the options of the parser. The zero value is the impl Default of upstream.
type ImageHrefResolver struct {
	// Resolver function that will be used when `xlink:href` contains a
	// [Data URL](https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/Data_URIs).
	//
	// A function would be called with mime and decoded base64 data.
	ResolveData ImageHrefDataResolverFn

	// Resolver function that will be used to handle an arbitrary string in `xlink:href`.
	ResolveString ImageHrefStringResolverFn
}

// DefaultMaxDepth is DEFAULT_MAX_DEPTH in usvg src/parser/options.rs. It
// is the default nesting limit when parsing.
const DefaultMaxDepth uint32 = 1024

// Option is an option of the functions that parse an SVG file. It sets a
// field of the struct Options in usvg src/parser/options.rs (D20).
type Option func(*options)

// options is Options in usvg src/parser/options.rs. It holds the processing
// options.
type options struct {
	// Directory that will be used during relative paths resolving.
	//
	// Expected to be the same as the directory that contains the SVG file,
	// but can be set to any.
	//
	// Default: `None`. Port: the empty string is None.
	resourcesDir string

	// Target DPI.
	//
	// Impacts units conversion.
	//
	// Default: 96.0
	dpi float32

	// A default font family.
	//
	// Will be used when no `font-family` attribute is set in the SVG.
	//
	// Default: Times New Roman
	fontFamily string

	// A default font size.
	//
	// Will be used when no `font-size` attribute is set in the SVG.
	//
	// Default: 12
	fontSize float32

	// A list of languages.
	//
	// Will be used to resolve a `systemLanguage` conditional attribute.
	//
	// Format: en, en-US.
	//
	// Default: `[en]`
	languages []string

	// Specifies the default shape rendering method.
	//
	// Will be used when an SVG element's `shape-rendering` property is set to `auto`.
	//
	// Default: GeometricPrecision
	shapeRendering ShapeRendering

	// Specifies the default text rendering method.
	//
	// Will be used when an SVG element's `text-rendering` property is set to `auto`.
	//
	// Default: OptimizeLegibility
	textRendering TextRendering

	// Specifies the default image rendering method.
	//
	// Will be used when an SVG element's `image-rendering` property is set to `auto`.
	//
	// Default: OptimizeQuality
	imageRendering ImageRendering

	// Default viewport size to assume if there is no `viewBox` attribute and
	// the `width` or `height` attributes are relative.
	//
	// Default: `(100, 100)`
	defaultSize Size

	// Specifies the way `xlink:href` in `<image>` elements should be handled.
	//
	// Default: see type's documentation for details
	imageHrefResolver ImageHrefResolver

	// Specifies how fonts should be resolved and loaded.
	fontResolver FontResolver

	// A database of fonts usable by text.
	//
	// This is a base database. If a custom `font_resolver` is specified,
	// additional fonts can be loaded during parsing. Those will be added to a
	// copy of this database. The full database containing all fonts referenced
	// in a `Tree` becomes available as [`Tree::fontdb`](crate::Tree::fontdb)
	// after parsing. If no fonts were loaded dynamically, that database will
	// be the same as this one.
	fontdb *FontDatabase

	// fontdbOwned reports whether fontdb is a copy that these options own,
	// which an option can change. It is the strong count of the Arc that
	// Arc::make_mut reads in upstream.
	fontdbOwned bool

	// A CSS stylesheet that should be injected into the SVG. Can be used to overwrite
	// certain attributes.
	//
	// Port: the empty string is None.
	styleSheet string

	// The maximum nesting allowed while parsing. If this limit is exceeded,
	// the parser will return an error.
	maxDepth uint32

	// logger is the logger of WithLogger. Nil logs nothing (D20). Upstream
	// has no such field, and logs with the crate log.
	logger *slog.Logger

	// render holds the options of the Renderer of renderer.go (D36), which
	// the parser ignores. Upstream has no such field.
	render renderOptions
}

// defaultOptions is the impl Default for Options in usvg
// src/parser/options.rs.
func defaultOptions() options {
	return options{
		resourcesDir: "",
		dpi:          96.0,
		// Default font is user-agent dependent so we can use whichever we like.
		fontFamily:        familyTimesNewRoman,
		fontSize:          12.0,
		languages:         []string{"en"},
		shapeRendering:    DefaultShapeRendering(),
		textRendering:     DefaultTextRendering(),
		imageRendering:    ImageRenderingOptimizeQuality,
		defaultSize:       mustSize(path.SizeFromWH(100.0, 100.0)),
		imageHrefResolver: ImageHrefResolver{},
		fontResolver:      FontResolver{},
		fontdb:            NewFontDatabase(),
		fontdbOwned:       true,
		styleSheet:        "",
		maxDepth:          DefaultMaxDepth,
		logger:            nil,
	}
}

// newOptions returns the default options with each option of opts applied
// in order.
func newOptions(opts ...Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return &o
}

// getAbsPath is Options::get_abs_path in usvg src/parser/options.rs. It
// converts a relative path into absolute relative to the SVG file itself.
//
// If `Options::resources_dir` is not set, returns itself.
//
// Port: Rust joins with PathBuf::join, which returns relPath when it is
// absolute. Go does the same, and cleans the result with filepath.Join.
func (o *options) getAbsPath(relPath string) string {
	if o.resourcesDir == "" || filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(o.resourcesDir, relPath)
}

// fontdbMut is Options::fontdb_mut in usvg src/parser/options.rs. It
// mutably acquires the database.
//
// This clones the database if it is currently shared.
func (o *options) fontdbMut() *FontDatabase {
	if !o.fontdbOwned {
		o.fontdb = o.fontdb.Clone()
		o.fontdbOwned = true
	}
	if o.fontdb.logger == nil {
		o.fontdb.logger = o.logger
	}
	return o.fontdb
}

// warnf logs the text of a log::warn! of upstream, made with format and args
// as fmt.Sprintf makes it, to the logger of WithLogger. Without the option,
// it logs nothing (D20).
func (o *options) warnf(format string, args ...any) {
	if o == nil || o.logger == nil {
		return
	}
	o.logger.Warn(fmt.Sprintf(format, args...))
}

// debugf logs the text of a log::debug! of upstream, made with format and
// args as fmt.Sprintf makes it, to the logger of WithLogger. Without the
// option, it logs nothing (D20).
func (o *options) debugf(format string, args ...any) {
	if o == nil || o.logger == nil {
		return
	}
	o.logger.Debug(fmt.Sprintf(format, args...))
}

// WithResourcesDir sets Options::resources_dir in usvg src/parser/options.rs:
// the directory that will be used during relative paths resolving.
//
// Expected to be the same as the directory that contains the SVG file,
// but can be set to any.
//
// Default: none. The empty string is none.
func WithResourcesDir(dir string) Option {
	return func(o *options) {
		o.resourcesDir = dir
	}
}

// WithDPI sets Options::dpi in usvg src/parser/options.rs: the target DPI.
//
// Impacts units conversion.
//
// Default: 96.0.
func WithDPI(dpi float32) Option {
	return func(o *options) {
		o.dpi = dpi
	}
}

// WithFontFamily sets Options::font_family in usvg src/parser/options.rs: a
// default font family.
//
// Will be used when no `font-family` attribute is set in the SVG.
//
// Default: Times New Roman.
func WithFontFamily(family string) Option {
	return func(o *options) {
		o.fontFamily = family
		o.render.fontFamilySet = true
	}
}

// WithFontSize sets Options::font_size in usvg src/parser/options.rs: a
// default font size.
//
// Will be used when no `font-size` attribute is set in the SVG.
//
// Default: 12.
func WithFontSize(size float32) Option {
	return func(o *options) {
		o.fontSize = size
	}
}

// WithLanguages sets Options::languages in usvg src/parser/options.rs: a list
// of languages.
//
// Will be used to resolve a `systemLanguage` conditional attribute.
//
// Format: en, en-US.
//
// Default: `[en]`.
func WithLanguages(languages ...string) Option {
	return func(o *options) {
		o.languages = slices.Clone(languages)
	}
}

// WithShapeRendering sets Options::shape_rendering in usvg
// src/parser/options.rs: the default shape rendering method.
//
// Will be used when an SVG element's `shape-rendering` property is set to `auto`.
//
// Default: GeometricPrecision.
func WithShapeRendering(rendering ShapeRendering) Option {
	return func(o *options) {
		o.shapeRendering = rendering
	}
}

// WithTextRendering sets Options::text_rendering in usvg
// src/parser/options.rs: the default text rendering method.
//
// Will be used when an SVG element's `text-rendering` property is set to `auto`.
//
// Default: OptimizeLegibility.
func WithTextRendering(rendering TextRendering) Option {
	return func(o *options) {
		o.textRendering = rendering
	}
}

// WithImageRendering sets Options::image_rendering in usvg
// src/parser/options.rs: the default image rendering method.
//
// Will be used when an SVG element's `image-rendering` property is set to `auto`.
//
// Default: OptimizeQuality.
func WithImageRendering(rendering ImageRendering) Option {
	return func(o *options) {
		o.imageRendering = rendering
	}
}

// WithDefaultSize sets Options::default_size in usvg src/parser/options.rs:
// the default viewport size to assume if there is no `viewBox` attribute and
// the `width` or `height` attributes are relative.
//
// Default: `(100, 100)`.
func WithDefaultSize(size Size) Option {
	return func(o *options) {
		o.defaultSize = size
	}
}

// WithImageHrefResolver sets Options::image_href_resolver in usvg
// src/parser/options.rs: the way `xlink:href` in `<image>` elements should
// be handled.
//
// Default: see the documentation of ImageHrefResolver.
func WithImageHrefResolver(resolver ImageHrefResolver) Option {
	return func(o *options) {
		o.imageHrefResolver = resolver
	}
}

// WithFontResolver sets Options::font_resolver in usvg
// src/parser/options.rs: how fonts should be resolved and loaded.
//
// Default: the zero FontResolver, which selects fonts with
// DefaultFontSelector and DefaultFallbackSelector.
func WithFontResolver(resolver FontResolver) Option {
	return func(o *options) {
		o.fontResolver = resolver
	}
}

// WithFontDatabase sets Options::fontdb in usvg src/parser/options.rs: a
// database of fonts usable by text.
//
// This is a base database. If a custom font resolver loads more fonts during
// parsing, they go into a copy of this database, which Tree.FontDB returns.
// The parse never changes db, so a program can load its fonts once and share
// db between parses. The options WithFontFile, WithFontFS and the options of
// the generic families change a copy of db. A Renderer with this option
// loads no font and sets no generic family (D36).
//
// Default: an empty database (D9).
func WithFontDatabase(db *FontDatabase) Option {
	return func(o *options) {
		o.fontdb = db
		o.fontdbOwned = false
		o.render.customFontDB = true
	}
}

// WithFontFile loads a font file, given as its bytes, into the database of
// Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().load_font_data(data)` does. A collection loads each of
// its faces. The database keeps data, so the caller must not change it
// afterwards.
func WithFontFile(data []byte) Option {
	return func(o *options) {
		o.fontdbMut().LoadFontData(data)
	}
}

// WithFontFS loads each font file of fsys, in its folders too, into the
// database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().load_fonts_dir(dir)` does. It reads the files whose
// names end in .ttf, .ttc, .otf or .otc. Use os.DirFS for a folder of the
// operating system, and LoadSystemFonts of FontDatabase for the fonts of the
// system (D9).
func WithFontFS(fsys fs.FS) Option {
	return func(o *options) {
		o.fontdbMut().LoadFontsFS(fsys, ".")
	}
}

// WithSerifFamily sets the family of the generic family serif in the
// database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().set_serif_family(family)` does.
//
// Default: Times New Roman.
func WithSerifFamily(family string) Option {
	return func(o *options) {
		o.fontdbMut().SetSerifFamily(family)
		o.render.families |= familySerifSet
	}
}

// WithSansSerifFamily sets the family of the generic family sans-serif in
// the database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().set_sans_serif_family(family)` does.
//
// Default: Arial.
func WithSansSerifFamily(family string) Option {
	return func(o *options) {
		o.fontdbMut().SetSansSerifFamily(family)
		o.render.families |= familySansSerifSet
	}
}

// WithCursiveFamily sets the family of the generic family cursive in the
// database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().set_cursive_family(family)` does.
//
// Default: Comic Sans MS.
func WithCursiveFamily(family string) Option {
	return func(o *options) {
		o.fontdbMut().SetCursiveFamily(family)
		o.render.families |= familyCursiveSet
	}
}

// WithFantasyFamily sets the family of the generic family fantasy in the
// database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().set_fantasy_family(family)` does.
//
// Default: Impact.
func WithFantasyFamily(family string) Option {
	return func(o *options) {
		o.fontdbMut().SetFantasyFamily(family)
		o.render.families |= familyFantasySet
	}
}

// WithMonospaceFamily sets the family of the generic family monospace in the
// database of Options::fontdb in usvg src/parser/options.rs, as
// `opt.fontdb_mut().set_monospace_family(family)` does.
//
// Default: Courier New.
func WithMonospaceFamily(family string) Option {
	return func(o *options) {
		o.fontdbMut().SetMonospaceFamily(family)
		o.render.families |= familyMonospaceSet
	}
}

// WithStyleSheet sets Options::style_sheet in usvg src/parser/options.rs: a
// CSS stylesheet that should be injected into the SVG. Can be used to
// overwrite certain attributes.
//
// Default: none. The empty string is none.
func WithStyleSheet(styleSheet string) Option {
	return func(o *options) {
		o.styleSheet = styleSheet
	}
}

// WithMaxDepth sets Options::max_depth in usvg src/parser/options.rs: the
// maximum nesting allowed while parsing. If this limit is exceeded, the
// parser will return an error that wraps okage.ErrXMLNodesLimitReached.
//
// Default: DefaultMaxDepth.
func WithMaxDepth(depth uint32) Option {
	return func(o *options) {
		o.maxDepth = depth
	}
}

// WithLogger sets the logger that takes the warnings and the debug messages
// that upstream logs with the crate log (D20). Without this option, the
// package logs nothing. Upstream has no such field.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// mustSize is the unwrap of an Option<Size> that upstream relies on. It
// panics when ok is false.
func mustSize(s Size, ok bool) Size {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return s
}
