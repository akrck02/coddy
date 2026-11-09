# 7. A mode for uses, with their graph and the graph of repositories

> Status: done · Author's approval: proposal ☑ plan ☑

## Why

Proposal 6 finds the uses of a name across repositories, but lists them in the finder: a small
popup of lines, gone with Esc, with no code around each use. A review that asks "what does this
change break?" needs to go through every use calmly, read each one in its code, come back, and see
at a glance which repositories are touched and how they hang together. The author asked for a
screen of its own, comfortable to use, with a dependency graph.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | A third mode, **Uses**, in the rail next to Review and Files. `⇧F12` opens it on the uses of the name at the caret; it keeps the last search until another is made. | A window test: ⇧F12 turns the mode on, with the uses. |
| R2 | Its sidebar lists the files with uses as the other modes list files: by project, then folders and packages, each file with its count; projects in the order of proposal 6 (the name's own, those with changes, the others). | A sidebar test. |
| R3 | Opening a file shows it with every use marked, the lines away from them folded as Review folds lines away from changes; the file stays editable. | A viewer test. |
| R4 | `n` and `⇧N` go to the next and previous use, across files, as `n` goes through changes in Review; a use can be ticked as seen (`v`), and the header says "12 of 30 seen". | Window tests. |
| R5 | The list fills in as proposal 6 checks places, and says how far it is; a search can be asked again (`r`). | A window test with the fake server busy. |
| R6 | **Graph of the name**: the name in the middle, the files that use it around it grouped in their projects, an arrow from each user to the name. A click opens the file at its use; a node can be opened one level further (who uses that file's class or function), on demand. | A test of the layout, and one of the click. |
| R7 | **Graph of repositories**: every project in sight as a node, an arrow to each project it depends on, read from its Gradle (and Maven) build files; projects with changes marked, and those that use the name, when a search was made, lit. | Tests of the build-file reading on small builds, and of the layout. |
| R8 | The view switches between list, graph of the name and graph of repositories with `1`, `2`, `3` or the tabs over the viewer; the graphs zoom and pan (wheel, drag, `0` to fit). | Window tests. |

## Out of scope

- Call hierarchies deeper than one level opened by hand at a time.
- Dependencies that are neither in Gradle nor Maven build files (a jar copied by hand).
- Rename across the uses.

## Design

### The mode

`internal/uses` is a mode like `review` and `files` (`mode.Mode`): one package, one line in main.
It holds the search of proposal 6 (moved there from the window) and the uses found, by file. As a
`sidebar.Listing` it lists only the files with uses, with their counts as decorations, and ticks
(`v`) as Review ticks files viewed. The viewer shows a file of the mode with its uses as marked
lines, the others folded by the same code that folds Review's unchanged lines; the word used is
washed as a change is.

### The graph of the name

Nodes: the name, each project with uses (a box), each file in it (a node in the box), with the
count. Laid out in rings: the name's own project nearest, then those with changes, then the
others, each box sized to its files. Drawn with go-ui `Paint`, as the sidebar's badges are. Opening
a node one level further asks its file's server for the symbol around the use
(`textDocument/documentSymbol`), then for that symbol's uses as proposal 6 does, and adds them as
a ring outside it.

### The graph of repositories

A new package, `internal/builds`, reads each project's `settings.gradle(.kts)`, `build.gradle(.kts)`
and `pom.xml`: the name and group it publishes as, and the coordinates it depends on
(`implementation("group:artifact:version")`, `project(":module")`, `<dependency>`). A dependency
whose group and name a project in sight publishes is an arrow to it. It is text reading, not
running Gradle: fast, with nothing written, and enough for the plain declarations the big workspace uses; what it
cannot read (a coordinate built in code) is listed under the graph as "not understood". Laid out in
layers, the projects nothing depends on at the top (Sugiyama-style, as small graphs need).

## Tasks

- [x] T1 The Uses mode: the search moved from the window, the sidebar listing with counts, ⇧F12 opens it. (R1, R2, R5)
- [x] T2 The viewer marks the uses and folds the rest; `n`/`⇧N`, `v` and the header. (R3, R4)
- [x] T3 The views and their keys; zoom and pan for graphs. (R8)
- [x] T4 The graph of the name, its clicks, and opening a node a level further. (R6)
- [x] T5 `internal/builds` and the graph of repositories. (R7)
- [x] T6 Try it on the big workspace: the `LampStore` case, a repository graph of the 44; README and Result.

## Notes from building it

- The views are `⇧1`, `⇧2`, `⇧3`, not `1`, `2`, `3`: those already switch modes. A mode's own keys
  (`n`, `v`, `r`, `0`, the views) come first while it is on, so `n` goes through uses, not changes.
- The search stays in the window, which knows the servers; the mode receives the uses as they are
  found (`mode.UsesHolder`) and asks the window to search again. Not "moved there", as the design
  said, but the same split as Review and git.
- A file opens on its uses, folded; editing it is in the whole file (`d`), as in Review.
- Opening a node further asks only that file's own server: the declaration the use is in, from
  `textDocument/documentSymbol`, then that declaration's references. It asks about the use `n`
  went to, if it is in that file, or else the file's last one: the first is most often an import,
  in no declaration. The files found go in a column beside the file, not a ring: rings crossed the
  other side's arrows. A `⌘`-click opens it, a plain click still opens the file.
- The graph of the name lays the projects on either side, alternating, nearest first, instead of
  rings: a column of boxes reads top to bottom, and labels do not turn.
- The build reading, on the big workspace: 44 projects, 130 links, 7 declarations not understood (version
  catalogs and coordinates made in code).

## Result (T6, on the big workspace, 9 November 2026)

44 repositories, 23 with changes. The author tried the mode in the window; then a scripted run, with
the real servers, searched the uses of a Kotlin class of a data-access repository (`LampStore`,
declared in Kotlin, used by its test and by a Java configuration class of another repository).

| | |
|---|---|
| Projects read | 11–15 s |
| Uses of `LampStore` | 37–46 s, with jdtls reading and kotlin-lsp importing: 3 uses in 2 files, listed by project, counted, folded |
| Graph of the name | the name, its project, its two files with their counts |
| ⌘-click on the test file | `prepareLampStore` (the method the use is in), in 33 s; no other user (kotlin-lsp was still importing) |
| Graph of the repositories | 44 projects, 130 links, 7 declarations not understood |
| After the window closed (`⌃C`) | no server and no Gradle daemon left |

- **A use missed**: the Java configuration class of another repository that builds a `LampStore`.
  The candidate was found by the text search, but jdtls answers no definition there, though the
  class is in the jar that repository depends on. Most likely jdtls (Buildship) puts the open
  repository that publishes the jar in its place, and that repository's code is Kotlin, which
  jdtls does not read; not checked. So a Kotlin name used from Java in another repository is lost.
- **Disk** (proposal 8): kotlin-lsp's folder grew from 1.2 to 12 GB, and the disk's free space from
  62 to 47 GB. Two indexes, 5.5 and 2.4 GB, one per set of projects: the window read the 23 with
  changes, the run all 44, and each set is imported anew; and 4.2 GB of Gradle caches in its own
  Gradle home (each Gradle version's generated API and transformed jars), which the read-only
  dependency cache does not spare. The cold import of proposal 8 T7, seen at last.
- Found by trying it: a note (toast) showed for one frame, gone with the next move of the mouse,
  and a long signature ran out of the hover card and the window. Both fixed.
