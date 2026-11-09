// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/image.rs. The types ImageHrefResolver,
// ImageHrefDataResolverFn and ImageHrefStringResolverFn of this file are in
// usvg_parser_options.go.
//
// Port: the crate data-url, which upstream uses to read a `data:` URL, is
// replaced by the function processDataURL at the end of this file, and the
// crate base64 by encoding/base64 (D7). The crate imagesize, which upstream
// uses to find the format of an image, is replaced by getImageDataFormat,
// which checks the same magic bytes.

// DefaultDataResolver is ImageHrefResolver::default_data_resolver in usvg
// src/parser/image.rs. It creates a default
// [Data URL](https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/Data_URIs)
// resolver closure.
//
// base64 encoded data is already decoded.
//
// The default implementation would try to load JPEG, PNG, GIF, WebP, SVG and SVGZ types.
// Note that it will simply match the `mime` or data's magic.
// The actual images would not be decoded. It's up to the renderer.
//
// Port: upstream passes the parsing options to the closure. Go takes them
// here, because ImageHrefDataResolverFn has no options (D20). They are used
// to parse a nested SVG image. A nil ResolveData of ImageHrefResolver is this
// resolver with the options of the parser.
func DefaultDataResolver(opts ...Option) ImageHrefDataResolverFn {
	opt := newOptions(opts...)
	return func(mime string, data []byte) (ImageKind, bool) {
		return defaultDataResolver(mime, data, opt)
	}
}

// defaultDataResolver is the closure of
// ImageHrefResolver::default_data_resolver in usvg src/parser/image.rs.
func defaultDataResolver(mime string, data []byte, opts *options) (ImageKind, bool) {
	switch mime {
	case "image/jpg", "image/jpeg":
		return ImageKindJPEG(data), true
	case "image/png":
		return ImageKindPNG(data), true
	case "image/gif":
		return ImageKindGIF(data), true
	case "image/webp":
		return ImageKindWEBP(data), true
	case "image/svg+xml":
		return loadSubSvg(data, opts)
	case "text/plain":
		format, _ := getImageDataFormat(data)
		switch format {
		case imageFormatJPEG:
			return ImageKindJPEG(data), true
		case imageFormatPNG:
			return ImageKindPNG(data), true
		case imageFormatGIF:
			return ImageKindGIF(data), true
		case imageFormatWEBP:
			return ImageKindWEBP(data), true
		}
		return loadSubSvg(data, opts)
	}
	return nil, false
}

// DefaultStringResolver is ImageHrefResolver::default_string_resolver in
// usvg src/parser/image.rs. It creates a default string resolver.
//
// The default implementation treats an input string as a file path and tries to open.
// If a string is an URL or something else it would be ignored.
//
// Paths have to be absolute or relative to the input SVG file or relative to
// [Options::resources_dir](crate::Options::resources_dir).
//
// Port: upstream passes the parsing options to the closure. Go takes them
// here, because ImageHrefStringResolverFn has no options (D20). A nil
// ResolveString of ImageHrefResolver is this resolver with the options of the
// parser.
func DefaultStringResolver(opts ...Option) ImageHrefStringResolverFn {
	opt := newOptions(opts...)
	return func(href string) (ImageKind, bool) {
		return defaultStringResolver(href, opt)
	}
}

// defaultStringResolver is the closure of
// ImageHrefResolver::default_string_resolver in usvg src/parser/image.rs.
//
//nolint:gosec // upstream opens and reads the file that the SVG names
func defaultStringResolver(href string, opts *options) (ImageKind, bool) {
	p := opts.getAbsPath(href)

	if _, err := os.Stat(p); err != nil {
		opts.warnf("'%s' is not a path to an image.", href)
		return nil, false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		opts.warnf("Failed to load '%s'. Skipped.", href)
		return nil, false
	}

	format, _ := getImageFileFormat(p, data)
	switch format {
	case imageFormatJPEG:
		return ImageKindJPEG(data), true
	case imageFormatPNG:
		return ImageKindPNG(data), true
	case imageFormatGIF:
		return ImageKindGIF(data), true
	case imageFormatWEBP:
		return ImageKindWEBP(data), true
	case imageFormatSVG:
		return loadSubSvg(data, opts)
	}
	opts.warnf("'%s' is not a PNG, JPEG, GIF, WebP or SVG(Z) image.", href)
	return nil, false
}

