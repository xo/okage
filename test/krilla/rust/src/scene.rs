//! The PDF files of krilla: of an SVG file with krilla-svg, and of a scene of
//! calls of the surface of krilla.

use std::sync::Arc;

use base64::Engine;
use krilla::blend::BlendMode;
use krilla::color::{cmyk, luma, rgb, Color};
use krilla::configure::{ConfigurationBuilder, PdfVersion};
use krilla::geom::{PathBuilder, Point, Rect, Size, Transform};
use krilla::image::Image;
use krilla::mask::{Mask, MaskType};
use krilla::num::NormalizedF32;
use krilla::page::{NumberingStyle, PageLabel, PageSettings};
use krilla::paint::{
    Fill, FillRule, LineCap, LineJoin, LinearGradient, Paint, Pattern, RadialGradient,
    SpreadMethod, Stop, Stroke, StrokeDash, SweepGradient,
};
use krilla::surface::Surface;
use krilla::text::{Font, GlyphId, KrillaGlyph, Tag, TextDirection};
use krilla::{Document, SerializeSettings};
use krilla_svg::usvg::{self, fontdb};
use krilla_svg::{SurfaceExt, SvgSettings};
use serde::Deserialize;

/// The fields of `SerializeSettings`. A missing field takes the value of
/// `SerializeSettings::default()`, except that SVG glyphs are drawn with
/// `krilla_svg::render_svg_glyph`, as the tests of krilla do.
#[derive(Deserialize, Default)]
#[serde(default)]
pub struct SettingsSpec {
    pretty: Option<bool>,
    ascii_compatible: Option<bool>,
    compress_content_streams: Option<bool>,
    no_device_cs: Option<bool>,
    xmp_metadata: Option<bool>,
    enable_tagging: Option<bool>,
    /// "1.4", "1.5", "1.6", "1.7" or "2.0".
    version: Option<String>,
    /// false leaves the SVG glyphs out, as the default function of krilla
    /// does.
    svg_glyphs: Option<bool>,
}

fn settings(s: &SettingsSpec) -> Result<SerializeSettings, String> {
    let d = SerializeSettings::default();
    let mut out = SerializeSettings {
        pretty: s.pretty.unwrap_or(d.pretty),
        ascii_compatible: s.ascii_compatible.unwrap_or(d.ascii_compatible),
        compress_content_streams: s.compress_content_streams.unwrap_or(d.compress_content_streams),
        no_device_cs: s.no_device_cs.unwrap_or(d.no_device_cs),
        xmp_metadata: s.xmp_metadata.unwrap_or(d.xmp_metadata),
        enable_tagging: s.enable_tagging.unwrap_or(d.enable_tagging),
        ..d
    };
    if s.svg_glyphs.unwrap_or(true) {
        out.render_svg_glyph_fn = krilla_svg::render_svg_glyph;
    }
    if let Some(v) = &s.version {
        let version = match v.as_str() {
            "1.4" => PdfVersion::Pdf14,
            "1.5" => PdfVersion::Pdf15,
            "1.6" => PdfVersion::Pdf16,
            "1.7" => PdfVersion::Pdf17,
            "2.0" => PdfVersion::Pdf20,
            _ => return Err(format!("the version {v:?} is not known")),
        };
        out.configuration = ConfigurationBuilder::new()
            .with_version(version)
            .finish()
            .map_err(|e| format!("the version {v:?}: {e:?}"))?;
    }
    Ok(out)
}

/// The fonts of the font database of usvg.
#[derive(Deserialize, Default)]
#[serde(default)]
struct FontsSpec {
    dirs: Vec<String>,
    files: Vec<String>,
    serif: Option<String>,
    sans_serif: Option<String>,
    cursive: Option<String>,
    fantasy: Option<String>,
    monospace: Option<String>,
}

