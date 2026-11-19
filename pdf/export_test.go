// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

// The tests of the package pdf_test import pdfsvg, which imports pdf, so
// they cannot be in the package pdf. These names give them the helpers of
// the snapshot tests.
var (
	// TextSnapshotPage is textSnapshotPage.
	TextSnapshotPage = textSnapshotPage
	// CheckSnapshotDecoded is checkSnapshotDecoded.
	CheckSnapshotDecoded = checkSnapshotDecoded
	// LoadTestFont is testFont.
	LoadTestFont = testFont
)
