// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"log/slog"
	"path"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// This file ports no file of usvg. usvg finds its fonts with the crate fontdb
// 0.24, which okage does not port (D9). The matching of fontdb decides which
// font renders a text, so this file holds a small font database that does the
// same work with the same rules as fontdb 0.24 src/lib.rs: the faces of each
// font in the order of loading, the generic families, the CSS query and its
// matching. The faces are parsed with go-text. Each item names the item of
// fontdb that it replaces.
//
// Port: fontdb finds the fonts of a system in its folders and with
// fontconfig. The database takes its fonts from the caller instead, as bytes
// or as an fs.FS (D9). LoadSystemFonts reads the known font folders of each
// system from an fs.FS, so no build tag selects a platform.

// FontID is ID in fontdb 0.24 src/lib.rs. It is the unique identifier of a
// face in a FontDatabase.
//
// Port: fontdb keeps its faces in a slot map. The database keeps them in a
// slice and never removes one, so a FontID is the index of the face.
type FontID uint32

// FontLanguageEnglishUnitedStates is the language of a family name in English
// (United States), the value of Language::English_UnitedStates in ttf-parser,
// which fontdb uses.
const FontLanguageEnglishUnitedStates uint16 = 0x0409

// FontFamilyName is the pair of a family name and its language in the field
// families of FaceInfo in fontdb 0.24 src/lib.rs.
//
// Port: ttf-parser names a language with the enum Language. Language holds
// the Windows language ID of the name. A Macintosh name in English has
// FontLanguageEnglishUnitedStates, as in ttf-parser, and a name of another
// platform has 0.
type FontFamilyName struct {
	Name     string
	Language uint16
}

// FaceInfo is FaceInfo in fontdb 0.24 src/lib.rs. It is a single font face
// info.
//
// A font can have multiple faces.
//
// A single item of the `Database`.
type FaceInfo struct {
	// ID is an unique ID.
	ID FontID

	// Index is a face index in the source.
	Index uint32

	// Families is a list of family names.
	//
	// Contains pairs of Name + Language. Where the first family is always English US,
	// unless it's missing from the font.
	//
	// Corresponds to a *Typographic Family* (ID 16) or a *Font Family* (ID 1) [name ID]
	// in a TrueType font.
	//
	// This is not an *Extended Typographic Family* or a *Full Name*.
	// Meaning it will contain _Arial_ and not _Arial Bold_.
	Families []FontFamilyName

	// PostScriptName is a PostScript name.
	//
	// Corresponds to a *PostScript name* (6) [name ID] in a TrueType font.
	PostScriptName string

	// Style is a font face style.
	Style FontStyle

	// Weight is a font face weight.
	Weight uint16

	// Stretch is a font face stretch.
	Stretch FontStretch

	// Monospaced indicates that the font face is monospaced.
	Monospaced bool

	// source holds the bytes of the font, which the faces of a collection
	// share. It is the field source of upstream.
	source *fontSource
}

// fontSource is Source in fontdb 0.24 src/lib.rs: Source::Binary, the bytes
// of a font, or Source::File, a font file that is read on first use. It holds
// the faces that go-text parses from the bytes on first use.
//
// Port: fontdb maps a file into memory with the feature memmap. Go reads the
// file of an fs.FS into memory the first time that a face of it is used, so
// that a database of the fonts of a system does not hold every file.
type fontSource struct {
	// fsys and name are the file of Source::File. They are nil and empty for
	// Source::Binary.
	fsys fs.FS
	name string

	dataOnce sync.Once
	data     []byte

	once  sync.Once
	faces []*parsedFace

	// cmaps holds the character map of each face, which cmapOnce reads on
	// first use from the tables cmap and OS/2 alone.
	cmapOnce sync.Once
	cmaps    []font.Cmap
}

// charmap returns the character map of the face of index i, as go-text
// builds it from the tables cmap and OS/2, or false when the face has none.
// It reads only those tables, so that a search for a fallback font does not
// read and parse each font of a system whole.
func (s *fontSource) charmap(i uint32) (font.Cmap, bool) {
	s.cmapOnce.Do(func() {
		var res opentype.Resource = bytes.NewReader(s.data)
		if s.fsys != nil {
			f, err := s.fsys.Open(s.name)
			if err != nil {
				return
			}
			defer func() { _ = f.Close() }()
			r, ok := f.(opentype.Resource)
			if !ok {
				r = bytes.NewReader(s.bytes())
			}
			res = r
		}
		lds, err := opentype.NewLoaders(res)
		if err != nil {
			return
		}
		s.cmaps = make([]font.Cmap, len(lds))
		for j, ld := range lds {
			raw, _ := ld.RawTable(opentype.MustNewTag("OS/2"))
			os2, _, _ := tables.ParseOs2(raw)
			raw, err := ld.RawTable(opentype.MustNewTag("cmap"))
			if err != nil {
				continue
			}
			tb, _, err := tables.ParseCmap(raw)
			if err != nil {
				continue
			}
			if cmap, _, err := font.ProcessCmap(tb, os2.FontPage()); err == nil {
				s.cmaps[j] = cmap
			}
		}
	})
	if int(i) >= len(s.cmaps) || s.cmaps[i] == nil {
		return nil, false
	}
	return s.cmaps[i], true
}

// bytes returns the bytes of the font, read from its file on first use. It
// returns nil when the file cannot be read.
func (s *fontSource) bytes() []byte {
	s.dataOnce.Do(func() {
		if s.fsys == nil {
			return
		}
		data, err := fs.ReadFile(s.fsys, s.name)
		if err == nil {
			s.data = data
		}
	})
	return s.data
}

