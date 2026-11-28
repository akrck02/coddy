package controls

import (
	"github.com/akrck02/coddy/editor/internal/theme"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/coddy/editor/internal/icons"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
)

// harbourFolder is the made-up icon theme of the icons package's tests.
var harbourFolder = filepath.Join("..", "icons", "testdata", "harbour-icons")

// withHarbourIcons is the reader's dark look with the made-up icon theme, its problems kept.
func withHarbourIcons(test *testing.T) (theme.Look, *[]error) {
	test.Helper()
	content, err := os.ReadFile(filepath.Join(harbourFolder, "icon_themes", "harbour.json"))
	if err != nil {
		test.Fatal(err)
	}
	themes, err := icons.ReadThemes(content, harbourFolder)
	if err != nil || len(themes) != 1 {
		test.Fatalf("the made-up set reads into %d icon themes: %v", len(themes), err)
	}
	told := &[]error{}
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	look.Icons, look.Pictures = &themes[0], icons.NewPictures(func(problem error) { *told = append(*told, problem) })
	return look, told
}

// picture is a path of the made-up set, as its icon theme gives it.
func picture(name string) string {
	return filepath.Join(harbourFolder, "icons", name+".svg")
}

func TestWithoutAnIconThemeTheFilesKeepTodaysIcons(test *testing.T) {
	look := theme.Find(theme.BuiltIn(), theme.DefaultDark, true).Look
	icon := FileIcon("lighthouse.go", look)
	if icon.Name != "file-code" || icon.Picture != "" {
		test.Errorf("a Go file is drawn with %+v, want Lucide's file-code and no picture", icon)
	}
	var list draw2d.List
	DrawIcon(&list, look, icon, 0, 0, 16)
	if len(list.Commands) != 1 || list.Commands[0].Kind != draw2d.IconKind || list.Commands[0].Text != "file-code" {
		test.Errorf("drawing it gives %+v, want Lucide's icon", list.Commands)
	}
}

func TestAnIconThemesPictureIsDrawnSharpAndKept(test *testing.T) {
	look, _ := withHarbourIcons(test)
	icon := FileIcon("lighthouse.go", look)
	if icon.Picture != picture("go") || icon.Name != "file-code" {
		test.Errorf("a Go file is %+v, want the theme's picture and Lucide's icon behind it", icon)
	}
	var list draw2d.List
	DrawIcon(&list, look, icon, 0, 0, 16)
	if len(list.Commands) != 1 || list.Commands[0].Kind != draw2d.ImageKind {
		test.Fatalf("drawing it gives %+v, want its picture", list.Commands)
	}
	if width, _ := list.Commands[0].Picture.Size(); width != 16*pictureDensity {
		test.Errorf("the picture is %d pixels wide, want %d, twice its size for a dense screen", width, 16*pictureDensity)
	}
	first := list.Commands[0].Picture
	list.Reset()
	DrawIcon(&list, look, icon, 0, 0, 16)
	if list.Commands[0].Picture != first {
		test.Error("the picture is drawn again on the next frame, not kept")
	}
}

func TestAMissingOrBrokenIconFallsBackAndIsToldByItsCode(test *testing.T) {
	look, told := withHarbourIcons(test)
	for _, path := range []string{picture("broken"), picture("not-there")} {
		icon := icons.Icon{Name: "file", Ink: look.Theme.InkQuiet, Picture: path, Opacity: 1}
		var list draw2d.List
		for range 2 { // told once, however often it is drawn
			list.Reset()
			DrawIcon(&list, look, icon, 0, 0, 16)
		}
		if len(list.Commands) != 1 || list.Commands[0].Kind != draw2d.IconKind {
			test.Errorf("%s is drawn as %+v, want Lucide's icon instead", path, list.Commands)
		}
	}
	if len(*told) != 2 || !failure.Is((*told)[0], icons.NotDrawn) || !failure.Is((*told)[1], icons.NotDrawn) {
		test.Errorf("the two pictures not drawn are told as %v, want each once with its code", *told)
	}
}

// Drawing a listed file's icon runs every frame: once its picture is drawn, it allocates nothing.
func TestDrawingAKnownIconAllocatesNothing(test *testing.T) {
	look, _ := withHarbourIcons(test)
	icon := FileIcon("lighthouse.go", look)
	list := draw2d.List{Commands: make([]draw2d.Command, 0, 8)}
	DrawIcon(&list, look, icon, 0, 0, 16)
	allocations := testing.AllocsPerRun(100, func() {
		list.Reset()
		DrawIcon(&list, look, FileIcon("lighthouse.go", look), 0, 0, 16)
	})
	if allocations != 0 {
		test.Errorf("drawing a known icon allocates %v times a frame", allocations)
	}
}
