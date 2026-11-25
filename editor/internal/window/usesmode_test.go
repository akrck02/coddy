package window

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/coddy/editor/internal/files"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/review"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/uses"
	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/input"
	uitest "github.com/akrck02/go-ui/uitest"
)

// the window is what the uses mode needs to search again and to open a graph further.
var _ uses.Host = (*Window)(nil)

// readerOfUses opens two repositories, harbour using lighthouse's Lamp through a jar, with the
// uses mode, and asks the uses of Lamp where it is declared. Kotlin's server is not installed.
func readerOfUses(test *testing.T) (*Window, *uitest.Window) {
	folder := test.TempDir()
	javaRepository(test, filepath.Join(folder, "lighthouse"), map[string]string{
		"src/main/java/org/lighthouse/Lamp.java": "package org.lighthouse;\n\npublic class Lamp {\n    public int beam() { return 2; }\n}\n",
	})
	javaRepository(test, filepath.Join(folder, "harbour"), map[string]string{
		"src/main/java/org/harbour/Quay.java":  "package org.harbour;\n\nimport org.lighthouse.Lamp;\n\npublic class Quay {\n    Lamp lamp = new Lamp();\n}\n",
		"src/main/java/org/harbour/Other.java": "package org.harbour;\n\npublic class Other {\n}\n",
		"src/main/java/org/harbour/Dock.java":  "package org.harbour;\n\npublic class Dock {\n    Quay quay = new Quay();\n}\n",
		// Kotlin's server is not installed: its use of Lamp is never confirmed
		"src/main/kotlin/org/harbour/Pier.kt": "package org.harbour\n\nimport org.lighthouse.Lamp\n\nval lamp = Lamp()\n",
	})
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"java"}, Environment: []string{"FAKE_LSP_JARS=1"}},
		{Name: "kotlinls", Command: "kotlinls-not-installed", Languages: []string{"kotlin"}}}
	application, err := New(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/main/java/org/lighthouse/Lamp.java", Servers: servers}, files.New(), review.New(""), uses.New())
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	window.Frames(1)
	application.viewer.PlaceCaret(viewer.Place{Line: 2, Column: 14})
	press(window, input.KeyF12, input.KeyShift)
	for frame := 0; frame < 40 && application.uses.Last() != nil && application.uses.Last().Noted() != ""; frame++ {
		window.Frames(1)
	}
	window.Frames(1)
	return application, window
}

// listedFiles are the files the sidebar lists.
func listedFiles(application *Window) string {
	var listed []string
	for _, row := range application.sidebar.Rows {
		if row.Kind == sidebar.FileRow {
			listed = append(listed, row.Label)
		}
	}
	return strings.Join(listed, " ")
}

func TestUsesOpenTheirModeListingOnlyTheFilesWithUses(test *testing.T) {
	application, window := readerOfUses(test)
	if application.mode.Name() != "Uses" {
		test.Fatalf("⇧F12 left the reader in %s", application.mode.Name())
	}
	if listed := listedFiles(application); !strings.Contains(listed, "Quay.java") || strings.Contains(listed, "Other.java") {
		test.Errorf("the sidebar lists %q", listed)
	}
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "uses in 3 files") {
		test.Errorf("the bar over the viewer says %q", shown)
	}
}

func TestF3GoesThroughTheUsesAndAltVTicksThem(test *testing.T) {
	application, window := readerOfUses(test)
	application.focus = sidebarPane
	for step := 0; step < 3 && !strings.HasSuffix(application.viewer.Reference.Path, "Quay.java"); step++ {
		press(window, input.KeyF3)
		window.Frames(1)
	}
	view := &application.viewer
	if !strings.HasSuffix(view.Reference.Path, "Quay.java") || view.Mode != viewer.UsesView || len(view.Uses[6]) != 2 {
		test.Fatalf("F3 opened %s in mode %d with uses %v", view.Reference.Path, view.Mode, view.Uses)
	}
	press(window, input.KeyV, input.KeyAlt)
	window.Frames(1)
	if header := application.powers.header.Header(); !strings.HasPrefix(header, "1 of ") {
		test.Errorf("after v the header says %q", header)
	}
}

