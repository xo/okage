// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"log/slog"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/siphasher"
)

// The limits of krilla src/serialize.rs.
const (
	strLen  = 32767
	nameLen = 127

	// These only apply to PDF 1.4 and PDF/A-1.
	maxFloat = 32767.0
	dictLen  = 4095
	arrayLen = 8191
)

// serializeSettings is SerializeSettings in krilla src/serialize.rs. The
// public fields of upstream are the options of NewDocument (D20). The port
// keeps enable_tagging without an option, because pdf has no tags (D62).
// The field render_svg_glyph_fn is the option WithSVGGlyphs.
//
// Settings that should be applied when creating a PDF document.
type serializeSettings struct {
	// pretty writes the PDF in a way that is easier to inspect manually.
	// This will result in larger file sizes.
	pretty bool
	// compressContentStreams compresses the content streams. It leads to
	// significantly smaller file sizes, but also longer running times.
	compressContentStreams bool
	// noDeviceCS uses device-independent colors instead of device-dependent
	// ones.
	noDeviceCS bool
	// asciiCompatible writes a PDF that only consists of characters in the
	// ASCII range, on a best-effort basis.
	asciiCompatible bool
	// cmykProfile is the ICC profile for CMYK colors when noDeviceCS is
	// true. Nil is None.
	cmykProfile *ICCProfile
	// xmpMetadata says whether the PDF should include XMP metadata. A
	// validator can turn it on.
	xmpMetadata bool
	// configuration holds the validators and the PDF version.
	// NewDocument builds it from builder after the last option.
	configuration configuration
	builder       configurationBuilder
	// enableTagging is the field enable_tagging of upstream, which is true by
	// default. The tags are not in the base port, so no option changes it.
	// It still decides the key /Tabs of a page with annotations, as in
	// upstream.
	enableTagging bool
	// renderSVGGlyphFn is the field render_svg_glyph_fn of upstream, which
	// WithSVGGlyphs sets. Nil is the default function of upstream, which
	// draws no SVG glyph.
	renderSVGGlyphFn SVGGlyphFunc

	// subsetFallback draws the glyphs of a font as paths when its subset
	// fails, and keeps its text as invisible text. It is not in krilla, and
	// WithSubsetFallback sets it (D80).
	subsetFallback bool
	// logger is the logger of WithLogger, or nil. It is not in krilla (D20).
	logger *slog.Logger

	// err is the error of an option, such as a version that is not known.
	// Document.Finish returns it.
	err error
}

// pdfVersion is SerializeSettings::pdf_version in krilla src/serialize.rs.
func (s *serializeSettings) pdfVersion() PDFVersion {
	return s.configuration.version
}

// validators is SerializeSettings::validators in krilla src/serialize.rs.
func (s *serializeSettings) validators() validators {
	return s.configuration.validators
}

// supportsAssociatedFiles is SerializeSettings::supports_associated_files
// in krilla src/serialize.rs.
//
// Whether the /AF key is supported, accounting for the PDF version and
// active standards.
func (s *serializeSettings) supportsAssociatedFiles() bool {
	return s.configuration.version.specifiesAssociatedFiles() ||
		s.configuration.validators.specifiesAssociatedFiles()
}

// defaultSerializeSettings is the Default of SerializeSettings in krilla
// src/serialize.rs.
func defaultSerializeSettings() serializeSettings {
	return serializeSettings{
		pretty:                 false,
		asciiCompatible:        false,
		compressContentStreams: true,
		noDeviceCS:             false,
		xmpMetadata:            true,
		configuration:          defaultConfiguration(),
		enableTagging:          true,
		subsetFallback:         true,
	}
}

// Option is an option of NewDocument. Each option sets one field of the
// struct SerializeSettings of krilla src/serialize.rs (D20).
type Option func(*serializeSettings)

// WithPretty is the field pretty of SerializeSettings in krilla
// src/serialize.rs. The default is false.
//
// Whether to write PDFs in a way that is easier to inspect manually. This
// will result in larger file sizes.
func WithPretty(pretty bool) Option {
	return func(s *serializeSettings) {
		s.pretty = pretty
	}
}

// WithCompression is the field compress_content_streams of SerializeSettings
// in krilla src/serialize.rs. The default is true.
//
// Whether content streams should be compressed. Leads to significantly
// smaller file sizes, but also longer running times. It is highly
// recommended that you set this to true.
func WithCompression(compress bool) Option {
	return func(s *serializeSettings) {
		s.compressContentStreams = compress
	}
}

