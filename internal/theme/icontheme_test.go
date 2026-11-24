package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-renderer/failure"
)

// harbourIcons is the made-up icon theme of testdata, read as a Zed extension's.
func harbourIcons(test *testing.T) IconTheme {
	test.Helper()
	folder := filepath.Join("testdata", "harbour-icons")
	content, err := os.ReadFile(filepath.Join(folder, "icon_themes", "harbour.json"))
	if err != nil {
		test.Fatal(err)
	}
	themes, err := ReadIconThemes(content, folder)
	if err != nil || len(themes) != 1 {
		test.Fatalf("the made-up set reads into %d icon themes: %v", len(themes), err)
	}
	return themes[0]
}

// picture is a path of the made-up set, as an icon theme gives it.
func picture(name string) string {
	return filepath.Join("testdata", "harbour-icons", "icons", name+".svg")
}

func TestAnIconThemeNamesTheIconOfEachFileAndFolder(test *testing.T) {
	icons := harbourIcons(test)
	if icons.Name != "Harbour" || icons.Dark != true {
		test.Errorf("the icon theme is %q, dark %v; want Harbour, dark", icons.Name, icons.Dark)
	}
	checks := map[string][2]string{
		"a Go file":        {icons.FilePicture("lighthouse.go"), picture("go")},
		"an unknown file":  {icons.FilePicture("notes.unknown"), picture("file")},
		"a named folder":   {icons.FolderPicture("docs", false), picture("folder-docs")},
		"it open":          {icons.FolderPicture("docs", true), picture("folder-docs-open")},
		"another folder":   {icons.FolderPicture("harbour", false), picture("folder")},
		"another one open": {icons.FolderPicture("harbour", true), picture("folder-open")},
		"a closed chevron": {icons.ChevronPicture(false), picture("chevron-right")},
		"an open chevron":  {icons.ChevronPicture(true), picture("chevron-down")},
	}
	for what, check := range checks {
		if check[0] != check[1] {
			test.Errorf("%s is drawn with %q, want %q", what, check[0], check[1])
		}
	}
}

// Files are matched as Zed matches them, so a set looks here as it does in Zed.
func TestAFileTakesTheIconOfItsNameThenOfItsLongestExtension(test *testing.T) {
	icons := harbourIcons(test)
	for name, want := range map[string]string{
		"Makefile":        "make",       // its whole name
		"tide.spec.ts":    "spec",       // its longest extension first
		"tide.ts":         "typescript", // then a shorter one
		"Dockerfile.prod": "docker",     // what comes before its first dot
		"MAKEFILE":        "file",       // names are matched as they are written
		"no-extension":    "file",
		".hidden.go":      "go",
	} {
		if got := icons.FilePicture(name); got != picture(want) {
			test.Errorf("%s is drawn with %q, want the %s icon", name, got, want)
		}
	}
}

func TestABrokenIconThemeFileIsToldByItsCode(test *testing.T) {
	for content, code := range map[string]*failure.Code{"{": NotAnIconThemeFile, `{"themes": []}`: NoIconThemes} {
		if _, err := ReadIconThemes([]byte(content), "anywhere"); !failure.Is(err, code) {
			test.Errorf("%q gives %v, not %s", content, err, code.Name())
		}
	}
}