// parsedFace is a face that go-text parses: the loader of its tables and the
// font. It is what Database::with_face_data in fontdb 0.24 src/lib.rs gives
// to its closure, the data and the index of the face, after the parse that
// each caller of upstream does.
type parsedFace struct {
	ld   *opentype.Loader
	font *font.Font

	// glyf holds the table glyf, which glyfOnce parses on first use. It is
	// nil when the face has none.
	glyfOnce sync.Once
	glyf     tables.Glyf
}

// parsed returns the face of index i, parsed by go-text on first use, or
// false when go-text cannot parse it.
func (s *fontSource) parsed(i uint32) (*parsedFace, bool) {
	s.once.Do(func() {
		lds, err := opentype.NewLoaders(bytes.NewReader(s.bytes()))
		if err != nil {
			return
		}
		s.faces = make([]*parsedFace, len(lds))
		for j, ld := range lds {
			ft, err := font.NewFont(ld)
			if err != nil {
				continue
			}
			s.faces[j] = &parsedFace{ld: ld, font: ft}
		}
	})
	if int(i) >= len(s.faces) || s.faces[i] == nil {
		return nil, false
	}
	return s.faces[i], true
}

// rawTable returns the bytes of the table tag of the face, or nil when the
// face has no such table.
func (f *parsedFace) rawTable(tag string) []byte {
	b, err := f.ld.RawTable(opentype.MustNewTag(tag))
	if err != nil {
		return nil
	}
	return b
}

// FontDatabase is Database in fontdb 0.24 src/lib.rs. It is a font database.
//
// Usvg reads its fonts from it, as `Options::fontdb` (D9). A FontDatabase is
// safe for concurrent use once no font is loaded into it any more.
type FontDatabase struct {
	faces           []*FaceInfo
	familySerif     string
	familySansSerif string
	familyCursive   string
	familyFantasy   string
	familyMonospace string

	// logger takes the warnings of the loading of fonts. Upstream logs them
	// with the crate log (D20).
	logger *slog.Logger
}

// FontDatabaseOption is an option of NewFontDatabase (D20).
type FontDatabaseOption func(*FontDatabase)

// WithFontDatabaseLogger sets the logger that takes the warnings that fontdb
// logs with the crate log, such as a font that fails to load. Without it, the
// database logs nothing (D20).
func WithFontDatabaseLogger(logger *slog.Logger) FontDatabaseOption {
	return func(db *FontDatabase) {
		db.logger = logger
	}
}

// NewFontDatabase is Database::new in fontdb 0.24 src/lib.rs. It creates a
// new, empty `Database`.
//
// Generic font families would be set to:
//
//   - `serif` - Times New Roman
//   - `sans-serif` - Arial
//   - `cursive` - Comic Sans MS
//   - `fantasy` - Impact
//   - `monospace` - Courier New
//
// Port: fontdb sets fantasy to Papyrus on macOS and iOS. No code of okage
// depends on the platform (D9), so it is Impact on each platform.
func NewFontDatabase(opts ...FontDatabaseOption) *FontDatabase {
	db := &FontDatabase{
		familySerif:     "Times New Roman",
		familySansSerif: "Arial",
		familyCursive:   "Comic Sans MS",
		familyFantasy:   "Impact",
		familyMonospace: "Courier New",
	}
	for _, opt := range opts {
		opt(db)
	}
	return db
}

// Clone is the derived Clone for Database in fontdb 0.24 src/lib.rs. The
// copy shares the faces, which never change, and a font that is loaded into
// the copy does not go into db. usvg makes such a copy with Arc::make_mut.
func (db *FontDatabase) Clone() *FontDatabase {
	c := *db
	c.faces = slices.Clone(db.faces)
	return &c
}

// warnf logs the text of a log::warn! of fontdb to the logger of the
// database. Without a logger, it logs nothing (D20).
func (db *FontDatabase) warnf(format string, args ...any) {
	if db.logger == nil {
		return
	}
	db.logger.Warn(fmt.Sprintf(format, args...))
}

// LoadFontData is Database::load_font_data and Database::load_font_source in
// fontdb 0.24 src/lib.rs. It loads a font data into the `Database`, and
// returns the IDs of the loaded faces.
//
// Will load all font faces in case of a font collection.
//
// The database keeps data, so the caller must not change it afterwards.
func (db *FontDatabase) LoadFontData(data []byte) []FontID {
	return db.loadFontSource(data, "source")
}

// loadFontSource is Database::load_font_source and
// Database::load_fonts_from_file in fontdb 0.24 src/lib.rs. name names the
// source in a warning.
func (db *FontDatabase) loadFontSource(data []byte, name string) []FontID {
	source := &fontSource{data: data}
	source.dataOnce.Do(func() {})
	return db.loadFaces(source, bytes.NewReader(data), name)
}

// loadFontFile is Database::load_font_file in fontdb 0.24 src/lib.rs, for a
// file of fsys. It reads only the table directory and the tables of each
// FaceInfo, and the source reads the whole file when a face is used.
func (db *FontDatabase) loadFontFile(fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		db.warnf("Failed to load '%s' cause %s.", name, err)
		return
	}
	defer func() { _ = f.Close() }()
	source := &fontSource{fsys: fsys, name: name}
	res, ok := f.(opentype.Resource)
	if !ok {
		// The file cannot be read at an offset, so it is read whole.
		data, err := io.ReadAll(f)
		if err != nil {
			db.warnf("Failed to load '%s' cause %s.", name, err)
			return
		}
		source = &fontSource{data: data}
		source.dataOnce.Do(func() {})
		res = bytes.NewReader(data)
	}
	db.loadFaces(source, res, "'"+name+"'")
}

