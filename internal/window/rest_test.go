package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/viewer"
)

func TestAServerNotAskedForAWhileRestsAndAQuestionWakesIt(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	application.servers.rest = 300 * time.Millisecond
	window := newWindow(test, application)
	key := "fakels " + application.projects[0].Path
	first := application.servers.running[key]
	if first == nil {
		test.Fatal("no server runs for lamp.go")
	}
	for attempt := 0; attempt < 200 && application.servers.running[key] != nil; attempt++ {
		time.Sleep(10 * time.Millisecond)
		window.Frames(1)
	}
	select {
	case <-first.Done():
	case <-time.After(5 * time.Second):
		test.Fatal("the server unasked still runs")
	}
	window.Frames(2)
	if application.servers.running[key] != nil || application.serverStatus() != "fakels resting" {
		test.Fatalf("with lamp.go still open, the status bar says %q", application.serverStatus())
	}
	application.GoToDefinition(viewer.Place{Line: 2, Column: 5})
	// it starts once the one resting has ended, as both share the data folder
	for attempt := 0; attempt < 200 && application.servers.running[key] == nil; attempt++ {
		time.Sleep(10 * time.Millisecond)
		window.Frames(1)
	}
	if again := application.servers.running[key]; again == nil || again == first {
		test.Errorf("a question did not wake the server: the status bar says %q", application.serverStatus())
	}
	if crashes := application.servers.crashes["fakels"]; crashes != 0 {
		test.Errorf("resting was taken for %d crashes", crashes)
	}
}

func TestTheServersPanelShowsWhatAServerTakesAndStopsIt(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log))
	window := newWindow(test, application)
	key := "fakels " + application.projects[0].Path
	client := application.servers.running[key]
	application.serversOpen = true
	for attempt := 0; attempt < 200 && application.usageOf(key) == ""; attempt++ {
		time.Sleep(10 * time.Millisecond)
		window.Frames(1)
	}
	window.Frames(1)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "· memory ") {
		test.Fatalf("the panel shows %q", shown)
	}
	clickText(test, window, "Stop")
	window.Frames(1)
	select {
	case <-client.Done():
	case <-time.After(5 * time.Second):
		test.Fatal("the server stopped from the panel still runs")
	}
	if application.serverStatus() != "fakels resting" {
		test.Errorf("the status bar says %q", application.serverStatus())
	}
}

func TestAServerWokenWhileTheOneBeforeSavesStartsOnceItHasEnded(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	servers := fakeGo(test, log)
	servers[0].Environment = append(servers[0].Environment, "FAKE_LSP_SLOW_EXIT=800")
	application := readerWithServers(test, "lighthouse/src/lamp.go", servers)
	window := newWindow(test, application)
	key := "fakels " + application.projects[0].Path
	first := application.servers.running[key]
	application.restServer(key, first)
	application.GoToDefinition(viewer.Place{Line: 2, Column: 5})
	for attempt := 0; attempt < 300 && application.servers.running[key] == nil; attempt++ {
		time.Sleep(10 * time.Millisecond)
		window.Frames(1)
	}
	content, _ := os.ReadFile(log)
	saved, again := strings.Index(string(content), "saved"), strings.LastIndex(string(content), "initialize")
	if application.servers.running[key] == nil || saved < 0 || again < saved {
		test.Errorf("the server woke before the one resting had saved: it logged %q", content)
	}
}
