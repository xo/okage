//! The typed calls of `content.rs` of pdf-writer, for the question
//! "pdf_writer".
//!
//! An operation of a content stream whose "op" is the name of a method of
//! `Content`, such as "set_line_width" or "show_positioned", calls that
//! method. Its operands are the arguments of the method, in their order. A
//! method that starts a writer, such as `show_positioned`, takes its fixed
//! arguments first. Each operand after them is a call on the writer: an
//! array whose first item is the name of the method, such as
//! `{"array": [{"name": "adjust"}, {"real": 2}]}`. The calls nest in the same
//! way. The names "ext_graphics" and "resources" write an operation whose
//! operator is their first operand, and whose one operand is an
//! `ExtGraphicsState` or a `Resources` that the calls after it write.
//!
//! The arguments have these forms:
//!
//! 1. A number is a "real", and an integer of `i32` is an "int".
//! 2. A list of numbers, a `Rect` and a matrix are an "array" of "real".
//! 3. An enum without data is a "name" with the name of its variant. An
//!    enum that `to_name` writes, such as `BlendMode`, takes the name of PDF
//!    that it writes, such as "ColorDodge" or "ImageB".
//! 4. A `ColorSpaceOperand` or an `ArtifactSubtype` is a "name" for the
//!    variants `Named` and `Custom`, and a "text" with the name of the Rust
//!    variant for the others, such as {"text": "DeviceRgb"}.
//! 5. A writer that derefs to an `Operation` takes the call "obj", which
//!    writes its one value with `Operation::obj`. A writer that derefs to a
//!    `Dict` takes the call "pair", with a key and a value, and one that
//!    derefs to an `Array` takes the call "item", with a value.

use pdf_writer::types::{
    ArtifactAttachment, ArtifactSubtype, ArtifactType, BlendMode, ColorSpaceOperand,
    LineCapStyle, LineJoinStyle, MaskType, OverprintMode, ProcSet, RenderingIntent,
    TextRenderingMode,
};
use pdf_writer::writers::{
    Artifact, ExtGraphicsState, MarkContent, PositionedItems, PropertyList, Resources,
    ShowPositioned, SoftMask,
};
use pdf_writer::{Content, Dict, Name, Rect, Ref, Str, TextStr};

use super::{hex, reference, write_value, Operation, Value};

