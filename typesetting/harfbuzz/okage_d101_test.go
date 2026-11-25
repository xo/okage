package harfbuzz

import (
	"testing"

	"github.com/xo/okage/typesetting/harfbuzz/internal/usetable"
)

// TestUSECategoryValues makes sure that the values of the categories of
// usetable are the constants of the syllable machine (D101).
func TestUSECategoryValues(t *testing.T) {
	want := map[string]uint8{
		"B": useSM_ex_B, "CGJ": useSM_ex_CGJ, "CMAbv": useSM_ex_CMAbv, "CMBlw": useSM_ex_CMBlw,
		"CS": useSM_ex_CS, "FAbv": useSM_ex_FAbv, "FBlw": useSM_ex_FBlw, "FMAbv": useSM_ex_FMAbv,
		"FMBlw": useSM_ex_FMBlw, "FMPst": useSM_ex_FMPst, "FPst": useSM_ex_FPst, "G": useSM_ex_G,
		"GB": useSM_ex_GB, "H": useSM_ex_H, "HM": useSM_ex_HM, "HN": useSM_ex_HN,
		"HR": useSM_ex_HR, "HVM": useSM_ex_HVM, "IS": useSM_ex_IS, "J": useSM_ex_J,
		"MAbv": useSM_ex_MAbv, "MBlw": useSM_ex_MBlw, "MPre": useSM_ex_MPre, "MPst": useSM_ex_MPst,
		"N": useSM_ex_N, "O": useSM_ex_O, "R": useSM_ex_R, "RK": useSM_ex_RK,
		"SB": useSM_ex_SB, "SE": useSM_ex_SE, "SMAbv": useSM_ex_SMAbv, "SMBlw": useSM_ex_SMBlw,
		"SP": useSM_ex_SP, "SUB": useSM_ex_SUB, "Sk": useSM_ex_Sk, "VAbv": useSM_ex_VAbv,
		"VBlw": useSM_ex_VBlw, "VMAbv": useSM_ex_VMAbv, "VMBlw": useSM_ex_VMBlw, "VMPre": useSM_ex_VMPre,
		"VMPst": useSM_ex_VMPst, "VPre": useSM_ex_VPre, "VPst": useSM_ex_VPst, "WJ": useSM_ex_WJ,
		"ZWNJ": useSM_ex_ZWNJ,
	}
	if len(want) != len(usetable.Categories) {
		t.Errorf("usetable has %d categories, want %d", len(usetable.Categories), len(want))
	}
	for name, v := range want {
		if got, ok := usetable.Categories[name]; !ok || got != v {
			t.Errorf("category %s: got %d (%v), want %d", name, got, ok, v)
		}
	}
}

// TestUSETableGenerator compares the category of each code point in the
// table of ot_shape_use_table.go with the category that the generator
// gives from the files in upstream/unicode-ucd. The test skips when the
// folder is not there (D101).
func TestUSETableGenerator(t *testing.T) {
	dir, err := usetable.FindDir()
	if err != nil {
		t.Skip(err)
	}
	table, err := usetable.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := table.Headers[0], "# IndicSyllabicCategory-18.0.0.txt"; got != want {
		t.Errorf("first header: got %q, want %q", got, want)
	}
	n := 0
	for u := rune(0); u <= 0x10FFFF; u++ {
		name, ok := table.Category[u]
		if !ok {
			name = "O"
		}
		want := usetable.Categories[name]
		if got := useLookup(u); got != want {
			t.Errorf("%U: got %d, want %d (%s)", u, got, want, name)
			if n++; n == 20 {
				t.Fatal("too many differences")
			}
		}
	}
	// The space separators of D97 have the category SPACE, and the
	// no-break space, a consonant placeholder, the category GB.
	for _, u := range []rune{0x0020, 0x1680, 0x2000, 0x200A, 0x202F, 0x205F, 0x3000} {
		if got := useLookup(u); got != useSM_ex_SP {
			t.Errorf("%U: got %d, want SP", u, got)
		}
	}
	if got := useLookup(0x00A0); got != useSM_ex_GB {
		t.Errorf("U+00A0: got %d, want GB", got)
	}
}