// loadFaces pushes a FaceInfo for each face of the font that res reads, as
// Database::load_font_source in fontdb 0.24 src/lib.rs does.
func (db *FontDatabase) loadFaces(source *fontSource, res opentype.Resource, name string) []FontID {
	lds, err := opentype.NewLoaders(res)
	if err != nil {
		// Port: ttf_parser::fonts_in_collection gives 1 for data that is
		// not a collection, and the parse of that face fails.
		db.warnf("Failed to load a font face %d from %s cause %s.", 0, name, "malformed font")
		return nil
	}
	ids := make([]FontID, 0, len(lds))
	for index, ld := range lds {
		info, err := parseFaceInfo(source, ld, uint32(index))
		if err != "" {
			db.warnf("Failed to load a font face %d from %s cause %s.", index, name, err)
			continue
		}
		ids = append(ids, db.PushFaceInfo(info))
	}
	return ids
}

// LoadFontsFS is Database::load_fonts_dir in fontdb 0.24 src/lib.rs. It loads
// font files from the selected directory of fsys into the `Database`.
//
// This method will scan directories recursively.
//
// Will load `ttf`, `otf`, `ttc` and `otc` fonts.
//
// Unlike other `load_*` methods, this one doesn't return an error.
// It will simply skip malformed fonts and will print a warning into the log for each of them.
//
// Port: fontdb reads a folder of the operating system. The database reads
// dir of fsys, which can be os.DirFS (D9). It visits the entries of a folder
// in the order of fs.ReadDir, sorted by name, where fontdb takes the order
// of the operating system.
func (db *FontDatabase) LoadFontsFS(fsys fs.FS, dir string) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		p := path.Join(dir, entry.Name())
		if entry.IsDir() {
			db.LoadFontsFS(fsys, p)
			continue
		}
		switch strings.ToLower(path.Ext(p)) {
		case ".ttf", ".ttc", ".otf", ".otc":
			db.loadFontFile(fsys, p)
		}
	}
}

// systemFontDirs holds the folders that Database::load_system_fonts in
// fontdb 0.24 src/lib.rs reads on Windows, macOS and Linux, without the
// first slash, so that they are names of an fs.FS of the root.
var systemFontDirs = []string{
	// Windows
	"Windows/Fonts",
	// macOS
	"Library/Fonts",
	"System/Library/Fonts",
	"System/Library/AssetsV2/com_apple_MobileAsset_Font6",
	"Network/Library/Fonts",
	// Linux and the other systems of fontdb without fontconfig
	"usr/share/fonts",
	"usr/local/share/fonts",
}

// LoadSystemFonts is Database::load_system_fonts in fontdb 0.24
// src/lib.rs. It loads the fonts of the known font folders of Windows, macOS
// and Linux from fsys, which is the root of a file system, such as
// os.DirFS("/") or os.DirFS(`C:\`).
//
// A folder that fsys does not have is skipped. On WebAssembly, where no such
// folder exists, it loads no font and that is no error.
//
// Port: fontdb reads the folders of one platform, the folders of the home
// directory and the configuration of fontconfig. This helper reads the
// folders of each platform from fsys, so that no build tag selects them
// (D9), and it reads no home directory and no fontconfig. Load the fonts of
// a home directory with LoadUserFonts.
func (db *FontDatabase) LoadSystemFonts(fsys fs.FS) {
	for _, dir := range systemFontDirs {
		db.LoadFontsFS(fsys, dir)
	}
}

// userFontDirs holds the folders of the home directory that
// Database::load_system_fonts in fontdb 0.24 src/lib.rs reads on Windows,
// macOS and Linux, relative to the home directory.
var userFontDirs = []string{
	// Windows, from USERPROFILE
	"AppData/Local/Microsoft/Windows/Fonts",
	"AppData/Roaming/Microsoft/Windows/Fonts",
	// macOS, from HOME
	"Library/Fonts",
	// Linux and the other systems of fontdb without fontconfig, from HOME
	".fonts",
	".local/share/fonts",
}

// LoadUserFonts loads the fonts of the folders of a home directory that
// Database::load_system_fonts in fontdb 0.24 src/lib.rs reads, from home,
// which is the home directory, such as os.DirFS of os.UserHomeDir.
//
// A folder that home does not have is skipped, and that is no error.
//
// Port: fontdb reads these folders in load_system_fonts, from the variable
// HOME or USERPROFILE. This helper reads the folders of each platform from
// home, so that no build tag selects them and no environment is read (D9).
func (db *FontDatabase) LoadUserFonts(home fs.FS) {
	for _, dir := range userFontDirs {
		db.LoadFontsFS(home, dir)
	}
}

// PushFaceInfo is Database::push_face_info in fontdb 0.24 src/lib.rs. It
// pushes a face info into the database and returns its ID.
func (db *FontDatabase) PushFaceInfo(info FaceInfo) FontID {
	id := FontID(len(db.faces))
	info.ID = id
	db.faces = append(db.faces, &info)
	return id
}

// IsEmpty is Database::is_empty in fontdb 0.24 src/lib.rs. It returns true
// if the `Database` contains no font faces.
func (db *FontDatabase) IsEmpty() bool {
	return len(db.faces) == 0
}