// String is the impl Debug for ImageHrefResolver in usvg
// src/parser/image.rs.
func (ImageHrefResolver) String() string {
	return "ImageHrefResolver { .. }"
}

// imageFormat is ImageFormat in usvg src/parser/image.rs.
type imageFormat uint8

// The values of imageFormat. The zero value is no format.
const (
	imageFormatNone imageFormat = iota
	imageFormatPNG
	imageFormatJPEG
	imageFormatGIF
	imageFormatWEBP
	imageFormatSVG
)

// convertImage is convert in usvg src/parser/image.rs.
//
// Port: upstream returns Option<()>. Go returns false for None.
func convertImage(
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) bool {
	href, ok := nodeTryAttribute(node, AIdHref, fromValueStr)
	href, ok = logNone(href, ok, func() {
		state.opt.warnf("Image lacks the 'xlink:href' attribute. Skipped.")
	})
	if !ok {
		return false
	}

	kind, ok := getHrefData(href, state)
	if !ok {
		return false
	}

	vis, _ := nodeFindAttribute(node, AIdVisibility, fromValueVisibility)
	visible := vis == visibilityVisible

	renderingMode, ok := nodeFindAttribute(node, AIdImageRendering, fromValueImageRendering)
	if !ok {
		renderingMode = state.opt.imageRendering
	}

	// Nodes generated by markers must not have an ID. Otherwise we would have duplicates.
	id := ""
	if len(state.parentMarkers) == 0 {
		id = node.elementID()
	}

	actualSize, ok := imageKindActualSize(kind, state.opt)
	if !ok {
		return false
	}

	x := node.convertUserLength(AIdX, state, svgtypes.LengthZero())
	y := node.convertUserLength(AIdY, state, svgtypes.LengthZero())
	width := node.convertUserLength(
		AIdWidth,
		state,
		svgtypes.NewLengthNumber(float64(actualSize.Width())),
	)
	height := node.convertUserLength(
		AIdHeight,
		state,
		svgtypes.NewLengthNumber(float64(actualSize.Height())),
	)

	_, hasWidth := node.attributeLength(AIdWidth)
	_, hasHeight := node.attributeLength(AIdHeight)
	switch {
	case hasWidth && !hasHeight:
		// Only width was defined, so we need to scale height accordingly.
		height = actualSize.Height() * (width / actualSize.Width())
	case !hasWidth && hasHeight:
		// Only height was defined, so we need to scale width accordingly.
		width = actualSize.Width() * (height / actualSize.Height())
	}

	aspect, ok := nodeAttribute(node, AIdPreserveAspectRatio, fromValueAspectRatio)
	if !ok {
		aspect = svgtypes.DefaultAspectRatio()
	}

	rect, ok := path.NonZeroRectFromXYWH(x, y, width, height)
	rect, ok = logNone(rect, ok, func() {
		state.opt.warnf("Image has an invalid size. Skipped.")
	})
	if !ok {
		return false
	}

	return convertInner(
		kind,
		id,
		visible,
		renderingMode,
		aspect,
		actualSize,
		rect,
		cache,
		parent,
	)
}