// WithNoDeviceCS is the field no_device_cs of SerializeSettings in krilla
// src/serialize.rs. The default is false.
//
// Whether device-independent colors should be used instead of
// device-dependent ones.
func WithNoDeviceCS(noDeviceCS bool) Option {
	return func(s *serializeSettings) {
		s.noDeviceCS = noDeviceCS
	}
}

// WithASCIICompatible is the field ascii_compatible of SerializeSettings in
// krilla src/serialize.rs. The default is false.
//
// Whether the PDF should be ASCII-compatible, i.e. only consist of
// characters in the ASCII range. This is only on a best-effort basis.
func WithASCIICompatible(asciiCompatible bool) Option {
	return func(s *serializeSettings) {
		s.asciiCompatible = asciiCompatible
	}
}

// WithCMYKProfile is the field cmyk_profile of SerializeSettings in krilla
// src/serialize.rs. The default is nil, which is None. The type
// ICCProfile<4> of upstream takes only a profile with four components. The
// option takes nil in place of a profile with another number of components.
//
// The ICC profile that should be used for CMYK colors when no_device_cs is
// enabled.
//
// This is usually not required, but it is for example required when
// exporting to PDF/A and using a CMYK color, since they have to be
// device-independent.
func WithCMYKProfile(profile *ICCProfile) Option {
	return func(s *serializeSettings) {
		if profile != nil && profile.components != 4 {
			profile = nil
		}
		s.cmykProfile = profile
	}
}

// WithXMPMetadata is the field xmp_metadata of SerializeSettings in krilla
// src/serialize.rs. The default is true.
//
// Whether the PDF should include XMP metadata.
//
// Note that this value might be overridden depending on which validator
// you use. For example, when exporting to PDF/A, this value will be set to
// true, regardless of what value will be passed.
func WithXMPMetadata(xmpMetadata bool) Option {
	return func(s *serializeSettings) {
		s.xmpMetadata = xmpMetadata
	}
}

// WithPDFVersion is the version of the field configuration of
// SerializeSettings in krilla src/serialize.rs, as
// ConfigurationBuilder::with_version sets it. NewDocument builds the
// configuration with ConfigurationBuilder::finish after the last option.
// The default is PDF17, or the highest version of the validator of
// WithArchivalValidator. A version that is not one of the constants of
// PDFVersion, or that the validator does not allow, makes Document.Finish
// return ErrPDFVersion.
func WithPDFVersion(version PDFVersion) Option {
	return func(s *serializeSettings) {
		s.builder = s.builder.withVersion(version)
	}
}

// WithArchivalValidator is the validator of PDF/A of the field
// configuration of SerializeSettings in krilla src/serialize.rs, as
// ConfigurationBuilder::with_archival_validator sets it (D99). The default
// is no validator.
//
// With a validator, the document is written in a way that conforms to the
// standard, for example with its metadata, and Document.Finish returns an
// error that wraps ErrPDFValidation for each input that the standard does
// not allow. Some requirements are semantic in nature and cannot be
// validated. The caller must make sure that they are fulfilled.
func WithArchivalValidator(archival Archival) Option {
	return func(s *serializeSettings) {
		s.builder = s.builder.withArchivalValidator(archival)
	}
}

// WithSubsetFallback is not in krilla. The default is true (D80).
//
// When the subset of a font fails in Document.Finish and fallback is true,
// the document draws each glyph of that font as a path, in the place of
// the text, with the fill and the stroke of the text. The text stays in
// the file as invisible text, with the font without its font program, so
// that a reader can copy and search it. Finish then returns no error for
// the font, and logs the font and the error of the subset to the logger of
// WithLogger. When fallback is false, Finish returns the error, as krilla
// does.
func WithSubsetFallback(fallback bool) Option {
	return func(s *serializeSettings) {
		s.subsetFallback = fallback
	}
}

// WithLogger is not in krilla. The document logs to the logger, and
// without it, the document logs nothing (D20). The default is nil.
func WithLogger(logger *slog.Logger) Option {
	return func(s *serializeSettings) {
		s.logger = logger
	}
}

