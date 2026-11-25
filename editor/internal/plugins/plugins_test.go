package plugins

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/akrck02/coddy/sdk"
	"github.com/akrck02/coddy/sdk/jsonrpc"
)

var (
	buildFake  sync.Once
	fakePath   string
	fakeFailed error
)

// fakePlugin builds the fake plugin once, for every test.
func fakePlugin(test *testing.T) string {
	test.Helper()
	buildFake.Do(func() {
		folder, err := os.MkdirTemp("", "fakeplugin")
		if err != nil {
			fakeFailed = err
			return
		}
		fakePath = filepath.Join(folder, "fakeplugin")
		output, err := exec.Command("go", "build", "-o", fakePath, "./testdata/fakeplugin").CombinedOutput()
		if err != nil {
			fakeFailed = errors.Join(err, errors.New(string(output)))
		}
	})
	if fakeFailed != nil {
		test.Fatal(fakeFailed)
	}
	return fakePath
}

// install writes a plugin's folder with its manifest, in a folder of plugins.
func install(test *testing.T, plugins, name, manifest string) {
	test.Helper()
	folder := filepath.Join(plugins, name)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, manifestName), []byte(manifest), 0o644); err != nil {
		test.Fatal(err)
	}
}

// fake is the fake plugin, installed and on.
func fake(test *testing.T, arguments ...string) *Plugin {
	test.Helper()
	command := append([]string{fakePlugin(test)}, arguments...)
	plugin := &Plugin{Manifest: Manifest{Name: "fake", Command: command}, Folder: test.TempDir(), On: true}
	test.Cleanup(plugin.Stop)
	return plugin
}

func TestAManifestSaysWhatThePluginAddsAndUnknownFieldsAreIgnored(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "python", `{"name": "python-lsp", "version": "0.1.0", "colour": "blue",
		"command": ["pylsp", "--check-parent-process"], "speaks": "lsp",
		"requires": [{"program": "pylsp", "install": {"`+runtime.GOOS+`": ["pipx install python-lsp-server"]}}],
		"contributes": {"languageServer": {"languages": ["python"], "rootMarkers": ["pyproject.toml"]}}}`)
	install(test, folder, "export", `{"name": "export-pdf", "command": ["export-pdf"], "speaks": "coddy",
		"contributes": {"actions": [{"id": "export", "title": "Export to PDF"}]}}`)
	found := Find(folder, []string{"export-pdf"})
	if len(found) != 2 || found[0].Name != "export-pdf" || found[1].Name != "python-lsp" {
		test.Fatalf("found %d plugins: %+v", len(found), found)
	}
	if found[0].On || !found[1].On || found[0].State != stateOff || found[1].State != stateOn {
		test.Errorf("every plugin but the one named off is on: %v %q, %v %q", found[0].On, found[0].State, found[1].On, found[1].State)
	}
	server, isServer := found[1].Server()
	if !isServer || server.Name != "pylsp" || server.Command != "pylsp" || server.Arguments[0] != "--check-parent-process" ||
		server.Languages[0] != "python" || server.Install == "" {
		test.Errorf("the language server declared is %+v", server)
	}
	if _, isServer := found[0].Server(); isServer {
		test.Error("a plugin that speaks coddy runs as a language server")
	}
	if actions := found[0].Contributes.Actions; len(actions) != 1 || actions[0].Title != "Export to PDF" {
		test.Errorf("the actions declared are %+v", actions)
	}
}

func TestAManifestThatCannotBeReadIsListedFailedWithItsCode(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "broken", `{"name": `)
	install(test, folder, "nameless", `{}`)
	install(test, folder, "idle", `{"name": "idle", "contributes": {"actions": [{"id": "a", "title": "A"}]}}`)
	install(test, folder, "mute", `{"name": "mute", "command": ["mute"], "contributes": {"actions": [{"id": "a", "title": "A"}]}}`)
	for _, plugin := range Find(folder, nil) {
		if !errors.Is(plugin.failed, BadManifest) || plugin.State != "Failed: plugins.bad-manifest" {
			test.Errorf("%s is %q", plugin.Name, plugin.State)
		}
	}
}

func TestAnActionAnswersWithItsNoteAndTheFileItMade(test *testing.T) {
	plugin := fake(test)
	file := filepath.Join(test.TempDir(), "lamp.go")
	noted, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "note", File: file})
	if err != nil || noted.Note != "noted "+file {
		test.Fatalf("the note is %+v, %v", noted, err)
	}
	made, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "make", File: file})
	if err != nil || made.Created != file+".out" {
		test.Fatalf("made %+v, %v", made, err)
	}
}

