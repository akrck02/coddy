# How it is made

| Package | |
|---|---|
| `cmd/code-review` | The program: flags and wiring |
| `internal/git` | The read-only git commands, and reading their output |
| `internal/highlight` | Chroma's tokens, cut into lines of coloured pieces |
| `internal/language` | The table of languages: names, extensions, the server's name for each, icon, Markdown fences |
| `internal/workspace` | Projects, changes, a file's two sides and its diff, folding, the tree, what servers wrote |
| `internal/text` | A file being edited: lines, caret, selection, undo |
| `internal/lsp` | Language servers: JSON-RPC over their input and output, the table of servers |
| `internal/gradle` | What servers that import Gradle builds need: their own Gradle home and its daemons, the builds they cannot import, the models kept between starts |
| `internal/builds` | What a project's builds publish and depend on, read as text: one reader per build system (Gradle, Maven) |
| `internal/java` | The IntelliJ-style tree and the kinds of class |
| `internal/theme` | Zed's One colours, file icons and the reader's own controls |
| `internal/english` | A count of things in the singular or the plural, a sentence's first capital |
| `internal/pane` | What every pane shares: the keys and a scrolling list |
| `internal/viewer`, `internal/sidebar`, `internal/finder`, `internal/explorer` | The panes |
| `internal/work` | Background jobs, and what they give back to the frame |
| `internal/servers` | The language servers a window runs: started for the file open, told its edits, rested when unasked, installed |
| `internal/navigation` | Following names: where one is declared, the trail back, what a server says of it, its uses across projects |
| `internal/session` | What is open: the folders, their projects and bases, and the documents read, kept up to 64 MB and read again when their file changes on disk |
| `internal/window` | The window: layout, commands and dialogs |
| `internal/mode` | What a mode is, and what it asks of the window |
| `internal/files`, `internal/review` | The two modes: read any file; review the changes, marking each viewed |

How the packages talk to each other, how to add a mode and how to propose a change:
[the contributing guide](8-contributing.md).

go-ui describes the window again at every frame. The sidebar, the code and the finder's results
are painted by the program (`Context.Paint`) as lists that only draw the rows in sight, with their
own scrolling, so the keyboard can move them; go-ui's window hands the program the keys through
its `Driver`. Repositories and files are read on other goroutines, and the window wakes when they
are ready.

Bubble 4 is still on its `feature/v4` branch and its `goui` module points at its neighbours
through local paths, so `go.mod` replaces Bubble's and go-ui's `v0.0.0` with real versions.
