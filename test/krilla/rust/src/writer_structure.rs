//! The calls of the typed writers of `structure.rs`, `annotations.rs` and
//! `actions.rs` of pdf-writer, for unit 7 of phase 12.
//!
//! An object with the field "structure" starts the typed writer that its
//! field "start" names, and then makes the calls of "structure" on it, in
//! order:
//!
//! ```json
//! {"id": 3, "start": "page", "structure": [
//!     {"call": "media_box", "args": [0, 0, 595, 842]},
//!     {"call": "resources", "then": [{"call": "pair", "args": ["Font", {"dict": []}]}]},
//!     {"call": "annotations", "args": [4, 5]}]}
//! ```
//!
//! A call names a method of the writer by its Rust name. "args" holds its
//! arguments as JSON values: a number for an integer, a float or a
//! reference, a string for a name, a text string or an enum variant, a hex
//! string for a byte string, a list for a rectangle, a date or a list of
//! items, and null for None. A call that starts another writer, such as
//! "resources", makes the calls of "then" on that writer, and the writer is
//! dropped at the end of them. The call "pair" writes a pair of the value
//! format of `writer.rs` through the dictionary that the writer derefs to,
//! and "item" writes an item into the array that a `Destination` derefs to.
//! The field "finish" of a call is for the Go side only, and this module
//! ignores it.
//!
//! The starts are catalog and document_info, which need a `Pdf`, and pages,
//! page, outline, outline_item, destination, destinations, annotation,
//! action, page_label, page_labels (a `NumberTree<Ref>`), names,
//! viewer_preferences and metadata, a stream with the data of the object.

use pdf_writer::types::{
    ActionType, AnnotationFlags, AnnotationType, Direction, HighlightEffect, NumberingStyle,
    OutlineItemFlags, PageLayout, PageMode, TabOrder, TrappingStatus,
};
use pdf_writer::writers::{
    Action, Annotation, Catalog, Destination, DocumentInfo, NameTree, Names, NumberTree, Outline,
    OutlineItem, Page, PageLabel, Pages, ViewerPreferences,
};
use pdf_writer::{Chunk, Date, Dict, Name, Pdf, Rect, Ref, Str, TextStr, TypedDict};
use serde::Deserialize;
use serde_json::Value as Json;

use super::{filter, hex, reference, write_value, Object, Value};

/// A call of a method of a typed writer.
#[derive(Deserialize)]
pub struct Call {
    call: String,
    #[serde(default)]
    args: Vec<Json>,
    #[serde(default)]
    then: Vec<Call>,
}

type Res = Result<(), String>;

/// write_pdf writes an object whose typed writer can need a `Pdf`.
pub fn write_pdf(pdf: &mut Pdf, o: &Object, calls: &[Call]) -> Res {
    let id = reference(o.id)?;
    match o.start.as_deref() {
        Some("catalog") => catalog(&mut pdf.catalog(id), calls),
        Some("document_info") => document_info(&mut pdf.document_info(id), calls),
        _ => write(pdf, o, calls),
    }
}

/// write writes an object into a chunk.
pub fn write(c: &mut Chunk, o: &Object, calls: &[Call]) -> Res {
    let id = reference(o.id)?;
    match o.start.as_deref() {
        Some("pages") => pages(&mut c.pages(id), calls),
        Some("page") => page(&mut c.page(id), calls),
        Some("outline") => outline(&mut c.outline(id), calls),
        Some("outline_item") => outline_item(&mut c.outline_item(id), calls),
        Some("destination") => destination(c.destination(id), calls),
        Some("destinations") => destinations(&mut c.destinations(id), calls),
        Some("annotation") => annotation(&mut c.annotation(id), calls),
        Some("action") => action(&mut c.indirect(id).start::<Action>(), calls),
        Some("page_label") => page_label(&mut c.indirect(id).start::<PageLabel>(), calls),
        Some("page_labels") => number_tree(&mut c.indirect(id).start::<NumberTree<Ref>>(), calls),
        Some("names") => names(&mut c.indirect(id).start::<Names>(), calls),
        Some("viewer_preferences") => {
            viewer_preferences(&mut c.indirect(id).start::<ViewerPreferences>(), calls)
        }
        Some("metadata") => {
            let data = match &o.data {
                Some(d) => base64::Engine::decode(&base64::engine::general_purpose::STANDARD, d)
                    .map_err(|e| format!("decoding the data of object {}: {e}", o.id))?,
                None => return Err(format!("object {}: metadata needs data", o.id)),
            };
            let mut m = c.metadata(id, &data);
            for call in calls {
                match call.call.as_str() {
                    "filter" => {
                        m.filter(filter(&string(call, 0)?)?);
                    }
                    _ => dict(&mut m, call)?,
                }
            }
            Ok(())
        }
        Some(s) => Err(format!("object {}: the start {s:?} has no calls", o.id)),
        None => Err(format!("object {}: calls need a start", o.id)),
    }
}