// Len is Database::len in fontdb 0.24 src/lib.rs. It returns the number of
// font faces in the `Database`.
//
// Note that `Database` stores font faces, not fonts.
// For example, if a caller will try to load a font collection (`*.ttc`) that contains 5 faces,
// then the `Database` will load 5 font faces and this method will return 5, not 1.
func (db *FontDatabase) Len() int {
	return len(db.faces)
}

// SetSerifFamily is Database::set_serif_family in fontdb 0.24 src/lib.rs. It
// sets the family that will be used by `Family::Serif`.
func (db *FontDatabase) SetSerifFamily(family string) {
	db.familySerif = family
}

// SetSansSerifFamily is Database::set_sans_serif_family in fontdb 0.24
// src/lib.rs. It sets the family that will be used by `Family::SansSerif`.
func (db *FontDatabase) SetSansSerifFamily(family string) {
	db.familySansSerif = family
}

// SetCursiveFamily is Database::set_cursive_family in fontdb 0.24
// src/lib.rs. It sets the family that will be used by `Family::Cursive`.
func (db *FontDatabase) SetCursiveFamily(family string) {
	db.familyCursive = family
}

// SetFantasyFamily is Database::set_fantasy_family in fontdb 0.24
// src/lib.rs. It sets the family that will be used by `Family::Fantasy`.
func (db *FontDatabase) SetFantasyFamily(family string) {
	db.familyFantasy = family
}

// SetMonospaceFamily is Database::set_monospace_family in fontdb 0.24
// src/lib.rs. It sets the family that will be used by `Family::Monospace`.
func (db *FontDatabase) SetMonospaceFamily(family string) {
	db.familyMonospace = family
}

// FamilyName is Database::family_name in fontdb 0.24 src/lib.rs. It returns
// the generic family name or the `Family::Name` itself.
//
// Generic family names should be set via `Database::set_*_family` methods.
//
// Port: fontdb has the enum Family. It has the same cases as FontFamily of
// svgtypes, which usvg maps to it one to one, so the database takes a
// FontFamily.
func (db *FontDatabase) FamilyName(family FontFamily) string {
	switch family.Kind {
	case FontFamilySerif:
		return db.familySerif
	case FontFamilySansSerif:
		return db.familySansSerif
	case FontFamilyCursive:
		return db.familyCursive
	case FontFamilyFantasy:
		return db.familyFantasy
	case FontFamilyMonospace:
		return db.familyMonospace
	}
	return family.Name
}

// FontQuery is Query in fontdb 0.24 src/lib.rs. It is a database query.
//
// Mainly used by `Database::query()`.
type FontQuery struct {
	// Families is a prioritized list of font family names or generic family
	// names.
	//
	// [font-family](https://www.w3.org/TR/2018/REC-css-fonts-3-20180920/#propdef-font-family) in CSS.
	Families []FontFamily

	// Weight specifies the weight of glyphs in the font, their degree of
	// blackness or stroke thickness.
	//
	// [font-weight](https://www.w3.org/TR/2018/REC-css-fonts-3-20180920/#font-weight-prop) in CSS.
	Weight uint16

	// Stretch selects a normal, condensed, or expanded face from a font
	// family.
	//
	// [font-stretch](https://www.w3.org/TR/2018/REC-css-fonts-3-20180920/#font-stretch-prop) in CSS.
	Stretch FontStretch

	// Style allows italic or oblique faces to be selected.
	//
	// [font-style](https://www.w3.org/TR/2018/REC-css-fonts-3-20180920/#font-style-prop) in CSS.
	Style FontStyle
}

// Query is Database::query in fontdb 0.24 src/lib.rs. It performs a CSS-like
// query and returns the best matched font face.
func (db *FontDatabase) Query(query FontQuery) (FontID, bool) {
	for _, family := range query.Families {
		name := db.FamilyName(family)
		var candidates []*FaceInfo
		for _, face := range db.faces {
			if slices.ContainsFunc(face.Families, func(f FontFamilyName) bool { return f.Name == name }) {
				candidates = append(candidates, face)
			}
		}

		if len(candidates) != 0 {
			if index, ok := findBestMatch(candidates, query); ok {
				return candidates[index].ID, true
			}
		}
	}

	return 0, false
}

// Faces is Database::faces in fontdb 0.24 src/lib.rs. It returns an iterator
// over the internal storage.
//
// This can be used for manual font matching.
func (db *FontDatabase) Faces() iter.Seq[*FaceInfo] {
	return func(yield func(*FaceInfo) bool) {
		for _, face := range db.faces {
			if !yield(face) {
				return
			}
		}
	}
}

// Face is Database::face in fontdb 0.24 src/lib.rs. It selects a `FaceInfo`
// by `id`.
//
// Returns `None` if a face with such ID was already removed,
// or this ID belong to the other `Database`.
func (db *FontDatabase) Face(id FontID) (*FaceInfo, bool) {
	if int(id) >= len(db.faces) {
		return nil, false
	}
	return db.faces[id], true
}

// FaceData returns the bytes of the font of the face id and the index of the
// face in it. It is Database::face_source in fontdb 0.24 src/lib.rs, for a
// source of bytes. The caller must not change the bytes.
func (db *FontDatabase) FaceData(id FontID) ([]byte, uint32, bool) {
	face, ok := db.Face(id)
	if !ok {
		return nil, 0, false
	}
	data := face.source.bytes()
	return data, face.Index, data != nil
}

