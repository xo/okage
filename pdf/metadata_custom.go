// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/internal/xmpwriter"
)

// This file ports krilla src/interchange/metadata/custom.rs (D99).

// serializeCustomXMP is Metadata::serialize_custom_xmp in krilla
// src/interchange/metadata/custom.rs.
func (m *Metadata) serializeCustomXMP(xmp *xmpwriter.Writer, sc *serializeContext) error {
	namespaces := newXMPNamespaces()
	schemas := map[string]*Schema{}
	for i := range m.customXMPSchemas {
		schema := &m.customXMPSchemas[i]
		if err := namespaces.registerSchema(schema); err != nil {
			return err
		}
		uri := schema.namespace.uri
		_, dup := schemas[uri]
		schemas[uri] = schema
		if dup {
			return xmpError(XMPErrorDuplicateSchema, uri)
		}
	}

	validators := sc.serializeSettings().validators()
	checkDescriptions := validators.requiresXMPMetadataExtensionSchema()
	if checkDescriptions {
		checkPDFAValueTypes(m.extensionSchemas(validators), sc)
	}
	xmp2005 := validators.usesXMP2005PredefinedSchemas()
	builtinSchemas := validators.extensionSchemas()

	type key struct{ uri, name string }
	written := map[key]bool{}
	for i := range m.customXMPProperties {
		property := &m.customXMPProperties[i]
		if err := checkName(property.Name); err != nil {
			return err
		}
		uri := property.Namespace.uri
		if prefix, ok := managedPrefix(uri, property.Name); ok {
			return xmpError(XMPErrorManagedProperty, prefix+":"+property.Name)
		}

		var description *PropertyDescription
		if s, ok := schemas[uri]; ok {
			description, _ = s.get(property.Name)
		}
		if checkDescriptions &&
			description == nil &&
			!isPredefinedPDFAProperty(uri, property.Name, xmp2005) &&
			!slices.ContainsFunc(builtinSchemas, func(s Schema) bool {
				_, ok := s.get(property.Name)
				return s.namespace.uri == uri && ok
			}) {
			sc.registerValidationError(validationError{
				kind:         validationMissingXMPPropertyDescription,
				namespaceURI: uri,
				propertyName: property.Name,
			})
		}

		writer := valueWriter{namespaces: namespaces, property: property.Name}
		namespace, err := writer.namespace(property.Namespace)
		if err != nil {
			return err
		}
		k := key{uri, property.Name}
		if written[k] {
			return xmpError(XMPErrorDuplicateProperty, namespace.Prefix()+":"+property.Name)
		}
		written[k] = true
		var valueType *ValueType
		if description != nil {
			valueType = &description.valueType
		}
		if err := writer.writeProperty(xmp.Element(property.Name, namespace), &property.Value, valueType); err != nil {
			return err
		}
	}

	for i := range m.customXMPSchemas {
		schema := &m.customXMPSchemas[i]
		err := checkRequired(schema.properties, func(name string) bool {
			return written[key{schema.namespace.uri, name}]
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// customXMPDate is custom_xmp_date in krilla
// src/interchange/metadata/custom.rs.
//
// Formats a date of a custom property in the closest of the forms allowed
// by XMP Specification Part 1, section 8.2.1.2.
func customXMPDate(date DateTime) string {
	text := fmt.Sprintf("%04d", date.year)
	if !date.hasMonth || !date.hasDay {
		if date.hasMonth {
			text += fmt.Sprintf("-%02d", date.month)
		}
		return text
	}

	text += fmt.Sprintf("-%02d-%02d", date.month, date.day)
	if !date.hasHour {
		return text
	}

	text += fmt.Sprintf("T%02d:%02d", date.hour, orU8(date.minute, date.hasMinute, 0))
	if date.hasSecond {
		text += fmt.Sprintf(":%02d", date.second)
	}

	if date.hasUTCOffsetHour {
		sign := '+'
		offset := int(date.utcOffsetHour)
		if offset < 0 {
			sign = '-'
			offset = -offset
		}
		text += fmt.Sprintf("%c%02d:%02d", sign, offset, date.utcOffsetMinute)
	}

	return text
}

// knownXMPNamespace is known_xmp_namespace in krilla
// src/interchange/metadata/custom.rs. It returns false where upstream
// returns None.
//
// Returns the predefined xmp-writer namespace with the given URI, if any.
//
// A user-supplied namespace with the URI of a namespace that xmp-writer
// serializes natively must be mapped to the corresponding built-in variant,
// since xmp-writer would otherwise declare the same namespace twice.
func knownXMPNamespace(uri string) (xmpwriter.Namespace, bool) {
	for _, ns := range xmpwriter.Predefined {
		if ns.URL() == uri {
			return ns, true
		}
	}
	return xmpwriter.Namespace{}, false
}

// isReservedXMPPrefix is is_reserved_xmp_prefix in krilla
// src/interchange/metadata/custom.rs.
func isReservedXMPPrefix(prefix string) bool {
	if prefix == "xml" || prefix == "xmlns" {
		return true
	}
	for _, ns := range xmpwriter.Predefined {
		if ns.Prefix() == prefix {
			return true
		}
	}
	return false
}

// checkName is check_name in krilla src/interchange/metadata/custom.rs.
//
// Checks that name is an NCName, i.e. an XML name without a colon.
func checkName(name string) error {
	isStartChar := func(c rune) bool {
		switch {
		case c >= 'A' && c <= 'Z', c == '_', c >= 'a' && c <= 'z',
			c >= 0xC0 && c <= 0xD6, c >= 0xD8 && c <= 0xF6, c >= 0xF8 && c <= 0x2FF,
			c >= 0x370 && c <= 0x37D, c >= 0x37F && c <= 0x1FFF, c >= 0x200C && c <= 0x200D,
			c >= 0x2070 && c <= 0x218F, c >= 0x2C00 && c <= 0x2FEF, c >= 0x3001 && c <= 0xD7FF,
			c >= 0xF900 && c <= 0xFDCF, c >= 0xFDF0 && c <= 0xFFFD, c >= 0x10000 && c <= 0xEFFFF:
			return true
		}
		return false
	}
	isChar := func(c rune) bool {
		return isStartChar(c) || c == '-' || c == '.' || (c >= '0' && c <= '9') || c == 0xB7 ||
			(c >= 0x300 && c <= 0x36F) || (c >= 0x203F && c <= 0x2040)
	}

	ok := name != ""
	for i, c := range name {
		if i == 0 {
			ok = ok && isStartChar(c)
		} else {
			ok = ok && isChar(c)
		}
	}
	if !ok {
		return xmpError(XMPErrorInvalidName, name)
	}
	return nil
}

// checkDescriptionNames is check_description_names in krilla
// src/interchange/metadata/custom.rs.
func checkDescriptionNames(descriptions []PropertyDescription) error {
	for i := range descriptions {
		if err := checkName(descriptions[i].name); err != nil {
			return err
		}
	}
	return nil
}

// mergeSchemas is merge_schemas in krilla
// src/interchange/metadata/custom.rs. Upstream borrows a schema until it
// changes it. The port copies the slice of properties of a schema that it
// changes, so that the schemas it came from do not change.
//
// Merges the user schemas into the builtin schemas of the same namespace.
func mergeSchemas(builtin, user []Schema) []Schema {
	schemas := slices.Clone(builtin)
	for _, schema := range user {
		uri := schema.namespace.uri
		i := slices.IndexFunc(schemas, func(s Schema) bool { return s.namespace.uri == uri })
		if i < 0 {
			schemas = append(schemas, schema)
			continue
		}
		existing := &schemas[i]
		var added []PropertyDescription
		for _, desc := range schema.properties {
			if _, ok := existing.get(desc.name); !ok {
				added = append(added, desc)
			}
		}
		existing.properties = append(slices.Clip(existing.properties), added...)
	}

	return schemas
}

// xmpNamespaces is Namespaces in krilla
// src/interchange/metadata/custom.rs.
type xmpNamespaces struct {
	byURI    map[string]string
	byPrefix map[string]string
}

// newXMPNamespaces is the Default of Namespaces in krilla
// src/interchange/metadata/custom.rs.
func newXMPNamespaces() *xmpNamespaces {
	return &xmpNamespaces{byURI: map[string]string{}, byPrefix: map[string]string{}}
}

// register is Namespaces::register in krilla
// src/interchange/metadata/custom.rs.
func (n *xmpNamespaces) register(ns Namespace) error {
	// Known schemas are always serialized under their canonical prefix, so
	// the user-supplied prefix can't conflict.
	if _, ok := knownXMPNamespace(ns.uri); ok {
		return nil
	}

	if err := checkName(ns.prefix); err != nil {
		return err
	}
	if isReservedXMPPrefix(ns.prefix) {
		return xmpError(XMPErrorReservedPrefix, ns.prefix)
	}

	if prefix, ok := n.byURI[ns.uri]; !ok {
		n.byURI[ns.uri] = ns.prefix
	} else if prefix != ns.prefix {
		return xmpError(XMPErrorConflictingNamespace, ns.uri)
	}

	if uri, ok := n.byPrefix[ns.prefix]; !ok {
		n.byPrefix[ns.prefix] = ns.uri
	} else if uri != ns.uri {
		return xmpError(XMPErrorConflictingPrefix, ns.prefix)
	}

	return nil
}

// registerSchema is Namespaces::register_schema in krilla
// src/interchange/metadata/custom.rs.
func (n *xmpNamespaces) registerSchema(schema *Schema) error {
	if err := n.register(schema.namespace); err != nil {
		return err
	}
	if err := checkDescriptionNames(schema.properties); err != nil {
		return err
	}
	for i := range schema.properties {
		for _, ty := range structTypes(&schema.properties[i].valueType) {
			if err := n.register(ty.namespace); err != nil {
				return err
			}
			if err := checkName(ty.name); err != nil {
				return err
			}
			if err := checkDescriptionNames(ty.fields); err != nil {
				return err
			}
		}
	}

	return nil
}

// valueWriter is ValueWriter in krilla src/interchange/metadata/custom.rs.
type valueWriter struct {
	namespaces *xmpNamespaces
	property   string
}

// namespace is ValueWriter::namespace in krilla
// src/interchange/metadata/custom.rs.
func (w *valueWriter) namespace(ns Namespace) (xmpwriter.Namespace, error) {
	if err := w.namespaces.register(ns); err != nil {
		return xmpwriter.Namespace{}, err
	}
	return buildXMPNamespace(ns), nil
}

// writeProperty is ValueWriter::write_property in krilla
// src/interchange/metadata/custom.rs. A nil value type is None.
func (w *valueWriter) writeProperty(element *xmpwriter.Element, value *Value, valueType *ValueType) error {
	if valueType != nil && !matchesValueType(value, valueType) {
		return xmpError(XMPErrorInvalidValue, w.property)
	}

	var itemType *ValueType
	if valueType != nil {
		itemType = valueType.arrayItem()
	}

	switch value.kind {
	case valueText:
		element.Value(xmpwriter.Text(value.text))
	case valueURI:
		element.URI(value.text)
	case valueBool:
		element.Value(xmpwriter.Bool(value.b))
	case valueInteger:
		element.Value(xmpwriter.Int64(value.i))
	case valueReal:
		if !isFinite64(value.r) {
			return xmpError(XMPErrorInvalidNumber, w.property)
		}
		element.Value(xmpwriter.Float64(value.r))
	case valueRational:
		if value.den == 0 {
			return xmpError(XMPErrorInvalidNumber, w.property)
		}
		element.Value(xmpwriter.Text(strconv.FormatInt(value.i, 10) + "/" + strconv.FormatUint(value.den, 10)))
	case valueDate:
		element.Value(xmpwriter.Text(customXMPDate(value.date)))
	case valueOrderedArray:
		return w.writeArray(element.Array(xmpwriter.RdfCollectionSeq), value.items, itemType)
	case valueUnorderedArray:
		return w.writeArray(element.Array(xmpwriter.RdfCollectionBag), value.items, itemType)
	case valueAlternativeArray:
		return w.writeArray(element.Array(xmpwriter.RdfCollectionAlt), value.items, itemType)
	case valueLanguageAlternative:
		for _, item := range value.langs {
			if item.Lang != nil && !isLocale(*item.Lang) {
				return xmpError(XMPErrorInvalidValue, w.property)
			}
		}
		items := make([]xmpwriter.LangAlt, len(value.langs))
		for i, item := range value.langs {
			items[i].Value = item.Text
			if item.Lang != nil {
				lang := xmpwriter.LangID(*item.Lang)
				items[i].Lang = &lang
			}
		}
		element.LanguageAlternative(items)
	case valueStruct:
		var structType *StructType
		if valueType != nil {
			switch valueType.kind {
			case valueTypeStruct:
				structType = valueType.structType
			case valueTypePredefinedStruct:
				structType = predefinedStructType(valueType.predefined)
			}
		}
		return w.writeStruct(element.Obj(), value.fields, structType)
	}

	return nil
}

// isFinite64 reports whether a float64 is neither infinite nor NaN, as
// f64::is_finite does.
func isFinite64(f float64) bool {
	return f-f == 0
}

// writeStruct is ValueWriter::write_struct in krilla
// src/interchange/metadata/custom.rs. The struct closes at the end, as
// upstream drops it, also when a field gives an error.
func (w *valueWriter) writeStruct(s *xmpwriter.Struct, fields []Property, structType *StructType) error {
	defer s.Finish()
	type key struct{ uri, name string }
	written := map[key]bool{}
	for i := range fields {
		field := &fields[i]
		var fieldType *ValueType
		if structType != nil {
			j := slices.IndexFunc(structType.fields, func(desc PropertyDescription) bool {
				return desc.name == field.Name && field.Namespace.uri == structType.namespace.uri
			})
			if j < 0 {
				return xmpError(XMPErrorInvalidValue, w.property)
			}
			fieldType = &structType.fields[j].valueType
		}

		if err := checkName(field.Name); err != nil {
			return err
		}
		namespace, err := w.namespace(field.Namespace)
		if err != nil {
			return err
		}
		k := key{field.Namespace.uri, field.Name}
		if written[k] {
			return xmpError(XMPErrorDuplicateProperty, namespace.Prefix()+":"+field.Name)
		}
		written[k] = true
		element := s.Element(field.Name, namespace)
		if err := w.writeProperty(element, &field.Value, fieldType); err != nil {
			return err
		}
	}

	if structType != nil {
		return checkRequired(structType.fields, func(name string) bool {
			return slices.ContainsFunc(fields, func(field Property) bool {
				return field.Name == name && field.Namespace.uri == structType.namespace.uri
			})
		})
	}

	return nil
}

// writeArray is ValueWriter::write_array in krilla
// src/interchange/metadata/custom.rs. The array closes at the end, as
// upstream drops it, also when an item gives an error.
func (w *valueWriter) writeArray(array *xmpwriter.Array, items []Value, itemType *ValueType) error {
	defer array.Finish()
	for i := range items {
		if err := w.writeProperty(array.Element(), &items[i], itemType); err != nil {
			return err
		}
	}
	return nil
}

// checkRequired is check_required in krilla
// src/interchange/metadata/custom.rs.
func checkRequired(descriptions []PropertyDescription, isPresent func(name string) bool) error {
	for i := range descriptions {
		desc := &descriptions[i]
		if desc.required && !isPresent(desc.name) {
			return xmpError(XMPErrorMissingProperty, desc.name)
		}
	}
	return nil
}

// matchesValueType is matches_value_type in krilla
// src/interchange/metadata/custom.rs.
//
// Whether the outermost shape of value matches valueType.
func matchesValueType(value *Value, valueType *ValueType) bool {
	switch valueType.kind {
	case valueTypeSimple, valueTypeOpenChoice:
		return matchesSimpleType(value, valueType.simple)
	case valueTypeClosedChoice:
		if !matchesSimpleType(value, valueType.simple) {
			return false
		}
		text, ok := simpleValueText(value)
		return ok && slices.Contains(valueType.choices, text)
	case valueTypeOrderedArray:
		return value.kind == valueOrderedArray
	case valueTypeUnorderedArray:
		return value.kind == valueUnorderedArray
	case valueTypeAlternativeArray:
		return value.kind == valueAlternativeArray
	case valueTypeLanguageAlternative:
		return value.kind == valueLanguageAlternative
	case valueTypePredefinedStruct, valueTypeStruct:
		return value.kind == valueStruct
	}
	return false
}

// matchesSimpleType is matches_simple_type in krilla
// src/interchange/metadata/custom.rs.
func matchesSimpleType(value *Value, ty SimpleType) bool {
	switch ty {
	case SimpleTypeBoolean:
		return value.kind == valueBool
	case SimpleTypeInteger:
		return value.kind == valueInteger
	case SimpleTypeReal:
		return value.kind == valueReal || value.kind == valueInteger
	case SimpleTypeRational:
		return value.kind == valueRational
	case SimpleTypeDate:
		return value.kind == valueDate
	case SimpleTypeURI, SimpleTypeURL:
		return value.kind == valueURI
	case SimpleTypeLocale:
		return value.kind == valueText && isLocale(value.text)
	case SimpleTypeMIMEType:
		return value.kind == valueText && isMIMEType(value.text)
	}
	return value.kind == valueText
}

// isLocale is is_locale in krilla src/interchange/metadata/custom.rs.
//
// Whether s matches [a-zA-Z]+(-[a-zA-Z0-9]+)*, the pattern the RELAX NG
// schema uses for locales.
func isLocale(s string) bool {
	for i, subtag := range strings.Split(s, "-") {
		if subtag == "" {
			return false
		}
		for _, b := range []byte(subtag) {
			alpha := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
			if !alpha && (i == 0 || b < '0' || b > '9') {
				return false
			}
		}
	}
	return true
}

// isMIMEType is is_mime_type in krilla src/interchange/metadata/custom.rs.
//
// Whether s matches the pattern the RELAX NG schema uses for MIME types.
func isMIMEType(s string) bool {
	types := []string{
		"application",
		"audio",
		"example",
		"image",
		"message",
		"model",
		"multipart",
		"text",
		"video",
	}

	ty, subtype, ok := strings.Cut(s, "/")
	if !ok || !slices.Contains(types, ty) {
		return false
	}
	start := 0
	for i := 0; i <= len(subtype); i++ {
		if i < len(subtype) && subtype[i] != '+' && subtype[i] != '-' && subtype[i] != '.' {
			continue
		}
		part := subtype[start:i]
		if part == "" {
			return false
		}
		for _, b := range []byte(part) {
			if (b < 'a' || b > 'z') && (b < 'A' || b > 'Z') && (b < '0' || b > '9') {
				return false
			}
		}
		start = i + 1
	}
	return true
}

// simpleValueText is simple_value_text in krilla
// src/interchange/metadata/custom.rs. It returns false where upstream
// returns None.
//
// The serialized form of a simple value, as compared against the values of
// a closed choice.
func simpleValueText(value *Value) (string, bool) {
	switch value.kind {
	case valueText, valueURI:
		return value.text, true
	case valueBool:
		if value.b {
			return "True", true
		}
		return "False", true
	case valueInteger:
		return strconv.FormatInt(value.i, 10), true
	case valueReal:
		return string(xmpwriter.Float64(value.r).Append(nil)), true
	}
	return "", false
}

// structTypes is struct_types in krilla
// src/interchange/metadata/custom.rs.
//
// The user-defined structure types used by valueType, including those
// nested in other structure types.
func structTypes(valueType *ValueType) []*StructType {
	var types []*StructType
	var visit func(valueType *ValueType)
	visit = func(valueType *ValueType) {
		switch valueType.kind {
		case valueTypeOrderedArray, valueTypeUnorderedArray, valueTypeAlternativeArray:
			visit(valueType.item)
		case valueTypeStruct:
			ty := valueType.structType
			if slices.ContainsFunc(types, ty.equal) {
				return
			}
			types = append(types, ty)
			for i := range ty.fields {
				visit(&ty.fields[i].valueType)
			}
		}
	}

	visit(valueType)
	return types
}

// buildXMPNamespace is build_xmp_namespace in krilla
// src/interchange/metadata/custom.rs.
func buildXMPNamespace(ns Namespace) xmpwriter.Namespace {
	if known, ok := knownXMPNamespace(ns.uri); ok {
		return known
	}
	return xmpwriter.NewCustom(xmpwriter.NewCustomNamespace(ns.prefix, ns.prefix, ns.escapedURI))
}
