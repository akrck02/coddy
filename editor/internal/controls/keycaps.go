package controls

import (
	"runtime"
	"strings"

	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// KeyCapHeight is a key cap's height, and keyIconSize a symbol's on it, in pixels.
const (
	KeyCapHeight = 24
	keyIconSize  = 13
)

// KeyCaps draws a key as caps, one for each key pressed together: "⌘⇧F" is ⌘, ⇧ and F, the
// modifiers and arrows drawn as symbols. What is not a key ("Double click") is written as it is.
func KeyCaps(context *ui.Context, key string) {
	context.Box(ui.Layout{Gap: 4, AlignY: ui.Center}, func() {
		for _, part := range keyParts(key) {
			if strings.Contains(part, " ") {
				context.Text(part, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft})
				continue
			}
			keyCap(context, capName(part, runtime.GOOS), capIcon(part, runtime.GOOS))
		}
	})
}

// keyIcons are the symbols drawn for keys on every system: the arrows and Shift, by its symbol
// or by its name.
var keyIcons = map[string]string{"⇧": "arrow-big-up", "Shift": "arrow-big-up", "←": "arrow-left", "→": "arrow-right", "↑": "arrow-up", "↓": "arrow-down"}

// macIcons are the symbols of a Mac's own modifiers there; offMacNames are the names written on
// the keys they stand for elsewhere, where keyboards print words: the Command key's shortcuts
// take Control off a Mac, and Option is Alt.
var (
	macIcons    = map[string]string{"⌘": "command", "⌃": "chevron-up", "⌥": "option"}
	offMacNames = map[string]string{"⌘": "Ctrl", "⌃": "Ctrl", "⌥": "Alt"}
)

// capIcon is the symbol a key's cap shows on a system, or "" for a key written as its name.
func capIcon(part, system string) string {
	if icon, found := keyIcons[part]; found {
		return icon
	}
	if system == "darwin" {
		return macIcons[part]
	}
	return ""
}

// capName is the word a key's cap shows on a system, when it shows no symbol: Ctrl and Alt for
// the modifiers off a Mac, the key as the help writes it otherwise.
func capName(part, system string) string {
	if name, found := offMacNames[part]; found && system != "darwin" {
		return name
	}
	return part
}

// keyCap is one key drawn as a cap: its symbol, or its name, on a raised, rounded key.
func keyCap(context *ui.Context, label, icon string) {
	palette := context.Theme
	look := ui.Look{Fill: palette.Background, Radius: 6, Border: palette.ControlLine, BorderWidth: 1}
	layout := ui.Layout{Height: ui.Fixed(KeyCapHeight), Padding: ui.PadXY(8, 0), AlignX: ui.Center, AlignY: ui.Center}
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

// KeyCapWords are the words a key's caps show on this system, one after the other: "⌘⇧F" shows F on
// every system, as ⌘ and ⇧ are drawn as symbols. Tests find a key in the screen's text with it.
func KeyCapWords(key string) string {
	var words []string
	for _, part := range keyParts(key) {
		if capIcon(part, runtime.GOOS) == "" {
			words = append(words, capName(part, runtime.GOOS))
		}
	}
	return strings.Join(words, " ")
}

// ModifierName is how a modifier is written in a sentence on this system: ⌘ and ⌥ on a Mac, Ctrl
// and Alt elsewhere, as the keys there are marked.
func ModifierName(modifier string) string { return capName(modifier, runtime.GOOS) }
