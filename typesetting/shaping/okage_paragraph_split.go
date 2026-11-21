// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"math"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
)

// okage: this file is okage's own. It splits the runes of a Paragraph from
// the properties of each rune, which Init finds once (D72).

// The index of a paired delimiter fits in runeInfo.delim.
const _ = int16(len(pairedDelims))

// initSplit decides whether p splits its runes with splitRange, and finds
// the properties of each rune that splitRange reads.
//
// p splits its runes with splitRange when the paragraph has runes, when
// the bidi algorithm gives level 0 to each of them, and when the faces do
// not take a script with [FontmapScript.SetScript]. The faces must give the
// same face for a rune each time, as [Paragraph.Init] says, so splitRange
// takes the face of a rune from Init. Init asks the faces for the face of
// each rune that can change the face, as [Segmenter.Split] does.
//
// okage: D72.
func (p *Paragraph) initSplit() {
	_, withScript := p.faces.(FontmapScript)
	p.ownSplit = p.level0 && !withScript && len(p.text) > 0
	if !p.ownSplit {
		return
	}
	if cap(p.info) < len(p.text) {
		p.info = make([]runeInfo, len(p.text))
	}
	p.info = p.info[:len(p.text)]
	// okage: D87. The loop also marks the runes that can change the split,
	// as initNext says.
	p.nextScript = resized(p.nextScript, len(p.text))
	p.nextFace = resized(p.nextFace, len(p.text))
	lastScript := language.Common
	var lastFace *font.Face
	hasFace := false
	for i, r := range p.text {
		info := runeInfo{script: language.LookupScript(r), delim: -1, ignore: ignoreFaceChange(r)}
		strong := info.script.Strong()
		if !strong {
			info.delim = int16(lookupDelimIndex(r))
		}
		if !info.ignore {
			info.face = p.faces.ResolveFace(r)
		}
		p.info[i] = info
		p.nextScript[i], p.nextFace[i] = noChange, noChange
		if info.delim >= 0 || strong && info.script != lastScript {
			p.nextScript[i] = int32(i)
		}
		if strong {
			lastScript = info.script
		}
		if !info.ignore {
			if !hasFace || info.face != lastFace {
				p.nextFace[i] = int32(i)
			}
			lastFace, hasFace = info.face, true
		}
	}
	p.initNext()
	lang := p.input.Language
	if lang == "" {
		lang = "en"
	}
	p.lang, p.langOK = language.NewLangID(lang)
}

// noChange marks a rune that cannot change the split in p.nextScript and
// p.nextFace, before initNext.
//
// okage: D87.
const noChange = int32(math.MaxInt32)

// initNext sets p.nextScript and p.nextFace for each rune to the first
// rune from it that can change the split by script or the split by face,
// or to the end of the text. On entry, they hold the index of each rune
// that can change the split, and noChange for each other rune.
//
// A rune can change the split by script when it is a paired delimiter, or
// when its script is strong and is not the script of the last rune before
// it with a strong script. Each other rune with a strong script has the
// script of the run of splitRange when the run has the script of the last
// strong rune, so it does not change the run, and no other rune does.
//
// A rune can change the split by face when it can change the face, and the
// last rune before it that can change the face has another face. A rune
// that cannot change the face does not change the run while the run has a
// face, and each other rune has the face of the run when the run has the
// face of the last rune that can change the face.
//
// okage: D87.
func (p *Paragraph) initNext() {
	nextScript, nextFace := int32(len(p.text)), int32(len(p.text))
	for i := len(p.text) - 1; i >= 0; i-- {
		nextScript = min(nextScript, p.nextScript[i])
		nextFace = min(nextFace, p.nextFace[i])
		p.nextScript[i], p.nextFace[i] = nextScript, nextFace
	}
}

// resized returns s with length n, and reuses its memory when it can.
func resized(s []int32, n int) []int32 {
	if cap(s) < n {
		return make([]int32, n)
	}
	return s[:n]
}

