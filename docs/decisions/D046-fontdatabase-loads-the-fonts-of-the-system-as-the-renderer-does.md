# D46. `FontDatabase` loads the fonts of the system as the renderer does

Status: Decided, amends D36.

D36 gives the `Renderer` of `svg` a private function that loads the fonts of
the system and of the home directory. It finds the root of the file system,
which on Windows is the volume of `%WINDIR%`. iv measures text with
`ShapeText` and its own font database, and it copied that function so that it
loads the same fonts as the renderer. A copy can fall out of step when the
rule changes. The maintainers want the function in okage, for each program.

## The decision

1. `FontDatabase.LoadOSFonts()` loads the fonts of the system and of the home
   directory into the database. It calls `LoadSystemFonts` with the root of
   the file system and `LoadUserFonts` with the home directory that
   `os.UserHomeDir` gives. On Windows, the root is the volume of `%WINDIR%`,
   or `C:` when `WINDIR` is not set.
2. The `Renderer` calls `LoadOSFonts` where it called the private function,
   so the renderer and a program that calls `LoadOSFonts` load the same fonts.
3. `LoadOSFonts` loads fonts only. It does not set the generic families. The
   `Renderer` still sets them, as D36 says.
4. `LoadOSFonts` is in `svg/renderer.go`. It is now the one place outside
   `internal/wide` where the behavior depends on the platform. Hard rule 8 of
   `AGENTS.md` names it.

## Why

A program that shapes text with its own database, beside a `Renderer`, needs
the same fonts as the renderer, or the same text gets other fonts in the two
places. One exported method keeps the rule in one place.
