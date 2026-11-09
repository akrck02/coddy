package window

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
)

func TestTheStatusBarSaysWhatAServerDoes(test *testing.T) {
	started := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		progress lsp.Progress
		want     string
	}{
		{lsp.Progress{Phase: "importing", Detail: "harbour.api.order", Percent: -1, Done: 7, Total: 22, Started: started},
			"kotlin-lsp · importing harbour.api.order (7 of 22) · 3:12"},
		{lsp.Progress{Phase: "indexing", Detail: "45%", Percent: 45, Started: started}, "kotlin-lsp · indexing 45 % · 3:12"},
		{lsp.Progress{Phase: "starting", Detail: "Importing Gradle project(s)", Percent: -1, Started: started},
			"kotlin-lsp · starting Importing Gradle project(s) · 3:12"},
		{lsp.Progress{Phase: "ready"}, "kotlin-lsp ready"},
	}
	for _, each := range cases {
		if said := statusOf("kotlin-lsp", each.progress, started.Add(3*time.Minute+12*time.Second)); said != each.want {
			test.Errorf("the status bar says %q, want %q", said, each.want)
		}
	}
}

func TestAQuestionSaysWhatItWaitsFor(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log, "FAKE_LSP_BUSY=3000"))
	window := newWindow(test, application)
	client := application.servers.running["fakels "+application.projects[0].Path]
	for deadline := time.Now().Add(2 * time.Second); client != nil && !client.Busy() && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	if client == nil || client.Ready() {
		test.Fatalf("the fake server runs as %v, and should be busy", client)
	}
	application.waitFor(client)
	window.Frames(1)
	if shown := strings.Join(window.Texts(), " "); !strings.Contains(shown, "Waiting for fakels · importing") {
		test.Errorf("while the server works the window shows %q", shown)
	}
	if status := application.serverStatus(); !strings.HasPrefix(status, "fakels · importing") {
		test.Errorf("the status bar says %q", status)
	}
}