// splitRange returns the runs that [Segmenter.Split] gives for the runes
// start to end of the paragraph, with Text set to these runes alone. It
// follows the steps of Split, and reads the properties of each rune from
// p.info, so it does not look them up again for each line:
//
//  1. The bidi algorithm gives one run at level 0, since it gives level 0
//     to each rune of the paragraph, and a paragraph separator can only be
//     its last rune.
//  2. splitByScript splits the run by script, with the same paired
//     delimiters.
//  3. enforceLanguages sets the language of each run.
//  4. splitByFace splits each run by face.
//
// The runs are valid until the next call of splitRange. splitRange needs
// p.ownSplit and start < end.
//
// okage: D72.
func (p *Paragraph) splitRange(start, end int) []Input {
	in := p.input
	in.Text, in.RunStart, in.RunEnd = p.text[start:end], 0, end-start
	in.Level = 0

	// The split by script, as splitByScript does it.
	clear(p.scriptRuns)
	p.scriptRuns = p.scriptRuns[:0]
	p.delims = p.delims[:0]
	current := in
	current.Script = language.Common
	// okage: D87. When synced is true, current has the script of the last
	// strong rune before i, so the runes before p.nextScript[i] do not
	// change the runs, and the loop skips them.
	last, known, synced := language.Common, false, false
	for i := start; i < end; i++ {
		if synced {
			if i = int(p.nextScript[i]); i >= end {
				break
			}
		}
		info := &p.info[i]
		rScript := info.script
		if info.delim >= 0 {
			delimIndex := int(info.delim)
			if delimIndex%2 == 0 {
				p.delims = append(p.delims, delimEntry{delimIndex, current.Script})
			} else {
				counterPartIndex := delimIndex - 1
				j := len(p.delims) - 1
				for ; j >= 0; j-- {
					if p.delims[j].index == counterPartIndex {
						rScript = p.delims[j].script
						break
					}
				}
				if j == -1 {
					j = 0
				}
				p.delims = p.delims[:j]
			}
		}
		switch {
		case !rScript.Strong() || rScript == current.Script:
		case current.Script == language.Common:
			for k := range p.delims {
				p.delims[k].script = rScript
			}
			current.Script = rScript
		default:
			if i != start {
				current.RunEnd = i - start
				p.scriptRuns = append(p.scriptRuns, current)
			}
			current.RunStart = i - start
			current.Script = rScript
		}
		if info.script.Strong() {
			last, known = info.script, true
		}
		synced = known && current.Script == last
	}
	current.RunEnd = end - start
	p.scriptRuns = append(p.scriptRuns, current)

	// The languages, as enforceLanguages sets them.
	if p.langOK {
		for k := range p.scriptRuns {
			p.scriptRuns[k].Language = enforceLang(p.lang, p.scriptRuns[k].Script).Language()
		}
	}

	// The split by face, as Segmenter.splitByFace does it.
	clear(p.faceRuns)
	p.faceRuns = p.faceRuns[:0]
	lastRunWithoutFace := -1
	for k, run := range p.scriptRuns {
		l := len(p.faceRuns)
		p.splitRunByFace(run, start, k == len(p.scriptRuns)-1)
		if face := p.faceRuns[l].Face; face != nil {
			if lastRunWithoutFace != -1 {
				for m := lastRunWithoutFace; m < l; m++ {
					p.faceRuns[m].Face = face
				}
				lastRunWithoutFace = -1
			}
		} else if lastRunWithoutFace == -1 {
			lastRunWithoutFace = l
		}
	}
	return p.faceRuns
}

// splitRunByFace splits a run of splitRange by face, as splitByFace does,
// and appends the runs to p.faceRuns. The text of the run starts at the
// rune start of the paragraph.
//
// okage: D72.
func (p *Paragraph) splitRunByFace(input Input, start int, isLast bool) {
	current := input
	// okage: D87. When synced is true, current has the face of the last
	// rune that can change the face, and that face is not nil. So the runes
	// before p.nextFace do not change the runs, and the loop skips them.
	synced := false
	for i := input.RunStart; i < input.RunEnd; i++ {
		if synced {
			if i = int(p.nextFace[start+i]) - start; i >= input.RunEnd {
				break
			}
		}
		info := &p.info[start+i]
		if info.ignore && (current.Face != nil || !isLast || i < input.RunEnd-1) {
			continue
		}
		// After a rune that can change the face, current has its face.
		synced = !info.ignore && info.face != nil
		selectedFace := info.face
		if info.ignore {
			selectedFace = p.faces.ResolveFace(input.Text[i])
		}
		if current.Face == nil {
			current.Face = selectedFace
		}
		if current.Face == selectedFace {
			continue
		}
		if i != input.RunStart {
			current.RunEnd = i
			p.faceRuns = append(p.faceRuns, current)
		}
		current = input
		current.RunStart = i
		current.Face = selectedFace
	}
	current.RunEnd = input.RunEnd
	p.faceRuns = append(p.faceRuns, current)
}
