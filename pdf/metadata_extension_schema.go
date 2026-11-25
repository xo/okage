// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"slices"

	"github.com/xo/okage/internal/xmpwriter"
)

// This file ports krilla src/interchange/metadata/extension_schema.rs
// (D99).

// checkPDFAValueTypes is check_pdfa_value_types in krilla
// src/interchange/metadata/extension_schema.rs.
//
// Registers a validation error for each property and structure field in
// schemas whose value type PDF/A extension schemas can't express.
func checkPDFAValueTypes(schemas []Schema, sc *serializeContext) {
	isExpressible := func(valueType *ValueType) bool {
		item := valueType.arrayItem()
		if item == nil {
			return true
		}
		switch item.kind {
		case valueTypeOrderedArray, valueTypeUnorderedArray, valueTypeAlternativeArray,
			valueTypeLanguageAlternative:
			return false
		}
		return true
	}

	check := func(namespace Namespace, descriptions []PropertyDescription) {
		for i := range descriptions {
			desc := &descriptions[i]
			if !isExpressible(&desc.valueType) {
				sc.registerValidationError(validationError{
					kind:         validationUnsupportedXMPValueType,
					namespaceURI: namespace.uri,
					propertyName: desc.name,
				})
			}
		}
	}

	for i := range schemas {
		schema := &schemas[i]
		check(schema.namespace, schema.properties)
		for j := range schema.properties {
			for _, ty := range structTypes(&schema.properties[j].valueType) {
				check(ty.namespace, ty.fields)
			}
		}
	}
}

// pdfaValueType is pdfa_value_type in krilla
// src/interchange/metadata/extension_schema.rs.
//
// The value of pdfaProperty:valueType describing valueType.
//
// The property is an "Open Choice of Text" (TechNote 0009, Section 4.4).
func pdfaValueType(valueType *ValueType) string {
	switch valueType.kind {
	case valueTypeSimple:
		return valueType.simple.String()
	case valueTypeLanguageAlternative:
		return "Lang Alt"
	case valueTypeOpenChoice:
		return "Open Choice of " + valueType.simple.String()
	case valueTypeClosedChoice:
		return "Closed Choice of " + valueType.simple.String()
	case valueTypeOrderedArray:
		return "seq " + pdfaValueType(valueType.item)
	case valueTypeUnorderedArray:
		return "bag " + pdfaValueType(valueType.item)
	case valueTypeAlternativeArray:
		return "alt " + pdfaValueType(valueType.item)
	case valueTypePredefinedStruct:
		return valueType.predefined.String()
	default:
		return valueType.structType.name
	}
}

// writeExtensionSchemas is write_extension_schemas in krilla
// src/interchange/metadata/extension_schema.rs.
//
// Writes the PDF/A extension schemas for krilla's own builtin schemas and
// the user schemas, merging schemas of the same namespace.
func writeExtensionSchemas(writer *xmpwriter.PDFAExtSchemasWriter, builtin, user []Schema) {
	for _, schema := range mergeSchemas(builtin, user) {
		writeExtensionSchema(writer, &schema)
	}
}

// writeExtensionSchema is write_extension_schema in krilla
// src/interchange/metadata/extension_schema.rs.
func writeExtensionSchema(writer *xmpwriter.PDFAExtSchemasWriter, schema *Schema) {
	xmpNamespace := buildXMPNamespace(schema.namespace)
	// A namespace xmp-writer knows natively is serialized under its
	// canonical prefix, not the user-supplied one.
	prefix := xmpNamespace.Prefix()

	schemaWriter := writer.AddSchema()
	schemaName := prefix + " schema"
	if schema.name != nil {
		schemaName = *schema.name
	}
	schemaWriter.Element("schema", xmpwriter.NamespacePdfASchema).Value(xmpwriter.Text(schemaName))
	schemaWriter.Element("namespaceURI", xmpwriter.NamespacePdfASchema).Value(xmpwriter.Text(schema.namespace.uri))
	schemaWriter.Element("prefix", xmpwriter.NamespacePdfASchema).Value(xmpwriter.Text(prefix))

	if len(schema.properties) != 0 {
		properties := schemaWriter.Properties()
		for i := range schema.properties {
			desc := &schema.properties[i]
			properties.AddProperty().
				Category(desc.category == CategoryInternal).
				Description(desc.description).
				Name(desc.name).
				ValueType(pdfaValueType(&desc.valueType)).
				Finish()
		}
		properties.Finish()
	}

	var types []*StructType
	for i := range schema.properties {
		for _, ty := range structTypes(&schema.properties[i].valueType) {
			if !slices.ContainsFunc(types, ty.equal) {
				types = append(types, ty)
			}
		}
	}

	if len(types) != 0 {
		valueTypes := schemaWriter.ValueTypes()
		for _, ty := range types {
			typeWriter := valueTypes.AddValueType()
			typeWriter.
				Name(ty.name).
				NamespaceURI(ty.namespace.uri).
				Prefix(buildXMPNamespace(ty.namespace).Prefix()).
				Description(ty.description)
			writeFields(typeWriter, ty.fields)
			typeWriter.Finish()
		}
		valueTypes.Finish()
	}
	schemaWriter.Finish()
}

// writeFields is write_fields in krilla
// src/interchange/metadata/extension_schema.rs.
func writeFields(typeWriter *xmpwriter.PDFAExtTypeWriter, fields []PropertyDescription) {
	fieldsWriter := typeWriter.Fields()
	for i := range fields {
		field := &fields[i]
		fieldsWriter.AddField().
			Name(field.name).
			ValueType(pdfaValueType(&field.valueType)).
			Description(field.description).
			Finish()
	}
	fieldsWriter.Finish()
}