/// call makes the typed call of an operation. It returns false when the
/// operation is not a typed call.
pub(super) fn call(c: &mut Content, op: &Operation) -> Result<bool, String> {
    let a = &op.operands;
    match op.op.as_str() {
        "set_line_width" => {
            c.set_line_width(real(a, 0)?);
        }
        "set_line_cap" => {
            c.set_line_cap(line_cap(name(a, 0)?)?);
        }
        "set_line_join" => {
            c.set_line_join(line_join(name(a, 0)?)?);
        }
        "set_miter_limit" => {
            c.set_miter_limit(real(a, 0)?);
        }
        "set_dash_pattern" => {
            c.set_dash_pattern(reals(a, 0)?, real(a, 1)?);
        }
        "set_rendering_intent" => {
            c.set_rendering_intent(rendering_intent(name(a, 0)?)?);
        }
        "set_flatness" => {
            c.set_flatness(int(a, 0)?);
        }
        "set_parameters" => {
            c.set_parameters(Name(name(a, 0)?.as_bytes()));
        }
        "save_state" => {
            c.save_state();
        }
        "restore_state" => {
            c.restore_state();
        }
        "transform" => {
            c.transform(matrix(a, 0)?);
        }
        "move_to" => {
            c.move_to(real(a, 0)?, real(a, 1)?);
        }
        "line_to" => {
            c.line_to(real(a, 0)?, real(a, 1)?);
        }
        "cubic_to" => {
            c.cubic_to(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?, real(a, 4)?, real(a, 5)?);
        }
        "cubic_to_initial" => {
            c.cubic_to_initial(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?);
        }
        "cubic_to_final" => {
            c.cubic_to_final(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?);
        }
        "close_path" => {
            c.close_path();
        }
        "rect" => {
            c.rect(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?);
        }
        "stroke" => {
            c.stroke();
        }
        "close_and_stroke" => {
            c.close_and_stroke();
        }
        "fill_nonzero" => {
            c.fill_nonzero();
        }
        "fill_even_odd" => {
            c.fill_even_odd();
        }
        "fill_nonzero_and_stroke" => {
            c.fill_nonzero_and_stroke();
        }
        "fill_even_odd_and_stroke" => {
            c.fill_even_odd_and_stroke();
        }
        "close_fill_nonzero_and_stroke" => {
            c.close_fill_nonzero_and_stroke();
        }
        "close_fill_even_odd_and_stroke" => {
            c.close_fill_even_odd_and_stroke();
        }
        "end_path" => {
            c.end_path();
        }
        "clip_nonzero" => {
            c.clip_nonzero();
        }
        "clip_even_odd" => {
            c.clip_even_odd();
        }
        "begin_text" => {
            c.begin_text();
        }
        "end_text" => {
            c.end_text();
        }
        "set_char_spacing" => {
            c.set_char_spacing(real(a, 0)?);
        }
        "set_word_spacing" => {
            c.set_word_spacing(real(a, 0)?);
        }
        "set_horizontal_scaling" => {
            c.set_horizontal_scaling(real(a, 0)?);
        }
        "set_leading" => {
            c.set_leading(real(a, 0)?);
        }
        "set_font" => {
            c.set_font(Name(name(a, 0)?.as_bytes()), real(a, 1)?);
        }
        "set_text_rendering_mode" => {
            c.set_text_rendering_mode(text_rendering_mode(name(a, 0)?)?);
        }
        "set_rise" => {
            c.set_rise(real(a, 0)?);
        }
        "next_line" => {
            c.next_line(real(a, 0)?, real(a, 1)?);
        }
        "next_line_and_set_leading" => {
            c.next_line_and_set_leading(real(a, 0)?, real(a, 1)?);
        }
        "set_text_matrix" => {
            c.set_text_matrix(matrix(a, 0)?);
        }
        "next_line_using_leading" => {
            c.next_line_using_leading();
        }
        "show" => {
            c.show(Str(&string(a, 0)?));
        }
        "next_line_show" => {
            c.next_line_show(Str(&string(a, 0)?));
        }
        "next_line_show_and_set_word_and_char_spacing" => {
            c.next_line_show_and_set_word_and_char_spacing(
                real(a, 0)?,
                real(a, 1)?,
                Str(&string(a, 2)?),
            );
        }
        "show_positioned" => show_positioned(c.show_positioned(), a)?,
        "start_color_glyph" => {
            c.start_color_glyph(real(a, 0)?);
        }
        "start_shape_glyph" => {
            c.start_shape_glyph(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?, real(a, 4)?);
        }
        "set_stroke_color_space" => {
            c.set_stroke_color_space(color_space(arg(a, 0)?)?);
        }
        "set_fill_color_space" => {
            c.set_fill_color_space(color_space(arg(a, 0)?)?);
        }
        "set_stroke_color" => {
            c.set_stroke_color(reals(a, 0)?);
        }
        "set_stroke_pattern" => {
            c.set_stroke_pattern(reals(a, 0)?, Name(name(a, 1)?.as_bytes()));
        }
        "set_fill_color" => {
            c.set_fill_color(reals(a, 0)?);
        }
        "set_fill_pattern" => {
            c.set_fill_pattern(reals(a, 0)?, Name(name(a, 1)?.as_bytes()));
        }
        "set_stroke_gray" => {
            c.set_stroke_gray(real(a, 0)?);
        }
        "set_fill_gray" => {
            c.set_fill_gray(real(a, 0)?);
        }
        "set_stroke_rgb" => {
            c.set_stroke_rgb(real(a, 0)?, real(a, 1)?, real(a, 2)?);
        }
        "set_fill_rgb" => {
            c.set_fill_rgb(real(a, 0)?, real(a, 1)?, real(a, 2)?);
        }
        "set_stroke_cmyk" => {
            c.set_stroke_cmyk(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?);
        }
        "set_fill_cmyk" => {
            c.set_fill_cmyk(real(a, 0)?, real(a, 1)?, real(a, 2)?, real(a, 3)?);
        }
        "shading" => {
            c.shading(Name(name(a, 0)?.as_bytes()));
        }
        "x_object" => {
            c.x_object(Name(name(a, 0)?.as_bytes()));
        }
        "marked_content_point" => {
            c.marked_content_point(Name(name(a, 0)?.as_bytes()));
        }
        "marked_content_point_with_properties" => {
            let tag = name(a, 0)?;
            mark_content(c.marked_content_point_with_properties(Name(tag.as_bytes())), &a[1..])?
        }
        "begin_marked_content" => {
            c.begin_marked_content(Name(name(a, 0)?.as_bytes()));
        }
        "begin_marked_content_with_properties" => {
            let tag = name(a, 0)?;
            mark_content(c.begin_marked_content_with_properties(Name(tag.as_bytes())), &a[1..])?
        }
        "end_marked_content" => {
            c.end_marked_content();
        }
        "begin_compat" => {
            c.begin_compat();
        }
        "end_compat" => {
            c.end_compat();
        }
        "ext_graphics" => {
            let operator = name(a, 0)?;
            let mut op = c.op(operator);
            ext_graphics(op.obj().start::<ExtGraphicsState>(), &a[1..])?;
        }
        "resources" => {
            let operator = name(a, 0)?;
            let mut op = c.op(operator);
            resources(op.obj().start::<Resources>(), &a[1..])?;
        }
        _ => return Ok(false),
    }
    Ok(true)
}

