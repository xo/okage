// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage/internal/xmlwriter"
)

// This file ports krilla src/interchange/metadata/relax_ng.rs (D99), with
// internal/xmlwriter in place of the crate xmlwriter.
//
// RELAX NG schemas describing XMP metadata.
//
// The simple type patterns are taken from the informative schemas of ISO
// 16684-2.

// uiNamespace is UI_NAMESPACE in krilla
// src/interchange/metadata/relax_ng.rs.
const uiNamespace = "http://ns.iso.org/iso-16684-2/xmp-schema-ui-info/1.0"

// undescribedProperty is the (&Namespace, &str) of an undescribed property
// in krilla src/interchange/metadata/relax_ng.rs.
type undescribedProperty struct {
	namespace Namespace
	name      string
}

// rngNamespace is the (String, &str) of a namespace in krilla
// src/interchange/metadata/relax_ng.rs: its prefix and its URI.
type rngNamespace struct {
	prefix string
	uri    string
}

// relaxNGSchema is relax_ng_schema in krilla
// src/interchange/metadata/relax_ng.rs.
//
// A RELAX NG schema for a packet with the properties described by schemas
// and the undescribed properties, which may have any content.
func relaxNGSchema(schemas []Schema, undescribed []undescribedProperty) string {
	var namespaces []rngNamespace
	for i := range schemas {
		schema := &schemas[i]
		addNamespace(&namespaces, schema.namespace)
		for j := range schema.properties {
			addValueTypeNamespaces(&namespaces, &schema.properties[j].valueType)
		}
	}
	for _, u := range undescribed {
		addNamespace(&namespaces, u.namespace)
	}

	g := &grammar{
		w: xmlwriter.New(
			xmlwriter.WithUseSingleQuote(false),
			xmlwriter.WithIndent(xmlwriter.Indent{Kind: xmlwriter.IndentNone}),
			xmlwriter.WithAttributesIndent(xmlwriter.Indent{Kind: xmlwriter.IndentNone}),
		),
		any: len(undescribed) != 0,
		ui:  uiPrefix(namespaces),
	}

	g.w.WriteDeclaration()
	g.w.StartElement("grammar")
	g.w.WriteAttribute("xmlns", "http://relaxng.org/ns/structure/1.0")
	g.w.WriteAttribute("xmlns:rdf", "http://www.w3.org/1999/02/22-rdf-syntax-ns#")
	g.w.WriteAttribute("xmlns:"+g.ui, uiNamespace)
	for _, ns := range namespaces {
		g.w.WriteAttribute("xmlns:"+ns.prefix, rngEscape(ns.uri))
	}
	g.w.WriteAttribute("datatypeLibrary", "http://www.w3.org/2001/XMLSchema-datatypes")

	var orphans []undescribedProperty
	for _, u := range undescribed {
		if !slices.ContainsFunc(schemas, func(s Schema) bool { return s.namespace.uri == u.namespace.uri }) {
			orphans = append(orphans, u)
		}
	}

	g.element("start", func(g *grammar) {
		g.element("element", func(g *grammar) {
			g.w.WriteAttribute("name", "rdf:RDF")
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:Description")
				g.element("attribute", func(g *grammar) { g.w.WriteAttribute("name", "rdf:about") })
				g.interleave(len(schemas)+len(orphans), func(g *grammar) {
					for i := range schemas {
						g.reference(schemaDefine(schemas[i].namespace))
					}
					for _, o := range orphans {
						g.anyProperty(rngPrefix(o.namespace), o.name)
					}
				})
			})
		})
	})

	for i := range schemas {
		schema := &schemas[i]
		var extra []string
		for _, u := range undescribed {
			if u.namespace.uri == schema.namespace.uri {
				extra = append(extra, u.name)
			}
		}
		g.schema(schema, extra)
	}

	return g.finish()
}

// grammar is Grammar in krilla src/interchange/metadata/relax_ng.rs.
type grammar struct {
	w                   *xmlwriter.XMLWriter
	simpleTypes         []SimpleType
	structTypes         []*StructType
	languageAlternative bool
	any                 bool
	ui                  string
}

// element is Grammar::element in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) element(name string, f func(g *grammar)) {
	g.w.StartElement(name)
	f(g)
	g.w.EndElement()
}

// define is Grammar::define in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) define(name string, f func(g *grammar)) {
	g.element("define", func(g *grammar) {
		g.w.WriteAttribute("name", rngEscape(name))
		g.w.WriteAttribute("combine", "choice")
		f(g)
	})
}

// reference is Grammar::reference in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) reference(name string) {
	g.element("ref", func(g *grammar) { g.w.WriteAttribute("name", rngEscape(name)) })
}