/// An SVG file to draw with krilla-svg.
#[derive(Deserialize)]
struct SvgSpec {
    file: String,
    /// The size to draw the tree at. The default is the size of the tree.
    #[serde(default)]
    width: Option<f32>,
    #[serde(default)]
    height: Option<f32>,
    /// The fields of `SvgSettings`. A missing field takes the value of
    /// `SvgSettings::default()`.
    #[serde(default)]
    embed_text: Option<bool>,
    #[serde(default)]
    filter_scale: Option<f32>,
    #[serde(default)]
    fonts: FontsSpec,
}

/// A question for the PDF file of an SVG file. The file has one page with
/// the size of the tree, or the size of the question.
#[derive(Deserialize)]
pub struct SvgQuestion {
    #[serde(default)]
    settings: SettingsSpec,
    #[serde(flatten)]
    svg: SvgSpec,
}

/// answer_svg writes the PDF file of an SVG file, as the test `svg_impl` of
/// krilla does.
pub fn answer_svg(q: &SvgQuestion) -> Result<Vec<u8>, String> {
    let tree = load_svg(&q.svg)?;
    let size = svg_size(&q.svg, &tree)?;
    let mut doc = Document::new_with(settings(&q.settings)?);
    let mut page = doc.start_page_with(
        PageSettings::from_wh(size.width(), size.height()).ok_or("the size of the page is not valid")?,
    );
    let mut surface = page.surface();
    surface.draw_svg(&tree, size, svg_settings(&q.svg));
    surface.finish();
    page.finish();
    doc.finish().map_err(|e| format!("{e:?}"))
}

fn load_svg(s: &SvgSpec) -> Result<usvg::Tree, String> {
    let data = std::fs::read(&s.file).map_err(|e| format!("reading {}: {e}", s.file))?;
    let mut db = fontdb::Database::new();
    for d in &s.fonts.dirs {
        db.load_fonts_dir(d);
    }
    for f in &s.fonts.files {
        db.load_font_file(f).map_err(|e| format!("loading {f}: {e}"))?;
    }
    if let Some(f) = &s.fonts.serif {
        db.set_serif_family(f);
    }
    if let Some(f) = &s.fonts.sans_serif {
        db.set_sans_serif_family(f);
    }
    if let Some(f) = &s.fonts.cursive {
        db.set_cursive_family(f);
    }
    if let Some(f) = &s.fonts.fantasy {
        db.set_fantasy_family(f);
    }
    if let Some(f) = &s.fonts.monospace {
        db.set_monospace_family(f);
    }
    let opt = usvg::Options {
        resources_dir: std::path::Path::new(&s.file).parent().map(|p| p.to_owned()),
        fontdb: Arc::new(db),
        ..usvg::Options::default()
    };
    usvg::Tree::from_data(&data, &opt).map_err(|e| format!("parsing {}: {e}", s.file))
}

fn svg_size(s: &SvgSpec, tree: &usvg::Tree) -> Result<Size, String> {
    Size::from_wh(
        s.width.unwrap_or(tree.size().width()),
        s.height.unwrap_or(tree.size().height()),
    )
    .ok_or_else(|| "the size of the SVG file is not valid".to_string())
}

fn svg_settings(s: &SvgSpec) -> SvgSettings {
    let d = SvgSettings::default();
    SvgSettings {
        embed_text: s.embed_text.unwrap_or(d.embed_text),
        filter_scale: s.filter_scale.unwrap_or(d.filter_scale),
    }
}

/// A scene: a document, its fonts and images, and the calls of the surface
/// of each page.
#[derive(Deserialize)]
pub struct Scene {
    #[serde(default)]
    settings: SettingsSpec,
    #[serde(default)]
    fonts: Vec<FontSpec>,
    #[serde(default)]
    images: Vec<ImageSpec>,
    pages: Vec<PageSpec>,
}

#[derive(Deserialize)]
struct FontSpec {
    file: String,
    #[serde(default)]
    index: u32,
    #[serde(default)]
    coords: Vec<(String, f32)>,
}

