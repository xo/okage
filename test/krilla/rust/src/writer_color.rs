//! The calls of the writers of `color.rs` and `functions.rs` of pdf-writer.
//!
//! An object whose field "start" begins with "color:" holds a script in JSON
//! after the colon: `[writer, calls]`, or `[writer, calls, ops]` for a
//! PostScript function. The writer is a method of `Chunk`, such as
//! "shading_pattern" or "icc_profile", or "separation_info" or
//! "output_intent" for a writer that `Obj::start` starts. Each call is a list
//! `[method, arguments...]`. An argument that is a writer is the list of the
//! calls of that writer, or the one call of a `ColorSpace`, which takes self.
//! A null for a `ColorSpace` leaves it unused. The ops of a PostScript
//! function are the data of its stream, as `PostScriptOp::encode` writes
//! them. The Go side is the function goColor in
//! `test/krilla/pdfwriter_color_test.go`.
//!
//! The calls run first. Then the filter and the pairs of the object are
//! written, as `write_object` writes them for the other typed writers.
//!
//! `DeviceColorSpace` is not public in pdf-writer, so no script calls
//! `alternate_device`.

use pdf_writer::types::{
    DeviceNSubtype, FunctionShadingType, InterpolationOrder, OutputIntentSubtype,
    PaintType, PostScriptOp, TilingType,
};
use pdf_writer::writers::{
    ColorSpace, DeviceN, DeviceNAttrs, DeviceNMixingHints, DeviceNProcess,
    ExponentialFunction, FunctionShading, IccProfile, OutputIntent, PostScriptFunction,
    SampledFunction, Separation, SeparationInfo, ShadingPattern, StitchingFunction,
    StreamShading, StreamShadingType, TilingPattern,
};
use pdf_writer::{Chunk, Filter, Name, Rect, Ref, TextStr};
use serde_json::Value as J;

use super::{hex, reference, write_value, Value};

/// The prefix of the field "start" of a script.
pub const PREFIX: &str = "color:";

/// write writes the object of a script into a chunk.
pub fn write(
    c: &mut Chunk,
    id: Ref,
    start: &str,
    data: Option<Vec<u8>>,
    filter: Option<Filter>,
    pairs: &[(String, Value)],
) -> Result<(), String> {
    let script: J = serde_json::from_str(&start[PREFIX.len()..])
        .map_err(|e| format!("parsing the script {start:?}: {e}"))?;
    let s = arr(&script)?;
    let kind = st(arg(s, 0)?)?;
    let calls = arg(s, 1)?;

    macro_rules! pairs {
        ($w:expr) => {
            for (k, v) in pairs {
                write_value($w.insert(Name(k.as_bytes())), v)?;
            }
        };
    }
    macro_rules! stream {
        ($w:expr) => {
            if let Some(f) = filter {
                $w.filter(f);
            }
            pairs!($w);
        };
    }

    match (kind, data) {
        ("color_space", None) => color_space(c.color_space(id), calls)?,
        ("function_shading", None) => {
            let mut w = c.function_shading(id);
            function_shading(&mut w, calls)?;
            pairs!(w);
        }
        ("shading_pattern", None) => {
            let mut w = c.shading_pattern(id);
            shading_pattern(&mut w, calls)?;
            pairs!(w);
        }
        ("exponential_function", None) => {
            let mut w = c.exponential_function(id);
            exponential(&mut w, calls)?;
            pairs!(w);
        }
        ("stitching_function", None) => {
            let mut w = c.stitching_function(id);
            stitching(&mut w, calls)?;
            pairs!(w);
        }
        ("separation_info", None) => {
            separation_info(&mut c.indirect(id).start::<SeparationInfo>(), calls)?
        }
        ("output_intent", None) => {
            output_intent(&mut c.indirect(id).start::<OutputIntent>(), calls)?
        }
        ("stream_shading", Some(d)) => {
            let mut w = c.stream_shading(id, &d);
            stream_shading(&mut w, calls)?;
            stream!(w);
        }
        ("tiling_pattern", Some(d)) => {
            let mut w = c.tiling_pattern(id, &d);
            tiling_pattern(&mut w, calls)?;
            stream!(w);
        }
        ("icc_profile", Some(d)) => {
            let mut w = c.icc_profile(id, &d);
            icc_profile(&mut w, calls)?;
            stream!(w);
        }
        ("sampled_function", Some(d)) => {
            let mut w = c.sampled_function(id, &d);
            sampled(&mut w, calls)?;
            stream!(w);
        }
        ("post_script_function", Some(d)) => {
            let code = match s.get(2) {
                Some(ops) => PostScriptOp::encode(ps_ops(ops)?).into_vec(),
                None => d,
            };
            let mut w = c.post_script_function(id, &code);
            post_script(&mut w, calls)?;
            stream!(w);
        }
        _ => return Err(format!("object {}: the start {start:?} is not known", id.get())),
    }
    Ok(())
}

