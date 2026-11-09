// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/binary"
	"math"
	"slices"
	"strconv"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/converter.rs.
//
// Port: the structs State and Cache are private to the crate. They become
// converterState and converterCache, because the parser names its local
// variables state and cache after the Rust code. A shared Arc<T> of upstream
// is a pointer *T.

// converterState is State in usvg src/parser/converter.rs.
type converterState struct {
	// parentClipPath is nil for None.
	parentClipPath *svgNode
	parentMarkers  []svgNode
	// Stores the resolved fill and stroke of a use node
	// or a path element (for markers)
	//
	// Port: nil is None.
	contextElement *contextFillStroke
	feImageLink    bool
	// A viewBox of the parent SVG element.
	viewBox NonZeroRect
	// A size of the parent `use` element.
	// Used only during nested `svg` size resolving.
	// Width and height can be set independently.
	//
	// Port: the field use_size of upstream is a pair of Option<f32>. The
	// flags hasUseWidth and hasUseHeight are false for None.
	useWidth     float32
	useHeight    float32
	hasUseWidth  bool
	hasUseHeight bool
	opt          *options
}

// contextFillStroke is the (Option<Fill>, Option<Stroke>) of the field
// context_element of State in usvg src/parser/converter.rs. A nil field is
// None.
type contextFillStroke struct {
	fill   *Fill
	stroke *Stroke
}

// clone is the impl Clone for State in usvg src/parser/converter.rs. It
// copies the list of parent markers, so that an append to the copy does not
// change the original.
func (s *converterState) clone() *converterState {
	c := *s
	c.parentMarkers = slices.Clone(s.parentMarkers)
	return &c
}

// converterCache is Cache in usvg src/parser/converter.rs.
//
// Port: upstream keeps the hash of each id in all_ids, a HashSet<u64>. Go
// keeps the ids themselves, so a new id never collides with the hash of
// another id.
type converterCache struct {
	// This fontdb is initialized from [`Options::fontdb`] and then populated
	// over the course of conversion.
	//
	// Port: it is a copy of the database of the options, which the
	// conversion can change (see FontSelectionFn).
	fontdb *FontDatabase

	// Port: Go keys each cache with a text that glyphCacheKey makes.
	cacheOutline map[string]*path.Path
	cacheColr    map[string]*Tree
	cacheSvg     map[string]Node
	cacheRaster  map[string]*bitmapImage
	cacheHasOpsz map[FontID]bool
	shapingFonts shapingFonts

	clipPaths map[string]*ClipPath
	masks     map[string]*Mask
	filters   map[string]*Filter
	paint     map[string]Paint

	// used for ID generation
	allIDs              map[string]struct{}
	linearGradientIndex int
	radialGradientIndex int
	patternIndex        int
	clipPathIndex       int
	textPathIndex       int
	maskIndex           int
	filterIndex         int
	imageIndex          int

	// opt holds the logger of the options (D20). Upstream logs with the
	// crate log, so its Cache has no such field.
	opt *options
	// paintRefs counts, for each paint server, the paints of the tree that
	// hold it, while update_paint_servers runs at the end of convert_doc.
	// It is nil at other times. Upstream has no such field: it is the strong
	// count of each Arc that Arc::get_mut reads.
	paintRefs map[Paint]int
}

// newConverterCache is Cache::new in usvg src/parser/converter.rs.
func newConverterCache(fontdb *FontDatabase) *converterCache {
	return &converterCache{
		fontdb: fontdb,

		cacheOutline: map[string]*path.Path{},
		cacheColr:    map[string]*Tree{},
		cacheSvg:     map[string]Node{},
		cacheRaster:  map[string]*bitmapImage{},
		cacheHasOpsz: map[FontID]bool{},
		shapingFonts: shapingFonts{},

		clipPaths: map[string]*ClipPath{},
		masks:     map[string]*Mask{},
		filters:   map[string]*Filter{},
		paint:     map[string]Paint{},

		allIDs:              map[string]struct{}{},
		linearGradientIndex: 0,
		radialGradientIndex: 0,
		patternIndex:        0,
		clipPathIndex:       0,
		textPathIndex:       0,
		maskIndex:           0,
		filterIndex:         0,
		imageIndex:          0,
	}
}

// genID is the loop that each Cache::gen_*_id in usvg
// src/parser/converter.rs has. It increments index until the id prefix and
// index is not an id of the document.
//
// TODO: macros?
func (c *converterCache) genID(prefix string, index *int) nonEmptyString {
	for {
		*index++
		newID := prefix + strconv.Itoa(*index)
		if _, ok := c.allIDs[stringHash(newID)]; !ok {
			id, _ := newNonEmptyString(newID)
			return id
		}
	}
}

// genLinearGradientID is Cache::gen_linear_gradient_id in usvg
// src/parser/converter.rs.
func (c *converterCache) genLinearGradientID() nonEmptyString {
	return c.genID("linearGradient", &c.linearGradientIndex)
}

// genRadialGradientID is Cache::gen_radial_gradient_id in usvg
// src/parser/converter.rs.
func (c *converterCache) genRadialGradientID() nonEmptyString {
	return c.genID("radialGradient", &c.radialGradientIndex)
}

