package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot is the folder of go.mod, seen from this package's folder.
const moduleRoot = "../.."

// goSources gives each Go file of the module that is not a test, with its text.
func goSources(test *testing.T) map[string]string {
	test.Helper()
	sources := map[string]string{}
	err := filepath.WalkDir(moduleRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
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
	for _, domain := range []string{"workspace", "text"} {
		if found := importsAny(test, domain, interfaces...); found != "" {
			test.Errorf("%s imports %s", domain, found)
		}
	}
}

// A pane asks the window for what it needs through an interface of its own, so the arrows only
// point down: the window knows its panes, and no pane knows the window.
func TestNoPaneImportsTheWindow(test *testing.T) {
	for _, pane := range []string{"theme", "pane", "viewer", "sidebar", "finder"} {
		if found := importsAny(test, pane, "github.com/akrck02/code-review/internal/window"); found != "" {
			test.Errorf("%s imports %s", pane, found)
		}
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
