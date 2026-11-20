package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// write puts files in a folder, making the folders they are in.
func write(test *testing.T, folder string, files map[string]string) {
	for name, content := range files {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			test.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			test.Fatal(err)
		}
	}
}

func TestAFingerprintChangesWithTheBuildFilesAlone(test *testing.T) {
	project := test.TempDir()
	if Fingerprint(project) != "" {
		test.Error("a folder with no build has a fingerprint")
	}
	write(test, project, map[string]string{"settings.gradle.kts": "rootProject.name = \"lamp\"\n",
		"app/build.gradle.kts": "plugins {}\n", "app/src/Lamp.kt": "object Lamp\n"})
	first := Fingerprint(project)
	write(test, project, map[string]string{"app/src/Lamp.kt": "object Lamp { val beam = 2 }\n",
		"build/tmp/build.gradle": "written by a build\n", ".gradle/gradle.properties": "x=1\n"})
	if Fingerprint(project) != first {
		test.Error("a source, or what a build writes, changed the fingerprint")
	}
	write(test, project, map[string]string{"app/build.gradle.kts": "plugins { kotlin(\"jvm\") }\n"})
	if Fingerprint(project) == first {
		test.Error("a module's build changed and the fingerprint did not")
	}
	write(test, project, map[string]string{"app/build.gradle.kts": "plugins {}\n", "gradle/libs.versions.toml": "[versions]\n"})
	if Fingerprint(project) == first {
		test.Error("the versions catalogue was added and the fingerprint did not change")
	}
}

// model is an exported model of two projects, lamp and keeper, both using one library; keeper's
// build names another checkout, as one copied from elsewhere does.
func model(lamp, keeper string) map[string]any {
	module := func(name, root, build string, libraries ...string) any {
		dependencies := []any{map[string]any{"type": "moduleSource"}, map[string]any{"type": "sdk", "name": "21"}}
		for _, library := range libraries {
			dependencies = append(dependencies, map[string]any{"type": "library", "name": library})
		}
		return map[string]any{"name": name, "contentRoots": []any{map[string]any{"path": root}},
			"externalProjectPath": build, "dependencies": dependencies}
	}
	return map[string]any{
		"modules": []any{module("lamp.main", lamp+"/src/main", lamp, "kotlin-stdlib", "lamp-only"),
			module("keeper.main", keeper+"/src/main", "/elsewhere/keeper", "kotlin-stdlib")},
		"libraries":      []any{map[string]any{"name": "kotlin-stdlib"}, map[string]any{"name": "lamp-only"}, map[string]any{"name": "unused"}},
		"sdks":           []any{map[string]any{"name": "21"}, map[string]any{"name": "17"}},
		"kotlinSettings": []any{map[string]any{"module": "lamp.main"}, map[string]any{"module": "keeper.main"}},
		"javaSettings":   []any{map[string]any{"module": "keeper.main"}},
		"externalSystem": "GRADLE",
	}
}

func TestAProjectsModelIsItsModulesAndWhatTheyUse(test *testing.T) {
	code, _ := filepath.EvalSymlinks(test.TempDir()) // an exported model's paths are resolved
	lamp, keeper := filepath.Join(code, "lamp"), filepath.Join(code, "keeper")
	whole := model(lamp, keeper)
	part, modules := modelOf(whole, lamp)
	if modules != 1 || len(listOf(part["libraries"])) != 2 || len(listOf(part["sdks"])) != 1 ||
		len(listOf(part["kotlinSettings"])) != 1 || len(listOf(part["javaSettings"])) != 0 || part["externalSystem"] != "GRADLE" {
		test.Errorf("lamp's model is %v", part)
	}
	if part, modules := modelOf(whole, keeper); modules != 1 || len(listOf(part["libraries"])) != 1 {
		test.Errorf("keeper, its build named elsewhere, has %d modules and %v", modules, part["libraries"])
	}
	home, _ := os.UserHomeDir()
	marked := model(homeMark+"/Code/lamp", homeMark+"/Code/keeper")
	if _, modules := modelOf(marked, filepath.Join(home, "Code", "lamp")); modules != 1 {
		test.Error("a project in the person's home, written <HOME>/…, has no modules")
	}
	if _, modules := modelOf(whole, filepath.Join(code, "lam")); modules != 0 {
		test.Error("a folder whose name begins another's took its modules")
	}
	link := filepath.Join(test.TempDir(), "lamp")
	if os.Symlink(lamp, link) == nil {
		os.MkdirAll(lamp, 0o755)
		if _, modules := modelOf(whole, link); modules != 1 {
			test.Error("a project reached through a link has no modules: the model's paths are resolved")
		}
	}
}

// A project whose build files did not change since its model was kept is given to the server as
// its stand-in, read with the json importer; one with no model, or an old one, is imported.
func TestAProjectIsReadFromItsModelWhileItsBuildIsUnchanged(test *testing.T) {
	data, lamp, keeper := test.TempDir(), test.TempDir(), test.TempDir()
	write(test, lamp, map[string]string{"build.gradle.kts": "plugins {}\n"})
	write(test, keeper, map[string]string{"build.gradle.kts": "plugins {}\n"})
	if err := keepModel(data, lamp, Fingerprint(lamp), map[string]any{"modules": []any{}}); err != nil {
		test.Fatal(err)
	}
	server, _ := serverNamed("kotlin-lsp")
	client := &Client{Server: server, data: data}
	client.chooseModels([]string{lamp, keeper})
	if client.serverFolder(lamp) != modelFolder(data, lamp) || client.serverFolder(keeper) != keeper {
		test.Fatalf("lamp is given as %s, keeper as %s", client.serverFolder(lamp), client.serverFolder(keeper))
	}
	if _, err := os.Stat(filepath.Join(client.serverFolder(lamp), modelFile)); err != nil {
		test.Error("lamp's stand-in holds no model")
	}
	options := client.initializationOptions([]string{lamp, keeper})
	if options[server.IndexOption] != indexFolder(data, server, []string{lamp, keeper}) {
		test.Error("the index is not the projects' own: a project read from its model made another")
	}
	tools, _ := options["buildTools"].(map[string]string)
	if len(tools) != 1 || tools[FileURI(modelFolder(data, lamp))] != modelTool {
		test.Errorf("the server is told to read %v", options["buildTools"])
	}
	if client.toImport[keeper] == "" || client.toImport[lamp] != "" {
		test.Errorf("to import: %v", client.toImport)
	}
	write(test, lamp, map[string]string{"build.gradle.kts": "plugins { kotlin(\"jvm\") }\n"})
	again := &Client{Server: server, data: data}
	again.chooseModels([]string{lamp})
	if again.serverFolder(lamp) != lamp {
		test.Error("a project whose build changed is still read from its old model")
	}
}

// The projects read from their models count as done when the server says how far it is.
func TestAProjectReadFromItsModelCountsAsDone(test *testing.T) {
	client := &Client{folders: map[string]bool{"/lamp": true, "/keeper": true}, imported: map[string]time.Time{},
		standIns: map[string]string{"/lamp": "/data/models/1"}, working: map[string]*work{}}
	client.noteImport([]byte(`{"message": "Importing with Gradle: /keeper"}`))
	if progress := client.Progress(); progress.Done != 2 || progress.Total != 2 {
		test.Errorf("done %d of %d", progress.Done, progress.Total)
	}
	client.noteImport([]byte(`{"message": "Importing with Gradle: /data/models/1"}`))
	if _, found := client.imported["/lamp"]; !found {
		test.Error("a stand-in imported is not counted as its project")
	}
}
