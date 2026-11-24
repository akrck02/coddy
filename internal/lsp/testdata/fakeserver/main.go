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
//   - FAKE_LSP_ASKS: a section of settings it asks the client for once initialized, logging the answer.
//   - FAKE_JVM_OPTIONS: logged at initialize, with its arguments and initializationOptions.
//   - FAKE_LSP_READY: a language/status type it sends a moment after it is initialized, as jdtls
//     sends ServiceReady once it has read the projects; it answers every request at once meanwhile.
//   - FAKE_LSP_SLOW_EXIT: milliseconds it takes to end after exit, as jdtls saves its workspace;
//     it logs "saved" just before it ends.
//   - FAKE_LSP_BLANK: when set, it answers every request with nothing until it sends FAKE_LSP_READY.
//   - FAKE_LSP_JARS: when set, a name declared in no workspace folder but imported
//     ("import org.lighthouse.Lamp") is declared in a jar, answered as kotlin-lsp answers it.
//   - FAKE_LSP_NO_FOLDERS: when set, it does not say it follows workspace folders, as kotlin-lsp.
//   - FAKE_LSP_BUSY: milliseconds of work it reports with $/progress once initialized, answering
//     every request with nothing meanwhile, as kotlin-lsp does while Gradle imports the projects:
//     it logs "Importing with Gradle: <folder>" for each folder, and reports 50 % halfway.
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

	"github.com/akrck02/code-review/internal/language"
	"github.com/akrck02/code-review/internal/lsp"
)

// server is what the fake knows: the workspace folders and the documents open, as the client has them.
type server struct {
	lock      sync.Mutex
	folders   []string
	documents map[string]string // by URI
	conn      *lsp.Conn
	stopped   chan struct{}
	busy      bool
	blank     bool // answering nothing until it says it is ready
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
	case "textDocument/didClose":
		fake.close(params)
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		return fake.request(method, params)
	case "textDocument/documentSymbol":
		return fake.symbols(params), nil
	case "shutdown":
		return nil, nil
	case "exit":
		go fake.exit()
	}
	return nil, nil
}

// request answers a question about a document, or nothing while the fake reports work running.
func (fake *server) request(method string, params json.RawMessage) (any, error) {
	fake.lock.Lock()
	busy := fake.busy || fake.blank
	fake.lock.Unlock()
	switch {
	case busy:
		return nil, nil
	case method == "textDocument/definition":
		return fake.definition(params)
	case method == "textDocument/references":
		return fake.references(params)
	}
	return fake.hover(params)
}

// work reports work running for some milliseconds, as a begin and an end of $/progress.
func (fake *server) work(milliseconds string) {
	var length int
	if _, err := fmt.Sscan(milliseconds, &length); err != nil {
		return
	}
	fake.lock.Lock()
	fake.busy = true
	fake.lock.Unlock()
	_ = fake.conn.Notify("$/progress", map[string]any{"token": 7, "value": map[string]string{"kind": "begin", "title": "Importing"}})
	fake.lock.Lock()
	for _, folder := range fake.folders {
		_ = fake.conn.Notify("intellij/importLog", map[string]any{"type": 3, "message": "Importing with Gradle: " + folder})
	}
	fake.lock.Unlock()
	go func() {
		time.Sleep(time.Duration(length/2) * time.Millisecond)
		_ = fake.conn.Notify("$/progress", map[string]any{"token": 7, "value": map[string]any{"kind": "report", "percentage": 50}})
		time.Sleep(time.Duration(length-length/2) * time.Millisecond)
		fake.lock.Lock()
		fake.busy = false
		fake.lock.Unlock()
		_ = fake.conn.Notify("$/progress", map[string]any{"token": 7, "value": map[string]string{"kind": "end"}})
	}()
}

// logLine appends a line to the log, when the test asked for one.
// exit ends the fake, after the time it was asked to take.
func (fake *server) exit() {
	if slow := os.Getenv("FAKE_LSP_SLOW_EXIT"); slow != "" {
		var length int
		_, _ = fmt.Sscan(slow, &length)
		time.Sleep(time.Duration(length) * time.Millisecond)
		logLine("saved")
	}
	close(fake.stopped)
}

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
		RootURI          string          `json:"rootUri"`
		WorkspaceFolders []folder        `json:"workspaceFolders"`
		Options          json.RawMessage `json:"initializationOptions"`
	}
	if err := json.Unmarshal(params, &asked); err != nil {
		return nil, err
	}
	logLine("arguments " + strings.Join(os.Args[1:], " ") + " · options " + string(asked.Options) +
		" · jvm options " + os.Getenv("FAKE_JVM_OPTIONS"))
	folders := asked.WorkspaceFolders
	if len(folders) == 0 && asked.RootURI != "" {
		folders = []folder{{URI: asked.RootURI}}
	}
	fake.addFolderList(folders)
	capabilities := map[string]any{
		"definitionProvider": true, "referencesProvider": true, "hoverProvider": true, "textDocumentSync": 1,
	}
	if os.Getenv("FAKE_LSP_NO_FOLDERS") == "" {
		capabilities["workspace"] = map[string]any{"workspaceFolders": map[string]any{"supported": true, "changeNotifications": true}}
	}
	return map[string]any{"capabilities": capabilities}, nil
}

