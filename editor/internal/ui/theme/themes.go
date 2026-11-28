package theme

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akrck02/bubble"
	"github.com/akrck02/coddy/editor/internal/format/highlight"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
)

// Theme is a look with a name, as the settings list it: one of the reader's own, or one read from
// a file of the themes folder.
type Theme struct {
	Name string
	Dark bool
	// File is the file it was read from; empty for the reader's own
	File string
	Look Look
}

// The themes the reader starts in, when none was chosen.
const (
	DefaultDark  = "IntelliJ Dark"
	DefaultLight = "One Light"
)

// builtIn holds the reader's own theme files. dracula.json is Dracula Solid, one of the themes of
// https://github.com/dracula/zed (themes/dracula.json, commit c419d71, the others left out), under
// the MIT licence kept beside it in themes/dracula.LICENSE. vscode.json is Visual Studio Code's
// Dark Modern, its colours taken from https://github.com/microsoft/vscode
// (extensions/theme-defaults/themes, commit 3bdd3dd4191a; MIT) and written in Zed's format.
// mrpink.json is juanmnl's Mr Pink, its colours taken from https://github.com/juanmnl/vs-mr-pink
// (themes/mrpink.json, commit 133a656) and written in Zed's format, under the MIT licence kept
// beside it in themes/mrpink.LICENSE.
//
//go:embed themes/*.json
var builtIn embed.FS

// BuiltIn are the reader's own themes: Zed's One, IntelliJ's and Dracula's, sorted by name.
func BuiltIn() []Theme {
	entries, err := builtIn.ReadDir("themes")
	if err != nil {
		panic(err) // the folder is in the program
	}
	var themes []Theme
	for _, entry := range entries {
		content, err := builtIn.ReadFile("themes/" + entry.Name())
		if err != nil {
			panic(err)
		}
		read, err := Read(content)
		if err != nil {
			panic(fmt.Sprintf("the theme %s of the program: %v", entry.Name(), err)) // a test reads them all
		}
		themes = append(themes, read...)
	}
	sortThemes(themes)
	return themes
}

// Folder is where a person's own themes are read from: themes/ in the reader's own folder.
func Folder() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "coddy", "themes")
}

// Load are the reader's own themes and those of the .json files in a folder, a theme of the folder
// taking the place of one of the reader's with its name. A file that cannot be read is told in
// problems and left out; a folder that is not there has no themes.
func Load(folder string) (themes []Theme, problems []error) {
	themes = BuiltIn()
	if folder == "" {
		return themes, nil
	}
	paths, _ := filepath.Glob(filepath.Join(folder, "*.json"))
	for _, path := range paths {
		read, err := readFile(path)
		if err != nil {
			problems = append(problems, failure.Wrap(NotAThemeFile, filepath.Base(path), err))
		}
		for _, each := range read {
			each.File = path
			themes = replaced(themes, each)
		}
	}
	sortThemes(themes)
	return themes, problems
}

// readFile reads the themes of a file of the themes folder.
func readFile(path string) ([]Theme, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Read(content)
}

// Find is the theme of a name among some, or the first of the right darkness when it is not there.
func Find(themes []Theme, name string, dark bool) Theme {
	for _, each := range themes {
		if each.Name == name {
			return each
		}
	}
	fallback := DefaultLight
	if dark {
		fallback = DefaultDark
	}
	for _, each := range themes {
		if each.Name == fallback {
			return each
		}
	}
	return themes[0]
}

// Read reads a theme file: Zed's format, a family of themes, each with its appearance, the colours
// of its interface (style) and of its code (style.syntax). The reader takes the colours it paints
// with and makes up those a file leaves out from the others.
func Read(content []byte) ([]Theme, error) {
	var family struct {
		Name   string `json:"name"`
		Themes []struct {
			Name       string         `json:"name"`
			Appearance string         `json:"appearance"`
			Style      map[string]any `json:"style"`
		} `json:"themes"`
	}
	if err := json.Unmarshal(content, &family); err != nil {
		return nil, failure.Wrap(NotAThemeFile, "", err)
	}
	if len(family.Themes) == 0 {
		return nil, NoThemes
	}
	themes := make([]Theme, 0, len(family.Themes))
	for _, each := range family.Themes {
		dark := !strings.EqualFold(each.Appearance, "light")
		look, found := lookOf(style(each.Style), dark)
		if !found {
			return nil, failure.Wrap(MissingColours, each.Name, nil)
		}
		themes = append(themes, Theme{Name: each.Name, Dark: dark, Look: look})
	}
	return themes, nil
}

// style is a theme's style as the file has it: colours by name, and the lists of players and syntax.
type style map[string]any

// colour is the first of the named colours the style gives, and whether there was one.
func (given style) colour(names ...string) (draw2d.Color, bool) {
	for _, name := range names {
		if text, isText := given[name].(string); isText {
			if colour, err := draw2d.ParseHex(text); err == nil {
				return colour, true
			}
		}
	}
	return draw2d.Color{}, false
}

// or is the first of the named colours the style gives, else the fallback.
func (given style) or(fallback draw2d.Color, names ...string) draw2d.Color {
	if colour, found := given.colour(names...); found {
		return colour
	}
	return fallback
}

