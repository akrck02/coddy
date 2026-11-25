package icons

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/akrck02/go-renderer/failure"
)

// ThemesFolder is where a person's own icon themes are read from: icon-themes/ in the
// reader's own folder, beside themes/, each in a folder of its own as Zed's extensions are.
func ThemesFolder() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "coddy", "icon-themes")
}

// ZedExtensionsFolder is where Zed keeps the extensions installed in it, so that an icon theme
// installed in Zed is listed here too; "" without a home.
func ZedExtensionsFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return zedExtensionsFolder(runtime.GOOS, home, os.Getenv("XDG_DATA_HOME"))
}

// zedExtensionsFolder is Zed's folder of installed extensions on a system: in Application
// Support on a Mac, in the data home elsewhere (XDG's, or ~/.local/share).
func zedExtensionsFolder(system, home, dataHome string) string {
	if system == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Zed", "extensions", "installed")
	}
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "zed", "extensions", "installed")
}

// LoadThemes are the icon themes of the extensions in some folders: every folder in them
// with icon_themes/*.json, its pictures' paths starting at that folder. A file that cannot be
// read is told in problems and left out; a folder that is not there has none.
func LoadThemes(roots ...string) (themes []Theme, problems []error) {
	for _, root := range roots {
		if root == "" {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(root, "*", "icon_themes", "*.json")) // only a malformed pattern errs
		for _, path := range files {
			read, err := readThemeFile(path)
			if err != nil {
				problems = append(problems, failure.Wrap(NotAThemeFile, path, err))
			}
			themes = append(themes, read...)
		}
	}
	sort.SliceStable(themes, func(one, other int) bool { return themes[one].Name < themes[other].Name })
	return themes, problems
}

// readThemeFile reads a file of icon themes, its pictures' paths starting at its extension's
// folder: the one above icon_themes/.
func readThemeFile(path string) ([]Theme, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadThemes(content, filepath.Dir(filepath.Dir(path)))
}