// pageInfo is the case PageInfo::Krilla of PageInfo in krilla
// src/serialize.rs. The case PageInfo::Pdf is for the pages of an embedded
// PDF file, which the base port leaves out (D62).
type pageInfo struct {
	// ref is the reference of the page in the chunk.
	ref pdfwriter.Ref
	// surfaceHeight is the page height, which converts from PDF coordinates
	// to krilla coordinates.
	surfaceHeight float32
	// annotations holds the references of the annotations of the page. It
	// is empty until the page is serialized. The tag of each annotation of
	// upstream is not in the base port.
	annotations []pdfwriter.Ref
	// pageLabel is the page label of the page.
	pageLabel PageLabel
}

// validationEntry is an item of the field validation_errors of
// SerializeContext in krilla src/serialize.rs: an error, and the validators
// that raised it.
type validationEntry struct {
	err        validationError
	validators validators
}

// pageEntry is an item of the field pages of GlobalObjects in krilla
// src/serialize.rs.
type pageEntry struct {
	ref  pdfwriter.Ref
	page *internalPage
}

// globalObjects is GlobalObjects in krilla src/serialize.rs. The wrapper
// MaybeTaken of upstream makes sure that the serializer takes each field
// once. The port sets a field to nil when it takes it. The forms, the tags
// and the embedded PDF files are not in pdf (D62).
type globalObjects struct {
	// namedDestinations holds all named destinations that have been
	// registered, with the reference of their destination and the
	// destination itself.
	namedDestinations map[string]namedDestinationEntry
	// xyzDestinations holds all XYZ destinations used in the document. They
	// can only be written in the very end, once all pages have been written,
	// so that the reference of the page they belong to is known.
	xyzDestinations []xyzDestinationEntry
	// pages holds all pages and their corresponding chunks. Pages need to
	// be written in the very end, because pages might contain annotations
	// which in turn depend on future pages.
	pages []pageEntry
	// outline stores the document outline. nil is None.
	outline *Outline
	// fontMap is the map from fonts to font containers, in the order of
	// their first use, as the IndexMap of upstream keeps them. fontIndex
	// finds the container of a font by its hash.
	fontMap   []*fontContainer
	fontIndex map[siphasher.Hash128]*fontContainer
	// embeddedFiles holds the association of the names of embedded files to
	// their references, for the catalog dictionary. Upstream keeps them in
	// a BTreeMap, so the port writes them in the order of their names.
	embeddedFiles map[string]pdfwriter.Ref
}

// serializeContext is SerializeContext in krilla src/serialize.rs. It is
// passed around throughout the whole conversion process, and contains all
// mutable state that is needed when writing a PDF file.
type serializeContext struct {
	// pageTreeRef is the reference of the page tree.
	pageTreeRef pdfwriter.Ref
	// globalObjects holds the global objects, such as the pages, that are
	// populated over time.
	globalObjects globalObjects
	// pageInfos holds information for each page written so far, indexed by
	// the page index.
	pageInfos []pageInfo
	// cachedMappings maps the hashes of objects to their references, so
	// that the same image is not embedded twice in the document.
	cachedMappings map[siphasher.Hash128]pdfwriter.Ref
	// curRef is the current reference in use. All serializers use newRef
	// to generate a new reference.
	curRef pdfwriter.Ref
	// validationErrors holds all validation errors that are collected as
	// part of the export process alongside the validators that raised the
	// error.
	validationErrors []validationEntry
	// settings holds the settings used for serialization.
	settings *serializeSettings
	// chunkSettings holds the settings used for all PDF object chunks.
	chunkSettings pdfwriter.Settings
	// limits holds the limits created as part of the serialization process.
	limits pdfwriter.Limits
	// location is the current location. Zero is None.
	location Location
	// err is the first error of a call of a surface, for a panic of krilla
	// that a caller can cause, such as a pop without a push. Document.Finish
	// returns it.
	err error
	// textStreams holds each content stream that names a font, and
	// failedFonts holds each CID font whose subset failed, by the reference
	// of its font object. They are not in krilla (D80).
	textStreams []textStream
	failedFonts map[pdfwriter.Ref]*failedFont
}

// newSerializeContext is SerializeContext::new in krilla src/serialize.rs.
// The two references of the namespaces of PDF 2.0 of upstream are for the
// tags, which the base port leaves out. The references of upstream are
// renumbered in the order of the file, so the numbers of the file do not
// change.
func newSerializeContext(settings serializeSettings) *serializeContext {
	// Override flags as required by the validator.
	settings.noDeviceCS = settings.noDeviceCS || settings.validators().requiresNoDeviceCS()
	settings.enableTagging = settings.enableTagging || settings.validators().requiresTagging()
	settings.xmpMetadata = settings.xmpMetadata || settings.validators().requiresXMPMetadata()

	curRef := pdfwriter.NewRef(1)
	pageTreeRef := curRef.Bump()

	return &serializeContext{
		cachedMappings: map[siphasher.Hash128]pdfwriter.Ref{},
		curRef:         curRef,
		pageTreeRef:    pageTreeRef,
		settings:       &settings,
		chunkSettings:  pdfwriter.Settings{Pretty: settings.pretty},
	}
}