// genPatternID is Cache::gen_pattern_id in usvg src/parser/converter.rs.
func (c *converterCache) genPatternID() nonEmptyString {
	return c.genID("pattern", &c.patternIndex)
}

// genClipPathID is Cache::gen_clip_path_id in usvg src/parser/converter.rs.
func (c *converterCache) genClipPathID() nonEmptyString {
	return c.genID("clipPath", &c.clipPathIndex)
}

// genTextPathID is Cache::gen_text_path_id in usvg src/parser/converter.rs.
func (c *converterCache) genTextPathID() nonEmptyString {
	return c.genID("textPathPath", &c.textPathIndex)
}

// genMaskID is Cache::gen_mask_id in usvg src/parser/converter.rs.
func (c *converterCache) genMaskID() nonEmptyString {
	return c.genID("mask", &c.maskIndex)
}

// genFilterID is Cache::gen_filter_id in usvg src/parser/converter.rs.
func (c *converterCache) genFilterID() nonEmptyString {
	return c.genID("filter", &c.filterIndex)
}

// genImageID is Cache::gen_image_id in usvg src/parser/converter.rs.
func (c *converterCache) genImageID() nonEmptyString {
	return c.genID("image", &c.imageIndex)
}

// paintIsUnique reports whether no other value holds the paint server p,
// which is when Arc::get_mut of upstream gives a mutable reference.
//
// Port: Go has no reference count. While the cache holds the paint servers,
// each paint server of the document is in cache.paint, so upstream sees a
// second Arc. When convert_doc has cleared the cache, paintRefs holds the
// count of each paint server.
func (c *converterCache) paintIsUnique(p Paint) bool {
	if c.paintRefs != nil {
		return c.paintRefs[p] <= 1
	}
	for _, v := range c.paint {
		if v == p {
			return false
		}
	}
	return true
}

// replacePaint records that a paint which held old holds repl now, as the
// assignment of a new Arc does in upstream.
func (c *converterCache) replacePaint(old, repl Paint) {
	if c.paintRefs != nil {
		c.paintRefs[old]--
		c.paintRefs[repl]++
	}
}

// countPaintRefs counts the paints that hold each paint server in the tree
// of root, its subroots and its nested SVG images. A subroot that many nodes
// share counts once, as the Arc that holds it does in upstream.
func countPaintRefs(root *Group) map[Paint]int {
	refs := map[Paint]int{}
	add := func(p Paint) {
		if _, ok := p.(Color); !ok && p != nil {
			refs[p]++
		}
	}
	seen := map[*Group]bool{}
	var walk func(g *Group)
	walk = func(g *Group) {
		if g == nil || seen[g] {
			return
		}
		seen[g] = true
		for _, node := range g.children {
			switch n := node.(type) {
			case *Group:
				walk(n)
			case *Path:
				if n.fill != nil {
					add(n.fill.paint)
				}
				if n.stroke != nil {
					add(n.stroke.paint)
				}
			case *Image:
				if svg, ok := n.kind.(ImageKindSVG); ok {
					for _, p := range svg.Tree.linearGradients {
						add(p)
					}
					for _, p := range svg.Tree.radialGradients {
						add(p)
					}
					for _, p := range svg.Tree.patterns {
						add(p)
					}
				}
			case *Text:
				addFill := func(f *Fill) {
					if f != nil {
						add(f.paint)
					}
				}
				addStroke := func(s *Stroke) {
					if s != nil {
						add(s.paint)
					}
				}
				addStyle := func(s *TextDecorationStyle) {
					if s != nil {
						addFill(s.fill)
						addStroke(s.stroke)
					}
				}
				addPath := func(p *Path) {
					if p != nil {
						addFill(p.fill)
						addStroke(p.stroke)
					}
				}
				for i := range n.chunks {
					for j := range n.chunks[i].spans {
						span := &n.chunks[i].spans[j]
						addFill(span.fill)
						addStroke(span.stroke)
						addStyle(span.decoration.underline)
						addStyle(span.decoration.overline)
						addStyle(span.decoration.lineThrough)
					}
				}
				for i := range n.layouted {
					span := &n.layouted[i]
					addFill(span.Fill)
					addStroke(span.Stroke)
					addPath(span.Overline)
					addPath(span.Underline)
					addPath(span.LineThrough)
				}
				walk(n.flattened)
			}
			node.Subroots(walk)
		}
	}
	walk(root)
	return refs
}

// glyphCacheKey returns the key of a glyph of a font with some
// variations in a cache of Cache in usvg src/parser/converter.rs, which keys
// it with the tuple (ID, GlyphId, Vec<FontVariation>).
func glyphCacheKey(font FontID, glyph GlyphID, variations []FontVariation) string {
	b := make([]byte, 0, 8+8*len(variations))
	b = binary.BigEndian.AppendUint32(b, uint32(font))
	b = binary.BigEndian.AppendUint32(b, uint32(glyph))
	for _, v := range variations {
		b = append(b, v.Tag[:]...)
		b = binary.BigEndian.AppendUint32(b, math.Float32bits(v.Value))
	}
	return string(b)
}

