package theme

import (
	"slices"
	"testing"
)

func TestAKeyIsDrawnAsACapForEachKeyPressedTogether(test *testing.T) {
	for key, caps := range map[string][]string{
		"⌘⇧F":          {"⌘", "⇧", "F"},
		"Shift+Space":  {"Shift", "Space"},
		"⌘+":           {"⌘", "+"},
		"⌘⇧+":          {"⌘", "⇧", "+"},
		"?":            {"?"},
		"PgUp":         {"PgUp"},
		"Double click": {"Double click"},
	} {
		if got := keyParts(key); !slices.Equal(got, caps) {
			test.Errorf("%q is drawn as %q, not %q", key, got, caps)
		}
	}
}

func TestAModifierIsDrawnAsTheSymbolOfItsSystem(test *testing.T) {
	cases := []struct{ part, system, icon string }{
		{"⌘", "darwin", "command"}, {"⌥", "darwin", "option"}, {"⇧", "linux", "arrow-big-up"},
		{"←", "linux", "arrow-left"}, {"F", "linux", ""},
	}
	for _, each := range cases {
		if got := capIcon(each.part, each.system); got != each.icon {
			test.Errorf("on %s, %s is drawn as %q, not %q", each.system, each.part, got, each.icon)
		}
	}
}

// Off a Mac, keyboards print words on their modifiers: Coddy's ⌘ is the key marked Ctrl there.
func TestAModifierOffAMacIsWrittenAsItsKeyIsMarked(test *testing.T) {
	cases := []struct{ part, system, name string }{
		{"⌘", "linux", "Ctrl"}, {"⌃", "windows", "Ctrl"}, {"⌥", "linux", "Alt"}, {"F", "linux", "F"},
	}
	for _, each := range cases {
		if icon, name := capIcon(each.part, each.system), capName(each.part, each.system); icon != "" || name != each.name {
			test.Errorf("on %s, %s is drawn as %q %q, not the word %q", each.system, each.part, icon, name, each.name)
		}
	}
}
