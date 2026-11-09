package window

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/go-ui/uitest"
)

// inSightReader opens lighthouse's lamp.go, only the changes in sight, with the fake server for the
// whole window: harbour, with no changes, is out of sight.
func inSightReader(test *testing.T, environment ...string) (*Window, *uitest.Window, string, string) {
	folder, log := makeRepository(test), filepath.Join(test.TempDir(), "log")
	addHarbour(test, folder)
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true,
		Environment: append([]string{"FAKE_LSP_LOG=" + log}, environment...)}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, OnlyChanged: true,
		Open: "lighthouse/src/lamp.go", Servers: servers})
	test.Cleanup(application.stopServers)
	window := newWindow(test, application)
	window.Frames(2)
	return application, window, folder, log
}

func TestAServerReadsOnlyTheProjectsInSight(test *testing.T) {
	application, window, _, log := inSightReader(test)
	if !logSays(test, log, "reads lighthouse") || logSays(test, log, "reads harbour") {
		test.Fatal("the server does not read lighthouse alone")
	}
	application.setOnlyChanged(false)
	window.Frames(2)
	if !logSays(test, log, "reads harbour") {
		test.Error("harbour came into sight and the server was not told")
	}
	application.setOnlyChanged(true)
	window.Frames(2)
	if !logSays(test, log, "removed harbour") {
		test.Error("harbour left sight and the server was not told")
	}
}

func TestAServerThatDoesNotFollowFoldersStartsAgainForThem(test *testing.T) {
	application, window, folder, _ := inSightReader(test, "FAKE_LSP_NO_FOLDERS=1")
	first := application.servers.running["fakels"]
	if first == nil || first.FollowsFolders() {
		test.Fatalf("the server runs as %v", first)
	}
	application.setOnlyChanged(false)
	for attempt := 0; attempt < 100 && application.servers.running["fakels"] == first; attempt++ {
		window.Frames(1)
		time.Sleep(10 * time.Millisecond)
	}
	window.Frames(2)
	again := application.servers.running["fakels"]
	if again == nil || again == first || !slices.Contains(again.Folders(), filepath.Join(folder, "harbour")) {
		test.Fatalf("after harbour came into sight the server runs as %v, reading %v", again, folderList(again))
	}
	if application.servers.crashes["fakels"] != 0 {
		test.Error("starting it again counted as a crash")
	}
}

// folderList is what a client reads, or nothing for none.
func folderList(client *lsp.Client) []string {
	if client == nil {
		return nil
	}
	return client.Folders()
}
