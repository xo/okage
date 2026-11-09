# D27. A Rust enum with data becomes a struct or an interface

Status: Decided.

`docs/UPSTREAM.md` says that the port of the first Rust enum with data
chooses its Go form, and that a decision records it. The port of tiny-skia
made two such choices on 2026-11-09:

1. When each case holds the same kind of data, the enum becomes a struct with
   a kind field and the data. `PathSegment` of tiny-skia-path is
   `path.PathSegment{Kind, Points}`, and the private `GradientType` of
   tiny-skia is a struct with a kind. A switch on the kind takes the place of
   the Rust match.
2. When the cases hold different data and the code calls methods on each, the
   enum becomes an interface with one value type for each case. `Shader` is
   an interface with the methods `IsOpaque`, `Transform`, `ApplyOpacity` and
   an unexported `pushStages`, and the cases are `SolidColor`,
   `LinearGradient`, `RadialGradient`, `SweepGradient` and `Pattern`. A
   method that changes a shader returns a changed copy, so a copy of a
   `Paint` behaves as a clone in Rust does.

A later port chooses one of the two forms by the same test. A form that is
neither is a decision of its own.
