// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 constants.py (D49). A frozenset of upstream
// becomes a function with a switch, and a dict becomes a function that
// returns the value and true, so that no table is built when the program
// starts.

// The namespaces of an element or an attribute, as the namespaces of
// tinyhtml5 constants.py name them. An element of HTML has NamespaceHTML.
const (
	NamespaceHTML   = "http://www.w3.org/1999/xhtml"
	NamespaceMathML = "http://www.w3.org/1998/Math/MathML"
	NamespaceSVG    = "http://www.w3.org/2000/svg"
	NamespaceXLink  = "http://www.w3.org/1999/xlink"
	NamespaceXML    = "http://www.w3.org/XML/1998/namespace"
	NamespaceXMLNS  = "http://www.w3.org/2000/xmlns/"
)

// eof is EOF in tinyhtml5 constants.py: the character that the input stream
// gives at the end of its input.
const eof rune = -1

// scopingElement is scoping_elements in tinyhtml5 constants.py.
func scopingElement(ns, name string) bool {
	switch ns {
	case NamespaceHTML:
		switch name {
		case "applet", "caption", "html", "marquee", "object", "table", "td", "th":
			return true
		}
	case NamespaceMathML:
		switch name {
		case "mi", "mo", "mn", "ms", "mtext", "annotation-xml":
			return true
		}
	case NamespaceSVG:
		switch name {
		case "foreignObject", "desc", "title":
			return true
		}
	}
	return false
}

// specialElement is special_elements in tinyhtml5 constants.py.
func specialElement(ns, name string) bool {
	switch ns {
	case NamespaceHTML:
		switch name {
		case "address", "applet", "area", "article", "aside", "base", "basefont",
			"bgsound", "blockquote", "body", "br", "button", "caption", "center",
			"col", "colgroup", "command", "dd", "details", "dir", "div", "dl",
			"dt", "embed", "fieldset", "figure", "footer", "form", "frame",
			"frameset", "h1", "h2", "h3", "h4", "h5", "h6", "head", "header", "hr",
			"html", "iframe",
			// Note that image is commented out in the spec as "this isn't an
			// element that can end up on the stack, so it doesn't matter,"
			"image",
			"img", "input", "isindex", "li", "link", "listing", "marquee", "menu",
			"meta", "nav", "noembed", "noframes", "noscript", "object", "ol", "p",
			"param", "plaintext", "pre", "script", "section", "select", "style",
			"table", "tbody", "td", "textarea", "tfoot", "th", "thead", "title",
			"tr", "ul", "wbr", "xmp":
			return true
		}
	case NamespaceSVG:
		return name == "foreignObject"
	}
	return false
}

// htmlIntegrationPointElement is html_integration_point_elements in
// tinyhtml5 constants.py.
func htmlIntegrationPointElement(ns, name string) bool {
	switch ns {
	case NamespaceMathML:
		return name == "annotation-xml"
	case NamespaceSVG:
		switch name {
		case "foreignObject", "desc", "title":
			return true
		}
	}
	return false
}

// mathMLTextIntegrationPointElement is
// mathml_text_integration_point_elements in tinyhtml5 constants.py.
func mathMLTextIntegrationPointElement(ns, name string) bool {
	if ns != NamespaceMathML {
		return false
	}
	switch name {
	case "mi", "mo", "mn", "ms", "mtext":
		return true
	}
	return false
}