/// An image, from a file or from RGBA bytes.
#[derive(Deserialize)]
struct ImageSpec {
    /// The path of a PNG, JPEG, GIF or WebP file.
    #[serde(default)]
    file: Option<String>,
    /// "png", "jpeg", "gif" or "webp". The default comes from the extension
    /// of the file.
    #[serde(default)]
    kind: Option<String>,
    #[serde(default)]
    interpolate: bool,
    /// The bytes of an RGBA image, not premultiplied, in base64, for
    /// `Image::from_rgba8`.
    #[serde(default)]
    rgba: Option<String>,
    #[serde(default)]
    width: u32,
    #[serde(default)]
    height: u32,
}

#[derive(Deserialize)]
struct PageSpec {
    #[serde(default)]
    width: f32,
    #[serde(default)]
    height: f32,
    #[serde(default)]
    ops: Vec<Op>,
    /// The media box as [x, y, w, h], for PageSettings::new. Without it, the
    /// page is PageSettings::from_wh(width, height).
    #[serde(default)]
    media_box: Option<[f32; 4]>,
    #[serde(default)]
    crop_box: Option<[f32; 4]>,
    #[serde(default)]
    bleed_box: Option<[f32; 4]>,
    #[serde(default)]
    trim_box: Option<[f32; 4]>,
    #[serde(default)]
    art_box: Option<[f32; 4]>,
    #[serde(default)]
    label: Option<LabelSpec>,
    /// false finishes the page without a call of Page::surface.
    #[serde(default = "yes")]
    surface: bool,
}

fn yes() -> bool {
    true
}

/// A page label. "style" is "D", "r", "R", "a" or "A", as the key /S of PDF.
#[derive(Deserialize)]
struct LabelSpec {
    #[serde(default)]
    style: Option<String>,
    #[serde(default)]
    prefix: Option<String>,
    #[serde(default)]
    offset: Option<u32>,
}

fn rect_xywh(r: [f32; 4]) -> Result<Rect, String> {
    Rect::from_xywh(r[0], r[1], r[2], r[3]).ok_or_else(|| format!("the rectangle {r:?} is not valid"))
}

fn page_settings(p: &PageSpec) -> Result<PageSettings, String> {
    let mut s = match p.media_box {
        Some(r) => PageSettings::new(rect_xywh(r)?),
        None => PageSettings::from_wh(p.width, p.height).ok_or("the size of a page is not valid")?,
    };
    let opt = |r: Option<[f32; 4]>| r.map(rect_xywh).transpose();
    s = s
        .with_crop_box(opt(p.crop_box)?)
        .with_bleed_box(opt(p.bleed_box)?)
        .with_trim_box(opt(p.trim_box)?)
        .with_art_box(opt(p.art_box)?);
    if let Some(l) = &p.label {
        let style = match l.style.as_deref() {
            None => None,
            Some("D") => Some(NumberingStyle::Arabic),
            Some("r") => Some(NumberingStyle::LowerRoman),
            Some("R") => Some(NumberingStyle::UpperRoman),
            Some("a") => Some(NumberingStyle::LowerAlpha),
            Some("A") => Some(NumberingStyle::UpperAlpha),
            Some(other) => return Err(format!("the numbering style {other:?} is not known")),
        };
        let offset = l.offset.and_then(std::num::NonZeroU32::new);
        s = s.with_page_label(PageLabel::new(style, l.prefix.clone(), offset));
    }
    Ok(s)
}

