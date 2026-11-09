//! The Rust oracle of okage (D22).
//!
//! It reads one question as JSON on each line of its input and writes one
//! answer for each: four bytes with the length of the answer, little-endian,
//! then the answer. There are two kinds of question:
//!
//! 1. A scene of the renderer. The oracle renders it with tiny-skia 0.12.0.
//!    The answer is the premultiplied RGBA bytes of the pixmap, then the
//!    bytes of each mask of the scene. A scene whose pixmap cannot be made
//!    gives a length of zero. An operation whose shader, path or stroke
//!    tiny-skia refuses is skipped, as the Go tests skip it too.
//! 2. A question of the path crate, with the field "path_op". The oracle
//!    strokes, dashes or transforms a path. The answer is the path that
//!    results, written as `write_path` says, or a length of zero if there is
//!    no path.
//!
//! The Go test writes each number as the shortest decimal of a float64 that
//! gives back the float32. serde_json can read a decimal one unit in the last
//! place of a float64 off, and the conversion to f32 still gives the float32.

use std::io::{self, BufRead, Write};

use serde::Deserialize;
use tiny_skia::{
    BlendMode, Color, ColorSpace, FillRule, FilterQuality, GradientStop, IntSize, LineCap,
    LineJoin, LinearGradient, Mask, MaskType, Paint, Path, PathBuilder, Pattern, Pixmap,
    PixmapPaint, Point, RadialGradient, Rect, Shader, SpreadMode, Stroke, StrokeDash,
    SweepGradient, Transform,
};

#[derive(Deserialize)]
#[serde(untagged)]
enum Question {
    Path(PathQuestion),
    Scene(Scene),
}

#[derive(Deserialize)]
struct PathQuestion {
    path_op: String,
    path: Vec<Vec<f64>>,
    #[serde(default)]
    stroke: Option<StrokeSpec>,
    #[serde(default = "one")]
    res_scale: f32,
    #[serde(default = "identity")]
    transform: [f32; 6],
}

#[derive(Deserialize)]
struct Scene {
    width: u32,
    height: u32,
    #[serde(default)]
    sources: Vec<SourceSpec>,
    #[serde(default)]
    masks: Vec<MaskSpec>,
    ops: Vec<Op>,
}

#[derive(Deserialize)]
struct SourceSpec {
    width: u32,
    height: u32,
    seed: u32,
}

#[derive(Deserialize)]
struct MaskSpec {
    #[serde(default)]
    from_source: Option<usize>,
    #[serde(default)]
    luminance: bool,
    #[serde(default)]
    path: Vec<Vec<f64>>,
    #[serde(default)]
    even_odd: bool,
    #[serde(default)]
    anti_alias: bool,
    #[serde(default = "identity")]
    transform: [f32; 6],
    #[serde(default)]
    intersect: Vec<Vec<f64>>,
    #[serde(default)]
    invert: bool,
}

#[derive(Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
enum Op {
    FillPath {
        path: Vec<Vec<f64>>,
        paint: PaintSpec,
        #[serde(default)]
        even_odd: bool,
        #[serde(default = "identity")]
        transform: [f32; 6],
        #[serde(default)]
        mask: Option<usize>,
    },
    StrokePath {
        path: Vec<Vec<f64>>,
        paint: PaintSpec,
        stroke: StrokeSpec,
        #[serde(default = "identity")]
        transform: [f32; 6],
        #[serde(default)]
        mask: Option<usize>,
    },
    FillRect {
        rect: [f32; 4],
        paint: PaintSpec,
        #[serde(default = "identity")]
        transform: [f32; 6],
        #[serde(default)]
        mask: Option<usize>,
    },
    DrawPixmap {
        x: i32,
        y: i32,
        source: usize,
        opacity: f32,
        blend_mode: u8,
        quality: u8,
        #[serde(default = "identity")]
        transform: [f32; 6],
        #[serde(default)]
        mask: Option<usize>,
    },
    ApplyMask {
        mask: usize,
    },
    Fill {
        color: [f32; 4],
    },
}

#[derive(Deserialize)]
struct PaintSpec {
    shader: ShaderSpec,
    blend_mode: u8,
    anti_alias: bool,
    force_hq: bool,
    colorspace: u8,
    #[serde(default)]
    opacity: Option<f32>,
}

