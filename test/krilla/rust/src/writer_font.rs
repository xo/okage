//! The typed calls of `font.rs`, `xobject.rs` and `files.rs` of pdf-writer,
//! for unit 6 of phase 12.
//!
//! An object whose `start` is in `STARTS` starts a typed writer, and its
//! `dict` is a list of calls on that writer, in order. A call is a pair of
//! the name of a method of pdf-writer and a value:
//!
//! 1. A method that writes values takes an array of its arguments, such as
//!    `["base_font", {"array": [{"name": "Helvetica"}]}]`. A method that
//!    takes an iterator takes its items as the rest of the array.
//! 2. A method that starts another writer takes the calls on that writer as
//!    a dictionary, such as `["encoding_custom", {"dict": [...]}]`. If the
//!    method also takes arguments, they come first in an array, and the
//!    dictionary of calls is the last item.
//! 3. The call "insert" writes a pair through the dictionary that the
//!    writer derefs to, and the call "item" writes an item through the
//!    array. They test that the writers deref as in pdf-writer.
//!
//! An enum is the name of its variant, as `CidFontType::Type2` is
//! `{"name": "CIDFontType2"}`, or its number, as `WMode` and `SMaskInData`
//! are. A `Date` is an array of up to eight numbers, for `Date::new` and
//! each method of `Date` in order.
//!
//! "calls.unicode_cmap" builds a `UnicodeCmap`. Its first call is "new_u8"
//! or "new_u16", with the name, the registry, the ordering, the supplement
//! and the writing mode. The calls "pair" and "pair_with_multiple" follow,
//! with a glyph and a text string. The object is then `Chunk::cmap` with the
//! bytes of the map.

use pdf_writer::types::{
    AssociationKind, CidFontType, CjkClass, FontFlags, FontStretch, GlyphId, SMaskInData,
    SystemInfo, UnicodeCmap,
};
use pdf_writer::writers::{
    CidFont, Cmap, Differences, Encoding, FileSpec, FontDescriptor, FontDescriptorOverride,
    FormXObject, Group, ImageXObject, Reference, Type0Font, Type1Font, Type3Font, WMode, Widths,
};
use pdf_writer::{Chunk, Date, Name, Rect, Ref, Str, TextStr};

use super::{filter, hex, reference, write_value, Object, Value};

/// The starts of the objects that this module writes.
const STARTS: &[&str] = &[
    "calls.type1_font",
    "calls.type3_font",
    "calls.type0_font",
    "calls.cid_font",
    "calls.font_descriptor",
    "calls.file_spec",
    "calls.cmap",
    "calls.image_xobject",
    "calls.form_xobject",
    "calls.unicode_cmap",
];

type Calls = [(String, Value)];

/// starts reports whether this module writes the objects of a start.
pub fn starts(start: &str) -> bool {
    STARTS.contains(&start)
}

/// write writes one object whose start is in `STARTS`.
pub fn write(c: &mut Chunk, o: &Object, start: &str, data: Option<Vec<u8>>) -> Result<(), String> {
    let id = reference(o.id)?;
    let f = match &o.filter {
        Some(f) => Some(filter(f)?),
        None => None,
    };
    let calls = &o.dict;
    match (start, data) {
        ("calls.type1_font", None) => type1_font(c.type1_font(id), calls),
        ("calls.type3_font", None) => type3_font(c.type3_font(id), calls),
        ("calls.type0_font", None) => type0_font(c.type0_font(id), calls),
        ("calls.cid_font", None) => cid_font(c.cid_font(id), calls),
        ("calls.font_descriptor", None) => font_descriptor(c.font_descriptor(id), calls),
        ("calls.file_spec", None) => file_spec(c.file_spec(id), calls),
        ("calls.unicode_cmap", None) => {
            let data = unicode_cmap(calls)?;
            c.cmap(id, &data);
            Ok(())
        }
        ("calls.cmap", Some(d)) => {
            let mut w = c.cmap(id, &d);
            if let Some(f) = f {
                w.filter(f);
            }
            cmap(w, calls)
        }
        ("calls.image_xobject", Some(d)) => {
            let mut w = c.image_xobject(id, &d);
            if let Some(f) = f {
                w.filter(f);
            }
            image_xobject(w, calls)
        }
        ("calls.form_xobject", Some(d)) => {
            let mut w = c.form_xobject(id, &d);
            if let Some(f) = f {
                w.filter(f);
            }
            form_xobject(w, calls)
        }
        (s, _) => Err(format!("object {}: the start {s:?} needs data, or no data", o.id)),
    }
}

