package oracle

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	flagN      = flag.Int("oracle.n", 2000, "the number of random scenes and paths")
	flagSeed   = flag.Uint64("oracle.seed", 1, "the seed of the random scenes and paths")
	flagReplay = flag.String("oracle.replay", "", "a pattern of saved scenes to replay")
)

// root is the root of the repository, seen from this package.
const root = "../.."

// failDir is the folder where a scene that differs is saved.
const failDir = "../testdata/oracle"

// maxCubicDiffs is the part of the scenes with a cubic curve that can differ
// from the oracle (D26).
const maxCubicDiffs = 0.01

// oraclePath is the path of the oracle program. skipReason is set when the
// oracle cannot be built, and each test skips with it.
var oraclePath, skipReason string

func TestMain(m *testing.M) {
	flag.Parse()
	oraclePath, skipReason = build()
	os.Exit(m.Run())
}

// build builds the oracle with cargo into .cache/oracle in the root of the
// repository. It returns the path of the program, or the reason that it
// cannot be built.
//
// resvg turns on the default features of tiny-skia, and so its SIMD backends,
// which compute some values in another way than the scalar code that okage
// ports (D16). build unpacks tiny-skia 0.12.0 from the cache of cargo into
// .cache/oracle, takes "simd" out of its default features, and patches the
// oracle to use that copy, so that every answer comes from the scalar code.
func build() (string, string) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		return "", "cargo is not installed"
	}
	target, err := filepath.Abs(filepath.Join(root, ".cache", "oracle"))
	if err != nil {
		return "", err.Error()
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err.Error()
	}
	manifest := filepath.Join("rust", "Cargo.toml")
	// The crates must be in the cache of cargo before the copy of
	// tiny-skia can be made. cargo fetch reads the network only when a
	// crate is missing.
	if out, err := exec.CommandContext(context.Background(), cargo, "fetch", "--quiet",
		"--manifest-path", manifest).CombinedOutput(); err != nil {
		return "", fmt.Sprintf("cargo cannot fetch the crates of the oracle: %v\n%s", err, out)
	}
	scalar, err := scalarTinySkia(target)
	if err != nil {
		return "", fmt.Sprintf("making the scalar copy of tiny-skia: %v", err)
	}
	cmd := exec.CommandContext(context.Background(), cargo, "build", "--offline", "--release", "--quiet",
		"--manifest-path", manifest, "--target-dir", target,
		"--config", fmt.Sprintf("patch.crates-io.tiny-skia.path=%q", scalar))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Sprintf("cargo cannot build the oracle: %v\n%s", err, out)
	}
	return filepath.Join(target, "release", "okage-oracle"), ""
}

// scalarTinySkia unpacks the crate tiny-skia 0.12.0 from the cache of cargo
// into dir, takes "simd" out of its default features, and returns the folder
// of the copy.
func scalarTinySkia(dir string) (string, error) {
	home := os.Getenv("CARGO_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(h, ".cargo")
	}
	crates, err := filepath.Glob(filepath.Join(home, "registry", "cache", "*", "tiny-skia-0.12.0.crate"))
	if err != nil || len(crates) == 0 {
		return "", errors.New("tiny-skia-0.12.0.crate is not in the cache of cargo")
	}
	f, err := os.Open(crates[0])
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // the file is only read
	zr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Clean(h.Name)
		if !strings.HasPrefix(name, "tiny-skia-0.12.0"+string(filepath.Separator)) || h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return "", err
		}
		if name == filepath.Join("tiny-skia-0.12.0", "Cargo.toml") {
			s := strings.Replace(string(b), "default = [\n    \"std\",\n    \"simd\",", "default = [\n    \"std\",", 1)
			if s == string(b) {
				return "", errors.New("the default features of tiny-skia are not as expected")
			}
			b = []byte(s)
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "tiny-skia-0.12.0"), nil
}