func (fake *server) addFolders(params json.RawMessage) {
	var changed struct {
		Event struct {
			Added   []folder `json:"added"`
			Removed []folder `json:"removed"`
		} `json:"event"`
	}
	if json.Unmarshal(params, &changed) != nil {
		return
	}
	fake.addFolderList(changed.Event.Added)
	fake.lock.Lock()
	defer fake.lock.Unlock()
	for _, each := range changed.Event.Removed {
		path := lsp.PathOf(each.URI)
		logLine("removed " + filepath.Base(path))
		for index, kept := range fake.folders {
			if kept == path {
				fake.folders = append(fake.folders[:index], fake.folders[index+1:]...)
				break
			}
		}
	}
}

// addFolderList keeps workspace folders, writing in each the file the test asked for.
func (fake *server) addFolderList(folders []folder) {
	fake.lock.Lock()
	defer fake.lock.Unlock()
	for _, each := range folders {
		path := lsp.PathOf(each.URI)
		logLine("reads " + filepath.Base(path))
		fake.folders = append(fake.folders, path)
		if name := os.Getenv("FAKE_LSP_WRITES"); name != "" {
			_ = os.WriteFile(filepath.Join(path, name), []byte("written by the fake server\n"), 0o644)
		}
	}
}

// afterInitialized asks the client to apply an edit, when the test asked for it, and logs the answer.
func (fake *server) afterInitialized() {
	if busy := os.Getenv("FAKE_LSP_BUSY"); busy != "" {
		fake.work(busy)
	}
	if ready := os.Getenv("FAKE_LSP_READY"); ready != "" {
		fake.lock.Lock()
		fake.blank = os.Getenv("FAKE_LSP_BLANK") != ""
		fake.lock.Unlock()
		go func() {
			time.Sleep(300 * time.Millisecond)
			fake.lock.Lock()
			fake.blank = false
			fake.lock.Unlock()
			_ = fake.conn.Notify("language/status", map[string]string{"type": ready, "message": "ready"})
		}()
	}
	if section := os.Getenv("FAKE_LSP_ASKS"); section != "" {
		go func() {
			var answer json.RawMessage
			_ = fake.conn.Call(context.Background(), "workspace/configuration",
				map[string]any{"items": []map[string]string{{"section": section}}}, &answer)
			logLine("configuration " + string(answer))
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

// close forgets a document, as a server reads it from disk again once it is closed.
func (fake *server) close(params json.RawMessage) {
	var closed struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &closed) == nil {
		fake.lock.Lock()
		delete(fake.documents, closed.TextDocument.URI)
		fake.lock.Unlock()
		logLine("closed " + filepath.Base(lsp.PathOf(closed.TextDocument.URI)))
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
	found := fake.find(declaration, word)
	if len(found) > 0 || os.Getenv("FAKE_LSP_JARS") == "" {
		return found, nil
	}
	return fake.inJar(params, word), nil
}

// inJar is where a name imported by the document is declared, in a jar of its package.
func (fake *server) inJar(params json.RawMessage, word string) []lsp.Location {
	var asked lsp.TextDocumentPositionParams
	_ = json.Unmarshal(params, &asked)
	imported := regexp.MustCompile(`import\s+([\w.]+)\.` + regexp.QuoteMeta(word) + `\b`).FindStringSubmatch(fake.text(asked.TextDocument.URI))
	if imported == nil {
		return []lsp.Location{}
	}
	uri := "jar:///cache/lighthouse.jar!/" + strings.ReplaceAll(imported[1], ".", "/") + "/" + word + ".class"
	return []lsp.Location{{URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 3}, End: lsp.Position{Line: 3}}}}
}

func (fake *server) references(params json.RawMessage) (any, error) {
	word := fake.wordAt(params)
	if word == "" {
		return []lsp.Location{}, nil
	}
	return fake.find(regexp.MustCompile(`\b`+regexp.QuoteMeta(word)+`\b`), word), nil
}

// symbols are a document's declarations, each from its line to the next one's, or to the end.
func (fake *server) symbols(params json.RawMessage) []lsp.Symbol {
	var asked struct {
		TextDocument lsp.TextDocumentIdentifier `json:"textDocument"`
	}
	_ = json.Unmarshal(params, &asked)
	declaration := regexp.MustCompile(`\b(?:func|class|interface|type)\s+(\w+)|^\s+(?:public|private|protected)\s+[\w<>]+\s+(\w+)\(`)
	lines := strings.Split(fake.text(asked.TextDocument.URI), "\n")
	symbols := []lsp.Symbol{}
	for number, line := range lines {
		found := declaration.FindStringSubmatchIndex(line)
		if found == nil {
			continue
		}
		start, end := found[2], found[3]
		if start < 0 {
			start, end = found[4], found[5]
		}
		name := lsp.Range{Start: lsp.Position{Line: number, Character: start}, End: lsp.Position{Line: number, Character: end}}
		if count := len(symbols); count > 0 {
			symbols[count-1].Range.End = lsp.Position{Line: number}
		}
		symbols = append(symbols, lsp.Symbol{Name: line[start:end], Selection: name,
			Range: lsp.Range{Start: lsp.Position{Line: number}, End: lsp.Position{Line: len(lines)}}})
	}
	return symbols
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
			if !entry.IsDir() && language.ServerLanguageOf(path) != "" {
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
