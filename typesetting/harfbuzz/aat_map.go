package harfbuzz

import (
	"sort"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// ported from harfbuzz/src/hb-aat-map.cc, hb-att-map.hh Copyright © 2018  Google, Inc. Behdad Esfahbod

type rangeFlags struct {
	flags        GlyphMask
	clusterFirst int
	clusterLast  int // end - 1
}

type aatMap struct {
	chainFlags [][]rangeFlags
}

func (map_ *aatMap) resizeChainFlags(N int) {
	if cap(map_.chainFlags) >= N {
		map_.chainFlags = map_.chainFlags[0:N]
	} else {
		map_.chainFlags = make([][]rangeFlags, N)
	}
}

type aatFeatureRange struct {
	info       aatFeatureInfo
	start, end int
}

type aatFeatureEvent struct {
	index   int
	start   bool
	feature aatFeatureInfo
}

func (a aatFeatureEvent) isLess(b aatFeatureEvent) bool {
	if a.index < b.index {
		return true
	} else if a.index > b.index {
		return false
	} else {
		if !a.start && b.start {
			return true
		} else if a.start && !b.start {
			return false
		}
		return a.feature.isLess(b.feature)
	}
}

type aatFeatureInfo struct {
	type_       aatLayoutFeatureType
	setting     aatLayoutFeatureSelector
	isExclusive bool

	// okage: seq is the order of the request, from 1, as in HarfBuzz
	// 15.2.0. It also tells two requests apart (D97).
	seq int
}

func (fi aatFeatureInfo) key() uint32 {
	return uint32(fi.type_)<<16 | uint32(fi.setting)
}

const selMask = ^aatLayoutFeatureSelector(1)

func (a aatFeatureInfo) isLess(b aatFeatureInfo) bool {
	if a.type_ != b.type_ {
		return a.type_ < b.type_
	}
	if !a.isExclusive && (a.setting&selMask) != (b.setting&selMask) {
		return a.setting < b.setting
	}
	// okage: a later request comes first, so that the merge of the
	// duplicates keeps it, as in HarfBuzz 15.2.0. go-text kept the first
	// request (D97).
	return a.seq > b.seq
}

type aatMapBuilder struct {
	tables *font.Font
	props  SegmentProperties

	features        []aatFeatureRange
	currentFeatures []aatFeatureInfo // sorted by (type_, setting) after compilation
	rangeFirst      int
	rangeLast       int

	// okage: budget is the work that the compilation of the map has left,
	// as in HarfBuzz 15.1.0 (D73).
	budget int64
}

// aatBudgetMap is the work of the compilation of one map of morx, as
// HB_BUDGET_AAT_MAP of hb-limits.hh of HarfBuzz 15.1.0. A chain costs 1,
// and each of its feature records costs 1, for each range of the text with
// its own features. A range that a chain keeps costs 16 more. When the
// budget runs out, the last range that the map compiled goes to the end of
// the text.
//
// okage: the commit 370a0da5a1 of HarfBuzz adds the budget. go-text had
// no limit (D73).
const aatBudgetMap = 1 << 24

func newAatMapBuilder(tables *font.Font, props SegmentProperties) aatMapBuilder {
	return aatMapBuilder{
		tables:     tables,
		props:      props,
		rangeFirst: FeatureGlobalStart,
		rangeLast:  FeatureGlobalEnd,
		budget:     aatBudgetMap,
	}
}

// binary search into `currentFeatures`, comparing type_ and setting only
func (mb *aatMapBuilder) hasFeature(info aatFeatureInfo) bool {
	key := info.key()
	for i, j := 0, len(mb.currentFeatures); i < j; {
		h := i + (j-i)/2
		entry := mb.currentFeatures[h].key()
		if key < entry {
			j = h
		} else if entry < key {
			i = h + 1
		} else {
			return true
		}
	}
	return false
}

// okage: compileMap extends the last range of a chain when the new range
// has the same flags, as the commit 370a0da5a1 of HarfBuzz, in 15.1.0,
// does. The ranges come in order and touch, and the flags of each cluster
// do not change. go-text kept one range for each chain and each range of
// the text, so a font with many chains and a text with many ranges of
// features took memory for the product of the two (D73).
func (mb *aatMapBuilder) compileMap(map_ *aatMap) {
	morx := mb.tables.Morx
	map_.resizeChainFlags(len(morx))
	for i, chain := range morx {
		flags := mb.compileMorxFlag(chain)
		ranges := map_.chainFlags[i]
		if n := len(ranges); n != 0 && ranges[n-1].flags == flags {
			ranges[n-1].clusterLast = mb.rangeLast
			continue
		}
		// A range that the map keeps costs more than a range that it visits.
		mb.budget -= 16
		map_.chainFlags[i] = append(ranges, rangeFlags{
			flags:        flags,
			clusterFirst: mb.rangeFirst,
			clusterLast:  mb.rangeLast,
		})
	}

	// TODO: for now we dont support deprecated mort table
	// mort := mapper.face.table.mort
	// if mort.has_data() {
	// 	mort.compile_flags(mapper, map_)
	// 	return
	// }
}

func (mb *aatMapBuilder) compileMorxFlag(chain font.MorxChain) GlyphMask {
	flags := chain.DefaultFlags
	// okage: the chain and its feature records take from the budget, as in
	// HarfBuzz 15.1.0 (D73).
	mb.budget -= int64(1 + len(chain.Features))

	for _, feature := range chain.Features {
		type_, setting := feature.FeatureType, feature.FeatureSetting

	retry:
		// Check whether this type_/setting pair was requested in the map, and if so, apply its flags.
		// (The search here only looks at the type_ and setting fields of feature_info_t.)
		info := aatFeatureInfo{type_: type_, setting: setting}
		if mb.hasFeature(info) {
			flags &= feature.DisableFlags
			flags |= feature.EnableFlags
		} else if type_ == aatLayoutFeatureTypeLetterCase && setting == aatLayoutFeatureSelectorSmallCaps {
			/* Deprecated. https://github.com/harfbuzz/harfbuzz/issues/1342 */
			type_ = aatLayoutFeatureTypeLowerCase
			setting = aatLayoutFeatureSelectorLowerCaseSmallCaps
			goto retry
		} else if type_ == aatLayoutFeatureTypeLanguageTagType && setting != 0 &&
			// okage: the tag matches the language as in HarfBuzz 15.1.0. go-text
			// matched the language as the start of the tag (D73).
			mb.tables.Ltag.MatchesLanguage(int(setting)-1, mb.props.Language) {
			flags &= feature.DisableFlags
			flags |= feature.EnableFlags
		}
	}
	return flags
}

func (mb *aatMapBuilder) addFeature(feature Feature) {
	feat := mb.tables.Feat
	if len(feat.Names) == 0 {
		return
	}

	if feature.Tag == ot.NewTag('a', 'a', 'l', 't') {
		if fn := feat.GetFeature(aatLayoutFeatureTypeCharacterAlternatives); fn == nil || len(fn.SettingTable) == 0 {
			return
		}
		range_ := aatFeatureRange{
			info: aatFeatureInfo{
				type_:       aatLayoutFeatureTypeCharacterAlternatives,
				setting:     aatLayoutFeatureSelector(feature.Value),
				isExclusive: true,
				seq:         len(mb.features) + 1, // okage: D97
			},
			start: feature.Start,
			end:   feature.End,
		}
		mb.features = append(mb.features, range_)
		return
	}

	mapping := aatLayoutFindFeatureMapping(feature.Tag)
	if mapping == nil {
		return
	}

	featureName := feat.GetFeature(mapping.aatFeatureType)
	if featureName == nil || len(featureName.SettingTable) == 0 {
		/* Special case: compileMorxFlag() will fall back to the deprecated version of
		 * small-caps if necessary, so we need to check for that possibility.
		 * https://github.com/harfbuzz/harfbuzz/issues/2307 */
		if mapping.aatFeatureType == aatLayoutFeatureTypeLowerCase &&
			mapping.selectorToEnable == aatLayoutFeatureSelectorLowerCaseSmallCaps {
			featureName = feat.GetFeature(aatLayoutFeatureTypeLetterCase)
			if featureName == nil || len(featureName.SettingTable) == 0 {
				return
			}
		} else {
			return
		}
	}

	var info aatFeatureInfo
	info.type_ = mapping.aatFeatureType
	if feature.Value != 0 {
		info.setting = mapping.selectorToEnable
	} else {
		info.setting = mapping.selectorToDisable
	}
	info.isExclusive = featureName.IsExclusive()
	info.seq = len(mb.features) + 1 // okage: D97
	mb.features = append(mb.features, aatFeatureRange{
		info:  info,
		start: feature.Start,
		end:   feature.End,
	})
}

func (mb *aatMapBuilder) compile(m *aatMap) {
	// Compute active features per range, and compile each.

	if len(mb.features) == 0 {
		mb.compileMap(m)
		return
	}

	// Sort features by start/end events.
	featureEvents := make([]aatFeatureEvent, 0, 2*len(mb.features)+1)
	for _, feature := range mb.features {
		if feature.start == feature.end {
			continue
		}

		featureEvents = append(featureEvents, aatFeatureEvent{
			index:   feature.start,
			start:   true,
			feature: feature.info,
		}, aatFeatureEvent{
			index:   feature.end,
			start:   false,
			feature: feature.info,
		})
	}
	sort.SliceStable(featureEvents, func(i, j int) bool { return featureEvents[i].isLess(featureEvents[j]) })

	// Add a strategic final event.
	{
		featureEvents = append(featureEvents, aatFeatureEvent{
			index:   -1, /* This value does magic. */
			start:   false,
			feature: aatFeatureInfo{seq: len(mb.features) + 1}, // okage: D97
		})
	}

	// Scan events and save features for each range.
	var activeFeatures []aatFeatureInfo
	lastIndex := 0
	for _, event := range featureEvents {
		if event.index != lastIndex {
			// Save a snapshot of active features and the range.

			// sort features and merge duplicates
			mb.currentFeatures = activeFeatures
			mb.rangeFirst = lastIndex
			mb.rangeLast = event.index - 1
			if len(mb.currentFeatures) != 0 {
				sort.SliceStable(mb.currentFeatures, func(i, j int) bool {
					return mb.currentFeatures[i].isLess(mb.currentFeatures[j])
				})
				j := 0
				for i := 1; i < len(mb.currentFeatures); i++ {
					/* Nonexclusive feature selectors come in even/odd pairs to turn a setting on/off
					* respectively, so we mask out the low-order bit when checking for "duplicates"
					* (selectors referring to the same feature setting) here. */
					if mb.currentFeatures[i].type_ != mb.currentFeatures[j].type_ ||
						(!mb.currentFeatures[i].isExclusive && ((mb.currentFeatures[i].setting & selMask) != (mb.currentFeatures[j].setting & selMask))) {
						j++
						mb.currentFeatures[j] = mb.currentFeatures[i]
					}
				}
				mb.currentFeatures = mb.currentFeatures[:j+1]
			}

			mb.compileMap(m)

			// okage: when the budget runs out, the compilation stops, and
			// the code after the loop extends the last ranges to the end of
			// the text, as in HarfBuzz 15.1.0 (D73).
			mb.budget -= int64(len(activeFeatures))
			if mb.budget < 0 {
				break
			}

			lastIndex = event.index
		}

		if event.start {
			activeFeatures = append(activeFeatures, event.feature)
		} else {
			// okage: the end of a request removes that request, found by
			// its seq, as in HarfBuzz 15.2.0. go-text removed the first
			// request with the same type and setting (D97).
			for i, f := range activeFeatures {
				if f.seq == event.feature.seq {
					// remove the item
					activeFeatures = append(activeFeatures[:i], activeFeatures[i+1:]...)
					break
				}
			}
		}
	}

	for _, chainFlags := range m.chainFlags {
		// With our above setup this value is one less than desired; adjust it.
		chainFlags[len(chainFlags)-1].clusterLast = FeatureGlobalEnd
	}
}
