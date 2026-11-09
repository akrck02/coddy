# Code review

**A small, fast code reader for developments that change many repositories at once.**
Open a folder of projects, see which ones a development changed against their base branch, and
read each changed file's diff, coloured, without leaving the keyboard. Fix what you find in the
file as you read it, or switch to read-only and only read.

Built with [go-ui](https://github.com/akrck02/go-ui) over
[go-renderer](https://github.com/akrck02/go-renderer), dressed in
[Bubble 4](https://github.com/akrck02/bubble)'s shapes and spacing (compact), in Zed's One
colours, light and dark; code is coloured with [chroma](https://github.com/alecthomas/chroma),
as papiro-indexer colours its code blocks.

> Bubble 4 is still on its `feature/v4` branch and its `goui` module points at its neighbours
> through local paths, so `go.mod` replaces Bubble's and go-ui's `v0.0.0` with real versions.

## 🤖 AI disclosure

- **Written by an AI assistant (Claude, by Anthropic) under my direction, in November 2026:** all
  of it so far.

Commits made with AI carry a `Co-Authored-By: Claude` line. **Status: pending thorough review.**

## What it does

It has two modes, chosen on the rail at the far left (or with `1`, `2`, `c`):

- **Files** (the default) is for reading code: every file of every project in a tree, the changed
  ones named in colour (green added, amber modified, red deleted), and a file opens whole with its
  changes marked beside the line numbers, as Zed marks them: a bar for lines added or changed, a
  wedge where lines were removed.
- **Review** is for going through a development: only the changed files, each with the lines it
  adds and removes, opened as diffs. `v` marks the file as viewed and opens the next one not viewed;
  each project shows how much of it was viewed, and the status bar the whole. A mark is kept by the
  file's text, so a file that changes again is shown as not viewed (the marks live in
  `~/Library/Application Support/code-review/reviewed.json`, never in the repositories). With no
  file open, the review mode shows an overview of the projects: branch against base, commits,
  files, lines, progress.

- **Opens several repositories at once.** Give it repositories, or folders of repositories side by
  side (a workspace of twenty projects), on the command line or with `o`. The folders are
  remembered for the next start.
- **Shows what the development changed in each project**: added, modified, deleted and renamed
  files, measured from where the branch left its base (`git merge-base`), so what landed on the base
  since is not counted. The base is each project's `origin/HEAD`, or the first of `origin/main`,
  `origin/master`, `main`, `master`, `develop`; `b` (or `-base`) chooses another branch, tag or
  commit for one project or for all of them. Changes not committed yet count too, unless `w` (or
  `-committed`) leaves them out.
- **Reads a changed file as a diff**, both sides interleaved with their line numbers, the unchanged
  lines folded away (`f` shows them all, Enter or a click opens one fold), and a map of the changes
  along the scroll bar. Inside a replaced line the words that changed are washed stronger. `d`
  switches to the whole file.
- **Only the changes**, by default: the projects a development did not touch are left out of the
  sidebar, the finder and the language servers, and the *Files* mode lists only the changed files,
  with its folders left as you opened them. `g` (or `h`), the button over the sidebar or `-all`
  show every project and file; the choice is remembered.
- **Every other file stays readable**: the *Files* mode lists them all, and the finder finds them.
- **Enter a project** to see only its tree, from its top (Enter on its row, `e`, or the → that
  shows on its row under the pointer); `Esc` goes back to every project.
- **Java and Kotlin projects shown as IntelliJ shows them**: Gradle and Maven modules marked,
  source roots blue (`src/main/java`, `kotlin`…), test roots green, resources apart, the packages
  inside a root joined with dots (`com.example.lighthouse.lamp`), and classes named without their
  extension with a badge for their kind (C class, I interface, E enum, @ annotation, R record, O
  Kotlin object), read from the files as they come into sight.
- **Icons by file type** in the tree and the finder, tinted with the code's own colours.
- **Syntax colouring** for Java, Kotlin, Vue, SQL, Gradle, Groovy, XML, YAML, Markdown and the
  rest of chroma's languages; text that is not UTF-8 is read as Latin-1.
- **Edits a file opened whole**: a caret, a selection (Shift, a drag, a double click), undo by
  word, cut, copy and paste. The file is coloured again a moment after you stop typing. `⌘S`
  writes the whole file at once, and refuses to overwrite one changed on disk since it was read: it
  asks whether to keep yours or take theirs. A dot after the name marks edits not saved, and
  opening another file or quitting asks first. A saved file shows as changed at once. The diff
  stays for reading.
- **Follows names with language servers**: `gopls` for Go, `jdtls` for Java and `kotlin-lsp` for
  Kotlin (JetBrains' preview, `brew install JetBrains/utils/kotlin-lsp`), found on
  the `PATH` and started the first time a file of their language opens. `F12` or `⌘`-click goes
  to where a name is declared, even in another project, and `⌃-` comes back; `⇧F12` opens the
  uses mode on every use of it (below); resting the pointer on a name shows its
  type and documentation. A server not installed is named, with the command that installs it.
  One `jdtls` reads every Java project of the window, and one `kotlin-lsp` every Kotlin one, so
  names cross repositories; a name another repository gives as a jar opens in that repository's
  source when it is open. While a server reads the projects (`kotlin-lsp` imports each Gradle
  build, minutes the first time) the status bar says so, and a question waits for it. Servers
  start once the projects are read, and read only the projects in sight.
- **A mode for the uses of a name** (`⇧F12`, or `3` once a search was made): the sidebar lists only
  the files that use it, by project, the name's own first, then those with changes, each with its
  count. A file opens with its uses marked and the lines away from them folded, still editable
  (`d` shows it whole). `n` and `⇧N` go from use to use across files, `v` ticks one as seen, and
  the header counts them; `r` searches again. Over the viewer, three views (`⇧1` `⇧2` `⇧3`):
  - the list of uses, in the files;
  - the graph of the name: the name in the middle, the projects that use it on either side, each
    file with its count. A click opens the file at its use; a `⌘`-click asks that file's server
    for the method or class the use is in, and lays out beside it the files that use that;
  - the graph of the repositories: each project, an arrow to each it depends on, read from its
    Gradle and Maven build files as text (nothing is run). Changed projects are edged, those that
    use the name lit; a dependency declared in a way not read (a version catalog, a coordinate
    made in code) is counted under it.

  The graphs zoom with the wheel, move with a drag, and `0` fits them in again.
- **Servers that leave nothing behind**:
  - Indexes go in the reader's own folder, or in the one `-server-data` names (another disk; the
    choice is remembered). `kotlin-lsp` builds one index per set of projects, about 5 GB for 44
    repositories, so it reads every Kotlin project of the window, whatever the filter shows, and
    imports them once. The reader keeps the index used last (`-keep-indexes` keeps more), and
    does not start it on a disk with less than 10 GB free, saying why.
  - `jdtls` writes nothing in the repositories: its `.project`, `.classpath` and `.settings/` go in
    its data folder, and it builds nothing into `bin/` or `target/`.
  - `kotlin-lsp` imports with a Gradle home of its own, in its data folder, holding only its
    daemons and their settings: its caches, Gradle versions and JDKs are yours, linked, so
    nothing is downloaded or made twice. Its Gradle daemons stop two minutes after their last
    build, and with the server, never touching yours.
  - Each Java server takes at most 2 GB of memory (`-server-memory` changes it).
  - A server nobody asks anything for 15 minutes rests, giving its memory back; following a name
    or asking its uses starts it again, from its index. The servers panel shows what each takes,
    memory and index, and stops one.
- **What a server writes is not your change**: `kotlin-lsp`'s Gradle import writes `.gradle/` and
  `build/`, as a build does, and `jdtls` wrote `.project` and `.classpath` before it was told not
  to. The reader notes what each project held before a server started in it, and a file
  that appears or changes afterwards, which you did not save in the reader, leaves the review, the
  counts and the progress. A dashed badge on the project, `kotlin-lsp · 4`, lists those files to open
  them, and the reader remembers them after a restart. Edits a server asks to make
  (`workspace/applyEdit`) are refused.
- **A read-only mode**: the lock in the title bar, `⌘⇧L` or `-read-only` turn editing off, and
  the choice is remembered.
- **Writes a file only when you save it; git only reads.** Language servers may write their own
  files, shown apart as above. It only runs `git cat-file`, `diff`,
  `ls-files`, `merge-base`, `rev-list`, `rev-parse` and `symbolic-ref` (any other command is
  refused by `internal/git/run.go`), without the optional locks, the file system monitor or
  external diff programs: no commit, no push, no checkout. It never fetches, so a remote branch is
  as fresh as the last `git fetch` you ran.

## Build and run

You need Go 1.25 or later and the Xcode command line tools (`xcode-select --install`): the window
is opened through cgo and drawn with Metal.

```bash
go build -o bin/code-review ./cmd/code-review
./bin/code-review /path/to/workspace                 # a folder of repositories
./bin/code-review ../api ../web                      # or the repositories themselves
./bin/code-review -base origin/release/2.4 /path/to/workspace
```

| Flag | |
|---|---|
| `-base ref` | Compare every project against this branch, tag or commit |
| `-committed` | Compare the last commit only, leaving out changes not committed yet |
| `-open project/path` | Open a file at the start |
| `-review` | Start in the review mode |
| `-find text` | Start with the finder searching the files' content for a text |
| `-code-font file.ttf` | Read the code in this TrueType face (default SF Mono, from macOS; Go Mono where there is none) |
| `-all` | Show every project, not only those with changes |
| `-read-only` | Start read-only: files are only read, not edited |
| `-light` | Start in the light theme (dark by default) |
| `-width`, `-height`, `-capture shot.png` | go-ui's window flags; `-capture` draws one frame to a picture |

```bash
go vet ./... && go test ./...     # the interface tests drive the window with no display
```

## Keys

While you edit a file, letters type: commands then take the key with `⌘` (or `⌃`), shown second
below, an F-key or `Esc`. `Esc` gives the keyboard back to the sidebar, where the letters work.

| Key | |
|---|---|
| `⌘P` (or `Shift+P`) | Find a file by its name, in any project; changed files first |
| `⌘⇧F` (or `/`) | Find a text in the files of every project; `Tab` in the finder switches between names and content |
| `n` / `m` (`⌘⇧N` / `⌘⇧M`) | Next / previous changed file, across every project |
| `]` / `[` (`⌘]` / `⌘[`) | Next / previous change in the file (the diff, or the marked lines) |
| `1` / `2` / `3` (`⌘1` / `⌘2` / `⌘3`; `c` or `⌘0` switches) | Files mode / Review mode / Uses mode |
| `Enter` on a project, `e` (`⌘E`), or its → button | Enter the project: the sidebar lists only its tree, and `n`/`m`/`v` stay in it |
| `Esc` / `Backspace` | Back to every project |
| `v` (`⌘⇧V`) | Mark the file as viewed (or not); marking it opens the next file not viewed |
| `d` (`⌘D`) | Diff, or the whole file |
| `f` (`⌘U`) | Fold the unchanged lines, or show them all |
| `Tab` (`⌃Tab`) | Move between the sidebar and the editor |
| `↑ ↓ ← →`, `PgUp PgDn`, `Home End` | Move in the sidebar (←/→ close and open folders), or scroll the file |
| `j` / `k`, `Space` | Scroll a line, a page |
| `Enter` | Open a file or a folder (sidebar), open a fold (editor) |
| `g`, `h` (`⌘⇧G`) | Only the changes, projects and files, or everything (sidebar, finder and servers) |
| `b` (`⌘B`) | Compare against another branch, tag or commit |
| `w` (`⌘⇧W`) | Count the changes not committed yet, or the commits only |
| `r` (`⌘R`) | Read every project again |
| `o` (`⌘O`) | Open a repository or a folder of repositories |
| `y` (`⌘⇧C`) | Copy the open file's path |
| `t` (`⌘⇧T`) | Light or dark |
| `l` (`⌘⇧L`) | Read-only, or editing |
| `F12`, or `⌘`-click | Go to where the name at the caret (or clicked) is declared |
| `⇧F12` | Every use of the name at the caret (or clicked), in the uses mode |
| `⌃-` / `⌃⇧-` | Back to where you were before following a name, and forward again |
| `?` (`F1`) | These keys |

In the uses mode, its keys come first:

| Key | |
|---|---|
| `n` / `⇧N` | Next / previous use, across files |
| `v` | Tick the use as seen, and go to the next not seen |
| `r` | Search the uses again |
| `⇧1` / `⇧2` / `⇧3` | The list of uses / the graph of the name / the graph of the repositories |
| `0` | Fit the graph in the window |
| click / `⌘`-click on a file of the graph | Open it at its use / who uses the method or class that use is in |

Editing a file:

| Key | |
|---|---|
| `⌘S` | Save |
| `⌘Z` / `⌘⇧Z` | Undo / redo |
| `⌘X` `⌘C` `⌘V`, `⌘A` | Cut, copy, paste, select all |
| Arrows (`Shift` selects), `⌥←` `⌥→` | Move by a character, a word |
| `⌘←` `⌘→` (or `Home` `End`), `⌘↑` `⌘↓` | The start or end of the line, of the file |
| `⌥Backspace` | Delete the word before the caret |
| `Esc` | Stop editing: the sidebar takes the keyboard |

## Design

The interface follows what review tools have settled on, and keeps colour for meaning:

- **Colours are Zed's One Dark and One Light** ([one.json](https://github.com/zed-industries/zed/blob/main/assets/themes/one/one.json)),
  for the window and the code alike: neutral greys, the editor a step apart from the panels around
  it, green, amber and red only for added, changed and removed, blue only for where you are.
  Bubble 4 gives the shapes, spacing and type.
- **Changes marked in the gutter** while reading a whole file, and a map of them along the scroll
  bar, as [Zed's git integration](https://zed.dev/docs/git.html) does.
- **Words that changed inside a line** are marked over the line's wash, as most diff tools do; the
  `+` and `−` signs stay, so the diff reads without telling red from green
  ([Review Board's colour-blind mode](https://reviews.reviewboard.org/r/10981/)).
- **Viewed files and progress**, as GitHub's *Viewed* box and Reviewable: a large development is
  reviewed across days, and the reader remembers where you were.
- **Room to read**: rows of 28 pixels in the tree, code on lines of 22, the path to a file quiet
  and its name plain, icon buttons with their key in the tooltip.

## How it is made

| Package | |
|---|---|
| `cmd/code-review` | The program: flags and wiring |
| `internal/git` | The read-only git commands, and reading their output |
| `internal/highlight` | Chroma's tokens, cut into lines of coloured pieces |
| `internal/workspace` | Projects, changes, a file's two sides and its diff, folding, the tree, what servers wrote |
| `internal/text` | A file being edited: lines, caret, selection, undo |
| `internal/lsp` | Language servers: JSON-RPC over their input and output, the table of servers |
| `internal/java` | The IntelliJ-style tree and the kinds of class |
| `internal/theme` | Zed's One colours, file icons and the reader's own controls |
| `internal/pane` | What every pane shares: the keys and a scrolling list |
| `internal/viewer`, `internal/sidebar`, `internal/finder` | The panes |
| `internal/window` | The window: layout, commands, dialogs and background work |
| `internal/mode` | What a mode is, and what it asks of the window |
| `internal/files`, `internal/review` | The two modes: read any file; review the changes, marking each viewed |

How the packages talk to each other, how to add a mode and how to propose a change:
[docs/8-contributing.md](docs/8-contributing.md).

go-ui describes the window again at every frame. The sidebar, the code and the finder's results
are painted by the program (`Context.Paint`) as lists that only draw the rows in sight, with their
own scrolling, so the keyboard can move them; go-ui's window hands the program the keys through
its `Driver`. Repositories and files are read on other goroutines, and the window wakes when they
are ready.

## Not done yet

- No search inside a file.
- No side-by-side diff; the sides are interleaved.
- Long lines are not wrapped: scroll sideways (Shift+wheel, ←/→ in the editor).
- One typeface for code at a time: SF Mono by default, or the one `-code-font` names (a `.ttf`;
  collections such as `Menlo.ttc` are not read).
- Language servers only follow names: no rename, formatting, completion or diagnostics yet, and
  not on the old side of a diff, which is not on disk.