/// color_space makes the one call of a color space.
fn color_space(cs: ColorSpace<'_>, call: &J) -> Result<(), String> {
    if call.is_null() {
        return Ok(());
    }
    let (name, a) = split(call)?;
    match name {
        "cal_rgb" => cs.cal_rgb(
            f3(arg(a, 0)?)?,
            opt(arg(a, 1)?, f3)?,
            opt(arg(a, 2)?, f3)?,
            opt(arg(a, 3)?, f9)?,
        ),
        "cal_gray" => cs.cal_gray(f3(arg(a, 0)?)?, opt(arg(a, 1)?, f3)?, opt(arg(a, 2)?, f)?),
        "lab" => cs.lab(f3(arg(a, 0)?)?, opt(arg(a, 1)?, f3)?, opt(arg(a, 2)?, f4)?),
        "icc_based" => cs.icc_based(r(arg(a, 0)?)?),
        "srgb" => cs.srgb(),
        "adobe_rgb" => cs.adobe_rgb(),
        "display_p3" => cs.display_p3(),
        "pro_photo" => cs.pro_photo(),
        "eci_rgb" => cs.eci_rgb(),
        "ntsc" => cs.ntsc(),
        "pal" => cs.pal(),
        "d65_gray" => cs.d65_gray(),
        "d50_gray" => cs.d50_gray(opt(arg(a, 0)?, f)?),
        "c_gray" => cs.c_gray(),
        "e_gray" => cs.e_gray(opt(arg(a, 0)?, f)?),
        "device_rgb" => cs.device_rgb(),
        "device_cmyk" => cs.device_cmyk(),
        "device_gray" => cs.device_gray(),
        "separation" => {
            let n = st(arg(a, 0)?)?;
            separation(&mut cs.separation(Name(n.as_bytes())), arg(a, 1)?)?
        }
        "device_n" => {
            let names = strings(arg(a, 0)?)?;
            device_n(&mut cs.device_n(names.iter().map(|n| Name(n.as_bytes()))), arg(a, 1)?)?
        }
        "indexed" => {
            let base = st(arg(a, 0)?)?;
            cs.indexed(Name(base.as_bytes()), i(arg(a, 1)?)?, &hex(st(arg(a, 2)?)?)?)
        }
        "pattern" => cs.pattern(Name(st(arg(a, 0)?)?.as_bytes())),
        _ => return Err(format!("the call {call} of ColorSpace is not known")),
    }
    Ok(())
}

fn separation(w: &mut Separation<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "alternate_color_space" => color_space(w.alternate_color_space(), arg(a, 0)?)?,
            "alternate_color_space_ref" => {
                w.alternate_color_space_ref(r(arg(a, 0)?)?);
            }
            "tint_exponential" => exponential(&mut w.tint_exponential(), arg(a, 0)?)?,
            "tint_stitching" => stitching(&mut w.tint_stitching(), arg(a, 0)?)?,
            "tint_ref" => {
                w.tint_ref(r(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of Separation is not known")),
        }
    }
    Ok(())
}

fn device_n(w: &mut DeviceN<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "alternate_color_space" => color_space(w.alternate_color_space(), arg(a, 0)?)?,
            "alternate_color_space_ref" => {
                w.alternate_color_space_ref(r(arg(a, 0)?)?);
            }
            "tint_exponential" => exponential(&mut w.tint_exponential(), arg(a, 0)?)?,
            "tint_stitching" => stitching(&mut w.tint_stitching(), arg(a, 0)?)?,
            "tint_ref" => {
                w.tint_ref(r(arg(a, 0)?)?);
            }
            "attrs" => attrs(&mut w.attrs(), arg(a, 0)?)?,
            _ => return Err(format!("the call {call} of DeviceN is not known")),
        }
    }
    Ok(())
}

