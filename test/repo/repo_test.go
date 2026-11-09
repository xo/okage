package repo_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// root is the root of the repository, seen from this package.
const root = "../.."

// read returns the content of a file of the repository.
func read(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// documents returns the path of each Markdown document of the repository that
// this repository writes: the four documents of the root and docs/.
func documents(t *testing.T) []string {
	t.Helper()
	docs := []string{"README.md", "AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md"}
	err := fs.WalkDir(os.DirFS(root), "docs", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(name, ".md") {
			docs = append(docs, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

// TestSkillsAreCopies holds dbmeta D89, which D2 adopts. Each skill in
// skills-lock.json is an ordinary folder under .agents/skills and under
// .claude/skills, and the two folders hold the same files.
func TestSkillsAreCopies(t *testing.T) {
	t.Parallel()
	var lock struct {
		Skills map[string]json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal([]byte(read(t, "skills-lock.json")), &lock); err != nil {
		t.Fatalf("reading skills-lock.json: %v", err)
	}
	if len(lock.Skills) == 0 {
		t.Fatal("skills-lock.json names no skill")
	}
	agents := filepath.Join(root, ".agents", "skills")
	claude := filepath.Join(root, ".claude", "skills")
	for _, dir := range []string{agents, claude} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if _, ok := lock.Skills[e.Name()]; !ok {
				t.Errorf("%s is not in skills-lock.json. Add it with the command in CONTRIBUTING.md", filepath.Join(dir, e.Name()))
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(lock.Skills)) {
		a := skillFiles(t, filepath.Join(agents, name))
		c := skillFiles(t, filepath.Join(claude, name))
		if !maps.Equal(a, c) {
			t.Errorf("the skill %s differs between %s and %s", name, agents, claude)
		}
	}
}

// skillFiles returns the content of each file of one copy of a skill, keyed by
// its path in the copy. A symbolic link is an error, because a Windows
// checkout writes one as a text file (dbmeta D89).
func skillFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		t.Errorf("%s is missing or is not a folder. Install the skill with --copy", dir)
		return nil
	}
	out := map[string]string{}
	err = fs.WalkDir(os.DirFS(dir), ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.Type()&fs.ModeSymlink != 0:
			t.Errorf("%s is a symbolic link. See dbmeta D89", filepath.Join(dir, name))
		case d.Type().IsRegular():
			body, err := fs.ReadFile(os.DirFS(dir), name)
			if err != nil {
				return err
			}
			out[name] = string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	return out
}

// TestTheRootHoldsFourDocuments holds dbmeta D110, which D2 adopts. Only
// README.md, AGENTS.md, CLAUDE.md and CONTRIBUTING.md are Markdown files in the
// root of the repository.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{"README.md", "AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md"}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") && !slices.Contains(allowed, e.Name()) {
			t.Errorf("%s is a document in the root. Move it to docs/", e.Name())
		}
	}
}

// TestClaudeImportsAgents holds dbmeta D110, which D2 adopts. CLAUDE.md holds
// one line that imports AGENTS.md.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	if got := read(t, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md holds %q, want %q", got, "@AGENTS.md\n")
	}
}

// linkRE matches a Markdown link with a target.
var linkRE = regexp.MustCompile(`\]\(([^)#\s]+)(#[^)]*)?\)`)

// TestEveryMarkdownLinkResolves makes sure that each relative link in a
// document names a file or a folder that exists.
func TestEveryMarkdownLinkResolves(t *testing.T) {
	t.Parallel()
	for _, doc := range documents(t) {
		for _, m := range linkRE.FindAllStringSubmatch(read(t, doc), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			name := filepath.Join(root, filepath.Dir(doc), target)
			if _, err := os.Stat(name); err != nil {
				t.Errorf("%s links to %s, which does not exist", doc, target)
			}
		}
	}
}

// decisionFileRE matches the name of the file of a decision.
var decisionFileRE = regexp.MustCompile(`^D(\d{3})-[a-z0-9-]+\.md$`)

// decisions returns the number and the file name of each decision.
func decisions(t *testing.T) map[int]string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "docs", "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]string{}
	for _, e := range entries {
		if m := decisionFileRE.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			out[n] = e.Name()
		}
	}
	return out
}