/// dict makes the call "pair" on a dictionary.
fn dict(d: &mut Dict, c: &Call) -> Res {
    match c.call.as_str() {
        "pair" => {
            let key = string(c, 0)?;
            write_value(d.insert(Name(key.as_bytes())), &value(c, 1)?)
        }
        name => Err(format!("the call {name:?} is not known")),
    }
}

fn dict_calls(d: &mut Dict, calls: &[Call]) -> Res {
    for c in calls {
        dict(d, c)?;
    }
    Ok(())
}

fn catalog(w: &mut Catalog, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "pages" => {
                w.pages(refer(c, 0)?);
            }
            "page_layout" => {
                w.page_layout(page_layout(&string(c, 0)?)?);
            }
            "page_labels" => number_tree(&mut w.page_labels(), &c.then)?,
            "page_mode" => {
                w.page_mode(page_mode(&string(c, 0)?)?);
            }
            "viewer_preferences" => viewer_preferences(&mut w.viewer_preferences(), &c.then)?,
            "names" => names(&mut w.names(), &c.then)?,
            "destinations" => {
                w.destinations(refer(c, 0)?);
            }
            "outlines" => {
                w.outlines(refer(c, 0)?);
            }
            "lang" => {
                w.lang(TextStr(&string(c, 0)?));
            }
            "version" => {
                w.version(int(c, 0)? as u8, int(c, 1)? as u8);
            }
            "metadata" => {
                w.metadata(refer(c, 0)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn number_tree(w: &mut NumberTree<Ref>, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "nums" => {
                let mut nums = w.nums();
                for e in &c.then {
                    match e.call.as_str() {
                        "insert" => {
                            nums.insert(int(e, 0)? as i32, refer(e, 1)?);
                        }
                        name => return Err(format!("the call {name:?} is not known")),
                    }
                }
            }
            "kids" => {
                let mut kids = w.kids();
                for e in &c.then {
                    match e.call.as_str() {
                        "item" => {
                            kids.item(refer(e, 0)?);
                        }
                        name => return Err(format!("the call {name:?} is not known")),
                    }
                }
            }
            "limits" => {
                w.limits(int(c, 0)? as i32, int(c, 1)? as i32);
            }
            name => return Err(format!("the call {name:?} is not known")),
        }
    }
    Ok(())
}

fn name_tree(w: &mut NameTree<Ref>, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "names" => {
                let mut names = w.names();
                for e in &c.then {
                    match e.call.as_str() {
                        "insert" => {
                            names.insert(Str(&hex(&string(e, 0)?)?), refer(e, 1)?);
                        }
                        name => return Err(format!("the call {name:?} is not known")),
                    }
                }
            }
            "kids" => {
                let mut kids = w.kids();
                for e in &c.then {
                    match e.call.as_str() {
                        "item" => {
                            kids.item(refer(e, 0)?);
                        }
                        name => return Err(format!("the call {name:?} is not known")),
                    }
                }
            }
            "limits" => {
                w.limits(Name(string(c, 0)?.as_bytes()), Name(string(c, 1)?.as_bytes()));
            }
            name => return Err(format!("the call {name:?} is not known")),
        }
    }
    Ok(())
}

