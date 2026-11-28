package window

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/failure"
)

var (
	buildFakePlugin  sync.Once
	fakePluginPath   string
	fakePluginFailed error
)

// fakePlugin is the plugins package's fake plugin, built once, on and installed with three
// actions (one that answers a note, one that makes a file, one that crashes) and the fake version
// control, which reads the folders marked with .fakevc, and its review source.
func fakePlugin(test *testing.T) *plugins.Plugin {
	test.Helper()
	buildFakePlugin.Do(func() {
		folder, err := os.MkdirTemp("", "fakeplugin")
		if err != nil {
			fakePluginFailed = err
			return
		}
		fakePluginPath = filepath.Join(folder, "fakeplugin")
		output, err := exec.Command("go", "build", "-o", fakePluginPath, "../plugins/testdata/fakeplugin").CombinedOutput()
		if err != nil {
			fakePluginFailed = errors.Join(err, errors.New(string(output)))
		}
	})
	if fakePluginFailed != nil {
		test.Fatal(fakePluginFailed)
	}
	plugin := &plugins.Plugin{Folder: test.TempDir(), Manifest: plugins.Manifest{Name: "fake", Command: []string{fakePluginPath},
		Contributes: plugins.Contributions{Actions: []plugins.Action{{ID: "note", Title: "Note"}, {ID: "make", Title: "Make"}, {ID: "crash", Title: "Crash"}},
			VersionControl: &plugins.VersionControlContribution{Name: "fake", Marker: ".fakevc"},
			ReviewSource:   &plugins.ReviewSourceContribution{Name: "Fake"}}}}
	plugin.Turn(true)
	test.Cleanup(plugin.Stop)
	return plugin
}

func TestAPluginsActionShowsItsNoteAndOffersTheFileItMade(test *testing.T) {
	plugin := fakePlugin(test)
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go", Plugins: []*plugins.Plugin{plugin}})
	newWindow(test, application)
	file := filepath.Join(application.workspace.Projects[0].Path, "src", "lamp.go")
	application.runAction(plugin, "note")
	if application.toast != "noted "+file {
		test.Errorf("the action's note is %q", application.toast)
	}
	application.runAction(plugin, "make")
	if application.made.path != file+".out" || application.made.label != "Open lamp.go.out" {
		test.Errorf("the file made is offered as %+v", application.made)
	}
}

func TestAPluginThatCrashesIsToldWithItsCodeAndTheWindowKeepsWorking(test *testing.T) {
	plugin := fakePlugin(test)
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go", Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	application.runAction(plugin, "crash")
	if !strings.Contains(application.toast, "plugins.crashed") || plugin.State != "Failed: plugins.crashed" {
		test.Fatalf("a crash was told %q, the plugin's state is %q", application.toast, plugin.State)
	}
	window.Frames(2)
	if !application.viewer.IsOpen() || !sidebarShows(application, "lamp.go") {
		test.Error("the window stopped showing the file after the plugin crashed")
	}
	application.runAction(plugin, "note")
	if !strings.HasPrefix(application.toast, "noted ") || plugin.State != "On" {
		test.Errorf("after the crash the plugin answered %q, its state %q", application.toast, plugin.State)
	}
}

func TestAnIdleFrameWithAPluginOnMakesNoMoreThanWithout(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go", Plugins: []*plugins.Plugin{fakePlugin(test)}})
	window := newWindow(test, application)
	if allocations := idleFrameAllocations(window.Frames); allocations > idleFilesFrameAllocations {
		test.Errorf("an idle frame with a plugin on allocates %v times, more than %d", allocations, idleFilesFrameAllocations)
	}
}