/// args returns the arguments of a call, which must be n.
fn args<'v>(m: &str, v: &'v Value, n: usize) -> Result<&'v [Value], String> {
    let a = items(m, v)?;
    if a.len() != n {
        return Err(format!("the call {m:?} takes {n} arguments, not {}", a.len()));
    }
    Ok(a)
}

/// items returns the array of a call.
fn items<'v>(m: &str, v: &'v Value) -> Result<&'v [Value], String> {
    match v {
        Value::Array(a) => Ok(a),
        _ => Err(format!("the call {m:?} takes an array")),
    }
}

/// calls_of returns the calls of a dictionary.
fn calls_of<'v>(m: &str, v: &'v Value) -> Result<&'v Calls, String> {
    match v {
        Value::Dict(d) => Ok(d),
        _ => Err(format!("the call {m:?} takes a dictionary of calls")),
    }
}

fn int(v: &Value) -> Result<i32, String> {
    match v {
        Value::Int(i) => Ok(*i),
        _ => Err("an argument is not an int".into()),
    }
}

fn u8_(v: &Value) -> Result<u8, String> {
    u8::try_from(int(v)?).map_err(|e| e.to_string())
}

fn u16_(v: &Value) -> Result<u16, String> {
    u16::try_from(int(v)?).map_err(|e| e.to_string())
}

fn real(v: &Value) -> Result<f32, String> {
    match v {
        Value::Real(f) => Ok(*f),
        _ => Err("an argument is not a real".into()),
    }
}

fn reals(vs: &[Value]) -> Result<Vec<f32>, String> {
    vs.iter().map(real).collect()
}

fn boolean(v: &Value) -> Result<bool, String> {
    match v {
        Value::Bool(b) => Ok(*b),
        _ => Err("an argument is not a bool".into()),
    }
}

fn name(v: &Value) -> Result<Name<'_>, String> {
    match v {
        Value::Name(n) => Ok(Name(n.as_bytes())),
        _ => Err("an argument is not a name".into()),
    }
}

fn bytes(v: &Value) -> Result<Vec<u8>, String> {
    match v {
        Value::Str(s) => hex(s),
        _ => Err("an argument is not a str".into()),
    }
}

fn text(v: &Value) -> Result<&str, String> {
    match v {
        Value::Text(s) => Ok(s),
        _ => Err("an argument is not a text".into()),
    }
}

fn ref_(v: &Value) -> Result<Ref, String> {
    match v {
        Value::Ref(r) => reference(*r),
        _ => Err("an argument is not a ref".into()),
    }
}

fn rect(a: &[Value]) -> Result<Rect, String> {
    Ok(Rect::new(real(&a[0])?, real(&a[1])?, real(&a[2])?, real(&a[3])?))
}

fn matrix(a: &[Value]) -> Result<[f32; 6], String> {
    let m = reals(a)?;
    m.try_into().map_err(|_| "a matrix has six numbers".to_string())
}

fn date(a: &[Value]) -> Result<Date, String> {
    if a.is_empty() || a.len() > 8 {
        return Err("a date has one to eight numbers".into());
    }
    let mut d = Date::new(u16_(&a[0])?);
    for (i, v) in a.iter().enumerate().skip(1) {
        d = match i {
            1 => d.month(u8_(v)?),
            2 => d.day(u8_(v)?),
            3 => d.hour(u8_(v)?),
            4 => d.minute(u8_(v)?),
            5 => d.second(u8_(v)?),
            6 => d.utc_offset_hour(i8::try_from(int(v)?).map_err(|e| e.to_string())?),
            _ => d.utc_offset_minute(u8_(v)?),
        };
    }
    Ok(d)
}

