// Command fakeserver is a language server for the tests: it answers definition, references and
// hover by looking at the words of the files, the way a real server would answer for simple code.
//
//   - definition: where the word under the position is declared ("func", "type", "class",
//     "interface", "const" or "var" before it), in any workspace folder.
//   - references: every place the word appears as a whole word.
//   - hover: "**word** from the fake server".
//
// Environment variables make it do more, for the tests that need it:
//
//   - FAKE_LSP_LOG: a file it appends every method it receives to, one per line, and the answer
//     to its own applyEdit request as "applyEdit applied=…".
//   - FAKE_LSP_WRITES: a file name it writes at the top of each workspace folder, as jdtls writes
//     .classpath.
//   - FAKE_LSP_APPLY_EDIT: when set, it asks the client to apply an edit once initialized.
//   - FAKE_LSP_READY: a language/status type it sends a moment after it is initialized, as jdtls
//     sends ServiceReady once it has read the projects.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/akrck02/code-review/internal/lsp"
)

// server is what the fake knows: the workspace folders and the documents open, as the client has them.
type server struct {
	lock      sync.Mutex
	folders   []string
	documents map[string]string // by URI
	conn      *lsp.Conn
	stopped   chan struct{}
}

func main() {
	fake := &server{documents: map[string]string{}, stopped: make(chan struct{})}
	fake.conn = lsp.NewConn(os.Stdin, os.Stdout, fake.handle)
	select {
	case <-fake.conn.Done():
	case <-fake.stopped:
	}
}

// handle answers one message of the client.
func (fake *server) handle(method string, params json.RawMessage) (any, error) {
	logLine(method)
	switch method {
	case "initialize":
		return fake.initialize(params)
	case "initialized":
		fake.afterInitialized()
	case "workspace/didChangeWorkspaceFolders":
		fake.addFolders(params)
	case "textDocument/didOpen":
		fake.open(params)
	case "textDocument/didChange":
		fake.change(params)
	case "textDocument/definition":
		return fake.definition(params)
	case "textDocument/references":
		return fake.references(params)
	case "textDocument/hover":
		return fake.hover(params)
	case "shutdown":
		return nil, nil
	case "exit":
		close(fake.stopped)
	}
	return nil, nil
}

// logLine appends a line to the log, when the test asked for one.
func logLine(line string) {
	path := os.Getenv("FAKE_LSP_LOG")
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintln(file, line)
}

type folder struct {
	URI string `json:"uri"`
}

func (fake *server) initialize(params json.RawMessage) (any, error) {
	var asked struct {
		RootURI          string   `json:"rootUri"`
		WorkspaceFolders []folder `json:"workspaceFolders"`
	}
	if err := json.Unmarshal(params, &asked); err != nil {
		return nil, err
	}
	folders := asked.WorkspaceFolders
	if len(folders) == 0 && asked.RootURI != "" {
		folders = []folder{{URI: asked.RootURI}}
	}
	fake.addFolderList(folders)
	return map[string]any{"capabilities": map[string]any{
		"definitionProvider": true, "referencesProvider": true, "hoverProvider": true, "textDocumentSync": 1,
	}}, nil
}

func (fake *server) addFolders(params json.RawMessage) {
	var changed struct {
		Event struct {
			Added []folder `json:"added"`
		} `json:"event"`
	}
	if json.Unmarshal(params, &changed) == nil {
		fake.addFolderList(changed.Event.Added)
	}
}

// addFolderList keeps workspace folders, writing in each the file the test asked for.
func (fake *server) addFolderList(folders []folder) {
	fake.lock.Lock()
	defer fake.lock.Unlock()
	for _, each := range folders {
		path := lsp.PathOf(each.URI)
		fake.folders = append(fake.folders, path)
		if name := os.Getenv("FAKE_LSP_WRITES"); name != "" {
			_ = os.WriteFile(filepath.Join(path, name), []byte("written by the fake server\n"), 0o644)
		}
	}
}

// afterInitialized asks the client to apply an edit, when the test asked for it, and logs the answer.
func (fake *server) afterInitialized() {
	if ready := os.Getenv("FAKE_LSP_READY"); ready != "" {
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = fake.conn.Notify("language/status", map[string]string{"type": ready, "message": "ready"})
		}()
	}
	if os.Getenv("FAKE_LSP_APPLY_EDIT") == "" {
		return
	}
	go func() {
		var answer struct {
			Applied bool `json:"applied"`
		}
		err := fake.conn.Call(context.Background(), "workspace/applyEdit", map[string]any{"edit": map[string]any{}}, &answer)
		logLine(fmt.Sprintf("applyEdit applied=%v error=%v", answer.Applied, err != nil))
	}()
}

