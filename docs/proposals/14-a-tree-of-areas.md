# 14. A tree of areas

> Status: done · Author's approval: proposal ☑ plan ☑

## Why

`editor/internal/` holds 34 packages side by side, and every new one makes the list longer. A
newcomer reads it as one flat list and cannot tell the interface from the domain, nor which
packages belong together. Go gives a folder inside another no meaning, but a reader does: it says
"this is part of that". This change groups the packages into areas, and joins the pairs that are
one idea, so the first level stays short as Coddy grows.

## What

| Id | Requirement | Check |
|---|---|---|
| R1 | Everything that draws or reads the keyboard is under `internal/ui/`, a folder with no package of its own | `TestNothingOutsideTheInterfaceImportsIt`, which replaces `TestTheDomainImportsNoInterface`: its hand-written list of four packages missed `workspace`, `servers` and the rest, and the folder lets the rule cover every package, new ones too |
| R2 | The icons are part of the theme: one package `ui/theme`, colours and icons | `go build`; the icon tests pass in `theme` |
| R3 | A key and what it does is one package, `ui/shortcut`: `Press`, its makers, the profiles; `pane` keeps the keys pressed, the scrolling list and `Words` | `go build`; the keymap tests pass in `shortcut` |
| R4 | Formatting text is one area, `format/`: its own package counts things and capitalises (what `english` did), with `format/highlight` and `format/markdown` inside | `go build`; their tests pass |
| R5 | Running things is one area, `execution/`: `execution/programs` and `execution/work` | `go build`; their tests pass |
| R6 | The keyring is the plugins' own: `plugins/keyring` | `go build`; its test passes |
| R7 | Nothing changes for the person using Coddy | every test passes with only imports and names changed; a capture before and after is the same to the byte, in dark and light |
| R8 | The docs show the tree as it is | `4-how-it-is-made.md`, `8-contributing.md` and the guides |

## Out of scope

- The contracts, in proposal 15: `versioncontrol` with git as one of them, `language/jvm` taking
  Java out of `lsp` and `navigation`, a content search for folders without git. Until then
  `git`, `language`, `java`, `gradle` and `builds` stay where they are.
- Splitting `window`.

## Design

The tree after this change, and after proposal 15 in brackets:

```
internal/
├── ui/              window  viewer  sidebar  finder  settings  navigation
│                    mode/ (files review uses pluginmode)  shortcut  pane  controls  theme
├── codebase/
├── workspace/
├── text/
├── format/          highlight/  markdown/
├── lsp/
├── servers/
├── plugins/         keyring/
├── execution/       programs/  work/
├── git/  language/  java/  gradle/  builds/      [versioncontrol/git/  language/jvm/]
```

**`ui/` is a folder, not a package.** That keeps it clear of the alias `ui` every file gives
Go UI: nobody imports `internal/ui`, only `internal/ui/window` and its siblings.

**`shortcut`, not `keys`.** `keys` is a variable 372 times in the editor (`keys := …Keys()`),
and a package named so would be hidden by each. `shortcut` collides only with a test's type,
renamed. What moves: `press.go` (`Press`, `Key`, `CommandKey`, `StepKey`…), `PageStep` and
`EndStep`, and the whole of `keymap`. `pane.Keys` stays in `pane`: it is the state of the keys and
the pointer, and `Scroller` reads it; `shortcut.Recorded` reads it too, so the arrow points from
`shortcut` to `pane`.

**Icons join the theme with their names kept apart.** Both packages have a `Theme`, a
`NotAThemeFile` and loaders. The colour theme keeps `Theme`; the icon theme becomes `IconTheme`,
with `LoadIconThemes`, `ReadIconThemes`, `IconThemesFolder` and the error `NotAnIconThemeFile`. The
error codes `icons.…` become `theme.…`.

**`format`'s own package is `english`'s two functions,** `format.Count(3, "file")` and
`format.Capitalised(…)`: the words around a number are formatting too.

**One area, one package is not an area.** That is why the keyring goes under `plugins/`, its only
user, and not under a `security/` of one; when a second piece of security appears, it gets its
area.

**How.** Each step is mechanical, as in proposal 13: move with `git mv`, rewrite the imports and
the qualified names, build, test, and only then the next step.

## Tasks

- [x] T1 — `format/`, `execution/`, `plugins/keyring` · R4, R5, R6
- [x] T2 — `ui/theme` with the icons, renaming what clashes · R2
- [x] T3 — `ui/shortcut` from `keymap` and `pane`'s presses · R3
- [x] T4 — move the interface under `ui/`, with the test that nothing outside imports it · R1
- [x] T5 — docs, and the capture before and after · R7, R8

## Changed while building

- The error codes follow their packages: `icons.…` and `keymap.…` are `theme.…` and `shortcut.…`;
  the icon theme's are `theme.not-an-icon-theme-file` and `theme.no-icon-themes`, apart from the
  colour theme's.
- A test's type named `shortcut` is `binding`, so the package keeps its name.
- `TestNoPaneImportsTheWindow` lists only the panes now: the packages outside `ui/` it also
  listed are covered by the new test.

## Result

Built: R1 to R8. `TestNothingOutsideTheInterfaceImportsIt` replaces
`TestTheDomainImportsNoInterface`; a probe import of `ui/pane` into `text` makes it fail, for
`text` and for `servers`, which imports `text`. Every other test passes with only imports and
names changed, and a capture of the same file before and after is the same to the byte, in dark
and light. `internal/` holds 14 entries, `ui/` 11. Still flaky, as before this change:
`TestReadingTheServersRunningCostsNothingUntilOneChanges` fails about once in ten runs.
