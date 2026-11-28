# Contributing

This page is for someone who wants to change Coddy. By the end you will know where each
piece lives, how to build and test it, and how to add a mode.

## The one rule

Coddy writes a file of a repository only when someone saves it, and git only reads.

- `codebase.Save` is the one function that writes inside a repository: the whole file at once,
  through a file beside it renamed over it, and never over a change made on disk since it was read
  (`workspace.changed-on-disk`). `TestOnlyTheSaveWritesInARepository` fails on a file write
  anywhere else; the marks of the files viewed and the record of what language servers wrote,
  both kept in the reader's own folder, are the only other places allowed.
- Language servers are other programs and may write in a repository (`jdtls` does). What they
  write is never the person's change: `codebase.Written.SortOut` takes it out of the review, and
  the sidebar lists it. A server never edits a file the person reads: the client answers every
  `workspace/applyEdit` with "not applied".
- Every git command goes through `editor/internal/git`, which runs only the commands on its read-only
  list and refuses the rest (`git.command-not-read-only`): no commit, no push, no checkout.
- While a file is edited, plain keys type: every command keeps a key with Command, an F-key or
  Esc (`TestNoPlainKeyCommandWhileEditing`).

## Where things are

Packages are named by what they are about, and grouped by area: a folder inside another says
"this is part of that". Everything that draws or reads the keyboard is under `internal/ui/`;
everything else is the domain, which works without a window.

```
sdk/                           a module of its own: what a plugin is written with
  sdk                          the protocol plugins speak
  screen/                      a plugin's mode drawn with Go UI
  jsonrpc/                     the connection to another program: language servers and plugins
plugins/                       a module of its own: the official plugins, built on the sdk alone
editor/                        a module of its own: Coddy, built on the sdk
  cmd/coddy/                   the program: flags, the modes it registers, the window
  internal/
    ui/                        the interface: a folder, not a package
      window/                  the window: layout, mode rail, commands, dialogs
      viewer/                  a file's view: whole, as its changes, or as the uses of a name
      sidebar/                 the projects' tree, its filter and the rows' decorations
      finder/                  finding files by name, lines by content, and the uses of a name
      settings/                the settings screen
      navigation/              following names: declarations, the trail back, uses across projects
      mode/                    what a mode is, and what it asks of the window
        files/                 the files mode: read any file
        review/                the review mode: the changes, each marked as viewed
        uses/                  the uses mode: every use of a name across the projects in sight
        pluginmode/            a mode a plugin gives, its screen drawn by the plugin
      shortcut/                a key and what it does: presses, chords and the profiles of them
      pane/                    what every pane shares: the keys pressed, a scrolling list, Words
      controls/                the reader's own controls: key caps, buttons, rows, file icons
      theme/                   colour themes and icon themes in Zed's format, and Lucide's icons
    codebase/                  a project's code: its files, changes, a file's two sides and its diff, the tree
    workspace/                 what is open: folders, projects, their bases, the documents read
    text/                      a file being edited: lines, caret, selection, undo
    format/                    a count of things and a first capital, in English
      highlight/               Chroma's tokens, cut into lines of coloured pieces
      markdown/                a Markdown file read into blocks, for the viewer to lay out
    lsp/                       language servers: what to ask them, over jsonrpc, and how to run one
    servers/                   the language servers a window runs: start, tell, rest, install
    plugins/                   the plugins installed: their manifests, their programs, their actions
      keyring/                 the secrets plugins keep, in the system's keyring
    execution/
      programs/                finding the programs Coddy runs, and installing a missing one
      work/                    background jobs, and what they give back to the frame
    language/                  what Coddy knows of each language: names, extensions, server, icon, fences
    java/                      the IntelliJ-style tree and the kinds of class
    gradle/                    what servers that import Gradle builds need: their own Gradle home, its daemons, kept models
    builds/                    what a project's builds publish and depend on, one reader per build system
    git/                       the read-only git commands
```

How these lean on each other, floor by floor, is in [how it is made](4-how-it-is-made.md).
`editor/cmd/coddy/rules_test.go` keeps the arrows pointing down:

| Test | It checks |
|---|---|
| `TestNothingOutsideTheInterfaceImportsIt` | no package outside `ui/` imports one inside it, nor Go UI |
| `TestNoPaneImportsTheWindow` | no pane, nor `navigation`, imports `window` |
| `TestTheWindowImportsNoMode` | `window` imports none of `files`, `review` and `uses` |
| `TestTheThemeKnowsNeitherTheControlsNorGit` | `theme` imports neither `controls` nor `git` |
| `TestTheLanguageTableImportsNothingOfTheReader` | `language` is data, and imports nothing of Coddy |
| `TestGradleKnowsNothingOfTheLanguageServers` | `gradle` does not import `lsp` |

Tests check what the program does, and the arrows that keep it apart. How the code is written
(the length of a function or a file, how imports are grouped, a reason beside each `_ =`, the
language names kept in their table) is a rule of review, not a test: a test of style is code
that has nothing to do with the program, and every change pays for it.

## How a pane talks to the window

A pane never imports `window`. Each pane declares an interface named `Window` with the little it
needs, such as `Projects()` or `OpenFile(file)`, and `window.Window` implements all of them in
`editor/internal/ui/window/panes.go`. A new need is a new method on the pane's interface and its answer in
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