// fontdbSvg is Cache::fontdb_svg in usvg src/parser/converter.rs, which the
// macro font_lookup makes.
func (c *converterCache) fontdbSvg(font FontID, glyph GlyphID) (Node, bool) {
	key := glyphCacheKey(font, glyph, nil)
	if hit, ok := c.cacheSvg[key]; ok {
		return hit, hit != nil
	}
	lookup, _ := databaseSvg(c.fontdb, font, glyph, c.opt)
	c.cacheSvg[key] = lookup
	return lookup, lookup != nil
}

// fontdbRaster is Cache::fontdb_raster in usvg src/parser/converter.rs,
// which the macro font_lookup makes.
func (c *converterCache) fontdbRaster(font FontID, glyph GlyphID) (*bitmapImage, bool) {
	key := glyphCacheKey(font, glyph, nil)
	if hit, ok := c.cacheRaster[key]; ok {
		return hit, hit != nil
	}
	lookup, _ := databaseRaster(c.fontdb, font, glyph)
	c.cacheRaster[key] = lookup
	return lookup, lookup != nil
}

// fontdbOutline is Cache::fontdb_outline in usvg src/parser/converter.rs.
func (c *converterCache) fontdbOutline(font FontID, glyph GlyphID, variations []FontVariation) (*path.Path, bool) {
	key := glyphCacheKey(font, glyph, variations)
	if hit, ok := c.cacheOutline[key]; ok {
		return hit, hit != nil
	}
	lookup, _ := databaseOutline(c.fontdb, font, glyph, variations)
	c.cacheOutline[key] = lookup
	return lookup, lookup != nil
}

// fontdbColr is Cache::fontdb_colr in usvg src/parser/converter.rs.
func (c *converterCache) fontdbColr(font FontID, glyph GlyphID, variations []FontVariation) (*Tree, bool) {
	key := glyphCacheKey(font, glyph, variations)
	if hit, ok := c.cacheColr[key]; ok {
		return hit, hit != nil
	}
	lookup, _ := databaseColr(c.fontdb, font, glyph, variations, c.opt)
	c.cacheColr[key] = lookup
	return lookup, lookup != nil
}

// hasOpszAxis is Cache::has_opsz_axis in usvg src/parser/converter.rs.
func (c *converterCache) hasOpszAxis(font FontID) bool {
	if cached, ok := c.cacheHasOpsz[font]; ok {
		return cached
	}
	hasOpsz := databaseHasOpszAxis(c.fontdb, font)
	c.cacheHasOpsz[font] = hasOpsz
	return hasOpsz
}

// stringHash is string_hash in usvg src/parser/converter.rs.
//
// TODO: is there a simpler way?
//
// Port: Go keeps the string itself in the set, so the hash is the string.
func stringHash(s string) string {
	return s
}

// convertLength is SvgNode::convert_length in usvg src/parser/converter.rs.
func (n svgNode) convertLength(
	aid AId,
	objectUnits units,
	state *converterState,
	def svgtypes.Length,
) float32 {
	length, ok := n.attributeLength(aid)
	if !ok {
		length = def
	}
	return convertLength(
		length,
		n,
		aid,
		objectUnits,
		state,
	)
}

// convertUserLength is SvgNode::convert_user_length in usvg
// src/parser/converter.rs.
func (n svgNode) convertUserLength(aid AId, state *converterState, def svgtypes.Length) float32 {
	return n.convertLength(aid, unitsUserSpaceOnUse, state, def)
}

// parseViewbox is SvgNode::parse_viewbox in usvg src/parser/converter.rs.
func (n svgNode) parseViewbox() (NonZeroRect, bool) {
	vb, ok := nodeAttribute(n, AIdViewBox, fromValueViewBox)
	if !ok {
		return NonZeroRect{}, false
	}
	return path.NonZeroRectFromXYWH(float32(vb.X), float32(vb.Y), float32(vb.W), float32(vb.H))
}

// resolveLength is SvgNode::resolve_length in usvg src/parser/converter.rs.
func (n svgNode) resolveLength(aid AId, state *converterState, def float32) float32 {
	if a, ok := findAncestorWithAttribute(n, aid); ok {
		if length, ok := a.attributeLength(aid); ok {
			return convertUserLength(length, a, aid, state)
		}
	}

	return def
}

// resolveValidLength is SvgNode::resolve_valid_length in usvg
// src/parser/converter.rs.
func (n svgNode) resolveValidLength(
	aid AId,
	state *converterState,
	def float32,
) (NonZeroPositiveF32, bool) {
	v := n.resolveLength(aid, state, def)
	return path.NewNonZeroPositiveF32(v)
}

// tryConvertLength is SvgNode::try_convert_length in usvg
// src/parser/converter.rs.
func (n svgNode) tryConvertLength(
	aid AId,
	objectUnits units,
	state *converterState,
) (float32, bool) {
	length, ok := n.attributeLength(aid)
	if !ok {
		return 0, false
	}
	return convertLength(
		length,
		n,
		aid,
		objectUnits,
		state,
	), true
}