fn system_info(a: &[Value]) -> Result<(Vec<u8>, Vec<u8>, i32), String> {
    Ok((bytes(&a[0])?, bytes(&a[1])?, int(&a[2])?))
}

fn cid_font_type(v: &Value) -> Result<CidFontType, String> {
    Ok(match name(v)?.0 {
        b"CIDFontType0" => CidFontType::Type0,
        b"CIDFontType2" => CidFontType::Type2,
        _ => return Err("the CID font type is not known".into()),
    })
}

fn font_stretch(v: &Value) -> Result<FontStretch, String> {
    Ok(match name(v)?.0 {
        b"UltraCondensed" => FontStretch::UltraCondensed,
        b"ExtraCondensed" => FontStretch::ExtraCondensed,
        b"Condensed" => FontStretch::Condensed,
        b"SemiCondensed" => FontStretch::SemiCondensed,
        b"Normal" => FontStretch::Normal,
        b"SemiExpanded" => FontStretch::SemiExpanded,
        b"Expanded" => FontStretch::Expanded,
        b"ExtraExpanded" => FontStretch::ExtraExpanded,
        b"UltraExpanded" => FontStretch::UltraExpanded,
        _ => return Err("the font stretch is not known".into()),
    })
}

fn cjk_class(v: &Value) -> Result<CjkClass, String> {
    Ok(match name(v)?.0 {
        b"Alphabetic" => CjkClass::Alphabetic,
        b"AlphaNum" => CjkClass::AlphaNum,
        b"Dingbats" => CjkClass::Dingbats,
        b"DingbatsRot" => CjkClass::DingbatsRot,
        b"Generic" => CjkClass::Generic,
        b"GenericRot" => CjkClass::GenericRot,
        b"Hangul" => CjkClass::Hangul,
        b"Hanja" => CjkClass::Hanja,
        b"Hanzi" => CjkClass::Hanzi,
        b"HRoman" => CjkClass::HRoman,
        b"HRomanRot" => CjkClass::HRomanRot,
        b"HKana" => CjkClass::HKana,
        b"HKanaRot" => CjkClass::HKanaRot,
        b"HojoKanji" => CjkClass::HojoKanji,
        b"Kana" => CjkClass::Kana,
        b"Kanji" => CjkClass::Kanji,
        b"Proportional" => CjkClass::Proportional,
        b"ProportionalRot" => CjkClass::ProportionalRot,
        b"Ruby" => CjkClass::Ruby,
        _ => return Err("the CJK class is not known".into()),
    })
}

fn association_kind(v: &Value) -> Result<AssociationKind, String> {
    Ok(match name(v)?.0 {
        b"Source" => AssociationKind::Source,
        b"Data" => AssociationKind::Data,
        b"Alternative" => AssociationKind::Alternative,
        b"Supplement" => AssociationKind::Supplement,
        b"EncryptedPayload" => AssociationKind::EncryptedPayload,
        b"FormData" => AssociationKind::FormData,
        b"Schema" => AssociationKind::Schema,
        b"Unspecified" => AssociationKind::Unspecified,
        _ => return Err("the association kind is not known".into()),
    })
}

fn w_mode(v: &Value) -> Result<WMode, String> {
    Ok(match int(v)? {
        0 => WMode::Horizontal,
        1 => WMode::Vertical,
        _ => return Err("the writing mode is not known".into()),
    })
}

/// unknown is the error of a call that a writer does not have.
fn unknown(writer: &str, m: &str) -> String {
    format!("the writer {writer} has no call {m:?}")
}

/// Writes the call "insert" through the dictionary of a writer.
macro_rules! insert {
    ($w:expr, $m:expr, $v:expr) => {{
        let a = args($m, $v, 2)?;
        write_value($w.insert(name(&a[0])?), &a[1])?;
    }};
}

