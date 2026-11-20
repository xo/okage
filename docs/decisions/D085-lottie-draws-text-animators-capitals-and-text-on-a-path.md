# D85. lottie draws text animators, capitals and text on a path

Status: Decided, amends D79.

D79 lets `lottie` draw a text layer, and leaves its text animators and its
text on a path in the backlog (items 2 and 3 of "lottie" in
[`../BACKLOG.md`](../BACKLOG.md)). A text animator is an entry of the key
`a` of a text layer. It has a range selector, which chooses some of the
glyphs, and a style, which changes those glyphs: their position, scale,
rotation, opacity, fill and stroke, and the space around them. The key `m`
groups the glyphs that turn about one point. The key `ca` of a text
document draws small letters as capitals, and the key `p` puts the text on
the path of a mask. velato parses none of these, so each part is a
deliberate difference from velato. ThorVG, the second oracle of `lottie`
(D43), draws all of them. On 2026-11-21 the maintainers accepted all of
this decision, answer 1 of question 37 of [`../PLAN.md`](../PLAN.md).

On 2026-11-20 the source of ThorVG was read at the tag `v1.1.1`, the version
that `github.com/xo/lottie` v0.1.0 runs, as D82 says. Each part has a
prototype in files of okage that start with `okage_`, and a test in
`test/lottie/compat_test.go` that makes its own file and compares it with
ThorVG within the tolerance of D43.

## What ThorVG does

1. The range selector. `LottieTextRange::factor` in
   `src/loaders/lottie/tvgLottieModel.cpp` gives each glyph a share from 0
   to 1. The start, the end and the offset of the range are in percent of
   the length of the text in bytes, or in glyphs. The six shapes are
   square, ramp up, ramp down, triangle, round and smooth. The smoothness
   softens the square, and the keys `xe` and `ne` ease the share with a
   curve. The share is multiplied by the amount, the key `a`, which is 0
   when the file does not give it. The key `b` chooses what the index of a
   glyph counts: the bytes, the bytes without the spaces, the line plus the
   spaces before the glyph for the words, or the line. The key `rn` starts
   the range at a place that `rand` of C chooses.
2. The style, for a font with glyphs in the list `chars`.
   `updateTextRange` in `src/loaders/lottie/tvgLottieBuilder.cpp` adds the
   share of each animator to the glyph: the offset of the position, the
   scale, the rotation, the opacity, the fill opacity and the stroke
   opacity, the fill and the stroke color, and the stroke width. The
   tracking of an animator, the key `t`, moves the glyph and each glyph
   after it. The line spacing, the key `ls`, moves the next lines down by
   the largest spacing of the line. A glyph that an animator selects draws
   in the group of the key `m`: one character, word or line. The first such
   glyph of a group sets its rotation, which turns the group about the
   anchor point. ThorVG takes the x of the anchor where the y belongs in one
   term of the pivot, and the reverse. The group 4, all the text, gives
   each glyph a group of its own, as the group 1 does.
3. The style, for a font that ThorVG shapes. `updateURLFont` takes the share
   of the first glyph for the whole text. Each animator with a share sets
   the opacity and moves the fill color. An animator with a stroke width
   also sets the outline. The position, the scale, the rotation, the
   tracking and the line spacing do nothing.
4. The capitals. With the key `ca` at 1 or 2, `_searchGlyph` looks for the
   glyph of the capital of a small letter of ASCII, and with 2 it draws the
   glyph at 70 percent of the size, with 70 percent of its advance. A font
   that ThorVG shapes ignores the key. The build of ThorVG that
   `github.com/xo/lottie` runs draws no small letter at all when the key is
   not 0, even when the list `chars` holds it. `_searchGlyph` keeps a
   pointer to a local variable after its scope ends, so the search compares
   with a byte that is no longer there. lottie-web does not read the key.
5. The text on a path. `LottieTextFollowPath` in `tvgLottieModel.cpp` reads
   the path of the mask whose index is the key `m` of the key `p`, and the
   first margin, the key `f`. `_commit` puts the middle of each glyph on the
   path, at the length of the cursor plus half the advance plus the margin.
   It does not turn the glyph. Before the start and after the end, a closed
   path wraps around, and an open path goes on in a straight line. Each
   line of the text starts at the start of the path again. A glyph that an
   animator selects ignores the path. A font that ThorVG shapes ignores the
   path.

## The prototypes