// hasValidTransform is SvgNode::has_valid_transform in usvg
// src/parser/converter.rs.
func (n svgNode) hasValidTransform(aid AId) bool {
	// Do not use Node::attribute::<Transform>, because it will always
	// return a valid transform.

	attr, ok := n.attributeStr(aid)
	if !ok {
		return true
	}

	ts, err := svgtypes.ParseTransform(attr)
	if err != nil {
		return true
	}

	t := path.TransformFromRow(
		float32(ts.A),
		float32(ts.B),
		float32(ts.C),
		float32(ts.D),
		float32(ts.E),
		float32(ts.F),
	)
	return t.IsValid()
}

// isVisibleElement is SvgNode::is_visible_element in usvg
// src/parser/converter.rs.
func (n svgNode) isVisibleElement(opt *options) bool {
	display, ok := n.attributeStr(AIdDisplay)
	return (!ok || display != "none") && //nolint:goconst // an SVG keyword, as upstream writes it
		n.hasValidTransform(AIdTransform) &&
		isConditionPassed(n, opt)
}

// splitAlpha is SvgColorExt::split_alpha for svgtypes::Color in usvg
// src/parser/converter.rs.
func splitAlpha(c svgtypes.Color) (Color, Opacity) {
	return NewColorRGB(c.Red, c.Green, c.Blue), path.NormalizedF32FromU8(c.Alpha)
}

// convertDoc is convert_doc in usvg src/parser/converter.rs. It converts an
// input `Document` into a `Tree`.
//
// # Errors
//
// - If `Document` doesn't have an SVG node - returns an empty tree.
// - If `Document` doesn't have a valid size - returns `Error::InvalidSize`.
func convertDoc(svgDoc *document, opt *options) (*Tree, error) {
	svg := svgDoc.rootElement()
	size, sizeErr, restoreViewbox := resolveSvgSize(svg, opt)
	if sizeErr != nil {
		return nil, sizeErr
	}
	rect, ok := svg.parseViewbox()
	if !ok {
		rect = size.ToNonZeroRect(0.0, 0.0)
	}
	aspect, ok := nodeAttribute(svg, AIdPreserveAspectRatio, fromValueAspectRatio)
	if !ok {
		aspect = svgtypes.DefaultAspectRatio()
	}
	vb := viewBox{
		rect:   rect,
		aspect: aspect,
	}

	var backgroundColor svgtypes.Color
	hasBackgroundColor := false
	if s, ok := svg.attributeStr(AIdBackgroundColor); ok {
		if p, err := svgtypes.ParsePaint(s); err == nil && p.Kind == svgtypes.PaintColor {
			backgroundColor, hasBackgroundColor = p.Color, true
		}
	}

	tree := &Tree{
		size:            size,
		root:            emptyGroupValue(),
		linearGradients: nil,
		radialGradients: nil,
		patterns:        nil,
		clipPaths:       nil,
		masks:           nil,
		filters:         nil,
		fontdb:          opt.fontdb,
	}

	if !svg.isVisibleElement(opt) {
		return tree, nil
	}

	state := &converterState{
		parentClipPath: nil,
		contextElement: nil,
		parentMarkers:  nil,
		feImageLink:    false,
		viewBox:        vb.rect,
		opt:            opt,
	}

	cache := newConverterCache(opt.fontdb.Clone())
	cache.opt = opt

	for node := range svgDoc.descendants() {
		if node.elementID() != "" {
			cache.allIDs[stringHash(node.elementID())] = struct{}{}
		}
	}

	// The root transform wraps the viewBox mapping, so its origin is resolved
	// against the viewport rather than the viewBox.
	rootState := state.clone()
	rootState.viewBox = size.ToNonZeroRect(0.0, 0.0)
	svgTs := svg.resolveTransform(AIdTransform, rootState)
	rootTs := svgTs.PreConcat(vb.toTransform(tree.Size()))
	if rootTs.IsIdentity() && !hasBackgroundColor {
		convertChildren(svgDoc.root(), state, cache, &tree.root)
	} else {
		g := emptyGroup()

		if hasBackgroundColor {
			if p, ok := backgroundPath(backgroundColor, vb.rect.ToRect()); ok {
				g.children = append(g.children, p)
			}
		}

		g.transform = rootTs
		g.absTransform = rootTs
		convertChildren(svgDoc.root(), state, cache, g)
		tree.root.children = append(tree.root.children, g)
	}

	// Clear cache to make sure that all `Arc<T>` objects have a single strong reference.
	clear(cache.clipPaths)
	clear(cache.masks)
	clear(cache.filters)
	clear(cache.paint)

	cache.paintRefs = countPaintRefs(&tree.root)
	updatePaintServers(
		&tree.root,
		path.DefaultTransform(),
		nil,
		nil,
		cache,
	)
	cache.paintRefs = nil
	tree.collectPaintServers()
	tree.root.collectClipPaths(&tree.clipPaths)
	tree.root.collectMasks(&tree.masks)
	tree.root.collectFilters(&tree.filters)

	// The fontdb might have been mutated and we want to apply these changes to
	// the tree's fontdb.
	tree.fontdb = cache.fontdb

	if restoreViewbox {
		calculateSvgBBox(tree)
	}

	return tree, nil
}

