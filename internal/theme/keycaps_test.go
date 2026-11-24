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
		{"⌘", "darwin", "command"}, {"⌘", "linux", "chevron-up"}, {"⇧", "linux", "arrow-big-up"},
		{"⌥", "windows", "option"}, {"←", "linux", "arrow-left"}, {"F", "linux", ""},
	}
	for _, each := range cases {
		if got := capIcon(each.part, each.system); got != each.icon {
			test.Errorf("on %s, %s is drawn as %q, not %q", each.system, each.part, got, each.icon)
		}
	}
}
