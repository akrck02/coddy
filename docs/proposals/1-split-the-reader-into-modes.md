# 1. Split the reader into packages, one per mode

> Status: proposed · Author's approval: proposal ☑ plan ☐

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
| R2 | The reader still never writes to a repository it reviews. | `TestOnlyReadOnlyCommandsRun` passes unchanged. |
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
internal/theme/           Zed One colours, light and dark, and the icon of each file type
internal/window/          title bar, mode rail, status bar, layout, commands and keys,
                          background work
internal/viewer/          the code view both modes share: rows, scrolling, gutter, painting
internal/sidebar/         the tree pane; each mode says which rows it shows
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
| `app/editor.go`, `app/scroller.go` | `viewer` |
| `app/sidebar.go` | `sidebar` |
| `app/theme.go`, `app/fileicons.go` | `theme` |
| `app/app.go`, `build.go`, `input.go` | `window` |

**Modes.** `window` declares what a mode is; `files` and `review` implement it; `main` registers
them. `window` imports no mode, and the panes import `theme` instead of `window`, so the arrows only
point down: modes, then `window`, then the panes, then `theme`, `workspace` and `git`.

```go
// Mode is one way of looking at the workspace, chosen from the mode rail.
type Mode interface {
	Name() string
	Icon() string
	Rows(workspace *workspace.Workspace) []sidebar.Row
	Show(document *workspace.Document) viewer.Presentation
	Commands() []Command
}
```

**Commands.** A command has a name, a key and the mode it belongs to, such as
`{Name: "Mark as viewed", Key: "v", Mode: "review"}`. The window finds clashes at start and builds
the help from the list, so `handleKeys` and the hand-written help go away.

**Background work.** `window.Run(key, work)` replaces `later()` and the finder's generation
counter: work under a key that is already running cancels it through a `context.Context`. It keeps
today's limit of six readers and the synchronous mode for captures and tests.

**Errors.** Each package declares its codes in `errors.go`, such as
`var CommandNotReadOnly = failure.Define("git.command-not-read-only", "the git command is not on the read-only list")`. Go Engine already
has `failure`; the rules ask for one shared module instead of a copy, so `failure` moves to its
own module that both use. That module is a new dependency and needs the author's yes.

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

- [ ] T1 — Pin today's behaviour: a test for each shortcut of each mode · `internal/app/app_test.go` · serves R1
- [ ] T2 — Move `main.go` to `cmd/code-review/` · serves R1 · test: `go build ./...`
- [ ] T3 — Move `failure` to its own module and give `git` and `workspace` error codes · `errors.go` per package · serves R5 · test: `errors_test.go`
- [ ] T4 — Turn `review`'s domain files into `workspace` · serves R4 · test: `TestTheWorkspaceImportsNoInterface`
- [ ] T5 — Move the Java tree and class kinds into `java` · serves R1 · test: `TestAJavaTreeMarksRootsAndJoinsPackages`
- [ ] T6 — Background work with keys and cancelling · `window/work.go` · serves R8 · test: `TestANewerSearchCancelsTheOlderOne`
- [ ] T7 — Commands, keys and the help built from them · `window/commands.go` · serves R7 · tests: `TestNoTwoCommandsShareAKeyInOneMode`, `TestTheHelpListsEveryCommand`
- [ ] T8 — `viewer`, `sidebar` and `finder` as packages · serves R1 · test: the tests of T1
- [ ] T9 — The `Mode` interface; `files` and `review` as packages · serves R3, R6 · tests: `TestTheWindowImportsNoMode`, `TestMarksSavedBeforeTheSplitAreRead`
- [ ] T10 — Describe the packages in `docs/8-contributing.md` · serves R3

## Result

Not built yet.