// setLocation is SerializeContext::set_location in krilla src/serialize.rs.
func (sc *serializeContext) setLocation(location Location) {
	sc.location = location
}

// resetLocation is SerializeContext::reset_location in krilla
// src/serialize.rs.
func (sc *serializeContext) resetLocation() {
	sc.location = 0
}

// setError records the first error of a call of a surface. Document.Finish
// returns it.
func (sc *serializeContext) setError(err error) {
	if sc.err == nil {
		sc.err = err
	}
}

// newRef is SerializeContext::new_ref in krilla src/serialize.rs.
func (sc *serializeContext) newRef() pdfwriter.Ref {
	return sc.curRef.Bump()
}

// serializeSettings is SerializeContext::serialize_settings in krilla
// src/serialize.rs.
func (sc *serializeContext) serializeSettings() *serializeSettings {
	return sc.settings
}

// newChunk is SerializeContext::new_chunk in krilla src/serialize.rs. Never
// call pdfwriter.NewChunk, NewPdf or NewContent directly. Always use the
// methods of serializeContext, so that the settings apply to each chunk.
func (sc *serializeContext) newChunk() *pdfwriter.Chunk {
	return pdfwriter.NewChunkWithSettings(sc.chunkSettings)
}

// newContent is SerializeContext::new_content in krilla src/serialize.rs.
func (sc *serializeContext) newContent() *pdfwriter.Content {
	return pdfwriter.NewContentWithSettings(sc.chunkSettings)
}

// newPDFWithCapacity is SerializeContext::new_pdf_with_capacity in krilla
// src/serialize.rs.
func (sc *serializeContext) newPDFWithCapacity(capacity int) *pdfwriter.Pdf {
	return pdfwriter.NewPdfWithSettingsAndCapacity(sc.chunkSettings, capacity)
}

// finish is SerializeContext::finish in krilla src/serialize.rs. The objects
// are serialized in the right order: an object that one step takes cannot
// be read by a later step. The fonts come after the page tree, and the
// destinations after the fonts, as upstream does. The forms, the tags and
// the embedded PDF files are not in pdf (D62).
func (sc *serializeContext) finish(cc *chunkContainer) (*pdfwriter.Pdf, error) {
	// Serialize all objects that can only be written in the end.
	sc.serializeDestinationProfiles(cc)
	sc.serializePageLabelTree(cc)
	sc.serializeOutline(cc)
	if err := sc.serializePages(cc); err != nil {
		return nil, err
	}
	sc.serializePageTree(cc)
	if err := sc.serializeFonts(cc); err != nil {
		return nil, err
	}
	// okage: draw the text of each font whose subset failed as paths
	// (D80).
	sc.outlineFailedFonts()
	if err := sc.serializeXYZDestinations(cc); err != nil {
		return nil, err
	}
	sc.serializeTagTree()
	sc.serializeXMPSchema(cc)

	// Create the final PDF.
	pdf, err := cc.finish(sc)
	if err != nil {
		return nil, err
	}
	sc.registerLimits(pdf.Limits())

	sc.checkValidatorLimits()

	if len(sc.validationErrors) != 0 {
		// Deduplicate errors, while still preserving order.
		var errors []validationEntry
		seen := map[validationEntry]bool{}
		for _, e := range sc.validationErrors {
			if !seen[e] {
				seen[e] = true
				errors = append(errors, e)
			}
		}
		return nil, validationFailed(errors)
	}

	if limitErr, ok := sc.checkVersionLimits(); ok {
		return nil, limitErr.err()
	}

	return pdf, nil
}

// registerValidationError is SerializeContext::register_validation_error in
// krilla src/serialize.rs. It keeps an error that a validator prohibits.
func (sc *serializeContext) registerValidationError(err validationError) {
	if v, ok := sc.serializeSettings().validators().prohibits(err); ok {
		sc.validationErrors = append(sc.validationErrors, validationEntry{err: err, validators: v})
	}
}