/// A call of the surface.
#[derive(Deserialize)]
#[serde(tag = "op", rename_all = "snake_case")]
enum Op {
    SetFill {
        #[serde(default)]
        fill: Option<FillSpec>,
    },
    SetStroke {
        #[serde(default)]
        stroke: Option<StrokeSpec>,
    },
    DrawPath {
        path: Vec<Vec<f64>>,
    },
    PushTransform {
        transform: [f32; 6],
    },
    PushBlendMode {
        mode: String,
    },
    PushClipPath {
        path: Vec<Vec<f64>>,
        #[serde(default)]
        rule: Option<String>,
    },
    PushMask {
        mask_type: String,
        ops: Vec<Op>,
        #[serde(default)]
        bbox: Option<[f32; 4]>,
    },
    PushOpacity {
        opacity: f32,
    },
    PushIsolated,
    Pop,
    DrawImage {
        image: usize,
        width: f32,
        height: f32,
    },
    DrawGlyphs {
        start: [f32; 2],
        font: usize,
        size: f32,
        glyphs: Vec<GlyphSpec>,
        #[serde(default)]
        text: String,
        #[serde(default)]
        outlined: bool,
    },
    DrawText {
        start: [f32; 2],
        font: usize,
        size: f32,
        text: String,
        #[serde(default)]
        outlined: bool,
        #[serde(default)]
        direction: Option<String>,
    },
    DrawSvg(SvgSpec),
}

#[derive(Deserialize)]
struct FillSpec {
    #[serde(default)]
    paint: Option<PaintSpec>,
    #[serde(default = "one")]
    opacity: f32,
    #[serde(default)]
    rule: Option<String>,
}

#[derive(Deserialize)]
struct StrokeSpec {
    #[serde(default)]
    paint: Option<PaintSpec>,
    #[serde(default = "one")]
    width: f32,
    #[serde(default = "ten")]
    miter_limit: f32,
    #[serde(default)]
    line_cap: Option<String>,
    #[serde(default)]
    line_join: Option<String>,
    #[serde(default = "one")]
    opacity: f32,
    #[serde(default)]
    dash: Option<DashSpec>,
}

#[derive(Deserialize)]
struct DashSpec {
    array: Vec<f32>,
    #[serde(default)]
    offset: f32,
}

#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum ColorSpec {
    Rgb([u8; 3]),
    Luma(u8),
    Cmyk([u8; 4]),
}

#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum PaintSpec {
    Rgb([u8; 3]),
    Luma(u8),
    Cmyk([u8; 4]),
    Linear(LinearSpec),
    Radial(RadialSpec),
    Sweep(SweepSpec),
    Pattern(PatternSpec),
}

#[derive(Deserialize)]
struct StopSpec {
    offset: f32,
    color: ColorSpec,
    #[serde(default = "one")]
    opacity: f32,
}

#[derive(Deserialize)]
struct LinearSpec {
    x1: f32,
    y1: f32,
    x2: f32,
    y2: f32,
    #[serde(default = "identity")]
    transform: [f32; 6],
    #[serde(default)]
    spread: Option<String>,
    stops: Vec<StopSpec>,
    #[serde(default)]
    anti_alias: bool,
}

#[derive(Deserialize)]
struct RadialSpec {
    fx: f32,
    fy: f32,
    fr: f32,
    cx: f32,
    cy: f32,
    cr: f32,
    #[serde(default = "identity")]
    transform: [f32; 6],
    #[serde(default)]
    spread: Option<String>,
    stops: Vec<StopSpec>,
    #[serde(default)]
    anti_alias: bool,
}

#[derive(Deserialize)]
struct SweepSpec {
    cx: f32,
    cy: f32,
    start_angle: f32,
    end_angle: f32,
    #[serde(default = "identity")]
    transform: [f32; 6],
    #[serde(default)]
    spread: Option<String>,
    stops: Vec<StopSpec>,
    #[serde(default)]
    anti_alias: bool,
}

#[derive(Deserialize)]
struct PatternSpec {
    ops: Vec<Op>,
    width: f32,
    height: f32,
    #[serde(default = "identity")]
    transform: [f32; 6],
}

#[derive(Deserialize)]
struct GlyphSpec {
    id: u32,
    /// The byte range of the cluster of the glyph in the text.
    #[serde(default)]
    range: [usize; 2],
    #[serde(default)]
    x_advance: f32,
    #[serde(default)]
    x_offset: f32,
    #[serde(default)]
    y_offset: f32,
    #[serde(default)]
    y_advance: f32,
}