// faceCharmap returns the character map of the face id, which the source of
// the face reads once and keeps (see fontSource.charmap).
func (db *FontDatabase) faceCharmap(id FontID) (font.Cmap, bool) {
	face, ok := db.Face(id)
	if !ok {
		return nil, false
	}
	return face.source.charmap(face.Index)
}

// withFaceData is Database::with_face_data in fontdb 0.24 src/lib.rs, with
// the parse of the face by go-text that each caller of upstream does on the
// data. It returns false when the face does not exist or does not parse.
func (db *FontDatabase) withFaceData(id FontID) (*parsedFace, bool) {
	face, ok := db.Face(id)
	if !ok {
		return nil, false
	}
	return face.source.parsed(face.Index)
}

// parseFaceInfo is parse_face_info in fontdb 0.24 src/lib.rs. It returns the
// text of the LoadError of upstream, or the empty string.
//
// Port: ttf_parser::RawFace::parse becomes the loader of go-text, which
// reads the table directory. The tables name, OS/2 and post are read from
// their bytes, as ttf-parser reads them, because go-text keeps the fields
// that fontdb reads private.
func parseFaceInfo(source *fontSource, ld *opentype.Loader, index uint32) (FaceInfo, string) {
	rawTable := func(tag string) ([]byte, bool) {
		b, err := ld.RawTable(opentype.MustNewTag(tag))
		return b, err == nil
	}
	nameData, ok := rawTable("name")
	if !ok {
		return FaceInfo{}, "an unnamed font"
	}
	families, postScriptName, ok := parseNames(nameData)
	if !ok {
		return FaceInfo{}, "an unnamed font"
	}
	os2Data, _ := rawTable("OS/2")
	style, weight, stretch := parseOS2(os2Data)
	postData, hasPost := rawTable("post")
	monospaced, italic := parsePost(postData, hasPost)

	if style == FontStyleNormal && italic {
		style = FontStyleItalic
	}

	return FaceInfo{
		Index:          index,
		Families:       families,
		PostScriptName: postScriptName,
		Style:          style,
		Weight:         weight,
		Stretch:        stretch,
		Monospaced:     monospaced,
		source:         source,
	}, ""
}

// The name IDs of the name table that fontdb reads, as ttf_parser::name_id
// names them.
const (
	nameIDFamily            = 1
	nameIDPostScriptName    = 6
	nameIDTypographicFamily = 16
)

// The platform IDs of the name table, as ttf_parser::PlatformId names them.
const (
	platformUnicode   = 0
	platformMacintosh = 1
	platformWindows   = 3
)

// nameRecord is a record of the name table, with its string, as
// ttf_parser::name::Name holds it.
type nameRecord struct {
	platformID uint16
	encodingID uint16
	languageID uint16
	nameID     uint16
	name       []byte
}

// isUnicode is Name::is_unicode in ttf-parser src/tables/name.rs, which
// fontdb calls.
func (n nameRecord) isUnicode() bool {
	switch n.platformID {
	case platformUnicode:
		return true
	case platformWindows:
		// https://docs.microsoft.com/en-us/typography/opentype/spec/name#windows-encoding-ids
		return n.encodingID == 0 || n.encodingID == 1
	}
	return false
}

// isMacRoman is NameExt::is_mac_roman in fontdb 0.24 src/lib.rs.
func (n nameRecord) isMacRoman() bool {
	// https://docs.microsoft.com/en-us/typography/opentype/spec/name#macintosh-encoding-ids-script-manager-codes
	const macintoshRomanEncodingID = 0

	return n.platformID == platformMacintosh && n.encodingID == macintoshRomanEncodingID
}

// isSupportedEncoding is NameExt::is_supported_encoding in fontdb 0.24
// src/lib.rs.
func (n nameRecord) isSupportedEncoding() bool {
	return n.isUnicode() || n.isMacRoman()
}

// language is Name::language in ttf-parser src/tables/name.rs, which fontdb
// calls. See FontFamilyName for the values.
func (n nameRecord) language() uint16 {
	if n.platformID == platformWindows {
		return n.languageID
	} else if n.platformID == platformMacintosh && n.encodingID == 0 && n.languageID == 0 {
		return FontLanguageEnglishUnitedStates
	}
	return 0
}

// parseNameTable is name::Table::parse in ttf-parser src/tables/name.rs,
// which fontdb calls. It returns the records of the name table, in their
// order, or false for a malformed table.
func parseNameTable(data []byte) ([]nameRecord, bool) {
	if len(data) < 6 {
		return nil, false
	}
	version := binary.BigEndian.Uint16(data)
	count := int(binary.BigEndian.Uint16(data[2:]))
	storageOffset := int(binary.BigEndian.Uint16(data[4:]))
	if version > 1 {
		return nil, false
	}
	const recordSize = 12
	if len(data) < 6+count*recordSize {
		return nil, false
	}
	if storageOffset > len(data) {
		return nil, false
	}
	storage := data[storageOffset:]
	records := make([]nameRecord, 0, count)
	for i := range count {
		r := data[6+i*recordSize:]
		length := int(binary.BigEndian.Uint16(r[8:]))
		offset := int(binary.BigEndian.Uint16(r[10:]))
		var name []byte
		if offset+length <= len(storage) {
			name = storage[offset : offset+length]
		}
		records = append(records, nameRecord{
			platformID: binary.BigEndian.Uint16(r),
			encodingID: binary.BigEndian.Uint16(r[2:]),
			languageID: binary.BigEndian.Uint16(r[4:]),
			nameID:     binary.BigEndian.Uint16(r[6:]),
			name:       name,
		})
	}
	return records, true
}

