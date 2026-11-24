# 11. An architecture for the long run

> Status: built, pending review · Author's approval: proposal ☑ ("dale", 2026-11-24) plan ☑ ("dale, implementa todo", 2026-11-24)

## Why

Coddy grew feature by feature, and it shows. About 20,000 lines live in 20 packages, but one of
them, `window`, holds 5,400 lines in 32 files. Its `Window` has 48 fields and four concepts: the
language servers, finding names and uses, the projects and their documents, and the shell. Two
files, `viewer.go` and `sidebar.go`, are near 900 lines each.

Coddy should become the editor developers use every day, and the one others want to work on.
For that, a reader must find one concept in each package, each file and each function. Places
where several systems plug in should have one seam, not switches spread over the code. And
nothing should cost a frame what it only costs when its data changes.

Four reviewers with a clean context read the whole code on 24 November 2026, one part each.
Their findings were checked in the code before being written here. This proposal gathers them
into what to fix, in what order, and what stays as it is.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | An idle frame allocates nothing that depends only on data: no copy of the open file, no command table, no labels, counts or badges rebuilt | test: `an idle frame allocates no more than the interface itself` (`testing.AllocsPerRun` on a window with a big file open, in each mode) |
| R2 | No data race and no write outside a project: the server's folders are read under their lock, and a new file under a linked folder is refused when the link leads out | `go test -race ./...`; test: `a new file under a link that leads out of the project is refused` |
| R3 | `window` is the shell only: composition, focus, overlays, keys, the bars. The servers, finding names and uses, and the projects with their documents are each a package of their own, behind small interfaces declared where they are used | `rules_test`: none of them imports `window`; review: `window` has about 25 files at most |
| R4 | Each package and each file is one concept; no file above about 400 lines | review: a package of more than about 25 files, or a file above about 400 lines, is split by concept |
| R5 | A contract asks only what every implementation does: `Mode` without empty methods, the sidebar's and viewer's windows without features of a single caller; a capability is checked once, not each call | review of `mode`, `sidebar`, `viewer`; the Files mode has no method that does nothing |
| R6 | One seam for each thing that will have several kinds: how the viewer shows a file, where the finder looks, how a project's tree is laid out, how a build file is read | tests per kind; adding a kind touches one file and the table that lists it |
| R7 | What Coddy knows of each language is in one table: names, extensions, file names, the server's language id, icon, fence names | test: `every language of the table is highlighted, iconed and served the same way`; no language name outside the table and its kinds |
| R8 | Caches have bounds and know when they are stale: documents by bytes, the server's open files, trees per filter | tests per cache |
| R9 | Errors keep their codes up to the interface, none is dropped without a reason on its line; git and server commands stop when their work is cancelled | review: no `_ =` on an error without a comment; test: `a cancelled search stops its git process` |
| R10 | The technical docs are true and cover how a frame is built, the threads, the caches, the packages and how to add a language, a server, a theme or a mode | read by the author |

## Out of scope

- **Version control behind an interface.** Git is the only one, and every change is told in its
  words. An interface waits for a second kind. The likely first one is a plain folder with no
  version control.
- **Theme sources as a strategy.** There is one format, Zed's, for colours and for icons. A reader
  per format comes with the second format (VS Code, JetBrains).
- **Plugins.** The seams of R6 and R7 are where plugins would plug in, but loading code from
  outside is a proposal of its own.
- **File watching.** A cheap stamp check (time and size) when a cached document is opened comes
  first (R8). Watching needs a dependency (fsnotify) and is the author's call.

## Design

### The findings, by weight

**Must fix now**, because they are bugs or cost every frame:

- `window/servers.go:141`: `serveOpenFile` copies the whole open file each frame before it
  checks that nothing changed. On a file of 3,000 lines that is about 1.1 MB and 3,100
  allocations per frame, about 66 MB of garbage per second at 60 frames. Fix: compare the
  document and the edit version first.
- `lsp/models.go:312`: `anyProjectIn` ranges `client.folders` without its lock, while
  `changeFolder` writes it. This is a data race, and it can stop the program.
