package svg

import (
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

	"github.com/xo/okage"
)

// This file ports no upstream file. It holds the renderer of D36, which
// takes the place of the API of github.com/xo/resvg: a Renderer parses and
// draws SVG data in one call, and loads the fonts of the system when the first
// call needs them.

func init() {
	image.RegisterFormat("svg", "<?xml", Decode, DecodeConfig)
	image.RegisterFormat("svg", "<svg", Decode, DecodeConfig)
}

// defaultRenderer is the renderer of Decode and DecodeConfig. New loads no
// font, so making it when the package starts costs nothing (D36).
var defaultRenderer = New()

// Decode renders SVG data from r with a renderer that has the default
// options. The package registers it with image.RegisterFormat for the format
// "svg". The first call loads the fonts of the system.
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return defaultRenderer.Render(data)
}

// DecodeConfig returns the size of the image that Decode renders from r. The
// package registers it with image.RegisterFormat for the format "svg".
func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return image.Config{}, err
	}
	return defaultRenderer.ParseConfig(data)
}

// Renderer parses SVG data and renders it into an *image.RGBA (D36). New
// makes one. A Renderer is safe for concurrent use, and its calls share one
// font database.
//
// Unless WithLoadSystemFonts(false) or WithFontDatabase is given, the first
// call loads the fonts of the system and of the home directory, and sets each
// generic family to the first of its candidates that the database holds. See
// SerifCandidates and the other lists of candidates.
type Renderer struct {
	opts []Option
	once sync.Once
	opt  *options
}

// New returns a renderer with the options opts. It loads no font. The first
// call of the renderer applies the options and loads the fonts.
func New(opts ...Option) *Renderer {
	return &Renderer{opts: slices.Clone(opts)}
}

// options returns the options of the renderer, which the first call makes.
func (r *Renderer) options() *options {
	r.once.Do(r.init)
	return r.opt
}

// init applies the options, loads the fonts of the system and resolves the
// generic families.
func (r *Renderer) init() {
	o := newOptions(r.opts...)
	if !o.render.customFontDB {
		db := o.fontdbMut()
		if !o.render.noSystemFonts {
			loadOSFonts(db)
		}
		resolveGenericFamilies(db, o)
	}
	r.opt = o
}

// Parse parses SVG data, plain or compressed with gzip. Parsing is the
// expensive part of a render. A program that needs the size of the image and
// the image itself keeps the Document and does not parse data twice.
func (r *Renderer) Parse(data []byte) (*Document, error) {
	// A copy of the options that does not own the database, so that a parse
	// that changes the database changes a copy of it.
	opt := *r.options()
	opt.fontdbOwned = false
	tree, err := fromData(data, &opt)
	if err != nil {
		return nil, err
	}
	return &Document{r: r, tree: tree}, nil
}

// ParseConfig parses SVG data and returns the size of the image that Render
// makes from it.
func (r *Renderer) ParseConfig(data []byte) (image.Config, error) {
	doc, err := r.Parse(data)
	if err != nil {
		return image.Config{}, err
	}
	return doc.Config()
}

// Render parses SVG data and renders it into an *image.RGBA.
func (r *Renderer) Render(data []byte) (*image.RGBA, error) {
	doc, err := r.Parse(data)
	if err != nil {
		return nil, err
	}
	return doc.Render()
}

// Document is SVG data that Renderer.Parse parsed. It reads the size, the
// scale, the background and the transform of its renderer. A Document is safe
// for concurrent use.
type Document struct {
	r    *Renderer
	tree *Tree
}

// Tree returns the tree of the document.
func (d *Document) Tree() *Tree {
	return d.tree
}

// scale returns the size of the image and the scale of each axis, from the
// size of the tree and the options of the renderer.
func (d *Document) scale() (int, int, float32, float32, error) {
	ro := &d.r.options().render
	size := d.tree.Size().ToIntSize()
	width, height, scaleX, scaleY := ro.scaleMode.Scale(uint(size.Width()), uint(size.Height()), ro.width, ro.height)
	switch {
	case width <= 0 || height <= 0:
		return 0, 0, 0, 0, okage.ErrSVGInvalidRenderSize
	case scaleX == 0 || scaleY == 0:
		return 0, 0, 0, 0, okage.ErrSVGInvalidRenderScale
	}
	return width, height, scaleX, scaleY, nil
}

// Config returns the size of the image that Render makes. Its color model is
// color.RGBAModel.
func (d *Document) Config() (image.Config, error) {
	width, height, _, _, err := d.scale()
	if err != nil {
		return image.Config{}, err
	}
	return image.Config{ColorModel: color.RGBAModel, Width: width, Height: height}, nil
}