// TestEveryDecisionReferenceExists makes sure that a bare decision number in a
// document names a decision of this repository. A number after the name of
// another repository, such as dbmeta D110, is skipped.
func TestEveryDecisionReferenceExists(t *testing.T) {
	t.Parallel()
	known := decisions(t)
	others := regexp.MustCompile(`(?:dbmeta|transit)(?:\s+D\d+,?)*(?:\s+and)?\s*$`)
	for _, doc := range documents(t) {
		text := read(t, doc)
		for _, loc := range regexp.MustCompile(`\bD(\d+)\b`).FindAllStringSubmatchIndex(text, -1) {
			if loc[0] > 0 && strings.ContainsAny(text[loc[0]-1:loc[0]], "/-_") {
				continue
			}
			if others.MatchString(text[max(0, loc[0]-40):loc[0]]) {
				continue
			}
			n, _ := strconv.Atoi(text[loc[2]:loc[3]])
			if _, ok := known[n]; !ok {
				t.Errorf("%s names D%d, which is not a decision of this repository", doc, n)
			}
		}
	}
}

// TestTheDecisionIndexIsComplete makes sure that docs/decisions/README.md has
// one row for each decision, with its title and its status.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	index := read(t, filepath.Join("docs", "decisions", "README.md"))
	for n, name := range decisions(t) {
		text := read(t, filepath.Join("docs", "decisions", name))
		lines := strings.SplitN(text, "\n", 4)
		if len(lines) < 3 || !strings.HasPrefix(lines[0], "# D"+strconv.Itoa(n)+". ") || !strings.HasPrefix(lines[2], "Status: ") {
			t.Errorf("%s does not open with its title and its status", name)
			continue
		}
		title := strings.TrimPrefix(lines[0], "# D"+strconv.Itoa(n)+". ")
		status := strings.TrimSuffix(strings.TrimPrefix(lines[2], "Status: "), ".")
		row := "| D" + strconv.Itoa(n) + " | [" + title + "](" + name + ") | " + status + " |"
		if !strings.Contains(index, row) {
			t.Errorf("docs/decisions/README.md has no row %q", row)
		}
	}
}

// TestNoPersonIsNamed holds the rule of AGENTS.md that a document names no
// person. The copyright lines of LICENSE are not documents.
func TestNoPersonIsNamed(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`\b(Ken|Kenneth)\b`)
	for _, doc := range documents(t) {
		if re.MatchString(read(t, doc)) {
			t.Errorf("%s names a person. Write \"the maintainers\" or \"a maintainer\"", doc)
		}
	}
}

// TestProseIsSimpleEnglish checks the rules of the simple-english skill that a
// machine can check, in each document: no banned modal, no semicolon, no em
// dash and no contraction. Text in backticks and in code blocks is skipped.
func TestProseIsSimpleEnglish(t *testing.T) {
	t.Parallel()
	fence := regexp.MustCompile("(?s)```.*?```")
	code := regexp.MustCompile("`[^`]*`")
	quoted := regexp.MustCompile(`"[^"]*"`)
	modal := regexp.MustCompile(`\b(should|may|might|could|would)\b`)
	contraction := regexp.MustCompile(`\b\w+n't\b|\b(it|that|there|what)'s\b|\w'(re|ll|ve)\b`)
	for _, doc := range documents(t) {
		if doc == "CLAUDE.md" {
			continue
		}
		text := fence.ReplaceAllString(read(t, doc), "")
		for i, line := range strings.Split(text, "\n") {
			line = quoted.ReplaceAllString(code.ReplaceAllString(line, ""), "")
			switch {
			case strings.HasPrefix(strings.TrimSpace(line), "|"):
				line = strings.ReplaceAll(line, "|", " ")
			case strings.Contains(line, "never  ,  or"):
				continue
			}
			if m := modal.FindString(line); m != "" {
				t.Errorf("%s:%d uses %q. Use can, will or must", doc, i+1, m)
			}
			if strings.Contains(line, ";") {
				t.Errorf("%s:%d holds a semicolon. Write two sentences", doc, i+1)
			}
			if strings.ContainsAny(line, "—–") {
				t.Errorf("%s:%d holds a dash. Write two sentences", doc, i+1)
			}
			if m := contraction.FindString(line); m != "" {
				t.Errorf("%s:%d holds the contraction %q", doc, i+1, m)
			}
		}
	}
}
