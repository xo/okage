# D29. The filter types of `svg` take the suffix `Filter`

Status: Decided.

usvg keeps its filter types in the module `filter`, and `svg` holds them in
one package with the other types of usvg (D5). Names such as `Kind`, `Input`,
`Offset` and `Image` say little in the package `svg`, and `Image` collides
with the type of an image node. D10 says how a function that collides takes
the name of its module, and does not say it for a type.

1. Each type of `tree/filter.rs` of usvg takes the suffix `Filter` in Go:
   `Primitive` becomes `PrimitiveFilter`, `Kind` becomes `KindFilter`,
   `Offset` becomes `OffsetFilter`, and `Image` becomes `ImageFilter`. The
   type `Filter` keeps its name.
2. A name that the port makes from a type uses the new name of the type. A
   constant of an enum is the type and the case: `Input::SourceGraphic`
   becomes `InputFilterSourceGraphic`. The kind type of a struct with a kind
   (D27) is `InputFilterKind`. A case type of an interface is
   `ColorMatrixKindFilterMatrix`. A function of a type is
   `KindFilterHasInput` or `DefaultColorInterpolationFilter`.
3. A method keeps its Rust name, such as `Input` or `Kind`, because the type
   that holds it already says that it is a filter.
4. The doc comment of each type names the Rust type, such as
   `OffsetFilter is filter::Offset in usvg src/tree/filter.rs.`, so that a
   search for the Rust name still finds the port.
