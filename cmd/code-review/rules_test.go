package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// moduleRoot is the folder of go.mod, seen from this package's folder.
const moduleRoot = "../.."

// goSources gives each Go file of the module that is not a test or under testdata, with its text.
func goSources(test *testing.T) map[string]string {
	test.Helper()
	sources := map[string]string{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir // programs for the tests, which Go leaves out of the build too
		}
		if entry.IsDir() && path != moduleRoot && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir // not the module's: tools' folders, such as other checkouts
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources[path] = string(content)
		return nil
	})
	if err != nil {
		test.Fatal(err)
	}
	return sources
}

// Every error carries a code from failure, so no error is made any other way.
func TestNoErrorIsMadeWithoutACode(test *testing.T) {
	for path, source := range goSources(test) {
		for _, maker := range []string{"errors.New(", "fmt.Errorf("} {
			if strings.Contains(source, maker) {
				test.Errorf("%s makes an error with %s; declare a code in its errors.go", path, maker)
			}
		}
	}
}

// droppedError is a line that throws away what a call gives back: `_ = f()` or `_, _ = f()`.
var droppedError = regexp.MustCompile(`^\s*_(, _)* = `)

// An error thrown away says why on its line, so a reader sees it was meant.
func TestNoErrorIsDroppedWithoutAReasonOnItsLine(test *testing.T) {
	for path, source := range goSources(test) {
		for number, line := range strings.Split(source, "\n") {
			if droppedError.MatchString(line) && !strings.Contains(line, "//") {
				test.Errorf("%s:%d drops what it is given without saying why: %s", path, number+1, strings.TrimSpace(line))
			}
		}
	}
}

// The reader writes inside a repository only through workspace.Save; the only other file that
// writes is the one keeping its own marks, in its own folder. Git itself runs read-only commands.
func TestOnlyTheSaveWritesInARepository(test *testing.T) {
	allowed := map[string]bool{
		filepath.Join(moduleRoot, "internal/workspace/save.go"):  true,
		filepath.Join(moduleRoot, "internal/review/reviewed.go"): true,
		// what language servers wrote, kept in the reader's own folder as the viewed marks are
		filepath.Join(moduleRoot, "internal/workspace/written.go"): true,
		// old indexes of servers, in their data folder: the reader's own, never a repository
		filepath.Join(moduleRoot, "internal/lsp/indexes.go"): true,
		// the Gradle home of a server that imports Gradle builds, in its data folder, and the models
		// of the projects it imported, there too
		filepath.Join(moduleRoot, "internal/gradle/home.go"):   true,
		filepath.Join(moduleRoot, "internal/gradle/models.go"): true,
		// how long a server took to read the projects, in its data folder
		filepath.Join(moduleRoot, "internal/lsp/reading.go"): true,
		// the models of the projects a server imported, in its data folder
		filepath.Join(moduleRoot, "internal/lsp/models.go"): true,
	}
	writers := []string{"os.WriteFile(", "os.Create(", "os.CreateTemp(", "os.OpenFile(", "os.Rename(",
		"os.Remove(", "os.RemoveAll(", "os.Mkdir(", "os.MkdirAll(", "os.Chmod(", "os.Truncate(",
		"os.Symlink(", "os.Link(", "ioutil.WriteFile("}
	for path, source := range goSources(test) {
		for _, writer := range writers {
			if strings.Contains(source, writer) && !allowed[path] {
				test.Errorf("%s writes with %s; only workspace.Save may write in a repository", path, writer)
			}
		}
	}
}

// dependencies lists every package a package of this module imports, directly or not.
func dependencies(test *testing.T, name string) []string {
	test.Helper()
	command := exec.Command("go", "list", "-deps", "github.com/akrck02/code-review/internal/"+name)
	command.Dir = moduleRoot
	output, err := command.Output()
	if err != nil {
		test.Fatalf("go list %s: %v", name, err)
	}
	return strings.Fields(string(output))
}

