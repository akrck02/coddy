package window

import (
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/servers"
)

// absentServer is a Go server that is not on this computer, installed with an installer that is.
func absentServer(install string) []lsp.Server {
	return []lsp.Server{{Name: "lampls", Command: "no-such-lamp-server-here", Languages: []string{"go"}, Install: install}}
}

// viewNamed is the view of a server by its name, empty when it is not among them.
func viewNamed(views []servers.View, name string) servers.View {
	for _, view := range views {
		if view.Name == name {
			return view
		}
	}
	return servers.View{}
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
	if !application.servers.Missing("fakels") {
		test.Fatal("a server that is not on the computer is not missing")
	}
	application.serversOpen = true
	window.Frames(2)
	if err := window.Click("Install"); err != nil {
		test.Fatal(err)
	}
	window.Frames(3)
	if application.servers.Missing("fakels") || application.servers.RunningCount() != 1 {
		test.Errorf("after installing it, the server is missing (%v) or runs %d times", application.servers.Missing("fakels"), application.servers.RunningCount())
	}
}

func TestAFailedInstallSaysWhyAndOffersToTryAgain(test *testing.T) {
	application := readerWithServers(test, "lighthouse/src/lamp.go", absentServer("sh -c false"))
	window := newWindow(test, application)
	window.Frames(2)
	application.servers.Install("lampls")
	window.Frames(2)
	view := viewNamed(application.servers.Views(), "lampls")
	if view.Headline != "Install failed" || view.Why == "" || !view.Installable {
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
