# D21. Sentinel errors live in the root package

Status: Decided.

okage declares its sentinel errors in the root package. A sentinel error is an
error value that a caller compares with `errors.Is`.

1. Each error that a caller can test for is a sentinel error of the package
   `okage`, such as `okage.ErrInvalidSize`. `path`, `svg` and the internal
   packages return these values, or wrap them with `%w`, and declare none of
   their own.
2. A Rust error enum becomes one sentinel error for each case. The error
   `usvg::Error` has the cases `NotAnUtf8Str`, `MalformedGZip`,
   `ElementsLimitReached`, `InvalidSize` and `ParsingFailed`, so the root
   package declares a sentinel error for each case that okage keeps. A case
   that carries data, such as `ParsingFailed(roxmltree::Error)`, wraps the
   sentinel error with the data, such as the position of the fault.
3. A sentinel error is a constant of a defined string type, as in dbmeta, so
   that no importer can change it:

   ```go
   // Error is an error.
   type Error string

   // Error satisfies the error interface.
   func (err Error) Error() string {
   	return string(err)
   }
   ```

4. An error message is in lower case and starts with a gerund, such as
   `parsing svg: element nesting too deep`, as in the other `xo`
   repositories.
5. Where upstream returns an `Option` and no error, okage returns a `bool` and
   no error, as `docs/UPSTREAM.md` says.
