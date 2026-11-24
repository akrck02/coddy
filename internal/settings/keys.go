package settings

import (
	"runtime"
	"strings"

	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// keysListWidth is the widest the list of keys grows, so a command and its keys stay close
	keysListWidth = 860
	// keyCapsWidth is the room of a command's keys, on the right of its row
	keyCapsWidth = 300
	// keyCapHeight is a key cap's height, and keyIconSize a symbol's on it
	keyCapHeight = 24
	keyIconSize  = 13
)

// KeyGroup is the keys of one place: its name, and a row for each command.
type KeyGroup struct {
	Title string
	Rows  []KeyRow
}

// KeyRow is a command as the keys section shows it: its keys, those that do the same, and what it
// does.
type KeyRow struct {
	Keys        []string
	Description string
}

// buildKeys is the keys section: every command with its keys drawn as caps, grouped by where it
// works, each group a card of rows.
func buildKeys(context *ui.Context, window Window) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(keysListWidth), Gap: 24}, func() {
		context.Text("The reader writes a file only when you save it, and git only reads: no commit, no push.",
			ui.TextStyle{Color: palette.InkSoft})
		for _, group := range window.KeyGroups() {
			buildKeysGroup(context, group)
		}
	})
}

// buildKeysGroup is the keys of one place: its name, then a card with a row for each command, a
// hairline between rows.
func buildKeysGroup(context *ui.Context, group KeyGroup) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 10}, func() {
		context.Text(group.Title, ui.TextStyle{Size: palette.TextSize + 1, Weight: draw2d.Bold})
		card := ui.Look{Fill: palette.Sunken, Radius: 10}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(16, 4)}, card, func() {
			for index, row := range group.Rows {
				if index > 0 {
					context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: palette.Line}, nil)
				}
				buildKeysRow(context, row)
			}
		})
	})
}

// buildKeysRow is a command: what it does on the left, its keys on the right, those that do the
// same apart by a dot.
func buildKeysRow(context *ui.Context, row KeyRow) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 24, AlignY: ui.Center}, func() {
		// go-ui draws a text on one line, so the words wrap as pieces of a box
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, Wrap: true}, func() {
			for _, word := range strings.Fields(row.Description) {
				context.Label(word)
			}
		})
		context.Box(ui.Layout{Width: ui.Fixed(keyCapsWidth), Gap: 8, Wrap: true, AlignX: ui.End, AlignY: ui.Center}, func() {
			for index, key := range row.Keys {
				if index > 0 { // between two keys that do the same, not pressed together
					context.Text("·", ui.TextStyle{Color: palette.InkQuiet})
				}
				buildKeyCaps(context, key)
			}
		})
	})
}

// buildKeyCaps draws a key as caps, one for each key pressed together: "⌘⇧F" is ⌘, ⇧ and F, the
// modifiers and arrows drawn as symbols. What is not a key ("Double click") is written as it is.
func buildKeyCaps(context *ui.Context, key string) {
	context.Box(ui.Layout{Gap: 4, AlignY: ui.Center}, func() {
		for _, part := range keyParts(key) {
			if strings.Contains(part, " ") {
				context.Text(part, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft})
				continue
			}
			buildKeyCap(context, part, capIcon(part, runtime.GOOS))
		}
	})
}

// keyIcons are the symbols drawn for keys on every system: the arrows and Shift, by its symbol
// or by its name.
var keyIcons = map[string]string{"⇧": "arrow-big-up", "Shift": "arrow-big-up", "←": "arrow-left", "→": "arrow-right", "↑": "arrow-up", "↓": "arrow-down"}

// macIcons are the symbols of a Mac's own modifiers there, and offMacIcons those of the keys they
// stand for elsewhere: the Command key's shortcuts take Control (⌃) off a Mac, Option is Alt.
var (
	macIcons    = map[string]string{"⌘": "command", "⌃": "chevron-up", "⌥": "option"}
	offMacIcons = map[string]string{"⌘": "chevron-up", "⌃": "chevron-up", "⌥": "option"}
)

// capIcon is the symbol a key's cap shows on a system, or "" for a key written as its name.
func capIcon(part, system string) string {
	if icon, found := keyIcons[part]; found {
		return icon
	}
	if system == "darwin" {
		return macIcons[part]
	}
	return offMacIcons[part]
}

// buildKeyCap is one key drawn as a cap: its symbol, or its name, on a raised, rounded key.
func buildKeyCap(context *ui.Context, label, icon string) {
	palette := context.Theme
	look := ui.Look{Fill: palette.Background, Radius: 6, Border: palette.ControlLine, BorderWidth: 1}
	layout := ui.Layout{Height: ui.Fixed(keyCapHeight), Padding: ui.PadXY(8, 0), AlignX: ui.Center, AlignY: ui.Center}
	context.Frame(layout, look, func() {
		if icon != "" {
			context.IconSized(icon, keyIconSize, palette.Ink)
			return
		}
		context.Text(label, ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Medium, Color: palette.Ink})
	})
}

// modifierKeys are the keys a shortcut holds down, as the help writes them.
const modifierKeys = "⌘⇧⌥⌃"

// keyParts are the caps of a key as the help writes it: each modifier on its own, then the rest,
// and the keys joined by a plus apart. "⌘⇧F" is ⌘, ⇧ and F; "Shift+Space" is Shift and Space;
// "⌘+" is ⌘ and +.
func keyParts(key string) []string {
	var parts []string
	rest := []rune(key)
	for len(rest) > 1 && strings.ContainsRune(modifierKeys, rest[0]) {
		parts = append(parts, string(rest[0]))
		rest = rest[1:]
	}
	if joined := string(rest); len(rest) > 1 && strings.Contains(joined[1:], "+") && !strings.HasSuffix(joined, "+") {
		return append(parts, strings.Split(joined, "+")...)
	}
	return append(parts, string(rest))
}

// CapWords are the words a key's caps show on this system, one after the other: "⌘⇧F" shows F on
// every system, as ⌘ and ⇧ are drawn as symbols. Tests find a key in the screen's text with it.
func CapWords(key string) string {
	var words []string
	for _, part := range keyParts(key) {
		if capIcon(part, runtime.GOOS) == "" {
			words = append(words, part)
		}
	}
	return strings.Join(words, " ")
}