func (fake *server) open(params json.RawMessage) {
	var opened struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &opened) == nil {
		fake.lock.Lock()
		fake.documents[opened.TextDocument.URI] = opened.TextDocument.Text
		fake.lock.Unlock()
		logLine("opened " + filepath.Base(lsp.PathOf(opened.TextDocument.URI)) + " " + firstLine(opened.TextDocument.Text))
	}
}

func (fake *server) change(params json.RawMessage) {
	var changed struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if json.Unmarshal(params, &changed) == nil && len(changed.ContentChanges) > 0 {
		text := changed.ContentChanges[len(changed.ContentChanges)-1].Text
		fake.lock.Lock()
		fake.documents[changed.TextDocument.URI] = text
		fake.lock.Unlock()
		logLine("changed " + filepath.Base(lsp.PathOf(changed.TextDocument.URI)) + " " + firstLine(text))
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// text is a document as the client has it, or as it is on disk when it is not open.
func (fake *server) text(uri string) string {
	fake.lock.Lock()
	text, open := fake.documents[uri]
	fake.lock.Unlock()
	if open {
		return text
	}
	content, _ := os.ReadFile(lsp.PathOf(uri))
	return string(content)
}

// wordAt is the word of a document under a position.
func (fake *server) wordAt(params json.RawMessage) string {
	var asked lsp.TextDocumentPositionParams
	if json.Unmarshal(params, &asked) != nil {
		return ""
	}
	lines := strings.Split(fake.text(asked.TextDocument.URI), "\n")
	if asked.Position.Line >= len(lines) {
		return ""
	}
	line := []rune(lines[asked.Position.Line])
	at := min(lsp.CharacterColumn(string(line), asked.Position.Character), len(line))
	start, end := at, at
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	for end < len(line) && isWordRune(line[end]) {
		end++
	}
	return string(line[start:end])
}

func isWordRune(character rune) bool {
	return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
}

func (fake *server) definition(params json.RawMessage) (any, error) {
	word := fake.wordAt(params)
	if word == "" {
		return []lsp.Location{}, nil
	}
	declaration := regexp.MustCompile(`\b(func|type|class|interface|const|var)\s+(\([^)]*\)\s*)?` + regexp.QuoteMeta(word) + `\b`)
	return fake.find(declaration, word), nil
}

func (fake *server) references(params json.RawMessage) (any, error) {
	word := fake.wordAt(params)
	if word == "" {
		return []lsp.Location{}, nil
	}
	return fake.find(regexp.MustCompile(`\b`+regexp.QuoteMeta(word)+`\b`), word), nil
}

func (fake *server) hover(params json.RawMessage) (any, error) {
	word := fake.wordAt(params)
	if word == "" {
		return nil, nil
	}
	return map[string]any{"contents": map[string]string{"kind": "markdown", "value": "**" + word + "** from the fake server"}}, nil
}

// find is every place in the workspace folders where a pattern matches, at the word it names.
func (fake *server) find(pattern *regexp.Regexp, word string) []lsp.Location {
	fake.lock.Lock()
	folders := append([]string(nil), fake.folders...)
	fake.lock.Unlock()
	found := []lsp.Location{}
	for _, root := range folders {
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			if !entry.IsDir() && lsp.LanguageOf(path) != "" {
				found = append(found, fake.matches(path, pattern, word)...)
			}
			return nil
		})
	}
	return found
}

// matches are the places in one file where a pattern matches, at the word.
func (fake *server) matches(path string, pattern *regexp.Regexp, word string) []lsp.Location {
	var found []lsp.Location
	for number, line := range strings.Split(fake.text(lsp.FileURI(path)), "\n") {
		for _, match := range pattern.FindAllStringIndex(line, -1) {
			byteStart := match[0] + strings.LastIndex(line[match[0]:match[1]], word)
			start := lsp.UTF16Column(line, len([]rune(line[:byteStart])))
			end := start + lsp.UTF16Column(word, len([]rune(word)))
			found = append(found, lsp.Location{URI: lsp.FileURI(path), Range: lsp.Range{
				Start: lsp.Position{Line: number, Character: start}, End: lsp.Position{Line: number, Character: end},
			}})
		}
	}
	return found
}
