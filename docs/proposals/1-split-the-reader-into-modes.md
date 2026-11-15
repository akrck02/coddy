# 1. Split the reader into packages, one per mode

> Status: done · Author's approval: proposal ☑ plan ☑ (T8 change ☑, T9 change ☑)

## Why

The reader will soon need more than reading: finding the usages of a class, commenting, maybe
small edits. Today two thirds of the code (3,802 of 5,697 lines) sit in one package, `app`, around
one `App` type with 52 methods. The two modes are an enum checked with `if` in three files, and
every shortcut is a case of one 22-case `switch`. A third mode would touch all of them. Splitting
now, while there are two modes to learn from, is cheaper than after the third.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | Everything a reviewer does today works the same after the split. | The tests of today pass, plus new ones for every shortcut of each mode (T1). |
| R2 | The reader still never writes to a repository it reviews. | `TestOnlyReadOnlyCommandsRun` refuses the same commands as today. |
| R3 | A mode lives in one package; adding one takes that package and one line in `main`. | `TestTheWindowImportsNoMode`: `window` imports neither `files` nor `review`. |
| R4 | Dependencies point one way: the domain knows nothing of the window. | `TestTheWorkspaceImportsNoInterface`: `workspace` imports no `go-ui`, `window` or mode. |
| R5 | Every error carries a code that programs check and people can search for. | Each package's `errors_test.go` checks its constructors; no `errors.New` or `fmt.Errorf` outside the error package. |
| R6 | The files marked as viewed before the split stay marked after it. | `TestMarksSavedBeforeTheSplitAreRead`, with a `reviewed.json` of today's format. |
| R7 | Each shortcut is declared once, and the help lists what is declared. | `TestNoTwoCommandsShareAKeyInOneMode` and `TestTheHelpListsEveryCommand`. |
| R8 | A newer search replaces an older one still running. | `TestANewerSearchCancelsTheOlderOne`. |

## Out of scope

- New modes (usages, comments, editing) and language servers: each gets its own proposal.
- Any way to write to a repository. If a mode ever needs one, its proposal designs it.
- Visual changes: the window looks exactly as it does now.
- Bringing the repository up to standard (licence, CI, published docs, logo, a shorter README):
  a separate change through the `repository` skill.

## Design

Packages by what the program is about, following Go's official layout. `main` reads the flags,
builds the pieces and connects them; nothing else knows every piece.

```
cmd/code-review/main.go   flags, wiring; today's main.go
internal/workspace/       projects, changes, documents, diff rows, line marks, the file tree
internal/git/             the read-only runner, unchanged
internal/highlight/       unchanged
internal/java/            the IntelliJ-style tree and class kinds
internal/theme/           Zed One colours, light and dark, the icon of each file type, and
                          the reader's own controls (tool buttons, switches, fitted text)
internal/pane/            what every pane shares: the frame's keys and a list of rows that
                          scrolls
internal/window/          title bar, mode rail, status bar, layout, commands and keys,
                          background work
internal/mode/            what a mode is, and what it asks of the window
internal/viewer/          the code view both modes share: rows, scrolling, gutter, painting
internal/sidebar/         the tree pane; each mode says which files it lists, and how it marks them
internal/finder/          search by name and by content, with its view
internal/files/           the files mode: whole files, only-changed filter, entering a project
internal/review/          the review mode: diffs, viewed marks and their file, the overview
```

Where today's files go:

| Today | After |
|---|---|
| `main.go` | `cmd/code-review/main.go` |
| `review/project, document, rows, marks, tree` | `workspace` |
| `review/javatree.go`, `app/classes.go` | `java` (the badge painting moves to `sidebar`) |
| `review/fuzzy.go`, `app/finder.go` | `finder` |
| `review/reviewed.go`, `app/reviewing.go` | `review` |
| `app/editor.go` | `viewer` |
| `app/scroller.go`, the keys of `app/input.go` | `pane` |
| `fileReference` in `app/app.go` | `workspace.FileReference` |
| `app/sidebar.go` | `sidebar` |
| `app/theme.go`, `app/fileicons.go` | `theme` |
| `app/app.go`, `build.go`, `work.go`, `commands.go`, the input tap of `input.go` | `window` |

