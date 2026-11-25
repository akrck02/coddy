package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
		if entry.IsDir() && path != moduleRoot && fileExists(filepath.Join(path, "go.mod")) {
			return filepath.SkipDir // a module of its own, such as the official plugins
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

// fileExists says whether a file is there.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
		// the plugins folder, in the reader's own folder, made when the settings open it
		filepath.Join(moduleRoot, "internal/plugins/folder.go"): true,
		// the values set for each plugin's settings, in the reader's own folder, when one changes
		filepath.Join(moduleRoot, "internal/plugins/settings.go"): true,
		// the profiles of keys, in the reader's own folder, when one is made, changed or removed
		filepath.Join(moduleRoot, "internal/keymap/profile.go"): true,
		// the reader's own folder, moved once from its old name
		filepath.Join(moduleRoot, "cmd/coddy/datafolder.go"): true,
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

// dependencies lists every package an internal package of this module imports, directly or not.
func dependencies(test *testing.T, name string) []string {
	test.Helper()
	return dependenciesOf(test, "github.com/akrck02/coddy/editor/internal/"+name)
}

// dependenciesOf lists every package a package imports, directly or not, by its import path.
func dependenciesOf(test *testing.T, path string) []string {
	test.Helper()
	command := exec.Command("go", "list", "-deps", path)
	command.Dir = moduleRoot
	output, err := command.Output()
	if err != nil {
		test.Fatalf("go list %s: %v", path, err)
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
		"github.com/akrck02/coddy/editor/internal/theme",
		"github.com/akrck02/coddy/editor/internal/pane",
		"github.com/akrck02/coddy/editor/internal/window",
		"github.com/akrck02/coddy/editor/internal/mode",
		"github.com/akrck02/coddy/editor/internal/files",
		"github.com/akrck02/coddy/editor/internal/review",
	}
	for _, domain := range []string{"workspace", "text", "lsp", "plugins"} {
		if found := importsAny(test, domain, interfaces...); found != "" {
			test.Errorf("%s imports %s", domain, found)
		}
	}
}

// A pane asks the window for what it needs through an interface of its own, so the arrows only
// point down: the window knows its panes, and no pane knows the window.
func TestNoPaneImportsTheWindow(test *testing.T) {
	for _, pane := range []string{"theme", "pane", "viewer", "sidebar", "finder", "settings", "work", "session", "navigation", "servers", "plugins"} {
		if found := importsAny(test, pane, "github.com/akrck02/coddy/editor/internal/window"); found != "" {
			test.Errorf("%s imports %s", pane, found)
		}
	}
}

// The language table is what every other package reads, so it reads none of them: it is data.
func TestTheLanguageTableImportsNothingOfTheReader(test *testing.T) {
	self := "github.com/akrck02/coddy/editor/internal/language"
	for _, dependency := range dependencies(test, "language") {
		if dependency != self && strings.HasPrefix(dependency, "github.com/akrck02/coddy/") {
			test.Errorf("language imports %s", dependency)
		}
	}
}

// The language servers' client calls the gradle package for a server that imports Gradle builds;
// gradle knows builds and their daemons, and nothing of the protocol, so the arrow points one way.
func TestGradleKnowsNothingOfTheLanguageServers(test *testing.T) {
	if found := importsAny(test, "gradle", "github.com/akrck02/coddy/editor/internal/lsp"); found != "" {
		test.Errorf("gradle imports %s", found)
	}
}

// A mode is one package and one line in main: the window holds the modes it is given and imports
// none of them.
func TestTheWindowImportsNoMode(test *testing.T) {
	modes := []string{"github.com/akrck02/coddy/editor/internal/files", "github.com/akrck02/coddy/editor/internal/review"}
	if found := importsAny(test, "window", modes...); found != "" {
		test.Errorf("window imports %s", found)
	}
}