// backgroundPath is background_path in usvg src/parser/converter.rs.
func backgroundPath(backgroundColor svgtypes.Color, area Rect) (*Path, bool) {
	p := path.PathFromRect(area)

	opacity, ok := path.NewNormalizedF32(float32(backgroundColor.Alpha) / 255.0)
	if !ok {
		return nil, false
	}
	fill := DefaultFill()
	fill.paint = NewColorRGB(
		backgroundColor.Red,
		backgroundColor.Green,
		backgroundColor.Blue,
	)
	fill.opacity = opacity

	res := newSimplePath(p)
	res.fill = &fill

	return res, true
}

// resolveSvgSize is resolve_svg_size in usvg src/parser/converter.rs.
//
// Port: upstream returns (Result<Size, Error>, bool). Go returns the size,
// the error and the bool.
func resolveSvgSize(svg svgNode, opt *options) (Size, error, bool) { //nolint:revive // the order of the Rust tuple
	state := &converterState{
		parentClipPath: nil,
		contextElement: nil,
		parentMarkers:  nil,
		feImageLink:    false,
		viewBox:        mustNonZeroRect(path.NonZeroRectFromXYWH(0.0, 0.0, 100.0, 100.0)),
		opt:            opt,
	}

	def := svgtypes.NewLength(100.0, svgtypes.LengthUnitPercent)
	width, ok := svg.attributeLength(AIdWidth)
	if !ok {
		width = def
	}
	height, ok := svg.attributeLength(AIdHeight)
	if !ok {
		height = def
	}

	vbox, hasViewBox := svg.parseViewbox()

	restoreViewbox := false
	if (width.Unit == svgtypes.LengthUnitPercent || height.Unit == svgtypes.LengthUnitPercent) && !hasViewBox {
		// Apply the percentages to the fallback size.
		if width.Unit == svgtypes.LengthUnitPercent {
			width = svgtypes.NewLength(
				(width.Number/100.0)*float64(state.opt.defaultSize.Width()),
				svgtypes.LengthUnitNone,
			)
		}

		if height.Unit == svgtypes.LengthUnitPercent {
			height = svgtypes.NewLength(
				(height.Number/100.0)*float64(state.opt.defaultSize.Height()),
				svgtypes.LengthUnitNone,
			)
		}

		restoreViewbox = true
	}

	var size Size
	if hasViewBox {
		state.viewBox = vbox

		var w, h float32
		if width.Unit == svgtypes.LengthUnitPercent {
			w = vbox.Width() * (float32(width.Number) / 100.0)
		} else {
			w = svg.convertUserLength(AIdWidth, state, def)
		}

		if height.Unit == svgtypes.LengthUnitPercent {
			h = vbox.Height() * (float32(height.Number) / 100.0)
		} else {
			h = svg.convertUserLength(AIdHeight, state, def)
		}

		// If exactly one of width and height is specified, compute the missing
		// dimension from the specified one and the viewBox aspect ratio.
		_, hasW := svg.attributeLength(AIdWidth)
		_, hasH := svg.attributeLength(AIdHeight)
		switch {
		case hasW && !hasH:
			size, ok = path.SizeFromWH(w, float32(vbox.Height()*w)/vbox.Width())
		case !hasW && hasH:
			size, ok = path.SizeFromWH(float32(vbox.Width()*h)/vbox.Height(), h)
		default:
			size, ok = path.SizeFromWH(w, h)
		}
	} else {
		size, ok = path.SizeFromWH(
			svg.convertUserLength(AIdWidth, state, def),
			svg.convertUserLength(AIdHeight, state, def),
		)
	}

	if !ok {
		return Size{}, okage.ErrSVGInvalidSize, restoreViewbox
	}
	return size, nil, restoreViewbox
}

// calculateSvgBBox is calculate_svg_bbox in usvg src/parser/converter.rs. It
// calculates SVG's size and viewBox in case there were not set.
//
// Simply iterates over all nodes and calculates a bounding box.
func calculateSvgBBox(tree *Tree) {
	if bbox, ok := tree.root.AbsBoundingBox(); ok {
		if size, ok := path.SizeFromWH(bbox.Right(), bbox.Bottom()); ok {
			tree.size = size
		}
	}
}

// convertChildren is convert_children in usvg src/parser/converter.rs.
//
// Port: the Go name has no suffix, and convert_children of
// src/parser/use_node.rs takes its module as a suffix (D10).
func convertChildren(
	parentNode svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	for node := range parentNode.children() {
		convertElement(node, state, cache, parent)
	}
}

