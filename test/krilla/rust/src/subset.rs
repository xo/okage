//! The subset of a font, as krilla makes it for a CID font.

use std::hash::{Hash, Hasher};

use serde::Deserialize;
use siphasher::sip128::{Hasher128, SipHasher13};
use subsetter::{GlyphRemapper, Tag};

/// A question for the subset of a font.
#[derive(Deserialize)]
pub struct Question {
    /// The path of the font file.
    font: String,
    /// The index of the font in a collection.
    #[serde(default)]
    index: u32,
    /// The glyphs, in the order of their first use. The glyph 0 always comes
    /// first, as `GlyphRemapper::new` puts it there.
    glyphs: Vec<u16>,
    /// The coordinates of the variations, such as `[["wght", 700]]`. When the
    /// field is there, even empty, the oracle calls
    /// `subset_with_variations`, as krilla does. Without it, the oracle
    /// calls `subset`.
    #[serde(default)]
    coords: Option<Vec<(String, f32)>>,
}

/// answer subsets the font. The answer is the tag of six letters that krilla
/// puts in front of the name of the font, then the bytes of the subset.
pub fn answer(q: &Question) -> Result<Vec<u8>, String> {
    let data = std::fs::read(&q.font).map_err(|e| format!("reading {}: {e}", q.font))?;
    let mut mapper = GlyphRemapper::new();
    for g in &q.glyphs {
        mapper.remap(*g);
    }
    let subset = match &q.coords {
        None => subsetter::subset(&data, q.index, &mapper),
        Some(coords) => {
            let mut tags = Vec::with_capacity(coords.len());
            for (name, value) in coords {
                let b: [u8; 4] = name
                    .as_bytes()
                    .try_into()
                    .map_err(|_| format!("the tag {name:?} does not have four bytes"))?;
                tags.push((Tag::new(&b), *value));
            }
            subsetter::subset_with_variations(&data, q.index, &tags, &mapper)
        }
    }
    .map_err(|e| format!("subsetting: {e}"))?;
    let mut out = subset_tag(&mapper).into_bytes();
    out.extend_from_slice(&subset);
    Ok(out)
}

/// subset_tag is `subset_tag` of `text/cid.rs` of krilla, which hashes the
/// glyph remapper of a CID font into the tag of its name.
fn subset_tag<T: Hash>(data: &T) -> String {
    const BASE: u128 = 26;
    const SUBSET_TAG_LEN: usize = 6;
    let mut hash = stable_hash128(data);
    let mut letter = [b'A'; SUBSET_TAG_LEN];
    for l in letter.iter_mut() {
        *l = b'A' + (hash % BASE) as u8;
        hash /= BASE;
    }
    std::str::from_utf8(&letter).unwrap().to_string()
}

/// stable_hash128 is `stable_hash128` of `util.rs` of krilla.
fn stable_hash128<T: Hash + ?Sized>(value: &T) -> u128 {
    struct StableHasher(SipHasher13);

    impl Hasher for StableHasher {
        fn finish(&self) -> u64 {
            self.0.finish()
        }

        fn write(&mut self, bytes: &[u8]) {
            self.0.write(bytes);
        }

        fn write_usize(&mut self, i: usize) {
            self.0.write_u64(i as u64);
        }
    }

    let mut state = StableHasher(SipHasher13::new());
    value.hash(&mut state);
    state.0.finish128().as_u128()
}
