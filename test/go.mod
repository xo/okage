module github.com/xo/okage/test

go 1.27.3

require github.com/xo/okage v0.0.0

// The test module builds against the module in the parent folder, which is
// the code under test (D22).
replace github.com/xo/okage => ../
