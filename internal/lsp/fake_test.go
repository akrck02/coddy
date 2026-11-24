package lsp

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	fakeOnce sync.Once
	fakePath string
	fakeErr  error
)

// fakeServer builds the fake language server of testdata once, and gives its path.
func fakeServer(test *testing.T) string {
	test.Helper()
	fakeOnce.Do(func() {
		folder, err := os.MkdirTemp("", "fakeserver")
		if err != nil {
			fakeErr = err
			return
		}
		fakePath = filepath.Join(folder, "fakeserver")
		output, err := exec.Command("go", "build", "-o", fakePath, "./testdata/fakeserver").CombinedOutput()
		if err != nil {
			fakeErr = errors.New(err.Error() + "\n" + string(output))
		}
	})
	if fakeErr != nil {
		test.Fatalf("building the fake server: %v", fakeErr)
	}
	return fakePath
}

// project is a folder with Go files, for a server to read.
func project(test *testing.T, files map[string]string) string {
	folder := test.TempDir()
	for name, content := range files {
		path := filepath.Join(folder, name)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return folder
}

func TestTheFakeServerAnswersFromTheFiles(test *testing.T) {
	folder := project(test, map[string]string{
		"lamp.go": "package lamp\n\nfunc Beam() int { return 2 }\n",
		"use.go":  "package lamp\n\nvar x = Beam()\n",
	})
	command := exec.Command(fakeServer(test))
	input, _ := command.StdinPipe()
	output, _ := command.StdoutPipe()
	if err := command.Start(); err != nil {
		test.Fatal(err)
	}
	defer command.Process.Kill()
	conn := NewConn(output, input, nil)
	requestContext := within(test)
	if err := conn.Call(requestContext, "initialize", map[string]any{"rootUri": FileURI(folder)}, nil); err != nil {
		test.Fatal(err)
	}
	at := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: FileURI(filepath.Join(folder, "use.go"))},
		Position: Position{Line: 2, Character: 9}}
	var declared, used []Location
	var hover struct {
		Contents struct{ Value string } `json:"contents"`
	}
	conn.Call(requestContext, "textDocument/definition", at, &declared)
	conn.Call(requestContext, "textDocument/references", at, &used)
	conn.Call(requestContext, "textDocument/hover", at, &hover)
	if len(declared) != 1 || !strings.HasSuffix(declared[0].URI, "lamp.go") || declared[0].Range.Start != (Position{Line: 2, Character: 5}) {
		test.Errorf("Beam is declared at %+v", declared)
	}
	if len(used) != 2 {
		test.Errorf("Beam is used at %+v, want 2 places", used)
	}
	if !strings.Contains(hover.Contents.Value, "Beam") {
		test.Errorf("the hover says %q", hover.Contents.Value)
	}
}
