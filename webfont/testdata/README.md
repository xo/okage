# Test fonts of `webfont`

`make.py` writes these fonts from the fonts of `svg/testdata/fonts`. Run it in
this folder with fontTools, the Python package brotli and `woff2_compress` of
google/woff2. The fonts are small subsets:

| File | What it is |
| --- | --- |
| `noto.ttf` | a subset of Noto Sans Regular, with the flag OVERLAP_SIMPLE on each simple glyph |
| `noto.woff` | `noto.ttf` in WOFF 1.0, from fontTools |
| `noto.woff2` | `noto.ttf` in WOFF2, with the transforms of glyf, loca and hmtx, from fontTools |
| `noto-null.woff2` | `noto.ttf` in WOFF2, with no transform, from fontTools |
| `noto.eot`, `noto-xor.eot` | `noto.ttf` in EOT, without and with the XOR of the font data |
| `noto-mtx.eot` | an EOT file whose header says that it uses MicroType Express compression, which okage does not read |
| `cff.otf`, `cff.woff2` | a subset of `CFF-and-SBIX.otf` without its sbix table, and the same font in WOFF2 |
| `collection.ttc`, `collection.woff2` | Noto Sans Regular and Bold in a collection that shares tables, and the same collection in WOFF2, from `woff2_compress` |

The Noto fonts are under the OFL, in `Noto-LICENSE-OFL.txt`. `CFF-and-SBIX.otf`
is under the Apache License 2.0, in `CFF-and-SBIX-LICENSE-APACHE.txt`.