// convertInner is convert_inner in usvg src/parser/image.rs.
//
// Port: upstream returns Option<()>. Go returns false for None.
func convertInner(
	kind ImageKind,
	id string,
	visible bool,
	renderingMode ImageRendering,
	aspect svgtypes.AspectRatio,
	actualSize Size,
	rect NonZeroRect,
	cache *converterCache,
	parent *Group,
) bool {
	alignedSize := fitViewBox(actualSize, rect, aspect)
	alignedX, alignedY := alignedPos(
		aspect.Align,
		rect.X(),
		rect.Y(),
		rect.Width()-alignedSize.Width(),
		rect.Height()-alignedSize.Height(),
	)
	vb := alignedSize.ToNonZeroRect(alignedX, alignedY)

	imageTs := path.TransformFromRow(
		vb.Width()/actualSize.Width(),
		0.0,
		0.0,
		vb.Height()/actualSize.Height(),
		vb.X(),
		vb.Y(),
	)

	absTransform := parent.absTransform.PreConcat(imageTs)
	absBoundingBox, ok := vb.Transform(parent.absTransform)
	if !ok {
		return false
	}

	g := emptyGroup()
	g.id = id
	g.children = append(g.children, &Image{
		id:             "",
		visible:        visible,
		size:           actualSize,
		renderingMode:  renderingMode,
		kind:           kind,
		absTransform:   absTransform,
		absBoundingBox: absBoundingBox,
	})
	g.transform = imageTs
	g.absTransform = absTransform

	if aspect.Slice {
		// Image slice acts like a rectangular clip.
		clipShape := newSimplePath(path.PathFromRect(rect.ToRect()))
		fill := DefaultFill()
		clipShape.fill = &fill

		clip := emptyClipPath(cache.genClipPathID())
		clip.root.children = append(clip.root.children, clipShape)

		// Clip path should not be affected by the image viewbox transform.
		// The final structure should look like:
		// <g clip-path="url(#clipPath1)">
		//     <g transform="matrix(1 0 0 1 10 20)">
		//         <image/>
		//     </g>
		// </g>

		g2 := emptyGroup()
		g.id, g2.id = g2.id, g.id
		g2.absTransform = parent.absTransform
		g2.clipPath = clip
		g2.children = append(g2.children, g)

		// With `slice`, the image is scaled to *cover* the viewport and the
		// overflow is cut away by the rectangular clip above. The bounding box
		// of the element is therefore its `rect` (the viewport), not the
		// larger scaled-to-cover image that the children's bounding boxes
		// would give. Without this correction an `objectBoundingBox`
		// clip/mask/filter applied to the `<image>` would be resolved against
		// the oversized image. See https://github.com/linebender/resvg/issues/1034
		bbox := rect.ToRect()
		g2.boundingBox.set(some(bbox))
		g2.strokeBoundingBox.set(some(bbox))
		if abs, ok := bbox.Transform(g2.absTransform); ok {
			g2.absBoundingBox.set(some(abs))
			g2.absStrokeBoundingBox.set(some(abs))
		}

		parent.children = append(parent.children, g2)
	} else {
		parent.children = append(parent.children, g)
	}

	return true
}

// getHrefData is get_href_data in usvg src/parser/image.rs.
//
// Port: a nil function of the ImageHrefResolver of the options is the
// default resolver of upstream.
func getHrefData(href string, state *converterState) (ImageKind, bool) {
	if url, ok := processDataURL(href); ok {
		data, ok := url.decodeToVec()
		if !ok {
			return nil, false
		}

		mime := url.mimeType + "/" + url.mimeSubtype

		if f := state.opt.imageHrefResolver.ResolveData; f != nil {
			return f(mime, data)
		}
		return defaultDataResolver(mime, data, state.opt)
	}
	if f := state.opt.imageHrefResolver.ResolveString; f != nil {
		return f(href)
	}
	return defaultStringResolver(href, state.opt)
}