// importsAny gives the first of the packages that a package imports, directly or not.
func importsAny(test *testing.T, name string, forbidden ...string) string {
	test.Helper()
	for _, dependency := range dependencies(test, name) {
		for _, path := range forbidden {
			if dependency == path || strings.HasPrefix(dependency, path+"/") {
				return dependency
			}
		}
	}
	return ""
}

// The domain knows nothing of the window: what the reader opens, and the text being edited, can be
// read and changed without one.
func TestTheDomainImportsNoInterface(test *testing.T) {
	interfaces := []string{
		"github.com/akrck02/go-ui",
		"github.com/akrck02/code-review/internal/theme",
		"github.com/akrck02/code-review/internal/pane",
		"github.com/akrck02/code-review/internal/window",
		"github.com/akrck02/code-review/internal/mode",
		"github.com/akrck02/code-review/internal/files",
		"github.com/akrck02/code-review/internal/review",
	}
	for _, domain := range []string{"workspace", "text", "lsp"} {
		if found := importsAny(test, domain, interfaces...); found != "" {
			test.Errorf("%s imports %s", domain, found)
		}
	}
}

// A pane asks the window for what it needs through an interface of its own, so the arrows only
// point down: the window knows its panes, and no pane knows the window.
func TestNoPaneImportsTheWindow(test *testing.T) {
	for _, pane := range []string{"theme", "pane", "viewer", "sidebar", "finder", "settings", "explorer", "work", "session", "navigation", "servers"} {
		if found := importsAny(test, pane, "github.com/akrck02/code-review/internal/window"); found != "" {
			test.Errorf("%s imports %s", pane, found)
		}
	}
}

// The language table is what every other package reads, so it reads none of them: it is data.
func TestTheLanguageTableImportsNothingOfTheReader(test *testing.T) {
	self := "github.com/akrck02/code-review/internal/language"
	for _, dependency := range dependencies(test, "language") {
		if dependency != self && strings.HasPrefix(dependency, "github.com/akrck02/code-review/") {
			test.Errorf("language imports %s", dependency)
		}
	}
}

// The language servers' client calls the gradle package for a server that imports Gradle builds;
// gradle knows builds and their daemons, and nothing of the protocol, so the arrow points one way.
func TestGradleKnowsNothingOfTheLanguageServers(test *testing.T) {
	if found := importsAny(test, "gradle", "github.com/akrck02/code-review/internal/lsp"); found != "" {
		test.Errorf("gradle imports %s", found)
	}
}

// A mode is one package and one line in main: the window holds the modes it is given and imports
// none of them.
func TestTheWindowImportsNoMode(test *testing.T) {
	modes := []string{"github.com/akrck02/code-review/internal/files", "github.com/akrck02/code-review/internal/review"}
	if found := importsAny(test, "window", modes...); found != "" {
		test.Errorf("window imports %s", found)
	}
}

// longestFunction is how many lines a function may take, from func to its closing brace.
const longestFunction = 40

// A long function usually does two jobs, so none is longer than forty lines, tests included.
func TestNoFunctionIsLongerThanFortyLines(test *testing.T) {
	files := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return err
		}
		for _, declaration := range file.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if !isFunction || function.Body == nil {
				continue
			}
			start, end := files.Position(function.Pos()), files.Position(function.End())
			if lines := end.Line - start.Line + 1; lines > longestFunction {
				test.Errorf("%s:%d: %s is %d lines long; split it into functions of %d at most", path, start.Line, function.Name.Name, lines, longestFunction)
			}
		}
		return nil
	})
	if err != nil {
		test.Fatal(err)
	}
}

// longestFile is how many lines a file of the program may take: a longer one is usually two concepts.
const longestFile = 400

