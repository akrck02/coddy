package window

import (
	"testing"

	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/uitest"
)

func TestF11AsksForTheWholeScreenAndThenForAWindowAgain(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	var context *ui.Context
	build := func(built *ui.Context) {
		context = built
		application.Build(built)
	}
	window, err := uitest.New(build, uitest.Driver(application.Driver()), uitest.Size(1200, 800))
	if err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	press(window, input.KeyF11)
	if !context.Fullscreen() {
		test.Error("F11 does not ask for the whole screen")
	}
	press(window, input.KeyF11)
	if context.Fullscreen() {
		test.Error("F11 on the whole screen does not ask for a window again")
	}
}
