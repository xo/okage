//! The bytes of a list of calls of pdf-writer.
//!
//! A question writes indirect objects into a `Pdf` or into a `Chunk`. An
//! object is a value, a stream, or the start of a typed writer of pdf-writer,
//! such as `Chunk::pages`, followed by the pairs of its dictionary. A content
//! stream is a list of operations, each written with `Content::op` and its
//! operands. These calls reach the code of `buf.rs`, `object.rs`, `chunk.rs`,
//! `lib.rs` and `content.rs`, and the fixed keys that each typed writer
//! writes when it starts. Phase 12 adds the typed calls that its units need.
//!
//! A question can also renumber its objects, as `Chunk::renumber` does, and
//! give the filters of a cross-reference stream, as
//! `Pdf::finish_with_xref_stream_and_filter` takes them.

use std::collections::HashMap;

use base64::Engine;
use pdf_writer::{
    Chunk, Content, Filter, Name, Null, Obj, Pdf, Ref, Settings, Str, TextStr, XRefFilter,
};
use serde::Deserialize;

/// A question for pdf-writer.
#[derive(Deserialize)]
pub struct Question {
    /// The setting `pretty` of pdf-writer. Its default is true, as in
    /// pdf-writer.
    #[serde(default = "yes")]
    pretty: bool,
    /// Write a `Chunk` and answer its bytes, without the header and the
    /// trailer of a file.
    #[serde(default)]
    chunk: bool,
    /// The version of PDF, as `[1, 7]`.
    #[serde(default)]
    version: Option<(u8, u8)>,
    /// The four bytes of the binary marker of the header.
    #[serde(default)]
    binary_marker: Option<[u8; 4]>,
    /// The two strings of the file identifier, in hex.
    #[serde(default)]
    file_id: Option<(String, String)>,
    /// The objects, in the order of the calls.
    #[serde(default)]
    objects: Vec<Object>,
    /// The number of the cross-reference stream. Without it, the file ends
    /// with a cross-reference table.
    #[serde(default)]
    xref_stream: Option<i32>,
    /// The filters of the cross-reference stream: one name for
    /// `XRefFilter::Single`, or a list of names for `XRefFilter::Multiple`.
    /// The data of the stream stays as it is.
    #[serde(default)]
    xref_filter: Option<XRefFilterSpec>,
    /// The pairs [old, new] of a renumbering. When it is there, the objects
    /// go into a chunk, and `Chunk::renumber` renumbers the chunk with the
    /// map. A number that is not in the map stays as it is. Then the answer
    /// is the renumbered chunk, or a file that `Pdf::extend` adds it to. The
    /// typed writers catalog and document_info are then not allowed.
    #[serde(default)]
    renumber: Option<Vec<(i32, i32)>>,
}

/// The filters of a cross-reference stream.
#[derive(Deserialize)]
#[serde(untagged)]
enum XRefFilterSpec {
    Single(String),
    Multiple(Vec<String>),
}

fn yes() -> bool {
    true
}

/// An indirect object.
#[derive(Deserialize)]
struct Object {
    /// The object number.
    id: i32,
    /// The value of an object that is not a stream and has no typed writer.
    #[serde(default)]
    value: Option<Value>,
    /// The typed writer that starts the object, such as "catalog", "pages"
    /// or "form_xobject".
    #[serde(default)]
    start: Option<String>,
    /// The pairs of the dictionary, written after the typed writer starts.
    #[serde(default)]
    dict: Vec<(String, Value)>,
    /// The data of a stream, in base64.
    #[serde(default)]
    data: Option<String>,
    /// The content stream that is the data of a stream.
    #[serde(default)]
    content: Option<Vec<Operation>>,
    /// The filter of a stream, such as "FlateDecode". The data is written as
    /// it is given.
    #[serde(default)]
    filter: Option<String>,
}

/// An operation of a content stream.
#[derive(Deserialize)]
struct Operation {
    op: String,
    #[serde(default)]
    operands: Vec<Value>,
}

/// A value of PDF. A unit variant is the JSON string "null", and every other
/// one is an object with one key, such as {"int": 3} or {"name": "Type"}.
#[derive(Deserialize)]
#[serde(rename_all = "snake_case")]
enum Value {
    Null,
    Bool(bool),
    Int(i32),
    Real(f32),
    Name(String),
    /// A byte string, in hex.
    Str(String),
    /// A text string, in UTF-8.
    Text(String),
    Ref(i32),
    Array(Vec<Value>),
    Dict(Vec<(String, Value)>),
}