// Each file is one concept a reader can hold in mind, so none of the program's is longer than
// four hundred lines. Tests are left out: their length is their cases.
func TestNoFileIsLongerThanFourHundredLines(test *testing.T) {
	for path, content := range goSources(test) {
		if lines := strings.Count(content, "\n"); lines > longestFile {
			test.Errorf("%s is %d lines long; split it by part into files of %d at most", path, lines, longestFile)
		}
	}
}

// modePackages are the modes, whose methods are checked by TestNoModeHasAMethodThatDoesNothing.
var modePackages = []string{"internal/files", "internal/review", "internal/uses"}

// A power only some modes have is an interface of its own (mode.WithFileButton…), so no mode
// implements one with a method that does nothing to satisfy a contract it does not need.
func TestNoModeHasAMethodThatDoesNothing(test *testing.T) {
	files := token.NewFileSet()
	for _, folder := range modePackages {
		packages, err := parser.ParseDir(files, filepath.Join(moduleRoot, folder), func(info fs.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if err != nil {
			test.Fatal(err)
		}
		for _, found := range packages {
			for _, file := range found.Files {
				for _, declaration := range file.Decls {
					if method, isMethod := declaration.(*ast.FuncDecl); isMethod && method.Recv != nil && doesNothing(method) {
						test.Errorf("%s: %s does nothing; make it an optional interface the mode leaves out",
							files.Position(method.Pos()), method.Name.Name)
					}
				}
			}
		}
	}
}

// doesNothing tells whether a function's body is empty, or only gives back zero values (0, "",
// nil, an empty literal). A lone boolean is an answer to a question, so false is not nothing.
func doesNothing(function *ast.FuncDecl) bool {
	body := function.Body
	if body == nil || len(body.List) == 0 {
		return true
	}
	returned, isReturn := body.List[0].(*ast.ReturnStmt)
	if len(body.List) != 1 || !isReturn || len(returned.Results) == 0 {
		return false
	}
	if len(returned.Results) == 1 && isBoolean(returned.Results[0]) {
		return false
	}
	for _, result := range returned.Results {
		if !isZero(result) {
			return false
		}
	}
	return true
}

// isBoolean tells whether an expression is the word true or false.
func isBoolean(expression ast.Expr) bool {
	name, isName := expression.(*ast.Ident)
	return isName && (name.Name == "true" || name.Name == "false")
}

// isZero tells whether an expression is a zero value written out: 0, "", false, nil or T{}.
func isZero(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.BasicLit:
		return value.Value == "0" || value.Value == `""`
	case *ast.Ident:
		return value.Name == "nil" || value.Name == "false"
	case *ast.CompositeLit:
		return len(value.Elts) == 0
	}
	return false
}

// contributingGuide is the page that lists the packages, seen from this package's folder.
const contributingGuide = moduleRoot + "/docs/8-contributing.md"

// listedPackages are the packages named in the contributing guide's map of the code: the first
// word of each line of its code blocks that starts with "internal/".
func listedPackages(test *testing.T) map[string]bool {
	test.Helper()
	content, err := os.ReadFile(contributingGuide)
	if err != nil {
		test.Fatal(err)
	}
	listed := map[string]bool{}
	inCode := false
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
			continue
		}
		if inCode && strings.HasPrefix(line, "internal/") {
			listed[strings.Fields(line)[0]] = true
		}
	}
	return listed
}

// The contributing guide is where a newcomer finds each package, so it names every one of them
// and none that is gone.
func TestTheContributingGuideListsEveryPackage(test *testing.T) {
	listed := listedPackages(test)
	entries, err := os.ReadDir(moduleRoot + "/internal")
	if err != nil {
		test.Fatal(err)
	}
	present := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			present["internal/"+entry.Name()] = true
		}
	}
	for name := range present {
		if !listed[name] {
			test.Errorf("%s is not in %s", name, contributingGuide)
		}
	}
	for name := range listed {
		if !present[name] {
			test.Errorf("%s names %s, which is not a package", contributingGuide, name)
		}
	}
}
