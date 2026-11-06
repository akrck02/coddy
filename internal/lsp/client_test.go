package lsp

import (
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
	client.AddFolder(lamp)
	time.Sleep(50 * time.Millisecond) // the folder is a notification: let the server read it
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
	client.Open(path, "package lamp // as read\n")
	client.Open(path, "package lamp // edited\n")
	client.Stop()
	content, _ := os.ReadFile(log)
	if !strings.Contains(string(content), "opened lamp.go package lamp // as read") ||
		!strings.Contains(string(content), "changed lamp.go package lamp // edited") {
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