/// answer makes the calls and returns the bytes.
pub fn answer(q: &Question) -> Result<Vec<u8>, String> {
    let settings = Settings { pretty: q.pretty };
    let renumbered = match &q.renumber {
        Some(pairs) => {
            let mut c = Chunk::with_settings(settings);
            for o in &q.objects {
                write_object(&mut c, o, settings)?;
            }
            let mut map = HashMap::new();
            for &(old, new) in pairs {
                map.insert(reference(old)?, reference(new)?);
            }
            Some(c.renumber(|old| map.get(&old).copied().unwrap_or(old)))
        }
        None => None,
    };
    if let (true, Some(r)) = (q.chunk, &renumbered) {
        return Ok(r.as_bytes().to_vec());
    }
    if q.chunk {
        let mut c = Chunk::with_settings(settings);
        for o in &q.objects {
            write_object(&mut c, o, settings)?;
        }
        return Ok(c.as_bytes().to_vec());
    }
    let mut pdf = Pdf::with_settings(settings);
    if let Some((major, minor)) = q.version {
        pdf.set_version(major, minor);
    }
    if let Some(marker) = &q.binary_marker {
        pdf.set_binary_marker(marker);
    }
    if let Some((a, b)) = &q.file_id {
        pdf.set_file_id((hex(a)?, hex(b)?));
    }
    if let Some(r) = &renumbered {
        pdf.extend(r);
    }
    for o in q.objects.iter().filter(|_| renumbered.is_none()) {
        let id = reference(o.id)?;
        match o.start.as_deref() {
            Some("catalog") => {
                let mut w = pdf.catalog(id);
                for (k, v) in &o.dict {
                    write_value(w.insert(Name(k.as_bytes())), v)?;
                }
            }
            Some("document_info") => {
                let mut w = pdf.document_info(id);
                for (k, v) in &o.dict {
                    write_value(w.insert(Name(k.as_bytes())), v)?;
                }
            }
            _ => write_object(&mut pdf, o, settings)?,
        }
    }
    let xref_filter = match &q.xref_filter {
        Some(XRefFilterSpec::Single(f)) => Some(XRefFilter::Single(filter(f)?)),
        Some(XRefFilterSpec::Multiple(fs)) => {
            Some(XRefFilter::Multiple(fs.iter().map(|f| filter(f)).collect::<Result<_, _>>()?))
        }
        None => None,
    };
    Ok(match (q.xref_stream, xref_filter) {
        (Some(id), Some(f)) => {
            pdf.finish_with_xref_stream_and_filter(reference(id)?, |data| (data.to_vec(), f))
        }
        (Some(id), None) => pdf.finish_with_xref_stream(reference(id)?),
        (None, Some(_)) => return Err("xref_filter needs xref_stream".into()),
        (None, None) => pdf.finish(),
    })
}