/// show_positioned makes the calls on a `ShowPositioned`.
fn show_positioned(mut w: ShowPositioned<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "items" => positioned_items(w.items(), a)?,
            "obj" => write_value(w.obj(), arg(a, 0)?)?,
            _ => return Err(format!("ShowPositioned has no call {n:?}")),
        }
    }
    Ok(())
}

/// positioned_items makes the calls on a `PositionedItems`.
fn positioned_items(mut w: PositionedItems<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "show" => {
                w.show(Str(&string(a, 0)?));
            }
            "adjust" => {
                w.adjust(real(a, 0)?);
            }
            "item" => write_value(w.push(), arg(a, 0)?)?,
            _ => return Err(format!("PositionedItems has no call {n:?}")),
        }
    }
    Ok(())
}

/// mark_content makes the calls on a `MarkContent`. The call
/// "properties_named" takes the writer, so it must be the last one.
fn mark_content(mut w: MarkContent<'_>, calls: &[Value]) -> Result<(), String> {
    for (i, v) in calls.iter().enumerate() {
        let (n, a) = sub_call(v)?;
        match n {
            "properties" => property_list(w.properties(), a)?,
            "properties_named" if i == calls.len() - 1 => {
                w.properties_named(Name(name(a, 0)?.as_bytes()));
                return Ok(());
            }
            "obj" => write_value(w.obj(), arg(a, 0)?)?,
            _ => return Err(format!("MarkContent has no call {n:?} at {i}")),
        }
    }
    Ok(())
}

/// property_list makes the calls on a `PropertyList`. The call "artifact"
/// takes the writer, so it must be the last one.
fn property_list(mut w: PropertyList<'_>, calls: &[Value]) -> Result<(), String> {
    for (i, v) in calls.iter().enumerate() {
        let (n, a) = sub_call(v)?;
        match n {
            "identify" => {
                w.identify(int(a, 0)?);
            }
            "actual_text" => {
                w.actual_text(TextStr(text(a, 0)?));
            }
            "artifact" if i == calls.len() - 1 => return artifact(w.artifact(), a),
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("PropertyList has no call {n:?} at {i}")),
        }
    }
    Ok(())
}

/// artifact makes the calls on an `Artifact`.
fn artifact(mut w: Artifact<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "kind" => {
                w.kind(artifact_type(name(a, 0)?)?);
            }
            "subtype" => {
                w.subtype(artifact_subtype(arg(a, 0)?)?);
            }
            "bounding_box" => {
                let r = reals(a, 0)?;
                if r.len() != 4 {
                    return Err("a Rect has four numbers".into());
                }
                w.bounding_box(Rect::new(r[0], r[1], r[2], r[3]));
            }
            "attached" => {
                let mut list = Vec::new();
                for i in 0..a.len() {
                    list.push(artifact_attachment(name(a, i)?)?);
                }
                w.attached(list);
            }
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("Artifact has no call {n:?}")),
        }
    }
    Ok(())
}

/// resources makes the calls on a `Resources`.
fn resources(mut w: Resources<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "x_objects" => dict(w.x_objects(), a)?,
            "fonts" => dict(w.fonts(), a)?,
            "color_spaces" => dict(w.color_spaces(), a)?,
            "patterns" => dict(w.patterns(), a)?,
            "shadings" => dict(w.shadings(), a)?,
            "ext_g_states" => dict(w.ext_g_states(), a)?,
            "proc_sets" => {
                let mut list = Vec::new();
                for i in 0..a.len() {
                    list.push(proc_set(name(a, i)?)?);
                }
                w.proc_sets(list);
            }
            "proc_sets_all" => {
                w.proc_sets_all();
            }
            "properties" => {
                let mut d = w.properties();
                for v in a {
                    let (n, a) = sub_call(v)?;
                    if n != "insert" {
                        return Err(format!("TypedDict<PropertyList> has no call {n:?}"));
                    }
                    let key = name(a, 0)?;
                    property_list(d.insert(Name(key.as_bytes())), &a[1..])?;
                }
            }
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("Resources has no call {n:?}")),
        }
    }
    Ok(())
}

