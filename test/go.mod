module github.com/xo/okage/test

go 1.27.3

require (
	github.com/cshum/vipsgen/v2 v2.1.2
	github.com/xo/okage v0.0.0
)

require (
	github.com/go-text/typesetting v0.3.5 // indirect
	golang.org/x/image v0.47.0 // indirect
)

// The test module builds against the module in the parent folder, which is
// the code under test (D22).
replace github.com/xo/okage => ../
