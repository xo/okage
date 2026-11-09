//! The questions of the parsers of phases 6 and 7: roxmltree, svgtypes and
//! usvg.
//!
//! An XML question names a file. The oracle parses it with roxmltree 0.21.1
//! and the options of usvg, and answers with the tree, written as the test
//! `tests/ast.rs` of roxmltree writes it. If the parse fails, it answers with
//! the error, also as that test writes it.
//!
//! An svgtypes question names a parser and a value. The oracle parses the
//! value with svgtypes 0.16.1 and answers with one line for each result. Each
//! f64 is written as the 16 hex digits of its bits, so that the Go test
//! compares every bit, and each string as `x` and the hex digits of its
//! bytes. An error is written as `err`, and a panic of svgtypes as `panic`.

use std::fmt::{self, Write};

use serde::Deserialize;

#[derive(Deserialize)]
pub struct XmlQuestion {
    pub xml: String,
}

#[derive(Deserialize)]
pub struct SvgtypesQuestion {
    pub svgtypes: String,
    pub value: String,
}

/// answer_xml parses the file of the question with roxmltree.
pub fn answer_xml(q: &XmlQuestion) -> Option<Vec<u8>> {
    let text = std::fs::read_to_string(&q.xml).ok()?;
    let opt = roxmltree::ParsingOptions {
        allow_dtd: true,
        ..roxmltree::ParsingOptions::default()
    };
    let s = match roxmltree::Document::parse_with_options(&text, opt) {
        Ok(doc) => to_yaml(&doc),
        Err(e) => format!("error: \"{}\"", e),
    };
    Some(s.into_bytes())
}

// to_yaml is to_yaml of roxmltree tests/ast.rs, at the base commit 67644e16.
fn to_yaml(doc: &roxmltree::Document) -> String {
    let mut s = String::new();
    to_yaml_inner(doc, &mut s).unwrap();
    s
}

fn to_yaml_inner(doc: &roxmltree::Document, s: &mut String) -> Result<(), fmt::Error> {
    use roxmltree::{Node, NodeType};

    if !doc.root().has_children() {
        return write!(s, "Document:");
    }

    fn indent(depth: usize, s: &mut String) -> fmt::Result {
        for _ in 0..depth {
            write!(s, "  ")?;
        }
        Ok(())
    }

    fn print_children(parent: Node, depth: usize, s: &mut String) -> fmt::Result {
        for child in parent.children() {
            match child.node_type() {
                NodeType::Element => {
                    indent(depth, s)?;
                    writeln!(s, "- Element:")?;
                    indent(depth + 2, s)?;
                    match child.tag_name().namespace() {
                        Some(ns) if !ns.is_empty() => {
                            writeln!(s, "tag_name: {}@{}", child.tag_name().name(), ns)?
                        }
                        _ => writeln!(s, "tag_name: {}", child.tag_name().name())?,
                    }

                    let attributes = child.attributes();
                    if attributes.len() != 0 {
                        let mut attrs = Vec::new();
                        for attr in attributes {
                            match attr.namespace() {
                                Some(ns) => {
                                    attrs.push((format!("{}@{}", attr.name(), ns), attr.value()))
                                }
                                None => attrs.push((attr.name().to_string(), attr.value())),
                            }
                        }
                        attrs.sort_by(|a, b| a.0.cmp(&b.0));
                        indent(depth + 2, s)?;
                        writeln!(s, "attributes:")?;
                        for (name, value) in attrs {
                            indent(depth + 3, s)?;
                            writeln!(s, "{}: {:?}", name, value)?;
                        }
                    }

                    let namespaces = child.namespaces();
                    if namespaces.len() != 0 {
                        let mut ns_list = Vec::new();
                        for ns in namespaces {
                            let name = ns.name().unwrap_or("None");
                            let uri = if ns.uri().is_empty() { "\"\"" } else { ns.uri() };
                            ns_list.push((name, uri));
                        }
                        ns_list.sort_by(|a, b| a.0.cmp(b.0));
                        indent(depth + 2, s)?;
                        writeln!(s, "namespaces:")?;
                        for (name, uri) in ns_list {
                            indent(depth + 3, s)?;
                            writeln!(s, "{}: {}", name, uri)?;
                        }
                    }

                    if child.has_children() {
                        indent(depth + 2, s)?;
                        writeln!(s, "children:")?;
                        print_children(child, depth + 3, s)?;
                    }
                }
                NodeType::Text => {
                    indent(depth, s)?;
                    writeln!(s, "- Text: {:?}", child.text().unwrap())?;
                }
                NodeType::Comment => {
                    indent(depth, s)?;
                    writeln!(s, "- Comment: {:?}", child.text().unwrap())?;
                }
                NodeType::PI => {
                    if child.parent().unwrap().is_root() {
                        continue;
                    }
                    indent(depth, s)?;
                    writeln!(s, "- PI:")?;
                    let pi = child.pi().unwrap();
                    indent(depth + 2, s)?;
                    writeln!(s, "target: {:?}", pi.target)?;
                    if let Some(value) = pi.value {
                        indent(depth + 2, s)?;
                        writeln!(s, "value: {:?}", value)?;
                    }
                }
                NodeType::Root => {}
            }
        }
        Ok(())
    }

    writeln!(s, "Document:")?;
    print_children(doc.root(), 1, s)
}