fn attrs(w: &mut DeviceNAttrs<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "subtype" => {
                w.subtype(match st(arg(a, 0)?)? {
                    "DeviceN" => DeviceNSubtype::DeviceN,
                    "NChannel" => DeviceNSubtype::NChannel,
                    s => return Err(format!("the subtype {s:?} is not known")),
                });
            }
            "colorants" => {
                let mut d = w.colorants();
                for entry in arr(arg(a, 0)?)? {
                    let e = arr(entry)?;
                    let pairs: Vec<(String, Value)> = serde_json::from_value(arg(e, 1)?.clone())
                        .map_err(|err| format!("the pairs {entry}: {err}"))?;
                    let mut inner = d.insert(Name(st(arg(e, 0)?)?.as_bytes()));
                    for (k, v) in &pairs {
                        write_value(inner.insert(Name(k.as_bytes())), v)?;
                    }
                }
            }
            "process" => process(&mut w.process(), arg(a, 0)?)?,
            "mixing_hints" => mixing_hints(&mut w.mixing_hints(), arg(a, 0)?)?,
            _ => return Err(format!("the call {call} of DeviceNAttrs is not known")),
        }
    }
    Ok(())
}

fn process(w: &mut DeviceNProcess<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "color_space" => {
                w.color_space(Name(st(arg(a, 0)?)?.as_bytes()));
            }
            "color_space_array" => color_space(w.color_space_array(), arg(a, 0)?)?,
            "components" => {
                let names = strings(arg(a, 0)?)?;
                w.components(names.iter().map(|n| Name(n.as_bytes())));
            }
            _ => return Err(format!("the call {call} of DeviceNProcess is not known")),
        }
    }
    Ok(())
}

fn mixing_hints(w: &mut DeviceNMixingHints<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "solidities" | "dot_gain" => {
                let mut d = if name == "solidities" { w.solidities() } else { w.dot_gain() };
                for entry in arr(arg(a, 0)?)? {
                    let e = arr(entry)?;
                    d.pair(Name(st(arg(e, 0)?)?.as_bytes()), f(arg(e, 1)?)?);
                }
            }
            "printing_order" => {
                let names = strings(arg(a, 0)?)?;
                w.printing_order(names.iter().map(|n| Name(n.as_bytes())));
            }
            _ => return Err(format!("the call {call} of DeviceNMixingHints is not known")),
        }
    }
    Ok(())
}

fn tiling_pattern(w: &mut TilingPattern<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "paint_type" => {
                w.paint_type(match st(arg(a, 0)?)? {
                    "Colored" => PaintType::Colored,
                    "Uncolored" => PaintType::Uncolored,
                    s => return Err(format!("the paint type {s:?} is not known")),
                });
            }
            "tiling_type" => {
                w.tiling_type(match st(arg(a, 0)?)? {
                    "ConstantSpacing" => TilingType::ConstantSpacing,
                    "NoDistortion" => TilingType::NoDistortion,
                    "FastConstantSpacing" => TilingType::FastConstantSpacing,
                    s => return Err(format!("the tiling type {s:?} is not known")),
                });
            }
            "bbox" => {
                w.bbox(rect(arg(a, 0)?)?);
            }
            "x_step" => {
                w.x_step(f(arg(a, 0)?)?);
            }
            "y_step" => {
                w.y_step(f(arg(a, 0)?)?);
            }
            "matrix" => {
                w.matrix(f6(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of TilingPattern is not known")),
        }
    }
    Ok(())
}

fn shading_pattern(w: &mut ShadingPattern<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "function_shading" => function_shading(&mut w.function_shading(), arg(a, 0)?)?,
            "shading_ref" => {
                w.shading_ref(r(arg(a, 0)?)?);
            }
            "matrix" => {
                w.matrix(f6(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of ShadingPattern is not known")),
        }
    }
    Ok(())
}

fn function_shading(w: &mut FunctionShading<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "shading_type" => {
                w.shading_type(match st(arg(a, 0)?)? {
                    "Function" => FunctionShadingType::Function,
                    "Axial" => FunctionShadingType::Axial,
                    "Radial" => FunctionShadingType::Radial,
                    s => return Err(format!("the shading type {s:?} is not known")),
                });
            }
            "color_space" => color_space(w.color_space(), arg(a, 0)?)?,
            "background" => {
                w.background(fs(arg(a, 0)?)?);
            }
            "bbox" => {
                w.bbox(rect(arg(a, 0)?)?);
            }
            "anti_alias" => {
                w.anti_alias(b(arg(a, 0)?)?);
            }
            "domain" => {
                w.domain(f4(arg(a, 0)?)?);
            }
            "matrix" => {
                w.matrix(f6(arg(a, 0)?)?);
            }
            "function" => {
                w.function(r(arg(a, 0)?)?);
            }
            "coords" => {
                w.coords(fs(arg(a, 0)?)?);
            }
            "extend" => {
                let e = arr(arg(a, 0)?)?;
                w.extend([b(arg(e, 0)?)?, b(arg(e, 1)?)?]);
            }
            _ => return Err(format!("the call {call} of FunctionShading is not known")),
        }
    }
    Ok(())
}

