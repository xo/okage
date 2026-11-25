//! The metadata, the outline, the named destinations and the links of a
//! scene of the question "krilla" (phase 16, unit 6). `test/krilla/doc.go`
//! documents the fields. The module `scene` reads them with the scene, and
//! calls the functions of this module.

use krilla::annotation::{Annotation, LinkAnnotation, LinkBorder, Target};
use krilla::color::{cmyk, luma, rgb, Color};
use krilla::destination::{Destination, NamedDestination, XyzDestination};
use krilla::geom::{Point, Quadrilateral, Rect};
use krilla::embed::{AssociationKind, EmbeddedFile, MimeType};
use krilla::metadata::xmp::{
    Namespace, PredefinedStruct, Property, PropertyDescription, Schema, SimpleType, StructType,
    Value, ValueType,
};
use krilla::metadata::{DateTime, Metadata, PageLayout, TextDirection};
use krilla::outline::{Outline, OutlineNode};
use krilla::page::Page;
use krilla::action::{Action, LinkAction};
use krilla::Document;
use base64::Engine;
use serde::Deserialize;

/// The fields of a scene for the metadata, the outline and the named
/// destinations. Each field is optional.
#[derive(Deserialize, Default)]
#[serde(default)]
pub struct DocSpec {
    metadata: Option<MetadataSpec>,
    /// With this field, even empty, the oracle calls Document::set_outline.
    outline: Option<Vec<NodeSpec>>,
    /// The destinations of Document::register_named_destination, which the
    /// oracle registers before the pages.
    named_destinations: Vec<NamedSpec>,
    /// The files of Document::embed_file, which the oracle embeds after the
    /// pages, before the metadata.
    embedded_files: Vec<EmbedSpec>,
}

/// An embedded file (D99).
#[derive(Deserialize)]
struct EmbedSpec {
    path: String,
    #[serde(default)]
    mime: Option<String>,
    #[serde(default)]
    description: Option<String>,
    /// The name of a case of AssociationKind, such as "Alternative".
    kind: String,
    /// The bytes of the file in base64.
    data: String,
    #[serde(default)]
    date: Option<DateSpec>,
    #[serde(default)]
    compress: Option<bool>,
}

/// A custom XMP property: the prefix and the URI of its namespace, its
/// name and its value (D99).
#[derive(Deserialize)]
struct PropertySpec {
    ns: (String, String),
    name: String,
    value: ValueSpec,
}

/// A value of Value.
#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum ValueSpec {
    Text(String),
    Uri(String),
    Bool(bool),
    Int(i64),
    Real(f64),
    Rational(i64, u64),
    Date(DateSpec),
    Seq(Vec<ValueSpec>),
    Bag(Vec<ValueSpec>),
    Alt(Vec<ValueSpec>),
    Lang(Vec<(Option<String>, String)>),
    Struct(Vec<PropertySpec>),
}

/// A value type of ValueType. A simple type is the name of its case of
/// SimpleType, such as "Text" or "GUID", and a predefined structure type
/// the name of its case of PredefinedStruct.
#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum TypeSpec {
    Simple(String),
    Open(String, Vec<String>),
    Closed(String, Vec<String>),
    Seq(Box<TypeSpec>),
    Bag(Box<TypeSpec>),
    Alt(Box<TypeSpec>),
    Lang,
    Predefined(String),
    Struct(StructSpec),
}

/// A user-defined structure type.
#[derive(Deserialize)]
struct StructSpec {
    ns: (String, String),
    name: String,
    #[serde(default)]
    description: String,
    #[serde(default)]
    fields: Vec<DescSpec>,
}

/// A description of a property or a field.
#[derive(Deserialize)]
struct DescSpec {
    name: String,
    #[serde(rename = "type")]
    ty: TypeSpec,
    #[serde(default)]
    description: String,
    #[serde(default)]
    internal: bool,
    #[serde(default)]
    required: bool,
}