// registerLimits is SerializeContext::register_limits in krilla
// src/serialize.rs.
func (sc *serializeContext) registerLimits(limits pdfwriter.Limits) {
	sc.limits.Merge(limits)
}

// registerPage is SerializeContext::register_page in krilla
// src/serialize.rs.
func (sc *serializeContext) registerPage(page *internalPage) {
	ref := sc.newRef()
	sc.pageInfos = append(sc.pageInfos, pageInfo{
		ref:           ref,
		surfaceHeight: page.pageSettings.surfaceHeight(),
		pageLabel:     page.pageSettings.pageLabel,
	})
	sc.globalObjects.pages = append(sc.globalObjects.pages, pageEntry{ref: ref, page: page})
}

// cacheable is the trait Cacheable in krilla src/serialize.rs. An object
// that the serializer writes once for each value, such as a graphics state
// or a shading, satisfies it.
type cacheable interface {
	sipHashable
	serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref)
}

// registerCached is SerializeContext::register_cached in krilla
// src/serialize.rs. It calls fn with a new reference for an item that it
// has not seen, and returns the reference of the item.
func (sc *serializeContext) registerCached(item sipHashable, fn func(sc *serializeContext, rootRef pdfwriter.Ref)) pdfwriter.Ref {
	hash := sipHash(item)
	if ref, ok := sc.cachedMappings[hash]; ok {
		return ref
	}
	rootRef := sc.newRef()
	fn(sc, rootRef)
	sc.cachedMappings[hash] = rootRef
	return rootRef
}

// registerCacheable is SerializeContext::register_cacheable in krilla
// src/serialize.rs.
func (sc *serializeContext) registerCacheable(cc *chunkContainer, object cacheable) pdfwriter.Ref {
	return sc.registerCached(object, func(sc *serializeContext, rootRef pdfwriter.Ref) {
		object.serialize(sc, cc, rootRef)
	})
}

// registerResourceable is SerializeContext::register_resourceable in krilla
// src/serialize.rs.
func registerResourceable[R resource](sc *serializeContext, cc *chunkContainer, object resourceable[R]) R {
	return object.newResource(sc.registerCacheable(cc, object))
}

// registerPageLabel is SerializeContext::register_page_label in krilla
// src/serialize.rs.
func (sc *serializeContext) registerPageLabel(cc *chunkContainer, label PageLabel) pdfwriter.Ref {
	ref := sc.newRef()
	label.serialize(cc, ref)
	return ref
}

// serializeDestinationProfiles is
// SerializeContext::serialize_destination_profiles in krilla
// src/serialize.rs.
func (sc *serializeContext) serializeDestinationProfiles(cc *chunkContainer) {
	subtype, ok := sc.settings.validators().outputIntent()
	if !ok {
		return
	}
	rootRef := sc.newRef()
	chunk := sc.newChunk()

	oiRef := sc.newRef()
	oi := pdfwriter.Start[pdfwriter.OutputIntent](chunk.Indirect(oiRef))
	iccProfile := sc.settings.pdfVersion().rgbICC()

	// NOTE: when support for CMYK or custom RBG output intents is added,
	// revisit ContentBuilder::get_color_space.
	metadata := iccProfile.metadata()
	oi.DestOutputProfile(sc.registerCacheable(cc, iccProfile)).
		Subtype(subtype).
		OutputConditionIdentifier(pdfwriter.TextStr("Custom")).
		OutputCondition(pdfwriter.TextStr("sRGB")).
		RegistryName(pdfwriter.TextStr("")).
		Info(pdfwriter.TextStr(fmt.Sprintf("sRGB v%d.%d", metadata.major, metadata.minor)))
	oi.Finish()

	array := chunk.Indirect(rootRef).Array()
	array.Item(oiRef)
	array.Finish()

	cc.nonStream.destinationProfiles = &refChunk{ref: rootRef, chunk: chunk}
}

// serializeXMPSchema is SerializeContext::serialize_xmp_schema in krilla
// src/serialize.rs.
func (sc *serializeContext) serializeXMPSchema(cc *chunkContainer) {
	const path = "xmp-metadata.rng"

	metadata := cc.metadata
	if metadata == nil {
		return
	}

	if !sc.settings.validators().recommendsXMPRelaxNGSchema() ||
		len(metadata.customXMPProperties) == 0 {
		return
	}
	// The schema is only recommended, so a file the user embedded under the
	// same name takes precedence.
	if _, ok := sc.globalObjects.embeddedFiles[path]; ok {
		return
	}

	mimeType, _ := NewMIMEType("application/xml")
	description := "RELAX NG schema of the XMP metadata"
	file := &EmbeddedFile{
		Path:             path,
		MIMEType:         mimeType,
		Description:      &description,
		AssociationKind:  AssociationKindSchema,
		Data:             []byte(metadata.relaxNGSchema()),
		ModificationDate: metadata.creationDate,
	}

	sc.embedFile(cc, file)
	if ref, ok := sc.globalObjects.embeddedFiles[path]; ok {
		cc.xmpSchema = &ref
	}
}

