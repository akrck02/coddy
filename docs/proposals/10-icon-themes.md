# 10. Zed's icon themes

> Status: built, pending review · Author's approval: proposal ☑ ("aprobado", 2026-11-24) plan ☑ ("dale", 2026-11-24)

## Why

The colour themes already come from Zed, but the icons do not: every file is a Lucide shape in one
colour, tinted by the kind of code. People who use Zed, or who like a set such as Catppuccin's or
Material's, cannot bring their icons along. Zed's icon themes are a small, documented format
(a JSON file that names an SVG for each kind of file), so reading them gives the reader every set
already made for Zed.

## What

An icon theme is Zed's format (https://zed.dev/docs/extensions/icon-themes, schema
`icon_themes/v0.3.0`): a folder with `icon_themes/*.json` and the SVG files they name, the paths
taken from that folder.

| Id | Requirement | Check |
|---|---|---|
| R1 | The reader reads an icon theme folder: each theme's name, its icons for files by whole name (`file_stems`) and by extension (`file_suffixes`), the `default` file icon, folder icons (open and closed, and by folder name) and chevrons | test: `an icon theme names the icon of each file and folder` |
| R2 | A file is matched as Zed matches it: whole name first, then the longest extension (`tar.gz` before `gz`), then `default` | test: `a file takes the icon of its name, then of its longest extension` |
| R3 | Icon themes are found in the reader's themes folder (`icon-themes/`, beside `themes/`) and in Zed's installed extensions, so a set installed in Zed shows up without copying | test: `the icon themes of the folder and of Zed's extensions are listed`; by hand on a Mac and on Fedora with Zed |
| R4 | The SVGs are drawn in their own colours, sharp at any size and scale, in the sidebar, the finder and the review | probe on Catppuccin's and Material's sets (see Design); capture in light and dark |
| R5 | The icon theme is chosen in Settings › Appearance, each one shown by a few of its icons, and remembered | test: `choosing an icon theme paints the sidebar with it`; capture of the settings |
| R6 | Lucide, tinted by the colour theme as today, stays the built-in icon theme and the default | test: `without an icon theme the files keep today's icons` |
| R7 | A broken theme file or an SVG that cannot be read never stops the reader: the file falls back to `default`, then to Lucide, and the problem is told once, with its code | test: `a missing or broken icon falls back and is told by its code` |
| R8 | Drawing icons costs nothing per frame: an SVG is read and drawn into the atlas once per size, never in the frame loop | test with `testing.AllocsPerRun` on drawing a listed file |

## Out of scope

- The whole of SVG. Only what icon sets use: `path` (every command), `rect`, `circle`, `ellipse`,
  `line`, `polyline`, `polygon`, `g` with `transform`, fill and stroke colours with opacity,
  `viewBox`. No text, gradients, masks, filters, CSS or animation; a shape that uses them is drawn
  without them.
- Installing icon themes from Zed's extension store. They are copied in, or installed with Zed.
- Changing the reader's own interface icons (rail, toolbar, settings): those stay Lucide, as in
  Zed, where icon themes change only files, folders and chevrons.

## Design

Two parts, in two repositories, built in this order:

1. **Go Renderer reads and draws SVG**: its proposal 6 (`go-renderer/docs/proposals/6-svg-pictures.md`),
   a new `svg` package. `svg.Read` gives a drawing, `Drawing.Picture(width, height)` a
   `draw2d.Image` in the drawing's own colours. Coddy waits for that release before it can merge.
