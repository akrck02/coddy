package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-renderer/failure"
)

// extension writes an extension folder with one icon theme file, as Zed installs them.
func extension(test *testing.T, root, name, content string) {
	test.Helper()
	folder := filepath.Join(root, name, "icon_themes")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, name+".json"), []byte(content), 0o644); err != nil {
		test.Fatal(err)
	}
}

func TestTheIconThemesOfTheFolderAndOfZedsExtensionsAreListed(test *testing.T) {
	own, zeds := test.TempDir(), test.TempDir()
	extension(test, own, "tides", `{"themes": [{"name": "Tides", "appearance": "light"}]}`)
	extension(test, zeds, "lanterns", `{"themes": [{"name": "Lanterns Dark"}, {"name": "Lanterns Light", "appearance": "light"}]}`)
	extension(test, zeds, "broken", `{`)
	themes, problems := LoadIconThemes(own, zeds, filepath.Join(own, "not-there"))
	var names []string
	for _, each := range themes {
		names = append(names, each.Name)
	}
	if want := []string{"Lanterns Dark", "Lanterns Light", "Tides"}; len(names) != len(want) || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		test.Errorf("the icon themes listed are %v, want %v", names, want)
	}
	if len(themes) > 0 && themes[0].Folder != filepath.Join(zeds, "lanterns") {
		test.Errorf("an icon theme's pictures start at %q, not at its extension's folder", themes[0].Folder)
	}
	if len(problems) != 1 || !failure.Is(problems[0], NotAnIconThemeFile) {
		test.Errorf("the broken file is told as %v, want one problem with its code", problems)
	}
}

// Zed keeps its installed extensions in a folder of its own on each system.
func TestZedsExtensionsAreLookedForWhereZedKeepsThem(test *testing.T) {
	cases := []struct{ system, dataHome, want string }{
		{"darwin", "", "/home/sea/Library/Application Support/Zed/extensions/installed"},
		{"linux", "", "/home/sea/.local/share/zed/extensions/installed"},
		{"linux", "/data", "/data/zed/extensions/installed"},
	}
	for _, each := range cases {
		if got := zedExtensionsFolder(each.system, "/home/sea", each.dataHome); got != each.want {
			test.Errorf("on %s (data home %q) Zed's extensions are looked for in %q, want %q", each.system, each.dataHome, got, each.want)
		}
	}
}