fn type1_font(mut w: Type1Font<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "name" => _ = w.name(name(&args(m, v, 1)?[0])?),
            "base_font" => _ = w.base_font(name(&args(m, v, 1)?[0])?),
            "first_char" => _ = w.first_char(u8_(&args(m, v, 1)?[0])?),
            "last_char" => _ = w.last_char(u8_(&args(m, v, 1)?[0])?),
            "widths" => _ = w.widths(reals(items(m, v)?)?),
            "font_descriptor" => _ = w.font_descriptor(ref_(&args(m, v, 1)?[0])?),
            "encoding_predefined" => _ = w.encoding_predefined(name(&args(m, v, 1)?[0])?),
            "encoding_custom" => encoding(w.encoding_custom(), calls_of(m, v)?)?,
            "to_unicode" => _ = w.to_unicode(ref_(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Type1Font", m)),
        }
    }
    Ok(())
}

fn type3_font(mut w: Type3Font<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "name" => _ = w.name(name(&args(m, v, 1)?[0])?),
            "bbox" => _ = w.bbox(rect(args(m, v, 4)?)?),
            "matrix" => _ = w.matrix(matrix(items(m, v)?)?),
            "char_procs" => {
                let a = items(m, v)?;
                let mut d = w.char_procs();
                for p in a.chunks(2) {
                    if p.len() != 2 {
                        return Err("char_procs takes pairs".into());
                    }
                    d.pair(name(&p[0])?, ref_(&p[1])?);
                }
            }
            "encoding_predefined" => _ = w.encoding_predefined(name(&args(m, v, 1)?[0])?),
            "encoding_custom" => encoding(w.encoding_custom(), calls_of(m, v)?)?,
            "first_char" => _ = w.first_char(u8_(&args(m, v, 1)?[0])?),
            "last_char" => _ = w.last_char(u8_(&args(m, v, 1)?[0])?),
            "widths" => _ = w.widths(reals(items(m, v)?)?),
            "font_descriptor" => _ = w.font_descriptor(ref_(&args(m, v, 1)?[0])?),
            "to_unicode" => _ = w.to_unicode(ref_(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Type3Font", m)),
        }
    }
    Ok(())
}

fn encoding(mut w: Encoding<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "base_encoding" => _ = w.base_encoding(name(&args(m, v, 1)?[0])?),
            "differences" => differences(w.differences(), calls_of(m, v)?)?,
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Encoding", m)),
        }
    }
    Ok(())
}

fn differences(mut w: Differences<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "consecutive" => {
                let a = items(m, v)?;
                if a.is_empty() {
                    return Err("consecutive takes a start".into());
                }
                let names = a[1..].iter().map(name).collect::<Result<Vec<_>, _>>()?;
                w.consecutive(u8_(&a[0])?, names);
            }
            "item" => write_value(w.push(), &args(m, v, 1)?[0])?,
            _ => return Err(unknown("Differences", m)),
        }
    }
    Ok(())
}

fn type0_font(mut w: Type0Font<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "base_font" => _ = w.base_font(name(&args(m, v, 1)?[0])?),
            "encoding_predefined" => _ = w.encoding_predefined(name(&args(m, v, 1)?[0])?),
            "encoding_cmap" => _ = w.encoding_cmap(ref_(&args(m, v, 1)?[0])?),
            "descendant_font" => _ = w.descendant_font(ref_(&args(m, v, 1)?[0])?),
            "to_unicode" => _ = w.to_unicode(ref_(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Type0Font", m)),
        }
    }
    Ok(())
}

