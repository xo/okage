// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// This file ports tinycss2 tests/test_tinycss2.py. The JSON files of
// css-parsing-tests are a git submodule of upstream that the checkout lacks.
// In their place, testdata/make.py runs upstream on the cases of
// testdata/*.txt and writes its results, and the tests compare the port with
// them. A case where upstream raises an exception is skipped, and the tests
// below cover such cases on their own.

// testCase is a line of a file of testdata: the JSON of the input, and the
// JSON of the result of upstream.
type testCase struct {
	line  int
	input string
	want  string
}

// readCases reads testdata/<name>.txt.
func readCases(t *testing.T, name string) []testCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var cases []testCase
	s := bufio.NewScanner(f)
	s.Buffer(nil, 1<<20)
	for n := 1; s.Scan(); n++ {
		input, want, ok := strings.Cut(s.Text(), "\t")
		if !ok {
			t.Fatalf("%s.txt:%d: no tab", name, n)
		}
		cases = append(cases, testCase{n, input, want})
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return cases
}

// runCases runs f on the string input of each case of a file, and compares
// the JSON of its result with the result of upstream. known maps the JSON of
// an input to the result of the port, where the port differs from upstream
// on purpose (D50).
func runCases(t *testing.T, name string, f func(string) any, known ...map[string]string) {
	t.Helper()
	for _, c := range readCases(t, name) {
		if strings.HasPrefix(c.want, `["exception", `) {
			continue
		}
		for _, k := range known {
			if want, ok := k[c.input]; ok {
				c.want = want
			}
		}
		input, err := strconv.Unquote(c.input)
		if err != nil {
			t.Fatalf("%s.txt:%d: %v", name, c.line, err)
		}
		if got := toJSON(f(input)); got != c.want {
			t.Errorf("%s.txt:%d: %s\n got: %s\nwant: %s", name, c.line, c.input, got, c.want)
		}
	}
}

// toJSON is to_json of the tests of upstream and json.dumps of Python, with
// ensure_ascii=False.
func toJSON(v any) string {
	var b strings.Builder
	writeJSON(&b, v)
	return b.String()
}

// writeJSON writes v as json.dumps of Python does.
func writeJSON(b *strings.Builder, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		writeJSONString(b, v)
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case float64:
		switch {
		case math.IsInf(v, 1):
			b.WriteString("Infinity")
		case math.IsInf(v, -1):
			b.WriteString("-Infinity")
		case math.IsNaN(v):
			b.WriteString("NaN")
		default:
			b.WriteString(formatFloat(v))
		}
	case []any:
		b.WriteByte('[')
		for i, x := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeJSON(b, x)
		}
		b.WriteByte(']')
	case []Node:
		writeJSON(b, nodesJSON(v))
	case Node:
		writeJSON(b, nodeJSON(v))
	default:
		panic(fmt.Sprintf("toJSON: %T", v))
	}
}

// writeJSONString writes s as json.dumps of Python does.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if c < 0x20 {
				fmt.Fprintf(b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
}

// nodesJSON is to_json of a list of nodes.
func nodesJSON(nodes []Node) []any {
	if nodes == nil {
		return nil
	}
	items := make([]any, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, nodeJSON(n))
	}
	return items
}

// numeric is numeric of the tests of upstream.
func numeric(repr string, value float64, isInteger bool) []any {
	kind := "number"
	if isInteger {
		kind = "integer"
	}
	return []any{repr, value, kind}
}

