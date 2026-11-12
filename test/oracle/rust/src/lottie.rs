//! The questions of velato (D41). A question with the field "lottie" names a
//! Lottie file and a list of frames. The oracle renders each frame with
//! velato onto a sink that records each call, and the answer is one line of
//! text for each call, as `Recorder` writes it. A file that velato cannot
//! read gives the line "error", and a frame that velato cannot evaluate
//! gives the line "evaluation error".

use std::fmt::Write as _;

use kurbo::{Affine, PathEl, Shape};
use peniko::{BlendMode, Brush, GradientKind};
use serde::Deserialize;
use velato::model::{fixed, ImageAsset};
use velato::{Composition, FilterEffect, FilterLayerResult, RenderSink, Renderer};

#[derive(Deserialize)]
pub struct LottieQuestion {
    pub lottie: String,
    pub frames: Vec<f64>,
}

/// Recorder is a RenderSink that writes each call as a line of text. The
/// numbers have six decimals.
#[derive(Default)]
struct Recorder {
    out: String,
}

fn num(out: &mut String, v: f64) {
    let _ = write!(out, " {:.6}", v);
}

fn affine(out: &mut String, t: Affine) {
    for c in t.as_coeffs() {
        num(out, c);
    }
}

fn shape(out: &mut String, s: &impl Shape) {
    for el in s.path_elements(0.1) {
        match el {
            PathEl::MoveTo(p) => {
                out.push_str(" M");
                num(out, p.x);
                num(out, p.y);
            }
            PathEl::LineTo(p) => {
                out.push_str(" L");
                num(out, p.x);
                num(out, p.y);
            }
            PathEl::QuadTo(p1, p2) => {
                out.push_str(" Q");
                for p in [p1, p2] {
                    num(out, p.x);
                    num(out, p.y);
                }
            }
            PathEl::CurveTo(p1, p2, p3) => {
                out.push_str(" C");
                for p in [p1, p2, p3] {
                    num(out, p.x);
                    num(out, p.y);
                }
            }
            PathEl::ClosePath => out.push_str(" Z"),
        }
    }
}

fn blend(out: &mut String, b: BlendMode) {
    let _ = write!(out, " mix {} compose {}", b.mix as u8, b.compose as u8);
}

fn color(out: &mut String, c: [f32; 4]) {
    for v in c {
        num(out, v as f64);
    }
}

fn brush(out: &mut String, b: &Brush) {
    match b {
        Brush::Solid(c) => {
            out.push_str(" solid");
            color(out, c.components);
        }
        Brush::Gradient(g) => {
            match g.kind {
                GradientKind::Linear(l) => {
                    out.push_str(" linear");
                    num(out, l.start.x);
                    num(out, l.start.y);
                    num(out, l.end.x);
                    num(out, l.end.y);
                }
                GradientKind::Radial(r) => {
                    out.push_str(" radial");
                    num(out, r.start_center.x);
                    num(out, r.start_center.y);
                    num(out, r.end_radius as f64);
                }
                GradientKind::Sweep(_) => out.push_str(" sweep"),
            }
            out.push_str(" stops");
            for s in g.stops.iter() {
                num(out, s.offset as f64);
                color(out, s.color.components);
            }
        }
        Brush::Image(_) => out.push_str(" image"),
    }
}

impl RenderSink for Recorder {
    fn push_layer(&mut self, b: impl Into<BlendMode>, alpha: f32, transform: Affine, s: &impl Shape) {
        self.out.push_str("push");
        blend(&mut self.out, b.into());
        num(&mut self.out, alpha as f64);
        affine(&mut self.out, transform);
        shape(&mut self.out, s);
        self.out.push('\n');
    }

    fn push_clip_layer(&mut self, transform: Affine, s: &impl Shape) {
        self.out.push_str("clip");
        affine(&mut self.out, transform);
        shape(&mut self.out, s);
        self.out.push('\n');
    }

    fn pop_layer(&mut self) {
        self.out.push_str("pop\n");
    }

    fn push_filter(&mut self, _: Affine, _: &FilterEffect) -> FilterLayerResult {
        self.out.push_str("filter\n");
        FilterLayerResult::Unsupported
    }

    fn draw(&mut self, stroke: Option<&fixed::Stroke>, transform: Affine, b: &fixed::Brush, s: &impl Shape) {
        self.out.push_str("draw");
        match stroke {
            Some(st) => {
                self.out.push_str(" stroke");
                num(&mut self.out, st.width);
                let _ = write!(self.out, " {} {}", st.join as u8, st.start_cap as u8);
                num(&mut self.out, st.miter_limit);
            }
            None => self.out.push_str(" fill"),
        }
        affine(&mut self.out, transform);
        brush(&mut self.out, b);
        shape(&mut self.out, s);
        self.out.push('\n');
    }

    fn draw_image(&mut self, _: &ImageAsset, _: Affine, _: f64) {
        self.out.push_str("image\n");
    }
}

pub fn answer_lottie(q: &LottieQuestion) -> Option<Vec<u8>> {
    let data = std::fs::read(&q.lottie).ok()?;
    let mut rec = Recorder::default();
    let composition = match Composition::from_slice(&data) {
        Ok(c) => c,
        Err(_) => return Some(b"error\n".to_vec()),
    };
    let mut renderer = Renderer::new();
    for &frame in &q.frames {
        let _ = writeln!(rec.out, "frame {}", frame);
        if renderer
            .try_append(&composition, frame, Affine::IDENTITY, 1.0, &mut rec)
            .is_err()
        {
            rec.out.push_str("evaluation error\n");
        }
    }
    Some(rec.out.into_bytes())
}