fn cid_font(mut w: CidFont<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "subtype" => _ = w.subtype(cid_font_type(&args(m, v, 1)?[0])?),
            "base_font" => _ = w.base_font(name(&args(m, v, 1)?[0])?),
            "system_info" => {
                let (r, o, s) = system_info(args(m, v, 3)?)?;
                w.system_info(SystemInfo { registry: Str(&r), ordering: Str(&o), supplement: s });
            }
            "font_descriptor" => _ = w.font_descriptor(ref_(&args(m, v, 1)?[0])?),
            "default_width" => _ = w.default_width(real(&args(m, v, 1)?[0])?),
            "widths" => widths(w.widths(), calls_of(m, v)?)?,
            "cid_to_gid_map_predefined" => {
                _ = w.cid_to_gid_map_predefined(name(&args(m, v, 1)?[0])?)
            }
            "cid_to_gid_map_stream" => _ = w.cid_to_gid_map_stream(ref_(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("CidFont", m)),
        }
    }
    Ok(())
}

fn widths(mut w: Widths<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "consecutive" => {
                let a = items(m, v)?;
                if a.is_empty() {
                    return Err("consecutive takes a start".into());
                }
                w.consecutive(u16_(&a[0])?, reals(&a[1..])?);
            }
            "same" => {
                let a = args(m, v, 3)?;
                w.same(u16_(&a[0])?, u16_(&a[1])?, real(&a[2])?);
            }
            "item" => write_value(w.push(), &args(m, v, 1)?[0])?,
            _ => return Err(unknown("Widths", m)),
        }
    }
    Ok(())
}

fn font_descriptor(mut w: FontDescriptor<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "name" => _ = w.name(name(&args(m, v, 1)?[0])?),
            "family" => _ = w.family(Str(&bytes(&args(m, v, 1)?[0])?)),
            "stretch" => _ = w.stretch(font_stretch(&args(m, v, 1)?[0])?),
            "weight" => _ = w.weight(u16_(&args(m, v, 1)?[0])?),
            "flags" => _ = w.flags(FontFlags::from_bits_retain(int(&args(m, v, 1)?[0])? as u32)),
            "bbox" => _ = w.bbox(rect(args(m, v, 4)?)?),
            "italic_angle" => _ = w.italic_angle(real(&args(m, v, 1)?[0])?),
            "ascent" => _ = w.ascent(real(&args(m, v, 1)?[0])?),
            "descent" => _ = w.descent(real(&args(m, v, 1)?[0])?),
            "leading" => _ = w.leading(real(&args(m, v, 1)?[0])?),
            "cap_height" => _ = w.cap_height(real(&args(m, v, 1)?[0])?),
            "x_height" => _ = w.x_height(real(&args(m, v, 1)?[0])?),
            "stem_v" => _ = w.stem_v(real(&args(m, v, 1)?[0])?),
            "stem_h" => _ = w.stem_h(real(&args(m, v, 1)?[0])?),
            "avg_width" => _ = w.avg_width(real(&args(m, v, 1)?[0])?),
            "max_width" => _ = w.max_width(real(&args(m, v, 1)?[0])?),
            "missing_width" => _ = w.missing_width(real(&args(m, v, 1)?[0])?),
            "font_file" => _ = w.font_file(ref_(&args(m, v, 1)?[0])?),
            "font_file2" => _ = w.font_file2(ref_(&args(m, v, 1)?[0])?),
            "font_file3" => _ = w.font_file3(ref_(&args(m, v, 1)?[0])?),
            "char_set" => _ = w.char_set(Str(&bytes(&args(m, v, 1)?[0])?)),
            "style" => {
                let a = args(m, v, 12)?;
                let mut panose = [0u8; 10];
                for (p, x) in panose.iter_mut().zip(&a[2..]) {
                    *p = u8_(x)?;
                }
                w.style(u8_(&a[0])?, u8_(&a[1])?, panose);
            }
            "descriptor_override" => {
                font_descriptor_override(w.descriptor_override(), calls_of(m, v)?)?
            }
            "cid_set" => _ = w.cid_set(ref_(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("FontDescriptor", m)),
        }
    }
    Ok(())
}

