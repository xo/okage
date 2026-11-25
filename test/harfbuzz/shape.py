"""Shape texts in a font with uharfbuzz, for the oracle of typesetting/harfbuzz.

Usage: python3 -I shape.py FONT [--flags] [--concat] [--language LANGUAGE] [--ranged TAG PERIOD] [--off TAG] [--scale SCALE] [--direction DIRECTION] [--feature TAG START END VALUE]... [--not-found GLYPH] [--cluster-level LEVEL] [--item-offset OFFSET]

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
per em of the font. With the argument --direction, the direction of each
buffer is DIRECTION, such as ttb. Each argument --feature adds a request
for the feature TAG with VALUE from START to END, in the order of the
arguments. An END of -1 is the end of the text, and START 0 with END -1
is a global request. With the argument --not-found, the buffer gives the
glyph GLYPH to a character that the font does not have. With the argument
--cluster-level, the cluster level of the buffer is LEVEL, from 0 to 2.
With the argument --item-offset, the first OFFSET characters of a text are
the context before the text, and the buffer holds the rest.

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
    direction = None
    if "--direction" in args:
        direction = args[args.index("--direction") + 1]
    requests = []
    for i, arg in enumerate(args):
        if arg == "--feature":
            end = int(args[i + 3])
            requests.append((args[i + 1], int(args[i + 2]), 0xFFFFFFFF if end < 0 else end, int(args[i + 4])))
    not_found = None
    if "--not-found" in args:
        not_found = int(args[args.index("--not-found") + 1])
    cluster_level = None
    if "--cluster-level" in args:
        cluster_level = int(args[args.index("--cluster-level") + 1])
    item_offset = 0
    if "--item-offset" in args:
        item_offset = int(args[args.index("--item-offset") + 1])
    font = hb.Font(hb.Face(blob))
    if "--scale" in args:
        scale = int(args[args.index("--scale") + 1])
        font.scale = (scale, scale)
    out = sys.stdout
    for line in sys.stdin:
        buf = hb.Buffer()
        codepoints = [int(v, 16) for v in line.split()]
        buf.add_codepoints(codepoints, item_offset)
        codepoints = codepoints[item_offset:]
        if not_found is not None:
            buf.not_found_glyph = not_found
        if cluster_level is not None:
            buf.cluster_level = hb.BufferClusterLevel(cluster_level)
        if concat:
            buf.flags = hb.BufferFlags.PRODUCE_UNSAFE_TO_CONCAT
        if language is not None:
            buf.language = language
        buf.guess_segment_properties()
        if direction is not None:
            buf.direction = direction
        features = {}
        for tag, start, end, value in requests:
            features.setdefault(tag, []).append((start, end, value))
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
