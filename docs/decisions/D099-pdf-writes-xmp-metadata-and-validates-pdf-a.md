# D99. `pdf` writes XMP metadata and validates PDF/A

Status: Decided, amends D62.

D62 left the checks of PDF/A and PDF/UA, their XMP metadata and the
embedded files out of the base port of krilla. Question 22 asked for that
scope, and D62 records it in item 1. So `pdf` wrote no XMP metadata, where
krilla writes it by default. On 2026-11-21, the commit `bb320e49` of krilla
added custom XMP metadata, and the ledger of krilla left most of it out. On
2026-11-25 the maintainers decided to port these parts of krilla at the
commit `7b15610d`, the matched commit of krilla.

## What is ported

1. The folder `src/interchange/metadata` of krilla. `mod.rs` stays
   `pdf/metadata.go`, and gains the XMP metadata of a document and the
   methods `custom_xmp_property` and `custom_xmp_schema`. `custom.rs`,
   `extension_schema.rs`, `predefined.rs`, `relax_ng.rs` and `xmp.rs` are
   `pdf/metadata_custom.go`, `pdf/metadata_extension_schema.go`,
   `pdf/metadata_predefined.go`, `pdf/metadata_relax_ng.go` and
   `pdf/metadata_xmp.go`. `internal/xmlwriter` writes the RELAX NG schema of
   PDF/A-4f and PDF/A-4e.
2. The validators of PDF/A of `src/configure/validate.rs`, with the
   builder of `src/configure/mod.rs`, in `pdf/configure_validate.go` and
   `pdf/configure.go`.
3. The embedded files of `src/interchange/embed.rs`, in `pdf/embed.go`,
   with `Document::embed_file`. PDF/A-3 and Factur-X need them. The writers
   `EmbeddedFile` and `EmbeddingParams` of `src/files.rs` of pdf-writer come
   into `internal/pdfwriter` for them.
4. The parts of `src/error.rs`, `src/chunk_container.rs` and
   `src/serialize.rs` that these files need: the XMP stream of the catalog,
   the output intent of PDF/A, the names and the associated files of the
   embedded files, and `KrillaError::Xmp`.
5. The crate xmp-writer, as the new package `internal/xmpwriter`. krilla at
   `7b15610d` uses xmp-writer 0.3.3 at the commit
   `9498ae2b2ab874c11c22edb9e393470584e419c8` of
   `https://github.com/typst/xmp-writer`, from git and not from crates.io.
   That commit has changes that the release 0.3.3 on crates.io does not
   have, such as `Namespace::PREDEFINED` and URIs as resources. The port
   follows the commit. The cargo cache held the commit after the oracle in
   `test/krilla` built it, so `upstream/xmp-writer` is a copy of that cargo
   checkout, made without a fetch from the network. The crate is under MIT
   or Apache-2.0. The package keeps the MIT license with the copyright lines
   of the crate and of okage (D13). It gets a row in
   `docs/upstream/upstreams.tsv` and a ledger (D12). Its base commit and its
   matched commit are `9498ae2b`, the head of its main branch on
   2026-11-25. The crate has no tests, so `internal/xmpwriter` compares
   itself with the output of a small Rust program, `testdata/reference.rs`,
   which calls each method of the crate.

## What is not ported

1. The validator of PDF/UA, `Accessibility::UA1`. It requires the tags of
   PDF/UA, which `pdf` does not have (D62). So each document with it fails
   with the validation error `MissingTagging`. The validators `A1_A`,
   `A2_A` and `A3_A` of PDF/A also require the tags. They are ported,
   because they are cases of the same enum, and a document with one of them
   fails with `MissingTagging`, as a document of krilla without tags does.
2. The separation colors, as in D62. So `ValidationStore` has no port, and
   no error `InconsistentSeparationFallback` is registered.

## The API

1. `WithXMPMetadata` is the field `xmp_metadata` of `SerializeSettings`.
   Its default is true, as in krilla, so each document of `pdf` now holds
   XMP metadata. A validator of PDF/A turns it on.
2. `WithArchivalValidator` takes an `Archival`, the validator of PDF/A.
   `NewDocument` builds the configuration of krilla after its last option,
   so the order of `WithPDFVersion` and `WithArchivalValidator` does not
   matter. A version that the validator does not allow gives
   `okage.ErrPDFVersion`, as `ConfigurationError` of krilla does. Without a
   version, the version is the highest version that the validator allows.
3. The types of the public module `krilla::metadata::xmp`, such as
   `Namespace`, `Schema`, `Property` and `Value`, are in `pdf` with their
   Rust names, because no name of `pdf` collides with them (D10). A case of
   an enum with data has a constructor, such as `TextValue` and
   `OrderedArrayType`.
4. `Document.EmbedFile` takes an `EmbeddedFile`, and returns false where
   krilla returns `None`.
5. A document that a validator does not allow gives an error that wraps
   `okage.ErrPDFValidation`, with the number of the errors. The list of the
   validation errors is not public. A later decision can make it public.
6. Custom XMP metadata that is not valid gives an `*XMPError`, which wraps
   the new sentinel error `okage.ErrPDFXMP` (D21). It holds the case of
   `XmpError` and its name, so that a caller can tell the cases apart.

## The tests

1. The tests of `tests/src/metadata.rs` of krilla, with the seven snapshots
   of `bb320e49` and the snapshots of metadata that hold XMP metadata:
   `metadata_full_with_xmp`, `metadata_version_id`,
   `metadata_version_id_pdf_a1` and `metadata_pdf_20_author`. The unit tests
   of `mod.rs`, `custom.rs`, `extension_schema.rs` and `predefined.rs`.
2. The tests of `tests/src/embed.rs` with their snapshots, except
   `embedded_file_pdf_a2`, which needs a tag tree.
3. A file with a validator of PDF/A holds the ICC profile of its output
   intent, which krilla compresses. The compressed bytes of okage differ
   (D62), and the instance ID of the XMP metadata hashes them. So these
   snapshots are compared after their streams are decoded, with the
   instance ID replaced.
4. The test module compares the files of `pdf` with the oracle of krilla in
   `test/krilla/xmp_test.go`: the XMP metadata in each version, each
   validator of PDF/A, custom XMP metadata of each type, and embedded files.
   It also compares the errors. The random scenes of
   `interchange_test.go` now turn the XMP metadata on at random. The XMP
   metadata holds the strings of `/ID`, so the comparison replaces them in
   each stream.

## Why

The maintainers want the XMP metadata of krilla, and PDF/A-3 for the
invoices of Factur-X. The code is small next to the rest of the port, and
the oracle of krilla compares it, as it compares the rest of `pdf`.