// nodeJSON is to_json of a node.
func nodeJSON(n Node) any {
	switch t := n.(type) {
	case nil:
		return nil
	case *ParseError:
		return []any{"error", t.Kind}
	case *Comment:
		return "/* … */"
	case *WhitespaceToken:
		return " "
	case *LiteralToken:
		return t.Value
	case *IdentToken:
		return []any{"ident", t.Value}
	case *AtKeywordToken:
		return []any{"at-keyword", t.Value}
	case *HashToken:
		kind := "unrestricted"
		if t.IsIdentifier {
			kind = "id"
		}
		return []any{"hash", t.Value, kind}
	case *StringToken:
		return []any{"string", t.Value}
	case *URLToken:
		return []any{"url", t.Value}
	case *NumberToken:
		return append([]any{"number"}, numeric(t.Representation, t.Value, t.IsInteger)...)
	case *PercentageToken:
		return append([]any{"percentage"}, numeric(t.Representation, t.Value, t.IsInteger)...)
	case *DimensionToken:
		return append(append([]any{"dimension"}, numeric(t.Representation, t.Value, t.IsInteger)...), t.Unit)
	case *UnicodeRangeToken:
		return []any{"unicode-range", t.Start, t.End}
	case *CurlyBracketsBlock:
		return append([]any{"{}"}, nodesJSON(t.Content)...)
	case *SquareBracketsBlock:
		return append([]any{"[]"}, nodesJSON(t.Content)...)
	case *ParenthesesBlock:
		return append([]any{"()"}, nodesJSON(t.Content)...)
	case *FunctionBlock:
		return append([]any{"function", t.Name}, nodesJSON(t.Arguments)...)
	case *Declaration:
		return []any{"declaration", t.Name, nodesJSON(t.Value), t.Important}
	case *AtRule:
		var content any
		if t.Content != nil {
			content = nodesJSON(t.Content)
		}
		return []any{"at-rule", t.AtKeyword, nodesJSON(t.Prelude), content}
	case *QualifiedRule:
		return []any{"qualified rule", nodesJSON(t.Prelude), nodesJSON(t.Content)}
	}
	panic(fmt.Sprintf("nodeJSON: %T", n))
}

// positions is positions of testdata/make.py: the type, the line and the
// column of each node, with the lists of its children.
func positions(nodes []Node) []any {
	result := []any{}
	for _, n := range nodes {
		line, column := n.Source()
		item := []any{n.Type(), line, column}
		switch t := n.(type) {
		case *ParenthesesBlock:
			item = append(item, positions(t.Content))
		case *SquareBracketsBlock:
			item = append(item, positions(t.Content))
		case *CurlyBracketsBlock:
			item = append(item, positions(t.Content))
		case *FunctionBlock:
			item = append(item, positions(t.Arguments))
		case *QualifiedRule:
			item = append(item, positions(t.Content), positions(t.Prelude))
		case *AtRule:
			if t.Content != nil {
				item = append(item, positions(t.Content))
			}
			item = append(item, positions(t.Prelude))
		case *Declaration:
			item = append(item, positions(t.Value))
		}
		result = append(result, item)
	}
	return result
}

var skip = []Option{WithSkipComments(true), WithSkipWhitespace(true)}

func TestComponentValueList(t *testing.T) {
	runCases(t, "component_value_list", func(s string) any {
		return ParseComponentValueList(s, WithSkipComments(true))
	})
	runCases(t, "component_value_list_comments", func(s string) any {
		return ParseComponentValueList(s)
	})
	runCases(t, "positions", func(s string) any {
		return positions(ParseComponentValueList(s))
	})
}

func TestOneComponentValue(t *testing.T) {
	runCases(t, "one_component_value", func(s string) any {
		return ParseOneComponentValue(s, WithSkipComments(true))
	})
}

func TestDeclarationList(t *testing.T) {
	runCases(t, "declaration_list", func(s string) any {
		return ParseDeclarationList(s, skip...)
	})
}

func TestBlocksContents(t *testing.T) {
	runCases(t, "blocks_contents", func(s string) any {
		return ParseBlocksContents(s, skip...)
	})
	runCases(t, "blocks_contents_positions", func(s string) any {
		return positions(ParseBlocksContents(s))
	})
}

func TestOneDeclaration(t *testing.T) {
	runCases(t, "one_declaration", func(s string) any {
		return ParseOneDeclaration(s, WithSkipComments(true))
	})
}