// convertElement is convert_element in usvg src/parser/converter.rs.
func convertElement(node svgNode, state *converterState, cache *converterCache, parent *Group) {
	tagName, ok := node.tagName()
	if !ok {
		return
	}

	if !tagName.IsGraphic() && tagName != EIdG && tagName != EIdSwitch && tagName != EIdSvg {
		return
	}

	if !node.isVisibleElement(state.opt) {
		return
	}

	if tagName == EIdUse {
		convertUseNode(node, state, cache, parent)
		return
	}

	if tagName == EIdSwitch {
		convertSwitch(node, state, cache, parent)
		return
	}

	// A nested `svg` is handled just like `use`: it must not be wrapped in an
	// additional `convert_group`, otherwise its `transform` (and other group
	// properties) would be applied twice, since `convert_svg`
	// already creates its own group. The outermost `svg` (no parent element)
	// is intentionally not handled here - it's processed directly in
	// `convert_doc`.
	if _, hasParent := node.parentElement(); tagName == EIdSvg && hasParent {
		convertSvg(node, state, cache, parent)

		return
	}

	if g, ok := convertGroup(node, state, false, cache, parent, func(cache *converterCache, g *Group) {
		convertElementImpl(tagName, node, state, cache, g)
	}); ok {
		parent.children = append(parent.children, g)
	}
}

// convertElementImpl is convert_element_impl in usvg
// src/parser/converter.rs.
func convertElementImpl(
	tagName EId,
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	switch tagName {
	case EIdRect,
		EIdCircle,
		EIdEllipse,
		EIdLine,
		EIdPolyline,
		EIdPolygon,
		EIdPath:
		if p, ok := convertShapes(node, state); ok {
			convertPath(node, p, state, cache, parent)
		}
	case EIdImage:
		convertImage(node, state, cache, parent)
	case EIdText:
		convertTextNode(node, state, cache, parent)
	case EIdSvg:
		// Only the outermost `svg` reaches this point; nested `svg` elements are
		// handled earlier in `convert_element`. The root `svg` itself is
		// skipped and only its children are converted.
		convertChildren(node, state, cache, parent)
	case EIdG:
		convertChildren(node, state, cache, parent)
	}
}

// convertClipPathElements is convert_clip_path_elements in usvg
// src/parser/converter.rs.
//
// `clipPath` can have only shape and `text` children.
//
// `line` doesn't impact rendering because stroke is always disabled
// for `clipPath` children.
func convertClipPathElements(
	clipNode svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	for node := range clipNode.children() {
		tagName, ok := node.tagName()
		if !ok {
			continue
		}

		if !tagName.IsGraphic() {
			continue
		}

		if !node.isVisibleElement(state.opt) {
			continue
		}

		if tagName == EIdUse {
			convertUseNode(node, state, cache, parent)
			continue
		}

		if g, ok := convertGroup(node, state, false, cache, parent, func(cache *converterCache, g *Group) {
			convertClipPathElementsImpl(tagName, node, state, cache, g)
		}); ok {
			parent.children = append(parent.children, g)
		}
	}
}

// convertClipPathElementsImpl is convert_clip_path_elements_impl in usvg
// src/parser/converter.rs.
func convertClipPathElementsImpl(
	tagName EId,
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	switch tagName {
	case EIdRect, EIdCircle, EIdEllipse, EIdPolyline, EIdPolygon, EIdPath:
		if p, ok := convertShapes(node, state); ok {
			convertPath(node, p, state, cache, parent)
		}
	case EIdText:
		convertTextNode(node, state, cache, parent)
	default:
		state.opt.warnf("'%s' is no a valid 'clip-path' child.", tagName)
	}
}

// isolation is Isolation in usvg src/parser/converter.rs.
//
// The zero value is isolationAuto, the Default of upstream.
type isolation uint8

// The values of isolation.
const (
	isolationAuto isolation = iota
	isolationIsolate
)

// fromValueIsolation is the impl FromValue for Isolation in usvg
// src/parser/converter.rs.
func fromValueIsolation(_ svgNode, _ AId, value string) (isolation, bool) {
	switch value {
	case "auto": //nolint:goconst // an SVG keyword, as upstream writes it
		return isolationAuto, true
	case "isolate":
		return isolationIsolate, true
	}
	return 0, false
}

