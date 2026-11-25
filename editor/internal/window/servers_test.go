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

	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/coddy/editor/internal/workspace"
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
	test.Cleanup(application.servers.Stop)
	return application
}

func TestAServerStartsOnFirstUse(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/notes.txt", fakeGo(test, log))
	window := newWindow(test, application)
	if application.servers.RunningCount() != 0 {
		test.Fatal("a server started before a file of its language was opened")
	}
	application.openFile(workspace.FileReference{Project: 0, Path: "src/lamp.go"})
	window.Frames(2)
	if application.servers.RunningCount() != 1 || !logSays(test, log, "opened lamp.go package lamp") {
		test.Fatalf("opening lamp.go ran %d servers", application.servers.RunningCount())
	}
	if !strings.Contains(application.servers.Status(), "fakels") {
		test.Errorf("the status bar says %q of the servers", application.servers.Status())
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
	client := application.servers.Running("fakels " + application.session.Projects[0].Path)
	if client == nil {
		test.Fatalf("no server runs for lamp.go: %s", application.servers.Status())
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
	if application.servers.Snapshot(application.session.Projects[0].Path) != nil {
		test.Error("a server that never ran left a snapshot, which would take later changes for its own")
	}
	if application.servers.Status() != "nols not installed" {
		test.Errorf("the status bar says %q", application.servers.Status())
	}
}

// writingServer is the fake server for Go, for the whole window, writing .classpath in each project
// as jdtls does.
func writingServer(test *testing.T) []lsp.Server {
	return []lsp.Server{{Name: "fakels", Command: fakeServer(test), Languages: []string{"go"}, OnePerWindow: true,
		Environment: []string{"FAKE_LSP_WRITES=.classpath"}}}
}

func TestFilesWrittenByAServerAreNotChanges(test *testing.T) {
	record, folder := filepath.Join(test.TempDir(), "wrote.json"), makeRepository(test)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: writingServer(test), ServersWrote: record})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	application.focus = viewerPane
	application.viewer.SetMode(viewer.FileView)
	typeText(window, "// lit\n")
	press(window, input.KeyS, input.KeySuper)
	if saved, _ := os.ReadFile(filepath.Join(folder, "lighthouse", "src", "lamp.go")); !strings.HasPrefix(string(saved), "// lit") {
		test.Fatalf("lamp.go was not saved: %q", saved)
	}
	application.reloadAll()
	window.Frames(2)
	project := application.session.Projects[0]
	if _, changed := project.ChangeOf(".classpath"); changed || len(project.ServerFiles) != 1 || project.ServerFiles[0].Server != "fakels" {
		test.Fatalf("the server's .classpath is a change (%t); its files are %+v", changed, project.ServerFiles)
	}
	if _, changed := project.ChangeOf("src/lamp.go"); !changed || len(project.Changes) != 3 {
		test.Errorf("the person's changes are %v, want the 3 of the branch, lamp.go saved among them", project.ChangedPaths())
	}
	again := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true, ServersWrote: record})
	newWindow(test, again)
	if _, changed := again.session.Projects[0].ChangeOf(".classpath"); changed {
		test.Error("after a restart, the server's .classpath counts as a change")
	}
}

func TestFilesWrittenByAServerAreListed(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: writingServer(test)})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	application.reloadAll()
	window.Frames(2)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "fakels · 1") {
		test.Fatalf("the project shows no badge of the server's files: %q", shown)
	}
	application.ShowServerFiles(0)
	window.Frames(1)
	if hits := application.finder.Hits; len(hits) != 1 || hits[0].Path != ".classpath" || !strings.HasPrefix(hits[0].Text, "new while fakels ran") {
		test.Fatalf("the list of the server's files is %+v", hits)
	}
	press(window, input.KeyEnter)
	if application.viewer.Reference.Path != ".classpath" || application.viewer.Document == nil {
		test.Errorf("Enter opened %q, want .classpath", application.viewer.Reference.Path)
	}
}

