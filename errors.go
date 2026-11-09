package okage

// This file ports no upstream file. It holds the sentinel errors of okage, and
// of the internal packages that port the crates of svg (D21).

// Error is an error of okage. Each sentinel error of okage is a constant of
// this type, so that a caller can compare it with errors.Is.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The sentinel errors of okage.
const (
	// ErrDecodingPNG is the error of DecodePNG, LoadPNG, DecodeMaskPNG and
	// LoadMaskPNG when the package image/png, or the file system, returns an
	// error. okage wraps that error with this one. Upstream returns the error
	// of the crate png.
	ErrDecodingPNG Error = "decoding png"

	// ErrEncodingPNG is the error of EncodePNG and SavePNG of a Pixmap and of
	// a Mask when the package image/png, or the file system, returns an
	// error. okage wraps that error with this one. Upstream returns the error
	// of the crate png.
	ErrEncodingPNG Error = "encoding png"

	// ErrUnsupportedPNG is the error of DecodePNG and DecodeMaskPNG for a PNG
	// image that tiny-skia does not support. Upstream gives "unsupported bit
	// depth", "indexed PNG is not supported" and "only grayscale masks are
	// supported".
	ErrUnsupportedPNG Error = "decoding png: unsupported image"

	// ErrInvalidImageSize is the error of DecodePNG and DecodeMaskPNG for an
	// image with a zero width or height. Upstream gives "invalid image size".
	ErrInvalidImageSize Error = "decoding png: invalid image size"

	// ErrImageTooBig is the error of DecodePNG for an image that is larger
	// than a Pixmap can hold. Upstream gives "image is too big".
	ErrImageTooBig Error = "decoding png: image is too big"

	// ErrCreatingPixmap is the error of DecodePNG when the decoded pixels do
	// not make a Pixmap. Upstream gives "failed to create a pixmap".
	ErrCreatingPixmap Error = "decoding png: creating a pixmap"

	// ErrCreatingMask is the error of DecodeMaskPNG when the decoded values do
	// not make a Mask. Upstream gives "failed to create a mask".
	ErrCreatingMask Error = "decoding png: creating a mask"
)

// internal/roxmltree returns a *roxmltree.Error that wraps one of the
// following errors. Its message is "parsing xml: " and the Display text of
// upstream, with the position and the data of the case.

