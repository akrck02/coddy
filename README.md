# Code review

**A small, fast, read-only code reader for developments that change many repositories at once.**
Open a folder of projects, see which ones a development changed against their base branch, and
read each changed file's diff, coloured, without leaving the keyboard.

Built with [go-ui](https://github.com/akrck02/go-ui) over
[go-renderer](https://github.com/akrck02/go-renderer); code is coloured with
[chroma](https://github.com/alecthomas/chroma), as papiro-indexer colours its code blocks.

## 🤖 AI disclosure

- **Written by an AI assistant (Claude, by Anthropic) under my direction, in November 2026:** all
  of it so far.

Commits made with AI carry a `Co-Authored-By: Claude` line. **Status: pending thorough review.**

## What it does

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
  along the scroll bar. `d` switches to the whole file.
- **Every other file stays readable**: the sidebar's *Files* mode lists them all, and the finder
  finds them.
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
| `-light` | Start in the light theme (dark by default) |
| `-width`, `-height`, `-capture shot.png` | go-ui's window flags; `-capture` draws one frame to a picture |

```bash
go vet ./... && go test ./...     # the interface tests drive the window with no display
```

## Keys

| Key | |
|---|---|
| `⌘P` (or `Shift+P`) | Find a file in any project; changed files first |
| `n` / `m` | Next / previous changed file, across every project |
| `]` / `[` | Next / previous change in the file |
| `d` | Diff, or the whole file |
| `f` | Fold the unchanged lines, or show them all |
| `Tab` | Move between the sidebar and the editor |
| `↑ ↓ ← →`, `PgUp PgDn`, `Home End` | Move in the sidebar (←/→ close and open folders), or scroll the file |
| `j` / `k`, `Space` | Scroll a line, a page |
| `Enter` | Open a file or a folder (sidebar), open a fold (editor) |
| `c` | Sidebar: the changes only, or every file |
| `h` | Show or hide the projects without changes |
| `b` | Compare against another branch, tag or commit |
| `w` | Count the changes not committed yet, or the commits only |
| `r` (`⌘R`) | Read every project again |
| `o` (`⌘O`) | Open a repository or a folder of repositories |
| `y` | Copy the open file's path |
| `t` | Light or dark |
| `?` | These keys |

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

- No search inside a file, no word-level highlighting inside a changed line.
- No side-by-side diff; the sides are interleaved.
- Long lines are not wrapped: scroll sideways (Shift+wheel, ←/→ in the editor).
- One typeface for code (Go Mono).