// interleave is Grammar::interleave in krilla
// src/interchange/metadata/relax_ng.rs.
//
// An interleave of the n patterns written by f, or empty if there are
// none.
func (g *grammar) interleave(n int, f func(g *grammar)) {
	if n == 0 {
		g.element("empty", func(*grammar) {})
	} else {
		g.element("interleave", f)
	}
}

// optional is Grammar::optional in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) optional(optional bool, f func(g *grammar)) {
	if optional {
		g.element("optional", f)
	} else {
		f(g)
	}
}

// resource is Grammar::resource in krilla
// src/interchange/metadata/relax_ng.rs.
//
// An rdf:resource attribute, the serialization of URI values (XMP
// Specification Part 1, section 7.5).
func (g *grammar) resource(f func(g *grammar)) {
	g.element("attribute", func(g *grammar) {
		g.w.WriteAttribute("name", "rdf:resource")
		f(g)
	})
}

// anyAttribute is Grammar::any_attribute in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) anyAttribute() {
	g.element("attribute", func(g *grammar) { g.element("anyName", func(*grammar) {}) })
}

// anyProperty is Grammar::any_property in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) anyProperty(prefix, name string) {
	g.any = true
	g.optional(true, func(g *grammar) {
		g.element("element", func(g *grammar) {
			g.w.WriteAttribute("name", rngEscape(prefix+":"+name))
			g.reference("Any")
		})
	})
}

// uiInfo is Grammar::ui_info in krilla
// src/interchange/metadata/relax_ng.rs. A nil description is None.
func (g *grammar) uiInfo(label string, description *string) {
	g.element(g.ui+":label", func(g *grammar) {
		g.w.WriteAttribute("xml:lang", "x-default")
		g.w.WriteAttribute("value", rngEscape(label))
	})
	if description != nil {
		g.element(g.ui+":description", func(g *grammar) {
			g.w.WriteAttribute("xml:lang", "x-default")
			g.w.WriteAttribute("value", rngEscape(*description))
		})
	}
}

// schema is Grammar::schema in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) schema(schema *Schema, undescribed []string) {
	prefix := rngPrefix(schema.namespace)

	g.define(schemaDefine(schema.namespace), func(g *grammar) {
		g.element(g.ui+":schema", func(g *grammar) {
			label := prefix
			if schema.name != nil {
				label = *schema.name
			}
			g.uiInfo(label, schema.description)
		})
		g.interleave(len(schema.properties)+len(undescribed), func(g *grammar) {
			for i := range schema.properties {
				desc := &schema.properties[i]
				g.optional(!desc.required, func(g *grammar) {
					g.reference(prefix + "." + desc.name)
				})
			}
			for _, name := range undescribed {
				g.anyProperty(prefix, name)
			}
		})
	})

	for i := range schema.properties {
		desc := &schema.properties[i]
		g.define(prefix+"."+desc.name, func(g *grammar) {
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", rngEscape(prefix+":"+desc.name))
				g.valueType(&desc.valueType)
				g.element(g.ui+":property", func(g *grammar) {
					g.uiInfo(desc.name, &desc.description)
				})
			})
		})
	}
}

// valueType is Grammar::value_type in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) valueType(valueType *ValueType) {
	switch valueType.kind {
	case valueTypeSimple, valueTypeOpenChoice:
		ty := valueType.simple
		if !slices.Contains(g.simpleTypes, ty) {
			g.simpleTypes = append(g.simpleTypes, ty)
		}
		g.reference("ISO16684-1.Types.QValue." + ty.String())
	case valueTypeClosedChoice:
		choice := func(g *grammar) {
			g.element("choice", func(g *grammar) {
				for _, value := range valueType.choices {
					g.element("value", func(g *grammar) { g.w.WriteText(rngEscape(value)) })
				}
			})
		}
		if isURIType(valueType.simple) {
			g.resource(choice)
		} else {
			choice(g)
		}
	case valueTypeOrderedArray:
		g.array("rdf:Seq", valueType.item)
	case valueTypeUnorderedArray:
		g.array("rdf:Bag", valueType.item)
	case valueTypeAlternativeArray:
		g.array("rdf:Alt", valueType.item)
	case valueTypeLanguageAlternative:
		g.languageAlternative = true
		g.reference("ISO16684-1.Types.QValue.LanguageAlternative")
	case valueTypePredefinedStruct:
		if ty := predefinedStructType(valueType.predefined); ty != nil {
			g.structType(ty)
		} else {
			g.any = true
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:Description")
				g.reference("Any")
			})
		}
	case valueTypeStruct:
		g.structType(valueType.structType)
	}
}