fn one() -> f32 {
    1.0
}

fn ten() -> f32 {
    10.0
}

fn identity() -> [f32; 6] {
    [1.0, 0.0, 0.0, 1.0, 0.0, 0.0]
}

/// The fonts and the images of a scene.
struct Resources {
    fonts: Vec<Font>,
    images: Vec<Image>,
}

/// answer_scene writes the PDF file of a scene.
pub fn answer_scene(q: &Scene) -> Result<Vec<u8>, String> {
    let mut res = Resources {
        fonts: Vec::new(),
        images: Vec::new(),
    };
    for f in &q.fonts {
        let data = std::fs::read(&f.file).map_err(|e| format!("reading {}: {e}", f.file))?;
        let mut coords = Vec::with_capacity(f.coords.len());
        for (name, value) in &f.coords {
            let tag = Tag::try_from_str(name).ok_or_else(|| format!("the tag {name:?} is not valid"))?;
            coords.push((tag, *value));
        }
        let font = Font::new_variable(data.into(), f.index, &coords)
            .ok_or_else(|| format!("krilla cannot read the font {}", f.file))?;
        res.fonts.push(font);
    }
    for i in &q.images {
        res.images.push(image(i)?);
    }
    let mut doc = Document::new_with(settings(&q.settings)?);
    for p in &q.pages {
        let mut page = doc.start_page_with(page_settings(p)?);
        if p.surface {
            let mut surface = page.surface();
            run(&mut surface, &p.ops, &res)?;
            surface.finish();
        }
        page.finish();
    }
    doc.finish().map_err(|e| format!("{e:?}"))
}

fn image(i: &ImageSpec) -> Result<Image, String> {
    if let Some(b) = &i.rgba {
        let data = base64::engine::general_purpose::STANDARD
            .decode(b)
            .map_err(|e| format!("decoding an RGBA image: {e}"))?;
        if data.len() as u64 != 4 * u64::from(i.width) * u64::from(i.height) {
            return Err("the RGBA image does not have 4 bytes for each pixel".to_string());
        }
        return Ok(Image::from_rgba8(data, i.width, i.height));
    }
    let file = i.file.as_deref().ok_or("an image has no file and no RGBA bytes")?;
    let data = std::fs::read(file).map_err(|e| format!("reading {file}: {e}"))?;
    let kind = match &i.kind {
        Some(k) => k.clone(),
        None => std::path::Path::new(file)
            .extension()
            .and_then(|e| e.to_str())
            .unwrap_or_default()
            .to_ascii_lowercase(),
    };
    match kind.as_str() {
        "png" => Image::from_png(data.into(), i.interpolate),
        "jpeg" | "jpg" => Image::from_jpeg(data.into(), i.interpolate),
        "gif" => Image::from_gif(data.into(), i.interpolate),
        "webp" => Image::from_webp(data.into(), i.interpolate),
        _ => return Err(format!("the kind of the image {file} is not known")),
    }
    .map_err(|e| format!("krilla cannot read the image {file}: {e}"))
}

/// run makes the calls of a list on a surface. Each push of the list must
/// have its pop in the same list. If a call fails, run pops what the list
/// pushed, so that the surface can finish.
fn run(s: &mut Surface, ops: &[Op], res: &Resources) -> Result<(), String> {
    let mut depth = 0usize;
    for op in ops {
        if let Err(e) = step(s, op, res, &mut depth) {
            for _ in 0..depth {
                s.pop();
            }
            return Err(e);
        }
    }
    if depth != 0 {
        for _ in 0..depth {
            s.pop();
        }
        return Err(format!("{depth} pushes have no pop"));
    }
    Ok(())
}