#[derive(Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
enum ShaderSpec {
    Solid {
        color: [f32; 4],
    },
    Linear {
        start: [f32; 2],
        end: [f32; 2],
        stops: Vec<[f32; 5]>,
        spread: u8,
        transform: [f32; 6],
    },
    Radial {
        start: [f32; 2],
        start_radius: f32,
        end: [f32; 2],
        end_radius: f32,
        stops: Vec<[f32; 5]>,
        spread: u8,
        transform: [f32; 6],
    },
    Sweep {
        center: [f32; 2],
        start_angle: f32,
        end_angle: f32,
        stops: Vec<[f32; 5]>,
        spread: u8,
        transform: [f32; 6],
    },
    Pattern {
        source: usize,
        spread: u8,
        quality: u8,
        opacity: f32,
        transform: [f32; 6],
    },
}

#[derive(Deserialize)]
struct StrokeSpec {
    width: f32,
    miter: f32,
    cap: u8,
    join: u8,
    #[serde(default)]
    dash: Option<(Vec<f32>, f32)>,
}

fn identity() -> [f32; 6] {
    [1.0, 0.0, 0.0, 1.0, 0.0, 0.0]
}

fn one() -> f32 {
    1.0
}

fn transform(t: [f32; 6]) -> Transform {
    Transform::from_row(t[0], t[1], t[2], t[3], t[4], t[5])
}

/// Builds a path from segments. Each segment starts with its verb: 0 move,
/// 1 line, 2 quad, 3 cubic, 4 close. The points follow.
fn path(segments: &[Vec<f64>]) -> Option<Path> {
    let mut pb = PathBuilder::new();
    for s in segments {
        let v: Vec<f32> = s.iter().map(|x| *x as f32).collect();
        match v[0] as i32 {
            0 => pb.move_to(v[1], v[2]),
            1 => pb.line_to(v[1], v[2]),
            2 => pb.quad_to(v[1], v[2], v[3], v[4]),
            3 => pb.cubic_to(v[1], v[2], v[3], v[4], v[5], v[6]),
            _ => pb.close(),
        }
    }
    pb.finish()
}

fn spread(s: u8) -> SpreadMode {
    match s {
        1 => SpreadMode::Reflect,
        2 => SpreadMode::Repeat,
        _ => SpreadMode::Pad,
    }
}

fn quality(q: u8) -> FilterQuality {
    match q {
        1 => FilterQuality::Bilinear,
        2 => FilterQuality::Bicubic,
        _ => FilterQuality::Nearest,
    }
}

fn colorspace(c: u8) -> ColorSpace {
    match c {
        1 => ColorSpace::Gamma2,
        2 => ColorSpace::SimpleSRGB,
        3 => ColorSpace::FullSRGBGamma,
        _ => ColorSpace::Linear,
    }
}

/// The blend modes in the order of the Rust enum.
const BLENDS: [BlendMode; 29] = [
    BlendMode::Clear,
    BlendMode::Source,
    BlendMode::Destination,
    BlendMode::SourceOver,
    BlendMode::DestinationOver,
    BlendMode::SourceIn,
    BlendMode::DestinationIn,
    BlendMode::SourceOut,
    BlendMode::DestinationOut,
    BlendMode::SourceAtop,
    BlendMode::DestinationAtop,
    BlendMode::Xor,
    BlendMode::Plus,
    BlendMode::Modulate,
    BlendMode::Screen,
    BlendMode::Overlay,
    BlendMode::Darken,
    BlendMode::Lighten,
    BlendMode::ColorDodge,
    BlendMode::ColorBurn,
    BlendMode::HardLight,
    BlendMode::SoftLight,
    BlendMode::Difference,
    BlendMode::Exclusion,
    BlendMode::Multiply,
    BlendMode::Hue,
    BlendMode::Saturation,
    BlendMode::Color,
    BlendMode::Luminosity,
];

fn stops(s: &[[f32; 5]]) -> Vec<GradientStop> {
    s.iter()
        .map(|v| GradientStop::new(v[0], Color::from_rgba(v[1], v[2], v[3], v[4]).unwrap()))
        .collect()
}