// makeFakeRepository writes, beside the git repository, a repository of the fake version control
// whose base has lamp.txt with one line: now it has two, and new.txt is added. Only the folder that
// holds both is returned.
func makeFakeRepository(test *testing.T, alsoCrash bool) string {
	test.Helper()
	folder := makeRepository(test)
	harbour := filepath.Join(folder, "harbour")
	files := map[string]string{".fakevc/base/lamp.txt": "dark\n", "lamp.txt": "dark\nlit\n", "new.txt": "fresh\n"}
	if alsoCrash {
		files[".fakevc/crash"] = ""
	}
	for path, text := range files {
		full := filepath.Join(harbour, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return folder
}

// projectNamed is the workspace's project of that name.
func projectNamed(test *testing.T, application *Window, name string) *codebase.Project {
	test.Helper()
	for _, project := range application.workspace.Projects {
		if project.Name == name {
			return project
		}
	}
	test.Fatalf("no project %q among %d", name, len(application.workspace.Projects))
	return nil
}

func TestAProjectAPluginMarksListsItsChangesAndShowsAFilesTwoSides(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeFakeRepository(test, false)}, Synchronous: true, WorkingTree: true,
		Plugins: []*plugins.Plugin{fakePlugin(test)}})
	newWindow(test, application)
	harbour := projectNamed(test, application, "harbour")
	if harbour.Error != nil || harbour.Branch != "trunk" || strings.Join(harbour.ChangedPaths(), " ") != "lamp.txt new.txt" {
		test.Fatalf("the project reads branch %q, changes %v (%v)", harbour.Branch, harbour.ChangedPaths(), harbour.Error)
	}
	document := codebase.OpenDocument(harbour, "lamp.txt", true)
	if document.Error != nil || len(document.OldLines) != 1 || len(document.NewLines) != 2 || len(document.Diff) != 3 {
		test.Errorf("lamp.txt shows %d old lines, %d new, %d of diff (%v)", len(document.OldLines), len(document.NewLines), len(document.Diff), document.Error)
	}
	lighthouse := projectNamed(test, application, "lighthouse")
	if lighthouse.Control != nil || lighthouse.Error != nil || len(lighthouse.Changes) == 0 {
		test.Errorf("the git project beside it reads %d changes (%v)", len(lighthouse.Changes), lighthouse.Error)
	}
}

func TestAVersionControlPluginThatCrashesShowsItsCodeInItsProjectAndNothingElse(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeFakeRepository(test, true)}, Synchronous: true, WorkingTree: true,
		Open: "lighthouse/src/lamp.go", Plugins: []*plugins.Plugin{fakePlugin(test)}})
	window := newWindow(test, application)
	if harbour := projectNamed(test, application, "harbour"); !failure.Is(harbour.Error, plugins.Crashed) {
		test.Fatalf("the project's error is %v, want the code plugins.crashed", harbour.Error)
	}
	window.Frames(2)
	if lighthouse := projectNamed(test, application, "lighthouse"); lighthouse.Error != nil || !application.viewer.IsOpen() {
		test.Errorf("the git project broke too (%v), or the file open closed", lighthouse.Error)
	}
}

// The program starts again only when the person asked for it, from the settings' banner.
func TestTheWindowAsksToStartAgainOnlyAfterARestart(test *testing.T) {
	application := newReader(Settings{Synchronous: true})
	if application.RestartAsked() {
		test.Fatal("a window nobody restarted asks to start again")
	}
	application.Restart()
	if !application.RestartAsked() {
		test.Error("a window restarted does not ask to start again")
	}
}

// A setting's description longer than its sheet is wide breaks onto more lines, so none of it runs
// past the sheet's edge, where it could not be read.
func TestASettingsLongDescriptionWrapsInsideItsSheet(test *testing.T) {
	plugin := &plugins.Plugin{Manifest: plugins.Manifest{Name: "lamp", Settings: []plugins.Setting{{ID: "wick",
		Title: "Wick", Type: plugins.SettingText, Description: strings.Repeat("A wick lit along the pier at night. ", 6)}}}}
	plugin.Turn(true)
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	for _, label := range []string{"Settings (⌘,)", "Plugins"} {
		if err := window.Click(label); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
	}
	gear, err := window.ElementIn("lamp", "Settings") // the card's gear, not the rail's button
	if err != nil {
		test.Fatal(err)
	}
	window.ClickAt(gear.Rect.X+gear.Rect.Width/2, gear.Rect.Y+gear.Rect.Height/2)
	window.Frames(2)
	done, err := window.Element("Done")
	if err != nil {
		test.Fatal(err)
	}
	edge := done.Rect.X + done.Rect.Width
	lines := map[float32]bool{}
	for _, command := range window.DrawList().Commands {
		if command.Kind != draw2d.TextKind || command.Text != "pier" {
			continue
		}
		lines[command.Baseline] = true
		if right := command.Area.X + command.Area.Width; right > edge {
			test.Errorf("a word of the description ends at %v, past the sheet's edge at %v", right, edge)
		}
	}
	if len(lines) < 2 {
		test.Errorf("the description takes %d lines, want it wrapped onto several", len(lines))
	}
}

