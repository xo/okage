//! The PDF oracle of okage (D22, D62).
//!
//! It reads one question as JSON on each line of its input and writes one
//! answer for each: four bytes with the length of the answer, little-endian,
//! then the answer. The first byte of an answer is 0 for a result and 1 for
//! an error. The rest of an error is its message in UTF-8. A panic of a crate
//! is an error with the message of the panic. There are five kinds of
//! question, which `test/krilla/doc.go` documents:
//!
//! 1. "subset": the subset of a font by subsetter, with the tag that krilla
//!    gives to its name. The module `subset` answers it.
//! 2. "pdf_writer": the bytes of a list of calls of pdf-writer. The module
//!    `writer` answers it.
//! 3. "svg": the PDF file that krilla-svg writes for an SVG file.
//! 4. "krilla": the PDF file that krilla writes for a scene of calls of its
//!    surface. The module `scene` answers both 3 and 4. The module `image`
//!    makes the images that `scene` does not make itself, and the module
//!    `interchange` adds the metadata, the outline and the links of a scene.
//! 5. "point_ts": the rotation of get_point_ts in krilla, which calls atan2,
//!    sin and cos of the C library. The function `point_ts` answers it.
//!
//! The Go tests write each number of a float32 as the shortest decimal of a
//! float64 that gives back the float32, as the oracle of test/oracle says.

mod image;
mod interchange;
mod scene;
mod subset;
mod writer;

use std::io::{self, BufRead, Write};
use std::panic::{self, AssertUnwindSafe};

use serde::Deserialize;

#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum Question {
    Subset(subset::Question),
    PdfWriter(writer::Question),
    Svg(scene::SvgQuestion),
    Krilla(scene::Scene),
    PointTs(Vec<[f32; 4]>),
}

fn main() {
    // A panic is an answer, so its message is not printed.
    panic::set_hook(Box::new(|_| {}));
    let stdin = io::stdin();
    let mut out = io::BufWriter::new(io::stdout().lock());
    for line in stdin.lock().lines() {
        let line = line.expect("reading a question");
        if line.trim().is_empty() {
            continue;
        }
        let result = match serde_json::from_str::<Question>(&line) {
            Ok(q) => panic::catch_unwind(AssertUnwindSafe(|| answer(&q))).unwrap_or_else(|p| {
                let msg = p
                    .downcast_ref::<&str>()
                    .map(|s| s.to_string())
                    .or_else(|| p.downcast_ref::<String>().cloned())
                    .unwrap_or_default();
                Err(format!("panic: {msg}"))
            }),
            Err(e) => Err(format!("parsing the question: {e}")),
        };
        let data = match result {
            Ok(mut data) => {
                data.insert(0, 0);
                data
            }
            Err(msg) => {
                let mut data = vec![1];
                data.extend_from_slice(msg.as_bytes());
                data
            }
        };
        out.write_all(&(data.len() as u32).to_le_bytes()).unwrap();
        out.write_all(&data).unwrap();
        out.flush().unwrap();
    }
}

fn answer(q: &Question) -> Result<Vec<u8>, String> {
    match q {
        Question::Subset(q) => subset::answer(q),
        Question::PdfWriter(q) => writer::answer(q),
        Question::Svg(q) => scene::answer_svg(q),
        Question::Krilla(q) => scene::answer_scene(q),
        Question::PointTs(q) => Ok(point_ts(q)),
    }
}

/// Answers the question "point_ts". For each line from x1, y1 to x2, y2, it
/// computes the rotation of get_point_ts in
/// krilla/src/graphics/shading_function.rs, and writes its six numbers sx,
/// ky, kx, sy, tx and ty as f32, little-endian.
fn point_ts(lines: &[[f32; 4]]) -> Vec<u8> {
    let mut data = Vec::with_capacity(lines.len() * 24);
    for l in lines {
        let dx = l[2] - l[0];
        let dy = l[3] - l[1];
        let angle = dy.atan2(dx).to_degrees();
        let ts = krilla::geom::Transform::from_rotate_at(angle, l[0], l[1]);
        for v in [ts.sx(), ts.ky(), ts.kx(), ts.sy(), ts.tx(), ts.ty()] {
            data.extend_from_slice(&v.to_le_bytes());
        }
    }
    data
}
