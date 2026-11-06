package window

import (
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
)

// keeperUse is "Keeper" in harbour's line var keeper = lamp.Keeper{}.
var keeperUse = viewer.Place{Line: 3, Column: 20}

func TestReferencesAreListed(test *testing.T) {
	application, window := followingNames(test)
	application.viewer.PlaceCaret(keeperUse)
	press(window, input.KeyF12, input.KeyShift)
	found := application.finder
	if !found.Open || found.Scope != finder.ReferencesScope || found.Name != "uses of Keeper" || len(found.Hits) != 3 {
		test.Fatalf("⇧F12 listed %d uses of %q (open %t), want the 3 of Keeper", len(found.Hits), found.Name, found.Open)
	}
	typeText(window, "harbour")
	if len(application.finder.Hits) != 1 {
		test.Fatalf("typing harbour kept %d uses, want 1", len(application.finder.Hits))
	}
	press(window, input.KeyEnter)
	if !isAt(application, "harbour", "src/use.go", 4) {
		test.Errorf("Enter opened %s at line %d, want harbour's src/use.go at line 4",
			application.viewer.Reference.Path, application.viewer.MarkedLine)
	}
}

func TestHoverShowsTheServersText(test *testing.T) {
	application, window := followingNames(test)
	window.MoveTo(application.viewer.PointOf(keeperUse))
	window.Frames(3)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "Keeper from the fake server") {
		test.Fatalf("resting on Keeper shows no card: %q", application.hover.text)
	}
	window.MoveTo(application.viewer.PointOf(viewer.Place{Line: 0, Column: 2}))
	window.Frames(3)
	if shown := strings.Join(window.Texts(), " "); strings.Contains(shown, "Keeper from the fake server") {
		test.Error("the card stays after the pointer left Keeper")
	}
}

func TestCommandClickGoesToTheDeclaration(test *testing.T) {
	application, window := followingNames(test)
	x, y := application.viewer.PointOf(keeperUse)
	window.HoldKey(input.KeySuper)
	window.ClickAt(x, y)
	window.ReleaseKey(input.KeySuper)
	window.Frames(2)
	if !isAt(application, "lighthouse", "src/keeper.go", 4) {
		test.Errorf("⌘-click opened %s at line %d, want lighthouse's src/keeper.go at line 4",
			application.viewer.Reference.Path, application.viewer.MarkedLine)
	}
}