// convertGroup is convert_group in usvg src/parser/converter.rs.
//
// TODO: explain
//
// Port: collectChildren is the closure of upstream.
func convertGroup(
	node svgNode,
	state *converterState,
	force bool,
	cache *converterCache,
	parent *Group,
	collectChildren func(cache *converterCache, g *Group),
) (*Group, bool) {
	// A `clipPath` child cannot have an opacity.
	opacity := path.NormalizedF32One
	if state.parentClipPath == nil {
		if o, ok := node.attributeOpacity(AIdOpacity); ok {
			opacity = o
		}
	}

	_, hasParent := node.parentElement()
	isRootSvg := node.hasTagName(EIdSvg) && !hasParent
	var transform Transform
	if isRootSvg {
		// Already applied outside the viewBox mapping in `convert_doc`.
		transform = path.DefaultTransform()
	} else {
		transform = node.resolveTransform(AIdTransform, state)
	}
	blendMode, _ := nodeAttribute(node, AIdMixBlendMode, fromValueBlendMode)
	iso, _ := nodeAttribute(node, AIdIsolation, fromValueIsolation)
	isolate := iso == isolationIsolate

	// Nodes generated by markers must not have an ID. Otherwise we would have duplicates.
	isGOrUse := node.hasTagName(EIdG) || node.hasTagName(EIdUse)
	id := ""
	if isGOrUse && len(state.parentMarkers) == 0 {
		id = node.elementID()
	}

	absTransform := parent.absTransform.PreConcat(transform)
	g := emptyGroup()
	g.id = id
	g.transform = transform
	g.absTransform = absTransform
	g.opacity = opacity
	g.blendMode = blendMode
	g.isolate = isolate
	g.clipPath = nil
	g.mask = nil
	g.filters = nil
	g.isContextElement = false
	collectChildren(cache, g)

	objectBBox := func() (NonZeroRect, bool) {
		r, ok := g.BoundingBox()
		if !ok {
			return NonZeroRect{}, false
		}
		return r.ToNonZeroRect()
	}

	// `mask` and `filter` cannot be set on `clipPath` children.
	// But `clip-path` can.

	var clipPath *ClipPath
	if link, ok := node.attributeNode(AIdClipPath); ok {
		clipPath, ok = convertClipPath(link, state, objectBBox, cache)
		if !ok {
			return nil, false
		}
	}

	var mask *Mask
	if state.parentClipPath == nil {
		if link, ok := node.attributeNode(AIdMask); ok {
			mask, ok = convertMask(link, state, objectBBox, cache)
			if !ok {
				return nil, false
			}
		}
	}

	var filters []*Filter
	if state.parentClipPath == nil {
		if v, ok := node.attributeStr(AIdFilter); ok && v == "none" {
			// Do nothing.
		} else if node.hasAttribute(AIdFilter) {
			var bbox *NonZeroRect
			if r, ok := objectBBox(); ok {
				bbox = &r
			}
			f, ok := convertFilter(node, state, bbox, cache)
			if !ok {
				// A filter that not a link or a filter with a link to a non existing element.
				//
				// Unlike `clip-path` and `mask`, when a `filter` link is invalid
				// then the whole element should be ignored.
				//
				// This is kinda an undefined behaviour.
				// In most cases, Chrome, Firefox and rsvg will ignore such elements,
				// but in some cases Chrome allows it. Not sure why.
				// Inkscape (0.92) simply ignores such attributes, rendering element as is.
				// Batik (1.12) crashes.
				//
				// Test file: e-filter-051.svg
				return nil, false
			}
			filters = f
		}
	}

	required := !ApproxEqUlps32(opacity.Get(), 1.0, 4) ||
		clipPath != nil ||
		mask != nil ||
		len(filters) != 0 ||
		!transform.IsIdentity() ||
		blendMode != BlendModeNormal ||
		isolate ||
		isGOrUse ||
		force

	if !required {
		parent.children = append(parent.children, g.children...)
		return nil, false
	}

	g.clipPath = clipPath
	g.mask = mask
	g.filters = filters

	return g, true
}

