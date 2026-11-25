The files of this folder are copies of files of subsetter at the commit
c3f23c67 (v0.2.6), byte for byte (D63). `Astrolabe_subset.ttf`,
`Astrolabe_subset_1.ttx` and the new head tables of the other `.ttx` files
come from the commit 8221117c:

| File | Source | License |
| --- | --- | --- |
| `subsets.tests` | `tests/data/subsets.tests` | the MIT license of subsetter, in `../LICENSE` |
| `ClickerScript-Regular.ttf` | `fonts/` | `ClickerScript-LICENSE-OFL.txt` |
| `DejaVuSansMono.ttf` | `fonts/` | `DejaVuSansMono-LICENSE.txt` |
| `MPLUS1p-Regular.ttf` | `fonts/` | `MPLUS1p-LICENSE-OFL.txt` |
| `NotoSans-Regular.ttf` | `fonts/` | `NotoSans-LICENSE-OFL.txt` |
| `Roboto-Regular.ttf` | `fonts/` | `Roboto-LICENSE-APACHE.txt` |
| `TestTTC.ttc` | `fonts/` | `TestTTC-LICENSE-MIT.txt` |
| `LatinModernRoman-Regular.otf` | `fonts/` | `LatinModernRoman-LICENSE-GUST.txt` |
| `EndcharInSubroutine.otf` | `fonts/` | `LatinModernRoman-LICENSE-GUST.txt` |
| `NewCMMath-Regular.otf` | `fonts/` | `NewCMMath-LICENSE-GUST.txt` |
| `NotoSansCJKsc-Bold-subset1.otf` | `fonts/` | `NotoSansCJKsc-LICENSE-OFL.txt` |
| `NotoSans-Regular_var.ttf` | `fonts/` | `NotoSans-Regular_var-LICENSE-OFL.txt` |
| `Cantarell-VF.otf` | `fonts/` | `Cantarell-LICENSE-OFL.txt` |
| `Astrolabe_subset.ttf` | `fonts/` | `Astrolabe-LICENSE-OFL.txt` |
| `NotoSans-Regular_var_1.ttx`, `NotoSans-Regular_var_2.ttx`, `NotoSans-Regular_var_3.ttx`, `Cantarell-VF_1.ttx`, `Cantarell-VF_2.ttx`, `Astrolabe_subset_1.ttx` | `tests/ttx/` | the MIT license of subsetter, in `../LICENSE` |

The `NOTICE` file of subsetter names the license of each font. Each license
file holds that license, with the copyright of the font from its `name`
table. `EndcharInSubroutine.otf` comes from `LatinModernRoman-Regular.otf`,
and has its license.

These are the TrueType fonts, the CFF fonts and the variable fonts of
subsetter. A file `.ttx` is the dump by fonttools of an instance that
subsetter makes in `tests/src/font_tools.rs`. Two CFF fonts of `fonts/` are
not here. `NotoSansCJKsc-Regular.otf` has 16 MB, and the `NOTICE` file does
not name the license of `NotoSansCJKsc-Regular_custom_font_matrix.otf`. The
test module reads both from the checkout of subsetter.