// adjustSVGAttribute is adjust_svg_attributes in tinyhtml5 constants.py.
func adjustSVGAttribute(name string) (string, bool) {
	switch name {
	case "attributename":
		return "attributeName", true
	case "attributetype":
		return "attributeType", true
	case "basefrequency":
		return "baseFrequency", true
	case "baseprofile":
		return "baseProfile", true
	case "calcmode":
		return "calcMode", true
	case "clippathunits":
		return "clipPathUnits", true
	case "contentscripttype":
		return "contentScriptType", true
	case "contentstyletype":
		return "contentStyleType", true
	case "diffuseconstant":
		return "diffuseConstant", true
	case "edgemode":
		return "edgeMode", true
	case "externalresourcesrequired":
		return "externalResourcesRequired", true
	case "filterres":
		return "filterRes", true
	case "filterunits":
		return "filterUnits", true
	case "glyphref":
		return "glyphRef", true
	case "gradienttransform":
		return "gradientTransform", true
	case "gradientunits":
		return "gradientUnits", true
	case "kernelmatrix":
		return "kernelMatrix", true
	case "kernelunitlength":
		return "kernelUnitLength", true
	case "keypoints":
		return "keyPoints", true
	case "keysplines":
		return "keySplines", true
	case "keytimes":
		return "keyTimes", true
	case "lengthadjust":
		return "lengthAdjust", true
	case "limitingconeangle":
		return "limitingConeAngle", true
	case "markerheight":
		return "markerHeight", true
	case "markerunits":
		return "markerUnits", true
	case "markerwidth":
		return "markerWidth", true
	case "maskcontentunits":
		return "maskContentUnits", true
	case "maskunits":
		return "maskUnits", true
	case "numoctaves":
		return "numOctaves", true
	case "pathlength":
		return "pathLength", true
	case "patterncontentunits":
		return "patternContentUnits", true
	case "patterntransform":
		return "patternTransform", true
	case "patternunits":
		return "patternUnits", true
	case "pointsatx":
		return "pointsAtX", true
	case "pointsaty":
		return "pointsAtY", true
	case "pointsatz":
		return "pointsAtZ", true
	case "preservealpha":
		return "preserveAlpha", true
	case "preserveaspectratio":
		return "preserveAspectRatio", true
	case "primitiveunits":
		return "primitiveUnits", true
	case "refx":
		return "refX", true
	case "refy":
		return "refY", true
	case "repeatcount":
		return "repeatCount", true
	case "repeatdur":
		return "repeatDur", true
	case "requiredextensions":
		return "requiredExtensions", true
	case "requiredfeatures":
		return "requiredFeatures", true
	case "specularconstant":
		return "specularConstant", true
	case "specularexponent":
		return "specularExponent", true
	case "spreadmethod":
		return "spreadMethod", true
	case "startoffset":
		return "startOffset", true
	case "stddeviation":
		return "stdDeviation", true
	case "stitchtiles":
		return "stitchTiles", true
	case "surfacescale":
		return "surfaceScale", true
	case "systemlanguage":
		return "systemLanguage", true
	case "tablevalues":
		return "tableValues", true
	case "targetx":
		return "targetX", true
	case "targety":
		return "targetY", true
	case "textlength":
		return "textLength", true
	case "viewbox":
		return "viewBox", true
	case "viewtarget":
		return "viewTarget", true
	case "xchannelselector":
		return "xChannelSelector", true
	case "ychannelselector":
		return "yChannelSelector", true
	case "zoomandpan":
		return "zoomAndPan", true
	}
	return "", false
}

// adjustMathMLAttribute is adjust_mathml_attributes in tinyhtml5
// constants.py.
func adjustMathMLAttribute(name string) (string, bool) {
	if name == "definitionurl" {
		return "definitionURL", true
	}
	return "", false
}

// adjustForeignAttribute is adjust_foreign_attributes in tinyhtml5
// constants.py. It returns the namespace and the local name of an attribute
// of a foreign element. Upstream also gives the prefix, which the tree
// does not keep.
func adjustForeignAttribute(name string) (ns, local string, ok bool) {
	switch name {
	case "xlink:actuate", "xlink:arcrole", "xlink:href", "xlink:role",
		"xlink:show", "xlink:title", "xlink:type":
		return NamespaceXLink, name[len("xlink:"):], true
	case "xml:base", "xml:lang", "xml:space":
		return NamespaceXML, name[len("xml:"):], true
	case "xmlns":
		return NamespaceXMLNS, "xmlns", true
	case "xmlns:xlink":
		return NamespaceXMLNS, "xlink", true
	}
	return "", "", false
}