// convertPath is convert_path in usvg src/parser/converter.rs.
func convertPath(
	node svgNode,
	tinySkiaPath *path.Path,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	if tinySkiaPath.Len() < 2 {
		return
	}

	hasBBox := tinySkiaPath.Bounds().Width() > 0.0 && tinySkiaPath.Bounds().Height() > 0.0
	fill, hasFill := resolveFill(node, hasBBox, state, cache)
	stroke, hasStroke := resolveStroke(node, hasBBox, state, cache)
	vis, _ := nodeFindAttribute(node, AIdVisibility, fromValueVisibility)
	visible := vis == visibilityVisible
	renderingMode, ok := nodeFindAttribute(node, AIdShapeRendering, fromValueShapeRendering)
	if !ok {
		renderingMode = state.opt.shapeRendering
	}

	// TODO: handle `markers` before `stroke`
	rawPaintOrder, ok := nodeFindAttribute(node, AIdPaintOrder, fromValuePaintOrder)
	if !ok {
		rawPaintOrder = svgtypes.DefaultPaintOrder()
	}
	paintOrder := svgPaintOrderToUsvg(rawPaintOrder)
	pathTransform := parent.absTransform

	// If a path doesn't have a fill or a stroke then it's invisible.
	// By setting `visibility` to `hidden` we are disabling rendering of this path.
	if !hasFill && !hasStroke {
		visible = false
	}

	bounds := tinySkiaPath.Bounds()
	if hasFill {
		if ce := fill.contextElement; ce != nil && ce.kind == contextElementPathNode {
			var contextBBox *Rect
			if ce.bbox != nil {
				r := ce.bbox.ToRect()
				contextBBox = &r
			}
			processPaint(
				&fill.paint,
				true,
				ce.transform,
				contextBBox,
				pathTransform,
				&bounds,
				cache,
			)
			fill.contextElement = nil
		}
	}

	if hasStroke {
		if ce := stroke.contextElement; ce != nil && ce.kind == contextElementPathNode {
			var contextBBox *Rect
			if ce.bbox != nil {
				r := ce.bbox.ToRect()
				contextBBox = &r
			}
			processPaint(
				&stroke.paint,
				true,
				ce.transform,
				contextBBox,
				pathTransform,
				&bounds,
				cache,
			)
			stroke.contextElement = nil
		}
	}

	var marker *Group
	if isValid(node) && vis == visibilityVisible {
		markerGroup := emptyGroup()
		markerGroup.absTransform = parent.absTransform

		markerState := state.clone()

		var bbox *NonZeroRect
		if r, ok := tinySkiaPath.ComputeTightBounds(); ok {
			if nz, ok := r.ToNonZeroRect(); ok {
				bbox = &nz
			}
		}

		ctx := &contextFillStroke{}
		if hasFill {
			f := *fill
			f.contextElement = &contextElement{kind: contextElementPathNode, transform: pathTransform, bbox: bbox}
			ctx.fill = &f
		}
		if hasStroke {
			s := *stroke
			s.contextElement = &contextElement{kind: contextElementPathNode, transform: pathTransform, bbox: bbox}
			ctx.stroke = &s
		}

		markerState.contextElement = ctx

		convertMarker(
			node,
			tinySkiaPath,
			markerState,
			cache,
			markerGroup,
		)
		marker = markerGroup
	}

	// Nodes generated by markers must not have an ID. Otherwise we would have duplicates.
	id := ""
	if len(state.parentMarkers) == 0 {
		id = node.elementID()
	}

	if !hasFill {
		fill = nil
	}
	if !hasStroke {
		stroke = nil
	}
	p := newPath(
		id,
		visible,
		fill,
		stroke,
		paintOrder,
		renderingMode,
		tinySkiaPath,
		pathTransform,
	)

	order := rawPaintOrder.Order
	switch {
	case marker != nil && order[0] == svgtypes.PaintOrderKindMarkers:
		parent.children = append(parent.children, marker)
		parent.children = append(parent.children, p)
	case marker != nil && order[1] == svgtypes.PaintOrderKindMarkers:
		appendSinglePaintPath(order[0], p, parent)
		parent.children = append(parent.children, marker)
		appendSinglePaintPath(order[2], p, parent)
	case marker != nil && order[2] == svgtypes.PaintOrderKindMarkers:
		parent.children = append(parent.children, p)
		parent.children = append(parent.children, marker)
	default:
		parent.children = append(parent.children, p)
	}
}

// appendSinglePaintPath is append_single_paint_path in usvg
// src/parser/converter.rs.
func appendSinglePaintPath(paintOrderKind svgtypes.PaintOrderKind, p *Path, parent *Group) {
	switch paintOrderKind {
	case svgtypes.PaintOrderKindFill:
		if p.fill != nil {
			fillPath := p.clone()
			fillPath.stroke = nil
			fillPath.id = ""
			parent.children = append(parent.children, fillPath)
		}
	case svgtypes.PaintOrderKindStroke:
		if p.stroke != nil {
			strokePath := p.clone()
			strokePath.fill = nil
			strokePath.id = ""
			parent.children = append(parent.children, strokePath)
		}
	}
}

// svgPaintOrderToUsvg is svg_paint_order_to_usvg in usvg
// src/parser/converter.rs.
func svgPaintOrderToUsvg(order svgtypes.PaintOrder) PaintOrder {
	switch {
	case order.Order[0] == svgtypes.PaintOrderKindStroke:
		return PaintOrderStrokeAndFill
	case order.Order[0] == svgtypes.PaintOrderKindMarkers && order.Order[1] == svgtypes.PaintOrderKindStroke:
		return PaintOrderStrokeAndFill
	}
	return PaintOrderFillAndStroke
}

// resolveTransform is SvgNode::resolve_transform in usvg
// src/parser/converter.rs.
func (n svgNode) resolveTransform(transformAid AId, state *converterState) Transform {
	transform, ok := n.attributeTransform(transformAid)
	if !ok {
		transform = path.DefaultTransform()
	}
	transformOrigin, ok := nodeAttribute(n, AIdTransformOrigin, fromValueTransformOrigin)
	if !ok {
		// The root SVG uses the CSS initial origin; other SVG elements use (0, 0).
		if _, hasParent := n.parentElement(); n.hasTagName(EIdSvg) && !hasParent {
			transformOrigin = svgtypes.NewTransformOrigin(
				svgtypes.NewLength(50.0, svgtypes.LengthUnitPercent),
				svgtypes.NewLength(50.0, svgtypes.LengthUnitPercent),
				svgtypes.LengthZero(),
			)
			ok = true
		}
	}

	if ok {
		dx := convertLength(
			transformOrigin.XOffset,
			n,
			AIdWidth,
			unitsUserSpaceOnUse,
			state,
		)
		dy := convertLength(
			transformOrigin.YOffset,
			n,
			AIdHeight,
			unitsUserSpaceOnUse,
			state,
		)
		transform = path.DefaultTransform().
			PreTranslate(dx, dy).
			PreConcat(transform).
			PreTranslate(-dx, -dy)
	}

	return transform
}