// The sentinel errors of internal/roxmltree.
const (
	// ErrXMLInvalidXMLPrefixURI is Error::InvalidXmlPrefixUri of roxmltree.
	// The `xmlns:xml` attribute must have an
	// <http://www.w3.org/XML/1998/namespace> URI.
	ErrXMLInvalidXMLPrefixURI Error = "parsing xml: 'xml' namespace prefix mapped to wrong URI"

	// ErrXMLUnexpectedXMLURI is Error::UnexpectedXmlUri of roxmltree. Only
	// the `xmlns:xml` attribute can have the
	// <http://www.w3.org/XML/1998/namespace> URI.
	ErrXMLUnexpectedXMLURI Error = "parsing xml: the 'xml' namespace URI is used for not 'xml' prefix"

	// ErrXMLUnexpectedXMLNSURI is Error::UnexpectedXmlnsUri of roxmltree.
	// The <http://www.w3.org/2000/xmlns/> URI must not be declared.
	ErrXMLUnexpectedXMLNSURI Error = "parsing xml: the 'xmlns' URI is used, but it must not be declared"

	// ErrXMLInvalidElementNamePrefix is Error::InvalidElementNamePrefix of
	// roxmltree. `xmlns` can not be used as an element prefix.
	ErrXMLInvalidElementNamePrefix Error = "parsing xml: the 'xmlns' prefix is used, but it must not be"

	// ErrXMLDuplicatedNamespace is Error::DuplicatedNamespace of roxmltree.
	// A namespace was already defined on this element.
	ErrXMLDuplicatedNamespace Error = "parsing xml: namespace is already defined"

	// ErrXMLUnknownNamespace is Error::UnknownNamespace of roxmltree. An
	// element or an attribute has an unknown qualified name prefix.
	ErrXMLUnknownNamespace Error = "parsing xml: an unknown namespace prefix"

	// ErrXMLUnexpectedCloseTag is Error::UnexpectedCloseTag of roxmltree.
	// The tree structure is not correct.
	ErrXMLUnexpectedCloseTag Error = "parsing xml: unexpected close tag name"

	// ErrXMLUnexpectedEntityCloseTag is Error::UnexpectedEntityCloseTag of
	// roxmltree. An entity value starts with a close tag.
	ErrXMLUnexpectedEntityCloseTag Error = "parsing xml: unexpected close tag"

	// ErrXMLUnknownEntityReference is Error::UnknownEntityReference of
	// roxmltree. A reference names an entity that the DTD does not define.
	ErrXMLUnknownEntityReference Error = "parsing xml: unknown entity reference"

	// ErrXMLMalformedEntityReference is Error::MalformedEntityReference of
	// roxmltree. A `&` character inside an attribute value or text does not
	// start a correct reference.
	ErrXMLMalformedEntityReference Error = "parsing xml: malformed entity reference"

	// ErrXMLEntityReferenceLoop is Error::EntityReferenceLoop of roxmltree.
	// The depth limit of entities is 10, and an entity can expand 255
	// references at most.
	ErrXMLEntityReferenceLoop Error = "parsing xml: a possible entity reference loop is detected"

	// ErrXMLInvalidAttributeValue is Error::InvalidAttributeValue of
	// roxmltree. An attribute value cannot have a `<` character.
	ErrXMLInvalidAttributeValue Error = "parsing xml: unescaped '<' found"

	// ErrXMLDuplicatedAttribute is Error::DuplicatedAttribute of roxmltree.
	// An element has a duplicated attribute, after namespaces are resolved.
	ErrXMLDuplicatedAttribute Error = "parsing xml: attribute is already defined"

	// ErrXMLNoRootNode is Error::NoRootNode of roxmltree. The XML document
	// must have at least one element.
	ErrXMLNoRootNode Error = "parsing xml: the document does not have a root node"

	// ErrXMLUnclosedRootNode is Error::UnclosedRootNode of roxmltree. The
	// root node was opened but never closed.
	ErrXMLUnclosedRootNode Error = "parsing xml: the root node was opened but never closed"

	// ErrXMLUnexpectedDeclaration is Error::UnexpectedDeclaration of
	// roxmltree. An XML document can have only one XML declaration, at its
	// start.
	ErrXMLUnexpectedDeclaration Error = "parsing xml: unexpected XML declaration"

	// ErrXMLDTDDetected is Error::DtdDetected of roxmltree. The document has
	// a DTD and the option AllowDTD is false.
	ErrXMLDTDDetected Error = "parsing xml: XML with DTD detected"

	// ErrXMLNodesLimitReached is Error::NodesLimitReached of roxmltree. The
	// document has more nodes than the option NodesLimit allows.
	ErrXMLNodesLimitReached Error = "parsing xml: nodes limit reached"

	// ErrXMLAttributesLimitReached is Error::AttributesLimitReached of
	// roxmltree. The document has too many attributes.
	ErrXMLAttributesLimitReached Error = "parsing xml: more than 2^32 attributes were parsed"

	// ErrXMLNamespacesLimitReached is Error::NamespacesLimitReached of
	// roxmltree. The document has too many namespaces.
	ErrXMLNamespacesLimitReached Error = "parsing xml: more than 2^16 unique namespaces were parsed"

	// ErrXMLInvalidName is Error::InvalidName of roxmltree.
	ErrXMLInvalidName Error = "parsing xml: invalid name token"

	// ErrXMLNonXMLChar is Error::NonXmlChar of roxmltree. The valid
	// characters are those of <https://www.w3.org/TR/xml/#char32>.
	ErrXMLNonXMLChar Error = "parsing xml: a non-XML character found"

	// ErrXMLInvalidChar is Error::InvalidChar of roxmltree. A byte is not
	// the one that was expected.
	ErrXMLInvalidChar Error = "parsing xml: unexpected character"

	// ErrXMLInvalidChar2 is Error::InvalidChar2 of roxmltree. A byte is not
	// of the kind that was expected.
	ErrXMLInvalidChar2 Error = "parsing xml: unexpected character kind"

	// ErrXMLInvalidString is Error::InvalidString of roxmltree. A string is
	// not the one that was expected.
	ErrXMLInvalidString Error = "parsing xml: unexpected string"

	// ErrXMLInvalidExternalID is Error::InvalidExternalID of roxmltree. The
	// DTD has an ExternalID that is not correct.
	ErrXMLInvalidExternalID Error = "parsing xml: invalid ExternalID"

	// ErrXMLEntityResolver is Error::EntityResolver of roxmltree. The entity
	// resolver returned an error.
	ErrXMLEntityResolver Error = "parsing xml: entity resolver failed"

	// ErrXMLInvalidComment is Error::InvalidComment of roxmltree. A comment
	// cannot contain `--` or end with `-`.
	ErrXMLInvalidComment Error = "parsing xml: comment contains '--'"

	// ErrXMLInvalidCharacterData is Error::InvalidCharacterData of
	// roxmltree. Character data cannot contain `]]>`.
	ErrXMLInvalidCharacterData Error = "parsing xml: ']]>' is not allowed inside a character data"

	// ErrXMLUnknownToken is Error::UnknownToken of roxmltree.
	ErrXMLUnknownToken Error = "parsing xml: unknown token" //nolint:gosec // G101: an error message, not a credential.

	// ErrXMLUnexpectedEndOfStream is Error::UnexpectedEndOfStream of
	// roxmltree. The stream ended earlier than expected.
	ErrXMLUnexpectedEndOfStream Error = "parsing xml: unexpected end of stream"
)