fn h(x: f64) -> String {
    format!("{:016x}", x.to_bits())
}

// q writes a string as the hex digits of its bytes, so that no rule of quoting
// can differ.
fn q(s: &str) -> String {
    let mut out = String::from("x");
    for b in s.bytes() {
        write!(out, "{:02x}", b).unwrap();
    }
    out
}

fn length(l: &svgtypes::Length) -> String {
    use svgtypes::LengthUnit::*;
    let unit = match l.unit {
        None => "none",
        Em => "em",
        Ex => "ex",
        Px => "px",
        In => "in",
        Cm => "cm",
        Mm => "mm",
        Pt => "pt",
        Pc => "pc",
        Percent => "%",
    };
    format!("{} {}", h(l.number), unit)
}

fn angle(a: &svgtypes::Angle) -> String {
    use svgtypes::AngleUnit::*;
    let unit = match a.unit {
        Degrees => "deg",
        Gradians => "grad",
        Radians => "rad",
        Turns => "turn",
    };
    format!("{} {}", h(a.number), unit)
}

fn color(c: &svgtypes::Color) -> String {
    format!("{} {} {} {}", c.red, c.green, c.blue, c.alpha)
}

fn opt(s: Option<&str>) -> String {
    match s {
        Some(s) => q(s),
        None => "-".to_string(),
    }
}

fn path_segment(seg: &svgtypes::PathSegment) -> String {
    use svgtypes::PathSegment::*;
    let a = |abs: bool| if abs { "A" } else { "R" };
    match *seg {
        MoveTo { abs, x, y } => format!("M {} {} {}", a(abs), h(x), h(y)),
        LineTo { abs, x, y } => format!("L {} {} {}", a(abs), h(x), h(y)),
        HorizontalLineTo { abs, x } => format!("H {} {}", a(abs), h(x)),
        VerticalLineTo { abs, y } => format!("V {} {}", a(abs), h(y)),
        CurveTo { abs, x1, y1, x2, y2, x, y } => format!(
            "C {} {} {} {} {} {} {}",
            a(abs),
            h(x1),
            h(y1),
            h(x2),
            h(y2),
            h(x),
            h(y)
        ),
        SmoothCurveTo { abs, x2, y2, x, y } => {
            format!("S {} {} {} {} {}", a(abs), h(x2), h(y2), h(x), h(y))
        }
        Quadratic { abs, x1, y1, x, y } => {
            format!("Q {} {} {} {} {}", a(abs), h(x1), h(y1), h(x), h(y))
        }
        SmoothQuadratic { abs, x, y } => format!("T {} {} {}", a(abs), h(x), h(y)),
        EllipticalArc { abs, rx, ry, x_axis_rotation, large_arc, sweep, x, y } => format!(
            "A {} {} {} {} {} {} {} {}",
            a(abs),
            h(rx),
            h(ry),
            h(x_axis_rotation),
            large_arc,
            sweep,
            h(x),
            h(y)
        ),
        ClosePath { abs } => format!("Z {}", a(abs)),
    }
}

fn simple_segment(seg: &svgtypes::SimplePathSegment) -> String {
    use svgtypes::SimplePathSegment::*;
    match *seg {
        MoveTo { x, y } => format!("M {} {}", h(x), h(y)),
        LineTo { x, y } => format!("L {} {}", h(x), h(y)),
        CurveTo { x1, y1, x2, y2, x, y } => format!(
            "C {} {} {} {} {} {}",
            h(x1),
            h(y1),
            h(x2),
            h(y2),
            h(x),
            h(y)
        ),
        Quadratic { x1, y1, x, y } => format!("Q {} {} {} {}", h(x1), h(y1), h(x), h(y)),
        ClosePath => "Z".to_string(),
    }
}