// getImageFileFormat is get_image_file_format in usvg src/parser/image.rs.
// It checks that file has a PNG, a GIF, a JPEG or a WebP magic bytes.
// Or an SVG(Z) extension.
func getImageFileFormat(p string, data []byte) (imageFormat, bool) {
	ext, ok := pathExtension(p)
	if !ok {
		return imageFormatNone, false
	}
	ext = strings.ToLower(ext)
	if ext == "svg" || ext == "svgz" {
		return imageFormatSVG, true
	}

	return getImageDataFormat(data)
}

// pathExtension is Path::extension of the Rust standard library: the part
// of the file name after its last dot. A name with no dot, or with a dot only
// at its start, has none.
func pathExtension(p string) (string, bool) {
	name := filepath.Base(p)
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return "", false
	}
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return "", false
	}
	return name[i+1:], true
}

// getImageDataFormat is get_image_data_format in usvg src/parser/image.rs.
// It checks that file has a PNG, a GIF, a JPEG or a WebP magic bytes.
//
// Port: upstream calls imagesize::image_type, which reads 12 bytes and fails
// on less. Go checks the same magic bytes (D7).
func getImageDataFormat(data []byte) (imageFormat, bool) {
	if len(data) < 12 {
		return imageFormatNone, false
	}
	switch {
	case strings.HasPrefix(string(data), "\xFF\xD8\xFF"):
		return imageFormatJPEG, true
	case strings.HasPrefix(string(data), "\x89PNG"):
		return imageFormatPNG, true
	case strings.HasPrefix(string(data), "GIF8"):
		return imageFormatGIF, true
	case string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return imageFormatWEBP, true
	}
	return imageFormatNone, false
}

// loadSubSvg is load_sub_svg in usvg src/parser/image.rs. It tries to load
// the `ImageData` content as an SVG image or emits a warning and returns
// `None`.
func loadSubSvg(data []byte, opt *options) (ImageKind, bool) {
	tree, err := fromDataNested(data, opt)
	if err != nil {
		opt.warnf("Failed to load nested SVG image.")
		return nil, false
	}
	return ImageKindSVG{Tree: tree}, true
}

// fitViewBox is fit_view_box in usvg src/parser/image.rs. It fits size into
// a viewbox.
func fitViewBox(size Size, rect NonZeroRect, aspect svgtypes.AspectRatio) Size {
	s := rect.Size()

	switch {
	case aspect.Align == svgtypes.AlignNone:
		return s
	case aspect.Slice:
		return size.ExpandTo(s)
	}
	return size.ScaleTo(s)
}

// dataURL is DataUrl of the crate data-url 0.3.2, with the type and the
// subtype of its MIME type, the only parts that usvg reads (D7).
type dataURL struct {
	mimeType                string
	mimeSubtype             string
	base64                  bool
	encodedBodyPlusFragment string
}

// processDataURL is DataUrl::process of the crate data-url 0.3.2, which
// processes a `data:` URL as the Fetch Standard says. It returns false
// where the crate returns an error.
func processDataURL(input string) (dataURL, bool) {
	afterColon, ok := pretendParseDataURL(input)
	if !ok {
		return dataURL{}, false
	}

	fromColonToComma, encodedBodyPlusFragment, ok := findCommaBeforeFragment(afterColon)
	if !ok {
		return dataURL{}, false
	}

	mimeType, mimeSubtype, isBase64 := parseDataURLHeader(fromColonToComma)

	return dataURL{
		mimeType:                mimeType,
		mimeSubtype:             mimeSubtype,
		base64:                  isBase64,
		encodedBodyPlusFragment: encodedBodyPlusFragment,
	}, true
}

// decodeToVec is DataUrl::decode_to_vec of the crate data-url 0.3.2. It
// returns the body, and false where the base64 is not valid. The fragment
// is dropped, because usvg does not read it.
func (u dataURL) decodeToVec() ([]byte, bool) {
	body := decodeDataURLWithoutBase64(u.encodedBodyPlusFragment)
	if !u.base64 {
		return body, true
	}
	return forgivingBase64Decode(body)
}