- `workspace/save.go:115`: a new file under a linked folder is written through the link, out
  of the project.
- `sidebar/sidebar.go:623`: `window.(LeftOuter)` is asserted unchecked, so another window panics.
- `git/grep.go:52`: every failure of a search is read as "no match".

**Per-frame work that only changes with data** (R1). Measured headless: about 36 allocations
an idle frame in the Files mode, 82 in the Review mode, besides the file copy above.

- `window/modes.go:118`: the command table is rebuilt every frame.
- `review/mode.go:48`: progress is walked about three times a frame.
- `sidebar/paint.go`: each row is decorated twice a frame.
- `sidebar/servers.go:14`: the server badge does a map, a sort and a `Sprintf` per project row.
- `viewer/formatted.go:99`: Markdown joins the whole file to compare it with its cache.
- `uses/graphs.go:114`: the uses graph is rebuilt per frame.
- `sidebar.go:777`: `anyOpen` does a `Sprintf` per row.
- `finder.go:516`: the finder makes two maps per row.
- Crumbs, counts, the branch, line numbers and the measuring of `"M"` are built every frame.

**Resources** (R8, R9):

- `told` keeps every file opened while a server runs, and the viewer never closes them on the
  server.
- `documentCache` is wiped whole at 65 entries, whatever their size.
- The sidebar keeps a tree per filter prefix typed.
- Git runs without a context, so a cancelled search keeps running.
- `Fingerprint` walks the disk although git already listed the files.
- `Content-Length` is trusted whatever its size.

### The target layout

```
main ─▶ window (the shell) ─▶ servers · navigation · session · panes · modes' contracts
                               │           │           │
                               └───────────┴───────────┴─▶ work · lsp · workspace · git · language
```

- **`work`**: today's `window/work.go` as it is: the background jobs and what they give back.
- **`servers`**: today's eleven server files and the 30 fields of `languageServers`, as a `Fleet`.
  It is behind a `Host` the window gives: look, run a job, projects, tell, loading.
- **`navigation`**: going to a definition, the trail back, the hover card, uses across projects.
  It reaches servers through two methods: a client for a path, and wake.
- **`session`**: the projects, their bases, the document cache, what is read and in what order.
  It has no interface, so it can be tested alone.
- **`language`**: the table of R7. It is data, not interfaces: every language does the same things.
- **`lsp`** keeps the protocol and the client. Gradle's import, models and indexes go to a
  `gradle` package that `lsp` calls only for a server that asks for it. The 25 flat fields of
  `Server` are grouped: memory, index, readiness, import.

Arrows only point down. `rules_test` forbids the new packages to import `window`.

### The seams, and why each

A seam is worth it where a second kind already exists or is about to:

| Seam | Kinds today | Branches it removes | Next kind |
|---|---|---|---|
| `viewer.presentation` | whole file, changes, uses | 7 switches on `view.Mode` (29 reads) | blame, side-by-side diff |
| `finder.source` | names, content, references | 11 branches on `Scope` | symbols, commands |
| `workspace.Layout` | plain, Gradle/Maven | the choice in `sidebar.go:172` | Go modules, Cargo |
| `builds.Reader` | Gradle, Maven | the switch in `builds.Read`, 4 lists of build files | `go.mod` |
| `mode` capabilities | Files, Review, Uses | 6 empty methods in Files | any new mode |
| overlays | 7 dialogs | the `ModalOpen` chain, the builds by hand | any new dialog |

Each seam is an interface of a few methods, declared where it is used. An optional power is an
interface of its own, checked once when the thing is set, not at each call.

### Shared pieces, once

- One breadcrumb (`theme.Breadcrumb`), for the viewer and the explorer.
- One `theme.FolderIcon`, the twin of `FileIcon`.
- One key cap, one row background, one step-to-target for lists.
- Paths are joined and split with the standard library's `path`.

### Performance is not traded