// ask sends each question to the oracle as one line of JSON and returns the
// answers in the same order.
func ask[Q any](t *testing.T, questions []Q) [][]byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), oraclePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	go func() {
		w := bufio.NewWriter(in)
		enc := json.NewEncoder(w)
		for i := range questions {
			if err := enc.Encode(&questions[i]); err != nil {
				errc <- err
				return
			}
		}
		if err := w.Flush(); err != nil {
			errc <- err
			return
		}
		errc <- in.Close()
	}()
	r := bufio.NewReader(out)
	answers := make([][]byte, len(questions))
	for i := range questions {
		var n uint32
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			t.Fatalf("reading answer %d of the oracle: %v\n%s", i, err, stderr.Bytes())
		}
		answers[i] = make([]byte, n)
		if _, err := io.ReadFull(r, answers[i]); err != nil {
			t.Fatalf("reading answer %d of the oracle: %v\n%s", i, err, stderr.Bytes())
		}
	}
	if err := <-errc; err != nil {
		t.Fatalf("writing to the oracle: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the oracle failed: %v\n%s", err, stderr.Bytes())
	}
	return answers
}

// safely calls f and turns a panic into an error.
func safely(f func() []byte) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("okage panics: %v", r)
		}
	}()
	return f(), nil
}

// firstDiff describes the first byte that differs between got and want.
// A scene writes the pixmap first, so an offset in the pixmap gives the
// pixel and its four bytes.
func firstDiff(got, want []byte, width, height uint32) string {
	if len(got) != len(want) {
		return fmt.Sprintf("okage gives %d bytes, the oracle %d", len(got), len(want))
	}
	count, first := 0, -1
	for i := range got {
		if got[i] != want[i] {
			count++
			if first < 0 {
				first = i
			}
		}
	}
	if width == 0 || first >= int(width*height*4) {
		return fmt.Sprintf("%d bytes differ, the first at offset %d (okage %d, oracle %d)", count, first, got[first], want[first])
	}
	px := first / 4
	return fmt.Sprintf("%d bytes differ, the first in the pixel (%d, %d): okage %v, oracle %v",
		count, px%int(width), px/int(width), got[px*4:px*4+4], want[px*4:px*4+4])
}

