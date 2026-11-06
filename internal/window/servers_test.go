package window

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/input"
)

var (
	fakeOnce sync.Once
	fakePath string
	fakeErr  error
)

// fakeServer builds the fake language server of the lsp package once, and gives its path.
func fakeServer(test *testing.T) string {
	test.Helper()
	fakeOnce.Do(func() {
		folder, err := os.MkdirTemp("", "fakeserver")
		if err != nil {
			fakeErr = err
			return
		}
		fakePath = filepath.Join(folder, "fakeserver")
		output, err := exec.Command("go", "build", "-o", fakePath, "../lsp/testdata/fakeserver").CombinedOutput()
		if err != nil {
			fakeErr = errors.New(err.Error() + "\n" + string(output))
		}
	})
	if fakeErr != nil {
		test.Fatalf("building the fake server: %v", fakeErr)
	}
	return fakePath
}

// fakeGo is a table with the fake server reading Go, logging what it receives to the file given.
func fakeGo(test *testing.T, log string, environment ...string) []lsp.Server {
	return []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"},
		RootMarkers: []string{"go.mod"}, Install: "go build fakeserver",
		Environment: append([]string{"FAKE_LSP_LOG=" + log}, environment...)}}
}

// logSays waits a moment for the fake server's log to hold a line, as it writes from its own process.
func logSays(test *testing.T, log, line string) bool {
	test.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		content, _ := os.ReadFile(log)
		if strings.Contains(string(content), line) {
			return true
		}
	}
	return false
}

// readerWithServers opens a file of the lighthouse repository with the fake server for Go.
func readerWithServers(test *testing.T, open string, servers []lsp.Server) *Window {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: open, Servers: servers})
	test.Cleanup(application.stopServers)
	return application
}

func TestAServerStartsOnFirstUse(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/notes.txt", fakeGo(test, log))
	window := newWindow(test, application)
	if len(application.servers.running) != 0 {
		test.Fatal("a server started before a file of its language was opened")
	}
	application.openFile(workspace.FileReference{Project: 0, Path: "src/lamp.go"})
	window.Frames(2)
	if len(application.servers.running) != 1 || !logSays(test, log, "opened lamp.go package lamp") {
		test.Fatalf("opening lamp.go ran %d servers", len(application.servers.running))
	}
	if !strings.Contains(application.serverStatus(), "fakels") {
		test.Errorf("the status bar says %q of the servers", application.serverStatus())
	}
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	typeText(window, "go")
	press(window, input.KeyEnter)
	window.Frames(1)
	if !logSays(test, log, "changed lamp.go go") {
		test.Error("the server was not told of the edit")
	}
}

func TestServersStopWithTheWindow(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	newWindow(test, application)
	client := application.servers.running["fakels "+application.projects[0].Path]
	if client == nil {
		test.Fatalf("no server runs for lamp.go: %v", application.servers.running)
	}
	application.Closed()
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		test.Fatal("the server still runs after the window closed")
	}
	if !logSays(test, log, "shutdown") || !logSays(test, log, "exit") {
		test.Error("the server was not asked to shut down and exit")
	}
}

func TestAMissingServerIsNamed(test *testing.T) {
	servers := []lsp.Server{{Name: "nols", Command: "no-such-language-server", Languages: []string{"go"},
		Install: "brew install nols"}}
	application := readerWithServers(test, "lighthouse/notes.txt", servers)
	window := newWindow(test, application)
	application.openFile(workspace.FileReference{Project: 0, Path: "src/lamp.go"})
	window.Frames(1)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "nols is not installed: brew install nols") {
		test.Errorf("the window shows %q", shown)
	}
	if application.serverStatus() != "nols not installed" {
		test.Errorf("the status bar says %q", application.serverStatus())
	}
}
