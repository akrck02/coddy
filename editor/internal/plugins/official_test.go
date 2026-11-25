package plugins

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/akrck02/coddy/editor/internal/lsp"
)

// officialFolder is where Coddy's own plugins are, from this package's folder
var officialFolder = filepath.Join("..", "..", "..", "plugins")

// The official plugins give the servers Coddy ran from a table of its own before they moved out of
// its core: installed, Go, Java and Kotlin read as they did.
func TestTheOfficialLanguagePluginsRunTheirServersAsCoddyDidBefore(test *testing.T) {
	want := map[string]lsp.Server{
		"go": {Name: "gopls", Command: "gopls", Arguments: []string{}, Languages: []string{"go"}, RootMarkers: []string{"go.mod"},
			Install: "go install golang.org/x/tools/gopls@latest", InstallEnvironment: []string{"GOTOOLCHAIN=auto"}},
		"java": {Name: "jdtls", Command: "jdtls", Languages: []string{"java"}, OnePerWindow: true, DataFlag: "-data", FoldersInOptions: true,
			Arguments: []string{"--jvm-arg=-Djava.import.generatesMetadataFilesAtProjectRoot=false"},
			Install:   "brew install jdtls", Ready: lsp.Readiness{Status: "ServiceReady"}, ReadsAll: true,
			Options: map[string]any{"extendedClientCapabilities": map[string]any{"classFileContentsSupport": true}},
			Settings: map[string]any{"java": map[string]any{
				"import":        map[string]any{"generatesMetadataFilesAtProjectRoot": false},
				"autobuild":     map[string]any{"enabled": false},
				"configuration": map[string]any{"updateBuildConfiguration": "disabled"},
			}},
			Memory: lsp.Memory{Most: "2g", Flag: "--jvm-arg=-Xmx"}, Import: lsp.Import{OwnGradle: true}},
		"kotlin": {Name: "kotlin-lsp", Command: "kotlin-lsp", Arguments: []string{"--stdio"}, DataFlag: "--system-path",
			Languages: []string{"kotlin"}, OnePerWindow: true, Install: "brew install JetBrains/utils/kotlin-lsp",
			Index: lsp.Index{Folder: "indexes", Keep: 1, Option: "indexDir", Retired: []string{"rocks"}, NeedsFree: 10 << 30}, ReadsAll: true,
			Import: lsp.Import{Models: true, OwnGradle: true, After: []string{"jdtls"}}, Ready: lsp.Readiness{AfterWork: true},
			Memory: lsp.Memory{Most: "2g", Variable: "IJ_JAVA_OPTIONS"}},
	}
	for name, server := range want {
		manifest, err := ReadManifest(filepath.Join(officialFolder, name))
		if err != nil {
			test.Fatal(err)
		}
		if got, found := manifest.Server(); !found || !reflect.DeepEqual(got, server) {
			test.Errorf("%s runs %+v\nwant %+v", name, got, server)
		}
	}
}

func TestGitIsBuiltInAndOnUnlessTurnedOff(test *testing.T) {
	builtIn := BuiltIn(nil)
	if len(builtIn) != 1 || builtIn[0].Name != GitName || !builtIn[0].BuiltIn || !builtIn[0].On || GitOff(builtIn) {
		test.Fatalf("the built-in plugins are %+v, want git on", builtIn)
	}
	if !GitOff(BuiltIn([]string{GitName})) {
		test.Error("git named off is on")
	}
	if GitOff(nil) {
		test.Error("git is off with no plugin listed: a window without plugins would read no repository")
	}
}

// Coddy's own plugins, official and examples, say that Coddy made them, as git built in does: the
// settings show it on their cards.
func TestCoddysOwnPluginsSayCoddyMadeThem(test *testing.T) {
	folders, err := filepath.Glob(filepath.Join(officialFolder, "*", manifestName))
	if err != nil || len(folders) == 0 {
		test.Fatalf("no plugin found in %s: %v", officialFolder, err)
	}
	for _, path := range folders {
		manifest, err := ReadManifest(filepath.Dir(path))
		if err != nil || manifest.Author != "Coddy" {
			test.Errorf("%s is by %q: %v", path, manifest.Author, err)
		}
	}
	if git := BuiltIn(nil)[0]; git.Author != "Coddy" {
		test.Errorf("git is by %q", git.Author)
	}
}

// The check in the settings promises a plugin is Coddy's own, so it is not given for the author's
// word alone.
func TestOnlyCoddysOwnPluginsAreVerified(test *testing.T) {
	cases := []struct {
		plugin *Plugin
		want   bool
	}{
		{BuiltIn(nil)[0], true},
		{&Plugin{Manifest: Manifest{Name: "go", Author: coddyAuthor}}, true},
		{&Plugin{Manifest: Manifest{Name: "lamp", Author: coddyAuthor}}, false},
		{&Plugin{Manifest: Manifest{Name: "go", Author: "Someone"}}, false},
	}
	for _, each := range cases {
		if got := isVerified(each.plugin); got != each.want {
			test.Errorf("%s by %q verified is %v, want %v", each.plugin.Name, each.plugin.Author, got, each.want)
		}
	}
}
