# 7. A mode for uses, with their graph and the graph of repositories

> Status: approved · Author's approval: proposal ☑ plan ☑

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

- [ ] T1 The Uses mode: the search moved from the window, the sidebar listing with counts, ⇧F12 opens it. (R1, R2, R5)
- [ ] T2 The viewer marks the uses and folds the rest; `n`/`⇧N`, `v` and the header. (R3, R4)
- [ ] T3 The views and their keys; zoom and pan for graphs. (R8)
- [ ] T4 The graph of the name, its clicks, and opening a node a level further. (R6)
- [ ] T5 `internal/builds` and the graph of repositories. (R7)
- [ ] T6 Try it on the big workspace: the `LampStore` case, a repository graph of the 44; README and Result.