func TestStylesheet(t *testing.T) {
	runCases(t, "stylesheet", func(s string) any {
		return ParseStylesheet(s, skip...)
	})
	runCases(t, "stylesheet_positions", func(s string) any {
		return positions(ParseStylesheet(s))
	})
}

func TestRuleList(t *testing.T) {
	runCases(t, "rule_list", func(s string) any {
		return ParseRuleList(s, skip...)
	})
}

func TestOneRule(t *testing.T) {
	runCases(t, "one_rule", func(s string) any {
		return ParseOneRule(s, WithSkipComments(true))
	})
}

func TestNth(t *testing.T) {
	nthJSON := func(a, b int64, ok bool) any {
		if !ok {
			return nil
		}
		return []any{a, b}
	}
	// An integer out of the range of int64 saturates.
	saturated := map[string]string{
		`"99999999999999999999n"`:  `[9223372036854775807, 0]`,
		`"n+99999999999999999999"`: `[1, 9223372036854775807]`,
	}
	runCases(t, "nth", func(s string) any {
		return nthJSON(ParseNth(s))
	}, saturated)
	runCases(t, "nth_round_trip", func(s string) any {
		a, b, ok := ParseNth(s)
		if !ok {
			return nil
		}
		return nthJSON(ParseNth(SerializeNth(a, b)))
	}, saturated)
}

func TestSerializeNth(t *testing.T) {
	tests := []struct {
		a, b int64
		want string
	}{
		{0, 0, "0"}, {0, 1, "1"}, {0, -2, "-2"}, {1, 0, "n"}, {1, 3, "n+3"}, {1, -3, "n-3"},
		{-1, 0, "-n"}, {-1, 3, "-n+3"}, {-1, -3, "-n-3"}, {2, 0, "2n"}, {2, 1, "2n+1"},
		{2, -4, "2n-4"}, {-2, 0, "-2n"}, {-2, 1, "-2n+1"}, {-2, -4, "-2n-4"},
	}
	for _, test := range tests {
		if got := SerializeNth(test.a, test.b); got != test.want {
			t.Errorf("SerializeNth(%d, %d) = %q, want %q", test.a, test.b, got, test.want)
		}
		if a, b, ok := ParseNth(test.want); !ok || a != test.a || b != test.b {
			t.Errorf("ParseNth(%q) = %d, %d, %v", test.want, a, b, ok)
		}
	}
}

func TestNthInvalidDoesNotCrash(t *testing.T) {
	// Truncated or invalid An+B fragments must return false, not panic.
	for _, invalid := range []string{"+", "+/**/", "n+", "n +", "-n-", "2n +"} {
		if _, _, ok := ParseNth(invalid); ok {
			t.Errorf("ParseNth(%q) is valid", invalid)
		}
	}
}

func TestNthLeadingPlusWhitespaceStillInvalid(t *testing.T) {
	if a, b, ok := ParseNth("+n"); !ok || a != 1 || b != 0 {
		t.Errorf("ParseNth(+n) = %d, %d, %v", a, b, ok)
	}
	if _, _, ok := ParseNth("+ n"); ok {
		t.Error("ParseNth(+ n) is valid")
	}
}

func TestSerialize(t *testing.T) {
	runCases(t, "serialize", func(s string) any {
		return Serialize(ParseComponentValueList(s))
	})
	runCases(t, "serialize_round_trip", func(s string) any {
		parsed := ParseComponentValueList(s, WithSkipComments(true))
		return ParseComponentValueList(Serialize(parsed), WithSkipComments(true))
	})
	runCases(t, "serialize_rules", func(s string) any {
		return Serialize(ParseRuleList(s))
	})
	runCases(t, "serialize_blocks", func(s string) any {
		return Serialize(ParseBlocksContents(s))
	})
}

