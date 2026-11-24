package window

import (
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/lsp"
)

// absentServer is a Go server that is not on this computer, installed with an installer that is.
func absentServer(install string) []lsp.Server {
	return []lsp.Server{{Name: "lampls", Command: "no-such-lamp-server-here", Languages: []string{"go"}, Install: install}}
}

// A server installed from the panel lands where go install puts commands, off the PATH, and
// starts there for the file open.
func TestAMissingServerIsInstalledFromThePanelAndStarts(test *testing.T) {
	log := filepath.Join(test.TempDir(), "log")
	server := fakeGo(test, log)[0]
	bin := test.TempDir()
	test.Setenv("GOBIN", bin)
	server.Install = "cp " + server.Command + " " + filepath.Join(bin, "lampls-installed")
	server.Command = "lampls-installed"
	application := readerWithServers(test, "lighthouse/src/lamp.go", []lsp.Server{server})
	window := newWindow(test, application)
	window.Frames(2)
	if !application.servers.missing["fakels"] {
		test.Fatal("a server that is not on the computer is not missing")
	}
	application.serversOpen = true
	window.Frames(2)
	if err := window.Click("Install"); err != nil {
		test.Fatal(err)
	}
	window.Frames(3)
	if application.servers.missing["fakels"] || len(application.servers.running) != 1 {
		test.Errorf("after installing it, the server is missing (%v) or runs %d times", application.servers.missing["fakels"], len(application.servers.running))
	}
}

func TestAFailedInstallSaysWhyAndOffersToTryAgain(test *testing.T) {
	application := readerWithServers(test, "lighthouse/src/lamp.go", absentServer("sh -c false"))
	window := newWindow(test, application)
	window.Frames(2)
	application.installServer("lampls")
	window.Frames(2)
	view := application.missingView("lampls")
	if view.headline != "Install failed" || view.why == "" || !view.installable {
		test.Errorf("a failed install is shown as %+v", view)
	}
}

func TestAServerWhoseInstallerIsNotHereOffersNoButton(test *testing.T) {
	application := readerWithServers(test, "lighthouse/src/lamp.go", absentServer("no-such-installer-here install lampls"))
	window := newWindow(test, application)
	window.Frames(2)
	application.serversOpen = true
	window.Frames(2)
	if hasButton(window, "Install") {
		test.Error("the panel offers to install a server whose installer is not on the computer")
	}
}