fn font_descriptor_override(mut w: FontDescriptorOverride<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "custom_class" => {
                let a = args(m, v, 2)?;
                font_descriptor(w.custom_class(name(&a[0])?), calls_of(m, &a[1])?)?;
            }
            "custom_class_ref" => {
                let a = args(m, v, 2)?;
                w.custom_class_ref(name(&a[0])?, ref_(&a[1])?);
            }
            "cjk_class" => {
                let a = args(m, v, 2)?;
                font_descriptor(w.cjk_class(cjk_class(&a[0])?), calls_of(m, &a[1])?)?;
            }
            "cjk_class_ref" => {
                let a = args(m, v, 2)?;
                w.cjk_class_ref(cjk_class(&a[0])?, ref_(&a[1])?);
            }
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("FontDescriptorOverride", m)),
        }
    }
    Ok(())
}

fn cmap(mut w: Cmap<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "name" => _ = w.name(name(&args(m, v, 1)?[0])?),
            "system_info" => {
                let (r, o, s) = system_info(args(m, v, 3)?)?;
                w.system_info(SystemInfo { registry: Str(&r), ordering: Str(&o), supplement: s });
            }
            "writing_mode" => _ = w.writing_mode(w_mode(&args(m, v, 1)?[0])?),
            "use_cmap_stream" => _ = w.use_cmap_stream(ref_(&args(m, v, 1)?[0])?),
            "use_cmap_predefined" => _ = w.use_cmap_predefined(name(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Cmap", m)),
        }
    }
    Ok(())
}

fn unicode_cmap(calls: &Calls) -> Result<Vec<u8>, String> {
    let Some(((m, v), rest)) = calls.split_first() else {
        return Err("a UnicodeCmap needs the call new_u8 or new_u16".into());
    };
    let a = args(m, v, 5)?;
    let (r, o, s) = system_info(&a[1..4])?;
    let info = SystemInfo { registry: Str(&r), ordering: Str(&o), supplement: s };
    let mode = w_mode(&a[4])?;
    match m.as_str() {
        "new_u8" => cmap_pairs(UnicodeCmap::<u8>::with_writing_mode(name(&a[0])?, info, mode), rest),
        "new_u16" => cmap_pairs(UnicodeCmap::<u16>::with_writing_mode(name(&a[0])?, info, mode), rest),
        _ => Err("a UnicodeCmap needs the call new_u8 or new_u16".into()),
    }
}

fn cmap_pairs<G>(mut cmap: UnicodeCmap<G>, calls: &Calls) -> Result<Vec<u8>, String>
where
    G: GlyphId + TryFrom<i32>,
{
    for (m, v) in calls {
        let a = args(m, v, 2)?;
        let g = G::try_from(int(&a[0])?).map_err(|_| "the glyph is out of range".to_string())?;
        let t = text(&a[1])?;
        match m.as_str() {
            "pair" => {
                let mut chars = t.chars();
                match (chars.next(), chars.next()) {
                    (Some(c), None) => cmap.pair(g, c),
                    _ => return Err("pair takes one character".into()),
                }
            }
            "pair_with_multiple" => cmap.pair_with_multiple(g, t.chars()),
            _ => return Err(unknown("UnicodeCmap", m)),
        }
    }
    Ok(cmap.finish().into_vec())
}

