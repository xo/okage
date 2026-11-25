"""Shape texts in a font with uharfbuzz, for the oracle of typesetting/harfbuzz.

Usage: python3 -I shape.py FONT [--flags] [--concat] [--language LANGUAGE] [--ranged TAG PERIOD] [--off TAG] [--scale SCALE]

Each line of the standard input is a text, as the hexadecimal values of its
characters, separated by spaces. For each text, the script writes one line
for each glyph, "glyph cluster x_advance y_advance x_offset y_offset", then
the line "error" when the buffer is in error, then the line "--". With the
argument --flags, the line of a glyph ends with 1 when the glyph is unsafe
to break, and with 0 when it is not. With the argument --concat, the
buffer has the flag PRODUCE_UNSAFE_TO_CONCAT, and the line of a glyph ends
with its flags UNSAFE_TO_BREAK (1) and UNSAFE_TO_CONCAT (2), from 0 to 3.
With the argument --language, the
script sets the language of each buffer to LANGUAGE. With the argument
--ranged, each character i of a text has the feature TAG in a range of its
own, on when i is not a multiple of PERIOD and off when it is. With the
argument --off, the feature TAG is off for the whole text. With the argument
--scale, the scale of the font is SCALE on both axes, in place of the units
per em of the font.

The script adds the characters with add_codepoints, as harfbuzz.AddRunes
does, so that the buffers of HarfBuzz and of harfbuzz grow in the same
steps. uharfbuzz raises MemoryError when the buffer is in error after the
shaping, and the script still writes the glyphs of the buffer.

With the argument --version, the script writes the version of HarfBuzz.
"""

import sys

import uharfbuzz as hb


def main():
    if sys.argv[1] == "--version":
        print(hb.version_string())
        return
    blob = hb.Blob.from_file_path(sys.argv[1])
    args = sys.argv[2:]
    flags = "--flags" in args
    concat = "--concat" in args
    language = None
    if "--language" in args:
        language = args[args.index("--language") + 1]
    ranged = None
    if "--ranged" in args:
        i = args.index("--ranged")
        ranged = (args[i + 1], int(args[i + 2]))
    off = None
    if "--off" in args:
        off = args[args.index("--off") + 1]
    font = hb.Font(hb.Face(blob))
    if "--scale" in args:
        scale = int(args[args.index("--scale") + 1])
        font.scale = (scale, scale)
    out = sys.stdout
    for line in sys.stdin:
        buf = hb.Buffer()
        codepoints = [int(v, 16) for v in line.split()]
        buf.add_codepoints(codepoints)
        if concat:
            buf.flags = hb.BufferFlags.PRODUCE_UNSAFE_TO_CONCAT
        if language is not None:
            buf.language = language
        buf.guess_segment_properties()
        features = {}
        if ranged is not None:
            tag, period = ranged
            features[tag] = [(i, i + 1, i % period != 0) for i in range(len(codepoints))]
        if off is not None:
            features[off] = False
        error = False
        try:
            hb.shape(font, buf, features)
        except MemoryError:
            error = True
        for info, pos in zip(buf.glyph_infos, buf.glyph_positions):
            out.write(f"{info.codepoint} {info.cluster} {pos.x_advance} {pos.y_advance} {pos.x_offset} {pos.y_offset}")
            if flags:
                out.write(f" {int(info.flags & hb.GlyphFlags.UNSAFE_TO_BREAK != 0)}")
            if concat:
                out.write(f" {int(info.flags & (hb.GlyphFlags.UNSAFE_TO_BREAK | hb.GlyphFlags.UNSAFE_TO_CONCAT))}")
            out.write("\n")
        if error:
            out.write("error\n")
        out.write("--\n")


main()