fn stream_shading(w: &mut StreamShading<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "shading_type" => {
                w.shading_type(match st(arg(a, 0)?)? {
                    "FreeformGouraud" => StreamShadingType::FreeformGouraud,
                    "LatticeGouraud" => StreamShadingType::LatticeGouraud,
                    "CoonsPatch" => StreamShadingType::CoonsPatch,
                    "TensorProductPatch" => StreamShadingType::TensorProductPatch,
                    s => return Err(format!("the shading type {s:?} is not known")),
                });
            }
            "color_space" => color_space(w.color_space(), arg(a, 0)?)?,
            "background" => {
                w.background(fs(arg(a, 0)?)?);
            }
            "bbox" => {
                w.bbox(rect(arg(a, 0)?)?);
            }
            "anti_alias" => {
                w.anti_alias(b(arg(a, 0)?)?);
            }
            "function" => {
                w.function(r(arg(a, 0)?)?);
            }
            "bits_per_coordinate" => {
                w.bits_per_coordinate(i(arg(a, 0)?)?);
            }
            "bits_per_component" => {
                w.bits_per_component(i(arg(a, 0)?)?);
            }
            "bits_per_flag" => {
                w.bits_per_flag(i(arg(a, 0)?)?);
            }
            "decode" => {
                w.decode(fs(arg(a, 0)?)?);
            }
            "vertices_per_row" => {
                w.vertices_per_row(i(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of StreamShading is not known")),
        }
    }
    Ok(())
}

fn icc_profile(w: &mut IccProfile<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "n" => {
                w.n(i(arg(a, 0)?)?);
            }
            "alternate" => color_space(w.alternate(), arg(a, 0)?)?,
            "alternate_name" => {
                w.alternate_name(Name(st(arg(a, 0)?)?.as_bytes()));
            }
            "range" => {
                w.range(fs(arg(a, 0)?)?);
            }
            "metadata" => {
                w.metadata(r(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of IccProfile is not known")),
        }
    }
    Ok(())
}

fn separation_info(w: &mut SeparationInfo<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "pages" => {
                let refs = arr(arg(a, 0)?)?.iter().map(r).collect::<Result<Vec<_>, _>>()?;
                w.pages(refs);
            }
            "device_colorant" => {
                w.device_colorant(Name(st(arg(a, 0)?)?.as_bytes()));
            }
            "device_colorant_str" => {
                w.device_colorant_str(st(arg(a, 0)?)?);
            }
            "color_space" => color_space(w.color_space(), arg(a, 0)?)?,
            _ => return Err(format!("the call {call} of SeparationInfo is not known")),
        }
    }
    Ok(())
}