/// dict makes the calls "pair" on a `Dict`.
fn dict(mut w: Dict<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("Dict has no call {n:?}")),
        }
    }
    Ok(())
}

/// ext_graphics makes the calls on an `ExtGraphicsState`.
fn ext_graphics(mut w: ExtGraphicsState<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "line_width" => {
                w.line_width(real(a, 0)?);
            }
            "line_cap" => {
                w.line_cap(line_cap(name(a, 0)?)?);
            }
            "line_join" => {
                w.line_join(line_join(name(a, 0)?)?);
            }
            "miter_limit" => {
                w.miter_limit(real(a, 0)?);
            }
            "dash_pattern" => {
                w.dash_pattern(reals(a, 0)?, real(a, 1)?);
            }
            "rendering_intent" => {
                w.rendering_intent(rendering_intent(name(a, 0)?)?);
            }
            "overprint" => {
                w.overprint(boolean(a, 0)?);
            }
            "overprint_fill" => {
                w.overprint_fill(boolean(a, 0)?);
            }
            "overprint_mode" => {
                w.overprint_mode(overprint_mode(name(a, 0)?)?);
            }
            "font" => {
                w.font(Name(name(a, 0)?.as_bytes()), real(a, 1)?);
            }
            "black_generation" => {
                w.black_generation(refer(a, 0)?);
            }
            "black_generation_default" => {
                w.black_generation_default();
            }
            "undercolor_removal" => {
                w.undercolor_removal(refer(a, 0)?);
            }
            "undercolor_removal_default" => {
                w.undercolor_removal_default();
            }
            "transfer" => {
                w.transfer(refer(a, 0)?);
            }
            "transfer_default" => {
                w.transfer_default();
            }
            "halftone" => {
                w.halftone(refer(a, 0)?);
            }
            "halftone_default" => {
                w.halftone_default();
            }
            "flatness" => {
                w.flatness(real(a, 0)?);
            }
            "smoothness" => {
                w.smoothness(real(a, 0)?);
            }
            "stroke_adjustment" => {
                w.stroke_adjustment(boolean(a, 0)?);
            }
            "blend_mode" => {
                w.blend_mode(blend_mode(name(a, 0)?)?);
            }
            "soft_mask" => soft_mask(w.soft_mask(), a)?,
            "soft_mask_name" => {
                w.soft_mask_name(Name(name(a, 0)?.as_bytes()));
            }
            "stroking_alpha" => {
                w.stroking_alpha(real(a, 0)?);
            }
            "non_stroking_alpha" => {
                w.non_stroking_alpha(real(a, 0)?);
            }
            "alpha_source" => {
                w.alpha_source(boolean(a, 0)?);
            }
            "text_knockout" => {
                w.text_knockout(boolean(a, 0)?);
            }
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("ExtGraphicsState has no call {n:?}")),
        }
    }
    Ok(())
}

/// soft_mask makes the calls on a `SoftMask`.
fn soft_mask(mut w: SoftMask<'_>, calls: &[Value]) -> Result<(), String> {
    for v in calls {
        let (n, a) = sub_call(v)?;
        match n {
            "subtype" => {
                w.subtype(mask_type(name(a, 0)?)?);
            }
            "group" => {
                w.group(refer(a, 0)?);
            }
            "backdrop" => {
                w.backdrop(reals(a, 0)?);
            }
            "transfer_function" => {
                w.transfer_function(refer(a, 0)?);
            }
            "pair" => pair(&mut w, a)?,
            _ => return Err(format!("SoftMask has no call {n:?}")),
        }
    }
    Ok(())
}

/// pair writes the call "pair" of a writer that derefs to a `Dict`.
fn pair(d: &mut Dict<'_>, a: &[Value]) -> Result<(), String> {
    let key = name(a, 0)?;
    write_value(d.insert(Name(key.as_bytes())), arg(a, 1)?)
}