fn image_xobject(mut w: ImageXObject<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "width" => _ = w.width(int(&args(m, v, 1)?[0])?),
            "height" => _ = w.height(int(&args(m, v, 1)?[0])?),
            "color_space_name" => _ = w.color_space_name(name(&args(m, v, 1)?[0])?),
            "bits_per_component" => _ = w.bits_per_component(int(&args(m, v, 1)?[0])?),
            "image_mask" => _ = w.image_mask(boolean(&args(m, v, 1)?[0])?),
            "color_mask" => {
                let c = items(m, v)?.iter().map(int).collect::<Result<Vec<_>, _>>()?;
                w.color_mask(c);
            }
            "stencil_mask" => _ = w.stencil_mask(ref_(&args(m, v, 1)?[0])?),
            "decode" => _ = w.decode(reals(items(m, v)?)?),
            "interpolate" => _ = w.interpolate(boolean(&args(m, v, 1)?[0])?),
            "alternates" => {
                let r = items(m, v)?.iter().map(ref_).collect::<Result<Vec<_>, _>>()?;
                w.alternates(r);
            }
            "s_mask" => _ = w.s_mask(ref_(&args(m, v, 1)?[0])?),
            "s_mask_in_data" => {
                let mode = match int(&args(m, v, 1)?[0])? {
                    0 => SMaskInData::Ignore,
                    1 => SMaskInData::Use,
                    2 => SMaskInData::Preblended,
                    _ => return Err("the mode of SMaskInData is not known".into()),
                };
                w.s_mask_in_data(mode);
            }
            "struct_parent" => _ = w.struct_parent(int(&args(m, v, 1)?[0])?),
            "matte" => _ = w.matte(reals(items(m, v)?)?),
            "metadata" => _ = w.metadata(ref_(&args(m, v, 1)?[0])?),
            "associated_files" => {
                let mut a = w.associated_files();
                for d in items(m, v)? {
                    file_spec(a.push(), calls_of(m, d)?)?;
                }
            }
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("ImageXObject", m)),
        }
    }
    Ok(())
}

fn form_xobject(mut w: FormXObject<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "bbox" => _ = w.bbox(rect(args(m, v, 4)?)?),
            "matrix" => _ = w.matrix(matrix(items(m, v)?)?),
            "group" => group(w.group(), calls_of(m, v)?)?,
            "struct_parent" => _ = w.struct_parent(int(&args(m, v, 1)?[0])?),
            "struct_parents" => _ = w.struct_parents(int(&args(m, v, 1)?[0])?),
            "reference" => reference_(w.reference(), calls_of(m, v)?)?,
            "metadata" => _ = w.metadata(ref_(&args(m, v, 1)?[0])?),
            "last_modified" => _ = w.last_modified(date(items(m, v)?)?),
            "associated_files" => {
                let mut a = w.associated_files();
                for d in items(m, v)? {
                    file_spec(a.push(), calls_of(m, d)?)?;
                }
            }
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("FormXObject", m)),
        }
    }
    Ok(())
}

fn group(mut w: Group<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "transparency" => {
                args(m, v, 0)?;
                w.transparency();
            }
            "isolated" => _ = w.isolated(boolean(&args(m, v, 1)?[0])?),
            "knockout" => _ = w.knockout(boolean(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Group", m)),
        }
    }
    Ok(())
}

fn reference_(mut w: Reference<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "file_spec" => file_spec(w.file_spec(), calls_of(m, v)?)?,
            "page_number" => _ = w.page_number(int(&args(m, v, 1)?[0])?),
            "page_label" => _ = w.page_label(TextStr(text(&args(m, v, 1)?[0])?)),
            "id" => {
                let a = args(m, v, 2)?;
                let (x, y) = (bytes(&a[0])?, bytes(&a[1])?);
                w.id([Str(&x), Str(&y)]);
            }
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("Reference", m)),
        }
    }
    Ok(())
}

fn file_spec(mut w: FileSpec<'_>, calls: &Calls) -> Result<(), String> {
    for (m, v) in calls {
        match m.as_str() {
            "file_system" => _ = w.file_system(name(&args(m, v, 1)?[0])?),
            "path" => _ = w.path(Str(&bytes(&args(m, v, 1)?[0])?)),
            "unic_file" => _ = w.unic_file(TextStr(text(&args(m, v, 1)?[0])?)),
            "volatile" => _ = w.volatile(boolean(&args(m, v, 1)?[0])?),
            "description" => _ = w.description(TextStr(text(&args(m, v, 1)?[0])?)),
            "embedded_file" => _ = w.embedded_file(ref_(&args(m, v, 1)?[0])?),
            "embedded_file_with_unicode" => {
                _ = w.embedded_file_with_unicode(ref_(&args(m, v, 1)?[0])?)
            }
            "association_kind" => _ = w.association_kind(association_kind(&args(m, v, 1)?[0])?),
            "insert" => insert!(w, m, v),
            _ => return Err(unknown("FileSpec", m)),
        }
    }
    Ok(())
}