// parseNames is parse_names in fontdb 0.24 src/lib.rs.
func parseNames(data []byte) ([]FontFamilyName, string, bool) {
	names, ok := parseNameTable(data)
	if !ok {
		return nil, "", false
	}

	families := collectFamilies(nameIDTypographicFamily, names)

	// We have to fallback to Family Name when no Typographic Family Name was set.
	if len(families) == 0 {
		families = collectFamilies(nameIDFamily, names)
	}

	// Make English US the first one.
	if len(families) > 1 {
		if index := slices.IndexFunc(families, func(f FontFamilyName) bool {
			return f.Language == FontLanguageEnglishUnitedStates
		}); index > 0 {
			families[0], families[index] = families[index], families[0]
		}
	}

	if len(families) == 0 {
		return nil, "", false
	}

	for _, name := range names {
		if name.nameID == nameIDPostScriptName && name.isSupportedEncoding() {
			postScriptName, ok := nameToUnicode(name)
			if !ok {
				return nil, "", false
			}
			return families, postScriptName, true
		}
	}

	return nil, "", false
}

// collectFamilies is collect_families in fontdb 0.24 src/lib.rs.
func collectFamilies(nameID uint16, names []nameRecord) []FontFamilyName {
	var families []FontFamilyName
	for _, name := range names {
		if name.nameID == nameID && name.isUnicode() {
			if family, ok := nameToUnicode(name); ok {
				families = append(families, FontFamilyName{family, name.language()})
			}
		}
	}

	// If no Unicode English US family name was found then look for English MacRoman as well.
	if !slices.ContainsFunc(families, func(f FontFamilyName) bool {
		return f.Language == FontLanguageEnglishUnitedStates
	}) {
		for _, name := range names {
			if name.nameID == nameID && name.isMacRoman() {
				if family, ok := nameToUnicode(name); ok {
					families = append(families, FontFamilyName{family, name.language()})
					break
				}
			}
		}
	}

	return families
}

// nameToUnicode is name_to_unicode in fontdb 0.24 src/lib.rs.
func nameToUnicode(name nameRecord) (string, bool) {
	var rawData []uint16
	switch {
	case name.isUnicode():
		rawData = make([]uint16, 0, len(name.name)/2)
		for i := 0; i+2 <= len(name.name); i += 2 {
			rawData = append(rawData, binary.BigEndian.Uint16(name.name[i:]))
		}
	case name.isMacRoman():
		// We support only MacRoman encoding here, which should be enough in most cases.
		rawData = make([]uint16, 0, len(name.name))
		for _, b := range name.name {
			rawData = append(rawData, macRoman[b])
		}
	default:
		return "", false
	}

	// String::from_utf16 fails on an unpaired surrogate.
	for i := 0; i < len(rawData); i++ {
		c := rawData[i]
		switch {
		case c >= 0xD800 && c < 0xDC00:
			if i+1 >= len(rawData) || rawData[i+1] < 0xDC00 || rawData[i+1] >= 0xE000 {
				return "", false
			}
			i++
		case c >= 0xDC00 && c < 0xE000:
			return "", false
		}
	}
	return string(utf16.Decode(rawData)), true
}

// parseOS2 is parse_os2 in fontdb 0.24 src/lib.rs.
//
// Port: os2::Table::parse of ttf-parser checks the size of the table for its
// version. Table::style, Table::weight and Table::width read the fields
// fsSelection, usWeightClass and usWidthClass.
func parseOS2(data []byte) (FontStyle, uint16, FontStretch) {
	if len(data) < 2 {
		return FontStyleNormal, 400, FontStretchNormal
	}
	version := binary.BigEndian.Uint16(data)
	var size int
	switch version {
	case 0:
		size = 78
	case 1:
		size = 86
	case 2, 3, 4:
		size = 96
	case 5:
		size = 100
	default:
		size = 100
	}
	if len(data) < size {
		return FontStyleNormal, 400, FontStretchNormal
	}

	fsSelection := binary.BigEndian.Uint16(data[62:])
	var style FontStyle
	switch {
	case fsSelection&1 != 0:
		style = FontStyleItalic
	case version >= 4 && fsSelection&(1<<9) != 0:
		style = FontStyleOblique
	default:
		style = FontStyleNormal
	}

	weight := binary.BigEndian.Uint16(data[4:])
	var stretch FontStretch
	switch binary.BigEndian.Uint16(data[6:]) {
	case 1:
		stretch = FontStretchUltraCondensed
	case 2:
		stretch = FontStretchExtraCondensed
	case 3:
		stretch = FontStretchCondensed
	case 4:
		stretch = FontStretchSemiCondensed
	case 6:
		stretch = FontStretchSemiExpanded
	case 7:
		stretch = FontStretchExpanded
	case 8:
		stretch = FontStretchExtraExpanded
	case 9:
		stretch = FontStretchUltraExpanded
	default:
		stretch = FontStretchNormal
	}

	return style, weight, stretch
}