// number is _number of the tests of upstream.
func number(value float64) string {
	value += 0.0000001
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return formatFloat(value)
	}
	value, _ = strconv.ParseFloat(strconv.FormatFloat(value, 'f', 6, 64), 64)
	if value == math.Trunc(value) {
		if value == 0 {
			return "0"
		}
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	return formatFloat(value)
}

// buildColor is build of testdata/make.py.
func buildColor(c *Color, currentColor bool, currentName string) any {
	if currentColor {
		return currentName
	}
	if c == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("color(" + c.Space)
	for i, v := range c.Coordinates {
		b.WriteByte(' ')
		if c.IsMissing(i) {
			b.WriteString("none")
		} else {
			b.WriteString(number(v))
		}
	}
	b.WriteString(" / " + number(c.Alpha) + ")")
	return b.String()
}

// buildRGBA is build of testdata/make.py for an RGBA.
func buildRGBA(c *RGBA, currentColor bool) any {
	if currentColor {
		return "currentColor"
	}
	if c == nil {
		return nil
	}
	return "rgba(" + number(c.Red) + " " + number(c.Green) + " " + number(c.Blue) + " " + number(c.Alpha) + ")"
}

// colorToken is the parse of a string in parse_color of upstream.
func colorToken(s string) Node {
	return ParseOneComponentValue(s, WithSkipComments(true))
}

func TestColor(t *testing.T) {
	runCases(t, "color3", func(s string) any {
		return buildRGBA(ParseColor3(colorToken(s)))
	})
	runCases(t, "color3_tokens", func(s string) any {
		return buildRGBA(ParseColor3(ParseOneComponentValue(s)))
	})
	runCases(t, "color4", func(s string) any {
		c, cur := ParseColor4(colorToken(s))
		return buildColor(c, cur, "currentcolor")
	})
	runCases(t, "color5", func(s string) any {
		c, cur := ParseColor5(colorToken(s))
		return buildColor(c, cur, "currentcolor")
	})
	runCases(t, "color5_dark", func(s string) any {
		c, cur := ParseColor5(colorToken(s), "dark")
		return buildColor(c, cur, "currentcolor")
	})
	runCases(t, "color4_to", func(s string) any {
		c, _ := ParseColor4(colorToken(s))
		if c == nil {
			return nil
		}
		var result []any
		for _, space := range []string{"srgb", "xyz", "xyz-d50", "xyz-d65", "lab"} {
			if to, ok := c.To(space); ok {
				result = append(result, buildColor(to, false, ""))
			} else {
				result = append(result, nil)
			}
		}
		return result
	})
}

func TestColorExceptions(t *testing.T) {
	// Upstream raises an exception for these values. The port returns no
	// color (D50).
	for _, s := range []string{"color()", "color(--custom 1 2 3 / none)", "device-cmyk(0 0 0 1 / 2 3)", "device-cmyk(none 0 0 0)"} {
		if c, cur := ParseColor5(colorToken(s)); c != nil || cur {
			t.Errorf("ParseColor5(%q) = %v, %v", s, c, cur)
		}
	}
	if c, cur := ParseColor4(colorToken("color()")); c != nil || cur {
		t.Errorf("ParseColor4(color()) = %v, %v", c, cur)
	}
}

func TestColorCurrentColor(t *testing.T) {
	for _, value := range []string{"currentcolor", "currentColor", "CURRENTCOLOR"} {
		if c, cur := ParseColor3(colorToken(value)); c != nil || !cur {
			t.Errorf("ParseColor3(%q) = %v, %v", value, c, cur)
		}
		if c, cur := ParseColor4(colorToken(value)); c != nil || !cur {
			t.Errorf("ParseColor4(%q) = %v, %v", value, c, cur)
		}
		if c, cur := ParseColor5(colorToken(value)); c != nil || !cur {
			t.Errorf("ParseColor5(%q) = %v, %v", value, c, cur)
		}
	}
}

