package harfbuzz

import (
	"fmt"
	"math"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// ported from harfbuzz/src/hb-aat-layout.h  Copyright © 2018 Ebrahim Byagowi, Behdad Esfahbod

// The possible feature types defined for AAT shaping,
// from https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html
type aatLayoutFeatureType = uint16

const (
	// Initial, unset feature type
	// aatLayoutFeatureTypeInvalid = 0xFFFF
	// [All Typographic Features](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type0)
	// aatLayoutFeatureTypeAllTypographic = 0
	// [Ligatures](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type1)
	aatLayoutFeatureTypeLigatures = 1
	// [Cursive Connection](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type2)
	// aatLayoutFeatureTypeCurisveConnection = 2
	// [Letter Case](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type3)
	aatLayoutFeatureTypeLetterCase = 3
	// [Vertical Substitution](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type4)
	aatLayoutFeatureTypeVerticalSubstitution = 4
	// [Linguistic Rearrangement](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type5)
	// aatLayoutFeatureTypeLinguisticRearrangement = 5
	// [Number Spacing](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type6)
	aatLayoutFeatureTypeNumberSpacing = 6
	// [Smart Swash](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type8)
	// aatLayoutFeatureTypeSmartSwashType = 8
	// [Diacritics](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type9)
	// aatLayoutFeatureTypeDiacriticsType = 9
	// [Vertical Position](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type10)
	aatLayoutFeatureTypeVerticalPosition = 10
	// [Fractions](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type11)
	aatLayoutFeatureTypeFractions = 11
	// [Overlapping Characters](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type13)
	// aatLayoutFeatureTypeOverlappingCharactersType = 13
	// [Typographic Extras](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type14)
	aatLayoutFeatureTypeTypographicExtras = 14
	// [Mathematical Extras](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type15)
	aatLayoutFeatureTypeMathematicalExtras = 15
	// [Ornament Sets](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type16)
	// aatLayoutFeatureTypeOrnamentSetsType = 16
	// [Character Alternatives](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type17)
	aatLayoutFeatureTypeCharacterAlternatives = 17
	// [Style Options](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type19)
	aatLayoutFeatureTypeStyleOptions = 19
	// [Character Shape](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type20)
	aatLayoutFeatureTypeCharacterShape = 20
	// [Number Case](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type21)
	aatLayoutFeatureTypeNumberCase = 21
	// [Text Spacing](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type22)
	aatLayoutFeatureTypeTextSpacing = 22
	// [Transliteration](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type23)
	aatLayoutFeatureTypeTransliteration = 23

	// [Ruby Kana](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type28)
	aatLayoutFeatureTypeRubyKana = 28

	// [Italic CJK Roman](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type32)
	aatLayoutFeatureTypeItalicCjkRoman = 32
	// [Case Sensitive Layout](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type33)
	aatLayoutFeatureTypeCaseSensitiveLayout = 33
	// [Alternate Kana](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type34)
	aatLayoutFeatureTypeAlternateKana = 34
	// [Stylistic Alternatives](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type35)
	aatLayoutFeatureTypeStylisticAlternatives = 35
	// [Contextual Alternatives](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type36)
	aatLayoutFeatureTypeContextualAlternatives = 36
	// [Lower Case](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type37)
	aatLayoutFeatureTypeLowerCase = 37
	// [Upper Case](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type38)
	aatLayoutFeatureTypeUpperCase = 38
	// [Language Tag](https://developer.apple.com/fonts/TrueType-Reference-Manual/RM09/AppendixF.html#Type39)
	aatLayoutFeatureTypeLanguageTagType = 39
)

// The selectors defined for specifying AAT feature settings.
type aatLayoutFeatureSelector = uint16

const (
	/* Selectors for #aatLayoutFeatureTypeLigatures */
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorRequiredLigaturesOn = 0
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorRequiredLigaturesOff = 1
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorCommonLigaturesOn = 2
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorCommonLigaturesOff = 3
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorRareLigaturesOn = 4
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorRareLigaturesOff = 5

	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorContextualLigaturesOn = 18
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorContextualLigaturesOff = 19
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorHistoricalLigaturesOn = 20
	// for #aatLayoutFeatureTypeLigatures
	aatLayoutFeatureSelectorHistoricalLigaturesOff = 21

	/* Selectors for #aatLayoutFeatureTypeLetterCase */

	// Deprecated
	aatLayoutFeatureSelectorSmallCaps = 3 /* deprecated */

	/* Selectors for #aatLayoutFeatureTypeVerticalSubstitution */
	// for #aatLayoutFeatureTypeVerticalSubstitution
	aatLayoutFeatureSelectorSubstituteVerticalFormsOn = 0
	// for #aatLayoutFeatureTypeVerticalSubstitution
	aatLayoutFeatureSelectorSubstituteVerticalFormsOff = 1

	/* Selectors for #aatLayoutFeatureTypeNumberSpacing */
	// for #aatLayoutFeatureTypeNumberSpacing
	aatLayoutFeatureSelectorMonospacedNumbers = 0
	// for #aatLayoutFeatureTypeNumberSpacing
	aatLayoutFeatureSelectorProportionalNumbers = 1

	/* Selectors for #aatLayoutFeatureTypeVerticalPosition */
	// for #aatLayoutFeatureTypeVerticalPosition
	aatLayoutFeatureSelectorNormalPosition = 0
	// for #aatLayoutFeatureTypeVerticalPosition
	aatLayoutFeatureSelectorSuperiors = 1
	// for #aatLayoutFeatureTypeVerticalPosition
	aatLayoutFeatureSelectorInferiors = 2
	// for #aatLayoutFeatureTypeVerticalPosition
	aatLayoutFeatureSelectorOrdinals = 3
	// for #aatLayoutFeatureTypeVerticalPosition
	aatLayoutFeatureSelectorScientificInferiors = 4

	/* Selectors for #aatLayoutFeatureTypeFractions */
	// for #aatLayoutFeatureTypeFractions
	aatLayoutFeatureSelectorNoFractions = 0
	// for #aatLayoutFeatureTypeFractions
	aatLayoutFeatureSelectorVerticalFractions = 1
	// for #aatLayoutFeatureTypeFractions
	aatLayoutFeatureSelectorDiagonalFractions = 2

	// for #aatLayoutFeatureTypeTypographicExtras
	aatLayoutFeatureSelectorSlashedZeroOn = 4
	// for #aatLayoutFeatureTypeTypographicExtras
	aatLayoutFeatureSelectorSlashedZeroOff = 5

	/* Selectors for #aatLayoutFeatureTypeMathematicalExtras */
	// for #aatLayoutFeatureTypeMathematicalExtras
	aatLayoutFeatureSelectorMathematicalGreekOn = 10
	// for #aatLayoutFeatureTypeMathematicalExtras
	aatLayoutFeatureSelectorMathematicalGreekOff = 11

	/* Selectors for #aatLayoutFeatureTypeStyleOptions */
	// for #aatLayoutFeatureTypeStyleOptions
	aatLayoutFeatureSelectorNoStyleOptions = 0

	// for #aatLayoutFeatureTypeStyleOptions
	aatLayoutFeatureSelectorTitlingCaps = 4

	/* Selectors for #aatLayoutFeatureTypeCharacterShape */
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorTraditionalCharacters = 0
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorSimplifiedCharacters = 1
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorJis1978Characters = 2
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorJis1983Characters = 3
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorJis1990Characters = 4

	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorExpertCharacters = 10
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorJis2004Characters = 11
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorHojoCharacters = 12
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorNlccharacters = 13
	// for #aatLayoutFeatureTypeCharacterShape
	aatLayoutFeatureSelectorTraditionalNamesCharacters = 14

	/* Selectors for #aatLayoutFeatureTypeNumberCase */
	// for #aatLayoutFeatureTypeNumberCase
	aatLayoutFeatureSelectorLowerCaseNumbers = 0
	// for #aatLayoutFeatureTypeNumberCase
	aatLayoutFeatureSelectorUpperCaseNumbers = 1

	/* Selectors for #aatLayoutFeatureTypeTextSpacing */
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorProportionalText = 0
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorMonospacedText = 1
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorHalfWidthText = 2
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorThirdWidthText = 3
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorQuarterWidthText = 4
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorAltProportionalText = 5
	// for #aatLayoutFeatureTypeTextSpacing
	aatLayoutFeatureSelectorAltHalfWidthText = 6

	/* Selectors for #aatLayoutFeatureTypeTransliteration */
	// for #aatLayoutFeatureTypeTransliteration
	aatLayoutFeatureSelectorNoTransliteration = 0
	// for #aatLayoutFeatureTypeTransliteration
	aatLayoutFeatureSelectorHanjaToHangul = 1

	/* Selectors for #aatLayoutFeatureTypeRubyKana */
	// for #aatLayoutFeatureTypeRubyKana
	aatLayoutFeatureSelectorRubyKanaOn = 2
	// for #aatLayoutFeatureTypeRubyKana
	aatLayoutFeatureSelectorRubyKanaOff = 3

	/* Selectors for #aatLayoutFeatureTypeItalicCjkRoman */
	// for #aatLayoutFeatureTypeItalicCjkRoman
	aatLayoutFeatureSelectorCjkItalicRomanOn = 2
	// for #aatLayoutFeatureTypeItalicCjkRoman
	aatLayoutFeatureSelectorCjkItalicRomanOff = 3

	/* Selectors for #aatLayoutFeatureTypeCaseSensitiveLayout */
	// for #aatLayoutFeatureTypeCaseSensitiveLayout
	aatLayoutFeatureSelectorCaseSensitiveLayoutOn = 0
	// for #aatLayoutFeatureTypeCaseSensitiveLayout
	aatLayoutFeatureSelectorCaseSensitiveLayoutOff = 1
	// for #aatLayoutFeatureTypeCaseSensitiveLayout
	aatLayoutFeatureSelectorCaseSensitiveSpacingOn = 2
	// for #aatLayoutFeatureTypeCaseSensitiveLayout
	aatLayoutFeatureSelectorCaseSensitiveSpacingOff = 3

	/* Selectors for #aatLayoutFeatureTypeAlternateKana */
	// for #aatLayoutFeatureTypeAlternateKana
	aatLayoutFeatureSelectorAlternateHorizKanaOn = 0
	// for #aatLayoutFeatureTypeAlternateKana
	aatLayoutFeatureSelectorAlternateHorizKanaOff = 1
	// for #aatLayoutFeatureTypeAlternateKana
	aatLayoutFeatureSelectorAlternateVertKanaOn = 2
	// for #aatLayoutFeatureTypeAlternateKana
	aatLayoutFeatureSelectorAlternateVertKanaOff = 3

	/* Selectors for #aatLayoutFeatureTypeStylisticAlternatives */

	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltOneOn = 2
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltOneOff = 3
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwoOn = 4
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwoOff = 5
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltThreeOn = 6
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltThreeOff = 7
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFourOn = 8
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFourOff = 9
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFiveOn = 10
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFiveOff = 11
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSixOn = 12
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSixOff = 13
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSevenOn = 14
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSevenOff = 15
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltEightOn = 16
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltEightOff = 17
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltNineOn = 18
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltNineOff = 19
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTenOn = 20
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTenOff = 21
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltElevenOn = 22
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltElevenOff = 23
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwelveOn = 24
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwelveOff = 25
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltThirteenOn = 26
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltThirteenOff = 27
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFourteenOn = 28
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFourteenOff = 29
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFifteenOn = 30
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltFifteenOff = 31
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSixteenOn = 32
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSixteenOff = 33
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSeventeenOn = 34
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltSeventeenOff = 35
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltEighteenOn = 36
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltEighteenOff = 37
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltNineteenOn = 38
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltNineteenOff = 39
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwentyOn = 40
	// for #aatLayoutFeatureTypeStylisticAlternatives
	aatLayoutFeatureSelectorStylisticAltTwentyOff = 41

	/* Selectors for #aatLayoutFeatureTypeContextualAlternatives */
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorContextualAlternatesOn = 0
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorContextualAlternatesOff = 1
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorSwashAlternatesOn = 2
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorSwashAlternatesOff = 3
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorContextualSwashAlternatesOn = 4
	// for #aatLayoutFeatureTypeContextualAlternatives
	aatLayoutFeatureSelectorContextualSwashAlternatesOff = 5

	/* Selectors for #aatLayoutFeatureTypeLowerCase */
	// for #aatLayoutFeatureTypeLowerCase
	aatLayoutFeatureSelectorDefaultLowerCase = 0
	// for #aatLayoutFeatureTypeLowerCase
	aatLayoutFeatureSelectorLowerCaseSmallCaps = 1
	// for #aatLayoutFeatureTypeLowerCase
	aatLayoutFeatureSelectorLowerCasePetiteCaps = 2

	/* Selectors for #aatLayoutFeatureTypeUpperCase */
	// for #aatLayoutFeatureTypeUpperCase
	aatLayoutFeatureSelectorDefaultUpperCase = 0
	// for #aatLayoutFeatureTypeUpperCase
	aatLayoutFeatureSelectorUpperCaseSmallCaps = 1
	// for #aatLayoutFeatureTypeUpperCase
	aatLayoutFeatureSelectorUpperCasePetiteCaps = 2
)

// Mapping from OpenType feature tags to AAT feature names and selectors.
//
// Table data courtesy of Apple.  Converted from mnemonics to integers
// when moving to this file.
var featureMappings = [...]aatFeatureMapping{
	{ot.NewTag('a', 'f', 'r', 'c'), aatLayoutFeatureTypeFractions, aatLayoutFeatureSelectorVerticalFractions, aatLayoutFeatureSelectorNoFractions},
	{ot.NewTag('c', '2', 'p', 'c'), aatLayoutFeatureTypeUpperCase, aatLayoutFeatureSelectorUpperCasePetiteCaps, aatLayoutFeatureSelectorDefaultUpperCase},
	{ot.NewTag('c', '2', 's', 'c'), aatLayoutFeatureTypeUpperCase, aatLayoutFeatureSelectorUpperCaseSmallCaps, aatLayoutFeatureSelectorDefaultUpperCase},
	{ot.NewTag('c', 'a', 'l', 't'), aatLayoutFeatureTypeContextualAlternatives, aatLayoutFeatureSelectorContextualAlternatesOn, aatLayoutFeatureSelectorContextualAlternatesOff},
	{ot.NewTag('c', 'a', 's', 'e'), aatLayoutFeatureTypeCaseSensitiveLayout, aatLayoutFeatureSelectorCaseSensitiveLayoutOn, aatLayoutFeatureSelectorCaseSensitiveLayoutOff},
	{ot.NewTag('c', 'l', 'i', 'g'), aatLayoutFeatureTypeLigatures, aatLayoutFeatureSelectorContextualLigaturesOn, aatLayoutFeatureSelectorContextualLigaturesOff},
	{ot.NewTag('c', 'p', 's', 'p'), aatLayoutFeatureTypeCaseSensitiveLayout, aatLayoutFeatureSelectorCaseSensitiveSpacingOn, aatLayoutFeatureSelectorCaseSensitiveSpacingOff},
	{ot.NewTag('c', 's', 'w', 'h'), aatLayoutFeatureTypeContextualAlternatives, aatLayoutFeatureSelectorContextualSwashAlternatesOn, aatLayoutFeatureSelectorContextualSwashAlternatesOff},
	{ot.NewTag('d', 'l', 'i', 'g'), aatLayoutFeatureTypeLigatures, aatLayoutFeatureSelectorRareLigaturesOn, aatLayoutFeatureSelectorRareLigaturesOff},
	{ot.NewTag('e', 'x', 'p', 't'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorExpertCharacters, 16},
	{ot.NewTag('f', 'r', 'a', 'c'), aatLayoutFeatureTypeFractions, aatLayoutFeatureSelectorDiagonalFractions, aatLayoutFeatureSelectorNoFractions},
	{ot.NewTag('f', 'w', 'i', 'd'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorMonospacedText, 7},
	{ot.NewTag('h', 'a', 'l', 't'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorAltHalfWidthText, 7},
	{ot.NewTag('h', 'i', 's', 't'), 40, 0, 1},
	{ot.NewTag('h', 'k', 'n', 'a'), aatLayoutFeatureTypeAlternateKana, aatLayoutFeatureSelectorAlternateHorizKanaOn, aatLayoutFeatureSelectorAlternateHorizKanaOff},
	{ot.NewTag('h', 'l', 'i', 'g'), aatLayoutFeatureTypeLigatures, aatLayoutFeatureSelectorHistoricalLigaturesOn, aatLayoutFeatureSelectorHistoricalLigaturesOff},
	{ot.NewTag('h', 'n', 'g', 'l'), aatLayoutFeatureTypeTransliteration, aatLayoutFeatureSelectorHanjaToHangul, aatLayoutFeatureSelectorNoTransliteration},
	{ot.NewTag('h', 'o', 'j', 'o'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorHojoCharacters, 16},
	{ot.NewTag('h', 'w', 'i', 'd'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorHalfWidthText, 7},
	{ot.NewTag('i', 't', 'a', 'l'), aatLayoutFeatureTypeItalicCjkRoman, aatLayoutFeatureSelectorCjkItalicRomanOn, aatLayoutFeatureSelectorCjkItalicRomanOff},
	{ot.NewTag('j', 'p', '0', '4'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorJis2004Characters, 16},
	{ot.NewTag('j', 'p', '7', '8'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorJis1978Characters, 16},
	{ot.NewTag('j', 'p', '8', '3'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorJis1983Characters, 16},
	{ot.NewTag('j', 'p', '9', '0'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorJis1990Characters, 16},
	{ot.NewTag('l', 'i', 'g', 'a'), aatLayoutFeatureTypeLigatures, aatLayoutFeatureSelectorCommonLigaturesOn, aatLayoutFeatureSelectorCommonLigaturesOff},
	{ot.NewTag('l', 'n', 'u', 'm'), aatLayoutFeatureTypeNumberCase, aatLayoutFeatureSelectorUpperCaseNumbers, 2},
	{ot.NewTag('m', 'g', 'r', 'k'), aatLayoutFeatureTypeMathematicalExtras, aatLayoutFeatureSelectorMathematicalGreekOn, aatLayoutFeatureSelectorMathematicalGreekOff},
	{ot.NewTag('n', 'l', 'c', 'k'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorNlccharacters, 16},
	{ot.NewTag('o', 'n', 'u', 'm'), aatLayoutFeatureTypeNumberCase, aatLayoutFeatureSelectorLowerCaseNumbers, 2},
	{ot.NewTag('o', 'r', 'd', 'n'), aatLayoutFeatureTypeVerticalPosition, aatLayoutFeatureSelectorOrdinals, aatLayoutFeatureSelectorNormalPosition},
	{ot.NewTag('p', 'a', 'l', 't'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorAltProportionalText, 7},
	{ot.NewTag('p', 'c', 'a', 'p'), aatLayoutFeatureTypeLowerCase, aatLayoutFeatureSelectorLowerCasePetiteCaps, aatLayoutFeatureSelectorDefaultLowerCase},
	{ot.NewTag('p', 'k', 'n', 'a'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorProportionalText, 7},
	{ot.NewTag('p', 'n', 'u', 'm'), aatLayoutFeatureTypeNumberSpacing, aatLayoutFeatureSelectorProportionalNumbers, 4},
	{ot.NewTag('p', 'w', 'i', 'd'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorProportionalText, 7},
	{ot.NewTag('q', 'w', 'i', 'd'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorQuarterWidthText, 7},
	{ot.NewTag('r', 'l', 'i', 'g'), aatLayoutFeatureTypeLigatures, aatLayoutFeatureSelectorRequiredLigaturesOn, aatLayoutFeatureSelectorRequiredLigaturesOff},
	{ot.NewTag('r', 'u', 'b', 'y'), aatLayoutFeatureTypeRubyKana, aatLayoutFeatureSelectorRubyKanaOn, aatLayoutFeatureSelectorRubyKanaOff},
	{ot.NewTag('s', 'i', 'n', 'f'), aatLayoutFeatureTypeVerticalPosition, aatLayoutFeatureSelectorScientificInferiors, aatLayoutFeatureSelectorNormalPosition},
	{ot.NewTag('s', 'm', 'c', 'p'), aatLayoutFeatureTypeLowerCase, aatLayoutFeatureSelectorLowerCaseSmallCaps, aatLayoutFeatureSelectorDefaultLowerCase},
	{ot.NewTag('s', 'm', 'p', 'l'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorSimplifiedCharacters, 16},
	{ot.NewTag('s', 's', '0', '1'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltOneOn, aatLayoutFeatureSelectorStylisticAltOneOff},
	{ot.NewTag('s', 's', '0', '2'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltTwoOn, aatLayoutFeatureSelectorStylisticAltTwoOff},
	{ot.NewTag('s', 's', '0', '3'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltThreeOn, aatLayoutFeatureSelectorStylisticAltThreeOff},
	{ot.NewTag('s', 's', '0', '4'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltFourOn, aatLayoutFeatureSelectorStylisticAltFourOff},
	{ot.NewTag('s', 's', '0', '5'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltFiveOn, aatLayoutFeatureSelectorStylisticAltFiveOff},
	{ot.NewTag('s', 's', '0', '6'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltSixOn, aatLayoutFeatureSelectorStylisticAltSixOff},
	{ot.NewTag('s', 's', '0', '7'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltSevenOn, aatLayoutFeatureSelectorStylisticAltSevenOff},
	{ot.NewTag('s', 's', '0', '8'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltEightOn, aatLayoutFeatureSelectorStylisticAltEightOff},
	{ot.NewTag('s', 's', '0', '9'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltNineOn, aatLayoutFeatureSelectorStylisticAltNineOff},
	{ot.NewTag('s', 's', '1', '0'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltTenOn, aatLayoutFeatureSelectorStylisticAltTenOff},
	{ot.NewTag('s', 's', '1', '1'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltElevenOn, aatLayoutFeatureSelectorStylisticAltElevenOff},
	{ot.NewTag('s', 's', '1', '2'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltTwelveOn, aatLayoutFeatureSelectorStylisticAltTwelveOff},
	{ot.NewTag('s', 's', '1', '3'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltThirteenOn, aatLayoutFeatureSelectorStylisticAltThirteenOff},
	{ot.NewTag('s', 's', '1', '4'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltFourteenOn, aatLayoutFeatureSelectorStylisticAltFourteenOff},
	{ot.NewTag('s', 's', '1', '5'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltFifteenOn, aatLayoutFeatureSelectorStylisticAltFifteenOff},
	{ot.NewTag('s', 's', '1', '6'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltSixteenOn, aatLayoutFeatureSelectorStylisticAltSixteenOff},
	{ot.NewTag('s', 's', '1', '7'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltSeventeenOn, aatLayoutFeatureSelectorStylisticAltSeventeenOff},
	{ot.NewTag('s', 's', '1', '8'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltEighteenOn, aatLayoutFeatureSelectorStylisticAltEighteenOff},
	{ot.NewTag('s', 's', '1', '9'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltNineteenOn, aatLayoutFeatureSelectorStylisticAltNineteenOff},
	{ot.NewTag('s', 's', '2', '0'), aatLayoutFeatureTypeStylisticAlternatives, aatLayoutFeatureSelectorStylisticAltTwentyOn, aatLayoutFeatureSelectorStylisticAltTwentyOff},
	{ot.NewTag('s', 'u', 'b', 's'), aatLayoutFeatureTypeVerticalPosition, aatLayoutFeatureSelectorInferiors, aatLayoutFeatureSelectorNormalPosition},
	{ot.NewTag('s', 'u', 'p', 's'), aatLayoutFeatureTypeVerticalPosition, aatLayoutFeatureSelectorSuperiors, aatLayoutFeatureSelectorNormalPosition},
	{ot.NewTag('s', 'w', 's', 'h'), aatLayoutFeatureTypeContextualAlternatives, aatLayoutFeatureSelectorSwashAlternatesOn, aatLayoutFeatureSelectorSwashAlternatesOff},
	{ot.NewTag('t', 'i', 't', 'l'), aatLayoutFeatureTypeStyleOptions, aatLayoutFeatureSelectorTitlingCaps, aatLayoutFeatureSelectorNoStyleOptions},
	{ot.NewTag('t', 'n', 'a', 'm'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorTraditionalNamesCharacters, 16},
	{ot.NewTag('t', 'n', 'u', 'm'), aatLayoutFeatureTypeNumberSpacing, aatLayoutFeatureSelectorMonospacedNumbers, 4},
	{ot.NewTag('t', 'r', 'a', 'd'), aatLayoutFeatureTypeCharacterShape, aatLayoutFeatureSelectorTraditionalCharacters, 16},
	{ot.NewTag('t', 'w', 'i', 'd'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorThirdWidthText, 7},
	{ot.NewTag('u', 'n', 'i', 'c'), aatLayoutFeatureTypeLetterCase, 14, 15},
	{ot.NewTag('v', 'a', 'l', 't'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorAltProportionalText, 7},
	{ot.NewTag('v', 'e', 'r', 't'), aatLayoutFeatureTypeVerticalSubstitution, aatLayoutFeatureSelectorSubstituteVerticalFormsOn, aatLayoutFeatureSelectorSubstituteVerticalFormsOff},
	{ot.NewTag('v', 'h', 'a', 'l'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorAltHalfWidthText, 7},
	{ot.NewTag('v', 'k', 'n', 'a'), aatLayoutFeatureTypeAlternateKana, aatLayoutFeatureSelectorAlternateVertKanaOn, aatLayoutFeatureSelectorAlternateVertKanaOff},
	{ot.NewTag('v', 'p', 'a', 'l'), aatLayoutFeatureTypeTextSpacing, aatLayoutFeatureSelectorAltProportionalText, 7},
	{ot.NewTag('v', 'r', 't', '2'), aatLayoutFeatureTypeVerticalSubstitution, aatLayoutFeatureSelectorSubstituteVerticalFormsOn, aatLayoutFeatureSelectorSubstituteVerticalFormsOff},
	{ot.NewTag('v', 'r', 't', 'r'), aatLayoutFeatureTypeVerticalSubstitution, 2, 3},
	{ot.NewTag('z', 'e', 'r', 'o'), aatLayoutFeatureTypeTypographicExtras, aatLayoutFeatureSelectorSlashedZeroOn, aatLayoutFeatureSelectorSlashedZeroOff},
}

const deletedGlyph = 0xFFFF

type aatClassCache = otLayoutMappingCache

/* Note: This context is used for kerning, even without AAT, hence the condition. */

type aatApplyContext struct {
	plan      *otShapePlan
	font      *Font
	face      Face
	buffer    *Buffer
	gdef      tables.GDEF
	ankrTable tables.Ankr

	rangeFlags    []rangeFlags
	subtableFlags GlyphMask

	bufferIsReversed bool
	// Caches
	usingBufferGlyphSet bool
	bufferGlyphSet      intSet // runes or glyphs

	firstSet          intSet        // readonly
	secondSet         intSet        // readonly
	machineClassCache aatClassCache // readonly
}

func newAatApplyContext(plan *otShapePlan, font *Font, buffer *Buffer) *aatApplyContext {
	var out aatApplyContext
	out.plan = plan
	out.font = font
	out.face = font.face
	out.buffer = buffer
	return &out
}

// chargeBufferPass takes the length of the buffer in operations for a pass
// of a subtable over the buffer, as charge_buffer_pass of HarfBuzz 15.1.0.
// The font gives the number of subtables (D73).
func (c *aatApplyContext) chargeBufferPass() bool {
	return c.buffer.spendOps(len(c.buffer.Info))
}

func (c *aatApplyContext) reverseBuffer() {
	c.buffer.Reverse()
	c.bufferIsReversed = !c.bufferIsReversed
}

func (c *aatApplyContext) setupBufferGlyphSet() {
	c.usingBufferGlyphSet = len(c.buffer.Info) >= 4
	if !c.usingBufferGlyphSet {
		return
	}
	c.buffer.collectGlyphs(&c.bufferGlyphSet)
}

func (c *aatApplyContext) bufferIntersectsMachine() bool {
	if c.usingBufferGlyphSet {
		return c.bufferGlyphSet.Intersects(c.firstSet)
	}

	// Faster for shorter buffers.
	for _, info := range c.buffer.Info {
		if c.firstSet.hasGlyph(info.Glyph) {
			return true
		}
	}
	return false
}

// okage: outputGlyphs, replaceGlyph and deleteGlyph return false when the
// arrays of the buffer cannot grow, as in HarfBuzz 15.1.0 (D73).
func (c *aatApplyContext) outputGlyphs(glyphs []GID) bool {
	if c.usingBufferGlyphSet {
		c.bufferGlyphSet.addGlyphs(glyphs)
	}
	for _, glyph := range glyphs {
		atEndOfText := c.buffer.idx == len(c.buffer.Info)
		if atEndOfText && !c.buffer.outputGlyphIndex(glyph) {
			return false
		}
		var info *GlyphInfo
		if atEndOfText {
			info = c.buffer.prev()
		} else {
			info = c.buffer.cur(0)
		}
		if glyph == deletedGlyph {
			c.buffer.scratchFlags |= bsfAatHasDeleted
			info.setAatDeleted()
		} else {
			if c.gdef.GlyphClassDef != nil {
				info.glyphProps = c.gdef.GlyphProps(gID(glyph))
			}
		}
		if !atEndOfText && !c.buffer.outputGlyphIndex(glyph) {
			return false
		}
	}
	return true
}

func (c *aatApplyContext) replaceGlyph(glyph GID) bool {
	if glyph == deletedGlyph {
		c.buffer.scratchFlags |= bsfAatHasDeleted
		c.buffer.cur(0).setAatDeleted()
	}

	if c.usingBufferGlyphSet {
		c.bufferGlyphSet.addGlyph(glyph)
	}
	if c.gdef.GlyphClassDef != nil {
		c.buffer.cur(0).glyphProps = c.gdef.GlyphProps(gID(glyph))
	}
	return c.buffer.replaceGlyphIndex(glyph)
}

// okage: deleteGlyph adds the deleted glyph to the glyph set of the buffer,
// as the commit 5a1c0c4660 of HarfBuzz, in 15.1.0, does. go-text did not
// add it, so a later subtable that only the deleted glyph starts did not
// apply to a buffer of four glyphs or more (D73).
func (c *aatApplyContext) deleteGlyph() bool {
	c.buffer.scratchFlags |= bsfAatHasDeleted
	c.buffer.cur(0).setAatDeleted()
	if c.usingBufferGlyphSet {
		c.bufferGlyphSet.addGlyph(deletedGlyph)
	}
	return c.buffer.replaceGlyphIndex(deletedGlyph)
}

func (c *aatApplyContext) replaceGlyphInplace(i int, glyph gID) {
	c.buffer.Info[i].Glyph = GID(glyph)
	if glyph == deletedGlyph {
		c.buffer.scratchFlags |= bsfAatHasDeleted
		c.buffer.Info[i].setAatDeleted()
	}
	if c.usingBufferGlyphSet {
		c.bufferGlyphSet.addGlyph(GID(glyph))
	}
	if c.gdef.GlyphClassDef != nil {
		c.buffer.Info[i].glyphProps = c.gdef.GlyphProps(glyph)
	}
}

func (c *aatApplyContext) hasAnyFlags(flag GlyphMask) bool {
	for _, fl := range c.rangeFlags {
		if fl.flags&flag != 0 {
			return true
		}
	}
	return false
}

// execute the state machine in AAT tables
type stateTableDriver struct {
	machine font.AATStateTable
}

func newStateTableDriver(machine font.AATStateTable, _ Face) stateTableDriver {
	return stateTableDriver{machine: machine}
}

func (s *stateTableDriver) getClass(glyph GID, cache *aatClassCache) uint16 {
	key := uint16(glyph)
	if v, ok := cache.get(key); ok {
		return v
	}
	v := s.machine.GetClass(glyph)
	cache.set(key, v)
	return v
}

// implemented by the subtables
type driverContext interface {
	inPlace() bool
	isActionable(entry tables.AATStateEntry) bool
	transition(buffer *Buffer, s stateTableDriver, entry tables.AATStateEntry)
}

func (s stateTableDriver) drive(c driverContext, ac *aatApplyContext) {
	const (
		stateStartOfText = uint16(0)
		classEndOfText   = uint16(0)
		DontAdvance      = 0x4000
	)
	buffer := ac.buffer
	if !c.inPlace() {
		buffer.clearOutput()
	}

	state := stateStartOfText
	// If there's only one range, we already checked the flag.
	var lastRange int = -1 // index in ac.rangeFlags, or -1
	if len(ac.rangeFlags) > 1 {
		lastRange = 0
	}
	startStateSafeToBreakEot := !c.isActionable(s.machine.GetEntry(stateStartOfText, classEndOfText))

	// okage: the loops stop when the buffer is in error, as in HarfBuzz
	// 15.1.0 (D73).
	for buffer.idx = 0; !buffer.inError; {
		class := classEndOfText
		if buffer.idx < len(buffer.Info) {
			class = s.getClass(buffer.Info[buffer.idx].Glyph, &ac.machineClassCache)
		}
	resume:
		if debugMode {
			fmt.Printf("\t\tState machine - class %d at index %d\n", class, buffer.idx)
		}
		entry := s.machine.GetEntry(state, class)
		nextState := entry.NewState // we only supported extended table

		isNotEpsilonTransition := entry.Flags&DontAdvance == 0
		isNotActionable := !c.isActionable(entry)

		if lastRange != -1 {
			// This block is copied in NoncontextualSubtable::apply. Keep in sync.
			range_ := lastRange
			if buffer.idx < len(buffer.Info) {
				cluster := buffer.cur(0).Cluster
				for cluster < ac.rangeFlags[range_].clusterFirst {
					range_--
				}
				for cluster > ac.rangeFlags[range_].clusterLast {
					range_++
				}

				lastRange = range_
			}
			if !(ac.rangeFlags[range_].flags&ac.subtableFlags != 0) {
				if buffer.idx == len(buffer.Info) {
					break
				}

				state = stateStartOfText
				buffer.nextGlyph()
				continue
			}
		} else {
			// Fast path for when transitioning from start-state to start-state with
			// no action and advancing. Do so as long as the class remains the same.
			// This is common with runs of non-actionable glyphs.

			isNullTransition := state == stateStartOfText &&
				nextState == stateStartOfText &&
				startStateSafeToBreakEot &&
				isNotActionable &&
				isNotEpsilonTransition

			if isNullTransition {
				oldKlass := class
				for class == oldKlass {
					c.transition(buffer, s, entry)

					if buffer.idx == len(buffer.Info) || buffer.inError {
						break
					}

					buffer.nextGlyph()

					class = classEndOfText
					if buffer.idx < len(buffer.Info) {
						class = s.getClass(buffer.Info[buffer.idx].Glyph, &ac.machineClassCache)
					}
				}

				if buffer.idx == len(buffer.Info) || buffer.inError {
					break
				}

				goto resume
			}
		}

		/* Conditions under which it's guaranteed safe-to-break before current glyph:
		 *
		 * 1. There was no action in this transition; and
		 *
		 * 2. If we break before current glyph, the results will be the same. That
		 *    is guaranteed if:
		 *
		 *    2a. We were already in start-of-text state; or
		 *
		 *    2b. We are epsilon-transitioning to start-of-text state; or
		 *
		 *    2c. Starting from start-of-text state seeing current glyph:
		 *
		 *        2c'. There won't be any actions; and
		 *
		 *        2c". We would end up in the same state that we were going to end up
		 *             in now, including whether epsilon-transitioning.
		 *
		 *    and
		 *
		 * 3. If we break before current glyph, there won't be any end-of-text action
		 *    after previous glyph.
		 *
		 * This triples the transitions we need to look up, but is worth returning
		 * granular unsafe-to-break results. See eg.:
		 *
		 *   https://github.com/harfbuzz/harfbuzz/issues/2860
		 */

		wouldbeEntry := s.machine.GetEntry(stateStartOfText, class)
		isSafeToBreak := /* 1. */ !c.isActionable(entry) &&
			/* 2. */
			(
			/* 2a. */
			state == stateStartOfText ||
				/* 2b. */
				((entry.Flags&DontAdvance != 0) && nextState == stateStartOfText) ||
				/* 2c. */
				(
				/* 2c'. */
				!c.isActionable(wouldbeEntry) &&
					/* 2c". */
					(nextState == wouldbeEntry.NewState) &&
					(entry.Flags&DontAdvance) == (wouldbeEntry.Flags&DontAdvance))) &&
			/* 3. */
			!c.isActionable(s.machine.GetEntry(state, classEndOfText))

		if !isSafeToBreak && buffer.backtrackLen() != 0 && buffer.idx < len(buffer.Info) {
			buffer.unsafeToBreakFromOutbuffer(buffer.backtrackLen()-1, buffer.idx+1)
		}

		c.transition(buffer, s, entry)

		state = nextState

		if debugMode {
			fmt.Printf("\t\tState machine - new state %d\n", state)
		}

		if buffer.idx == len(buffer.Info) {
			break
		}

		if isNotEpsilonTransition {
			buffer.nextGlyph()
		} else {
			if buffer.maxOps <= 0 {
				buffer.maxOps--
				buffer.nextGlyph()
			} else {
				buffer.maxOps--
			}
		}
	}

	if !c.inPlace() {
		buffer.swapBuffers()
	}
}

///////

type aatFeatureMapping struct {
	otFeatureTag      font.Tag
	aatFeatureType    aatLayoutFeatureType
	selectorToEnable  aatLayoutFeatureSelector
	selectorToDisable aatLayoutFeatureSelector
}

// FaatLayoutFindFeatureMapping fetches the AAT feature-and-selector combination that corresponds
// to a given OpenType feature tag, or `nil` if not found.
func aatLayoutFindFeatureMapping(tag font.Tag) *aatFeatureMapping {
	low, high := 0, len(featureMappings)
	for low < high {
		mid := low + (high-low)/2 // avoid overflow when computing mid
		p := featureMappings[mid].otFeatureTag
		if tag < p {
			high = mid
		} else if tag > p {
			low = mid + 1
		} else {
			return &featureMappings[mid]
		}
	}
	return nil
}

func (sp *otShapePlan) aatLayoutSubstitute(font *Font, buffer *Buffer, features []Feature) {
	var map_ aatMap
	if len(features) != 0 {
		builder := newAatMapBuilder(font.face.Font, sp.props)
		for _, feature := range features {
			builder.addFeature(feature)
		}
		builder.compile(&map_)
	} else {
		map_ = sp.aatMap
	}

	morx := font.face.Morx
	c := newAatApplyContext(sp, font, buffer)
	// okage: morx does nothing when the buffer is in error, and stops after
	// a chain that puts it in error, as in HarfBuzz 15.1.0 (D73).
	if c.buffer.inError {
		return
	}
	c.buffer.unsafeToConcat(0, maxInt)
	c.setupBufferGlyphSet()
	for i, chain := range morx {
		accel := font.morxAccels[i]
		c.rangeFlags = map_.chainFlags[i]
		c.applyMorx(chain, accel)
		if c.buffer.inError {
			return
		}
	}
	// NOTE: we dont support obsolete 'mort' table
}

const bsfAatHasDeleted = bsfShaper0

func aatLayoutRemoveDeletedGlyphs(buffer *Buffer) {
	if buffer.scratchFlags&bsfAatHasDeleted != 0 {
		buffer.deleteGlyphsInplace(func(info *GlyphInfo) bool { return info.isAatDeleted() })
	}
}

func (sp *otShapePlan) aatLayoutPosition(font *Font, buffer *Buffer) {
	kerx := font.face.Kerx

	c := newAatApplyContext(sp, font, buffer)
	c.ankrTable = font.face.Ankr
	c.setupBufferGlyphSet()
	c.applyKernx(kerx, font.kerxAccels)
}

func collectLookupGlyphs(dst *intSet, src tables.AatLookupMixed) {
	for _, r := range src.Coverage() {
		dst.addRange(uint32(r[0]), uint32(r[1]))
	}
}

// aatActionable is the part of a driverContext that tells whether an
// entry has an action.
type aatActionable interface {
	isActionable(entry tables.AATStateEntry) bool
}

// aatActionInitiable is the flag of an entry that starts an action: Push
// in kerx and kern, Mark in format 4 of kerx, MarkFirst in a
// rearrangement subtable, SetMark in a contextual or an insertion
// subtable, and SetComponent in a ligature subtable.
const aatActionInitiable = 0x8000

// aatClassFilter is a set of classes, as the hb_bit_page_t of HarfBuzz,
// which reads a class modulo 512.
type aatClassFilter [8]uint64

func (f *aatClassFilter) add(class uint16) { f[class&511/64] |= 1 << (class & 63) }

func (f *aatClassFilter) has(class uint16) bool { return f[class&511/64]&(1<<(class&63)) != 0 }

// collectInitialGlyphs adds to dst the glyphs of the class table of the
// machine whose class can start an action from the start state. A class
// can start an action when its entry from the start state goes to another
// state, has the flag aatActionInitiable, or has an action. extended is
// false for a state table of kern.
//
// okage: this is collect_initial_glyphs of HarfBuzz 15.1.0. A glyph outside
// the class table is never added, and the deleted glyph is added when its
// class can start an action. When the machine has more than 512 classes,
// HarfBuzz adds each glyph of the class table, without the glyphs of the
// class 1 in kern. go-text added each glyph of the class table (D73).
func collectInitialGlyphs(dst *intSet, machine *font.AATStateTable, table aatActionable, extended bool) {
	var filter aatClassFilter
	nClasses := machine.NClasses()
	if nClasses > 512 {
		for i := range filter {
			filter[i] = ^uint64(0)
		}
		if !extended {
			filter[0] &^= 1 << 1
		}
		collectLookupFiltered(dst, machine.Class, &filter)
		return
	}
	for i := range nClasses {
		entry := machine.GetEntry(0, uint16(i))
		if entry.NewState == 0 && entry.Flags&aatActionInitiable == 0 && !table.isActionable(entry) {
			continue
		}
		filter.add(uint16(i))
	}
	if filter.has(2) { // the class of the deleted glyph
		dst.addGlyph(deletedGlyph)
	}
	collectLookupFiltered(dst, machine.Class, &filter)
}

// collectLookupFiltered adds to dst the glyphs of the lookup whose value is
// in the filter, as collect_glyphs_filtered of the lookups of HarfBuzz
// 15.1.0. A glyph is a uint32 here, as in HarfBuzz, so that a glyph after
// 0xFFFF is not one of the first glyphs.
//
// okage: collectInitialGlyphs calls it (D73).
func collectLookupFiltered(dst *intSet, lookup tables.AATLookup, filter *aatClassFilter) {
	addValues := func(first tables.GlyphID, values []uint16) {
		if len(values) == 0 || first == deletedGlyph {
			return
		}
		for i, v := range values {
			if filter.has(v) {
				dst.add(uint32(first) + uint32(i))
			}
		}
	}
	switch lookup := lookup.(type) {
	case tables.AATLoopkup0:
		addValues(0, lookup.Values)
	case tables.AATLoopkup2:
		// HarfBuzz adds the segments one by one while they are sorted and
		// disjoint, and then adds the rest as a single range.
		var next uint32
		i := 0
		for ; i < len(lookup.Records); i++ {
			r := lookup.Records[i]
			if uint32(r.FirstGlyph) < next {
				break
			}
			if r.FirstGlyph == deletedGlyph || r.FirstGlyph > r.LastGlyph || !filter.has(r.Value) {
				continue
			}
			dst.addRange(uint32(r.FirstGlyph), uint32(r.LastGlyph))
			next = uint32(r.LastGlyph) + 1
		}
		first, last := uint32(math.MaxUint32), uint32(0)
		for ; i < len(lookup.Records); i++ {
			r := lookup.Records[i]
			if r.FirstGlyph == deletedGlyph || r.FirstGlyph > r.LastGlyph || !filter.has(r.Value) {
				continue
			}
			first = min(first, uint32(r.FirstGlyph))
			last = max(last, uint32(r.LastGlyph))
		}
		if first <= last {
			dst.addRange(first, last)
		}
	case tables.AATLoopkup4:
		for _, r := range lookup.Records {
			addValues(r.FirstGlyph, r.Values)
		}
	case tables.AATLoopkup6:
		for _, r := range lookup.Records {
			if r.Glyph != deletedGlyph && filter.has(r.Value) {
				dst.addGlyph(GID(r.Glyph))
			}
		}
	case tables.AATLoopkup8:
		addValues(lookup.FirstGlyph, lookup.Values)
	case tables.AATLoopkup10:
		addValues(lookup.FirstGlyph, lookup.Values)
	}
}

func (sp *otShapePlan) aatLayoutTrack(font *Font, buffer *Buffer) {
	c := newAatApplyContext(sp, font, buffer)
	c.applyTrak(0)
}