// serializePageLabelTree is SerializeContext::serialize_page_label_tree in
// krilla src/serialize.rs.
func (sc *serializeContext) serializePageLabelTree(cc *chunkContainer) {
	labels := make([]PageLabel, len(sc.pageInfos))
	for i, page := range sc.pageInfos {
		labels[i] = page.pageLabel
	}
	if container, ok := newPageLabelContainer(labels); ok {
		pageLabelTreeRef := sc.newRef()
		container.serialize(sc, cc, pageLabelTreeRef)
	}
}

// serializeOutline is SerializeContext::serialize_outline in krilla
// src/serialize.rs.
func (sc *serializeContext) serializeOutline(cc *chunkContainer) {
	outline := sc.globalObjects.outline
	sc.globalObjects.outline = nil
	if outline != nil {
		outlineRef := sc.newRef()
		outline.serialize(sc, cc, outlineRef)
	} else {
		sc.registerValidationError(validationError{kind: validationMissingDocumentOutline})
	}
}

// serializePages is SerializeContext::serialize_pages in krilla
// src/serialize.rs.
func (sc *serializeContext) serializePages(cc *chunkContainer) error {
	pages := sc.globalObjects.pages
	sc.globalObjects.pages = nil
	for _, p := range pages {
		if err := p.page.serialize(sc, cc, p.ref); err != nil {
			return err
		}
	}
	return nil
}

// serializePageTree is SerializeContext::serialize_page_tree in krilla
// src/serialize.rs.
func (sc *serializeContext) serializePageTree(cc *chunkContainer) {
	pageTreeChunk := sc.newChunk()
	kids := make([]pdfwriter.Ref, len(sc.pageInfos))
	for i, info := range sc.pageInfos {
		kids[i] = info.ref
	}
	pageTreeChunk.Pages(sc.pageTreeRef).
		Count(int32(len(sc.pageInfos))).
		Kids(kids...).
		Finish()
	cc.nonStream.pageTree = &refChunk{ref: sc.pageTreeRef, chunk: pageTreeChunk}
}

// serializeTagTree is SerializeContext::serialize_tag_tree in krilla
// src/serialize.rs, for a document without tags, which is each document of
// the base port (D62).
func (sc *serializeContext) serializeTagTree() {
	sc.registerValidationError(validationError{kind: validationMissingTagging})
}

// checkValidatorLimits is SerializeContext::check_validator_limits in krilla
// src/serialize.rs.
func (sc *serializeContext) checkValidatorLimits() {
	if sc.curRef.Get() > 8388607 {
		sc.registerValidationError(validationError{kind: validationTooManyIndirectObjects})
	}
	if sc.limits.StrLen() > strLen {
		sc.registerValidationError(validationError{kind: validationTooLongString})
	}
	if sc.limits.NameLen() > nameLen {
		sc.registerValidationError(validationError{kind: validationTooLongName})
	}
	if sc.limits.Real() > maxFloat {
		sc.registerValidationError(validationError{kind: validationTooLargeFloat})
	}
	if sc.limits.ArrayLen() > arrayLen {
		sc.registerValidationError(validationError{kind: validationTooLongArray})
	}
	if sc.limits.DictEntries() > dictLen {
		sc.registerValidationError(validationError{kind: validationTooLongDictionary})
	}
}

// checkVersionLimits is SerializeContext::check_version_limits in krilla
// src/serialize.rs. It returns the limit of PDF 1.4 that the file exceeds.
func (sc *serializeContext) checkVersionLimits() (limitError, bool) {
	if sc.serializeSettings().pdfVersion() != PDF14 {
		return 0, false
	}
	if sc.limits.Real() > maxFloat {
		return limitTooLargeFloat, true
	}
	if sc.limits.ArrayLen() > arrayLen {
		return limitTooLongArray, true
	}
	if sc.limits.DictEntries() > dictLen {
		return limitTooLongDictionary, true
	}
	return 0, false
}
