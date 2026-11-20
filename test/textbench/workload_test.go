package textbench_test

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/language"
)

// fontDir is the folder of the test fonts of svg, from this package.
const fontDir = "../../svg/testdata/fonts"

// gutenbergEnv names the environment variable that holds the folder of the
// books. defaultGutenberg is the folder when the variable is not set: the
// cache of ivy, in a checkout of ivy next to okage.
const (
	gutenbergEnv     = "OKAGE_GUTENBERG"
	defaultGutenberg = "../../../ivy/test/review/cache/files/gutenberg"
)

// workload is a text that the benchmarks lay out: its paragraphs, its
// fonts, its language and the size of its lines.
type workload struct {
	paragraphs []string
	runes      [][]rune
	// chars is the number of characters of all the paragraphs.
	chars int
	faces *runeFaces
	lang  language.Language
	dir   di.Direction
	// fontSize and width are the size of the font and the width of a line,
	// in pixels.
	fontSize, width float64
	// collapse is true when the spaces at the end of a line collapse, as
	// with white-space: normal.
	collapse bool
}

// newWorkload returns a workload of the paragraphs, with the fonts of the
// files in fontDir. The first font is the primary font.
func newWorkload(paragraphs []string, lang string, dir di.Direction, fontSize, width float64, fonts ...string) (*workload, error) {
	faces := make([]*font.Face, len(fonts))
	for i, name := range fonts {
		face, err := loadFace(name)
		if err != nil {
			return nil, err
		}
		faces[i] = face
	}
	w := &workload{
		paragraphs: paragraphs,
		faces:      newRuneFaces(faces),
		lang:       language.NewLanguage(lang),
		dir:        dir,
		fontSize:   fontSize,
		width:      width,
	}
	for _, p := range paragraphs {
		r := []rune(p)
		w.runes = append(w.runes, r)
		w.chars += len(r)
	}
	return w, nil
}

// faceCache holds the faces that loadFace parsed, by the name of the file.
var faceCache = map[string]*font.Face{}