fn transform_token(t: &svgtypes::TransformListToken) -> String {
    use svgtypes::TransformListToken::*;
    match *t {
        Matrix { a, b, c, d, e, f } => format!(
            "matrix {} {} {} {} {} {}",
            h(a),
            h(b),
            h(c),
            h(d),
            h(e),
            h(f)
        ),
        Translate { tx, ty } => format!("translate {} {}", h(tx), h(ty)),
        Scale { sx, sy } => format!("scale {} {}", h(sx), h(sy)),
        Rotate { angle } => format!("rotate {}", h(angle)),
        SkewX { angle } => format!("skewX {}", h(angle)),
        SkewY { angle } => format!("skewY {}", h(angle)),
    }
}

fn filter_value(v: &svgtypes::FilterValue) -> String {
    use svgtypes::FilterValue::*;
    match v {
        Blur(l) => format!("blur {}", length(l)),
        DropShadow { color: c, dx, dy, std_dev } => format!(
            "drop-shadow {} {} {} {}",
            c.as_ref().map(color).unwrap_or_else(|| "-".to_string()),
            length(dx),
            length(dy),
            length(std_dev)
        ),
        Brightness(n) => format!("brightness {}", h(*n)),
        Contrast(n) => format!("contrast {}", h(*n)),
        Grayscale(n) => format!("grayscale {}", h(*n)),
        HueRotate(a) => format!("hue-rotate {}", angle(a)),
        Invert(n) => format!("invert {}", h(*n)),
        Opacity(n) => format!("opacity {}", h(*n)),
        Sepia(n) => format!("sepia {}", h(*n)),
        Saturate(n) => format!("saturate {}", h(*n)),
        Url(u) => format!("url {}", q(u)),
    }
}

fn paint_fallback(f: &svgtypes::PaintFallback) -> String {
    use svgtypes::PaintFallback::*;
    match f {
        None => "none".to_string(),
        CurrentColor => "currentColor".to_string(),
        Color(c) => format!("color {}", color(c)),
    }
}

fn paint(p: &svgtypes::Paint) -> String {
    use svgtypes::Paint::*;
    match p {
        None => "none".to_string(),
        Inherit => "inherit".to_string(),
        CurrentColor => "currentColor".to_string(),
        Color(c) => format!("color {}", color(c)),
        FuncIRI(link, fallback) => format!(
            "funciri {} {}",
            q(link),
            fallback.as_ref().map(paint_fallback).unwrap_or_else(|| "-".to_string())
        ),
        ContextFill => "context-fill".to_string(),
        ContextStroke => "context-stroke".to_string(),
    }
}

fn align(a: svgtypes::Align) -> &'static str {
    use svgtypes::Align::*;
    match a {
        None => "none",
        XMinYMin => "xMinYMin",
        XMidYMin => "xMidYMin",
        XMaxYMin => "xMaxYMin",
        XMinYMid => "xMinYMid",
        XMidYMid => "xMidYMid",
        XMaxYMid => "xMaxYMid",
        XMinYMax => "xMinYMax",
        XMidYMax => "xMidYMax",
        XMaxYMax => "xMaxYMax",
    }
}

fn paint_order_kind(k: svgtypes::PaintOrderKind) -> &'static str {
    use svgtypes::PaintOrderKind::*;
    match k {
        Fill => "fill",
        Stroke => "stroke",
        Markers => "markers",
    }
}

fn font_family(f: &svgtypes::FontFamily) -> String {
    use svgtypes::FontFamily::*;
    match f {
        Serif => "serif".to_string(),
        SansSerif => "sans-serif".to_string(),
        Cursive => "cursive".to_string(),
        Fantasy => "fantasy".to_string(),
        Monospace => "monospace".to_string(),
        Named(s) => format!("named {}", q(s)),
    }
}

fn one<T, E>(r: Result<T, E>, f: impl Fn(&T) -> String) -> Vec<String> {
    match r {
        Ok(v) => vec![f(&v)],
        Err(_) => vec!["err".to_string()],
    }
}

fn list<T, E>(it: impl Iterator<Item = Result<T, E>>, f: impl Fn(&T) -> String) -> Vec<String> {
    it.map(|r| match r {
        Ok(v) => f(&v),
        Err(_) => "err".to_string(),
    })
    .collect()
}

