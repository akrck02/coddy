package window

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/files"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/uses"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/input"
	uitest "github.com/akrck02/go-ui/uitest"
)

// readerOfUses opens two repositories, harbour using lighthouse's Lamp through a jar, with the
// uses mode, and asks the uses of Lamp where it is declared.
func readerOfUses(test *testing.T) (*Window, *uitest.Window) {
	folder := test.TempDir()
	javaRepository(test, filepath.Join(folder, "lighthouse"), map[string]string{
		"src/main/java/org/lighthouse/Lamp.java": "package org.lighthouse;\n\npublic class Lamp {\n    public int beam() { return 2; }\n}\n",
	})
	javaRepository(test, filepath.Join(folder, "harbour"), map[string]string{
		"src/main/java/org/harbour/Quay.java":  "package org.harbour;\n\nimport org.lighthouse.Lamp;\n\npublic class Quay {\n    Lamp lamp = new Lamp();\n}\n",
		"src/main/java/org/harbour/Other.java": "package org.harbour;\n\npublic class Other {\n}\n",
	})
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"java"}, Environment: []string{"FAKE_LSP_JARS=1"}}}
	application, err := New(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/main/java/org/lighthouse/Lamp.java", Servers: servers}, files.New(), review.New(""), uses.New())
	if err != nil {
		test.Fatal(err)
	}
	test.Cleanup(application.stopServers)
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	window.Frames(1)
	application.viewer.PlaceCaret(viewer.Place{Line: 2, Column: 14})
	press(window, input.KeyF12, input.KeyShift)
	for frame := 0; frame < 40 && application.uses != nil && application.uses.noted != ""; frame++ {
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
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "uses in 2 files") {
		test.Errorf("the bar over the viewer says %q", shown)
	}
}

func TestNGoesThroughTheUsesAndVTicksThem(test *testing.T) {
	application, window := readerOfUses(test)
	application.focus = sidebarPane
	for step := 0; step < 3 && !strings.HasSuffix(application.viewer.Reference.Path, "Quay.java"); step++ {
		press(window, input.KeyN)
		window.Frames(1)
	}
	view := &application.viewer
	if !strings.HasSuffix(view.Reference.Path, "Quay.java") || view.Mode != viewer.UsesView || len(view.Uses[6]) != 2 {
		test.Fatalf("n opened %s in mode %d with uses %v", view.Reference.Path, view.Mode, view.Uses)
	}
	press(window, input.KeyV)
	window.Frames(1)
	if header := application.mode.Header(); !strings.HasPrefix(header, "1 of ") {
		test.Errorf("after v the header says %q", header)
	}
}
