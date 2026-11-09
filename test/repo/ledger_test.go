package repo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// upstream is one line of docs/upstream/upstreams.tsv.
type upstream struct {
	Repository string
	Checkout   string
	Base       string
	Matched    string
}

// upstreams reads docs/upstream/upstreams.tsv.
func upstreams(t *testing.T) []upstream {
	t.Helper()
	rows := readTSV(t, filepath.Join("docs", "upstream", "upstreams.tsv"))
	col := columns(t, rows[0], "repository", "checkout", "base", "matched")
	var out []upstream
	for _, r := range rows[1:] {
		out = append(out, upstream{
			Repository: r[col["repository"]],
			Checkout:   r[col["checkout"]],
			Base:       r[col["base"]],
			Matched:    r[col["matched"]],
		})
	}
	return out
}

// readTSV reads a file of values separated by tabs, with a header line. A
// missing last column reads as an empty value.
func readTSV(t *testing.T, name string) [][]string {
	t.Helper()
	text := strings.TrimRight(read(t, name), "\n")
	var rows [][]string
	for line := range strings.SplitSeq(text, "\n") {
		rows = append(rows, strings.Split(line, "\t"))
	}
	for i, r := range rows {
		if len(r) != len(rows[0]) {
			t.Fatalf("%s:%d has %d columns, want %d", name, i+1, len(r), len(rows[0]))
		}
	}
	return rows
}

// columns returns the index of each named column of a header line.
func columns(t *testing.T, header []string, names ...string) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, n := range names {
		i := slices.Index(header, n)
		if i < 0 {
			t.Fatalf("the header %q has no column %q", header, n)
		}
		out[n] = i
	}
	return out
}

// TestTheLedgersAreComplete holds D12. Each upstream whose checkout exists has
// a ledger with one line for each commit after its base commit, in the order
// of upstream and with no gap. The column matched of an upstream never moves
// past a commit that is pending. An upstream whose checkout is missing is
// skipped.
func TestTheLedgersAreComplete(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is missing")
	}
	for _, u := range upstreams(t) {
		t.Run(u.Repository, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(root, u.Checkout)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				t.Skipf("the checkout %s is missing", u.Checkout)
			}
			out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "rev-list", "--reverse", u.Base+"..origin/HEAD").Output()
			if err != nil {
				out, err = exec.CommandContext(t.Context(), "git", "-C", dir, "rev-list", "--reverse", u.Base+"..origin/main").Output()
			}
			if err != nil {
				t.Fatalf("listing the commits of %s after %s: %v", u.Repository, u.Base, err)
			}
			want := strings.Fields(string(out))
			name := filepath.Join("docs", "upstream", u.Repository+".tsv")
			if _, err := os.Stat(filepath.Join(root, name)); err != nil {
				t.Fatalf("%s is missing. Make the ledger as docs/UPSTREAM.md says", name)
			}
			rows := readTSV(t, name)
			col := columns(t, rows[0], "upstream", "status")
			var got []string
			for i, r := range rows[1:] {
				got = append(got, r[col["upstream"]])
				switch s := r[col["status"]]; s {
				case "ported", "not-applicable", "pending":
				default:
					t.Errorf("%s:%d has the status %q", name, i+2, s)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s does not list the %d commits after the base commit in order. Sort the new commits as docs/UPSTREAM.md says", name, len(want))
			}
			if u.Matched == "" {
				return
			}
			for _, r := range rows[1:] {
				if r[col["status"]] == "pending" {
					t.Errorf("the column matched of %s is %s, past the pending commit %s", u.Repository, u.Matched, r[col["upstream"]])
					break
				}
				if r[col["upstream"]] == u.Matched {
					break
				}
			}
		})
	}
}
