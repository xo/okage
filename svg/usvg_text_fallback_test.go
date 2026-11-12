package svg

import (
	"slices"
	"testing"
)

// TestIsEmojiSequence checks the test for an emoji sequence of D40.
func TestIsEmojiSequence(t *testing.T) {
	for _, c := range []struct {
		text string
		want bool
	}{
		{"\u00A9\uFE0F", true},
		{"1\uFE0F\u20E3", true},
		{"\U0001F1EF\U0001F1F5", true},
		{"\U0001F44D\U0001F3FD", true},
		{"\U0001F468\u200D\U0001F469\u200D\U0001F467", true},
		{"\U0001F3F4\U000E0067\U000E0062\U000E007F", true},
		{"\U0001F600", false},
		{"\u00A9", false},
		{"e\u0301", false},
		{"\u0915\u094D\u200D\u0937", false},
	} {
		if got := isEmojiSequence(c.text); got != c.want {
			t.Errorf("isEmojiSequence(%+q) = %v, want %v", c.text, got, c.want)
		}
	}
}

// TestReplaceMissingClustersRanks checks that a segment takes the glyphs of
// the fallback font only when their rank is better (D39, D40).
func TestReplaceMissingClustersRanks(t *testing.T) {
	// The text is "a©️b": a at 0, © at 1, U+FE0F at 3 and b at 6.
	g := func(idx int, id GlyphID) glyph { return glyph{byteIdx: newByteIndex(idx), id: id} }
	own := []glyph{g(0, 68), g(1, 107), g(1, 3), g(6, 69)}
	ownRanks := []fallbackRank{rankDrawn, rankNotEmoji, rankNotEmoji, rankDrawn}
	fallback := []glyph{g(0, 0), g(1, 37), g(6, 0)}
	fallbackRanks := []fallbackRank{rankMissing, rankDrawn, rankMissing}

	glyphs, ranks := replaceMissingClusters(own, ownRanks, fallback, fallbackRanks)
	var ids []GlyphID
	for _, gl := range glyphs {
		ids = append(ids, gl.id)
	}
	if want := []GlyphID{68, 37, 69}; !slices.Equal(ids, want) {
		t.Errorf("got the glyphs %v, want %v", ids, want)
	}
	if want := []fallbackRank{rankDrawn, rankDrawn, rankDrawn}; !slices.Equal(ranks, want) {
		t.Errorf("got the ranks %v, want %v", ranks, want)
	}

	// A fallback font that also draws © without color does not replace it.
	fallbackRanks = []fallbackRank{rankMissing, rankNotEmoji, rankMissing}
	glyphs, _ = replaceMissingClusters(own, ownRanks, fallback, fallbackRanks)
	if len(glyphs) != len(own) || glyphs[1].id != 107 {
		t.Errorf("a glyph that is not better replaced the ©: %v", glyphs)
	}
}
