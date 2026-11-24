# 11. An architecture for the long run

> Status: proposed · Author's approval: proposal ☐ plan ☐

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
| R3 | `window` is the shell only: composition, focus, overlays, keys, the bars. The servers, finding names and uses, and the projects with their documents are each a package of their own, behind small interfaces declared where they are used | `rules_test`: none of them imports `window`; `window` has at most 25 files |
| R4 | Each package and each file is one concept; no file above about 400 lines | `rules_test`: a package has at most 25 files; a list of files above 400 lines that only shrinks |
| R5 | A contract asks only what every implementation does: `Mode` without empty methods, the sidebar's and viewer's windows without features of a single caller; a capability is checked once, not each call | review of `mode`, `sidebar`, `viewer`; the Files mode has no method that does nothing |
| R6 | One seam for each thing that will have several kinds: how the viewer shows a file, where the finder looks, how a project's tree is laid out, how a build file is read | tests per kind; adding a kind touches one file and the table that lists it |
| R7 | What Coddy knows of each language is in one table: names, extensions, file names, the server's language id, icon, fence names | test: `every language of the table is highlighted, iconed and served the same way`; no language name outside the table and its kinds |
| R8 | Caches have bounds and know when they are stale: documents by bytes, the server's open files, trees per filter | tests per cache |
| R9 | Errors keep their codes up to the interface, none is dropped without a reason on its line; git and server commands stop when their work is cancelled | `rules_test`: no `_ =` on an error without a comment; test: `a cancelled search stops its git process` |
| R10 | The technical docs are true and cover how a frame is built, the threads, the caches, the packages and how to add a language, a server, a theme or a mode | read by the author; `rules_test` checks the package list of `docs/8-contributing.md` against the tree |

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

- [ ] T1 — bugs: the file copy, the race, the link, the unchecked assertion, grep's errors · `window/servers.go`, `lsp/models.go`, `workspace/save.go`, `sidebar`, `git/grep.go` · R1, R2, R9 · tests: `an idle frame with a big file open copies nothing`, `-race`, `a new file under a link that leads out is refused`, `a broken repository's search tells its error`
- [ ] T2 — the idle-frame allocation test, and everything it finds: commands, progress, decorations, badges, Markdown, the uses graph, crumbs, counts, metrics · `window`, `review`, `sidebar`, `viewer`, `uses`, `finder` · R1 · test: `an idle frame allocates no more than the interface itself`, in each mode
- [ ] T3 — `work` out of `window`; overlays as a list; the command table kept · `internal/work`, `window` · R3
- [ ] T4 — `servers` out of `window`, behind `Host`; `told` holds the open file only and closes the last one · `internal/servers` · R3, R8
- [ ] T5 — `navigation` out of `window` · `internal/navigation` · R3
- [ ] T6 — `session` out of `window`; the document cache by bytes with a stamp check · `internal/session` · R3, R8
- [ ] T7 — `language`, the table; every list of extensions and names replaced · `internal/language` · R7
- [ ] T8 — `Mode`'s capabilities; `sidebar.Window` and `viewer.Window` trimmed; capabilities resolved once · `mode`, `files`, `review`, `uses`, `sidebar`, `viewer` · R5
- [ ] T9 — viewer: tests of each presentation, then `presentation`; `viewer.go` split into layout, navigation, paint · `viewer` · R4, R6
- [ ] T10 — sidebar split into rows, expansion, filter, selection, header; trees per filter bounded · `sidebar` · R4, R8
- [ ] T11 — `finder.source`; `workspace.Layout`; `builds.Reader`; `gradle` out of `lsp` · R6
- [ ] T12 — shared pieces: breadcrumb, folder icon, key cap, row background, `path` · R4
- [ ] T13 — errors and context: codes kept to the interface, git and servers cancelled, named limits · R9
- [ ] T14 — docs: how a frame is built, threads, caches, the package diagram, how to add a language, server, theme, mode; the rules of `rules_test` · `docs/4-how-it-is-made.md`, `docs/8-contributing.md` · R10

## Decisions for the author

1. Approve this proposal, or change its order or scope.
2. File watching (a dependency) or the stamp check alone.
3. The theme package split in two (`theme` for colours, `icons`): it moves about 10 files and
   every import. It is optional.
4. The budget of the document cache, in megabytes.

## Result

Not built yet.