// Render renders the document into a new *image.RGBA. The pixels are
// premultiplied, as image.RGBA expects. The image is filled with the color of
// WithBackground first, and the transform of WithTransform replaces the scale
// of WithWidth, WithHeight and WithScaleMode.
func (d *Document) Render() (*image.RGBA, error) {
	width, height, scaleX, scaleY, err := d.scale()
	if err != nil {
		return nil, err
	}
	if uint64(width) > math.MaxUint32 || uint64(height) > math.MaxUint32 {
		return nil, okage.ErrSVGInvalidRenderSize
	}
	pixmap, ok := okage.NewPixmap(uint32(width), uint32(height))
	if !ok {
		return nil, okage.ErrSVGInvalidRenderSize
	}
	o := d.r.options()
	img := pixmap.AsRGBA()
	if bg := o.render.background; bg != nil {
		if r, g, b, a := bg.RGBA(); a != 0 {
			c := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			for i := 0; i < len(img.Pix); i += 4 {
				copy(img.Pix[i:i+4], c[:])
			}
		}
	}
	transform := okage.TransformFromScale(scaleX, scaleY)
	if o.render.hasTransform {
		transform = o.render.transform
	}
	mut := pixmap.AsMut()
	ctx := &context{maxBBox: maxFilterBBox(mut.Width(), mut.Height()), opt: o}
	renderNodes(d.tree.Root(), ctx, transform, &mut)
	return img, nil
}

// renderOptions holds the options of the renderer of D36. The functions of
// the parser ignore them.
type renderOptions struct {
	// background is the color of WithBackground. Nil is transparent.
	background color.Color

	// width and height are the sizes of WithWidth and WithHeight. Zero is
	// the size of the file.
	width, height uint

	// scaleMode is the mode of WithScaleMode.
	scaleMode ScaleMode

	// transform is the transform of WithTransform, when hasTransform is true.
	transform    Transform
	hasTransform bool

	// noSystemFonts is true after WithLoadSystemFonts(false).
	noSystemFonts bool

	// customFontDB is true after WithFontDatabase. The renderer then loads
	// no font and sets no family.
	customFontDB bool

	// fontFamilySet is true after WithFontFamily, and families holds a bit
	// for each generic family that an option set.
	fontFamilySet bool
	families      uint8
}

// The bits of renderOptions.families.
const (
	familySerifSet uint8 = 1 << iota
	familySansSerifSet
	familyCursiveSet
	familyFantasySet
	familyMonospaceSet
)

// WithBackground sets the color that the renderer fills each image with
// before it draws. The default is transparent. The parser ignores it.
func WithBackground(background color.Color) Option {
	return func(o *options) {
		o.render.background = background
	}
}

// WithWidth sets the width of the images of the renderer, which
// WithScaleMode uses. Zero is the width of the file. The parser ignores it.
func WithWidth(width int) Option {
	return func(o *options) {
		o.render.width = uint(max(width, 0))
	}
}

// WithHeight sets the height of the images of the renderer, which
// WithScaleMode uses. Zero is the height of the file. The parser ignores it.
func WithHeight(height int) Option {
	return func(o *options) {
		o.render.height = uint(max(height, 0))
	}
}

// WithScaleMode sets how the renderer scales an image to the width of
// WithWidth and the height of WithHeight. The default is ScaleNone. The
// parser ignores it.
func WithScaleMode(mode ScaleMode) Option {
	return func(o *options) {
		o.render.scaleMode = mode
	}
}

// WithTransform sets the root transform of the renderer. It replaces the
// scale of WithWidth, WithHeight and WithScaleMode, but the size of the image
// still comes from them. The parser ignores it.
func WithTransform(transform Transform) Option {
	return func(o *options) {
		o.render.transform = transform
		o.render.hasTransform = true
	}
}

// WithLoadSystemFonts sets whether the renderer loads the fonts of the system
// and of the home directory on its first call. The default is true. The
// parser ignores it, and never loads the fonts of the system.
func WithLoadSystemFonts(load bool) Option {
	return func(o *options) {
		o.render.noSystemFonts = !load
	}
}

// ScaleMode is how a renderer scales an image to the size of WithWidth and
// WithHeight.
type ScaleMode uint8

// The scale modes.
const (
	// ScaleNone uses the width and the height that are set, and the size of
	// the file for each one that is zero. The image can stretch.
	ScaleNone ScaleMode = iota
	// ScaleMinWidth scales the image up to the width, when it is narrower.
	ScaleMinWidth
	// ScaleMinHeight scales the image up to the height, when it is shorter.
	ScaleMinHeight
	// ScaleMaxWidth scales the image down to the width, when it is wider.
	ScaleMaxWidth
	// ScaleMaxHeight scales the image down to the height, when it is taller.
	ScaleMaxHeight
	// ScaleBestFit scales the image to fit in the width and the height, and
	// keeps its aspect ratio. A zero width or height is no limit.
	ScaleBestFit
)

