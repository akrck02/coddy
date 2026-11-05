# Code review

**A small, fast, read-only code reader for developments that change many repositories at once.**
Open a folder of projects, see which ones a development changed against their base branch, and
read each changed file's diff, coloured, without leaving the keyboard.

Built with [go-ui](https://github.com/akrck02/go-ui) over
[go-renderer](https://github.com/akrck02/go-renderer), dressed in
[Bubble 4](https://github.com/akrck02/bubble)'s paper theme (light and dark, compact);
code is coloured with [chroma](https://github.com/alecthomas/chroma),
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
- **Only the projects with changes**, by default: the projects a development did not touch are left
  out of the sidebar (both modes) and the finder. `h`, the *Only changed projects* button or
  `-all` show every project; the choice is remembered.
- **Every other file stays readable**: the *Files* mode lists them all, and the finder finds them.
- **Enter a project** to see only its tree, from its top (Enter on its row, `e`, or the → that
  shows on its row under the pointer); `Esc` goes back to every project.
- **Icons by file type** in the tree and the finder, tinted with the code's own colours.
- **Syntax colouring** for Java, Kotlin, Vue, SQL, Gradle, Groovy, XML, YAML, Markdown and the
  rest of chroma's languages; text that is not UTF-8 is read as Latin-1.
- **Never writes to a repository.** It only runs `git cat-file`, `diff`, `ls-files`, `merge-base`,
  `rev-list`, `rev-parse` and `symbolic-ref` (any other command is refused by
  `internal/git/run.go`), without the optional locks, the file system monitor or external diff
  programs. It never fetches, so a remote branch is as fresh as the last `git fetch` you ran.

## Build and run

You need Go 1.25 or later and the Xcode command line tools (`xcode-select --install`): the window
is opened through cgo and drawn with Metal.

```bash
go build -o bin/code-review .
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
| `-light` | Start in the light theme (dark by default) |
| `-width`, `-height`, `-capture shot.png` | go-ui's window flags; `-capture` draws one frame to a picture |

```bash
go vet ./... && go test ./...     # the interface tests drive the window with no display
```

## Keys

| Key | |
|---|---|
| `⌘P` (or `Shift+P`) | Find a file by its name, in any project; changed files first |
| `⌘⇧F` (or `/`) | Find a text in the files of every project; `Tab` in the finder switches between names and content |
| `n` / `m` | Next / previous changed file, across every project |
| `]` / `[` | Next / previous change in the file (the diff, or the marked lines) |
| `1` / `2` (`c` switches) | Files mode / Review mode |
| `Enter` on a project, `e`, or its → button | Enter the project: the sidebar lists only its tree, and `n`/`m`/`v` stay in it |
| `Esc` / `Backspace` | Back to every project |
| `g` | Files mode: list only the changed files (they still open whole), or every file |
| `v` | Mark the file as viewed (or not); marking it opens the next file not viewed |
| `d` | Diff, or the whole file |
| `f` | Fold the unchanged lines, or show them all |
| `Tab` | Move between the sidebar and the editor |
| `↑ ↓ ← →`, `PgUp PgDn`, `Home End` | Move in the sidebar (←/→ close and open folders), or scroll the file |
| `j` / `k`, `Space` | Scroll a line, a page |
| `Enter` | Open a file or a folder (sidebar), open a fold (editor) |
| `h` | Only the projects with changes, or every project (sidebar and finder) |
| `b` | Compare against another branch, tag or commit |
| `w` | Count the changes not committed yet, or the commits only |
| `r` (`⌘R`) | Read every project again |
| `o` (`⌘O`) | Open a repository or a folder of repositories |
| `y` | Copy the open file's path |
| `t` | Light or dark |
| `?` | These keys |

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
| `internal/git` | The read-only git commands, and reading their output |
| `internal/highlight` | Chroma's tokens, cut into lines of coloured pieces |
| `internal/review` | Projects, changes, a file's two sides and its diff, folding, the tree, the fuzzy finder |
| `internal/app` | The window: sidebar, editor, finder and dialogs |

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