/// A schema of a custom namespace.
#[derive(Deserialize)]
struct SchemaSpec {
    ns: (String, String),
    #[serde(default)]
    name: Option<String>,
    #[serde(default)]
    description: Option<String>,
    #[serde(default)]
    properties: Vec<DescSpec>,
}

#[derive(Deserialize, Default)]
#[serde(default)]
struct MetadataSpec {
    title: Option<String>,
    description: Option<String>,
    creator: Option<String>,
    producer: Option<String>,
    keywords: Option<Vec<String>>,
    authors: Option<Vec<String>>,
    document_id: Option<String>,
    version_id: Option<String>,
    language: Option<String>,
    creation_date: Option<DateSpec>,
    /// "ltr" or "rtl".
    text_direction: Option<String>,
    /// The name of a case of PageLayout, such as "TwoColumnRight".
    page_layout: Option<String>,
    display_doc_title: Option<bool>,
    trapped: Option<bool>,
    /// Pairs of a name and a value, for Metadata::custom_field in their
    /// order. A field that krilla does not accept is left out.
    custom_fields: Vec<(String, String)>,
    /// The properties of Metadata::custom_xmp_property, in their order.
    custom_xmp_properties: Vec<PropertySpec>,
    /// The schemas of Metadata::custom_xmp_schema, in their order.
    custom_xmp_schemas: Vec<SchemaSpec>,
}

#[derive(Deserialize, Default)]
#[serde(default)]
struct DateSpec {
    year: u16,
    month: Option<u8>,
    day: Option<u8>,
    hour: Option<u8>,
    minute: Option<u8>,
    second: Option<u8>,
    utc_offset_hour: Option<i8>,
    utc_offset_minute: Option<u8>,
}

/// A node of the outline.
#[derive(Deserialize)]
struct NodeSpec {
    text: String,
    dest: XyzSpec,
    #[serde(default)]
    open: bool,
    #[serde(default)]
    children: Vec<NodeSpec>,
}

#[derive(Deserialize, Clone)]
struct XyzSpec {
    page: usize,
    #[serde(default)]
    x: f32,
    #[serde(default)]
    y: f32,
}

#[derive(Deserialize, Clone)]
struct NamedSpec {
    name: String,
    #[serde(flatten)]
    xyz: XyzSpec,
}

/// A destination of a link or of a go-to action.
#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum DestSpec {
    Xyz(XyzSpec),
    Named(NamedSpec),
}

/// The target of a link: a destination, a URI or a go-to action.
#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum TargetSpec {
    Xyz(XyzSpec),
    Named(NamedSpec),
    Link(String),
    Goto(DestSpec),
}

#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum ColorSpec {
    Rgb([u8; 3]),
    Luma(u8),
    Cmyk([u8; 4]),
}

#[derive(Deserialize)]
struct BorderSpec {
    width: f32,
    color: ColorSpec,
}

/// A link annotation of a page.
#[derive(Deserialize)]
pub struct AnnotationSpec {
    /// The rectangle [x, y, w, h] of LinkAnnotation::new.
    #[serde(default)]
    rect: Option<[f32; 4]>,
    /// The quadrilaterals of LinkAnnotation::new_with_quad_points, each with
    /// the x and the y of its four points.
    #[serde(default)]
    quad_points: Option<Vec<[f32; 8]>>,
    target: TargetSpec,
    #[serde(default)]
    border: Option<BorderSpec>,
    /// The alt text of Annotation::new_link.
    #[serde(default)]
    alt: Option<String>,
}

fn xyz(s: &XyzSpec) -> XyzDestination {
    XyzDestination::new(s.page, Point::from_xy(s.x, s.y))
}

fn named(s: &NamedSpec) -> NamedDestination {
    NamedDestination::new(s.name.clone(), xyz(&s.xyz))
}

fn dest(s: &DestSpec) -> Destination {
    match s {
        DestSpec::Xyz(x) => xyz(x).into(),
        DestSpec::Named(n) => named(n).into(),
    }
}