// isDataURLSkipped reports whether the URL parser ignores b: an ASCII tab
// or a new line.
func isDataURLSkipped(b byte) bool {
	return b == '\t' || b == '\n' || b == '\r'
}

// pretendParseDataURL is pretend_parse_data_url of the crate data-url 0.3.2.
func pretendParseDataURL(input string) (string, bool) {
	// Trim C0 control or space
	leftTrimmed := strings.TrimLeftFunc(input, func(ch rune) bool { return ch <= ' ' })

	// Ignore ASCII tabs or newlines like the URL parser would
	const want = "data:"
	i, n := 0, 0
	for n < len(want) {
		if i >= len(leftTrimmed) {
			return "", false
		}
		b := leftTrimmed[i]
		i++
		if isDataURLSkipped(b) {
			continue
		}
		w := want[n]
		if w == ':' {
			if b != ':' {
				return "", false
			}
		} else if b|0x20 != w {
			return "", false
		}
		n++
	}
	afterColon := leftTrimmed[i:]

	// Trim C0 control or space
	return strings.TrimRightFunc(afterColon, func(ch rune) bool { return ch <= ' ' }), true
}

// findCommaBeforeFragment is find_comma_before_fragment of the crate
// data-url 0.3.2.
func findCommaBeforeFragment(afterColon string) (string, string, bool) {
	for i := range len(afterColon) {
		switch afterColon[i] {
		case ',':
			return afterColon[:i], afterColon[i+1:], true
		case '#':
			return "", "", false
		}
	}
	return "", "", false
}

// parseDataURLHeader is parse_header of the crate data-url 0.3.2. It returns
// the type and the subtype of the MIME type, and whether the body is base64.
func parseDataURLHeader(fromColonToComma string) (string, string, bool) {
	// "Strip leading and trailing ASCII whitespace"
	//     \t, \n, and \r would have been filtered by the URL parser
	//     \f percent-encoded by the URL parser
	//     space is the only remaining ASCII whitespace
	trimmed := strings.Trim(fromColonToComma, " \t\n\r")

	withoutBase64Suffix, isBase64 := removeBase64Suffix(trimmed)
	mimeType := trimmed
	if isBase64 {
		mimeType = withoutBase64Suffix
	}

	var b strings.Builder
	if strings.HasPrefix(mimeType, ";") {
		b.WriteString("text/plain")
	}
	inQuery := false
	for i := range len(mimeType) {
		c := mimeType[i]
		switch {
		// Ignore ASCII tabs or newlines like the URL parser would
		case isDataURLSkipped(c):
			continue
		// https://url.spec.whatwg.org/#c0-control-percent-encode-set
		case c <= 0x1F || c >= 0x7F:
			percentEncode(c, &b)
		// Bytes other than the C0 percent-encode set that are percent-encoded
		// by the URL parser in the query state.
		// '#' is also in that list but cannot occur here
		// since it indicates the start of the URL’s fragment.
		case (c == ' ' || c == '"' || c == '<' || c == '>') && inQuery:
			percentEncode(c, &b)
		case c == '?':
			inQuery = true
			b.WriteByte('?')
		// Printable ASCII
		default:
			b.WriteByte(c)
		}
	}

	typ, subtype, ok := parseMime(b.String())
	if !ok {
		return "text", "plain", isBase64 //nolint:goconst // an SVG keyword, as upstream writes it
	}
	return typ, subtype, isBase64
}

// removeBase64Suffix is remove_base64_suffix of the crate data-url 0.3.2.
func removeBase64Suffix(s string) (string, bool) {
	// Ignore ASCII tabs or newlines like the URL parser would
	// Search from the end
	i := len(s)
	next := func() (byte, bool) {
		for i > 0 {
			i--
			if !isDataURLSkipped(s[i]) {
				return s[i], true
			}
		}
		return 0, false
	}
	for _, w := range []byte("46esab") {
		b, ok := next()
		if !ok {
			return "", false
		}
		if w >= 'a' {
			b |= 0x20
		}
		if b != w {
			return "", false
		}
	}
	for {
		b, ok := next()
		if !ok {
			return "", false
		}
		if b == ' ' {
			continue
		}
		if b != ';' {
			return "", false
		}
		break
	}
	return s[:i], true
}

