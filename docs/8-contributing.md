# Contributing

This page is for someone who wants to change Coddy. By the end you will know where each
piece lives, how to build and test it, and how to add a mode.

## The one rule

Coddy writes a file of a repository only when someone saves it, and git only reads.

- `workspace.Save` is the one function that writes inside a repository: the whole file at once,
  through a file beside it renamed over it, and never over a change made on disk since it was read
  (`workspace.changed-on-disk`). `TestOnlyTheSaveWritesInARepository` fails on a file write
  anywhere else; the marks of the files viewed and the record of what language servers wrote,
  both kept in the reader's own folder, are the only other places allowed.
- Language servers are other programs and may write in a repository (`jdtls` does). What they
  write is never the person's change: `workspace.Written.SortOut` takes it out of the review, and
  the sidebar lists it. A server never edits a file the person reads: the client answers every
  `workspace/applyEdit` with "not applied".
- Every git command goes through `editor/internal/git`, which runs only the commands on its read-only
  list and refuses the rest (`git.command-not-read-only`): no commit, no push, no checkout.
- While a file is edited, plain keys type: every command keeps a key with Command, an F-key or
  Esc (`TestNoPlainKeyCommandWhileEditing`).

## Where things are

Packages are named by what they are about, one line each:

```
sdk/                  a module of its own: what a plugin is written with
sdk                   the protocol plugins speak
sdk/screen            a plugin's mode drawn with Go UI
sdk/jsonrpc           the connection to another program: language servers and plugins
plugins/              a module of its own: the official plugins, built on the sdk alone
editor/               a module of its own: Coddy, built on the sdk; its packages, below, are in it
cmd/coddy             the program: flags, the modes it registers, the window
internal/files        the files mode: read any file
internal/review       the review mode: the changes, each marked as viewed
internal/uses         the uses mode: every use of a name across the projects in sight
internal/window       the window: layout, mode rail, commands, dialogs
internal/mode         what a mode is, and what it asks of the window
internal/viewer       a file's view: whole, as its changes, or as the uses of a name
internal/sidebar      the projects' tree, its filter and the rows' decorations
internal/finder       finding files by name, lines by content, and the uses of a name
internal/settings     the settings screen
internal/work         background jobs, and what they give back to the frame
internal/servers      the language servers a window runs: start, tell, rest, install
internal/plugins      the plugins installed: their manifests, their programs, their actions
internal/navigation   following names: declarations, the trail back, uses across projects
internal/session      what is open: folders, projects, their bases, the documents read
internal/english      a count of things and a first capital, in English
internal/pane         what every pane shares: the keys, key presses, a scrolling list
internal/theme        colour themes in Zed's format, each file's icon in their colours, the reader's own controls
internal/icons        what files and folders are drawn as: Lucide's kinds, Zed's icon themes, their pictures
internal/java         the IntelliJ-style tree and the kinds of class
internal/workspace    projects, changes, a file's two sides and its diff, the tree
internal/text         a file being edited: lines, caret, selection, undo
internal/lsp          language servers: what to ask them, over jsonrpc, and how to run one
internal/gradle       what servers that import Gradle builds need: their own Gradle home, its daemons, kept models
internal/builds       what a project's builds publish and depend on, one reader per build system
internal/highlight    Chroma's tokens, cut into lines of coloured pieces
internal/markdown     a Markdown file read into blocks, for the viewer to lay out
internal/language     what Coddy knows of each language: names, extensions, server, icon, fences
internal/git          the read-only git commands
```

How these lean on each other, floor by floor, is in [how it is made](4-how-it-is-made.md).
`editor/cmd/coddy/rules_test.go` keeps the arrows pointing down:

| Test | It checks |
|---|---|
| `TestTheDomainImportsNoInterface` | `workspace`, `text`, `lsp` and `plugins` import no Go UI, pane, window or mode |
| `TestNoPaneImportsTheWindow` | no pane, nor `work`, `session`, `navigation`, `servers` or `plugins`, imports `window` |
| `TestTheWindowImportsNoMode` | `window` imports neither `files` nor `review` |
| `TestTheLanguageTableImportsNothingOfTheReader` | `language` is data, and imports nothing of Coddy |
| `TestGradleKnowsNothingOfTheLanguageServers` | `gradle` does not import `lsp` |

Tests check what the program does, and the arrows that keep it apart. How the code is written
(the length of a function or a file, how imports are grouped, a reason beside each `_ =`, the
language names kept in their table) is a rule of review, not a test: a test of style is code
that has nothing to do with the program, and every change pays for it.

## How a pane talks to the window

A pane never imports `window`. Each pane declares an interface named `Window` with the little it
needs, such as `Projects()` or `OpenFile(file)`, and `window.Window` implements all of them in
`editor/internal/window/panes.go`. A new need is a new method on the pane's interface and its answer in
`panes.go`.

What only some windows can do, such as going to a definition (`viewer.Navigator`) or reading Java
classes (`sidebar.ClassReader`), is an interface of its own beside `Window`. The pane checks for it
and does without it when it is missing, so a test's window only has to answer what every window
answers. `panes.go` checks at build time that the reader's window has every such power.

## Adding a language, a server, a theme, a mode or a plugin

Each has its guide: [a language](5-guides/1-adding-a-language.md),
[a language server](5-guides/2-adding-a-language-server.md),
[a theme](5-guides/3-adding-a-theme.md), [a mode](5-guides/4-adding-a-mode.md) and
[a plugin](5-guides/5-writing-a-plugin.md).

## Drawing a screen

[`editor/DESIGN.md`](../editor/DESIGN.md) says what every screen keeps to. Above all: no text runs past its box.

## Errors

Each package declares its errors in `errors.go` with Go Renderer's `failure` package, such as
`failure.Define("review.marks-not-kept", "the files marked as viewed could not be kept on disk")`, and tests each
one in `errors_test.go`. `errors.New` and `fmt.Errorf` are not used; `TestNoErrorIsMadeWithoutACode`
looks for them.

## Building and testing

```bash
go build -C editor -o ../out/coddy ./cmd/coddy
gofmt -l .                                    # every module at once
for module in editor sdk plugins; do (cd $module && go vet ./... && go test ./...); done
```

`gofmt -l` prints nothing when every file is formatted. The window's tests drive the real window
headless, with uitest, on small repositories they make in a temporary folder. `-capture out.png`
draws one frame to a picture, which is the quickest way to look at a change.

## Proposing a change

A small fix goes straight to a branch. A bigger change starts as a proposal in `docs/proposals/`,
numbered, with why, what (requirements with their checks), what is out of scope, the design and
the tasks. The author approves the proposal and then its plan before the code is written. If the
plan turns out wrong while building, the proposal changes first, and says what changed.