/// sub_call splits a call on a writer into its name and its arguments.
fn sub_call(v: &Value) -> Result<(&str, &[Value]), String> {
    match v {
        Value::Array(items) => match items.first() {
            Some(Value::Name(n)) => Ok((n.as_str(), &items[1..])),
            _ => Err("a call starts with a name".into()),
        },
        _ => Err("a call is an array".into()),
    }
}

fn arg(a: &[Value], i: usize) -> Result<&Value, String> {
    a.get(i).ok_or_else(|| format!("the argument {i} is missing"))
}

fn real(a: &[Value], i: usize) -> Result<f32, String> {
    match arg(a, i)? {
        Value::Real(f) => Ok(*f),
        _ => Err(format!("the argument {i} is not a real")),
    }
}

fn int(a: &[Value], i: usize) -> Result<i32, String> {
    match arg(a, i)? {
        Value::Int(n) => Ok(*n),
        _ => Err(format!("the argument {i} is not an int")),
    }
}

fn boolean(a: &[Value], i: usize) -> Result<bool, String> {
    match arg(a, i)? {
        Value::Bool(b) => Ok(*b),
        _ => Err(format!("the argument {i} is not a bool")),
    }
}

fn name(a: &[Value], i: usize) -> Result<&str, String> {
    match arg(a, i)? {
        Value::Name(n) => Ok(n.as_str()),
        _ => Err(format!("the argument {i} is not a name")),
    }
}

fn text(a: &[Value], i: usize) -> Result<&str, String> {
    match arg(a, i)? {
        Value::Text(s) => Ok(s.as_str()),
        _ => Err(format!("the argument {i} is not a text")),
    }
}

fn string(a: &[Value], i: usize) -> Result<Vec<u8>, String> {
    match arg(a, i)? {
        Value::Str(s) => hex(s),
        _ => Err(format!("the argument {i} is not a str")),
    }
}

fn refer(a: &[Value], i: usize) -> Result<Ref, String> {
    match arg(a, i)? {
        Value::Ref(r) => reference(*r),
        _ => Err(format!("the argument {i} is not a ref")),
    }
}

fn reals(a: &[Value], i: usize) -> Result<Vec<f32>, String> {
    match arg(a, i)? {
        Value::Array(items) => items
            .iter()
            .map(|v| match v {
                Value::Real(f) => Ok(*f),
                _ => Err(format!("the argument {i} is not an array of reals")),
            })
            .collect(),
        _ => Err(format!("the argument {i} is not an array")),
    }
}

fn matrix(a: &[Value], i: usize) -> Result<[f32; 6], String> {
    reals(a, i)?.try_into().map_err(|_| "a matrix has six numbers".to_string())
}

fn line_cap(n: &str) -> Result<LineCapStyle, String> {
    Ok(match n {
        "ButtCap" => LineCapStyle::ButtCap,
        "RoundCap" => LineCapStyle::RoundCap,
        "ProjectingSquareCap" => LineCapStyle::ProjectingSquareCap,
        _ => return Err(format!("the line cap style {n:?} is not known")),
    })
}

fn line_join(n: &str) -> Result<LineJoinStyle, String> {
    Ok(match n {
        "MiterJoin" => LineJoinStyle::MiterJoin,
        "RoundJoin" => LineJoinStyle::RoundJoin,
        "BevelJoin" => LineJoinStyle::BevelJoin,
        _ => return Err(format!("the line join style {n:?} is not known")),
    })
}

fn rendering_intent(n: &str) -> Result<RenderingIntent, String> {
    Ok(match n {
        "AbsoluteColorimetric" => RenderingIntent::AbsoluteColorimetric,
        "RelativeColorimetric" => RenderingIntent::RelativeColorimetric,
        "Saturation" => RenderingIntent::Saturation,
        "Perceptual" => RenderingIntent::Perceptual,
        _ => return Err(format!("the rendering intent {n:?} is not known")),
    })
}

fn text_rendering_mode(n: &str) -> Result<TextRenderingMode, String> {
    Ok(match n {
        "Fill" => TextRenderingMode::Fill,
        "Stroke" => TextRenderingMode::Stroke,
        "FillStroke" => TextRenderingMode::FillStroke,
        "Invisible" => TextRenderingMode::Invisible,
        "FillClip" => TextRenderingMode::FillClip,
        "StrokeClip" => TextRenderingMode::StrokeClip,
        "FillStrokeClip" => TextRenderingMode::FillStrokeClip,
        "Clip" => TextRenderingMode::Clip,
        _ => return Err(format!("the text rendering mode {n:?} is not known")),
    })
}

