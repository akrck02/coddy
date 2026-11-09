package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fake is the fake server's entry in a table of servers, logging to a file of the test.
func fake(test *testing.T, onePerWindow bool, environment ...string) (Server, string) {
	log := filepath.Join(test.TempDir(), "log")
	return Server{Name: "fake", Command: fakeServer(test), Languages: []string{"go"}, RootMarkers: []string{"go.mod"},
		OnePerWindow: onePerWindow, Environment: append(environment, "FAKE_LSP_LOG="+log)}, log
}

func TestAClientFollowsANameAcrossFolders(test *testing.T) {
	lamp := project(test, map[string]string{"go.mod": "module lamp\n", "lamp.go": "package lamp\n\nfunc Beam() int { return 2 }\n"})
	keeper := project(test, map[string]string{"go.mod": "module keeper\n", "keep.go": "package keeper\n\nvar x = Beam()\n"})
	server, _ := fake(test, true)
	client, err := Start(within(test), server, "", []string{keeper}, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	client.AddFolder(lamp) // a notification, handled before the requests that follow it
	use := filepath.Join(keeper, "keep.go")
	declared, err := client.Definition(within(test), use, Position{Line: 2, Character: 9})
	if err != nil || len(declared) != 1 || PathOf(declared[0].URI) != filepath.Join(lamp, "lamp.go") {
		test.Errorf("Beam is declared at %+v (%v)", declared, err)
	}
	hover, _ := client.Hover(within(test), use, Position{Line: 2, Character: 9})
	if !strings.Contains(hover, "Beam") {
		test.Errorf("the hover says %q", hover)
	}
}

func TestADocumentOpenIsSentWithItsEdits(test *testing.T) {
	folder := project(test, map[string]string{"go.mod": "module lamp\n", "lamp.go": "package lamp\n"})
	server, log := fake(test, false)
	client, err := Start(within(test), server, folder, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	path := filepath.Join(folder, "lamp.go")
	client.Tell(path, "package lamp // as read\n", 1)
	client.Tell(path, "package lamp // edited\n", 3)
	client.Tell(path, "package lamp // older\n", 2) // overtaken: dropped
	client.Stop()
	content, _ := os.ReadFile(log)
	if !strings.Contains(string(content), "opened lamp.go package lamp // as read") ||
		!strings.Contains(string(content), "changed lamp.go package lamp // edited") || strings.Contains(string(content), "older") {
		test.Errorf("the server heard:\n%s", content)
	}
	select {
	case <-client.Done():
	case <-time.After(time.Second):
		test.Error("the server did not stop")
	}
}

func TestAServersRootIsTheFolderOfItsMarker(test *testing.T) {
	folder := project(test, map[string]string{"tools/go.mod": "module tools\n", "tools/cmd/main.go": "package main\n", "top.go": "package top\n"})
	server := Servers[0]
	if root := server.RootOf(folder, filepath.Join(folder, "tools", "cmd", "main.go")); root != filepath.Join(folder, "tools") {
		test.Errorf("main.go's root is %s", root)
	}
	if root := server.RootOf(folder, filepath.Join(folder, "top.go")); root != folder {
		test.Errorf("top.go's root is %s, want the project", root)
	}
}

func TestARequestWaitsForTheServerToReadTheProjects(test *testing.T) {
	folder := project(test, map[string]string{"lamp.go": "package lamp\n\nfunc Beam() int { return 2 }\n"})
	server, _ := fake(test, false, "FAKE_LSP_READY=ServiceReady")
	server.ReadyStatus = "ServiceReady"
	client, err := Start(within(test), server, folder, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	if client.Ready() || client.Progress().Phase != "starting" {
		test.Errorf("the server is ready before it said so: %+v", client.Progress())
	}
	declared, err := client.Definition(within(test), filepath.Join(folder, "lamp.go"), Position{Line: 2, Character: 6})
	if err != nil || len(declared) != 1 || !client.Ready() {
		test.Errorf("asked while it read, the server declared Beam at %+v (%v)", declared, err)
	}
}

func TestARequestAnsweredWithNothingWhileTheServerWorksIsAskedAgain(test *testing.T) {
	folder := project(test, map[string]string{"lamp.go": "package lamp\n\nfunc Beam() int { return 2 }\n"})
	server, _ := fake(test, false, "FAKE_LSP_BUSY=600")
	client, err := Start(within(test), server, folder, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	for deadline := time.Now().Add(2 * time.Second); !client.Busy() && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	if client.Ready() {
		test.Error("the server is ready while it reports work running")
	}
	for deadline := time.Now().Add(2 * time.Second); client.Progress().Percent != 50 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	busy := client.Progress()
	if busy.Phase != "importing" || busy.Detail != filepath.Base(folder) || busy.Percent != 50 || busy.Done != 1 || busy.Total != 1 {
		test.Errorf("while it works the server is %+v, want importing %s, 50 %%, 1 of 1", busy, filepath.Base(folder))
	}
	declared, err := client.Definition(within(test), filepath.Join(folder, "lamp.go"), Position{Line: 2, Character: 6})
	if err != nil || len(declared) != 1 || client.Busy() {
		test.Errorf("asked while it worked, the server declared Beam at %+v (%v)", declared, err)
	}
	if done := client.Progress(); done.Phase != "ready" || !strings.Contains(fmt.Sprint(done.Log), "Importing with Gradle") {
		test.Errorf("once done the server is %+v", done)
	}
}

func TestApplyEditIsRefused(test *testing.T) {
	folder := project(test, map[string]string{"go.mod": "module lamp\n", "lamp.go": "package lamp\n"})
	server, log := fake(test, false, "FAKE_LSP_APPLY_EDIT=1")
	client, err := Start(within(test), server, folder, nil, "")
	if err != nil {
		test.Fatal(err)
	}
	defer client.Stop()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		heard, _ := os.ReadFile(log)
		if strings.Contains(string(heard), "applyEdit applied=false error=false") {
			return
		}
	}
	heard, _ := os.ReadFile(log)
	test.Fatalf("the server's edit was not answered \"not applied\":\n%s", heard)
}