func TestACrashIsToldWithItsCodeAndThePluginStartsAgainOnItsNextUse(test *testing.T) {
	plugin := fake(test)
	_, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "crash"})
	plugin.Tell(err)
	if !errors.Is(err, Crashed) || plugin.State != "Failed: plugins.crashed" {
		test.Fatalf("a crash gave %v, the state %q", err, plugin.State)
	}
	noted, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "note", File: "/lamp.go"})
	if err != nil || noted.Note == "" {
		test.Errorf("after a crash the plugin answered %+v, %v", noted, err)
	}
}

func TestAnActionThePluginCannotRunIsItsOwnWords(test *testing.T) {
	_, err := fake(test).Run(context.Background(), sdk.ActionParams{Action: "fly"})
	// the plugin's error crosses the pipe as words only, so its code is looked for by its name
	if err == nil || !strings.Contains(err.Error(), jsonrpc.UnknownMethod.Name()) {
		test.Errorf("an action the plugin cannot run gave %v", err)
	}
}

// Each protocol carries what it can: a language server is asked only what the Language Server
// Protocol asks, so the rest is Coddy's protocol's.
func TestAPluginContributesOnlyWhatItsProtocolCarries(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "server-with-action", `{"name": "a", "command": ["a"], "speaks": "lsp",
		"contributes": {"languageServer": {"languages": ["go"]}, "actions": [{"id": "a", "title": "A"}]}}`)
	install(test, folder, "server-without-languages", `{"name": "b", "command": ["b"], "speaks": "lsp",
		"contributes": {"languageServer": {}}}`)
	install(test, folder, "coddy-with-server", `{"name": "c", "command": ["c"], "speaks": "coddy",
		"contributes": {"languageServer": {"languages": ["go"]}}}`)
	install(test, folder, "unknown-protocol", `{"name": "d", "command": ["d"], "speaks": "dap"}`)
	for _, name := range []string{"server-with-action", "server-without-languages", "coddy-with-server", "unknown-protocol"} {
		if _, err := ReadManifest(filepath.Join(folder, name)); !errors.Is(err, BadManifest) {
			test.Errorf("%s was read with %v, want plugins.bad-manifest", name, err)
		}
	}
}

func TestAVersionControlNeedsAMarkerAndACommand(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "fine", `{"name": "fine", "command": ["fine"], "speaks": "coddy", "contributes": {"versionControl": {"name": "fine", "marker": ".fine"}}}`)
	install(test, folder, "unmarked", `{"name": "unmarked", "command": ["unmarked"], "speaks": "coddy", "contributes": {"versionControl": {"name": "unmarked"}}}`)
	install(test, folder, "idle", `{"name": "idle", "contributes": {"versionControl": {"name": "idle", "marker": ".idle"}}}`)
	if manifest, err := ReadManifest(filepath.Join(folder, "fine")); err != nil || manifest.Contributes.VersionControl.Marker != ".fine" {
		test.Errorf("a whole version control was read as %+v (%v)", manifest.Contributes.VersionControl, err)
	}
	for _, name := range []string{"unmarked", "idle"} {
		if _, err := ReadManifest(filepath.Join(folder, name)); !errors.Is(err, BadManifest) {
			test.Errorf("%s was read with %v, want plugins.bad-manifest", name, err)
		}
	}
}

func TestAReviewSourceListsItsPullRequestsAndTheirComments(test *testing.T) {
	plugin := fake(test)
	reviews, err := plugin.Reviews(context.Background(), test.TempDir())
	if err != nil || len(reviews) != 1 || reviews[0].ID != "7" || reviews[0].HeadRef != "feature/brighter" || reviews[0].BaseRef != "main" {
		test.Fatalf("the pull requests are %+v (%v)", reviews, err)
	}
	comments, err := plugin.ReviewComments(context.Background(), test.TempDir(), "7")
	if err != nil || len(comments) != 1 || comments[0].File != "src/lamp.go" || comments[0].Line != 4 {
		test.Errorf("the comments are %+v (%v)", comments, err)
	}
}

func TestAReviewSourceThatCrashesIsToldWithItsCode(test *testing.T) {
	_, err := fake(test).ReviewComments(context.Background(), test.TempDir(), "8")
	if !errors.Is(err, Crashed) {
		test.Errorf("a crash gave %v, want plugins.crashed", err)
	}
}

func TestAReviewSourceNeedsACommand(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "idle", `{"name": "idle", "contributes": {"reviewSource": {"name": "Idle"}}}`)
	if _, err := ReadManifest(filepath.Join(folder, "idle")); !errors.Is(err, BadManifest) {
		test.Errorf("a review source without a command was read with %v", err)
	}
}

