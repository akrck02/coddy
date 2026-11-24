package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
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

// A project whose build files did not change since its model was kept is given to the server as
// its stand-in, read with the json importer; one with no model, or an old one, is imported.
func TestAProjectIsReadFromItsModelWhileItsBuildIsUnchanged(test *testing.T) {
	data, lamp, keeper := test.TempDir(), test.TempDir(), test.TempDir()
	write(test, lamp, map[string]string{"build.gradle.kts": "plugins {}\n"})
	write(test, keeper, map[string]string{"build.gradle.kts": "plugins {}\n"})
	if err := gradle.KeepModel(data, lamp, gradle.Fingerprint(lamp), map[string]any{"modules": []any{}}); err != nil {
		test.Fatal(err)
	}
	server, _ := serverNamed("kotlin-lsp")
	client := &Client{Server: server, data: data}
	client.chooseModels([]string{lamp, keeper})
	if client.serverFolder(lamp) != gradle.ModelFolder(data, lamp) || client.serverFolder(keeper) != keeper {
		test.Fatalf("lamp is given as %s, keeper as %s", client.serverFolder(lamp), client.serverFolder(keeper))
	}
	if _, err := os.Stat(filepath.Join(client.serverFolder(lamp), gradle.ModelFile)); err != nil {
		test.Error("lamp's stand-in holds no model")
	}
	options := client.initializationOptions([]string{lamp, keeper})
	if options[server.Index.Option] != indexFolder(data, server, []string{lamp, keeper}) {
		test.Error("the index is not the projects' own: a project read from its model made another")
	}
	tools, _ := options["buildTools"].(map[string]string)
	if len(tools) != 1 || tools[FileURI(gradle.ModelFolder(data, lamp))] != gradle.ModelTool {
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