fn step(s: &mut Surface, op: &Op, res: &Resources, depth: &mut usize) -> Result<(), String> {
    match op {
        Op::SetFill { fill } => {
            let fill = match fill {
                Some(f) => Some(Fill {
                    paint: paint(f.paint.as_ref(), s, res)?,
                    opacity: normalized(f.opacity)?,
                    rule: fill_rule(f.rule.as_deref())?,
                }),
                None => None,
            };
            s.set_fill(fill);
        }
        Op::SetStroke { stroke } => {
            let stroke = match stroke {
                Some(st) => Some(Stroke {
                    paint: paint(st.paint.as_ref(), s, res)?,
                    width: st.width,
                    miter_limit: st.miter_limit,
                    line_cap: match st.line_cap.as_deref() {
                        None | Some("butt") => LineCap::Butt,
                        Some("round") => LineCap::Round,
                        Some("square") => LineCap::Square,
                        Some(c) => return Err(format!("the line cap {c:?} is not known")),
                    },
                    line_join: match st.line_join.as_deref() {
                        None | Some("miter") => LineJoin::Miter,
                        Some("round") => LineJoin::Round,
                        Some("bevel") => LineJoin::Bevel,
                        Some(j) => return Err(format!("the line join {j:?} is not known")),
                    },
                    opacity: normalized(st.opacity)?,
                    dash: st.dash.as_ref().map(|d| StrokeDash {
                        array: d.array.clone(),
                        offset: d.offset,
                    }),
                }),
                None => None,
            };
            s.set_stroke(stroke);
        }
        Op::DrawPath { path: p } => s.draw_path(&path(p)?),
        Op::PushTransform { transform: t } => {
            s.push_transform(&transform(t));
            *depth += 1;
        }
        Op::PushBlendMode { mode } => {
            s.push_blend_mode(blend_mode(mode)?);
            *depth += 1;
        }
        Op::PushClipPath { path: p, rule } => {
            s.push_clip_path(&path(p)?, &fill_rule(rule.as_deref())?);
            *depth += 1;
        }
        Op::PushMask {
            mask_type,
            ops,
            bbox,
        } => {
            let mask_type = match mask_type.as_str() {
                "luminosity" => MaskType::Luminosity,
                "alpha" => MaskType::Alpha,
                _ => return Err(format!("the mask type {mask_type:?} is not known")),
            };
            let mut sb = s.stream_builder();
            let mut sub = match bbox {
                Some(b) => sb.surface_with_bbox(
                    Rect::from_xywh(b[0], b[1], b[2], b[3]).ok_or("the box of a mask is not valid")?,
                ),
                None => sb.surface(),
            };
            let result = run(&mut sub, ops, res);
            sub.finish();
            let stream = sb.finish();
            result?;
            s.push_mask(Mask::new(stream, mask_type));
            *depth += 1;
        }
        Op::PushOpacity { opacity } => {
            s.push_opacity(normalized(*opacity)?);
            *depth += 1;
        }
        Op::PushIsolated => {
            s.push_isolated();
            *depth += 1;
        }
        Op::Pop => {
            if *depth == 0 {
                return Err("a pop has no push in its list".to_string());
            }
            s.pop();
            *depth -= 1;
        }
        Op::DrawImage {
            image,
            width,
            height,
        } => {
            let img = res
                .images
                .get(*image)
                .ok_or_else(|| format!("the image {image} does not exist"))?;
            s.draw_image(
                img.clone(),
                Size::from_wh(*width, *height).ok_or("the size of an image is not valid")?,
            );
        }
        Op::DrawGlyphs {
            start,
            font,
            size,
            glyphs,
            text,
            outlined,
        } => {
            let f = res
                .fonts
                .get(*font)
                .ok_or_else(|| format!("the font {font} does not exist"))?;
            let glyphs: Vec<KrillaGlyph> = glyphs
                .iter()
                .map(|g| KrillaGlyph {
                    glyph_id: GlyphId::new(g.id),
                    text_range: g.range[0]..g.range[1],
                    x_advance: g.x_advance,
                    x_offset: g.x_offset,
                    y_offset: g.y_offset,
                    y_advance: g.y_advance,
                    location: None,
                })
                .collect();
            for g in &glyphs {
                if g.text_range.start > g.text_range.end
                    || g.text_range.end > text.len()
                    || !text.is_char_boundary(g.text_range.start)
                    || !text.is_char_boundary(g.text_range.end)
                {
                    return Err(format!("the range {:?} is not in the text", g.text_range));
                }
            }
            s.draw_glyphs(
                Point::from_xy(start[0], start[1]),
                &glyphs,
                f.clone(),
                text,
                *size,
                *outlined,
            );
        }
        Op::DrawText {
            start,
            font,
            size,
            text,
            outlined,
            direction,
        } => {
            let f = res
                .fonts
                .get(*font)
                .ok_or_else(|| format!("the font {font} does not exist"))?;
            let dir = match direction.as_deref() {
                None | Some("auto") => TextDirection::Auto,
                Some("ltr") => TextDirection::LeftToRight,
                Some("rtl") => TextDirection::RightToLeft,
                Some("ttb") => TextDirection::TopToBottom,
                Some("btt") => TextDirection::BottomToTop,
                Some(d) => return Err(format!("the direction {d:?} is not known")),
            };
            s.draw_text(
                Point::from_xy(start[0], start[1]),
                f.clone(),
                *size,
                text,
                *outlined,
                dir,
            );
        }
        Op::DrawSvg(spec) => {
            let tree = load_svg(spec)?;
            let size = svg_size(spec, &tree)?;
            s.draw_svg(&tree, size, svg_settings(spec))
                .ok_or("krilla-svg cannot draw the tree")?;
        }
    }
    Ok(())
}

