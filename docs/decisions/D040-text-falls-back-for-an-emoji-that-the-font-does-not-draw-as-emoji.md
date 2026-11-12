# D40. Text falls back for an emoji that the font does not draw as an emoji

Status: Decided, amends D39.

After D39, a grapheme cluster falls back only when the font has a missing
glyph for it. A grapheme cluster is what a reader sees as one character, such
as a flag or a family of emoji. Two kinds of emoji then draw wrong:

- A character with U+FE0F, the selector that asks for the emoji form. Noto
  Sans has a glyph for ©, so "©️" draws as a black ©, with U+FE0F as a blank
  glyph. A browser draws it with the emoji font.
- An emoji sequence in a font that has each emoji but not the sequence. The
  font draws the emoji of a ZWJ sequence side by side, and the fallback does
  not look for a font that draws the sequence as one emoji.

## The decision

1. The fallback gives each cluster a rank. A cluster with a missing glyph has
   the rank "missing". An emoji cluster that the font does not draw as one
   emoji has the rank "not emoji". Each other cluster has the rank "drawn".
2. An emoji cluster is "not emoji" in two cases. It holds U+FE0F, and no
   glyph of the cluster is a glyph of the tables COLR, SVG, CBDT or sbix. Or
   it is an emoji sequence, and the font gives more than one glyph with an
   advance for it.
3. An emoji sequence is a cluster of two or more characters that holds
   U+FE0F, U+20E3, a tag from U+E0020 to U+E007F, or a character from U+1F000
   to U+1FAFF. This test uses ranges of characters, not the Unicode property
   Extended_Pictographic, because go-text keeps its emoji data in an internal
   package. A cluster of a script, such as a conjunct of Devanagari with
   U+200D, holds none of these characters.
4. The fallback goes on while a cluster is "missing" or "not emoji", and it
   tries the fonts in the order of the fallback selector. A piece of the text
   takes the glyphs of the fallback font only when their rank is better than
   the rank of the glyphs that it has. This extends item 2 of D39.
5. When no font draws a "not emoji" cluster better, the cluster keeps the
   glyphs of its font. It is not missing, so the layout does not warn about
   it.
6. Upstream replaces the glyphs of the whole run when the fallback font has
   no missing glyph. okage does this only when the first font has a missing
   glyph and the fallback font has the rank "drawn" for every cluster. So a
   run that has only a "not emoji" cluster to fix does not move to the emoji
   font as a whole.
7. This is a deliberate difference from upstream. It applies to `ShapeText`
   and to the `<text>` elements of SVG data. Text without U+FE0F and without
   an emoji sequence falls back as with D39 alone.

## When upstream changes

The differences of D39 and D40 replace parts of `shape_text` in
`crates/usvg/src/text/layout.rs` of resvg. On 2026-11-12, at resvg
`5bdbae64`, that function still copies glyphs one by one, with the comments
"We assume, that shaping with an any font will produce the same amount of
glyphs" and "TODO: Replace clusters and not glyphs". okage follows upstream
where upstream has an implementation. So when a resvg commit changes the
fallback of `shape_text`, follow these steps:

1. Remove the okage code of D39 and D40 from `shapeText` in
   `svg/usvg_text_layout.go`. This code is `replaceMissingClusters`,
   `fallbackRanks`, `fallbackRank`, `advancingGlyphs`, `isEmojiSequence`,
   the list `unresolved`, and the changed test for `allMatched`. Also remove
   `fontCache.hasColorGlyph` in `svg/usvg_text_cache.go`. Each part has a
   comment that starts with "Port:" and names D39 or D40.
2. Port the commit of upstream as `docs/UPSTREAM.md` says.
3. Run the tests of D39 and D40 in `svg/text_color_test.go` and
   `svg/usvg_text_fallback_test.go`. If a test fails because upstream does
   not cover that case, a maintainer decides whether okage keeps that part of
   the difference on top of the code of upstream. Record the result in D39,
   in D40, or in a new decision.
4. Update the list "Differences from upstream" in `README.md`.

Do not keep the okage code next to a fix of upstream that covers the same
case. Two implementations of one fallback give results that neither
upstream nor the tests of okage describe.

## Why

zuzu draws labels with `ShapeText`, and a label often holds ©️, ™️ or ❤️.
A reader expects the emoji form there, as a browser draws it. The checks of
D40 use only facts of the fonts: a missing glyph, a color glyph, and the
number of glyphs with an advance. The test fonts of resvg hold no cluster
with U+FE0F that a text font draws, and no emoji sequence without its
ligature, so the change gives the same images as resvg for its tests.