// parsePost is parse_post in fontdb 0.24 src/lib.rs. hasPost is false when
// the face has no post table.
func parsePost(data []byte, hasPost bool) (bool, bool) {
	// We need just a single value from the `post` table, while ttf-parser will parse all.
	// Therefore we have a custom parser.

	if !hasPost {
		return false, false
	}

	isNotZero := func(start int) bool {
		if len(data) < start+4 {
			return true
		}
		return binary.BigEndian.Uint32(data[start:]) != 0
	}

	// All we care about, it that u32 at offset 12 is non-zero.
	monospaced := isNotZero(12)

	// Italic angle as f16.16.
	italic := isNotZero(4)

	return monospaced, italic
}

// stretchToNumber is Width::to_number in ttf-parser src/tables/os2.rs, which
// fontdb calls on a stretch.
func stretchToNumber(s FontStretch) uint16 {
	return uint16(s) + 1
}

// findBestMatch is find_best_match in fontdb 0.24 src/lib.rs.
//
// https://www.w3.org/TR/2018/REC-css-fonts-3-20180920/#font-style-matching
// Based on https://github.com/servo/font-kit
//
// Port: min_by_key of Rust returns the first of the equal minimums, and so
// does minByKey. The subtractions of u16 wrap, as in a release build of
// Rust.
func findBestMatch(candidates []*FaceInfo, query FontQuery) (int, bool) {
	// minByKey is Iterator::min_by_key over the indexes of set that keep
	// passes, or false when none does.
	minByKey := func(set []int, keep func(int) bool, key func(int) uint16) (int, bool) {
		best, found := 0, false
		var bestKey uint16
		for _, index := range set {
			if !keep(index) {
				continue
			}
			k := key(index)
			if !found || k < bestKey {
				best, bestKey, found = index, k, true
			}
		}
		return best, found
	}
	all := func(int) bool { return true }

	// Step 4.
	matchingSet := make([]int, len(candidates))
	for i := range matchingSet {
		matchingSet[i] = i
	}

	// Step 4a (`font-stretch`).
	matches := slices.ContainsFunc(matchingSet, func(index int) bool {
		return candidates[index].Stretch == query.Stretch
	})
	var matchingStretch FontStretch
	switch {
	case matches:
		// Exact match.
		matchingStretch = query.Stretch
	case query.Stretch <= FontStretchNormal:
		// Closest stretch, first checking narrower values and then wider values.
		stretch, ok := minByKey(matchingSet,
			func(index int) bool { return candidates[index].Stretch < query.Stretch },
			func(index int) uint16 {
				return stretchToNumber(query.Stretch) - stretchToNumber(candidates[index].Stretch)
			})
		if ok {
			matchingStretch = candidates[stretch].Stretch
		} else {
			matchingIndex, ok := minByKey(matchingSet, all, func(index int) uint16 {
				return stretchToNumber(candidates[index].Stretch) - stretchToNumber(query.Stretch)
			})
			if !ok {
				return 0, false
			}

			matchingStretch = candidates[matchingIndex].Stretch
		}
	default:
		// Closest stretch, first checking wider values and then narrower values.
		stretch, ok := minByKey(matchingSet,
			func(index int) bool { return candidates[index].Stretch > query.Stretch },
			func(index int) uint16 {
				return stretchToNumber(candidates[index].Stretch) - stretchToNumber(query.Stretch)
			})
		if ok {
			matchingStretch = candidates[stretch].Stretch
		} else {
			matchingIndex, ok := minByKey(matchingSet, all, func(index int) uint16 {
				return stretchToNumber(query.Stretch) - stretchToNumber(candidates[index].Stretch)
			})
			if !ok {
				return 0, false
			}

			matchingStretch = candidates[matchingIndex].Stretch
		}
	}
	matchingSet = slices.DeleteFunc(matchingSet, func(index int) bool {
		return candidates[index].Stretch != matchingStretch
	})

	// Step 4b (`font-style`).
	var stylePreference [3]FontStyle
	switch query.Style {
	case FontStyleItalic:
		stylePreference = [3]FontStyle{FontStyleItalic, FontStyleOblique, FontStyleNormal}
	case FontStyleOblique:
		stylePreference = [3]FontStyle{FontStyleOblique, FontStyleItalic, FontStyleNormal}
	default:
		stylePreference = [3]FontStyle{FontStyleNormal, FontStyleOblique, FontStyleItalic}
	}
	styleIndex := slices.IndexFunc(stylePreference[:], func(queryStyle FontStyle) bool {
		return slices.ContainsFunc(matchingSet, func(index int) bool {
			return candidates[index].Style == queryStyle
		})
	})
	if styleIndex < 0 {
		return 0, false
	}
	matchingStyle := stylePreference[styleIndex]

	matchingSet = slices.DeleteFunc(matchingSet, func(index int) bool {
		return candidates[index].Style != matchingStyle
	})

	// Step 4c (`font-weight`).
	//
	// The spec doesn't say what to do if the weight is between 400 and 500 exclusive, so we
	// just use 450 as the cutoff.
	weight := query.Weight

	hasWeight := func(w uint16) bool {
		return slices.ContainsFunc(matchingSet, func(index int) bool {
			return candidates[index].Weight == w
		})
	}
	var matchingWeight uint16
	switch {
	case hasWeight(weight):
		matchingWeight = weight
	case weight >= 400 && weight < 450 && hasWeight(500):
		// Check 500 first.
		matchingWeight = 500
	case weight >= 450 && weight <= 500 && hasWeight(400):
		// Check 400 first.
		matchingWeight = 400
	case weight <= 500:
		// Closest weight, first checking thinner values and then fatter ones.
		idx, ok := minByKey(matchingSet,
			func(index int) bool { return candidates[index].Weight <= weight },
			func(index int) uint16 { return weight - candidates[index].Weight })
		if ok {
			matchingWeight = candidates[idx].Weight
		} else {
			matchingIndex, ok := minByKey(matchingSet, all,
				func(index int) uint16 { return candidates[index].Weight - weight })
			if !ok {
				return 0, false
			}
			matchingWeight = candidates[matchingIndex].Weight
		}
	default:
		// Closest weight, first checking fatter values and then thinner ones.
		idx, ok := minByKey(matchingSet,
			func(index int) bool { return candidates[index].Weight >= weight },
			func(index int) uint16 { return candidates[index].Weight - weight })
		if ok {
			matchingWeight = candidates[idx].Weight
		} else {
			matchingIndex, ok := minByKey(matchingSet, all,
				func(index int) uint16 { return weight - candidates[index].Weight })
			if !ok {
				return 0, false
			}
			matchingWeight = candidates[matchingIndex].Weight
		}
	}
	matchingSet = slices.DeleteFunc(matchingSet, func(index int) bool {
		return candidates[index].Weight != matchingWeight
	})

	// Ignore step 4d (`font-size`).

	// Return the result.
	if len(matchingSet) == 0 {
		return 0, false
	}
	return matchingSet[0], true
}