fn output_intent(w: &mut OutputIntent<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "subtype" => {
                w.subtype(match st(arg(a, 0)?)? {
                    "PDFX" => OutputIntentSubtype::PDFX,
                    "PDFA" => OutputIntentSubtype::PDFA,
                    "PDFE" => OutputIntentSubtype::PDFE,
                    "Custom" => OutputIntentSubtype::Custom(Name(st(arg(a, 1)?)?.as_bytes())),
                    s => return Err(format!("the subtype {s:?} is not known")),
                });
            }
            "output_condition" => {
                w.output_condition(TextStr(st(arg(a, 0)?)?));
            }
            "output_condition_identifier" => {
                w.output_condition_identifier(TextStr(st(arg(a, 0)?)?));
            }
            "registry_name" => {
                w.registry_name(TextStr(st(arg(a, 0)?)?));
            }
            "info" => {
                w.info(TextStr(st(arg(a, 0)?)?));
            }
            "dest_output_profile" => {
                w.dest_output_profile(r(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of OutputIntent is not known")),
        }
    }
    Ok(())
}

fn sampled(w: &mut SampledFunction<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "domain" => {
                w.domain(fs(arg(a, 0)?)?);
            }
            "range" => {
                w.range(fs(arg(a, 0)?)?);
            }
            "size" => {
                let size = arr(arg(a, 0)?)?.iter().map(i).collect::<Result<Vec<_>, _>>()?;
                w.size(size);
            }
            "bits_per_sample" => {
                w.bits_per_sample(i(arg(a, 0)?)?);
            }
            "order" => {
                w.order(match st(arg(a, 0)?)? {
                    "Linear" => InterpolationOrder::Linear,
                    "Cubic" => InterpolationOrder::Cubic,
                    s => return Err(format!("the order {s:?} is not known")),
                });
            }
            "encode" => {
                w.encode(fs(arg(a, 0)?)?);
            }
            "decode" => {
                w.decode(fs(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of SampledFunction is not known")),
        }
    }
    Ok(())
}

fn exponential(w: &mut ExponentialFunction<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "domain" => {
                w.domain(fs(arg(a, 0)?)?);
            }
            "range" => {
                w.range(fs(arg(a, 0)?)?);
            }
            "c0" => {
                w.c0(fs(arg(a, 0)?)?);
            }
            "c1" => {
                w.c1(fs(arg(a, 0)?)?);
            }
            "n" => {
                w.n(f(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of ExponentialFunction is not known")),
        }
    }
    Ok(())
}

fn stitching(w: &mut StitchingFunction<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "domain" => {
                w.domain(fs(arg(a, 0)?)?);
            }
            "range" => {
                w.range(fs(arg(a, 0)?)?);
            }
            "functions" => {
                let refs = arr(arg(a, 0)?)?.iter().map(r).collect::<Result<Vec<_>, _>>()?;
                w.functions(refs);
            }
            "bounds" => {
                w.bounds(fs(arg(a, 0)?)?);
            }
            "encode" => {
                w.encode(fs(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of StitchingFunction is not known")),
        }
    }
    Ok(())
}

fn post_script(w: &mut PostScriptFunction<'_>, calls: &J) -> Result<(), String> {
    for call in arr(calls)? {
        let (name, a) = split(call)?;
        match name {
            "domain" => {
                w.domain(fs(arg(a, 0)?)?);
            }
            "range" => {
                w.range(fs(arg(a, 0)?)?);
            }
            _ => return Err(format!("the call {call} of PostScriptFunction is not known")),
        }
    }
    Ok(())
}

/// ps_ops reads a list of PostScript operators: {"real": x}, {"int": n},
/// {"if": [ops]}, {"ifelse": [[ops], [ops]]}, or the name of an operator.
/// The lists live until the oracle exits.
fn ps_ops(j: &J) -> Result<&'static [PostScriptOp<'static>], String> {
    let mut ops = Vec::new();
    for op in arr(j)? {
        ops.push(match op {
            J::String(s) => match s.as_str() {
                "abs" => PostScriptOp::Abs,
                "add" => PostScriptOp::Add,
                "atan" => PostScriptOp::Atan,
                "ceiling" => PostScriptOp::Ceiling,
                "cos" => PostScriptOp::Cos,
                "cvi" => PostScriptOp::Cvi,
                "cvr" => PostScriptOp::Cvr,
                "div" => PostScriptOp::Div,
                "exp" => PostScriptOp::Exp,
                "floor" => PostScriptOp::Floor,
                "idiv" => PostScriptOp::Idiv,
                "ln" => PostScriptOp::Ln,
                "log" => PostScriptOp::Log,
                "mod" => PostScriptOp::Mod,
                "mul" => PostScriptOp::Mul,
                "neg" => PostScriptOp::Neg,
                "round" => PostScriptOp::Round,
                "sin" => PostScriptOp::Sin,
                "sqrt" => PostScriptOp::Sqrt,
                "sub" => PostScriptOp::Sub,
                "truncate" => PostScriptOp::Truncate,
                "and" => PostScriptOp::And,
                "bitshift" => PostScriptOp::Bitshift,
                "eq" => PostScriptOp::Eq,
                "false" => PostScriptOp::False,
                "ge" => PostScriptOp::Ge,
                "gt" => PostScriptOp::Gt,
                "le" => PostScriptOp::Le,
                "lt" => PostScriptOp::Lt,
                "ne" => PostScriptOp::Ne,
                "not" => PostScriptOp::Not,
                "or" => PostScriptOp::Or,
                "true" => PostScriptOp::True,
                "xor" => PostScriptOp::Xor,
                "copy" => PostScriptOp::Copy,
                "dup" => PostScriptOp::Dup,
                "exch" => PostScriptOp::Exch,
                "index" => PostScriptOp::Index,
                "pop" => PostScriptOp::Pop,
                "roll" => PostScriptOp::Roll,
                _ => return Err(format!("the PostScript operator {op} is not known")),
            },
            J::Object(m) if m.len() == 1 => {
                let (k, v) = m.iter().next().unwrap();
                match k.as_str() {
                    "real" => PostScriptOp::Real(f(v)?),
                    "int" => PostScriptOp::Integer(i(v)?),
                    "if" => PostScriptOp::If(ps_ops(v)?),
                    "ifelse" => {
                        let both = arr(v)?;
                        PostScriptOp::IfElse(ps_ops(arg(both, 0)?)?, ps_ops(arg(both, 1)?)?)
                    }
                    _ => return Err(format!("the PostScript operator {op} is not known")),
                }
            }
            _ => return Err(format!("the PostScript operator {op} is not known")),
        });
    }
    Ok(Box::leak(ops.into_boxed_slice()))
}

fn split(call: &J) -> Result<(&str, &[J]), String> {
    let a = arr(call)?;
    Ok((st(arg(a, 0)?)?, &a[1..]))
}

fn arg(a: &[J], n: usize) -> Result<&J, String> {
    a.get(n).ok_or_else(|| format!("argument {n} is missing"))
}

fn arr(j: &J) -> Result<&Vec<J>, String> {
    j.as_array().ok_or_else(|| format!("{j} is not a list"))
}

fn st(j: &J) -> Result<&str, String> {
    j.as_str().ok_or_else(|| format!("{j} is not a string"))
}

fn b(j: &J) -> Result<bool, String> {
    j.as_bool().ok_or_else(|| format!("{j} is not a bool"))
}

/// i reads an i32, as serde reads an i64 and `as` cuts it.
fn i(j: &J) -> Result<i32, String> {
    j.as_i64().map(|x| x as i32).ok_or_else(|| format!("{j} is not an integer"))
}

/// f reads an f32, as serde reads an f64 and rounds it.
fn f(j: &J) -> Result<f32, String> {
    j.as_f64().map(|x| x as f32).ok_or_else(|| format!("{j} is not a number"))
}

fn r(j: &J) -> Result<Ref, String> {
    reference(i(j)?)
}

fn fs(j: &J) -> Result<Vec<f32>, String> {
    arr(j)?.iter().map(f).collect()
}

fn fixed<const N: usize>(j: &J) -> Result<[f32; N], String> {
    fs(j)?.try_into().map_err(|v: Vec<f32>| format!("{j} has {} numbers, not {N}", v.len()))
}

fn f3(j: &J) -> Result<[f32; 3], String> {
    fixed(j)
}

fn f4(j: &J) -> Result<[f32; 4], String> {
    fixed(j)
}

fn f6(j: &J) -> Result<[f32; 6], String> {
    fixed(j)
}

fn f9(j: &J) -> Result<[f32; 9], String> {
    fixed(j)
}

fn rect(j: &J) -> Result<Rect, String> {
    let [x1, y1, x2, y2] = f4(j)?;
    Ok(Rect::new(x1, y1, x2, y2))
}

fn strings(j: &J) -> Result<Vec<String>, String> {
    arr(j)?.iter().map(|s| st(s).map(str::to_string)).collect()
}

fn opt<T>(j: &J, read: fn(&J) -> Result<T, String>) -> Result<Option<T>, String> {
    if j.is_null() {
        Ok(None)
    } else {
        read(j).map(Some)
    }
}
