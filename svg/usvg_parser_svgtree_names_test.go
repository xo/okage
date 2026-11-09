// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "testing"

// usvg src/parser/svgtree/names.rs has no tests. These tests check that each
// name maps to its value and back.

func TestEIdNames(t *testing.T) {
	if len(eidNames) != 53 || int(EIdUse) != 52 {
		t.Fatalf("EId has %d values, want 53", len(eidNames))
	}
	for id := range EId(len(eidNames)) {
		name := id.String()
		got, ok := eidFromStr(name)
		if !ok || got != id || id.ToStr() != name {
			t.Errorf("eidFromStr(%q) = %v, %v, want %v", name, got, ok, id)
		}
	}
	if EIdLinearGradient.String() != "linearGradient" || EIdFeDropShadow.String() != "feDropShadow" {
		t.Error("EId names are wrong")
	}
	if _, ok := eidFromStr("lineargradient"); ok {
		t.Error("eidFromStr ignored the case")
	}
	if EId(200).String() != "EId(?)" {
		t.Error("EId(200).String() is wrong")
	}
}

func TestAIdNames(t *testing.T) {
	if len(aidNames) != 211 || int(AIdZ) != 210 {
		t.Fatalf("AId has %d values, want 211", len(aidNames))
	}
	for id := range AId(len(aidNames)) {
		name := id.String()
		got, ok := aidFromStr(name)
		if !ok || got != id || id.ToStr() != name {
			t.Errorf("aidFromStr(%q) = %v, %v, want %v", name, got, ok, id)
		}
	}
	if AIdHref.String() != "href" || AIdStrokeDasharray.String() != "stroke-dasharray" || AIdId.String() != "id" {
		t.Errorf("AId names are wrong: %s %s", AIdHref, AIdStrokeDasharray)
	}
	if _, ok := aidFromStr("unknown"); ok {
		t.Error("aidFromStr found an unknown name")
	}
}