func TestTheUsesShowAsGraphsInTheViewerPlace(test *testing.T) {
	application, window := readerOfUses(test)
	application.focus = sidebarPane
	press(window, input.Key2, input.KeyAlt)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "A click opens the file at its first use") ||
		!strings.Contains(shown, "harbour") {
		test.Errorf("⌥2 shows %q", shown)
	}
	press(window, input.Key3, input.KeyAlt)
	window.Frames(2)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "lit: uses Lamp") {
		test.Errorf("⌥3 shows %q", shown)
	}
	press(window, input.Key1, input.KeyAlt)
	if application.mode.Name() != "Uses" || !application.viewer.IsOpen() {
		test.Error("⇧1 did not bring the list back")
	}
}

func TestTheUsersOfTheDeclarationAUseIsInAreFound(test *testing.T) {
	application, window := readerOfUses(test)
	use, found := lastUseIn(application, "Quay.java")
	if !found {
		test.Fatal("no use in Quay.java")
	}
	var declaration string
	var users []mode.Use
	if !application.UsersOf(use, func(name string, found []mode.Use) { declaration, users = name, found }) {
		test.Fatalf("it did not ask: %s", application.toast)
	}
	for frame := 0; frame < 40 && declaration == ""; frame++ {
		window.Frames(1)
	}
	if declaration != "Quay" || len(users) != 2 || !strings.HasSuffix(users[0].File.Path, "Dock.java") {
		test.Errorf("the use %+v is in %q, used by %+v (%s)", use, declaration, users, application.toast)
	}
}

// lastUseIn is the last use the uses mode holds in a file: one in its class, not its imports.
func lastUseIn(application *Window, base string) (mode.Use, bool) {
	holder, _ := application.usesHolder()
	last, found := mode.Use{}, false
	for index, project := range application.session.Projects {
		for _, path := range project.Files {
			reference := workspace.FileReference{Project: index, Path: path}
			for line, spans := range holder.UsesIn(reference) {
				if filepath.Base(path) == base && line > last.Line {
					last, found = mode.Use{File: reference, Line: line, Start: spans[0].Start, End: spans[0].End}, true
				}
			}
		}
	}
	return last, found
}

func TestANoteStaysLongEnoughToReadAndThenGoes(test *testing.T) {
	application, window := readerOfUses(test)
	application.Toast("No one else uses Quay")
	window.Frames(1)
	window.MoveTo(40, 40)
	window.Frames(2)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "No one else uses Quay") {
		test.Fatal("the note went with the mouse")
	}
	application.toastUntil = time.Now()
	window.Frames(1)
	if shown := strings.Join(window.Texts(), " "); strings.Contains(shown, "No one else uses Quay") {
		test.Error("the note did not go in its time")
	}
}

func TestAUseNoServerConfirmsIsShownAsProbable(test *testing.T) {
	application, _ := readerOfUses(test)
	holder, _ := application.usesHolder()
	pier, found := lastUseIn(application, "Pier.kt")
	if !found || !strings.Contains(application.powers.header.Header(), "probable") {
		test.Fatalf("Pier.kt's use is not listed as probable; the header says %q", application.powers.header.Header())
	}
	if spans := holder.UsesIn(pier.File); len(spans[5]) != 1 {
		test.Errorf("Pier.kt marks %v, want Lamp() on line 5", spans)
	}
	if header := application.powers.header.Header(); !strings.HasSuffix(header, "· 2 probable") {
		test.Errorf("the header says %q: the uses a server confirmed are still probable", header)
	}
}

// A long note on the search is cut before the views' buttons: it pushed them out of the window.
func TestALongNoteLeavesTheViewsButtonsInSight(test *testing.T) {
	application, window := readerOfUses(test)
	holder, held := application.usesHolder()
	if !held {
		test.Fatal("the uses mode is not on")
	}
	holder.SetUsesNote(strings.Repeat("some wait for kotlin-lsp · importing platform.libs.utils (4 of 17) · ", 6))
	window.Frames(2)
	button, err := window.Element("Who uses the name (⇧2)")
	if err != nil {
		test.Fatalf("the views' graph button is not there: %v", err)
	}
	if right := button.Rect.X + button.Rect.Width; right > 1200 {
		test.Errorf("the views' graph button ends at %v, past the window's 1200", right)
	}
}