// internal/simplecss wraps each of the following errors in an error whose text
// is the text of upstream, such as "invalid ident at 1:3", so that errors.Is
// finds the sentinel error.

// The sentinel errors of internal/simplecss.
const (
	// ErrCSSUnexpectedEndOfStream is Error::UnexpectedEndOfStream in
	// simplecss src/lib.rs. The stream ended earlier than the parser
	// expected. Upstream gives "unexpected end of stream".
	ErrCSSUnexpectedEndOfStream Error = "parsing css: unexpected end of stream"

	// ErrCSSInvalidIdent is Error::InvalidIdent in simplecss src/lib.rs.
	// Upstream gives "invalid ident at <row>:<col>".
	ErrCSSInvalidIdent Error = "parsing css: invalid ident"

	// ErrCSSInvalidComment is Error::InvalidComment in simplecss src/lib.rs,
	// for a comment that is not closed. Upstream gives "invalid comment at
	// <row>:<col>".
	ErrCSSInvalidComment Error = "parsing css: invalid comment"

	// ErrCSSInvalidValue is Error::InvalidValue in simplecss src/lib.rs, for
	// an invalid value of a declaration. Upstream gives "invalid value at
	// <row>:<col>".
	ErrCSSInvalidValue Error = "parsing css: invalid value"

	// ErrCSSInvalidByte is Error::InvalidByte in simplecss src/lib.rs.
	// Upstream gives "expected '<byte>' not '<byte>' at <row>:<col>".
	ErrCSSInvalidByte Error = "parsing css: invalid byte"

	// ErrCSSSelectorMissing is Error::SelectorMissing in simplecss
	// src/lib.rs. Upstream gives "selector missing".
	ErrCSSSelectorMissing Error = "parsing css: selector missing"

	// ErrCSSUnexpectedSelector is Error::UnexpectedSelector in simplecss
	// src/lib.rs. Upstream gives "unexpected selector".
	ErrCSSUnexpectedSelector Error = "parsing css: unexpected selector"

	// ErrCSSUnexpectedCombinator is Error::UnexpectedCombinator in simplecss
	// src/lib.rs. Upstream gives "unexpected combinator".
	ErrCSSUnexpectedCombinator Error = "parsing css: unexpected combinator"

	// ErrCSSInvalidAttributeSelector is Error::InvalidAttributeSelector in
	// simplecss src/lib.rs, for an invalid or unsupported attribute
	// selector. Upstream gives "invalid or unsupported attribute selector".
	ErrCSSInvalidAttributeSelector Error = "parsing css: invalid or unsupported attribute selector"

	// ErrCSSInvalidLanguagePseudoClass is Error::InvalidLanguagePseudoClass
	// in simplecss src/lib.rs. Upstream gives "invalid language
	// pseudo-class".
	ErrCSSInvalidLanguagePseudoClass Error = "parsing css: invalid language pseudo-class"
)

