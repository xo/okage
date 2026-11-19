//! The images of the question "krilla" that `scene` does not make itself,
//! for the comparison of the images of phase 15 (D62).
//!
//! An image of a scene can have these fields, besides the fields that
//! `scene` reads:
//!
//! - "custom": the samples of `Image::from_custom`, which `CustomSpec` holds.
//!   The image then has no file.
//! - "kind" "png_native": the file goes to `Image::from_png_native`.

use std::hash::{Hash, Hasher};

use base64::Engine;
use krilla::image::{BitsPerComponent, CustomImage, Image, ImageColorspace};
use serde::Deserialize;

/// The fields of an image that this module reads.
#[derive(Deserialize, Default)]
pub(crate) struct ExtraSpec {
    #[serde(default)]
    custom: Option<CustomSpec>,
}

/// The samples of a custom image. "color" and "alpha" hold the samples in
/// base64, with 8 or 16 bits each, in big-endian order. "color_space" is
/// "rgb", "luma" or "cmyk". "icc" is the path of an ICC profile.
#[derive(Deserialize)]
struct CustomSpec {
    color: String,
    #[serde(default)]
    alpha: Option<String>,
    bits: u8,
    width: u32,
    height: u32,
    color_space: String,
    #[serde(default)]
    icc: Option<String>,
}

/// A custom image with the samples of a `CustomSpec`.
#[derive(Clone)]
struct Custom {
    color: Vec<u8>,
    alpha: Option<Vec<u8>>,
    bits: BitsPerComponent,
    size: (u32, u32),
    icc: Option<Vec<u8>>,
    color_space: ImageColorspace,
}

impl Hash for Custom {
    fn hash<H: Hasher>(&self, state: &mut H) {
        self.color.hash(state);
        self.alpha.hash(state);
        self.bits.hash(state);
        self.size.hash(state);
        self.icc.hash(state);
        self.color_space.hash(state);
    }
}

impl CustomImage for Custom {
    fn color_channel(&self) -> &[u8] {
        &self.color
    }

    fn alpha_channel(&self) -> Option<&[u8]> {
        self.alpha.as_deref()
    }

    fn bits_per_component(&self) -> BitsPerComponent {
        self.bits
    }

    fn size(&self) -> (u32, u32) {
        self.size
    }

    fn icc_profile(&self) -> Option<&[u8]> {
        self.icc.as_deref()
    }

    fn color_space(&self) -> ImageColorspace {
        self.color_space
    }
}

fn base64(s: &str) -> Result<Vec<u8>, String> {
    base64::engine::general_purpose::STANDARD
        .decode(s)
        .map_err(|e| format!("decoding the samples of a custom image: {e}"))
}

fn read(file: &str) -> Result<Vec<u8>, String> {
    std::fs::read(file).map_err(|e| format!("reading {file}: {e}"))
}

/// custom makes the custom image of the field "custom", or returns None when
/// the image has no such field.
pub(crate) fn custom(x: &ExtraSpec, interpolate: bool) -> Option<Result<Image, String>> {
    let c = x.custom.as_ref()?;
    Some((|| {
        let bits = match c.bits {
            8 => BitsPerComponent::Eight,
            16 => BitsPerComponent::Sixteen,
            b => return Err(format!("{b} bits for each component are not known")),
        };
        let color_space = match c.color_space.as_str() {
            "rgb" => ImageColorspace::Rgb,
            "luma" => ImageColorspace::Luma,
            "cmyk" => ImageColorspace::Cmyk,
            s => return Err(format!("the color space {s} is not known")),
        };
        let image = Custom {
            color: base64(&c.color)?,
            alpha: c.alpha.as_deref().map(base64).transpose()?,
            bits,
            size: (c.width, c.height),
            icc: c.icc.as_deref().map(read).transpose()?,
            color_space,
        };
        Image::from_custom(image, interpolate)
    })())
}

/// by_kind makes the image of a file for the kinds of this module, or
/// returns None for another kind.
pub(crate) fn by_kind(kind: &str, data: &[u8], interpolate: bool) -> Option<Result<Image, String>> {
    match kind {
        "png_native" => Some(Image::from_png_native(data.to_vec().into(), interpolate)),
        _ => None,
    }
}