2. **The reader uses icon themes**, in `internal/theme`, beside the colour themes:
   - `icontheme.go` reads Zed's files into an `IconTheme`: its name, appearance, the folder its
     paths start from, and its maps (stems, suffixes, icons, folders, chevrons).
   - Matching follows Zed's own code (`crates/file_icons/src/file_icons.rs`, read on 24 November
     2026), exactly and case-sensitive: the whole name in the stems and then in the suffixes;
     then each suffix after a dot, the longest first (`a.spec.ts`: `spec.ts`, then `ts`); then
     what comes before the first dot (`Dockerfile.prod`: `Dockerfile`); then `default`. A folder
     goes by its name in `named_directory_icons`, then `directory_icons`.
   - Icon themes are looked for in the reader's `icon-themes/` folder (beside `themes/`) and in
     Zed's installed extensions (`~/Library/Application Support/Zed/extensions/installed` on a
     Mac, `~/.local/share/zed/extensions/installed` elsewhere): every folder with
     `icon_themes/*.json`, its paths taken from that folder.
   - `FileIcon` and the folder icon give back an `Icon`: a Lucide name and its tint, as today, or
     a picture. One `DrawIcon` draws either, so the sidebar, the finder and the review keep one
     call each.
   - An icon's SVG is read the first time it is drawn and its picture kept per size, drawn at
     twice the size, as Go Renderer draws strokes, so it stays sharp on a dense screen. Nothing
     is read or drawn again in later frames (R8).
   - The icon theme is part of `Look`, which every painter already gets, so no new argument goes
     down the layers. It is remembered as the preference `icon-theme`.
   - Java and Kotlin classes keep their badges (class, interface, enum…) whatever the icon theme:
     they say more than a file icon does. Folder roles (sources, tests, modules) are Lucide's only;
     with an icon theme, folders go by name, as in Zed.

**The author's questions, answered by default** (say if not): Zed's installed extensions are read
(R3); the reader ships no icon set besides Lucide, people bring theirs.

**Checked first.** Catppuccin's set (`catppuccin/zed-icons`, MIT: 8 themes, 5,248 SVGs) and
Material's (`zed-extensions/material-icon-theme`, Apache-2.0: 1,252 SVGs) were counted: mostly
`path`, strokes with round caps and joins (Catppuccin) and fills (Material), a few `circle`,
`ellipse`, `rect` and `g` with `transform`; gradients in 5 % of Material's, masks and clip paths
in 0.3 % of Catppuccin's. Catppuccin ships "Monochrome" variants of each flavour, a sign that Zed
draws the others in their own colours. Stems and suffixes come in several cases (`Rmd`,
`.PUG-LINTRC`), so matching must not fold case.

## Tasks

Go Renderer's tasks T1–T6 come first (its proposal 6). Then:

- [x] T1 — read icon theme files and match names as Zed does · `internal/theme/icontheme.go`, `errors.go`, `testdata/icons/` (a made-up set) · R1, R2, R7 · tests: `an icon theme names the icon of each file and folder`, `a file takes the icon of its name, then of its longest extension`, `a broken icon theme file is told by its code`
- [x] T2 — find icon themes in the reader's folder and in Zed's extensions · `internal/theme/iconthemes.go` · R3 · test: `the icon themes of the folder and of Zed's extensions are listed`
- [x] T3 — `Icon` and `DrawIcon`: pictures kept per size; the sidebar, the finder and the review draw through it; fallbacks · `internal/theme/fileicons.go`, `internal/sidebar/paint.go`, `classes.go`, `internal/finder/finder.go` · R4, R6, R7, R8 · tests: `without an icon theme the files keep today's icons`, `a missing or broken icon falls back and is told by its code`, allocations of drawing a listed file
- [x] T4 — choose the icon theme in Settings › Appearance, each a card of a few of its icons; remembered · `internal/window/settingsappearance.go`, `window.go`, `cmd/coddy/main.go` · R5 · test: `choosing an icon theme paints the sidebar with it`; captures in light and dark
- [x] T5 — docs: features, getting started (where icon themes go), this proposal's Result · `docs/1-features.md`, `docs/2-get-started.md` · by hand on a Mac and on Fedora with Catppuccin's set

## Result

Built on 24 November 2026, pending the author's review.

- Zed's icon themes are read, matched as Zed matches them, found in `icon-themes/` and in Zed's
  installed extensions, and drawn in the sidebar and the finder through one `DrawIcon`. Each test
  named above passes; drawing a listed file allocates nothing.
- Settings › Appearance has a *File icons* part: Lucide and a card per icon theme, each showing a
  folder and seven common files drawn on the colour theme's editor ground. Captured in light and
  dark with Catppuccin's and Material's sets.
- Found while building: a card's name too long for it (Catppuccin's "… Monochrome" themes) now
  ends in "…", and the settings' notes break into lines instead of running past a narrow window;
  both have their test or capture.
- The review's changed files are the sidebar's rows, so they take the icon theme with it; the
  review draws no file icon of its own.
- Not checked by hand: a Mac, and Fedora with Zed installed (R3). Coddy waits for Go Renderer's
  release with `svg` before it can merge.