// A plugin says what it needs on the computer and how to get it, so a person sees what is
// missing before a use fails.
func TestAPluginListsTheProgramsItRequiresThatAreMissingWithHowToInstallThem(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "lamp", `{"name": "lamp", "command": ["lamp"], "speaks": "coddy", "requires": [
		{"program": "sh"},
		{"program": "no-such-wick-here", "install": {"`+runtime.GOOS+`": ["no-such-installer-here install wick", "sh -c true"]}},
		{"program": "no-such-oil-here", "install": {"`+runtime.GOOS+`": ["sudo sh -c true"]}},
		{"program": "no-such-glass-here"}]}`)
	found := Find(folder, nil)
	if len(found) != 1 || found[0].State != stateOn {
		test.Fatalf("found %+v", found)
	}
	missing := found[0].Missing
	if len(missing) != 3 || missing[0].Program != "no-such-wick-here" {
		test.Fatalf("missing %+v, want the three programs not here", missing)
	}
	if missing[0].Command != "sh -c true" || !missing[0].Runnable {
		test.Errorf("the wick installs with %q (runnable %v), want the install whose installer is here", missing[0].Command, missing[0].Runnable)
	}
	if missing[1].Command != "sudo sh -c true" || missing[1].Runnable {
		test.Errorf("the oil installs with %q (runnable %v), want it shown, not run: it asks for a password", missing[1].Command, missing[1].Runnable)
	}
	if missing[2].Command != "" || missing[2].Runnable {
		test.Errorf("the glass, with no install for this system, installs with %q", missing[2].Command)
	}
}

// A language server's install is that of the program it requires, for the servers' panel.
func TestALanguageServerInstallsAsTheProgramItRequires(test *testing.T) {
	manifest := Manifest{Name: "lamp", Command: []string{"lamp-server"}, Speaks: SpeaksLanguageServer,
		Requires: []Requirement{{Program: "lamp-server", Install: map[string][]string{runtime.GOOS: {"sh -c true"}},
			Environment: []string{"LAMP=lit"}}},
		Contributes: Contributions{LanguageServer: &LanguageServer{Languages: []string{"go"}}}}
	server, found := manifest.Server()
	if !found || server.Install != "sh -c true" || len(server.InstallEnvironment) != 1 {
		test.Errorf("the server installs with %q %v", server.Install, server.InstallEnvironment)
	}
}

// The card shows one warning icon whose note says what is wrong, so the note must name each
// problem, and be empty once all is well.
func TestAPluginsWarningNamesWhatItMissesAndHowToGetIt(test *testing.T) {
	plugin := &Plugin{Manifest: Manifest{Name: "lamp", Requires: []Requirement{
		{Program: "no-such-wick-here", Install: map[string][]string{runtime.GOOS: {"sh -c true"}}},
		{Program: "no-such-oil-here", Install: map[string][]string{runtime.GOOS: {"sudo sh -c true"}}},
	}}}
	plugin.Turn(true)
	plugin.CheckRequirements()
	if !plugin.Installable {
		test.Error("the wick, which Coddy can install, is not installable")
	}
	for _, program := range []string{"no-such-wick-here", "no-such-oil-here"} {
		if !strings.Contains(plugin.Warning, program) {
			test.Errorf("the warning %q does not name %s", plugin.Warning, program)
		}
	}
	plugin.Requires = nil
	plugin.CheckRequirements()
	if plugin.Warning != "" || plugin.Installable {
		test.Errorf("with nothing missing the warning is %q", plugin.Warning)
	}
	plugin.Tell(Crashed)
	if !strings.Contains(plugin.Warning, Crashed.Name()) {
		test.Errorf("after a crash the warning is %q, want its code", plugin.Warning)
	}
}

// The settings grey a plugin that cannot work now, so ready must mean on and complete.
func TestAPluginIsReadyOnlyWhenOnAndMissingNothing(test *testing.T) {
	plugin := &Plugin{Manifest: Manifest{Name: "lamp", Requires: []Requirement{{Program: "no-such-wick-here"}}}}
	plugin.CheckRequirements()
	plugin.Turn(true)
	if plugin.Ready() {
		test.Error("a plugin missing a program is ready")
	}
	plugin.Requires = nil
	plugin.CheckRequirements()
	if !plugin.Ready() {
		test.Error("a plugin on and missing nothing is not ready")
	}
	plugin.Turn(false)
	if plugin.Ready() {
		test.Error("a plugin turned off is ready")
	}
}