func TestACommitWhileAServerRunsStaysAChange(test *testing.T) {
	folder := makeRepository(test)
	application := newReader(Settings{Folders: []string{folder}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: writingServer(test)})
	test.Cleanup(application.servers.Stop)
	window := newWindow(test, application)
	repository := filepath.Join(folder, "lighthouse")
	os.WriteFile(filepath.Join(repository, "src", "keeper.go"), []byte("package lamp\n\ntype Keeper struct{ Night bool }\n"), 0o644)
	for _, arguments := range [][]string{{"add", "src/keeper.go"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "night"}} {
		command := exec.Command("git", arguments...)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			test.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	application.reloadAll()
	window.Frames(2)
	if _, changed := application.session.Projects[0].ChangeOf("src/keeper.go"); !changed {
		test.Errorf("a commit made while the server ran left the review; the server's files are %+v", application.session.Projects[0].ServerFiles)
	}
}

func TestAServerThatStopsIsStartedAgain(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	window := newWindow(test, application)
	key := "fakels " + application.session.Projects[0].Path
	first := application.servers.Running(key)
	first.Stop() // as if it had crashed
	for attempt := 0; attempt < 100 && application.servers.Running(key) == first; attempt++ {
		time.Sleep(10 * time.Millisecond)
		window.Frames(1)
	}
	window.Frames(2)
	if again := application.servers.Running(key); again == nil || again == first {
		test.Fatalf("after it stopped, the server runs as %v (the first was %p)", again, first)
	}
}

func TestAProjectWhereOnlyAServerWroteStaysListed(test *testing.T) {
	application := newReader(Settings{Synchronous: true})
	application.onlyChanged = true
	project := &workspace.Project{Loaded: true, ServerFiles: []workspace.ServerFile{{Path: ".classpath", Server: "jdtls"}}}
	if application.hidden(project) {
		test.Error("a project with only a server's files is hidden, and its badge with it")
	}
}

func TestAServerThatNeedsMoreFreeSpaceThanTheDiskHasDoesNotStart(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	servers := fakeGo(test, log)
	servers[0].DataFlag, servers[0].Index.NeedsFree = "--data", 10<<30
	application := newReader(Settings{Folders: []string{makeRepository(test)}, WorkingTree: true, Synchronous: true,
		Open: "lighthouse/src/lamp.go", Servers: servers, ServerData: test.TempDir()})
	test.Cleanup(application.servers.Stop)
	application.servers.FreeSpace = func(string) (uint64, error) { return 3 << 30, nil }
	window := newWindow(test, application)
	window.Frames(1)
	if application.servers.RunningCount() != 0 {
		test.Fatal("the server started on a disk too full for it")
	}
	why := application.servers.Failed("fakels")
	if !strings.Contains(why, "it needs 10.0 GB free") || !strings.Contains(why, "which has 3.0 GB") || !strings.Contains(why, "-server-data") {
		test.Errorf("the reader says %q", why)
	}
}

func TestAServerWaitsForAnotherToHaveImportedTheProjects(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	first := fakeGo(test, log, "FAKE_LSP_READY=ServiceReady")[0]
	first.Ready.Status = "ServiceReady"
	second := lsp.Server{Name: "second", Command: fakeServer(test), Languages: []string{"kotlin"}, Import: lsp.Import{After: []string{"fakels"}}}
	application := readerWithServers(test, "lighthouse/src/lamp.go", []lsp.Server{first, second})
	window := newWindow(test, application)
	window.Frames(1)
	project := application.session.Projects[0].Path
	application.servers.Start(second, "", "second", project)
	if application.servers.Starting("second") || !strings.Contains(application.servers.Status(), "second waits for fakels") {
		test.Fatalf("second started while fakels imported; the status says %q", application.servers.Status())
	}
	for frame := 0; frame < 100 && strings.Contains(application.servers.Status(), "waits"); frame++ {
		window.Frames(1)
		time.Sleep(10 * time.Millisecond)
	}
	application.servers.Start(second, "", "second", project)
	if !application.servers.Starting("second") && application.servers.Running("second") == nil {
		test.Errorf("second did not start once fakels was ready; the status says %q", application.servers.Status())
	}
}

// Telling the server the file open runs every frame: while the file does not change it must cost
// the same for a file of any size, never a copy of its text.
func TestServingTheOpenFileCopiesNothingWhileItIsUnchanged(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	window := newWindow(test, application)
	window.Frames(2)
	if !logSays(test, log, "opened lamp.go") {
		test.Fatal("the server was not told of lamp.go")
	}
	small := testing.AllocsPerRun(50, func() { application.servers.Serve(application.servedFile()) })
	application.viewer.Edit.Insert(strings.Repeat("// a line of a long file\n", 3000))
	application.servers.Serve(application.servedFile()) // told once, as it changed
	big := testing.AllocsPerRun(50, func() { application.servers.Serve(application.servedFile()) })
	if big > small {
		test.Errorf("serving an unchanged file of 3,000 lines allocates %v times, one of 5 lines %v", big, small)
	}
}

// A server is told of the file open only: the file left for another is closed on it.
func TestTheFileLeftIsClosedOnItsServer(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	window := newWindow(test, application)
	window.Frames(2)
	application.openFile(workspace.FileReference{Project: 0, Path: "src/beam.go"})
	window.Frames(2)
	if !logSays(test, log, "opened beam.go") || !logSays(test, log, "closed lamp.go") {
		test.Error("opening beam.go did not close lamp.go on the server")
	}
}