fn target(s: &TargetSpec) -> Target {
    match s {
        TargetSpec::Xyz(x) => Target::Destination(xyz(x).into()),
        TargetSpec::Named(n) => Target::Destination(named(n).into()),
        TargetSpec::Link(uri) => Target::Action(LinkAction::new(uri.clone()).into()),
        TargetSpec::Goto(d) => Target::Action(Action::Goto(dest(d))),
    }
}

fn color(c: &ColorSpec) -> Color {
    match c {
        ColorSpec::Rgb(c) => rgb::Color::new(c[0], c[1], c[2]).into(),
        ColorSpec::Luma(l) => luma::Color::new(*l).into(),
        ColorSpec::Cmyk(c) => cmyk::Color::new(c[0], c[1], c[2], c[3]).into(),
    }
}

/// register_named_destinations registers the named destinations of a scene,
/// before its pages.
pub fn register_named_destinations(doc: &mut Document, s: &DocSpec) {
    for n in &s.named_destinations {
        let _ = doc.register_named_destination(named(n));
    }
}

/// add_annotations adds the link annotations of a page, before its surface.
pub fn add_annotations(page: &mut Page, annotations: &[AnnotationSpec]) -> Result<(), String> {
    for a in annotations {
        let mut link = match (&a.rect, &a.quad_points) {
            (Some(r), None) => LinkAnnotation::new(
                Rect::from_xywh(r[0], r[1], r[2], r[3])
                    .ok_or_else(|| format!("the rectangle {r:?} is not valid"))?,
                target(&a.target),
            ),
            (None, Some(q)) => LinkAnnotation::new_with_quad_points(
                q.iter()
                    .map(|v| {
                        Quadrilateral([
                            Point::from_xy(v[0], v[1]),
                            Point::from_xy(v[2], v[3]),
                            Point::from_xy(v[4], v[5]),
                            Point::from_xy(v[6], v[7]),
                        ])
                    })
                    .collect(),
                target(&a.target),
            ),
            _ => return Err("an annotation needs a rect or quad_points".to_string()),
        };
        if let Some(b) = &a.border {
            link = link.with_border(LinkBorder::new(b.width, color(&b.color)));
        }
        let annotation = match &a.alt {
            Some(alt) => Annotation::new_link(link, Some(alt.clone())),
            None => link.into(),
        };
        page.add_annotation(annotation);
    }
    Ok(())
}

fn node(s: &NodeSpec) -> OutlineNode {
    let mut n = OutlineNode::new(s.text.clone(), xyz(&s.dest)).with_open(s.open);
    for c in &s.children {
        n.push_child(node(c));
    }
    n
}

fn date(s: &DateSpec) -> DateTime {
    let mut d = DateTime::new(s.year);
    if let Some(v) = s.month {
        d = d.month(v);
    }
    if let Some(v) = s.day {
        d = d.day(v);
    }
    if let Some(v) = s.hour {
        d = d.hour(v);
    }
    if let Some(v) = s.minute {
        d = d.minute(v);
    }
    if let Some(v) = s.second {
        d = d.second(v);
    }
    if let Some(v) = s.utc_offset_hour {
        d = d.utc_offset_hour(v);
    }
    if let Some(v) = s.utc_offset_minute {
        d = d.utc_offset_minute(v);
    }
    d
}

