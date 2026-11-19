//! The metadata, the outline, the named destinations and the links of a
//! scene of the question "krilla" (phase 16, unit 6). `test/krilla/doc.go`
//! documents the fields. The module `scene` reads them with the scene, and
//! calls the functions of this module.

use krilla::annotation::{Annotation, LinkAnnotation, LinkBorder, Target};
use krilla::color::{cmyk, luma, rgb, Color};
use krilla::destination::{Destination, NamedDestination, XyzDestination};
use krilla::geom::{Point, Quadrilateral, Rect};
use krilla::metadata::{DateTime, Metadata, PageLayout, TextDirection};
use krilla::outline::{Outline, OutlineNode};
use krilla::page::Page;
use krilla::action::{Action, LinkAction};
use krilla::Document;
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
    /// Pairs of a name and a value, for Metadata::custom_field in their
    /// order. A field that krilla does not accept is left out.
    custom_fields: Vec<(String, String)>,
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
    for (name, value) in &s.custom_fields {
        if let Some(next) = m.clone().custom_field(name.clone(), value.clone()) {
            m = next;
        }
    }
    Ok(m)
}

/// finish_document sets the metadata and the outline of a scene, after its
/// pages.
pub fn finish_document(doc: &mut Document, s: &DocSpec) -> Result<(), String> {
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