// array is Grammar::array in krilla src/interchange/metadata/relax_ng.rs.
func (g *grammar) array(kind string, item *ValueType) {
	g.element("element", func(g *grammar) {
		g.w.WriteAttribute("name", kind)
		g.element("zeroOrMore", func(g *grammar) {
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:li")
				g.valueType(item)
			})
		})
	})
}

// structType is Grammar::struct_type in krilla
// src/interchange/metadata/relax_ng.rs.
//
// References the structure type, whose definition is written in finish.
func (g *grammar) structType(ty *StructType) {
	if !slices.ContainsFunc(g.structTypes, ty.equal) {
		g.structTypes = append(g.structTypes, ty)
	}
	g.reference(structDefine(ty) + ".QValue")
}

// qualified is Grammar::qualified in krilla
// src/interchange/metadata/relax_ng.rs.
//
// The base type name, or the same value in an rdf:Description with
// qualifiers.
func (g *grammar) qualified(name string) {
	g.element("choice", func(g *grammar) {
		g.reference(name)
		g.element("element", func(g *grammar) {
			g.w.WriteAttribute("name", "rdf:Description")
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:value")
				g.reference(name)
			})
			g.element("zeroOrMore", func(g *grammar) {
				g.element("element", func(g *grammar) {
					g.element("anyName", func(*grammar) {})
					g.element("text", func(*grammar) {})
				})
			})
		})
	})
}

// finish is Grammar::finish in krilla
// src/interchange/metadata/relax_ng.rs.
func (g *grammar) finish() string {
	// Writing a structure type can discover further ones.
	for i := 0; i < len(g.structTypes); i++ { //nolint:intrange // the loop grows the slice
		ty := g.structTypes[i]
		name := structDefine(ty)
		base := name + ".Base"
		prefix := rngPrefix(ty.namespace)

		g.define(name+".QValue", func(g *grammar) { g.qualified(base) })
		g.define(base, func(g *grammar) {
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:Description")
				g.interleave(len(ty.fields), func(g *grammar) {
					for j := range ty.fields {
						field := &ty.fields[j]
						g.optional(!field.required, func(g *grammar) {
							g.element("element", func(g *grammar) {
								g.w.WriteAttribute("name", rngEscape(prefix+":"+field.name))
								g.valueType(&field.valueType)
							})
						})
					}
				})
			})
		})
	}

	simpleTypes := g.simpleTypes
	g.simpleTypes = nil
	for _, ty := range simpleTypes {
		base := "ISO16684-1.Types.Base." + ty.String()
		g.define("ISO16684-1.Types.QValue."+ty.String(), func(g *grammar) { g.qualified(base) })
		g.define(base, func(g *grammar) {
			dataType, pattern, ok := simpleTypeData(ty)
			switch {
			case ok:
				g.element("data", func(g *grammar) {
					g.w.WriteAttribute("type", dataType)
					if pattern != "" {
						g.element("param", func(g *grammar) {
							g.w.WriteAttribute("name", "pattern")
							g.w.WriteText(pattern)
						})
					}
				})
			case isURIType(ty):
				g.resource(func(g *grammar) { g.element("text", func(*grammar) {}) })
			default:
				g.element("text", func(*grammar) {})
			}
		})
	}

	if g.languageAlternative {
		g.define("ISO16684-1.Types.QValue.LanguageAlternative", func(g *grammar) {
			g.arrayOfLanguageTaggedText()
		})
	}

	if g.any {
		g.define("Any", func(g *grammar) {
			g.element("zeroOrMore", func(g *grammar) {
				g.element("choice", func(g *grammar) {
					g.anyAttribute()
					g.element("text", func(*grammar) {})
					g.element("element", func(g *grammar) {
						g.element("anyName", func(*grammar) {})
						g.reference("Any")
					})
				})
			})
		})
	}

	return g.w.EndDocument()
}

// arrayOfLanguageTaggedText is Grammar::array_of_language_tagged_text in
// krilla src/interchange/metadata/relax_ng.rs.
func (g *grammar) arrayOfLanguageTaggedText() {
	g.element("element", func(g *grammar) {
		g.w.WriteAttribute("name", "rdf:Alt")
		g.element("zeroOrMore", func(g *grammar) {
			g.element("element", func(g *grammar) {
				g.w.WriteAttribute("name", "rdf:li")
				g.element("attribute", func(g *grammar) {
					g.w.WriteAttribute("name", "xml:lang")
					g.element("data", func(g *grammar) {
						g.w.WriteAttribute("type", "string")
						g.element("param", func(g *grammar) {
							g.w.WriteAttribute("name", "pattern")
							g.w.WriteText("x-default|([a-zA-Z]+(-[a-zA-Z0-9]+)*)")
						})
					})
				})
				g.element("text", func(*grammar) {})
			})
		})
	})
}

