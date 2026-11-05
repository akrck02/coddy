# 2. Keep every function short

> Status: building · Author's approval: proposal ☑ plan ☑

## Why

The guidelines ask for functions of about 40 lines at most: a longer one is usually doing two
jobs. Splitting the reader into packages (proposal 1) moved the code as it was, and 22 functions
are still longer. The longest, the sidebar's `paint`, is 122 lines and draws three kinds of row at
once. A newcomer has to read all of it to change one row. Nothing checks the rule, so it will be
broken again.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | The reader behaves and looks exactly as before. | Every test passes, and captures of both modes are the same pixels as before (T1, T8). |
| R2 | No function is longer than 40 lines, tests included. | `TestNoFunctionIsLongerThanFortyLines`, which reads every Go file with `go/ast`. |
| R3 | Each new function does one job, and its name says which. | The reviewer's pass at the end. |

## Out of scope

- Any change in behaviour or looks, new options, new packages.
- Renaming exported names, or moving code between packages.
- Rewriting history to remove old names from earlier commits.

## Design

Each long function is split by the parts it already has, which today are marked by comments or
by a `switch`:

| Function | Lines | Split into |
|---|---|---|
| `sidebar.paint` | 122 | the row's background, then one function per kind of row: project, folder, file |
| `viewer.paint` | 100 | the gutter, a diff row, a line of the whole file, the fold |
| `viewer.buildHeader` | 87 | the file's path, how it changed, the buttons |
| `finder.searchContent` | 65 | which projects to search, the search, putting the hits in order |
| `window.buildTitleBar` | 63 | the workspace, the search field, the comparison's buttons |
| `finder.Build` | 55 | the keys, the panel |
| `highlight.RoleOf` | 53 | a table from token type to role, read by a short function |
| `window.buildBaseDialog` | 51 | the field, the suggestions, the buttons |
| `sidebar.Build` | 50 | the header, the pointer's clicks |
| `sidebar.paintRight` | 49 | the common part, what the mode adds |
| `theme.look` | 48 | Bubble's shapes, the theme's colours, the reader's own colours |
| `sidebar.Rebuild` | 47 | listing every project, listing the project entered |
| `workspace.LoadProject` | 45 | finding the base, reading the changes |
| `main` | 53 | reading the flags, making the reader, running the window |
| five between 41 and 44 | | one part each taken out |

The tests that list every shortcut keep their tables, but as functions that return the cases
(`windowShortcuts()`), so the test itself is a short loop.

`TestNoFunctionIsLongerThanFortyLines` goes in `cmd/code-review/rules_test.go` beside the other
rules. It uses the standard library's `go/parser`, so it adds no dependency. It names each
function over the limit with its file and line.

To prove nothing moved on screen, a script draws one frame of each mode with `-capture`, before
and after, on a small repository it makes, and compares the pictures.

## Tasks

- [x] T1 — Captures of both modes, light and dark, before any change · serves R1 · test: the pictures
- [x] T2 — `sidebar`: `paint`, `Build`, `paintRight`, `Rebuild`, `buildEnteredHeader` · serves R2, R3 · test: the tests of T1 of proposal 1
- [x] T3 — `viewer`: `paint`, `buildHeader` · serves R2, R3 · test: the same
- [ ] T4 — `finder`: `searchContent`, `Build` · serves R2, R3 · test: the same
- [ ] T5 — `window` and `main`: the title bar, the base dialog, the status bar, `Build`, `main` · serves R2, R3 · test: the same
- [ ] T6 — The rest: `RoleOf`, `look`, `LoadProject`, `buildOverviewRow`, `parseUnifiedDiff` · serves R2, R3 · test: the same
- [ ] T7 — The shortcut tests' tables as functions · serves R2 · test: they still pass
- [ ] T8 — `TestNoFunctionIsLongerThanFortyLines`, and the captures compared with T1 · serves R1, R2

## Result

Not built yet.