// Each message of the following errors is the Display text of svgtypes 0.16.1
// after the prefix "parsing svg value: ". An error with data, such as a
// position, wraps its sentinel error and adds the data, as upstream writes it.

// The sentinel errors of the enum Error in svgtypes src/error.rs.
const (
	// ErrSVGTypesUnexpectedEndOfStream is Error::UnexpectedEndOfStream in
	// svgtypes src/error.rs. An input data ended earlier than expected.
	ErrSVGTypesUnexpectedEndOfStream Error = "parsing svg value: unexpected end of stream"

	// ErrSVGTypesUnexpectedData is Error::UnexpectedData in svgtypes
	// src/error.rs. An input text contains unknown data. The error wraps it
	// with the position.
	ErrSVGTypesUnexpectedData Error = "parsing svg value: unexpected data"

	// ErrSVGTypesInvalidValue is Error::InvalidValue in svgtypes src/error.rs.
	// A provided string does not have a valid data.
	ErrSVGTypesInvalidValue Error = "parsing svg value: invalid value"

	// ErrSVGTypesInvalidIdent is Error::InvalidIdent in svgtypes src/error.rs.
	// An invalid CSS ident.
	ErrSVGTypesInvalidIdent Error = "parsing svg value: invalid ident"

	// ErrSVGTypesInvalidChar is Error::InvalidChar in svgtypes src/error.rs.
	// An invalid or unexpected character. The error wraps it with the actual
	// character, the expected characters and the position.
	ErrSVGTypesInvalidChar Error = "parsing svg value: invalid char"

	// ErrSVGTypesInvalidString is Error::InvalidString in svgtypes
	// src/error.rs. An unexpected string. The error wraps it with the actual
	// string, the expected strings and the position.
	ErrSVGTypesInvalidString Error = "parsing svg value: invalid string"

	// ErrSVGTypesInvalidNumber is Error::InvalidNumber in svgtypes
	// src/error.rs. An invalid number. The error wraps it with the position.
	ErrSVGTypesInvalidNumber Error = "parsing svg value: invalid number"
)

// The sentinel errors of the enum ViewBoxError in svgtypes src/viewbox.rs.
const (
	// ErrSVGTypesViewBoxInvalidNumber is ViewBoxError::InvalidNumber in
	// svgtypes src/viewbox.rs. One of the numbers is invalid.
	ErrSVGTypesViewBoxInvalidNumber Error = "parsing svg value: viewBox contains an invalid number"

	// ErrSVGTypesViewBoxInvalidSize is ViewBoxError::InvalidSize in svgtypes
	// src/viewbox.rs. The viewBox has a negative or zero size.
	ErrSVGTypesViewBoxInvalidSize Error = "parsing svg value: viewBox has a negative or zero size"
)