**Modes.** `mode` declares what a mode is; `files` and `review` implement it; `main` registers
them. `window` holds the modes it is given and imports none of them, and the panes import `theme`
instead of `window`, so the arrows only point down: modes, then `window`, then `mode` and the panes,
then `theme`, `workspace` and `git`.

Both modes list the same tree and open the same files; what differs is which files are listed,
how a changed file opens, and what each row shows beside its name. So a mode answers those
questions, and the sidebar and the viewer ask them instead of checking which mode is on. The
sidebar declares the part it asks, `sidebar.Listing`; a mode is that and a little more:

```go
// Mode is one way of looking at the workspace, chosen from the mode rail and by its number.
type Mode interface {
	sidebar.Listing // Name, ChangesOnly, Header, Decorate(row), Press(file, moveOn)
	Icon() string
	Description() string // the rail's tooltip
	Start(window Window) // the window the mode works in, once
	ShowsChanges() bool  // opens a changed file on its changes, not whole
	// FileButton is the mode's button in the viewer's header, such as Viewed.
	FileButton(file workspace.FileReference) (viewer.Button, bool)
	Progress() (done, total int) // for the status bar
	BuildInsteadOfAFile(context *ui.Context) bool
	Commands() []Command // the mode's own keys, such as v
}
```

The review mode keeps its marks (`reviewed.json`), the progress, the overview and the `v` key; the
files mode keeps the `g` key of the only-changed filter. *Changed while building: the first
design had a mode build the sidebar's rows (`Rows`) and the viewer's presentation (`Show`). The
rows are the same tree in both modes, only filtered and marked differently, so a mode now answers
what the panes ask instead of building their contents. T9 is split in two for that. And the
interface lives in a small package, `mode`, rather than in `window`: a mode that imported `window`
could not be used by `window`'s own tests, which drive the real modes.*

**How a pane talks to the window.** A pane never imports `window`. Each pane package declares,
as an interface, the little it needs from the window, and `window` implements it. The sidebar,
for instance, asks for the projects, opens a file, and asks whether a file was viewed:

```go
// Window is what the sidebar needs from the window it sits in.
type Window interface {
	Projects() []*workspace.Project
	Open(file workspace.FileReference)
	IsViewed(file workspace.FileReference) bool
	ClassKind(file workspace.FileReference) java.ClassKind
}
```