func TestStylesheetBytes(t *testing.T) {
	for _, c := range readCases(t, "stylesheet_bytes") {
		kwargs := parseJSONObject(t, c.input)
		cssBytes := make([]byte, 0, len(kwargs["css_bytes"]))
		for _, r := range kwargs["css_bytes"] {
			cssBytes = append(cssBytes, byte(r))
		}
		var env *Encoding
		if label := kwargs["environment_encoding"]; label != "" {
			env = LookupEncoding(label)
		}
		rules, encoding := ParseStylesheetBytes(cssBytes, kwargs["protocol_encoding"], env, skip...)
		if got := toJSON([]any{nodesJSON(rules), encoding.Name}); got != c.want {
			t.Errorf("stylesheet_bytes.txt:%d: %s\n got: %s\nwant: %s", c.line, c.input, got, c.want)
		}
	}
}

// parseJSONObject parses a JSON object whose values are strings.
func parseJSONObject(t *testing.T, s string) map[string]string {
	t.Helper()
	m := map[string]string{}
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		t.Fatalf("not an object: %s", s)
	}
	s = s[1 : len(s)-1]
	readString := func() string {
		s = strings.TrimLeft(s, " ,:")
		if !strings.HasPrefix(s, `"`) {
			t.Fatalf("not a string: %s", s)
		}
		i := 1
		for s[i] != '"' {
			if s[i] == '\\' {
				i++
			}
			i++
		}
		v, err := strconv.Unquote(s[:i+1])
		if err != nil {
			t.Fatal(err)
		}
		s = s[i+1:]
		return v
	}
	for strings.TrimLeft(s, " ,") != "" {
		k := readString()
		m[k] = readString()
	}
	return m
}

func TestSkip(t *testing.T) {
	source := `
    /* foo */
    @media print {
        #foo {
            width: /* bar*/4px;
            color: green;
        }
    }
    `
	noWS := ParseStylesheet(source, WithSkipWhitespace(true))
	noComment := ParseStylesheet(source, WithSkipComments(true))
	def := ParseComponentValueList(source)
	if Serialize(noWS) == source {
		t.Error("serialize(no_ws) == source")
	}
	if Serialize(noComment) == source {
		t.Error("serialize(no_comment) == source")
	}
	if Serialize(def) != source {
		t.Error("serialize(default) != source")
	}
}

func TestCommentEOF(t *testing.T) {
	if got := Serialize(ParseComponentValueList("/* foo ")); got != "/* foo */" {
		t.Errorf("got %q", got)
	}
}

func TestParseDeclarationValueColor(t *testing.T) {
	source := "color:#369"
	declaration := as[*Declaration](t, ParseOneDeclaration(source))
	if len(declaration.Value) != 1 {
		t.Fatalf("value = %v", declaration.Value)
	}
	valueToken := declaration.Value[0]
	if c, _ := ParseColor3(valueToken); c == nil || *c != (RGBA{.2, .4, .6, 1}) {
		t.Errorf("ParseColor3 = %v", c)
	}
	if c, _ := ParseColor4(valueToken); c == nil || c.Coordinates[0] != .2 || c.Coordinates[1] != .4 ||
		c.Coordinates[2] != .6 || c.Alpha != 1 {
		t.Errorf("ParseColor4 = %v", c)
	}
	if got := declaration.Serialize(); got != source {
		t.Errorf("serialize = %q", got)
	}
}

func TestSerializeRules(t *testing.T) {
	source := `@import "a.css"; foo#bar.baz { color: red } /**/ @media print{}`
	if got := Serialize(ParseRuleList(source)); got != source {
		t.Errorf("got %q", got)
	}
}

func TestSerializeDeclarations(t *testing.T) {
	source := `color: #123; /**/ @top-left {} width:7px !important;`
	if got := Serialize(ParseBlocksContents(source)); got != source {
		t.Errorf("got %q", got)
	}
}