// The sentinel errors of the enum TransformOriginError in svgtypes
// src/transform_origin.rs.
const (
	// ErrSVGTypesTransformOriginMissingParameters is
	// TransformOriginError::MissingParameters in svgtypes
	// src/transform_origin.rs.
	ErrSVGTypesTransformOriginMissingParameters Error = "parsing svg value: transform origin doesn't have enough parameters"

	// ErrSVGTypesTransformOriginInvalidParameters is
	// TransformOriginError::InvalidParameters in svgtypes
	// src/transform_origin.rs. One of the parameters is invalid.
	ErrSVGTypesTransformOriginInvalidParameters Error = "parsing svg value: transform origin has invalid parameters"

	// ErrSVGTypesTransformOriginZIndexIsPercentage is
	// TransformOriginError::ZIndexIsPercentage in svgtypes
	// src/transform_origin.rs. The z offset is a percentage.
	ErrSVGTypesTransformOriginZIndexIsPercentage Error = "parsing svg value: z-index cannot be a percentage"
)

// The sentinel errors of the enum FilterValueListParserError in svgtypes
// src/filter_functions.rs. Each one wraps the position. The case
// StreamErrors holds an Error of src/error.rs, and okage returns that error
// as it is.
const (
	// ErrSVGTypesFilterPercentageValue is
	// FilterValueListParserError::PercentageValue in svgtypes
	// src/filter_functions.rs. A length has a percentage value.
	ErrSVGTypesFilterPercentageValue Error = "parsing svg value: a percentage value detected"

	// ErrSVGTypesFilterNegativeValue is
	// FilterValueListParserError::NegativeValue in svgtypes
	// src/filter_functions.rs. A value is negative.
	ErrSVGTypesFilterNegativeValue Error = "parsing svg value: a negative value detected"

	// ErrSVGTypesFilterInvalidAngle is
	// FilterValueListParserError::InvalidAngle in svgtypes
	// src/filter_functions.rs.
	ErrSVGTypesFilterInvalidAngle Error = "parsing svg value: an invalid angle"

	// ErrSVGTypesFilterMissingDropShadowOffset is
	// FilterValueListParserError::MissingDropShadowOffset in svgtypes
	// src/filter_functions.rs. The offset values of drop-shadow are not set.
	ErrSVGTypesFilterMissingDropShadowOffset Error = "parsing svg value: drop-shadow offset values are expected"

	// ErrSVGTypesFilterInvalidURL is FilterValueListParserError::InvalidUrl
	// in svgtypes src/filter_functions.rs. Usually an empty url.
	ErrSVGTypesFilterInvalidURL Error = "parsing svg value: an invalid url"
)

// Each message of the following errors is "parsing svg: " and the Display
// text of usvg 0.49.0. ErrSVGParsingFailed wraps the error of
// internal/roxmltree.

// The sentinel errors of the enum Error in usvg src/parser/mod.rs. The case
// SvgzFeatureNotEnabled has no port, because okage always reads SVGZ data.
const (
	// ErrSVGNotAnUTF8Str is Error::NotAnUtf8Str in usvg src/parser/mod.rs.
	// Only UTF-8 content are supported.
	ErrSVGNotAnUTF8Str Error = "parsing svg: provided data has not an UTF-8 encoding"

	// ErrSVGMalformedGZip is Error::MalformedGZip in usvg
	// src/parser/mod.rs. Compressed SVG must use the GZip algorithm.
	ErrSVGMalformedGZip Error = "parsing svg: provided data has a malformed GZip content"

	// ErrSVGElementsLimitReached is Error::ElementsLimitReached in usvg
	// src/parser/mod.rs. usvg does not allow SVG with more than 1_000_000
	// elements for security reasons.
	ErrSVGElementsLimitReached Error = "parsing svg: the maximum number of SVG elements has been reached"

	// ErrSVGInvalidSize is Error::InvalidSize in usvg src/parser/mod.rs. SVG
	// does not have a valid size. It occurs when width and/or height are
	// <= 0, and also if width, height and viewBox are not set.
	ErrSVGInvalidSize Error = "parsing svg: SVG has an invalid size"

	// ErrSVGParsingFailed is Error::ParsingFailed in usvg
	// src/parser/mod.rs. It failed to parse an SVG data. The error wraps
	// the error of internal/roxmltree. Upstream gives "SVG data parsing
	// failed cause " and the error of roxmltree.
	ErrSVGParsingFailed Error = "parsing svg: SVG data parsing failed"
)
