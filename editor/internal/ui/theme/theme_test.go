package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
)

func TestTheReaderComesWithTheThemesOfZedIntelliJVisualStudioCodeDraculaAndMrPink(test *testing.T) {
	themes := BuiltIn()
	want := []string{"Dark Modern", "Dracula Solid", "IntelliJ Dark", "Mr Pink", "One Dark", "IntelliJ Light", "One Light"}
	if len(themes) != len(want) {
		test.Fatalf("the reader comes with %d themes, want %v", len(themes), want)
	}
	for index, name := range want {
		if themes[index].Name != name {
			test.Errorf("theme %d is %q, want %q (the dark ones first, each by name)", index, themes[index].Name, name)
		}
	}
	modern := Find(themes, "Dark Modern", true)
	if modern.Look.Editor != draw2d.MustHex("#1f1f1f") || !modern.Dark {
		test.Errorf("Dark Modern's editor is %v, want #1f1f1f", modern.Look.Editor)
	}
}

func TestTheLightLookDeepensItsInks(test *testing.T) {
	light := Find(BuiltIn(), DefaultLight, false).Look.Theme.Ink
	if light[0] >= 0x24/255.0 {
		test.Errorf("the light look's ink %v is not deeper than Zed's", light)
	}
}

func TestAThemeWithOnlyItsTextAndEditorIsMadeWhole(test *testing.T) {
	themes, err := Read([]byte(`{"name": "Bare", "themes": [{"name": "Bare", "appearance": "dark",
		"style": {"text": "#e0e0e0ff", "editor.background": "#101010ff", "syntax": {"keyword": {"color": "#ff8800"}, "comment": {"color": null}}}}]}`))
	if err != nil {
		test.Fatal(err)
	}
	look := themes[0].Look
	if look.Editor != draw2d.MustHex("#101010") || look.Theme.Ink != draw2d.MustHex("#e0e0e0") {
		test.Errorf("the bare theme paints the editor %v and the text %v", look.Editor, look.Theme.Ink)
	}
	if look.Hover == (draw2d.Color{}) || look.Created == (draw2d.Color{}) || look.LineNumber == (draw2d.Color{}) {
		test.Error("the colours the bare theme leaves out are not made up")
	}
}

// A theme for an editor that blurs the desktop behind its window gives its window's ground
// see-through; the reader lays it over the editor's ground, so the window is not painted in it bare.
func TestASeeThroughGroundIsLaidOverTheEditor(test *testing.T) {
	themes, err := Read([]byte(`{"name": "Glass", "themes": [{"name": "Glass", "appearance": "dark",
		"style": {"text": "#e0e0e0ff", "editor.background": "#000000ff", "background": "#ffffff33"}}]}`))
	if err != nil {
		test.Fatal(err)
	}
	ground := themes[0].Look.Theme.Background
	if ground[3] != 1 || ground[0] > 0.25 {
		test.Errorf("a white ground a fifth opaque over a black editor is painted %v, not a dark grey", ground)
	}
}

func TestAFileThatIsNotAThemeIsToldAndLeftOut(test *testing.T) {
	folder := test.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	write("broken.json", "{")
	write("no-text.json", `{"themes": [{"name": "Empty", "style": {}}]}`)
	write("mine.json", `{"themes": [{"name": "Dark Modern", "appearance": "dark", "style": {"text": "#ffffff", "editor.background": "#000000"}}]}`)
	themes, problems := Load(folder)
	if len(problems) != 2 {
		test.Errorf("reading the folder told %d problems, want 2: %v", len(problems), problems)
	}
	for _, problem := range problems {
		if !failure.Is(problem, NotAThemeFile) {
			test.Errorf("the problem %v has not the code of a file that is not a theme", problem)
		}
	}
	if len(problems) == 2 && !failure.Is(problems[1], MissingColours) {
		test.Errorf("the theme without colours is told as %v, not as missing its colours", problems[1])
	}
	if len(themes) != len(BuiltIn()) {
		test.Errorf("the folder gives %d themes, want the reader's %d with Dark Modern replaced", len(themes), len(BuiltIn()))
	}
	if mine := Find(themes, "Dark Modern", true); mine.Look.Editor != draw2d.MustHex("#000000") || mine.File == "" {
		test.Errorf("a theme of the folder does not take the place of the reader's one of its name: %+v", mine.File)
	}
}

// Zed's own theme files, when a test is given them (CODDY_ZED_THEMES, a folder of them), are read
// whole: every theme of every file.
func TestZedsThemeFilesAreRead(test *testing.T) {
	folder := os.Getenv("CODDY_ZED_THEMES")
	if folder == "" {
		test.Skip("CODDY_ZED_THEMES names no folder of Zed's theme files")
	}
	themes, problems := Load(folder)
	if len(problems) > 0 {
		test.Fatal(problems)
	}
	for _, each := range themes {
		test.Logf("%s (dark %t, from %q): editor %v", each.Name, each.Dark, filepath.Base(each.File), each.Look.Editor)
	}
}
