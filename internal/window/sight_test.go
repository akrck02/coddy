package window

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	test.Cleanup(application.servers.Stop)
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
	first := application.servers.Running("fakels")
	if first == nil || first.FollowsFolders() {
		test.Fatalf("the server runs as %v", first)
	}
	application.setOnlyChanged(false)
	for attempt := 0; attempt < 100 && application.servers.Running("fakels") == first; attempt++ {
		window.Frames(1)
		time.Sleep(10 * time.Millisecond)
	}
	window.Frames(2)
	again := application.servers.Running("fakels")
	if again == nil || again == first || !slices.Contains(again.Folders(), filepath.Join(folder, "harbour")) {
		test.Fatalf("after harbour came into sight the server runs as %v, reading %v", again, folderList(again))
	}
	if application.servers.Crashes("fakels") != 0 {
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

func TestAServerWithAnIndexPerSetReadsEveryProjectWhateverIsInSight(test *testing.T) {
	folder, log := makeRepository(test), filepath.Join(test.TempDir(), "log")
	addHarbour(test, folder)
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true,
		ReadsAll: true, Environment: []string{"FAKE_LSP_LOG=" + log, "FAKE_LSP_NO_FOLDERS=1"}}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, OnlyChanged: true,
		Open: "lighthouse/src/lamp.go", Servers: servers})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	window.Frames(2)
	first := application.servers.Running("fakels")
	if first == nil || !slices.Contains(first.Folders(), filepath.Join(folder, "harbour")) {
		test.Fatalf("the server reads %v, not harbour out of sight", folderList(first))
	}
	application.setOnlyChanged(false)
	window.Frames(2)
	time.Sleep(50 * time.Millisecond)
	window.Frames(2)
	if application.servers.Running("fakels") != first {
		test.Error("the filter started the server again")
	}
}

func TestAServerThatImportsBuildsLeavesOutABuildItCannotImport(test *testing.T) {
	folder, log := makeRepository(test), filepath.Join(test.TempDir(), "log")
	addHarbour(test, folder)
	wrapper := filepath.Join(folder, "harbour", "gradle", "wrapper")
	_ = os.MkdirAll(wrapper, 0o755)
	_ = os.WriteFile(filepath.Join(wrapper, "gradle-wrapper.properties"),
		[]byte("distributionUrl=https\\://services.gradle.org/distributions/gradle-4.10.3-bin.zip\n"), 0o644)
	servers := []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true,
		ReadsAll: true, OwnGradle: true, Environment: []string{"FAKE_LSP_LOG=" + log}}}
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: servers})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	window.Frames(2)
	client := application.servers.Running("fakels")
	harbour := filepath.Join(folder, "harbour")
	if client == nil || slices.Contains(client.Folders(), harbour) || !slices.Contains(client.Folders(), filepath.Join(folder, "lighthouse")) {
		test.Fatalf("the server reads %v: want lighthouse, without harbour's Gradle 4 build", folderList(client))
	}
	if why := application.servers.CannotImport(harbour); !strings.Contains(why, "Gradle 4.10.3") {
		test.Errorf("the reader notes harbour is left out for %q", why)
	}
}
