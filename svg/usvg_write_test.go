// Copyright 2023 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"os"
	"sync"
	"testing"
)

// This file ports usvg tests/write.rs. The files are in testdata/usvg, a
// copy of usvg tests/files.

// writeFontDB is GLOBAL_FONTDB in usvg tests/write.rs.
//
// Port: upstream loads the fonts of resvg with load_fonts_dir, in the order
// of the folder. LoadFontsFS loads them sorted by name.
var writeFontDB = sync.OnceValue(func() *FontDatabase {
	fontdb := NewFontDatabase()
	fontdb.LoadFontsFS(os.DirFS("testdata/fonts"), ".")
	fontdb.SetSerifFamily("Noto Serif")
	fontdb.SetSansSerifFamily("Noto Sans")
	fontdb.SetCursiveFamily("Yellowtail")
	fontdb.SetFantasyFamily("Sedgwick Ave Display")
	fontdb.SetMonospaceFamily("Noto Mono")
	return fontdb
})

// resave is resave_impl in usvg tests/write.rs. An empty idPrefix is None.
func resave(t *testing.T, name, idPrefix string, preserveText bool) {
	t.Helper()

	inputSvg, err := os.ReadFile("testdata/usvg/" + name + ".svg")
	if err != nil {
		t.Fatal(err)
	}

	tree, err := FromStr(string(inputSvg), WithFontDatabase(writeFontDB()))
	if err != nil {
		t.Fatal(err)
	}
	opts := []WriteOption{
		WithPreserveText(preserveText),
		WithCoordinatesPrecision(4), // Reduce noise and file size.
		WithTransformsPrecision(4),
	}
	if idPrefix != "" {
		opts = append(opts, WithIDPrefix(idPrefix))
	}
	outputSvg := tree.ToString(opts...)

	expectedSvg, err := os.ReadFile("testdata/usvg/" + name + "-expected.svg")
	if err != nil {
		t.Fatal(err)
	}

	if outputSvg != string(expectedSvg) {
		t.Errorf("assertion failed: output_svg == expected_svg\n\nExpected:\n%s\n\nActual:\n%s", expectedSvg, outputSvg)
	}
}

// TestWrite runs the tests of usvg tests/write.rs, in their order. Each
// subtest has the name of the Rust test.
func TestWrite(t *testing.T) {
	for _, c := range []struct {
		test, file   string
		idPrefix     string
		preserveText bool
	}{
		{test: "path_simple_case", file: "path-simple-case"},
		{test: "ellipse_simple_case", file: "ellipse-simple-case"},
		{test: "text_simple_case", file: "text-simple-case"},
		{test: "clip_path_with_transform", file: "clip-path-with-transform"},
		{test: "preserve_id_filter", file: "preserve-id-filter"},
		{test: "preserve_id_fe_image", file: "preserve-id-fe-image"},
		{test: "preserve_id_fe_image_with_opacity", file: "preserve-id-fe-image-with-opacity"},
		{test: "generate_filter_id_function_v1", file: "generate-id-filter-function-v1"},
		{test: "generate_filter_id_function_v2", file: "generate-id-filter-function-v2"},
		{test: "filter_id_with_prefix", file: "filter-id-with-prefix", idPrefix: "prefix-"},
		{test: "filter_with_object_units_multi_use", file: "filter-with-object-units-multi-use"},
		{test: "preserve_id_clip_path_v1", file: "preserve-id-clip-path-v1"},
		{test: "preserve_id_clip_path_v2", file: "preserve-id-clip-path-v2"},
		{test: "preserve_id_for_clip_path_in_pattern", file: "preserve-id-for-clip-path-in-pattern"},
		{test: "generate_id_clip_path_for_symbol", file: "generate-id-clip-path-for-symbol"},
		{test: "clip_path_with_text", file: "clip-path-with-text"},
		{test: "clip_path_with_complex_text", file: "clip-path-with-complex-text"},
		{test: "clip_path_with_object_units_multi_use", file: "clip-path-with-object-units-multi-use"},
		{test: "mask_with_object_units_multi_use", file: "mask-with-object-units-multi-use"},
		{test: "text_with_generated_gradients", file: "text-with-generated-gradients"},
		{test: "preserve_text_multiple_font_families", file: "preserve-text-multiple-font-families", preserveText: true},
		{test: "preserve_text_on_path", file: "preserve-text-on-path", preserveText: true},
		{test: "preserve_text_in_clip_path", file: "preserve-text-in-clip-path", preserveText: true},
		{test: "preserve_text_in_mask", file: "preserve-text-in-mask", preserveText: true},
		{test: "preserve_text_in_pattern", file: "preserve-text-in-pattern", preserveText: true},
		{test: "preserve_text_simple_case", file: "preserve-text-simple-case"},
		{test: "preserve_text_with_dx_and_dy", file: "preserve-text-with-dx-and-dy", preserveText: true},
		{test: "preserve_text_with_rotate", file: "preserve-text-with-rotate", preserveText: true},
		{test: "preserve_text_with_complex_text_decoration", file: "preserve-text-with-complex-text-decoration", preserveText: true},
		{test: "preserve_text_with_nested_baseline_shift", file: "preserve-text-with-nested-baseline-shift", preserveText: true},
		{test: "optimize_paths_without_markers", file: "optimize-paths-without-markers"},
	} {
		t.Run(c.test, func(t *testing.T) {
			resave(t, c.file, c.idPrefix, c.preserveText)
		})
	}
}
