# Contributing

This page is for someone who wants to change Code Review. By the end you will know where each
piece lives, how to build and test it, and how to add a mode.

## The one rule

Code Review writes a file of a repository only when someone saves it, and git only reads.

- `workspace.Save` is the one function that writes inside a repository: the whole file at once,
  through a file beside it renamed over it, and never over a change made on disk since it was read
  (`workspace.changed-on-disk`). `TestOnlyTheSaveWritesInARepository` fails on a file write
  anywhere else; the marks of the files viewed and the record of what language servers wrote,
  both kept in the reader's own folder, are the only other places allowed.
- Language servers are other programs and may write in a repository (`jdtls` does). What they
  write is never the person's change: `workspace.Written.SortOut` takes it out of the review, and
  the sidebar lists it. A server never edits a file the person reads: the client answers every
  `workspace/applyEdit` with "not applied".
- Every git command goes through `internal/git`, which runs only the commands on its read-only
  list and refuses the rest (`git.command-not-read-only`): no commit, no push, no checkout.
- While a file is edited, plain keys type: every command keeps a key with Command, an F-key or
  Esc (`TestNoPlainKeyCommandWhileEditing`).

## Where things are

Packages are named by what they are about. The arrows point one way: a package only imports those
below it.

```
cmd/code-review       the program: flags, the modes it registers, the window
internal/files        the files mode: read any file
internal/review       the review mode: the changes, each marked as viewed
internal/window       the window: layout, mode rail, commands, dialogs
internal/mode         what a mode is, and what it asks of the window
internal/viewer       the panes: a file's view, the tree, the finder, the settings
internal/sidebar
internal/finder
internal/settings
internal/explorer     a folder's files, shown like a file manager
internal/work         background jobs, and what they give back to the frame
internal/servers      the language servers a window runs: start, tell, rest, install
internal/navigation   following names: declarations, the trail back, uses across projects
internal/session      what is open: folders, projects, their bases, the documents read
internal/pane         what every pane shares: the keys, key presses, a scrolling list
internal/theme        Zed's One colours, file icons, the reader's own controls
internal/java         the IntelliJ-style tree and the kinds of class
internal/workspace    projects, changes, a file's two sides and its diff, the tree
internal/text         a file being edited: lines, caret, selection, undo
internal/lsp          language servers: JSON-RPC over their input and output
internal/highlight    Chroma's tokens, cut into lines of coloured pieces
internal/git          the read-only git commands
```

Three tests in `cmd/code-review/rules_test.go` keep the arrows pointing down:

| Test | It checks |
|---|---|
| `TestTheDomainImportsNoInterface` | `workspace`, `text` and `lsp` import no go-ui, pane, window or mode |
| `TestNoPaneImportsTheWindow` | `theme`, `pane`, `viewer`, `sidebar` and `finder` don't import `window` |
| `TestTheWindowImportsNoMode` | `window` imports neither `files` nor `review` |

## How a pane talks to the window

A pane never imports `window`. Each pane declares an interface named `Window` with the little it
needs, such as `Projects()` or `OpenFile(file)`, and `window.Window` implements all of them in
`internal/window/panes.go`. A new need is a new method on the pane's interface and its answer in
`panes.go`.

## Adding a mode

A mode is one package and one line in `main`.

1. Make a package, say `internal/usages`, with a type that implements `mode.Mode`. The
   files mode, `internal/files/files.go`, is the smallest example.
2. Answer what the panes ask. `ChangesOnly` and `ShowsChanges` decide what is listed and how a file
   opens. `Decorate(row)` says what a row shows beside its name. `Commands` declares the mode's
   keys, with `pane.Key`.
3. Reach the window through the `mode.Window` it is given in `Start`, never by importing `window`.
4. Register it in `cmd/code-review/main.go`, after the others: `window.New(settings, files.New(),
   review.New(path), usages.New())`. It takes the next place on the rail and the next number key.

`TestNoTwoCommandsShareAKeyInOneMode` fails if a new key clashes with one already taken.
`TestTheHelpListsEveryCommand` checks the help shows it.

## Errors

Each package declares its errors in `errors.go` with Go Renderer's `failure` package, such as
`failure.Define("review.marks-not-kept", "the files marked as viewed could not be kept on disk")`, and tests each
one in `errors_test.go`. `errors.New` and `fmt.Errorf` are not used; `TestNoErrorIsMadeWithoutACode`
looks for them.

## Building and testing

```bash
go build -o bin/code-review ./cmd/code-review
gofmt -l . && go vet ./... && go test ./...
```

`gofmt -l` prints nothing when every file is formatted. The window's tests drive the real window
headless, with uitest, on small repositories they make in a temporary folder. `-capture out.png`
draws one frame to a picture, which is the quickest way to look at a change.

## Proposing a change

A small fix goes straight to a branch. A bigger change starts as a proposal in `docs/proposals/`,
numbered, with why, what (requirements with their checks), what is out of scope, the design and
the tasks. The author approves the proposal and then its plan before the code is written. If the
plan turns out wrong while building, the proposal changes first, and says what changed.