// The whole card of a plugin is its power switch, while the buttons inside it keep their own
// work: the gear opens its settings and leaves it on.
func TestAClickOnAPluginsCardTurnsItOnAndOffButItsGearOnlyOpensItsSettings(test *testing.T) {
	plugin := &plugins.Plugin{Manifest: plugins.Manifest{Name: "lamp", Settings: []plugins.Setting{{ID: "wick", Type: plugins.SettingText}}}}
	plugin.Turn(true)
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	for _, label := range []string{"Settings (⌘,)", "Plugins"} {
		if err := window.Click(label); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
	}
	clickIn := func(label string) {
		test.Helper()
		found, err := window.ElementIn("lamp", label)
		if err != nil {
			test.Fatal(err)
		}
		window.ClickAt(found.Rect.X+found.Rect.Width/2, found.Rect.Y+found.Rect.Height/2)
		window.Frames(2)
	}
	clickIn("On: click to turn it off")
	if plugin.On {
		test.Fatal("a click on the card of a plugin on leaves it on")
	}
	clickIn("Off: click to turn it on")
	if !plugin.On {
		test.Fatal("a click on the card of a plugin off leaves it off")
	}
	clickIn("Settings")
	if !plugin.On {
		test.Error("the gear turned the plugin off")
	}
	if _, err := window.Element("Done"); err != nil {
		test.Errorf("the gear opened no settings: %v", err)
	}
}

// The warning of a plugin that misses a program opens its prerequisites: the program, and the
// command to run in a terminal when Coddy cannot run it itself.
func TestTheWarningOfAPluginMissingAProgramShowsHowToInstallIt(test *testing.T) {
	plugin := &plugins.Plugin{Manifest: plugins.Manifest{Name: "lamp", Requires: []plugins.Requirement{{Program: "no-such-wick-here",
		Install: map[string][]string{runtime.GOOS: {"sudo wick-installer add no-such-wick-here"}}}}}}
	plugin.Turn(true)
	plugin.CheckRequirements()
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true,
		Plugins: []*plugins.Plugin{plugin}})
	window := newWindow(test, application)
	for _, label := range []string{"Settings (⌘,)", "Plugins"} {
		if err := window.Click(label); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
	}
	warning, err := window.ElementIn("lamp", "##warning")
	if err != nil {
		test.Fatal(err)
	}
	window.ClickAt(warning.Rect.X+warning.Rect.Width/2, warning.Rect.Y+warning.Rect.Height/2)
	window.Frames(2)
	texts := strings.Join(window.Texts(), " ")
	for _, wanted := range []string{"Prerequisites", "no-such-wick-here", "sudo", "wick-installer"} {
		if !strings.Contains(texts, wanted) {
			test.Errorf("the prerequisites do not say %q: %s", wanted, texts)
		}
	}
	if _, err := window.Element("Install"); err == nil {
		test.Error("the prerequisites offer to install a command that asks for a password")
	}
	if _, err := window.Element("Close"); err == nil {
		test.Error("the prerequisites have a Close button beside their cross")
	}
	if err := window.Click("x"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if strings.Contains(strings.Join(window.Texts(), " "), "Prerequisites") {
		test.Error("the cross leaves the prerequisites open")
	}
}
