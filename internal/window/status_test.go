package window

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/servers"
	"github.com/akrck02/go-ui/uitest"
)

func TestAQuestionSaysWhatItWaitsFor(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log, "FAKE_LSP_BUSY=3000"))
	window := newWindow(test, application)
	client := application.servers.Running("fakels " + application.session.Projects[0].Path)
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
	if status := application.servers.Status(); !strings.HasPrefix(status, "fakels · importing") {
		test.Errorf("the status bar says %q", status)
	}
}

func TestAClickOnAServerOpensWhatItDoesProjectByProject(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	application := readerWithServers(test, "lighthouse/src/lamp.go", fakeGo(test, log, "FAKE_LSP_BUSY=3000"))
	window := newWindow(test, application)
	if err := window.Resize(1600, 800); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	client := application.servers.Running("fakels " + application.session.Projects[0].Path)
	for deadline := time.Now().Add(2 * time.Second); client != nil && !client.Busy() && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	window.Frames(1)
	button, err := window.Element("Language servers")
	if err != nil {
		test.Fatal(err)
	}
	window.ClickAt(button.Rect.X+button.Rect.Width/2, button.Rect.Y+button.Rect.Height/2)
	window.Frames(2)
	if shown := strings.Join(window.Texts(), " "); !application.serversListed || !strings.Contains(shown, "fakels") ||
		!strings.Contains(shown, "Go") || !strings.Contains(shown, "Importing") {
		test.Fatalf("the servers' popover shows %q", shown)
	}
	if details, err := window.Element("Details…"); err != nil || details.Rect.Right() > 1600 {
		test.Fatalf("the popover's button is at %v (%v), past the window's edge", details.Rect, err)
	}
	clickText(test, window, "Details…")
	window.Frames(2)
	shown := strings.Join(window.Texts(), " ")
	if !application.serversOpen || !strings.Contains(shown, "Language servers") || strings.Contains(shown, "Importing with Gradle") {
		test.Fatalf("the servers panel shows %q, want the server's state and no messages yet", shown)
	}
	clickText(test, window, "Show details")
	window.Frames(2)
	shown = strings.Join(window.Texts(), " ")
	if !strings.Contains(shown, "lighthouse") || !strings.Contains(shown, "Importing with Gradle") {
		test.Errorf("the details show %q", shown)
	}
}

// clickText clicks the text the last frame drew that starts with some words.
func clickText(test *testing.T, window *uitest.Window, start string) {
	test.Helper()
	for _, text := range window.Texts() {
		if !strings.HasPrefix(text, start) {
			continue
		}
		found, err := window.Element(text)
		if err != nil {
			test.Fatal(err)
		}
		window.ClickAt(found.Rect.X+found.Rect.Width/2, found.Rect.Y+found.Rect.Height/2)
		return
	}
	test.Fatalf("no text starts with %q: %q", start, window.Texts())
}

func TestTheServersButtonSaysTheReadersMode(test *testing.T) {
	ready := servers.View{State: servers.Ready, Fraction: 1}
	reading := servers.View{State: servers.Working, Fraction: 0.4}
	cases := []struct {
		views    []servers.View
		mode     serversMode
		fraction float32
		broken   bool
	}{
		{nil, textOnly, -1, false},
		{[]servers.View{{State: servers.Resting, Fraction: -1}}, textOnly, -1, false},
		{[]servers.View{ready, reading}, serversLoading, 0.4, false},
		{[]servers.View{ready, {State: servers.Broken, Fraction: -1}}, serversFull, 1, true},
	}
	for _, each := range cases {
		mode, fraction, broken := modeOf(each.views)
		if mode != each.mode || fraction != each.fraction || broken != each.broken {
			test.Errorf("%+v is mode %d at %v (broken %t), want %d at %v (%t)", each.views, mode, fraction, broken, each.mode, each.fraction, each.broken)
		}
	}
}
