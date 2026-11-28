# Adding a mode

This guide is for someone who wants a new way of going through the projects, beside reading files
and reviewing changes. A mode is one package and one line in `main`.

## The shortest way

1. Make a package, say `editor/internal/usages`, with a type that implements `mode.Mode`. The files mode,
   `editor/internal/ui/mode/files/files.go`, is the smallest example.
2. Answer what every mode answers. `ChangesOnly` and `ShowsChanges` decide what is listed and how a
   file opens. `Decorate(row)` says what a row shows beside its name.
3. Reach the window through the `mode.Window` it is given in `Start`, never by importing `window`.
4. Register it in `editor/cmd/coddy/main.go`, after the others:
   `window.New(settings, files.New(), review.New(path), usages.New())`. It takes the next place on
   the rail and the next number key.

## The powers

A mode adds only the powers it has. Each is a small interface of `mode` or `sidebar`, and the
window looks for each once, when the mode is turned on:

| Power | What it gives |
|---|---|
| `WithCommands` | its keys, declared with `pane.Key` |
| `WithFileButton` | a button in the viewer's header, and what pressing a row does |
| `WithProgress` | a count of its work in the status bar |
| `WithPage` | a page shown while no file is open |
| `Cover` | a view drawn over the viewer |
| `UsesHolder` | the uses of a name |
| `sidebar.Headed` | words beside the sidebar's title |

A method that only fills a contract and does nothing is a sign the power is not the mode's:
leave it out instead.

## Checking it

`TestNoTwoCommandsShareAKeyInOneMode` fails if a new key clashes with one already taken, and
`TestTheHelpListsEveryCommand` checks the help shows it.
