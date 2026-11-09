// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports the tests of resvg src/render.rs.

// TestFilterBBoxOutsideIntRect is filter_bbox_outside_int_rect in resvg
// src/render.rs.
//
// Derived from https://github.com/servo/servo/issues/42258.
func TestFilterBBoxOutsideIntRect(t *testing.T) {
	svg := `<svg filter="url(#f)"><filter id="f" x="2em"><feFlood/></filter><path d="M0 0H1e8V1"/></svg>`
	tree, err := FromStr(svg)
	if err != nil {
		t.Fatal(err)
	}
	pixmap, ok := okage.NewPixmap(1, 1)
	if !ok {
		t.Fatal("NewPixmap(1, 1) failed")
	}

	// Just make sure we don't panic.
	m := pixmap.AsMut()
	Render(
		tree,
		path.IdentityTransform(),
		&m,
	)
}