// save writes a question as JSON to failDir, so that the flag -oracle.replay
// can run it again. It returns the name of the file.
func save(t *testing.T, name string, v any) string {
	t.Helper()
	if err := os.MkdirAll(failDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(failDir, fmt.Sprintf("%d-%s.json", *flagSeed, name))
	if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// counts holds the result of a comparison: the number of questions and of
// differences, with and without a cubic curve.
type counts struct {
	plain, plainDiffs, cubic, cubicDiffs int

	// cubicFailed holds the questions with a cubic curve that differ. They
	// are saved only if too many differ.
	cubicFailed []failed
}

// failed is a question that differs from the oracle.
type failed struct {
	name string
	v    any
}

// check applies the rule of D26 to the counts: a question without a cubic
// curve must match, and at most 1 percent of the questions with a cubic
// curve can differ.
func (c counts) check(t *testing.T, what string) {
	t.Helper()
	t.Logf("%s: %d without a cubic, %d differ; %d with a cubic, %d differ",
		what, c.plain, c.plainDiffs, c.cubic, c.cubicDiffs)
	if c.plainDiffs > 0 {
		t.Errorf("%d %s without a cubic curve differ from the oracle", c.plainDiffs, what)
	}
	if float64(c.cubicDiffs) > maxCubicDiffs*float64(c.cubic) {
		t.Errorf("%d of %d %s with a cubic curve differ from the oracle, more than 1 percent",
			c.cubicDiffs, c.cubic, what)
		for _, f := range c.cubicFailed {
			t.Logf("saved %s", save(t, f.name, f.v))
		}
	}
}

// compareScenes renders each scene with okage and with the oracle, and
// compares the bytes.
func compareScenes(t *testing.T, scenes []scene) counts {
	t.Helper()
	answers := ask(t, scenes)
	blank := 0
	for _, a := range answers {
		if len(bytes.Trim(a, "\x00")) == 0 {
			blank++
		}
	}
	t.Logf("%d of %d scenes give no pixel that is not zero", blank, len(scenes))
	var c counts
	for i := range scenes {
		sc := &scenes[i]
		cubic := sc.hasCubic()
		if cubic {
			c.cubic++
		} else {
			c.plain++
		}
		got, err := safely(func() []byte { return render(sc) })
		if err == nil && bytes.Equal(got, answers[i]) {
			continue
		}
		var msg string
		if err != nil {
			msg = err.Error()
		} else {
			msg = firstDiff(got, answers[i], sc.Width, sc.Height)
		}
		if cubic {
			// D26 allows a few of these, so the scene is saved only if too
			// many differ.
			c.cubicDiffs++
			c.cubicFailed = append(c.cubicFailed, failed{sc.Name, sc})
			t.Logf("scene %s, with a cubic curve, differs: %s", sc.Name, msg)
		} else {
			c.plainDiffs++
			t.Errorf("scene %s differs (%s): %s", sc.Name, save(t, sc.Name, sc), msg)
		}
	}
	return c
}

// TestRender compares the renderer with the oracle on the structured scenes
// and on random scenes from the seed.
func TestRender(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	scenes := structured()
	g := &gen{r: rand.New(rand.NewPCG(*flagSeed, 2))}
	for i := range *flagN {
		scenes = append(scenes, g.scene(i))
	}
	compareScenes(t, scenes).check(t, "scenes")
}

// TestPath compares the strokes, the dashes and the transforms of the
// package path with the oracle, as the bits of each float32.
func TestPath(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	g := &gen{r: rand.New(rand.NewPCG(*flagSeed, 3))}
	questions := make([]pathQuestion, *flagN)
	for i := range questions {
		questions[i] = g.pathQuestion(i)
	}
	answers := ask(t, questions)
	empty := 0
	for _, a := range answers {
		if len(a) == 0 {
			empty++
		}
	}
	t.Logf("%d of %d questions give no path", empty, len(questions))
	var c counts
	for i := range questions {
		q := &questions[i]
		cubic := q.hasCubic()
		if cubic {
			c.cubic++
		} else {
			c.plain++
		}
		got, err := safely(q.answer)
		if err == nil && bytes.Equal(got, answers[i]) {
			continue
		}
		var msg string
		if err != nil {
			msg = err.Error()
		} else {
			msg = firstDiff(got, answers[i], 0, 0)
		}
		if cubic {
			// D26 allows a few of these, so the scene is saved only if too
			// many differ.
			c.cubicDiffs++
			c.cubicFailed = append(c.cubicFailed, failed{q.Name, q})
			t.Logf("question %s, with a cubic curve, differs: %s", q.Name, msg)
		} else {
			c.plainDiffs++
			t.Errorf("question %s differs (%s): %s", q.Name, save(t, q.Name, q), msg)
		}
	}
	c.check(t, "paths")
}

// TestReplay runs the saved scenes that the flag -oracle.replay names. A
// file whose JSON has the field path_op is a question of the package path,
// and each other file is a scene. Each one must match the oracle.
func TestReplay(t *testing.T) {
	if *flagReplay == "" {
		t.Skip("no -oracle.replay pattern")
	}
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files, err := filepath.Glob(*flagReplay)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var probe struct {
			PathOp string `json:"path_op"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		var got, want []byte
		var width, height uint32
		if probe.PathOp != "" {
			var q pathQuestion
			if err := json.Unmarshal(b, &q); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			want = ask(t, []pathQuestion{q})[0]
			got, err = safely(q.answer)
		} else {
			var sc scene
			if err := json.Unmarshal(b, &sc); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			want = ask(t, []scene{sc})[0]
			got, err = safely(func() []byte { return render(&sc) })
			width, height = sc.Width, sc.Height
		}
		switch {
		case err != nil:
			t.Errorf("%s: %v", file, err)
		case !bytes.Equal(got, want):
			t.Errorf("%s: %s", file, firstDiff(got, want, width, height))
		}
	}
}