// loadFace parses a font of fontDir once.
func loadFace(name string) (*font.Face, error) {
	if face, ok := faceCache[name]; ok {
		return face, nil
	}
	data, err := os.ReadFile(filepath.Join(fontDir, name))
	if err != nil {
		return nil, err
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	faceCache[name] = face
	return face, nil
}

// runeFaces is the shaping.Fontmap of a workload. It gives each character
// the first face that has a glyph for it, or else the fallback face. It
// keeps the face of each character, as the fontmap of ivy does.
type runeFaces struct {
	faces []*font.Face
	// fallback is the face of a character that no face has. It is the
	// first face, unless the workload sets it.
	fallback *font.Face
	cache    map[rune]*font.Face
}

// newRuneFaces returns the fontmap of the faces.
func newRuneFaces(faces []*font.Face) *runeFaces {
	return &runeFaces{faces: faces, fallback: faces[0], cache: map[rune]*font.Face{}}
}

// primary returns the first face.
func (f *runeFaces) primary() *font.Face {
	return f.faces[0]
}

// ResolveFace satisfies shaping.Fontmap.
func (f *runeFaces) ResolveFace(r rune) *font.Face {
	if face, ok := f.cache[r]; ok {
		return face
	}
	face := f.fallback
	for _, c := range f.faces {
		if _, ok := c.NominalGlyph(r); ok {
			face = c
			break
		}
	}
	f.cache[r] = face
	return face
}

// workloadSpec names a workload and loads it.
type workloadSpec struct {
	name string
	load func() (*workload, error)
}

// workloadSpecs are the workloads of each benchmark, in their order.
var workloadSpecs = []workloadSpec{
	{"pg24264", loadPG24264},
	{"pg1342", loadPG1342},
	{"english", func() (*workload, error) {
		return newWorkload(synthetic(englishText, 8), "en", di.DirectionLTR, 16, 624, "NotoSans-Regular.ttf")
	}},
	{"arabic", func() (*workload, error) {
		return newWorkload(synthetic(arabicText, 8), "ar", di.DirectionRTL, 16, 624, "Amiri-Regular.ttf")
	}},
	{"mixed", func() (*workload, error) {
		return newWorkload(synthetic(mixedText, 8), "en", di.DirectionLTR, 16, 624, "NotoSans-Regular.ttf", "Amiri-Regular.ttf")
	}},
	{"devanagari", func() (*workload, error) {
		return newWorkload(synthetic(devanagariText, 8), "hi", di.DirectionLTR, 16, 624, "NotoSansDevanagari-Regular.ttf", "NotoSans-Regular.ttf")
	}},
	{"emoji", func() (*workload, error) {
		return newWorkload(synthetic(emojiText, 8), "en", di.DirectionLTR, 16, 624, "NotoSans-Regular.ttf", "TwitterColorEmoji.subset.ttf")
	}},
	{"variable", loadVariable},
}

// loaded holds the workloads that workloadSpec.get loaded, and the error of
// each.
var loaded = map[string]struct {
	w   *workload
	err error
}{}

// get returns the workload of spec, which it loads once. It skips the
// benchmark when the file of the workload is missing.
func (spec workloadSpec) get(b *testing.B) *workload {
	b.Helper()
	l, ok := loaded[spec.name]
	if !ok {
		l.w, l.err = spec.load()
		loaded[spec.name] = l
	}
	switch {
	case errors.Is(l.err, fs.ErrNotExist):
		b.Skipf("skipping %s: %v", spec.name, l.err)
	case l.err != nil:
		b.Fatal(l.err)
	}
	return l.w
}

// gutenbergFile returns the path of a file of the folder of the books.
func gutenbergFile(name string) string {
	dir := os.Getenv(gutenbergEnv)
	if dir == "" {
		dir = defaultGutenberg
	}
	return filepath.Join(dir, name)
}

// loadPG24264 loads gutenberg/txt/pg24264.txt as ivy lays out a text file:
// a monospace font of 13px on a page of US Letter with margins of 0.75in.
//
// MPLUS1p is a Japanese font, and it has no glyph for about 9% of the
// characters of this novel, which are traditional Chinese. ivy takes a
// Chinese font that has them all. So MPLUS1p is also the fallback face, and
// these characters do not cut the runs of MPLUS1p into many short runs.
func loadPG24264() (*workload, error) {
	data, err := os.ReadFile(gutenbergFile("txt/pg24264.txt"))
	if err != nil {
		return nil, err
	}
	w, err := newWorkload(textParagraphs(string(data)), "zh", di.DirectionLTR, 13, 672,
		"NotoMono-Regular.ttf", "MPLUS1p-Regular.ttf", "NotoSans-Regular.ttf")
	if err != nil {
		return nil, err
	}
	w.faces.fallback = w.faces.faces[1]
	return w, nil
}

// textParagraphs splits a text file into paragraphs, as the package text of
// ivy does. A blank line ends a paragraph. A line joins the line before it
// with one space, or with no space next to a wide character of Chinese or
// Japanese. A line that starts with white space starts a new paragraph:
// the style keeps its line feed, and a line feed ends a paragraph of the
// layout.
func textParagraphs(text string) []string {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	var out []string
	var b strings.Builder
	last := ""
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for line := range strings.Lines(text) {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		first, _ := utf8.DecodeRuneInString(line)
		switch {
		case b.Len() == 0:
		case unicode.IsSpace(first):
			flush()
		default:
			before, _ := utf8.DecodeLastRuneInString(last)
			if !wide(before) && !wide(first) {
				b.WriteByte(' ')
			}
		}
		b.WriteString(line)
		last = line
	}
	flush()
	return out
}

// wide reports whether r is a wide character of Chinese or Japanese, as
// the package text of ivy says.
func wide(r rune) bool {
	switch {
	case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo):
		return true
	case r >= 0x3000 && r <= 0x303f, r >= 0xff00 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6:
		return true
	}
	return false
}

// paragraphPattern matches an element p, and tagPattern matches a tag.
var (
	paragraphPattern = regexp.MustCompile(`(?s)<p\b[^>]*>(.*?)</p>`)
	tagPattern       = regexp.MustCompile(`<[^>]*>`)
)

