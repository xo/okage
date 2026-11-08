# D13. Each package keeps the license of its upstream

Status: Decided.

okage translates code under three licenses, and a translation keeps the
license of its source:

| Upstream | License | okage packages |
| --- | --- | --- |
| tiny-skia | BSD-3-Clause, copyright Google Inc. 2011 and Yevhenii Reizner 2020 | the root package, `path`, `internal/wide` |
| strict-num, xmlwriter | MIT, copyright Yevhenii Reizner | `path`, `internal/xmlwriter` |
| resvg, svgtypes, roxmltree, simplecss, kurbo | Apache-2.0 or MIT, at the choice of the user | `svg`, `internal/svgtypes`, `internal/roxmltree`, `internal/simplecss`, `internal/kurbo` |

1. `LICENSE` in the root is the BSD-3-Clause license of tiny-skia, with the
   copyright lines of tiny-skia and the copyright line of okage. It covers the
   root package, `path` and `internal/wide`.
2. A package that ports a crate under Apache-2.0 or MIT takes the MIT license,
   with the copyright line of the crate and of okage, in a `LICENSE` file in
   its own folder. pkg.go.dev reads the license of each folder.
3. `path` also ports strict-num, under MIT. Its folder holds a `NOTICE` file
   with the MIT license and the copyright line of strict-num.
4. The test images and the test fonts copied from upstream keep their own
   license files beside them. The fonts of resvg are under the OFL, Apache-2.0
   and MIT licenses.

BSD-3-Clause also asks that the names of the copyright holders are not used
to promote a product made from the code without permission.
