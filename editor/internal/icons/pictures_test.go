package icons

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-renderer/failure"
)

// A PNG keeps its own pixels whatever size it is drawn at, and one that cannot be read is told by
// its code, as an SVG is.
func TestAPNGIsDrawnAsItIsAndABrokenOneIsTold(test *testing.T) {
	folder := test.TempDir()
	path := filepath.Join(folder, "icon.png")
	file, err := os.Create(path)
	if err != nil {
		test.Fatal(err)
	}
	if err := png.Encode(file, image.NewNRGBA(image.Rect(0, 0, 6, 4))); err != nil {
		test.Fatal(err)
	}
	if err := file.Close(); err != nil {
		test.Fatal(err)
	}
	var told []error
	kept := NewPictures(func(problem error) { told = append(told, problem) })
	small, big := kept.Picture(path, 16), kept.Picture(path, 80)
	if small == nil || small != big {
		test.Fatalf("the PNG is drawn as %p at 16 pixels and %p at 80, want one picture", small, big)
	}
	if width, height := small.Size(); width != 6 || height != 4 {
		test.Errorf("the PNG is %d×%d, want its own 6×4", width, height)
	}
	broken := filepath.Join(folder, "broken.png")
	if err := os.WriteFile(broken, []byte("not a picture"), 0o600); err != nil {
		test.Fatal(err)
	}
	if kept.Picture(broken, 16) != nil || len(told) != 1 || !failure.Is(told[0], NotDrawn) {
		test.Errorf("a broken PNG was told as %v, want once with %s", told, NotDrawn.Name())
	}
}