// player is a colour of the first player, the person reading: their cursor or their selection.
func (given style) player(name string) (draw2d.Color, bool) {
	players, _ := given["players"].([]any)
	if len(players) == 0 {
		return draw2d.Color{}, false
	}
	first, _ := players[0].(map[string]any)
	return style(first).colour(name)
}

// syntax are the colours of the code, by role: each role is a name of Zed's syntax.
func (given style) syntax(plain draw2d.Color) map[highlight.Role]draw2d.Color {
	colours := map[highlight.Role]draw2d.Color{highlight.Plain: plain}
	roles, _ := given["syntax"].(map[string]any)
	for name, value := range roles {
		entry, _ := value.(map[string]any)
		if colour, found := style(entry).colour("color"); found {
			colours[highlight.Role(name)] = colour
		}
	}
	return colours
}

// lookOf makes a look of a theme's style, and tells whether it could: the text and the editor's
// ground are needed, the rest is made from them when the style leaves it out.
func lookOf(given style, dark bool) (Look, bool) {
	ink, hasInk := given.colour("text", "editor.foreground")
	editor, hasEditor := given.colour("editor.background", "background")
	if !hasInk || !hasEditor {
		return Look{}, false
	}
	colours := paletteOf(given, ink, editor)
	syntax := given.syntax(ink)
	if dark {
		colours.shadow, colours.scrim = draw2d.MustHex("#00000066"), draw2d.MustHex("#0000004d")
		colours.lineWash, colours.wordWash = 0.12, 0.32
		return colours.look(bubble.ModeDark, syntax), true
	}
	colours.shadow, colours.scrim = draw2d.MustHex("#0000001f"), draw2d.MustHex("#00000026")
	colours.lineWash, colours.wordWash = 0.14, 0.30
	colours.deepenInks(syntax)
	return colours.look(bubble.ModeLight, syntax), true
}

// paletteOf are the colours of the interface a style gives, those it leaves out made from its ink
// and its editor's ground; the shadows and washes are the look's darkness's.
func paletteOf(given style, ink, editor draw2d.Color) palette {
	// the grounds are painted opaque: one a theme gives see-through, for an editor that blurs
	// the desktop behind its window (Dracula's, in Zed), is laid over the editor's ground instead
	surface := over(given.or(editor, "surface.background", "panel.background", "background"), editor)
	accent := given.or(draw2d.MustHex("#3574f0"), "text.accent", "icon.accent")
	if cursor, found := given.player("cursor"); found && !has(given, "text.accent", "icon.accent") {
		accent = cursor
	}
	selection, found := given.player("selection")
	if !found {
		selection = accent.WithAlpha(0.24)
	}
	colours := palette{
		background: over(given.or(surface, "background", "panel.background"), editor), surface: surface, editor: editor,
		hover:  given.or(surface.Mix(ink, 0.06), "element.hover", "ghost_element.hover"),
		chosen: given.or(surface.Mix(ink, 0.12), "element.selected", "ghost_element.selected"),
		border: given.or(surface.Mix(ink, 0.14), "border"),
		ink:    ink, inkSoft: given.or(ink.Mix(editor, 0.25), "text.muted"),
		accent: accent, onAccent: editor, accentTint: editor.Mix(accent, 0.25),
		created:   given.or(draw2d.MustHex("#5fa860"), "created", "version_control.added"),
		modified:  given.or(draw2d.MustHex("#d0a540"), "modified", "version_control.modified"),
		deleted:   given.or(draw2d.MustHex("#d6605a"), "deleted", "version_control.deleted"),
		selection: selection,
	}
	colours.borderSoft = given.or(colours.border, "border.variant")
	colours.focus = given.or(accent, "border.focused")
	colours.inkQuiet = given.or(ink.Mix(editor, 0.45), "text.placeholder", "text.disabled")
	colours.lineNumber = given.or(colours.inkQuiet, "editor.line_number")
	colours.activeLineNumber = given.or(ink, "editor.active_line_number")
	colours.scrollThumb = given.or(ink.WithAlpha(0.3), "scrollbar.thumb.background")
	return colours
}

// over is a colour as it shows laid over an opaque ground: itself when it is opaque, else mixed
// with the ground by how much it covers it.
func over(colour, ground draw2d.Color) draw2d.Color {
	covering := colour[3]
	colour[3] = 1 // WithAlpha scales the opacity: set it whole instead
	return ground.Mix(colour, covering)
}

// has tells whether the style names any of these colours.
func has(given style, names ...string) bool {
	_, found := given.colour(names...)
	return found
}

// replaced puts a theme in the place of the one with its name, or adds it.
func replaced(themes []Theme, theme Theme) []Theme {
	for index, each := range themes {
		if each.Name == theme.Name {
			themes[index] = theme
			return themes
		}
	}
	return append(themes, theme)
}

// sortThemes puts the dark themes first, then the light ones, each by name.
func sortThemes(themes []Theme) {
	sort.SliceStable(themes, func(first, second int) bool {
		if themes[first].Dark != themes[second].Dark {
			return themes[first].Dark
		}
		return themes[first].Name < themes[second].Name
	})
}
