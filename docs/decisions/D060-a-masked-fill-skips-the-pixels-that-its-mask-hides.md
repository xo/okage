# D60. A masked fill skips the pixels that its mask hides

Status: Decided.

## Context

ivy clips most of what it draws with a mask of the size of the page. The
pipeline of tiny-skia runs on each pixel of the rectangle of a fill, also
where the mask is 0. A spreadsheet of ivy drew one image of 1605 by 801
pixels about 86 times on a page, each time through a clip of one row, and a
page took 1.3 s. `Mask.IntersectPath` also made, cleared and multiplied a
mask of the size of the page for each clip, and a form of PDF spent about
half of its 7.3 s there. The maintainers asked on 2026-11-18 for the speed
of okage and ivy on large files.

## Decision

1. `blitRect` of the pipeline leaves out the rows of its rectangle where the
   mask is 0, and the chunks of 16 pixels at the left and at the right
   where the mask is 0. The chunks that stay keep their places, so the
   stages see the same lanes as in upstream. The stage `mask_u8` of
   upstream already stores nothing for a chunk whose mask is 0, so the bytes
   are the same as the bytes of tiny-skia for each blend mode and each
   space of color.
2. `Mask.IntersectPath` takes its temporary mask from a pool, and clears and
   multiplies only the rows that the path can cover, with a margin of 2
   pixels. It sets the other pixels to 0, which is the result of the
   multiplication of upstream.
3. A test compares each change with the work of upstream on the whole
   rectangle, for each blend mode and in the pipelines lowp and highp.

## Consequences

A fill through a small clip of a large mask takes a time that depends on
the clip, not on the page. In ivy the image of the spreadsheet went from
1.3 s to 0.1 s for each page, and the form of PDF from 7.3 s to 3.1 s. The
Rust oracle and the reference images of tiny-skia give the same bytes.
`blitAntiH` and `blitMask` skip the rows of 0 in the same way since
2026-11-20, under D35, with the same bytes. A `Mask` also holds only the
rows and the columns that its paths cover, with a margin of 15 columns on
each side, so a clip allocates less. Each chunk that `blitRect` keeps is
then inside the columns that the mask holds.