/// write_object writes one indirect object into a chunk.
fn write_object(c: &mut Chunk, o: &Object, settings: Settings) -> Result<(), String> {
    let id = reference(o.id)?;
    let data = match (&o.data, &o.content) {
        (Some(d), _) => Some(
            base64::engine::general_purpose::STANDARD
                .decode(d)
                .map_err(|e| format!("decoding the data of object {}: {e}", o.id))?,
        ),
        (None, Some(ops)) => Some(content(ops, settings)?),
        (None, None) => None,
    };
    let filter = match &o.filter {
        Some(f) => Some(filter(f)?),
        None => None,
    };

    macro_rules! pairs {
        ($w:expr) => {{
            let mut w = $w;
            for (k, v) in &o.dict {
                write_value(w.insert(Name(k.as_bytes())), v)?;
            }
        }};
    }
    macro_rules! stream {
        ($w:expr) => {{
            let mut w = $w;
            if let Some(f) = filter {
                w.filter(f);
            }
            for (k, v) in &o.dict {
                write_value(w.insert(Name(k.as_bytes())), v)?;
            }
        }};
    }

    match (o.start.as_deref(), data) {
        (None, None) => match &o.value {
            Some(v) => write_value(c.indirect(id), v)?,
            None => pairs!(c.indirect(id).dict()),
        },
        (None, Some(d)) | (Some("stream"), Some(d)) => stream!(c.stream(id, &d)),
        (Some("form_xobject"), Some(d)) => stream!(c.form_xobject(id, &d)),
        (Some("image_xobject"), Some(d)) => stream!(c.image_xobject(id, &d)),
        (Some("cmap"), Some(d)) => stream!(c.cmap(id, &d)),
        (Some("icc_profile"), Some(d)) => stream!(c.icc_profile(id, &d)),
        (Some("tiling_pattern"), Some(d)) => stream!(c.tiling_pattern(id, &d)),
        (Some("stream_shading"), Some(d)) => stream!(c.stream_shading(id, &d)),
        (Some("post_script_function"), Some(d)) => stream!(c.post_script_function(id, &d)),
        (Some("sampled_function"), Some(d)) => stream!(c.sampled_function(id, &d)),
        (Some("metadata"), Some(d)) => stream!(c.metadata(id, &d)),
        (Some("embedded_file"), Some(d)) => stream!(c.embedded_file(id, &d)),
        (Some("pages"), None) => pairs!(c.pages(id)),
        (Some("page"), None) => pairs!(c.page(id)),
        (Some("outline"), None) => pairs!(c.outline(id)),
        (Some("outline_item"), None) => pairs!(c.outline_item(id)),
        (Some("ext_graphics"), None) => pairs!(c.ext_graphics(id)),
        (Some("type1_font"), None) => pairs!(c.type1_font(id)),
        (Some("type3_font"), None) => pairs!(c.type3_font(id)),
        (Some("type0_font"), None) => pairs!(c.type0_font(id)),
        (Some("cid_font"), None) => pairs!(c.cid_font(id)),
        (Some("font_descriptor"), None) => pairs!(c.font_descriptor(id)),
        (Some("function_shading"), None) => pairs!(c.function_shading(id)),
        (Some("shading_pattern"), None) => pairs!(c.shading_pattern(id)),
        (Some("exponential_function"), None) => pairs!(c.exponential_function(id)),
        (Some("stitching_function"), None) => pairs!(c.stitching_function(id)),
        (Some("annotation"), None) => pairs!(c.annotation(id)),
        (Some(s), _) => return Err(format!("object {}: the start {s:?} is not known", o.id)),
    }
    Ok(())
}

/// content writes the operations of a content stream.
fn content(ops: &[Operation], settings: Settings) -> Result<Vec<u8>, String> {
    let mut c = Content::with_settings(settings);
    for op in ops {
        let mut w = c.op(&op.op);
        for v in &op.operands {
            write_value(w.obj(), v)?;
        }
    }
    Ok(c.finish().into_vec())
}

/// write_value writes a value into an object.
fn write_value(obj: Obj<'_>, v: &Value) -> Result<(), String> {
    match v {
        Value::Null => obj.primitive(Null),
        Value::Bool(b) => obj.primitive(*b),
        Value::Int(i) => obj.primitive(*i),
        Value::Real(f) => obj.primitive(*f),
        Value::Name(n) => obj.primitive(Name(n.as_bytes())),
        Value::Str(s) => obj.primitive(Str(&hex(s)?)),
        Value::Text(s) => obj.primitive(TextStr(s)),
        Value::Ref(r) => obj.primitive(reference(*r)?),
        Value::Array(items) => {
            let mut a = obj.array();
            for item in items {
                write_value(a.push(), item)?;
            }
        }
        Value::Dict(pairs) => {
            let mut d = obj.dict();
            for (k, item) in pairs {
                write_value(d.insert(Name(k.as_bytes())), item)?;
            }
        }
    }
    Ok(())
}

fn reference(id: i32) -> Result<Ref, String> {
    if id < 1 {
        return Err(format!("the object number {id} is not positive"));
    }
    Ok(Ref::new(id))
}

fn filter(name: &str) -> Result<Filter, String> {
    Ok(match name {
        "ASCIIHexDecode" => Filter::AsciiHexDecode,
        "ASCII85Decode" => Filter::Ascii85Decode,
        "LZWDecode" => Filter::LzwDecode,
        "FlateDecode" => Filter::FlateDecode,
        "RunLengthDecode" => Filter::RunLengthDecode,
        "CCITTFaxDecode" => Filter::CcittFaxDecode,
        "JBIG2Decode" => Filter::Jbig2Decode,
        "DCTDecode" => Filter::DctDecode,
        "JPXDecode" => Filter::JpxDecode,
        "Crypt" => Filter::Crypt,
        _ => return Err(format!("the filter {name:?} is not known")),
    })
}

fn hex(s: &str) -> Result<Vec<u8>, String> {
    if s.len() % 2 != 0 {
        return Err(format!("the hex string {s:?} has an odd length"));
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).map_err(|e| format!("the hex string {s:?}: {e}")))
        .collect()
}