// uiPrefix is ui_prefix in krilla src/interchange/metadata/relax_ng.rs.
func uiPrefix(namespaces []rngNamespace) string {
	for i := 0; ; i++ {
		ui := "ui"
		if i != 0 {
			ui += strconv.Itoa(i)
		}
		if !slices.ContainsFunc(namespaces, func(ns rngNamespace) bool { return ns.prefix == ui }) {
			return ui
		}
	}
}

// rngPrefix is prefix in krilla src/interchange/metadata/relax_ng.rs.
func rngPrefix(ns Namespace) string {
	return buildXMPNamespace(ns).Prefix()
}

// addNamespace is add_namespace in krilla
// src/interchange/metadata/relax_ng.rs.
func addNamespace(namespaces *[]rngNamespace, ns Namespace) {
	if !slices.ContainsFunc(*namespaces, func(n rngNamespace) bool { return n.uri == ns.uri }) {
		*namespaces = append(*namespaces, rngNamespace{prefix: rngPrefix(ns), uri: ns.uri})
	}
}

// addValueTypeNamespaces is add_value_type_namespaces in krilla
// src/interchange/metadata/relax_ng.rs.
func addValueTypeNamespaces(namespaces *[]rngNamespace, ty *ValueType) {
	var structType *StructType
	switch ty.kind {
	case valueTypeOrderedArray, valueTypeUnorderedArray, valueTypeAlternativeArray:
		addValueTypeNamespaces(namespaces, ty.item)
		return
	case valueTypePredefinedStruct:
		structType = predefinedStructType(ty.predefined)
	case valueTypeStruct:
		structType = ty.structType
	}

	if structType != nil &&
		!slices.ContainsFunc(*namespaces, func(n rngNamespace) bool { return n.uri == structType.namespace.uri }) {
		addNamespace(namespaces, structType.namespace)
		for i := range structType.fields {
			addValueTypeNamespaces(namespaces, &structType.fields[i].valueType)
		}
	}
}

// schemaDefine is schema_define in krilla
// src/interchange/metadata/relax_ng.rs.
func schemaDefine(ns Namespace) string {
	return "XMP_Properties-" + rngPrefix(ns)
}

// structDefine is struct_define in krilla
// src/interchange/metadata/relax_ng.rs.
func structDefine(ty *StructType) string {
	return rngPrefix(ty.namespace) + ".Types." + ty.name
}

// isURIType is is_uri in krilla src/interchange/metadata/relax_ng.rs.
func isURIType(ty SimpleType) bool {
	return ty == SimpleTypeURI || ty == SimpleTypeURL
}

// simpleTypeData is simple_type_data in krilla
// src/interchange/metadata/relax_ng.rs. It returns false where upstream
// returns None, and an empty pattern where upstream has no pattern.
//
// The XML Schema datatype and pattern of a simple type, or none for
// unconstrained text.
func simpleTypeData(ty SimpleType) (dataType, pattern string, ok bool) {
	switch ty {
	case SimpleTypeBoolean:
		return "string", "True|False", true
	case SimpleTypeInteger:
		return "integer", "", true
	case SimpleTypeReal:
		return "double", "", true
	case SimpleTypeRational:
		return "string", `-?\d+/[1-9]\d*`, true
	case SimpleTypeDate:
		return "string", `(\d{4}((-)?(0[1-9]|1[0-2])((-)?(0[1-9]|[1-2][0-9]|3[0-1])(T(24:00(:00(\.[0]+)?)?|` +
			`(([0-1][0-9]|2[0-3])(:)[0-5][0-9])((:)[0-5][0-9](\.[\d]+)?)?)((\+|-)(14:00|` +
			`(0[0-9]|1[0-3])(:)[0-5][0-9])|Z)?)?)?)?)`, true
	case SimpleTypeLocale:
		return "string", "[a-zA-Z]+(-[a-zA-Z0-9]+)*", true
	case SimpleTypeMIMEType:
		return "string", "(application|audio|example|image|message|model|multipart|text|video)" +
			`/[a-zA-Z0-9]+([+\-.][a-zA-Z0-9]+)*`, true
	}
	return "", "", false
}

// rngEscape is escape in krilla src/interchange/metadata/relax_ng.rs.
//
// Escapes what xmlwriter doesn't: it only escapes quotes in attribute
// values and < in text.
func rngEscape(text string) string {
	if !strings.ContainsAny(text, "&<") {
		return text
	}
	return strings.ReplaceAll(strings.ReplaceAll(text, "&", "&amp;"), "<", "&lt;")
}