// percentEncode is percent_encode of the crate data-url 0.3.2.
func percentEncode(c byte, b *strings.Builder) {
	const hexUpper = "0123456789ABCDEF"
	b.WriteByte('%')
	b.WriteByte(hexUpper[c>>4])
	b.WriteByte(hexUpper[c&0x0f])
}

// decodeDataURLWithoutBase64 is decode_without_base64 of the crate data-url
// 0.3.2. It decodes the percent escapes and drops the fragment.
func decodeDataURLWithoutBase64(encodedBodyPlusFragment string) []byte {
	s := encodedBodyPlusFragment
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '%':
			if i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
				// '%' followed by two ASCII hex digits
				out = append(out, hexValue(s[i+1])*0x10+hexValue(s[i+2]))
				i += 2
			} else {
				// The % sign will be part of the next slice.
				out = append(out, c)
			}
		case c == '#':
			return out
		// Ignore over '\t' | '\n' | '\r'
		case isDataURLSkipped(c):
		default:
			out = append(out, c)
		}
	}
	return out
}

// isHexDigit reports whether c is an ASCII hex digit.
func isHexDigit(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// hexValue returns the value of the ASCII hex digit c.
func hexValue(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	}
	return c - 'a' + 10
}

// forgivingBase64Decode is forgiving_base64::decode_to_vec of the crate
// data-url 0.3.2, the forgiving-base64 decode of the Infra Standard, with
// encoding/base64 in place of its decoder (D7). It returns false where the
// crate returns InvalidBase64.
func forgivingBase64Decode(input []byte) ([]byte, bool) {
	alphabet := make([]byte, 0, len(input))
	padding := 0
	for _, c := range input {
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '+', c == '/':
			if padding > 0 {
				// alphabet symbol present after padding
				return nil, false
			}
			alphabet = append(alphabet, c)
		// Remove ASCII whitespace
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\x0C':
		case c == '=':
			padding++
		default:
			// symbol not expected
			return nil, false
		}
	}
	switch rem := len(alphabet) % 4; {
	case rem == 0 && padding == 0:
	case rem == 2 && (padding == 2 || padding == 0):
	case rem == 3 && (padding == 1 || padding == 0):
	default:
		// lone alphabet symbol present, or incorrect padding
		return nil, false
	}
	out := make([]byte, base64.RawStdEncoding.DecodedLen(len(alphabet)))
	n, err := base64.RawStdEncoding.Decode(out, alphabet)
	if err != nil {
		return nil, false
	}
	return out[:n], true
}

// parseMime is the parse of the module mime of the crate data-url 0.3.2,
// for the type and the subtype. The parameters never make it fail, and usvg
// does not read them.
func parseMime(s string) (string, string, bool) {
	trimmed := strings.Trim(s, " \t\n\r")

	typ, rest, hasRest := strings.Cut(trimmed, "/")
	if !onlyHTTPTokenCodePoints(typ) || typ == "" {
		return "", "", false
	}
	if !hasRest {
		return "", "", false
	}

	subtype, _, _ := strings.Cut(rest, ";")
	subtype = strings.TrimRight(subtype, " \t\n\r")
	if !onlyHTTPTokenCodePoints(subtype) || subtype == "" {
		return "", "", false
	}

	return asciiLower(typ), asciiLower(subtype), true
}

// onlyHTTPTokenCodePoints is only_http_token_code_points of the crate
// data-url 0.3.2.
func onlyHTTPTokenCodePoints(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// asciiLower is str::to_ascii_lowercase of Rust.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 0x20
		}
	}
	return string(b)
}
