package krilla

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// root is the root of the repository, seen from this package.
const root = "../.."

// oraclePath is the path of the oracle program. skipReason is set when the
// oracle cannot be built, and each test of the oracle skips with it.
var oraclePath, skipReason string

func TestMain(m *testing.M) {
	oraclePath, skipReason = build()
	os.Exit(m.Run())
}

// build builds the oracle with cargo into .cache/krilla in the root of the
// repository, as test/oracle builds its oracle into .cache/oracle. It
// returns the path of the program, or the reason that it cannot be built.
//
// resvg, which krilla-svg calls for the filters of SVG, turns on the SIMD
// code of tiny-skia. build patches the oracle to use a copy of tiny-skia
// 0.12.0 without the feature "simd", as test/oracle does, so that each
// raster comes from the scalar code that okage ports (D16).
func build() (string, string) {
	cargo, err := exec.LookPath("cargo")
	if err != nil {
		return "", "cargo is not installed"
	}
	target, err := filepath.Abs(filepath.Join(root, ".cache", "krilla"))
	if err != nil {
		return "", err.Error()
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err.Error()
	}
	manifest := filepath.Join("rust", "Cargo.toml")
	// cargo fetch reads the network only when a crate is missing from the
	// cache of cargo.
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
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Sprintf("cargo cannot build the oracle: %v\n%s", err, out)
	}
	return filepath.Join(target, "release", "okage-pdf-oracle"), ""
}

// scalarTinySkia unpacks the crate tiny-skia 0.12.0 from the cache of cargo
// into dir, takes "simd" out of its default features, and returns the folder
// of the copy. It is the function of the same name in test/oracle.
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

// needOracle skips the test when the oracle cannot be built.
func needOracle(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip(skipReason)
	}
}

// answer is the answer of the oracle to one question: the bytes of a result,
// or the message of an error.
type answer struct {
	Data []byte
	Err  string
}

// ask sends each question to the oracle as one line of JSON and returns the
// answers in the same order. doc.go describes the questions.
func ask(t *testing.T, questions ...any) []answer {
	t.Helper()
	needOracle(t)
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
		for _, q := range questions {
			if err := enc.Encode(q); err != nil {
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
	answers := make([]answer, len(questions))
	for i := range questions {
		var n uint32
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			t.Fatalf("reading answer %d of the oracle: %v\n%s", i, err, stderr.Bytes())
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(r, b); err != nil {
			t.Fatalf("reading answer %d of the oracle: %v\n%s", i, err, stderr.Bytes())
		}
		switch {
		case n > 0 && b[0] == 0:
			answers[i].Data = b[1:]
		case n > 0 && b[0] == 1:
			answers[i].Err = string(b[1:])
		default:
			t.Fatalf("answer %d of the oracle has no status byte", i)
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

// askPDF asks the oracle one question whose answer is a PDF file, and fails
// the test on an error.
func askPDF(t *testing.T, q any) []byte {
	t.Helper()
	a := ask(t, q)[0]
	if a.Err != "" {
		t.Fatalf("the oracle gives an error: %s", a.Err)
	}
	return a.Data
}