fn metadata(s: &MetadataSpec) -> Result<Metadata, String> {
    let mut m = Metadata::new();
    if let Some(v) = &s.title {
        m = m.title(v.clone());
    }
    if let Some(v) = &s.description {
        m = m.description(v.clone());
    }
    if let Some(v) = &s.creator {
        m = m.creator(v.clone());
    }
    if let Some(v) = &s.producer {
        m = m.producer(v.clone());
    }
    if let Some(v) = &s.keywords {
        m = m.keywords(v.clone());
    }
    if let Some(v) = &s.authors {
        m = m.authors(v.clone());
    }
    if let Some(v) = &s.document_id {
        m = m.document_id(v.clone());
    }
    if let Some(v) = &s.version_id {
        m = m.version_id(v.clone());
    }
    if let Some(v) = &s.language {
        m = m.language(v.clone());
    }
    if let Some(v) = &s.creation_date {
        m = m.creation_date(date(v));
    }
    if let Some(v) = &s.text_direction {
        m = m.text_direction(match v.as_str() {
            "ltr" => TextDirection::LeftToRight,
            "rtl" => TextDirection::RightToLeft,
            _ => return Err(format!("the text direction {v:?} is not known")),
        });
    }
    if let Some(v) = &s.page_layout {
        m = m.page_layout(match v.as_str() {
            "SinglePage" => PageLayout::SinglePage,
            "OneColumn" => PageLayout::OneColumn,
            "TwoColumnLeft" => PageLayout::TwoColumnLeft,
            "TwoColumnRight" => PageLayout::TwoColumnRight,
            "TwoPageLeft" => PageLayout::TwoPageLeft,
            "TwoPageRight" => PageLayout::TwoPageRight,
            _ => return Err(format!("the page layout {v:?} is not known")),
        });
    }
    if let Some(v) = s.display_doc_title {
        m = m.display_doc_title(v);
    }
    if let Some(v) = s.trapped {
        m = m.trapped(v);
    }
    for (name, value) in &s.custom_fields {
        if let Some(next) = m.clone().custom_field(name.clone(), value.clone()) {
            m = next;
        }
    }
    for p in &s.custom_xmp_properties {
        m = m.custom_xmp_property(property(p)?);
    }
    for sc in &s.custom_xmp_schemas {
        let mut schema = Schema::new(namespace(&sc.ns));
        if let Some(n) = &sc.name {
            schema = schema.name(n.clone());
        }
        if let Some(d) = &sc.description {
            schema = schema.description(d.clone());
        }
        for d in &sc.properties {
            schema = schema.property(description(d)?);
        }
        m = m.custom_xmp_schema(schema);
    }
    Ok(m)
}

fn namespace(ns: &(String, String)) -> Namespace {
    Namespace::new(ns.0.clone(), ns.1.clone())
}

fn property(p: &PropertySpec) -> Result<Property, String> {
    Ok(Property::new(namespace(&p.ns), p.name.clone(), value(&p.value)?))
}

fn value(v: &ValueSpec) -> Result<Value, String> {
    let list = |items: &[ValueSpec]| items.iter().map(value).collect::<Result<Vec<_>, _>>();
    Ok(match v {
        ValueSpec::Text(s) => Value::Text(s.clone()),
        ValueSpec::Uri(s) => Value::Uri(s.clone()),
        ValueSpec::Bool(b) => Value::Bool(*b),
        ValueSpec::Int(i) => Value::Integer(*i),
        ValueSpec::Real(r) => Value::Real(*r),
        ValueSpec::Rational(n, d) => Value::Rational(*n, *d),
        ValueSpec::Date(d) => Value::Date(date(d)),
        ValueSpec::Seq(items) => Value::OrderedArray(list(items)?),
        ValueSpec::Bag(items) => Value::UnorderedArray(list(items)?),
        ValueSpec::Alt(items) => Value::AlternativeArray(list(items)?),
        ValueSpec::Lang(items) => Value::LanguageAlternative(items.clone()),
        ValueSpec::Struct(fields) => {
            Value::Struct(fields.iter().map(property).collect::<Result<Vec<_>, _>>()?)
        }
    })
}

fn simple_type(name: &str) -> Result<SimpleType, String> {
    Ok(match name {
        "Text" => SimpleType::Text,
        "Boolean" => SimpleType::Boolean,
        "Integer" => SimpleType::Integer,
        "Real" => SimpleType::Real,
        "Date" => SimpleType::Date,
        "AgentName" => SimpleType::AgentName,
        "GUID" => SimpleType::Guid,
        "Locale" => SimpleType::Locale,
        "MIMEType" => SimpleType::MimeType,
        "ProperName" => SimpleType::ProperName,
        "RenditionClass" => SimpleType::RenditionClass,
        "URI" => SimpleType::Uri,
        "URL" => SimpleType::Url,
        "Rational" => SimpleType::Rational,
        "FrameRate" => SimpleType::FrameRate,
        "FrameCount" => SimpleType::FrameCount,
        "Part" => SimpleType::Part,
        "XPath" => SimpleType::XPath,
        _ => return Err(format!("the simple type {name:?} is not known")),
    })
}