fn color_space(v: &Value) -> Result<ColorSpaceOperand<'_>, String> {
    Ok(match v {
        Value::Name(n) => ColorSpaceOperand::Named(Name(n.as_bytes())),
        Value::Text(t) => match t.as_str() {
            "DeviceGray" => ColorSpaceOperand::DeviceGray,
            "DeviceRgb" => ColorSpaceOperand::DeviceRgb,
            "DeviceCmyk" => ColorSpaceOperand::DeviceCmyk,
            "Pattern" => ColorSpaceOperand::Pattern,
            _ => return Err(format!("the color space operand {t:?} is not known")),
        },
        _ => return Err("a color space operand is a name or a text".into()),
    })
}

fn artifact_type(n: &str) -> Result<ArtifactType, String> {
    Ok(match n {
        "Pagination" => ArtifactType::Pagination,
        "Layout" => ArtifactType::Layout,
        "Page" => ArtifactType::Page,
        "Background" => ArtifactType::Background,
        "Inline" => ArtifactType::Inline,
        _ => return Err(format!("the artifact type {n:?} is not known")),
    })
}

fn artifact_subtype(v: &Value) -> Result<ArtifactSubtype<'_>, String> {
    Ok(match v {
        Value::Name(n) => ArtifactSubtype::Custom(Name(n.as_bytes())),
        Value::Text(t) => match t.as_str() {
            "Header" => ArtifactSubtype::Header,
            "Footer" => ArtifactSubtype::Footer,
            "Watermark" => ArtifactSubtype::Watermark,
            "PageNumber" => ArtifactSubtype::PageNumber,
            "Bates" => ArtifactSubtype::Bates,
            "LineNumber" => ArtifactSubtype::LineNumber,
            "Redaction" => ArtifactSubtype::Redaction,
            _ => return Err(format!("the artifact subtype {t:?} is not known")),
        },
        _ => return Err("an artifact subtype is a name or a text".into()),
    })
}

fn artifact_attachment(n: &str) -> Result<ArtifactAttachment, String> {
    Ok(match n {
        "Left" => ArtifactAttachment::Left,
        "Top" => ArtifactAttachment::Top,
        "Right" => ArtifactAttachment::Right,
        "Bottom" => ArtifactAttachment::Bottom,
        _ => return Err(format!("the artifact attachment {n:?} is not known")),
    })
}

fn proc_set(n: &str) -> Result<ProcSet, String> {
    Ok(match n {
        "PDF" => ProcSet::Pdf,
        "Text" => ProcSet::Text,
        "ImageB" => ProcSet::ImageGrayscale,
        "ImageC" => ProcSet::ImageColor,
        "ImageI" => ProcSet::ImageIndexed,
        _ => return Err(format!("the procedure set {n:?} is not known")),
    })
}

fn blend_mode(n: &str) -> Result<BlendMode, String> {
    Ok(match n {
        "Normal" => BlendMode::Normal,
        "Multiply" => BlendMode::Multiply,
        "Screen" => BlendMode::Screen,
        "Overlay" => BlendMode::Overlay,
        "Darken" => BlendMode::Darken,
        "Lighten" => BlendMode::Lighten,
        "ColorDodge" => BlendMode::ColorDodge,
        "ColorBurn" => BlendMode::ColorBurn,
        "HardLight" => BlendMode::HardLight,
        "SoftLight" => BlendMode::SoftLight,
        "Difference" => BlendMode::Difference,
        "Exclusion" => BlendMode::Exclusion,
        "Hue" => BlendMode::Hue,
        "Saturation" => BlendMode::Saturation,
        "Color" => BlendMode::Color,
        "Luminosity" => BlendMode::Luminosity,
        _ => return Err(format!("the blend mode {n:?} is not known")),
    })
}

fn overprint_mode(n: &str) -> Result<OverprintMode, String> {
    Ok(match n {
        "OverrideAllColorants" => OverprintMode::OverrideAllColorants,
        "IgnoreZeroChannel" => OverprintMode::IgnoreZeroChannel,
        _ => return Err(format!("the overprint mode {n:?} is not known")),
    })
}

fn mask_type(n: &str) -> Result<MaskType, String> {
    Ok(match n {
        "Alpha" => MaskType::Alpha,
        "Luminosity" => MaskType::Luminosity,
        _ => return Err(format!("the mask type {n:?} is not known")),
    })
}