// macRoman is MAC_ROMAN in fontdb 0.24 src/lib.rs. It is the Macintosh Roman
// to UTF-16 encoding table.
//
// https://en.wikipedia.org/wiki/Mac_OS_Roman
//
//nolint:dupl // the table of fontdb, which looks like another table of numbers
var macRoman = [256]uint16{
	0x0000, 0x0001, 0x0002, 0x0003, 0x0004, 0x0005, 0x0006, 0x0007,
	0x0008, 0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x000E, 0x000F,
	0x0010, 0x2318, 0x21E7, 0x2325, 0x2303, 0x0015, 0x0016, 0x0017,
	0x0018, 0x0019, 0x001A, 0x001B, 0x001C, 0x001D, 0x001E, 0x001F,
	0x0020, 0x0021, 0x0022, 0x0023, 0x0024, 0x0025, 0x0026, 0x0027,
	0x0028, 0x0029, 0x002A, 0x002B, 0x002C, 0x002D, 0x002E, 0x002F,
	0x0030, 0x0031, 0x0032, 0x0033, 0x0034, 0x0035, 0x0036, 0x0037,
	0x0038, 0x0039, 0x003A, 0x003B, 0x003C, 0x003D, 0x003E, 0x003F,
	0x0040, 0x0041, 0x0042, 0x0043, 0x0044, 0x0045, 0x0046, 0x0047,
	0x0048, 0x0049, 0x004A, 0x004B, 0x004C, 0x004D, 0x004E, 0x004F,
	0x0050, 0x0051, 0x0052, 0x0053, 0x0054, 0x0055, 0x0056, 0x0057,
	0x0058, 0x0059, 0x005A, 0x005B, 0x005C, 0x005D, 0x005E, 0x005F,
	0x0060, 0x0061, 0x0062, 0x0063, 0x0064, 0x0065, 0x0066, 0x0067,
	0x0068, 0x0069, 0x006A, 0x006B, 0x006C, 0x006D, 0x006E, 0x006F,
	0x0070, 0x0071, 0x0072, 0x0073, 0x0074, 0x0075, 0x0076, 0x0077,
	0x0078, 0x0079, 0x007A, 0x007B, 0x007C, 0x007D, 0x007E, 0x007F,
	0x00C4, 0x00C5, 0x00C7, 0x00C9, 0x00D1, 0x00D6, 0x00DC, 0x00E1,
	0x00E0, 0x00E2, 0x00E4, 0x00E3, 0x00E5, 0x00E7, 0x00E9, 0x00E8,
	0x00EA, 0x00EB, 0x00ED, 0x00EC, 0x00EE, 0x00EF, 0x00F1, 0x00F3,
	0x00F2, 0x00F4, 0x00F6, 0x00F5, 0x00FA, 0x00F9, 0x00FB, 0x00FC,
	0x2020, 0x00B0, 0x00A2, 0x00A3, 0x00A7, 0x2022, 0x00B6, 0x00DF,
	0x00AE, 0x00A9, 0x2122, 0x00B4, 0x00A8, 0x2260, 0x00C6, 0x00D8,
	0x221E, 0x00B1, 0x2264, 0x2265, 0x00A5, 0x00B5, 0x2202, 0x2211,
	0x220F, 0x03C0, 0x222B, 0x00AA, 0x00BA, 0x03A9, 0x00E6, 0x00F8,
	0x00BF, 0x00A1, 0x00AC, 0x221A, 0x0192, 0x2248, 0x2206, 0x00AB,
	0x00BB, 0x2026, 0x00A0, 0x00C0, 0x00C3, 0x00D5, 0x0152, 0x0153,
	0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0x00F7, 0x25CA,
	0x00FF, 0x0178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02,
	0x2021, 0x00B7, 0x201A, 0x201E, 0x2030, 0x00C2, 0x00CA, 0x00C1,
	0x00CB, 0x00C8, 0x00CD, 0x00CE, 0x00CF, 0x00CC, 0x00D3, 0x00D4,
	0xF8FF, 0x00D2, 0x00DA, 0x00DB, 0x00D9, 0x0131, 0x02C6, 0x02DC,
	0x00AF, 0x02D8, 0x02D9, 0x02DA, 0x00B8, 0x02DD, 0x02DB, 0x02C7,
}