func TestSerializeRulesWithFunctions(t *testing.T) {
	source := `
        foo#bar.baz {
            background: url();
            color: rgb(0, 0, 0);
            width: calc(calc());
            height: calc(calc(calc()));
        }
    `
	if got := Serialize(ParseRuleList(source)); got != source {
		t.Errorf("got %q", got)
	}
}

func TestSerializeEInUnit(t *testing.T) {
	for _, source := range []string{`1\65 2`, `34\45 56a`} {
		want := strings.ReplaceAll(source, `\45`, `\65`)
		if got := Serialize(ParseComponentValueList(source)); got != want {
			t.Errorf("%q: got %q, want %q", source, got, want)
		}
	}
}

func TestBackslashDelim(t *testing.T) {
	source := "\\\nfoo"
	tokens := ParseComponentValueList(source)
	var types []string
	for _, token := range tokens {
		types = append(types, token.Type())
	}
	if strings.Join(types, " ") != "literal whitespace ident" {
		t.Fatalf("types = %v", types)
	}
	if !IsLiteral(tokens[0], `\`) {
		t.Errorf("tokens[0] = %v", tokens[0])
	}
	tokens = append(tokens[:1], tokens[2:]...)
	if got := Serialize(tokens); got != source {
		t.Errorf("got %q", got)
	}
}

func TestEscapeSurrogates(t *testing.T) {
	// Python keeps a lone surrogate. A Go string cannot, so the port gives
	// U+FFFD (D50).
	rule := as[*AtRule](t, ParseRuleList(`@\dca9`)[0])
	if rule.AtKeyword != "�" {
		t.Errorf("at_keyword = %q", rule.AtKeyword)
	}
	declaration := as[*Declaration](t, ParseOneDeclaration(`background:\dca9`))
	if v := as[*IdentToken](t, declaration.Value[0]).Value; v != "�" {
		t.Errorf("value = %q", v)
	}
	dimension := as[*DimensionToken](t, ParseComponentValueList(`0\dddf`)[0])
	if !dimension.IsInteger || dimension.IntValue != 0 || dimension.Unit != "�" {
		t.Errorf("dimension = %v %v %q", dimension.IsInteger, dimension.IntValue, dimension.Unit)
	}
	function := as[*FunctionBlock](t, ParseComponentValueList(`\dddf()`)[0])
	if function.Name != "�" {
		t.Errorf("name = %q", function.Name)
	}
}

func TestMaxDepth(t *testing.T) {
	deep := strings.Repeat("(", 100000) + strings.Repeat(")", 100000) + " a"
	tokens := ParseComponentValueList(deep)
	depth := 0
	for n := tokens[0]; ; depth++ {
		b, ok := n.(*ParenthesesBlock)
		if !ok {
			if e, ok := n.(*ParseError); !ok || e.Kind != "too-deep" {
				t.Fatalf("innermost node = %v", n)
			}
			break
		}
		n = b.Content[0]
	}
	if depth != DefaultMaxDepth {
		t.Errorf("depth = %d, want %d", depth, DefaultMaxDepth)
	}
	if len(tokens) != 3 || as[*IdentToken](t, tokens[2]).Value != "a" {
		t.Errorf("tokens = %v", tokens)
	}
	tokens = ParseComponentValueList("f(g([{a}]))", WithMaxDepth(2))
	if got := toJSON(tokens); got != `[["function", "f", ["function", "g", ["error", "too-deep"]]]]` {
		t.Errorf("got %s", got)
	}
	_ = Serialize(ParseStylesheet(deep))
}

func TestErrorSerialization(t *testing.T) {
	// Upstream raises TypeError here. The port writes nothing for the error.
	if got := Serialize(ParseRuleList("a")); got != "" {
		t.Errorf("got %q", got)
	}
}

// as is a checked type assertion of a node.
func as[T Node](t *testing.T, n Node) T {
	t.Helper()
	v, ok := n.(T)
	if !ok {
		t.Fatalf("node %v is a %T", n, n)
	}
	return v
}
