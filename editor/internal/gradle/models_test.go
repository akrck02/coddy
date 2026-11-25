package gradle

import (
	"os"
	"path/filepath"
	"testing"
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
	part, modules := ModelOf(whole, lamp)
	if modules != 1 || len(listOf(part["libraries"])) != 2 || len(listOf(part["sdks"])) != 1 ||
		len(listOf(part["kotlinSettings"])) != 1 || len(listOf(part["javaSettings"])) != 0 || part["externalSystem"] != "GRADLE" {
		test.Errorf("lamp's model is %v", part)
	}
	if part, modules := ModelOf(whole, keeper); modules != 1 || len(listOf(part["libraries"])) != 1 {
		test.Errorf("keeper, its build named elsewhere, has %d modules and %v", modules, part["libraries"])
	}
	home, _ := os.UserHomeDir()
	marked := model(homeMark+"/Code/lamp", homeMark+"/Code/keeper")
	if _, modules := ModelOf(marked, filepath.Join(home, "Code", "lamp")); modules != 1 {
		test.Error("a project in the person's home, written <HOME>/…, has no modules")
	}
	if _, modules := ModelOf(whole, filepath.Join(code, "lam")); modules != 0 {
		test.Error("a folder whose name begins another's took its modules")
	}
	link := filepath.Join(test.TempDir(), "lamp")
	if os.Symlink(lamp, link) == nil {
		os.MkdirAll(lamp, 0o755)
		if _, modules := ModelOf(whole, link); modules != 1 {
			test.Error("a project reached through a link has no modules: the model's paths are resolved")
		}
	}
}