/// answer_svgtypes parses the value of the question with the parser that it
/// names. An unknown parser gives no answer.
pub fn answer_svgtypes(question: &SvgtypesQuestion) -> Option<Vec<u8>> {
    use std::str::FromStr;
    use svgtypes::*;

    let v = question.value.as_str();
    let lines: Vec<String> = match question.svgtypes.as_str() {
        "number" => one(Number::from_str(v), |n| h(n.0)),
        "number_list" => list(NumberListParser::from(v), |n| h(*n)),
        "length" => one(Length::from_str(v), length),
        "length_list" => list(LengthListParser::from(v), length),
        "angle" => one(Angle::from_str(v), angle),
        "color" => one(Color::from_str(v), color),
        "paint" => one(Paint::from_str(v), paint),
        "filter" => list(FilterValueListParser::from(v), filter_value),
        "path" => list(PathParser::from(v), path_segment),
        "simple_path" => list(SimplifyingPathParser::from(v), simple_segment),
        "points" => PointsParser::from(v).map(|(x, y)| format!("{} {}", h(x), h(y))).collect(),
        "transform" => one(Transform::from_str(v), |t| {
            format!("{} {} {} {} {} {}", h(t.a), h(t.b), h(t.c), h(t.d), h(t.e), h(t.f))
        }),
        "transform_list" => list(TransformListParser::from(v), transform_token),
        "transform_origin" => one(TransformOrigin::from_str(v), |t| {
            format!("{} / {} / {}", length(&t.x_offset), length(&t.y_offset), length(&t.z_offset))
        }),
        "viewbox" => one(ViewBox::from_str(v), |b| {
            format!("{} {} {} {}", h(b.x), h(b.y), h(b.w), h(b.h))
        }),
        "aspect_ratio" => one(AspectRatio::from_str(v), |a| {
            format!("{} {} {}", a.defer, align(a.align), a.slice)
        }),
        "paint_order" => one(PaintOrder::from_str(v), |p| {
            p.order.iter().map(|k| paint_order_kind(*k)).collect::<Vec<_>>().join(" ")
        }),
        "font_families" => one(parse_font_families(v), |fs| {
            fs.iter().map(font_family).collect::<Vec<_>>().join(", ")
        }),
        "font_shorthand" => one(FontShorthand::from_str(v), |f| {
            format!(
                "{} {} {} {} {} {}",
                opt(f.font_style),
                opt(f.font_variant),
                opt(f.font_weight),
                opt(f.font_stretch),
                q(f.font_size),
                q(f.font_family)
            )
        }),
        "iri" => one(IRI::from_str(v), |i| q(i.0)),
        "funciri" => one(FuncIRI::from_str(v), |i| q(i.0)),
        "enable_background" => one(EnableBackground::from_str(v), |e| match *e {
            EnableBackground::Accumulate => "accumulate".to_string(),
            EnableBackground::New => "new".to_string(),
            EnableBackground::NewWithRegion { x, y, width, height } => {
                format!("new {} {} {} {}", h(x), h(y), h(width), h(height))
            }
        }),
        "directional_position" => one(DirectionalPosition::from_str(v), |d| {
            match d {
                DirectionalPosition::Top => "top",
                DirectionalPosition::Center => "center",
                DirectionalPosition::Bottom => "bottom",
                DirectionalPosition::Right => "right",
                DirectionalPosition::Left => "left",
            }
            .to_string()
        }),
        _ => return None,
    };
    let mut s = String::new();
    for line in lines {
        s.push_str(&line);
        s.push('\n');
    }
    Some(s.into_bytes())
}

#[derive(Deserialize)]
pub struct UsvgQuestion {
    pub usvg: String,
}

/// answer_usvg parses the SVG file of the question with usvg 0.49.0, built
/// without its feature `text`, as the tests of resvg do: with the folder of
/// the file as the folder of resources. The answer is the tree, written by
/// the writer of usvg with its default options, or `error: ` and the name of
/// the case of `usvg::Error`.
pub fn answer_usvg(q: &UsvgQuestion) -> Option<Vec<u8>> {
    let data = std::fs::read(&q.usvg).ok()?;
    let opt = usvg::Options {
        resources_dir: std::path::Path::new(&q.usvg).parent().map(|p| p.to_owned()),
        ..usvg::Options::default()
    };
    let s = match usvg::Tree::from_data(&data, &opt) {
        Ok(tree) => tree.to_string(&usvg::WriteOptions::default()),
        Err(e) => {
            let name = match e {
                usvg::Error::NotAnUtf8Str => "NotAnUtf8Str",
                usvg::Error::SvgzFeatureNotEnabled => "SvgzFeatureNotEnabled",
                usvg::Error::MalformedGZip => "MalformedGZip",
                usvg::Error::ElementsLimitReached => "ElementsLimitReached",
                usvg::Error::InvalidSize => "InvalidSize",
                usvg::Error::ParsingFailed(_) => "ParsingFailed",
            };
            format!("error: {}", name)
        }
    };
    Some(s.into_bytes())
}
