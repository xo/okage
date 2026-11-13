package lottie

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	for _, test := range []struct {
		name string
		src  string
		exp  bool
	}{
		{"lottie", `{"v":"5.8.1","fr":60,"ip":0,"op":40,"w":512,"h":512,"nm":"x","layers":[]}`, true},
		// The keys can come in any order, after other keys.
		{"reordered", `{"nm":"x","assets":[{"id":"a","layers":[]}],"h":512,"w":512,"op":40,"ip":0,"fr":60}`, true},
		// The start of the document is enough when it holds the keys.
		{"cut after the keys", `{"v":"5.8.1","fr":60,"ip":0,"op":40,"w":512,"h":512,"nm":"x","layers":[{"ind`, true},
		{"package.json", `{"name":"x","version":"1.0.0","scripts":{"build":"tsc"},"dependencies":{}}`, false},
		// Each key must be there.
		{"some of the keys", `{"v":"5.8.1","fr":60,"ip":0,"op":40,"nm":"x","layers":[]}`, false},
		// Only the top level counts.
		{"keys in a layer", `{"name":"x","layers":[{"fr":60,"ip":0,"op":40,"w":512,"h":512}]}`, false},
		// Each value must be a number.
		{"values are not numbers", `{"fr":"60","ip":"0","op":"40","w":"512","h":"512"}`, false},
		{"array", `[{"fr":60,"ip":0,"op":40,"w":512,"h":512}]`, false},
		{"not json", `<svg xmlns="http://www.w3.org/2000/svg"/>`, false},
		{"empty", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Detect([]byte(test.src)); got != test.exp {
				t.Errorf("expected %t, got %t", test.exp, got)
			}
		})
	}
}

// TestDetectTestdata checks that Detect finds each file of testdata from its
// first kilobyte.
func TestDetectTestdata(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no test data: %v", err)
	}
	for _, name := range names {
		buf, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !Detect(buf[:min(len(buf), 1024)]) {
			t.Errorf("%s: not detected", name)
		}
	}
}
