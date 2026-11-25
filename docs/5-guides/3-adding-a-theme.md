# Adding a theme

This guide is for someone who wants Coddy in other colours, or with other file icons. Both come in
Zed's formats, so any theme made for Zed works.

## A colour theme for yourself

Copy a Zed theme file (a `.json` with a `themes` list) into your themes folder:

```bash
mkdir -p ~/.config/coddy/themes
cp my-theme.json ~/.config/coddy/themes/
```

On a Mac the folder is `~/Library/Application Support/coddy/themes/`. Start Coddy, open the
settings (`⌘,`) and choose it under *Appearance*. A file whose theme has the name of a built-in one
takes its place.

## A colour theme for everyone

The built-in themes are embedded from `editor/internal/theme/themes/`. Add the `.json` there, with its
licence beside it as `name.LICENSE` when it is not ours, and say where it comes from in the
comment above `builtIn` in `editor/internal/theme/themes.go`. Then add its name to
`TestTheReaderComesWithTheThemesOfZedIntelliJVisualStudioCodeDraculaAndMrPink`.

A theme may give only some colours: the reader makes up the rest from the others
(`TestAThemeWithOnlyItsTextAndEditorIsMadeWhole`).

## An icon theme

Icon themes are Zed's too: a folder with `icon_themes/*.json` and the SVG files they name. Copy
the folder into `~/.config/coddy/icon-themes/`, or install it in Zed: Coddy also lists Zed's
installed extensions. [Get started](../2-get-started.md) says where they go on each system.

Icon themes are read and matched in `editor/internal/icons`, apart from the colours: `icons.LoadThemes`
lists them, `Theme.FilePicture` names a file's picture as Zed does, and `icons.Pictures` draws each
SVG once per size. The `theme` package only joins them to the look, in `theme.FileIcon` and
`theme.DrawIcon`. A file of a kind Lucide has no icon for gets one in `icons/kinds.go`.
