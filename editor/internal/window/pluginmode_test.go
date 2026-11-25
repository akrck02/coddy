package window

import (
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-ui/uitest"
)

// fakeModeName is the rail's tooltip of the fake plugin's mode.
const fakeModeName = "Fake screen: a mode of the fake plugin"

// openFakeMode is a window with the fake plugin's mode on the rail, turned on from it.
func openFakeMode(test *testing.T) (*Window, *uitest.Window) {
	test.Helper()
	plugin := fakePlugin(test)
	plugin.Contributes.Mode = &plugins.ModeContribution{Name: "Fake screen", Icon: "list-todo"}
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	if err := window.Click(fakeModeName); err != nil {
		test.Fatalf("the rail shows no button for the plugin's mode: %v", err)
	}
	window.Frames(3)
	return application, window
}

// paintedText is the text the last frame painted that starts with prefix, and where.
func paintedText(window *uitest.Window, prefix string) (draw2d.Command, bool) {
	for _, command := range window.DrawList().Commands {
		if command.Kind == draw2d.TextKind && strings.HasPrefix(command.Text, prefix) {
			return command, true
		}
	}
	return draw2d.Command{}, false
}

func TestAPluginsModeIsPaintedWhereTheViewerIs(test *testing.T) {
	application, window := openFakeMode(test)
	count, found := paintedText(window, "Count")
	if !found {
		test.Fatal("the plugin's screen is not painted")
	}
	windowWidth, _ := window.Context().Size()
	if left := windowWidth - application.viewerWidth(windowWidth); count.Area.X < left {
		test.Errorf("the plugin's button is at x %v, left of the viewer, which starts at %v", count.Area.X, left)
	}
}

func TestAClickOnAPluginsScreenReachesThePlugin(test *testing.T) {
	_, window := openFakeMode(test)
	count, _ := paintedText(window, "Count")
	window.ClickAt(count.Area.X+count.Area.Width/2, count.Area.Y+count.Area.Height/2)
	window.Frames(3)
	if line, _ := paintedText(window, "frame"); !strings.HasSuffix(line.Text, "clicked 1") {
		test.Errorf("after a click the plugin shows %q", line.Text)
	}
}

func TestAStillPluginScreenAsksThePluginForNothing(test *testing.T) {
	_, window := openFakeMode(test)
	before, _ := paintedText(window, "frame")
	window.Frames(5)
	after, found := paintedText(window, "frame")
	if !found || after.Text != before.Text {
		test.Errorf("idle frames show %q, then %q: the plugin was asked again, or nothing is shown", before.Text, after.Text)
	}
}
