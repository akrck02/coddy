package controls

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/theme"
)

func TestAFileIsKnownByItsNameBeforeItsExtension(test *testing.T) {
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	cases := map[string]string{
		"lighthouse/Lamp.java":   "coffee",
		"build.gradle.kts":       "file-cog",
		"docs/README.md":         "book-open",
		"Dockerfile.release":     "container",
		"go.sum":                 "file-lock",
		"notes/without-a-kind.q": "file",
	}
	for name, want := range cases {
		if icon := FileIcon(name, look); icon.Name != want {
			test.Errorf("FileIcon(%q) = %q, want %q", name, icon.Name, want)
		}
	}
}
