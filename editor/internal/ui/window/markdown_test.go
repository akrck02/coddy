package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAMarkdownFileOpensFormattedAndTheEyeShowsItsText(test *testing.T) {
	folder := makeRepository(test)
	text := "# The harbour\n\nA **lamp** and `Keeper`.\n\n- one\n- two\n"
	if err := os.WriteFile(filepath.Join(folder, "lighthouse", "guide.md"), []byte(text), 0o644); err != nil {
		test.Fatal(err)
	}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, Open: "lighthouse/guide.md"})
	window := newWindow(test, application)
	window.Frames(2)
	if !application.viewer.Formatted() || application.viewer.Editable() {
		test.Fatalf("a Markdown file opens formatted %v, editable %v; want it formatted, not editable", application.viewer.Formatted(), application.viewer.Editable())
	}
	if painted := application.viewer.FormattedWords(); !strings.Contains(painted, "The harbour") || strings.Contains(painted, "#") ||
		strings.Contains(painted, "**") || !strings.Contains(painted, "•") {
		test.Errorf("the formatted file paints %q", painted)
	}
	eye, err := window.Element("Show its text, to edit it")
	if err != nil {
		test.Fatal(err)
	}
	window.ClickAt(eye.Rect.X+eye.Rect.Width/2, eye.Rect.Y+eye.Rect.Height/2)
	window.Frames(2)
	if application.viewer.Formatted() || !application.viewer.Editable() {
		test.Errorf("after the eye, the file is formatted %v, editable %v; want its text, to edit", application.viewer.Formatted(), application.viewer.Editable())
	}
}