/// Makes a pixmap of random premultiplied pixels from a seed, with a linear
/// congruential generator that the Go test copies.
fn source(s: &SourceSpec) -> Pixmap {
    let n = (s.width * s.height) as usize;
    let mut data = vec![0u8; n * 4];
    let mut h = s.seed;
    for i in 0..n {
        h = h.wrapping_mul(1664525).wrapping_add(1013904223);
        let a = h >> 24;
        let a = if (h >> 3) & 3 == 0 { 255 } else { a };
        data[i * 4] = (((h >> 16) & 255) * a / 255) as u8;
        data[i * 4 + 1] = (((h >> 8) & 255) * a / 255) as u8;
        data[i * 4 + 2] = ((h & 255) * a / 255) as u8;
        data[i * 4 + 3] = a as u8;
    }
    Pixmap::from_vec(data, IntSize::from_wh(s.width, s.height).unwrap()).unwrap()
}

fn shader<'a>(s: &ShaderSpec, sources: &'a [Pixmap]) -> Option<Shader<'a>> {
    match s {
        ShaderSpec::Solid { color } => Some(Shader::SolidColor(Color::from_rgba(
            color[0], color[1], color[2], color[3],
        )?)),
        ShaderSpec::Linear { start, end, stops: st, spread: sp, transform: t } => {
            LinearGradient::new(
                Point::from_xy(start[0], start[1]),
                Point::from_xy(end[0], end[1]),
                stops(st),
                spread(*sp),
                transform(*t),
            )
        }
        ShaderSpec::Radial {
            start,
            start_radius,
            end,
            end_radius,
            stops: st,
            spread: sp,
            transform: t,
        } => RadialGradient::new(
            Point::from_xy(start[0], start[1]),
            *start_radius,
            Point::from_xy(end[0], end[1]),
            *end_radius,
            stops(st),
            spread(*sp),
            transform(*t),
        ),
        ShaderSpec::Sweep { center, start_angle, end_angle, stops: st, spread: sp, transform: t } => {
            SweepGradient::new(
                Point::from_xy(center[0], center[1]),
                *start_angle,
                *end_angle,
                stops(st),
                spread(*sp),
                transform(*t),
            )
        }
        ShaderSpec::Pattern { source, spread: sp, quality: q, opacity, transform: t } => {
            Some(Pattern::new(
                sources[*source].as_ref(),
                spread(*sp),
                quality(*q),
                *opacity,
                transform(*t),
            ))
        }
    }
}

fn paint<'a>(p: &PaintSpec, sources: &'a [Pixmap]) -> Option<Paint<'a>> {
    let mut paint = Paint::default();
    paint.shader = shader(&p.shader, sources)?;
    if let Some(o) = p.opacity {
        paint.shader.apply_opacity(o);
    }
    paint.blend_mode = BLENDS[p.blend_mode as usize];
    paint.anti_alias = p.anti_alias;
    paint.force_hq_pipeline = p.force_hq;
    paint.colorspace = colorspace(p.colorspace);
    Some(paint)
}

fn stroke(s: &StrokeSpec) -> Stroke {
    let mut stroke = Stroke::default();
    stroke.width = s.width;
    stroke.miter_limit = s.miter;
    stroke.line_cap = [LineCap::Butt, LineCap::Round, LineCap::Square][s.cap as usize];
    stroke.line_join =
        [LineJoin::Miter, LineJoin::MiterClip, LineJoin::Round, LineJoin::Bevel][s.join as usize];
    stroke.dash = s.dash.as_ref().and_then(|d| StrokeDash::new(d.0.clone(), d.1));
    stroke
}

fn fill_rule(even_odd: bool) -> FillRule {
    if even_odd {
        FillRule::EvenOdd
    } else {
        FillRule::Winding
    }
}