The schema is in `okage_text_schema.go`, the model, the range selector and
the style in `okage_text_animator.go`, the path in `okage_text_path.go`,
and the layout in `okage_text.go`. A file without these keys draws the same
bytes as before. The test `TestThorVGText` gives the same share of pixels
that differ as before, in each frame.

| Part | Test file of `compat_test.go` | Pixels that differ from ThorVG in a frame, before and after |
| --- | --- | --- |
| 1 and 2 | `text-animator-move`: the shape square, with the position, the opacity and the fill color, and an amount that changes | 0.7% to 3.4% before, 0% after |
| 1 and 2 | `text-animator-turn`: the shapes triangle and ramp down, with the scale and the rotation, for groups of characters and of words | 1.1% to 4.7% before, 0% after |
| 1 and 2 | `text-animator-stroke`: the shapes round and smooth, with the stroke and the fill opacity, the ease, the units of glyphs and groups of lines | 2.2% to 3.9% before, at most 0.18% after |
| 1 and 2 | `text-animator-spacing`: the shape ramp up, with the tracking and the line spacing, for the words of a box text | up to 4.0% before, 0% after |
| 3 | `text-animator-font`: Noto Sans from a font database, with the fill color, the opacity, the stroke, and a position and a rotation that ThorVG ignores | up to 3.1% before, at most 0.31% after |
| 4 | `text-caps-all` and `text-caps-small`: all capitals and small capitals, with a stroke and in a box | 5.3% and 2.4% before, at most 0.05% after |
| 5 | `text-path-open` and `text-path-closed`: an open curve and a closed path, with a margin that goes past both ends | 3.0% to 6.3% before, at most 0.01% after |

The build of ThorVG draws no small letter with the key `ca`, as part 4 of
"What ThorVG does" says. So the two tests of the capitals give ThorVG a file
that gives the same image without the key: the capitals in place of the
small letters, and for small capitals, a text of only capitals at 70
percent of the size and of the stroke width.

The tests of the corpus give the same results as before. No file of the
corpus has a text layer.

## The decision

1. `lottie` parses the text animators of a text layer, the key `m` and the
   key `p`, into the new field `TextAnimation` of `Content`. It reads the
   key `ca` of a text document into the new field `Caps` of
   `TextDocument`.
2. The `PixmapSink` draws the text animators as ThorVG does, with the
   faults of parts 1 to 3 of "What ThorVG does", such as the index of a
   word, the length in bytes, the pivot of a group and the group 4. A text
   in a font with glyphs in the list `chars` draws them glyph by glyph. A
   text that the sink shapes with the fonts of `WithFontDatabase` draws them
   for the whole text, as `updateURLFont` does.
3. okage reads the key `rn` and does not follow it, because the range of
   ThorVG depends on `rand` of C. A range with the key `rn` stays where the
   file puts it.
4. The key `ca` follows the source of ThorVG, not its build: a small letter
   of ASCII draws the glyph of its capital from the list `chars`, at 70
   percent for small capitals. A text that the sink shapes ignores the key,
   as in ThorVG.
5. A text in a font with glyphs in the list `chars` follows the path of the
   key `p` as `_commit` of ThorVG does, without turning a glyph. A
   character of such a text that the sink shapes, because the list `chars`
   does not hold it, takes the animators and the path as a glyph of the
   list does. ThorVG draws no such character.
6. Each part of the code has a comment that names D85.
7. Items 2 and 3 of "lottie" in the backlog are deleted, the status of D79
   says "amended by D82 and D85", and the item of `lottie` in `README.md`
   names the text animators, the capitals and the text on a path.

## When upstream changes

If velato parses text animators, capitals or a text on a path, port its
commit as `docs/UPSTREAM.md` says, and remove the part of this decision
that it replaces. If the result is not the same, a maintainer decides which
one okage keeps.

If a new version of ThorVG, as `github.com/xo/lottie` runs it, changes one
of the behaviors of "What ThorVG does", run the tests of
`test/lottie/compat_test.go`. For example, a fix of `_searchGlyph` lets the
tests of the capitals compare with the same file. A version that turns
the glyphs on a path changes part 5. If a test fails, change the part of
the prototype to follow ThorVG, or record a decision that keeps it.

## Why

D43 makes ThorVG the reference where velato leaves a feature out. A Lottie
file animates a title or a caption with text animators. Without them, a
text layer draws as a static text, and the animation does not move. Each
part makes a test against ThorVG pass that fails without it.

The fault of the build of ThorVG with the key `ca` makes a text with
capitals lose its small letters. That is not what the source of ThorVG
means to draw, so okage follows the source. The tests compare it with
ThorVG through a file that gives the same image.