fn paint(p: Option<&PaintSpec>, s: &mut Surface, res: &Resources) -> Result<Paint, String> {
    let p = match p {
        Some(p) => p,
        None => return Ok(luma::Color::black().into()),
    };
    Ok(match p {
        PaintSpec::Rgb(c) => rgb::Color::new(c[0], c[1], c[2]).into(),
        PaintSpec::Luma(l) => luma::Color::new(*l).into(),
        PaintSpec::Cmyk(c) => cmyk::Color::new(c[0], c[1], c[2], c[3]).into(),
        PaintSpec::Linear(g) => LinearGradient {
            x1: g.x1,
            y1: g.y1,
            x2: g.x2,
            y2: g.y2,
            transform: transform(&g.transform),
            spread_method: spread(g.spread.as_deref())?,
            stops: stops(&g.stops)?,
            anti_alias: g.anti_alias,
        }
        .into(),
        PaintSpec::Radial(g) => RadialGradient {
            fx: g.fx,
            fy: g.fy,
            fr: g.fr,
            cx: g.cx,
            cy: g.cy,
            cr: g.cr,
            transform: transform(&g.transform),
            spread_method: spread(g.spread.as_deref())?,
            stops: stops(&g.stops)?,
            anti_alias: g.anti_alias,
        }
        .into(),
        PaintSpec::Sweep(g) => SweepGradient {
            cx: g.cx,
            cy: g.cy,
            start_angle: g.start_angle,
            end_angle: g.end_angle,
            transform: transform(&g.transform),
            spread_method: spread(g.spread.as_deref())?,
            stops: stops(&g.stops)?,
            anti_alias: g.anti_alias,
        }
        .into(),
        PaintSpec::Pattern(pat) => {
            let mut sb = s.stream_builder();
            let mut sub = sb.surface();
            let result = run(&mut sub, &pat.ops, res);
            sub.finish();
            let stream = sb.finish();
            result?;
            Pattern {
                stream,
                transform: transform(&pat.transform),
                width: pat.width,
                height: pat.height,
            }
            .into()
        }
    })
}