fn render(scene: &Scene) -> Option<Vec<u8>> {
    let mut pixmap = Pixmap::new(scene.width, scene.height)?;
    let sources: Vec<Pixmap> = scene.sources.iter().map(source).collect();
    let mut masks = Vec::new();
    for m in &scene.masks {
        let mut mask = if let Some(i) = m.from_source {
            let mask_type = if m.luminance { MaskType::Luminance } else { MaskType::Alpha };
            Mask::from_pixmap(sources[i].as_ref(), mask_type)
        } else {
            let mut mask = Mask::new(scene.width, scene.height)?;
            if let Some(p) = path(&m.path) {
                mask.fill_path(&p, fill_rule(m.even_odd), m.anti_alias, transform(m.transform));
            }
            mask
        };
        if let Some(p) = path(&m.intersect) {
            mask.intersect_path(&p, fill_rule(m.even_odd), m.anti_alias, transform(m.transform));
        }
        if m.invert {
            mask.invert();
        }
        masks.push(mask);
    }
    for op in &scene.ops {
        match op {
            Op::FillPath { path: segs, paint: p, even_odd, transform: t, mask } => {
                let (Some(p), Some(pa)) = (path(segs), paint(p, &sources)) else {
                    continue;
                };
                let mask = mask.map(|i| &masks[i]);
                pixmap.fill_path(&p, &pa, fill_rule(*even_odd), transform(*t), mask);
            }
            Op::StrokePath { path: segs, paint: p, stroke: s, transform: t, mask } => {
                let (Some(p), Some(pa)) = (path(segs), paint(p, &sources)) else {
                    continue;
                };
                let mask = mask.map(|i| &masks[i]);
                pixmap.stroke_path(&p, &pa, &stroke(s), transform(*t), mask);
            }
            Op::FillRect { rect, paint: p, transform: t, mask } => {
                let r = Rect::from_xywh(rect[0], rect[1], rect[2], rect[3]);
                let (Some(r), Some(pa)) = (r, paint(p, &sources)) else {
                    continue;
                };
                pixmap.fill_rect(r, &pa, transform(*t), mask.map(|i| &masks[i]));
            }
            Op::DrawPixmap { x, y, source, opacity, blend_mode, quality: q, transform: t, mask } => {
                let pp = PixmapPaint {
                    opacity: *opacity,
                    blend_mode: BLENDS[*blend_mode as usize],
                    quality: quality(*q),
                };
                let mask = mask.map(|i| &masks[i]);
                pixmap.draw_pixmap(*x, *y, sources[*source].as_ref(), &pp, transform(*t), mask);
            }
            Op::ApplyMask { mask } => pixmap.apply_mask(&masks[*mask]),
            Op::Fill { color } => {
                pixmap.fill(Color::from_rgba(color[0], color[1], color[2], color[3]).unwrap())
            }
        }
    }
    let mut out = pixmap.take();
    for m in &masks {
        out.extend_from_slice(m.data());
    }
    Some(out)
}

/// Writes a path as bytes: the number of verbs as a u32, one byte for each
/// verb (0 move, 1 line, 2 quad, 3 cubic, 4 close), the number of points as
/// a u32, the x and y of each point as f32, the bounds as four f32 (left,
/// top, right, bottom), then one byte that is 1 if the tight bounds exist and
/// the four f32 of the tight bounds, or 0. Every number is little-endian.
fn write_path(p: &Path) -> Vec<u8> {
    let mut out = Vec::new();
    out.extend_from_slice(&(p.verbs().len() as u32).to_le_bytes());
    for v in p.verbs() {
        out.push(*v as u8);
    }
    out.extend_from_slice(&(p.points().len() as u32).to_le_bytes());
    for pt in p.points() {
        out.extend_from_slice(&pt.x.to_le_bytes());
        out.extend_from_slice(&pt.y.to_le_bytes());
    }
    let rect = |out: &mut Vec<u8>, r: Rect| {
        for v in [r.left(), r.top(), r.right(), r.bottom()] {
            out.extend_from_slice(&v.to_le_bytes());
        }
    };
    rect(&mut out, p.bounds());
    match p.compute_tight_bounds() {
        Some(r) => {
            out.push(1);
            rect(&mut out, r);
        }
        None => out.push(0),
    }
    out
}

fn answer_path(q: &PathQuestion) -> Option<Vec<u8>> {
    let p = path(&q.path)?;
    let result = match q.path_op.as_str() {
        "stroke" => p.stroke(&stroke(q.stroke.as_ref()?), q.res_scale),
        "dash" => {
            let s = stroke(q.stroke.as_ref()?);
            p.dash(s.dash.as_ref()?, q.res_scale)
        }
        "transform" => p.transform(transform(q.transform)),
        _ => panic!("unknown path_op {}", q.path_op),
    }?;
    Some(write_path(&result))
}

fn main() {
    let stdin = io::stdin();
    let mut out = io::BufWriter::new(io::stdout().lock());
    for line in stdin.lock().lines() {
        let line = line.expect("reading a question");
        if line.trim().is_empty() {
            continue;
        }
        let question: Question = serde_json::from_str(&line).expect("parsing a question");
        let data = match &question {
            Question::Path(q) => answer_path(q),
            Question::Scene(s) => render(s),
        }
        .unwrap_or_default();
        out.write_all(&(data.len() as u32).to_le_bytes()).unwrap();
        out.write_all(&data).unwrap();
        out.flush().unwrap();
    }
}