// loadPG1342 loads the text of each element p of
// gutenberg/html/pg1342-images.html, with the white space collapsed, in
// Noto Sans of 16px on a line of 624px.
func loadPG1342() (*workload, error) {
	data, err := os.ReadFile(gutenbergFile("html/pg1342-images.html"))
	if err != nil {
		return nil, err
	}
	var paragraphs []string
	for _, m := range paragraphPattern.FindAllStringSubmatch(string(data), -1) {
		text := html.UnescapeString(tagPattern.ReplaceAllString(m[1], ""))
		if text = strings.Join(strings.Fields(text), " "); text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	w, err := newWorkload(paragraphs, "en", di.DirectionLTR, 16, 624, "NotoSans-Regular.ttf", "NotoSerif-Regular.ttf")
	if err != nil {
		return nil, err
	}
	w.collapse = true
	return w, nil
}

// loadVariable loads the English text in Roboto Flex, at a weight of 700
// and a width of 80.
func loadVariable() (*workload, error) {
	base, err := loadFace("RobotoFlex.subset.ttf")
	if err != nil {
		return nil, err
	}
	face := font.NewFace(base.Font)
	face.SetVariations([]font.Variation{
		{Tag: ot.MustNewTag("wght"), Value: 700},
		{Tag: ot.MustNewTag("wdth"), Value: 80},
	})
	w, err := newWorkload(synthetic(englishText, 8), "en", di.DirectionLTR, 16, 624, "RobotoFlex.subset.ttf")
	if err != nil {
		return nil, err
	}
	w.faces = newRuneFaces([]*font.Face{face})
	return w, nil
}

// syntheticRunes is about the number of characters of each paragraph of a
// synthetic workload.
const syntheticRunes = 1500

// synthetic returns n paragraphs, each of which repeats the sentences of
// text until it has syntheticRunes characters. Paragraph i starts at
// sentence i, so that the paragraphs differ.
func synthetic(text string, n int) []string {
	sentences := strings.SplitAfter(strings.TrimSpace(text), "\n")
	out := make([]string, n)
	for i := range out {
		var b strings.Builder
		for j := i; utf8.RuneCountInString(b.String()) < syntheticRunes; j++ {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimSpace(sentences[j%len(sentences)]))
		}
		out[i] = b.String()
	}
	return out
}

// The synthetic texts hold one sentence on each line. The English text is
// written for these benchmarks. The Arabic and the Hindi texts are
// article 1 and the start of article 2 of the Universal Declaration of
// Human Rights. The emoji are those of TwitterColorEmoji.subset.ttf.
const (
	englishText = `
The river ran slowly past the old mill, and the miller's daughter watched the water turn the wheel.
In the morning she carried bread to the village, where the baker paid her in coins and gossip.
"You will not believe it," he said, "but the schoolmaster bought a telescope from a traveling merchant."
That evening the whole village climbed the hill to look at the moon, the planets and a comet.
Nobody slept until the clouds came, at a quarter past two, and covered the sky like a gray blanket.
The next day the miller grumbled that nobody worked, but he too had stood on the hill all night.
`
	arabicText = `
يولد جميع الناس أحرارًا متساوين في الكرامة والحقوق.
وقد وهبوا عقلاً وضميرًا وعليهم أن يعامل بعضهم بعضًا بروح الإخاء.
لكل إنسان حق التمتع بكافة الحقوق والحريات الواردة في هذا الإعلان، دون أي تمييز.
`
	mixedText = `
The Arabic word سلام means peace, and the greeting السلام عليكم answers it.
In 2024 the library held 1,250 books in English and 340 in العربية, about 27%.
The reader opened page 42 and read: "قال الراوي" (the narrator said) before he went on.
Numbers such as ١٢٣ and 456 keep their order, from left to right, inside a line from right to left.
`
	devanagariText = `
सभी मनुष्यों को गौरव और अधिकारों के मामले में जन्मजात स्वतन्त्रता और समानता प्राप्त है।
उन्हें बुद्धि और अन्तरात्मा की देन प्राप्त है और परस्पर उन्हें भाईचारे के भाव से बर्ताव करना चाहिए।
सभी को इस घोषणा में सन्निहित सभी अधिकारों और स्वतन्त्रताओं को प्राप्त करने का हक़ है।
`
	emojiText = `
Good morning 😀, said the crab 🦀, and the flag 🏳️‍🌈 waved over the beach 😁.
The cook pinched 🤏 the salt, kissed his fingers 🤌 and laughed 😃 at the soup.
A family of crabs 🦀🦀🦀 marched past 🏳️‍🌈🏳️‍🌈 with a grin 😀😁😃 on each face.
`
)