No task here costs a frame or memory without saying so. Where one trades a little memory for
speed, the task says how much, and the author approves it:

- Decorations kept per row: a few words per visible row.
- Command table kept: one slice for the program's life.
- Document cache by bytes: a budget the author picks.

Every task of R1 comes with its allocation test, so it stays fixed.

## Tasks

The order keeps every commit green and does the cheap, safe things first. Tests of a behaviour
go in before the code that moves it (the viewer and sidebar have few: about 190 and 94 lines).

- [x] T1 — bugs: the file copy, the race, the link, the unchecked assertion, grep's errors · `window/servers.go`, `lsp/models.go`, `workspace/save.go`, `sidebar`, `git/grep.go` · R1, R2, R9 · tests: `an idle frame with a big file open copies nothing`, `-race`, `a new file under a link that leads out is refused`, `a broken repository's search tells its error`
- [x] T2 — the idle-frame allocation test, and everything it finds: commands, progress, decorations, badges, Markdown, the uses graph, crumbs, counts, metrics · `window`, `review`, `sidebar`, `viewer`, `uses`, `finder` · R1 · test: `an idle frame allocates no more than the interface itself`, in each mode
- [x] T3 — `work` out of `window`; overlays as a list; the command table kept · `internal/work`, `window` · R3
- [x] T4 — `servers` out of `window`, behind `Host`; `told` holds the open file only and closes the last one · `internal/servers` · R3, R8
- [x] T5 — `navigation` out of `window` · `internal/navigation` · R3
- [x] T6 — `session` out of `window`; the document cache by bytes with a stamp check · `internal/session` · R3, R8
- [x] T7 — `language`, the table; every list of extensions and names replaced · `internal/language` · R7
- [x] T8 — `Mode`'s capabilities; `sidebar.Window` and `viewer.Window` trimmed; capabilities resolved once · `mode`, `files`, `review`, `uses`, `sidebar`, `viewer` · R5
- [x] T9 — viewer: tests of each presentation, then `presentation`; `viewer.go` split into layout, navigation, paint · `viewer` · R4, R6
- [x] T10 — sidebar split into rows, expansion, filter, selection, header; trees per filter bounded · `sidebar` · R4, R8
- [x] T11 — `finder.source`; `workspace.Layout`; `builds.Reader`; `gradle` out of `lsp` · R6
- [x] T12 — shared pieces: breadcrumb, folder icon, key cap, row background, `path` · R4
- [x] T13 — errors and context: codes kept to the interface, git and servers cancelled, named limits · R9
- [x] T14 — docs: how a frame is built, threads, caches, the package diagram, how to add a language, server, theme, mode; the rules of `rules_test` · `docs/4-how-it-is-made.md`, `docs/8-contributing.md` · R10

## Decisions for the author

1. Approve this proposal, or change its order or scope.
2. File watching (a dependency) or the stamp check alone.
3. The theme package split in two (`theme` for colours, `icons`): it moves about 10 files and
   every import. It is optional. Answered "yes, split" (2026-11-24): `icons` holds Lucide's kinds, Zed's
   icon themes and their pictures; `theme` keeps the colours and joins them to the icons in
   `FileIcon`, `FolderIcon` and `DrawIcon`, as only it knows the look.
4. The budget of the document cache, in megabytes. Answered (2026-11-24): a setting,
   *Settings › Memory* (a slider from 32 to 512 MB, in steps of 32), 64 MB by default and remembered.

## Result

Built on 24 November 2026, one commit per task, pending the author's review. `gofmt`, `go vet`,
`go test ./...` and `go test -race ./internal/...` pass.

| Task | Commit | What it gave |
|---|---|---|
| T1 | `53fb088` | The five bugs fixed, each with its test |
| T2 | `cce0906` | The idle-frame test; an idle frame went from about 80 and 126 allocations to 34 and 52 (files, review) |
| T3 | `51f1095` | `internal/work`; the overlays are one list, which `ModalOpen` walks |
| T4 | `c966b72` | `internal/servers`, a `Fleet` behind `Host` (`Loading`, `Projects`, `InSight`, `LoadOptions`, `Toast`) |
| T5 | `a668397` | `internal/navigation`: definitions, the trail, the hover card's lines, uses across projects |
| T6 | `bfb87f1` | `internal/session`; documents kept by bytes, least recently opened first, read again when their file's time or size changed |
| T7 | `bcd32b2` | `internal/language`, one table read by the icons, the highlighting, the servers and Markdown |
| T8 | `586ada7` | `mode.Mode` down to what every mode does; its other powers are interfaces found once |
| T9 | `8aded1f` | The viewer's three presentations; `viewer.go` from 1,012 lines to 330 |
| T10 | `8e01030` | `sidebar.go` from 971 lines to 158; trees kept for the last 4 filters |
| T11 | `3de955f`, `99ea7f7` | `finder.source`, `workspace.Layout`, `builds.Reader`; `internal/gradle` out of `lsp`, which keeps the protocol |
| T12 | `9f40eee` | One breadcrumb, folder icon, key cap, row background and list step; `internal/english` |
| T13 | `0cc53a1` | Codes up to the interface, no `_ =` without its reason, long git commands cancelled, `lsp.message-too-large` |
| T14 | `904dc66` | `docs/4-how-it-is-made.md` rewritten, `docs/5-guides/` (a language, a server, a theme, a mode), the package list checked by a test |

`window` has 24 files besides its tests, down from 32. The requirements' checks:

- R1, R8: the idle-frame and cache tests pass; R2: `-race` and the link test pass; R3, R5, R6, R7,
  R9, R10: each has its test beside its package, or in `cmd/code-review/rules_test.go` for the
  arrows. The author chose that tests check only what the program does: the checks of style
  (function length, import groups, `_ =` reasons, names outside the language table, empty mode
  methods, the contributing list) are rules of review.
- R3, R4: no file of the program is above 400 lines and no package holds more than 25: the
  window's small files joined the file of their concept (`805e751`). The author chose to keep
  these sizes as rules of review, not tests: they are how the code is written, not what it does.

**After the review.** A reviewer with a clean context read the whole change. What it found is
fixed, each with its test or rule:

| Finding | Commit |
|---|---|
| Marking a file viewed or a use seen left the ticks and counts stale until another file opened | `f1ad585` |
| No rule for files per package, nor for the imports' groups; `ctx` and `pkg` | `805e751` |
| `.java`, `.kt`, `.kts` and the servers' language ids written outside the table; now a rule | `90283c7` |
| The cancelling test only cancelled before git started; a failed second ask of the uses was silent | `f5e26cf` |
| A save over a link to nothing replaced the link with a file; now `workspace.dangling-link` | `e5bfdb9` |
| The uses mode's idle frame was not measured: 139 allocations, now 61 (budget 65) | `32956f2` |

**What a person still has to review:**

- That a document's size is an estimate from its text, not a measure: the budget of
  *Settings › Memory* (decision 4) counts by it.
- T5 moved "a new note alone does not lay the uses out again" to a fake host: it no longer checks
  that the sidebar itself goes stale, only how many times the uses are shown.
- Seen on screen: T12 gives the finder the settings' raised key caps and the sidebar's focus edge
  on its chosen row; T13 shows an error's code in front of its message in the sidebar
  (`workspace.no-base: …`). The finder and the sidebar were captured in light and dark; the codes
  in the sidebar were not.
- Small behaviour changes: TypeScript, JavaScript, Python and Rust now have a server language, so a
  server configured for them gets their files; `.markdown`, `.mdown` and `.mkd` are Markdown; a
  file that stopped changing maps its rows to lines in the changes view; a newer reading of the
  projects stops the git of the one it replaces.
- Not done: the servers' views are made each frame while a server runs (about 9 allocations, a
  `TODO(akrck02)` in `idle_test.go`); the uses mode still checks its `uses.Host` at each call; two readings of projects (a
  server's snapshot, `notCommitted`) cannot be stopped yet.