fn color(c: &ColorSpec) -> Color {
    match c {
        ColorSpec::Rgb(c) => rgb::Color::new(c[0], c[1], c[2]).into(),
        ColorSpec::Luma(l) => luma::Color::new(*l).into(),
        ColorSpec::Cmyk(c) => cmyk::Color::new(c[0], c[1], c[2], c[3]).into(),
    }
}

fn stops(stops: &[StopSpec]) -> Result<Vec<Stop>, String> {
    stops
        .iter()
        .map(|st| {
            Ok(Stop {
                offset: normalized(st.offset)?,
                color: color(&st.color),
                opacity: normalized(st.opacity)?,
            })
        })
        .collect()
}

fn spread(s: Option<&str>) -> Result<SpreadMethod, String> {
    Ok(match s {
        None | Some("pad") => SpreadMethod::Pad,
        Some("reflect") => SpreadMethod::Reflect,
        Some("repeat") => SpreadMethod::Repeat,
        Some(s) => return Err(format!("the spread method {s:?} is not known")),
    })
}

fn fill_rule(r: Option<&str>) -> Result<FillRule, String> {
    Ok(match r {
        None | Some("nonzero") => FillRule::NonZero,
        Some("evenodd") => FillRule::EvenOdd,
        Some(r) => return Err(format!("the fill rule {r:?} is not known")),
    })
}

fn blend_mode(m: &str) -> Result<BlendMode, String> {
    Ok(match m {
        "normal" => BlendMode::Normal,
        "multiply" => BlendMode::Multiply,
        "screen" => BlendMode::Screen,
        "overlay" => BlendMode::Overlay,
        "darken" => BlendMode::Darken,
        "lighten" => BlendMode::Lighten,
        "color_dodge" => BlendMode::ColorDodge,
        "color_burn" => BlendMode::ColorBurn,
        "hard_light" => BlendMode::HardLight,
        "soft_light" => BlendMode::SoftLight,
        "difference" => BlendMode::Difference,
        "exclusion" => BlendMode::Exclusion,
        "hue" => BlendMode::Hue,
        "saturation" => BlendMode::Saturation,
        "color" => BlendMode::Color,
        "luminosity" => BlendMode::Luminosity,
        _ => return Err(format!("the blend mode {m:?} is not known")),
    })
}

fn normalized(v: f32) -> Result<NormalizedF32, String> {
    NormalizedF32::new(v).ok_or_else(|| format!("{v} is not between 0 and 1"))
}

/// transform makes a transform from `[sx, ky, kx, sy, tx, ty]`, the order of
/// `Transform::from_row`.
fn transform(t: &[f32; 6]) -> Transform {
    Transform::from_row(t[0], t[1], t[2], t[3], t[4], t[5])
}

/// path makes a path from its segments, as the oracle of test/oracle does:
/// `[0, x, y]` moves, `[1, x, y]` draws a line, `[2, x1, y1, x, y]` a
/// quadratic curve, `[3, x1, y1, x2, y2, x, y]` a cubic curve, and `[4]`
/// closes.
fn path(segments: &[Vec<f64>]) -> Result<krilla::geom::Path, String> {
    let mut pb = PathBuilder::new();
    for seg in segments {
        let v: Vec<f32> = seg.iter().map(|x| *x as f32).collect();
        let want = match v.first().map(|x| *x as i32) {
            Some(0) | Some(1) => 3,
            Some(2) => 5,
            Some(3) => 7,
            Some(4) => 1,
            _ => return Err(format!("the segment {seg:?} is not known")),
        };
        if v.len() != want {
            return Err(format!("the segment {seg:?} has {} numbers, want {want}", v.len()));
        }
        match v[0] as i32 {
            0 => pb.move_to(v[1], v[2]),
            1 => pb.line_to(v[1], v[2]),
            2 => pb.quad_to(v[1], v[2], v[3], v[4]),
            3 => pb.cubic_to(v[1], v[2], v[3], v[4], v[5], v[6]),
            _ => pb.close(),
        }
    }
    pb.finish().ok_or_else(|| "the path is empty".to_string())
}