// Scale returns the size of an image of width by height pixels that is
// scaled to w by h pixels in the mode, and the scale of each axis. It gives
// the results of ScaleMode.Scale of github.com/xo/resvg.
func (mode ScaleMode) Scale(width, height, w, h uint) (int, int, float32, float32) {
	switch mode {
	case ScaleMinWidth:
		return scaleWidth(width, height, w, width < w)
	case ScaleMinHeight:
		return scaleHeight(width, height, h, height < h)
	case ScaleMaxWidth:
		return scaleWidth(width, height, w, width > w)
	case ScaleMaxHeight:
		return scaleHeight(width, height, h, height > h)
	case ScaleBestFit:
		return scaleBestFit(width, height, w, h)
	}
	scaleX, scaleY := float32(1), float32(1)
	if w != 0 {
		width, scaleX = w, float32(w)/float32(width)
	}
	if h != 0 {
		height, scaleY = h, float32(h)/float32(height)
	}
	return int(width), int(height), scaleX, scaleY
}

// scaleWidth scales an image to the width w when scale is true.
func scaleWidth(width, height, w uint, scale bool) (int, int, float32, float32) {
	if !scale {
		return int(width), int(height), 1, 1
	}
	s := float32(w) / float32(width)
	return int(w), int(math.Round(float64(float32(height) * s))), s, s
}

// scaleHeight scales an image to the height h when scale is true.
func scaleHeight(width, height, h uint, scale bool) (int, int, float32, float32) {
	if !scale {
		return int(width), int(height), 1, 1
	}
	s := float32(h) / float32(height)
	return int(math.Round(float64(float32(width) * s))), int(h), s, s
}

// scaleBestFit scales an image to fit in w by h, and keeps its aspect ratio.
func scaleBestFit(width, height, w, h uint) (int, int, float32, float32) {
	var s float32
	switch {
	case w == 0:
		s = float32(h) / float32(height)
	case h == 0:
		s = float32(w) / float32(width)
	default:
		s = min(float32(w)/float32(width), float32(h)/float32(height))
	}
	return int(math.Round(float64(float32(width) * s))), int(math.Round(float64(float32(height) * s))), s, s
}

// The families that more than one list of candidates, or the defaults of
// fontdb and usvg, name.
const (
	familyTimesNewRoman = "Times New Roman"
	familyDejaVuSans    = "DejaVu Sans"
)

// The candidates of each generic family of CSS, in order (D36). The renderer
// sets each generic family to the first candidate that its font database
// holds. They are metric-compatible fonts and common free fonts, so that text
// keeps its width and its script on each system.
var (
	SerifCandidates = []string{
		familyTimesNewRoman, "Liberation Serif", "Tinos", "Nimbus Roman",
		"DejaVu Serif", "Noto Serif", "FreeSerif",
	}
	SansSerifCandidates = []string{
		"Arial", "Liberation Sans", "Arimo", "Nimbus Sans", "Helvetica",
		familyDejaVuSans, "Noto Sans", "FreeSans",
	}
	MonospaceCandidates = []string{
		"Courier New", "Liberation Mono", "Cousine", "Nimbus Mono PS",
		"DejaVu Sans Mono", "Noto Sans Mono", "Menlo", "Consolas", "FreeMono",
	}
	CursiveCandidates = []string{
		"Comic Sans MS", "Apple Chancery", "Z003", "URW Chancery L",
		familyDejaVuSans,
	}
	FantasyCandidates = []string{
		"Impact", "Papyrus", "P052", "URW Palladio L", familyDejaVuSans,
	}
)

// resolveGenericFamilies sets each generic family of db that no option set
// to its first candidate that db holds, and sets the default family of o to
// the family of serif, unless WithFontFamily set it.
func resolveGenericFamilies(db *FontDatabase, o *options) {
	names := make(map[string]bool)
	for face := range db.Faces() {
		for _, f := range face.Families {
			names[f.Name] = true
		}
	}
	for _, g := range []struct {
		bit        uint8
		candidates []string
		set        func(string)
	}{
		{familySerifSet, SerifCandidates, db.SetSerifFamily},
		{familySansSerifSet, SansSerifCandidates, db.SetSansSerifFamily},
		{familyCursiveSet, CursiveCandidates, db.SetCursiveFamily},
		{familyFantasySet, FantasyCandidates, db.SetFantasyFamily},
		{familyMonospaceSet, MonospaceCandidates, db.SetMonospaceFamily},
	} {
		if o.render.families&g.bit != 0 {
			continue
		}
		if i := slices.IndexFunc(g.candidates, func(c string) bool { return names[c] }); i >= 0 {
			g.set(g.candidates[i])
		}
	}
	if !o.render.fontFamilySet {
		o.fontFamily = db.FamilyName(FontFamily{Kind: FontFamilySerif})
	}
}

// loadOSFonts loads the fonts of the system and of the home directory into
// db. On Windows, the root of the system is the volume of %WINDIR%. This is
// the one choice by platform outside internal/wide (D36).
func loadOSFonts(db *FontDatabase) {
	root := "/"
	if runtime.GOOS == "windows" {
		root = `C:\`
		if v := filepath.VolumeName(os.Getenv("WINDIR")); v != "" {
			root = v + `\`
		}
	}
	db.LoadSystemFonts(os.DirFS(root))
	if home, err := os.UserHomeDir(); err == nil {
		db.LoadUserFonts(os.DirFS(home))
	}
}
