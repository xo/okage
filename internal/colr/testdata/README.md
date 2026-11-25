The files of this folder are copies of files of the fontations repository
at the tag `skrifa-v0.49.0`, the commit `47a30161`, byte for byte (D101):

| File | Source | License |
| --- | --- | --- |
| `test_glyphs-glyf_colr_1_variable.ttf` | `font-test-data/test_data/ttf/` | the Apache-2.0 license or the MIT license of font-test-data |
| `LICENSE-APACHE` | `font-test-data/` | |
| `LICENSE-MIT` | `font-test-data/` | |

The font is the test font of COLRv1 with variations. Its table COLR has an
item variation store with deltas of 32 bits. The tests of `internal/colr`
and of `svg` read it.
