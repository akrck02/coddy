package language_test

import (
	"testing"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/language"
	"github.com/akrck02/code-review/internal/theme"
)

// unhighlighted are the languages the highlighter does not know: their files stay plain text.
var unhighlighted = map[string]bool{"Plain text": true, "Less": true, "Configuration": true}

// Every part of the reader reads a language from the table, so a file of any extension of it is
// highlighted, iconed and served alike.
func TestEveryLanguageOfTheTableIsHighlightedIconedAndServedTheSameWay(test *testing.T) {
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	for _, known := range language.Languages {
		names := append([]string{}, known.FileNames...)
		for _, extension := range known.Extensions {
			names = append(names, "lighthouse"+extension)
		}
		for _, name := range names {
			if icon := theme.FileIcon(name, look); known.Icon != "" && icon.Name != known.Icon {
				test.Errorf("%s (%s) is drawn as %q, the table says %q", name, known.Name, icon.Name, known.Icon)
			}
			if served := language.ServerLanguageOf(name); served != known.ServerLanguage {
				test.Errorf("%s (%s) is served as %q, the table says %q", name, known.Name, served, known.ServerLanguage)
			}
			if coloured := highlight.Language(name, ""); coloured == "Plain text" && !unhighlighted[known.Name] {
				test.Errorf("%s (%s) is not highlighted", name, known.Name)
			}
		}
	}
}
