package window

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/ui/viewer"
)

// javaRepository makes a git repository of Java files, committed, in a folder.
func javaRepository(test *testing.T, folder string, files map[string]string) {
	test.Helper()
	for path, text := range files {
		full := filepath.Join(folder, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "-m", "start"}} {
		command := exec.Command("git", arguments...)
		command.Dir = folder
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

// usesListedIn are the uses the finder lists, as "project/path:line".
func usesListedIn(application *Window) string {
	var listed []string
	for _, use := range application.finder.Hits {
		listed = append(listed, filepath.Base(application.workspace.Projects[use.Project()].Path)+"/"+use.Path+":"+strconv.Itoa(use.Line))
	}
	return strings.Join(listed, " ")
}

// Two repositories, each its own build as in the big workspace: harbour uses lighthouse's Lamp through a jar,
// and a server of its own, which knows nothing of lighthouse's sources.
func TestUsesAreFoundInOtherProjectsThroughTheirJars(test *testing.T) {
	folder := test.TempDir()
	javaRepository(test, filepath.Join(folder, "lighthouse"), map[string]string{
		"src/main/java/org/lighthouse/Lamp.java": "package org.lighthouse;\n\npublic class Lamp {\n    public int beam() { return 2; }\n}\n",
	})
	javaRepository(test, filepath.Join(folder, "harbour"), map[string]string{
		"src/main/java/org/harbour/Quay.java":  "package org.harbour;\n\nimport org.lighthouse.Lamp;\n\npublic class Quay {\n    Lamp lamp = new Lamp();\n}\n",
		"src/main/java/org/harbour/Other.java": "package org.harbour;\n\npublic class Other {\n    // a Lamp in words only\n}\n",
		// the server answers where a word of a comment is declared: only the link is a use
		"src/main/java/org/harbour/Buoy.java": "package org.harbour;\n\nimport org.lighthouse.Lamp;\n\n" +
			"/** Lit as a Lamp is.\n * @see Lamp\n */\npublic class Buoy {\n}\n",
	})
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"java"}, Environment: []string{"FAKE_LSP_JARS=1"}}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/main/java/org/lighthouse/Lamp.java", Servers: servers})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	window.Frames(1)
	application.viewer.PlaceCaret(viewer.Place{Line: 2, Column: 14}) // on Lamp, where it is declared
	application.usesAsked()
	for frame := 0; frame < 20 && application.uses.Last() != nil && application.finder.Note != ""; frame++ {
		window.Frames(1)
	}
	got := usesListedIn(application)
	for _, want := range []string{"harbour/src/main/java/org/harbour/Quay.java:3", "harbour/src/main/java/org/harbour/Quay.java:6",
		"harbour/src/main/java/org/harbour/Buoy.java:6"} {
		if !strings.Contains(got, want) {
			test.Errorf("the uses are %q, want %s among them (note %q)", got, want, application.finder.Note)
		}
	}
	if strings.Contains(got, "Buoy.java:5") {
		test.Errorf("a word in a Javadoc is listed as a use: %q", got)
	}
	if strings.Contains(got, "Other.java") {
		test.Errorf("a word in a comment, whose declaration is not Lamp's, is listed: %q", got)
	}
}

func TestUsesAskedWhileTheServerReadsAreAskedAgainOnceItIsDone(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log, "FAKE_LSP_BUSY=2500"))
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	window.Frames(1)
	client := application.servers.Running("fakels " + application.workspace.Projects[0].Path)
	for deadline := time.Now().Add(2 * time.Second); client != nil && !client.Busy() && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	application.viewer.PlaceCaret(viewer.Place{Line: 2, Column: 6}) // on On
	application.usesAsked()
	for frame := 0; frame < 50 && application.uses.Last() != nil && application.uses.Last().Busy(); frame++ {
		window.Frames(1)
		time.Sleep(20 * time.Millisecond)
	}
	window.Frames(2)
	heard, _ := os.ReadFile(log)
	if asked := strings.Count(string(heard), "textDocument/references"); asked < 2 {
		test.Errorf("the server was asked for the uses %d times, want once more after it was done", asked)
	}
	if len(application.finder.Hits) == 0 || strings.Contains(application.finder.Note, "more once") {
		test.Errorf("once the server was done the uses are %d, noted %q", len(application.finder.Hits), application.finder.Note)
	}
}