Today each pane reaches into `App` in 10 to 15 ways; most are reads of state that becomes a
method of one of these interfaces. *Changed while building: the first design did not say where
the pieces every pane shares go (the scrolling list, the keys, a file's reference), nor how a pane
asks the window for something. T8 is split into steps for that.*

**Commands.** A command has a name, a key and the mode it belongs to, such as
`{Name: "Mark as viewed", Key: "v", Mode: "review"}`. The window finds clashes at start and builds
the help from the list, so `handleKeys` and the hand-written help go away.

**Background work.** `window.Run(key, work)` replaces `later()` and the finder's generation
counter: work under a key that is already running cancels it through a `context.Context`. It keeps
today's limit of six readers and the synchronous mode for captures and tests.

**Errors.** Each package declares its codes in `errors.go`, such as
`var CommandNotReadOnly = failure.Define("git.command-not-read-only", "the git command is not on the read-only list")`.
The codes come from Go Renderer's `failure` package: the reader already depends on Go Renderer,
so this adds no dependency and copies nothing. *Changed while building: the plan first moved
Go Engine's `failure` into a module of its own, which would have meant publishing a new
repository; Go Renderer already publishes the same package.*

**Read-only stays where it is.** The whitelist in `git` is the only guarantee today, and it stays
the only one. No write interface or permission layer is added before a mode needs one.

Alternatives not taken:

- Layers by kind (`services/`, `panes/`, `modes/`): a change to reviewing would touch three
  folders instead of one.
- A version-control interface over `git`: there is one implementation, so the interface would be
  a layer with no second use.
- Keeping `app` and splitting it into more files: files don't stop `App` from knowing
  everything; packages do.

## Tasks

Each task leaves the reader working and every test passing.

- [x] T1 — Pin today's behaviour: a test for each shortcut of each mode · `internal/app/keys_test.go` · serves R1
- [x] T2 — Move `main.go` to `cmd/code-review/` · serves R1 · test: `go build ./...`
- [x] T3 — Give `git` and `review` error codes from Go Renderer's `failure` · `errors.go` per package · serves R5 · test: `errors_test.go`
- [x] T4 — Turn `review`'s domain files into `workspace` · serves R4 · test: `TestTheWorkspaceImportsNoInterface`
- [x] T5 — Move the Java tree and class kinds into `java` · serves R1 · test: `TestAJavaTreeMarksRootsAndJoinsPackages`
- [x] T6 — Background work with keys and cancelling · `app/work.go`, which becomes `window` · serves R8 · test: `TestANewerSearchCancelsTheOlderOne`
- [x] T7 — Commands, keys and the help built from them · `app/commands.go`, which becomes `window` · serves R7 · tests: `TestNoTwoCommandsShareAKeyInOneMode`, `TestTheHelpListsEveryCommand`
- [x] T8a — `theme` as a package: looks, file icons, the reader's controls · serves R1 · test: the tests of T1
- [x] T8b — `pane` as a package, and `workspace.FileReference` · serves R1 · test: the tests of T1
- [x] T8c — `viewer` as a package, with the interface it needs from the window · serves R1 · test: the tests of T1
- [x] T8d — `finder` as a package, with the fuzzy search · serves R1 · test: the tests of T1
- [x] T8e — `sidebar` as a package; `app` becomes `window` · serves R1 · test: the tests of T1
- [x] T9a — The `Mode` interface in `mode`; `files` as a package; the sidebar and the viewer ask the mode instead of checking it · serves R3 · test: `TestTheWindowImportsNoMode`
- [x] T9b — `review` as the review mode: marks, progress, overview, `v`; `main` registers both modes · serves R3, R6 · test: `TestMarksSavedBeforeTheSplitAreRead`
- [x] T10 — Describe the packages in `docs/8-contributing.md` · serves R3

## Result

The reader is thirteen packages instead of one `app` of 3,802 lines. The window, the largest, is
1,528 lines and knows neither mode. A third mode is a package and one line in `main`: it gets the
next place on the rail and the next number key.

| Id | Check | Result |
|---|---|---|
| R1 | Today's tests, and one test for every shortcut of each mode | Pass |
| R2 | `TestOnlyReadOnlyCommandsRun` | Pass, unchanged |
| R3 | `TestTheWindowImportsNoMode`, and `TestNoPaneImportsTheWindow` | Pass |
| R4 | `TestTheWorkspaceImportsNoInterface` | Pass |
| R5 | `errors_test.go` in `git`, `review` and `window`; `TestNoErrorIsMadeWithoutACode` | Pass |
| R6 | `TestMarksSavedBeforeTheSplitAreRead` | Pass |
| R7 | `TestNoTwoCommandsShareAKeyInOneMode`, `TestTheHelpListsEveryCommand` | Pass |
| R8 | `TestANewerSearchCancelsTheOlderOne` | Pass |

The plan changed twice while building, as the design says: T8 was split into steps, and a mode
answers what the panes ask instead of building their contents, from its own package, `mode`.

A review of the finished change found what the tests had not. A reading replaced by a newer one
never took back its count, so "Reading the projects…" could stay on. The files mode had lost the
Viewed button, and Esc, Tab, `[` and `]` no longer answered with a modifier held. These are
fixed, each with a test where one fits.

Left for later: a few functions moved as they were are still far over 40 lines (the sidebar's and
the viewer's `paint`, the viewer's header, the finder's content search, the title bar). Splitting
them changes no behaviour and belongs in a change of its own.
