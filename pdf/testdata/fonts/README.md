# Subsetting

The emoji fonts were subsetted using the following commands:

- Noto Color Emoji COLR: `fonttools subset --text-file=../emojis.txt --text-file=../emojis_colr_composite.txt --drop-tables+=SVG --output-file=NotoColorEmoji.COLR.subset.ttf NotoColorEmoji_COLR.ttf`
- Noto Color Emoji SVG: `fonttools subset --text-file=../emojis.txt --output-file=NotoColorEmoji.SVG.subset.ttf --drop-tables+=COLR NotoColorEmoji-Regular.ttf`
- Noto Color EMOJI EBDT: `hb-subset --text-file ../emojis.txt --output-file=NotoColorEmoji.CBDT.subset.ttf NotoColorEmoji.ttf`
- Twitter Color Emoji: `fonttools subset --text-file=../emojis.txt --output-file=TwitterColorEmoji.subset.ttf TwitterColorEmoji-SVGinOT.ttf`
- Emoji One: `fonttools subset --text-file=../emojis.txt --output-file=EmojiOne.subset.otf EmojiOneColor.otf`
# The copies of okage

okage copies these fonts from `assets/fonts` of krilla, byte for byte, for
the tests of `pdf` and `test/krilla`. This file, up to this section, is the
`README.md` of that folder. Each font has its license in this folder:

- `DejaVuSansMono.ttf`: `DejaVuSansMono_LICENSE.md`.
- `LatinModernRoman-Regular.otf`: `LatinModernRoman_LICENSE.md`.
- `LibertinusSerif-Regular.otf`: `LibertinusSerif_LICENSE.md`.
- `NewCMMath-Regular.otf`: `NewCMMath_LICENSE.md`.
- The Noto fonts and `NotoColorEmoji.*.subset.ttf`: `Noto_LICENSE.md`.
- `TwitterColorEmoji.subset.ttf`: `TwitterColorEmoji_LICENSE.md`.
- `sbix.ttf` and `sbix.otf`: `sbix_LICENSE.md`.

krilla has no license file for two of these fonts. okage wrote them from
the table `name` of each font:

- `LibertinusSerif_LICENSE.md` holds the copyright line of the font,
  `Copyright © 2012-2024 The Libertinus Project Authors.`, the notice of
  the SIL Open Font License that the font gives, and the text of the SIL
  Open Font License 1.1 without a change.
- `sbix_LICENSE.md` holds the copyright line and the notice of the font,
  `Copyright 2026 the Vello Authors`, with Apache-2.0 OR MIT. okage takes
  the MIT license, whose text follows the notice.
