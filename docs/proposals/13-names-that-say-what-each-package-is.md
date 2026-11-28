# 13. Names that say what each package is

> Status: done · Author's approval: proposal ☑ plan ☑ (with `codebase`)

## Why

Three of the editor's packages make a newcomer guess. Since proposal 12, *the workspace* is the
folder a person opens, but `workspace` holds the model of one project, and what is open lives in
`session`. The modes (`files`, `review`, `uses`, `pluginmode`) sit among forty packages with
nothing saying they are modes. And `theme` holds two things: the colour themes, and the reader's
own controls, which is why a package of colours imports `git`.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | What is open (the folders, the projects found in them, their bases, the documents read) is the package `workspace`, with its type `Workspace` | `go build`; `TestOnlyTheSaveWritesInARepository` and the architecture tests still pass |
| R2 | A project's model (`Project`, `FileReference`, `Document`, the tree, `VersionControl`, `Save`) is the package `codebase` | the same |
| R3 | Every mode is under `internal/mode/`: `mode/files`, `mode/review`, `mode/uses`, `mode/pluginmode`; the contract stays `mode` | `TestTheWindowImportsNoMode` checks the new paths |
| R4 | `theme` holds only the colour themes and the look made of them; the controls drawn with them (key caps, icon switch, buttons, rows, breadcrumbs, file icons, status colours) are the package `controls` | a new architecture test: `theme` imports neither `git` nor `controls` |
| R5 | Nothing changes for the person using Coddy | every test passes unchanged, but for their imports and names; a capture looks the same |
| R6 | The docs name the packages as they are | `4-how-it-is-made.md`, `8-contributing.md` and the guides list the new names |

## Out of scope

- Splitting `window` (point 3 of the review): it needs a design of its own.
- Taking highlighted lines out of `Document` (point 5), and moving Java's packages to a plugin
  (point 6).
- Rewriting earlier proposals: they keep the names they had, as the record of their time.

## Design

**Why `codebase` and not `project`.** The name agreed in conversation was `project`, but the
editor uses `project` as a variable 1 194 times (`for _, project := range …`). A package named so
is hidden by each of them, and Go's own advice is not to name a package after a common
variable. `codebase` collides with nothing, reads well (`codebase.Project`,
`codebase.FileReference`, `codebase.Save`) and says what it is: a project's code, its files, its
tree and its history. *This name needs the author's approval.*

**`session` becomes `workspace`.** The folder moves, and its type `Session` becomes `Workspace`;
the window's field `session` becomes `workspace`. `workspace.Workspace` repeats itself, as
`time.Time` does; the alternative, `workspace.Open`, reads as a verb.

**Modes.** `git mv` each mode into `internal/mode/`. Their package names stay (`files`,
`review`, `uses`, `pluginmode`), so only import paths change. `pluginmode` keeps its name: a
package `plugin` would be hidden by the 400 variables named `plugin`.

**Controls.** `breadcrumb.go`, `controls.go`, `fileicons.go`, `keycaps.go`, `rows.go` and
`status.go` move to `internal/controls`, with their tests. `look.go`, `themes.go` and the themes
stay in `theme`. `controls` imports `theme`, `git`, `icons` and `highlight`; `theme` imports no
`git` any more.

**How.** Each rename is one mechanical step: move the folder, rewrite the imports and the
qualified names with `gofmt -r` and `sed`, build, test. The order puts the smallest first, so a
mistake shows early.

## Tasks

- [x] T1 — move the modes into `internal/mode/` · `cmd/coddy`, `window`, `rules_test.go` · R3
- [x] T2 — split `controls` out of `theme`, with its architecture test · `theme`, `controls`,
  every pane that draws a control · R4
- [x] T3 — rename `workspace` to `codebase` · 80 files · R2
- [x] T4 — rename `session` to `workspace`, `Session` to `Workspace` · `session`, `window`,
  their tests · R1
- [x] T5 — docs, and a capture before and after · `docs/4-how-it-is-made.md`,
  `docs/8-contributing.md`, `docs/5-guides/*`, `editor/DESIGN.md` · R5, R6

## Changed while building

- `codebase`'s error codes are `codebase.…` (`codebase.changed-on-disk`), as each code starts with
  its package; nothing outside the editor checks them yet.
- Files named after the old package follow it: `session.go` is `workspace/workspace.go`, its test
  `workspace_test.go`. The test `TestAFileIsKnownByItsNameBeforeItsExtension` moved with
  `FileIcon` to `controls`.
- `TestTheWindowImportsNoMode` now also checks `mode/uses`; the domain's check covers every mode
  through `mode/`.

## Result

Built: R1 to R6. The new test `TestTheThemeKnowsNeitherTheControlsNorGit` passes; every other
test passes with only its imports and names changed. A capture of the same file before and after,
in dark and light, is the same to the byte. Not done, as planned: splitting `window`.
