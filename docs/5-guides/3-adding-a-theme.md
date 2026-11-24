# Adding a theme

This guide is for someone who wants Coddy in other colours, or with other file icons. Both come in
Zed's formats, so any theme made for Zed works.

## A colour theme for yourself

Copy a Zed theme file (a `.json` with a `themes` list) into your themes folder:

```bash
mkdir -p ~/.config/code-review/themes
cp my-theme.json ~/.config/code-review/themes/
```

On a Mac the folder is `~/Library/Application Support/code-review/themes/`. Start Coddy, open the
settings (`⌘,`) and choose it under *Appearance*. A file whose theme has the name of a built-in one
takes its place.

## A colour theme for everyone

The built-in themes are embedded from `internal/theme/themes/`. Add the `.json` there, with its
licence beside it as `name.LICENSE` when it is not ours, and say where it comes from in the
comment above `builtIn` in `internal/theme/themes.go`. Then add its name to
`TestTheReaderComesWithTheThemesOfZedIntelliJVisualStudioCodeDraculaAndMrPink`.

A theme may give only some colours: the reader makes up the rest from the others
(`TestAThemeWithOnlyItsTextAndEditorIsMadeWhole`).

## An icon theme

Icon themes are Zed's too: a folder with `icon_themes/*.json` and the SVG files they name. Copy
the folder into `~/.config/code-review/icon-themes/`, or install it in Zed: Coddy also lists Zed's
installed extensions. [Get started](../2-get-started.md) says where they go on each system.