fn value_type(t: &TypeSpec) -> Result<ValueType, String> {
    Ok(match t {
        TypeSpec::Simple(n) => ValueType::Simple(simple_type(n)?),
        TypeSpec::Open(n, v) => ValueType::open_choice(simple_type(n)?, v.clone()),
        TypeSpec::Closed(n, v) => ValueType::closed_choice(simple_type(n)?, v.clone()),
        TypeSpec::Seq(item) => ValueType::ordered_array(value_type(item)?),
        TypeSpec::Bag(item) => ValueType::unordered_array(value_type(item)?),
        TypeSpec::Alt(item) => ValueType::alternative_array(value_type(item)?),
        TypeSpec::Lang => ValueType::LanguageAlternative,
        TypeSpec::Predefined(n) => ValueType::PredefinedStruct(match n.as_str() {
            "Dimensions" => PredefinedStruct::Dimensions,
            "Thumbnail" => PredefinedStruct::Thumbnail,
            "ResourceEvent" => PredefinedStruct::ResourceEvent,
            "ResourceRef" => PredefinedStruct::ResourceRef,
            "Version" => PredefinedStruct::Version,
            "Job" => PredefinedStruct::Job,
            "Colorant" => PredefinedStruct::Colorant,
            "Font" => PredefinedStruct::Font,
            _ => return Err(format!("the predefined structure {n:?} is not known")),
        }),
        TypeSpec::Struct(st) => {
            let mut ty = StructType::new(namespace(&st.ns), st.name.clone(), st.description.clone());
            for f in &st.fields {
                ty = ty.field(description(f)?);
            }
            ty.into()
        }
    })
}

fn description(d: &DescSpec) -> Result<PropertyDescription, String> {
    let mut desc = PropertyDescription::new(d.name.clone(), value_type(&d.ty)?, d.description.clone());
    if d.internal {
        desc = desc.internal();
    }
    if d.required {
        desc = desc.required();
    }
    Ok(desc)
}

fn embedded_file(e: &EmbedSpec) -> Result<EmbeddedFile, String> {
    let mime_type = match &e.mime {
        Some(m) => Some(MimeType::new(m).ok_or_else(|| format!("the mime type {m:?} is not valid"))?),
        None => None,
    };
    let association_kind = match e.kind.as_str() {
        "Source" => AssociationKind::Source,
        "Data" => AssociationKind::Data,
        "Alternative" => AssociationKind::Alternative,
        "Supplement" => AssociationKind::Supplement,
        "Schema" => AssociationKind::Schema,
        "Unspecified" => AssociationKind::Unspecified,
        k => return Err(format!("the association kind {k:?} is not known")),
    };
    let data = base64::engine::general_purpose::STANDARD
        .decode(&e.data)
        .map_err(|err| format!("the data of {}: {err}", e.path))?;
    Ok(EmbeddedFile {
        path: e.path.clone(),
        mime_type,
        description: e.description.clone(),
        association_kind,
        data: data.into(),
        modification_date: e.date.as_ref().map(date),
        compress: e.compress,
        location: None,
    })
}

/// finish_document embeds the files, then sets the metadata and the outline
/// of a scene, after its pages.
pub fn finish_document(doc: &mut Document, s: &DocSpec) -> Result<(), String> {
    for e in &s.embedded_files {
        let _ = doc.embed_file(embedded_file(e)?);
    }
    if let Some(m) = &s.metadata {
        doc.set_metadata(metadata(m)?);
    }
    if let Some(nodes) = &s.outline {
        let mut outline = Outline::new();
        for n in nodes {
            outline.push_child(node(n));
        }
        doc.set_outline(outline);
    }
    Ok(())
}