// spaceCharacter is space_characters in tinyhtml5 constants.py.
func spaceCharacter(c rune) bool {
	switch c {
	case '\t', '\n', '\u000C', ' ', '\r':
		return true
	}
	return false
}

// spaceCharacters is space_characters in tinyhtml5 constants.py, as a string
// for charsUntil.
const spaceCharacters = "\t\n\u000C \r"

// tableInsertModeElement is table_insert_mode_elements in tinyhtml5
// constants.py.
func tableInsertModeElement(name string) bool {
	switch name {
	case "table", "tbody", "tfoot", "thead", "tr":
		return true
	}
	return false
}

// asciiLetter is ascii_letters in tinyhtml5 constants.py.
func asciiLetter(c rune) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// digit is digits in tinyhtml5 constants.py.
func digit(c rune) bool {
	return '0' <= c && c <= '9'
}

// hexDigit is hexdigits in tinyhtml5 constants.py.
func hexDigit(c rune) bool {
	return digit(c) || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// asciiUpperToLower is str.translate(ascii_upper_to_lower) with the table
// ascii_upper_to_lower of tinyhtml5 constants.py. It changes only the ASCII
// letters.
func asciiUpperToLower(s string) string {
	for i := range len(s) {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// headingElement is heading_elements in tinyhtml5 constants.py.
func headingElement(name string) bool {
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}

// headingElements is heading_elements in tinyhtml5 constants.py. The heading
// elements need to be ordered.
var headingElements = [...]string{"h1", "h2", "h3", "h4", "h5", "h6"}

// void_elements of tinyhtml5 constants.py has no port. Only a test of
// upstream reads it, for a token type that the tokenizer never gives.

// cdataElement is cdata_elements in tinyhtml5 constants.py.
func cdataElement(name string) bool {
	return name == "title" || name == "textarea"
}

// rcdataElement is rcdata_elements in tinyhtml5 constants.py.
func rcdataElement(name string) bool {
	switch name {
	case "style", "script", "xmp", "iframe", "noembed", "noframes", "noscript":
		return true
	}
	return false
}

// replacementCharacter is replacement_characters in tinyhtml5 constants.py.
func replacementCharacter(c int) (rune, bool) {
	switch {
	case c == 0x0:
		return '�', true
	case c == 0x0d:
		return '\u000D', true
	case 0x80 <= c && c <= 0x9F:
		return windows1252High[c-0x80], true
	}
	return 0, false
}

// windows1252High holds the values of replacement_characters in tinyhtml5
// constants.py from 0x80 to 0x9F.
var windows1252High = [32]rune{
	'€', '\u0081', '‚', 'ƒ', '„', '…', '†', '‡',
	'ˆ', '‰', 'Š', '‹', 'Œ', '\u008D', 'Ž', '\u008F',
	'\u0090', '‘', '’', '“', '”', '•', '–', '—',
	'˜', '™', 'š', '›', 'œ', '\u009D', 'ž', 'Ÿ',
}

// tokenType is Token in tinyhtml5 constants.py.
type tokenType uint8

// The values of tokenType.
const (
	tokenDoctype tokenType = iota
	tokenCharacters
	tokenSpaceCharacters
	tokenStartTag
	tokenEndTag
	tokenEmptyTag
	tokenComment
	tokenParseError
)

// prefixes is prefixes in tinyhtml5 constants.py: the prefix of each
// namespace, which the tests print.
func prefixes(ns string) string {
	switch ns {
	case NamespaceHTML:
		return "html"
	case NamespaceMathML:
		return "math"
	case NamespaceSVG:
		return "svg"
	case NamespaceXLink:
		return "xlink"
	case NamespaceXML:
		return "xml"
	case NamespaceXMLNS:
		return "xmlns"
	}
	return ""
}
