package texmath_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The two programs of the gate of phase 22 of D78. Each one reads a font
// with a MATH table and prints a constant. The first one reads the font
// from a file, and the second one takes STIX Two Math from
// texmath/stixtwomath. When texmath holds Go files, the first one imports it
// too, so that the gate covers the package that a program imports to lay
// out math.
const (
	programWithoutFont = `package main

import (
	"fmt"
	"os"

	"github.com/xo/okage/typesetting/font"
	%s
)

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	face, err := font.ParseTTF(f)
	if err != nil {
		panic(err)
	}
	fmt.Println(face.MathConstant(font.AxisHeight))
}
`
	programWithFont = `package main

import (
	"fmt"

	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
	%s
)

func main() {
	f, err := stixtwomath.Font()
	if err != nil {
		panic(err)
	}
	fmt.Println(font.NewFace(f).MathConstant(font.AxisHeight))
}
`
)

// texmathImport returns the import of texmath for the programs, or an
// empty line when texmath holds no Go file yet.
func texmathImport(t *testing.T) string {
	t.Helper()
	names, err := filepath.Glob("../../texmath/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Log("texmath holds no Go file yet, so the programs do not import it")
		return ""
	}
	return `_ "github.com/xo/okage/texmath"`
}

// writeProgram writes a module with one program in a new folder, which
// builds against the module in the root of the repository.
func writeProgram(t *testing.T, source string) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mod := "module probe\n\ngo 1.27\n\nrequire github.com/xo/okage v0.0.0\n\nreplace github.com/xo/okage => " + root + "\n"
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "go.sum": sum, "main.go": []byte(source)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// build runs a build command in dir, and returns the bytes of out.
func build(t *testing.T, dir, out string, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, msg)
	}
	data, err := os.ReadFile(filepath.Join(dir, out))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestFontStaysOut holds the gate of phase 22 of D78. A program that does
// not import texmath/stixtwomath does not hold the font, and a program that
// imports it does. The test builds both programs with Go for the host, and
// with TinyGo for wasip1 when TinyGo is installed, and logs their sizes.
func TestFontStaysOut(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the builds take some time")
	}
	font, err := os.ReadFile("../../texmath/stixtwomath/stixtwomath.otf")
	if err != nil {
		t.Fatal(err)
	}
	// A part of the outlines of the font, from its middle. A binary that
	// holds the font holds these bytes.
	probe := font[len(font)/2 : len(font)/2+64]
	imp := texmathImport(t)
	without := writeProgram(t, strings.Replace(programWithoutFont, "%s", imp, 1))
	with := writeProgram(t, strings.Replace(programWithFont, "%s", imp, 1))

	type builder struct {
		name string
		tool string
		out  string
		args []string
	}
	builders := []builder{{"go", "go", "probe", []string{"build", "-o", "probe", "."}}}
	if tinygo, err := exec.LookPath("tinygo"); err == nil {
		builders = append(builders, builder{"tinygo wasip1", tinygo, "probe.wasm",
			[]string{"build", "-target=wasip1", "-o", "probe.wasm", "."}})
	} else {
		t.Log("tinygo is missing, so the test builds with Go only")
	}
	for _, b := range builders {
		a := build(t, without, b.out, b.tool, b.args...)
		c := build(t, with, b.out, b.tool, b.args...)
		if bytes.Contains(a, probe) {
			t.Errorf("%s: the program without texmath/stixtwomath holds the font", b.name)
		}
		if !bytes.Contains(c, probe) {
			t.Errorf("%s: the program with texmath/stixtwomath does not hold the font", b.name)
		}
		t.Logf("%s: %d bytes without texmath/stixtwomath, %d bytes with it, %d bytes more", b.name, len(a), len(c), len(c)-len(a))
	}
}