fn viewer_preferences(w: &mut ViewerPreferences, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "hide_toolbar" => {
                w.hide_toolbar(boolean(c, 0)?);
            }
            "hide_menubar" => {
                w.hide_menubar(boolean(c, 0)?);
            }
            "fit_window" => {
                w.fit_window(boolean(c, 0)?);
            }
            "center_window" => {
                w.center_window(boolean(c, 0)?);
            }
            "display_doc_title" => {
                w.display_doc_title(boolean(c, 0)?);
            }
            "non_full_screen_page_mode" => {
                w.non_full_screen_page_mode(page_mode(&string(c, 0)?)?);
            }
            "direction" => {
                w.direction(direction(&string(c, 0)?)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn names(w: &mut Names, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "destinations" => name_tree(&mut w.destinations(), &c.then)?,
            "appearances" => name_tree(&mut w.appearances(), &c.then)?,
            "javascript" => name_tree(&mut w.javascript(), &c.then)?,
            "pages" => name_tree(&mut w.pages(), &c.then)?,
            "templates" => name_tree(&mut w.templates(), &c.then)?,
            "capture_ids" => name_tree(&mut w.capture_ids(), &c.then)?,
            "capture_urls" => name_tree(&mut w.capture_urls(), &c.then)?,
            "embedded_files" => name_tree(&mut w.embedded_files(), &c.then)?,
            "alternate_presentations" => name_tree(&mut w.alternate_presentations(), &c.then)?,
            "renditions" => name_tree(&mut w.renditions(), &c.then)?,
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn document_info(w: &mut DocumentInfo, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "title" => {
                w.title(TextStr(&string(c, 0)?));
            }
            "author" => {
                w.author(TextStr(&string(c, 0)?));
            }
            "subject" => {
                w.subject(TextStr(&string(c, 0)?));
            }
            "keywords" => {
                w.keywords(TextStr(&string(c, 0)?));
            }
            "creator" => {
                w.creator(TextStr(&string(c, 0)?));
            }
            "producer" => {
                w.producer(TextStr(&string(c, 0)?));
            }
            "creation_date" => {
                w.creation_date(date(c, 0)?);
            }
            "modified_date" => {
                w.modified_date(date(c, 0)?);
            }
            "trapped" => {
                w.trapped(trapping_status(&string(c, 0)?)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn pages(w: &mut Pages, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "parent" => {
                w.parent(refer(c, 0)?);
            }
            "kids" => {
                w.kids(refs(c, 0)?);
            }
            "count" => {
                w.count(int(c, 0)? as i32);
            }
            "media_box" => {
                w.media_box(rect(c, 0)?);
            }
            "resources" => dict_calls(&mut w.resources(), &c.then)?,
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn page(w: &mut Page, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "parent" => {
                w.parent(refer(c, 0)?);
            }
            "last_modified" => {
                w.last_modified(date(c, 0)?);
            }
            "media_box" => {
                w.media_box(rect(c, 0)?);
            }
            "crop_box" => {
                w.crop_box(rect(c, 0)?);
            }
            "bleed_box" => {
                w.bleed_box(rect(c, 0)?);
            }
            "trim_box" => {
                w.trim_box(rect(c, 0)?);
            }
            "art_box" => {
                w.art_box(rect(c, 0)?);
            }
            "resources" => dict_calls(&mut w.resources(), &c.then)?,
            "contents" => {
                w.contents(refer(c, 0)?);
            }
            "contents_array" => {
                w.contents_array(refs(c, 0)?);
            }
            "rotate" => {
                w.rotate(int(c, 0)? as i32);
            }
            "thumbnail" => {
                w.thumbnail(refer(c, 0)?);
            }
            "duration" => {
                w.duration(float(c, 0)?);
            }
            "annotations" => {
                w.annotations(refs(c, 0)?);
            }
            "struct_parents" => {
                w.struct_parents(int(c, 0)? as i32);
            }
            "tab_order" => {
                w.tab_order(tab_order(&string(c, 0)?)?);
            }
            "user_unit" => {
                w.user_unit(float(c, 0)?);
            }
            "metadata" => {
                w.metadata(refer(c, 0)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn outline(w: &mut Outline, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "first" => {
                w.first(refer(c, 0)?);
            }
            "last" => {
                w.last(refer(c, 0)?);
            }
            "count" => {
                w.count(int(c, 0)? as i32);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn outline_item(w: &mut OutlineItem, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "title" => {
                w.title(TextStr(&string(c, 0)?));
            }
            "parent" => {
                w.parent(refer(c, 0)?);
            }
            "prev" => {
                w.prev(refer(c, 0)?);
            }
            "next" => {
                w.next(refer(c, 0)?);
            }
            "first" => {
                w.first(refer(c, 0)?);
            }
            "last" => {
                w.last(refer(c, 0)?);
            }
            "count" => {
                w.count(int(c, 0)? as i32);
            }
            "dest" => destination(w.dest(), &c.then)?,
            "dest_name" => {
                w.dest_name(Name(string(c, 0)?.as_bytes()));
            }
            "color_rgb" => {
                w.color_rgb(float(c, 0)?, float(c, 1)?, float(c, 2)?);
            }
            "flags" => {
                w.flags(OutlineItemFlags::from_bits_retain(int(c, 0)? as u32));
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

/// destination makes the calls on a destination, which its methods take by
/// value. A call after one that ends the destination is an error.
fn destination(w: Destination, calls: &[Call]) -> Res {
    let mut w = Some(w);
    for c in calls {
        let Some(mut d) = w.take() else {
            return Err("the destination is finished".into());
        };
        match c.call.as_str() {
            "page" => w = Some(d.page(refer(c, 0)?)),
            "xyz" => {
                let zoom = match c.args.get(2) {
                    None | Some(Json::Null) => None,
                    Some(_) => Some(float(c, 2)?),
                };
                d.xyz(float(c, 0)?, float(c, 1)?, zoom);
            }
            "fit" => d.fit(),
            "fit_horizontal" => d.fit_horizontal(float(c, 0)?),
            "fit_vertical" => d.fit_vertical(float(c, 0)?),
            "fit_rect" => d.fit_rect(rect(c, 0)?),
            "fit_bounding_box" => d.fit_bounding_box(),
            "fit_bounding_box_horizontal" => d.fit_bounding_box_horizontal(float(c, 0)?),
            "fit_bounding_box_vertical" => d.fit_bounding_box_vertical(float(c, 0)?),
            "item" => {
                write_value(d.push(), &value(c, 0)?)?;
                w = Some(d);
            }
            name => return Err(format!("the call {name:?} is not known")),
        }
    }
    Ok(())
}

fn destinations<'a>(w: &mut TypedDict<'a, Destination<'a>>, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "insert" => destination(w.insert(Name(string(c, 0)?.as_bytes())), &c.then)?,
            name => return Err(format!("the call {name:?} is not known")),
        }
    }
    Ok(())
}

fn page_label(w: &mut PageLabel, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "style" => {
                w.style(numbering_style(&string(c, 0)?)?);
            }
            "prefix" => {
                w.prefix(TextStr(&string(c, 0)?));
            }
            "offset" => {
                w.offset(int(c, 0)? as i32);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn annotation(w: &mut Annotation, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "subtype" => {
                w.subtype(annotation_type(&string(c, 0)?)?);
            }
            "rect" => {
                w.rect(rect(c, 0)?);
            }
            "contents" => {
                w.contents(TextStr(&string(c, 0)?));
            }
            "page" => {
                w.page(refer(c, 0)?);
            }
            "name" => {
                w.name(TextStr(&string(c, 0)?));
            }
            "modified" => {
                w.modified(date(c, 0)?);
            }
            "flags" => {
                w.flags(AnnotationFlags::from_bits_retain(int(c, 0)? as u32));
            }
            "appearance_state" => {
                w.appearance_state(Name(string(c, 0)?.as_bytes()));
            }
            "border" => {
                let dash = match c.args.get(3) {
                    None | Some(Json::Null) => None,
                    Some(_) => Some(floats(c, 3)?),
                };
                w.border(float(c, 0)?, float(c, 1)?, float(c, 2)?, dash.as_deref());
            }
            "color_transparent" => {
                w.color_transparent();
            }
            "color_gray" => {
                w.color_gray(float(c, 0)?);
            }
            "color_rgb" => {
                w.color_rgb(float(c, 0)?, float(c, 1)?, float(c, 2)?);
            }
            "color_cmyk" => {
                w.color_cmyk(float(c, 0)?, float(c, 1)?, float(c, 2)?, float(c, 3)?);
            }
            "struct_parent" => {
                w.struct_parent(int(c, 0)? as i32);
            }
            "action" => action(&mut w.action(), &c.then)?,
            "highlight" => {
                w.highlight(highlight_effect(&string(c, 0)?)?);
            }
            "author" => {
                w.author(TextStr(&string(c, 0)?));
            }
            "subject" => {
                w.subject(TextStr(&string(c, 0)?));
            }
            "quad_points" => {
                w.quad_points(floats(c, 0)?);
            }
            "line_to" => {
                w.line_to(float(c, 0)?, float(c, 1)?, float(c, 2)?, float(c, 3)?);
            }
            "parent" => {
                w.parent(refer(c, 0)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

fn action(w: &mut Action, calls: &[Call]) -> Res {
    for c in calls {
        match c.call.as_str() {
            "action_type" => {
                w.action_type(action_type(&string(c, 0)?)?);
            }
            "destination" => destination(w.destination(), &c.then)?,
            "destination_named" => {
                w.destination_named(Name(string(c, 0)?.as_bytes()));
            }
            "new_window" => {
                w.new_window(boolean(c, 0)?);
            }
            "uri" => {
                w.uri(Str(&hex(&string(c, 0)?)?));
            }
            "is_map" => {
                w.is_map(boolean(c, 0)?);
            }
            "js_string" => {
                w.js_string(TextStr(&string(c, 0)?));
            }
            "js_stream" => {
                w.js_stream(refer(c, 0)?);
            }
            "annotation" => {
                w.annotation(refer(c, 0)?);
            }
            _ => dict(w, c)?,
        }
    }
    Ok(())
}

// The arguments.

fn arg<'a>(c: &'a Call, i: usize) -> Result<&'a Json, String> {
    c.args.get(i).ok_or_else(|| format!("the call {:?} has no argument {i}", c.call))
}

fn bad(c: &Call, i: usize, what: &str) -> String {
    format!("the argument {i} of the call {:?} is not {what}", c.call)
}

fn string(c: &Call, i: usize) -> Result<String, String> {
    arg(c, i)?.as_str().map(str::to_owned).ok_or_else(|| bad(c, i, "a string"))
}

fn boolean(c: &Call, i: usize) -> Result<bool, String> {
    arg(c, i)?.as_bool().ok_or_else(|| bad(c, i, "a bool"))
}

fn int(c: &Call, i: usize) -> Result<i64, String> {
    arg(c, i)?.as_i64().ok_or_else(|| bad(c, i, "an integer"))
}

fn json_float(c: &Call, i: usize, v: &Json) -> Result<f32, String> {
    v.as_f64().map(|f| f as f32).ok_or_else(|| bad(c, i, "a number"))
}

fn float(c: &Call, i: usize) -> Result<f32, String> {
    json_float(c, i, arg(c, i)?)
}

fn floats(c: &Call, i: usize) -> Result<Vec<f32>, String> {
    let list = arg(c, i)?.as_array().ok_or_else(|| bad(c, i, "a list"))?;
    list.iter().map(|v| json_float(c, i, v)).collect()
}

fn refer(c: &Call, i: usize) -> Result<Ref, String> {
    let id = int(c, i)?;
    reference(i32::try_from(id).map_err(|_| bad(c, i, "an object number"))?)
}

fn refs(c: &Call, i: usize) -> Result<Vec<Ref>, String> {
    let list = arg(c, i)?.as_array().ok_or_else(|| bad(c, i, "a list"))?;
    list.iter()
        .map(|v| {
            let id = v.as_i64().ok_or_else(|| bad(c, i, "a list of object numbers"))?;
            reference(i32::try_from(id).map_err(|_| bad(c, i, "an object number"))?)
        })
        .collect()
}

fn rect(c: &Call, i: usize) -> Result<Rect, String> {
    let f = floats(c, i)?;
    if f.len() != 4 {
        return Err(bad(c, i, "a rectangle"));
    }
    Ok(Rect::new(f[0], f[1], f[2], f[3]))
}

/// date reads a date from a list of its fields: the year, the month, the
/// day, the hour, the minute, the second, and the offset from UTC in hours
/// and minutes. The list can end after any field.
fn date(c: &Call, i: usize) -> Result<Date, String> {
    let list = arg(c, i)?.as_array().ok_or_else(|| bad(c, i, "a list"))?;
    let mut n = Vec::new();
    for v in list {
        n.push(v.as_i64().ok_or_else(|| bad(c, i, "a date"))?);
    }
    let Some(&year) = n.first() else {
        return Err(bad(c, i, "a date"));
    };
    let mut d = Date::new(year as u16);
    if let Some(&v) = n.get(1) {
        d = d.month(v as u8);
    }
    if let Some(&v) = n.get(2) {
        d = d.day(v as u8);
    }
    if let Some(&v) = n.get(3) {
        d = d.hour(v as u8);
    }
    if let Some(&v) = n.get(4) {
        d = d.minute(v as u8);
    }
    if let Some(&v) = n.get(5) {
        d = d.second(v as u8);
    }
    if let Some(&v) = n.get(6) {
        d = d.utc_offset_hour(v as i8);
    }
    if let Some(&v) = n.get(7) {
        d = d.utc_offset_minute(v as u8);
    }
    Ok(d)
}

fn value(c: &Call, i: usize) -> Result<Value, String> {
    serde_json::from_value(arg(c, i)?.clone()).map_err(|e| format!("{}: {e}", bad(c, i, "a value")))
}

// The enums, by the names of their variants.

fn unknown(kind: &str, s: &str) -> String {
    format!("the {kind} {s:?} is not known")
}

fn page_layout(s: &str) -> Result<PageLayout, String> {
    Ok(match s {
        "SinglePage" => PageLayout::SinglePage,
        "OneColumn" => PageLayout::OneColumn,
        "TwoColumnLeft" => PageLayout::TwoColumnLeft,
        "TwoColumnRight" => PageLayout::TwoColumnRight,
        "TwoPageLeft" => PageLayout::TwoPageLeft,
        "TwoPageRight" => PageLayout::TwoPageRight,
        _ => return Err(unknown("page layout", s)),
    })
}

fn page_mode(s: &str) -> Result<PageMode, String> {
    Ok(match s {
        "UseNone" => PageMode::UseNone,
        "UseOutlines" => PageMode::UseOutlines,
        "UseThumbs" => PageMode::UseThumbs,
        "FullScreen" => PageMode::FullScreen,
        "UseOC" => PageMode::UseOC,
        "UseAttachments" => PageMode::UseAttachments,
        _ => return Err(unknown("page mode", s)),
    })
}

fn direction(s: &str) -> Result<Direction, String> {
    Ok(match s {
        "L2R" => Direction::L2R,
        "R2L" => Direction::R2L,
        _ => return Err(unknown("direction", s)),
    })
}

fn numbering_style(s: &str) -> Result<NumberingStyle, String> {
    Ok(match s {
        "Arabic" => NumberingStyle::Arabic,
        "LowerRoman" => NumberingStyle::LowerRoman,
        "UpperRoman" => NumberingStyle::UpperRoman,
        "LowerAlpha" => NumberingStyle::LowerAlpha,
        "UpperAlpha" => NumberingStyle::UpperAlpha,
        _ => return Err(unknown("numbering style", s)),
    })
}

fn trapping_status(s: &str) -> Result<TrappingStatus, String> {
    Ok(match s {
        "Trapped" => TrappingStatus::Trapped,
        "NotTrapped" => TrappingStatus::NotTrapped,
        "Unknown" => TrappingStatus::Unknown,
        _ => return Err(unknown("trapping status", s)),
    })
}

fn tab_order(s: &str) -> Result<TabOrder, String> {
    Ok(match s {
        "RowOrder" => TabOrder::RowOrder,
        "ColumnOrder" => TabOrder::ColumnOrder,
        "StructureOrder" => TabOrder::StructureOrder,
        _ => return Err(unknown("tab order", s)),
    })
}

fn annotation_type(s: &str) -> Result<AnnotationType, String> {
    Ok(match s {
        "Text" => AnnotationType::Text,
        "Link" => AnnotationType::Link,
        "Line" => AnnotationType::Line,
        "Square" => AnnotationType::Square,
        "Circle" => AnnotationType::Circle,
        "Highlight" => AnnotationType::Highlight,
        "Underline" => AnnotationType::Underline,
        "Squiggly" => AnnotationType::Squiggly,
        "StrikeOut" => AnnotationType::StrikeOut,
        "FileAttachment" => AnnotationType::FileAttachment,
        "Widget" => AnnotationType::Widget,
        "Screen" => AnnotationType::Screen,
        _ => return Err(unknown("annotation type", s)),
    })
}

fn highlight_effect(s: &str) -> Result<HighlightEffect, String> {
    Ok(match s {
        "None" => HighlightEffect::None,
        "Invert" => HighlightEffect::Invert,
        "Outline" => HighlightEffect::Outline,
        "Push" => HighlightEffect::Push,
        _ => return Err(unknown("highlight effect", s)),
    })
}

fn action_type(s: &str) -> Result<ActionType, String> {
    Ok(match s {
        "GoTo" => ActionType::GoTo,
        "RemoteGoTo" => ActionType::RemoteGoTo,
        "Launch" => ActionType::Launch,
        "Uri" => ActionType::Uri,
        "SubmitForm" => ActionType::SubmitForm,
        "ResetForm" => ActionType::ResetForm,
        "ImportData" => ActionType::ImportData,
        "JavaScript" => ActionType::JavaScript,
        "Rendition" => ActionType::Rendition,
        _ => return Err(unknown("action type", s)),
    })
}
