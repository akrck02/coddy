package gradle

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/akrck02/code-review/internal/builds"
)

// A server that imports Gradle builds (kotlin-lsp) imports every project on every start, minutes
// for many projects, even with its index built. It can be given, instead, the model of a project
// it imported before: its modules, libraries and settings, in a workspace.json, read by its json
// importer. The reader keeps that model for each project, with a fingerprint of the project's build
// files, and starts the server with a stand-in folder holding the model for each project whose
// build files did not change; the server reads the project's own files by their paths, written in
// the model. On the big workspace: ready in 13 s instead of 452 s, with no Gradle daemon.

const (
	modelsFolder    = "models"      // in the server's data folder, a folder per project
	fingerprintFile = "build-files" // beside the model, the fingerprint of the build files it is of
	exportFolder    = "export"      // in modelsFolder, where the server writes the whole model
)

const (
	// ModelFile is the model, where the server's json importer reads it in a stand-in folder.
	ModelFile = "workspace.json"
	// ModelTool is kotlin-lsp's importer of a workspace.json, named in its buildTools option.
	ModelTool = "json"
	// ExportCommand is kotlin-lsp's command that writes the model it imported into a folder.
	ExportCommand = "exportWorkspace"
)

// modelFiles are the names of the files of a Gradle build whose change makes a model old: its
// settings and builds, and what they read.
var modelFiles = append(slices.Clone(builds.Gradle.Files()), "gradle.properties", "libs.versions.toml", "gradle-wrapper.properties")

// ExportFolder is the folder of a server's data folder it is asked to export its model into.
func ExportFolder(dataFolder string) string {
	return filepath.Join(dataFolder, modelsFolder, exportFolder)
}

// notBuildFolders are folders no build file is looked for in: what builds and tools write.
var notBuildFolders = []string{"build", "out", "target", "node_modules"}

// Fingerprint is a hash of a project's build files, their places and their contents; "" when none
// was found.
func Fingerprint(project string) string {
	hash, found := md5.New(), false
	_ = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error { // skips what it cannot read
		switch {
		case err != nil:
			return nil
		case entry.IsDir() && path != project && (strings.HasPrefix(entry.Name(), ".") || slices.Contains(notBuildFolders, entry.Name())):
			return filepath.SkipDir
		case entry.IsDir() || !slices.Contains(modelFiles, entry.Name()):
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relative, _ := filepath.Rel(project, path)
		hash.Write([]byte(relative + "\x00"))
		hash.Write(content)
		hash.Write([]byte{0})
		found = true
		return nil
	})
	if !found {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// ModelFolder is the folder of a server's data folder that stands in for a project, holding its
// model.
func ModelFolder(dataFolder, project string) string {
	hash := md5.Sum([]byte(project))
	return filepath.Join(dataFolder, modelsFolder, hex.EncodeToString(hash[:]))
}

// HasModel tells whether a project has a model kept with the fingerprint its build files have now.
func HasModel(dataFolder, project, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	folder := ModelFolder(dataFolder, project)
	kept, err := os.ReadFile(filepath.Join(folder, fingerprintFile))
	if err != nil || strings.TrimSpace(string(kept)) != fingerprint {
		return false
	}
	_, err = os.Stat(filepath.Join(folder, ModelFile))
	return err == nil
}

// KeepModel writes a project's model and the fingerprint of the build files it is of.
func KeepModel(dataFolder, project, fingerprint string, model map[string]any) error {
	folder := ModelFolder(dataFolder, project)
	content, err := json.Marshal(model)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(folder, ModelFile), content, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, fingerprintFile), []byte(fingerprint+"\n"), 0o644)
}

// ModelOf is a project's part of the model the server exported for all its folders: the modules
// whose content is in the project, the libraries and SDKs they use, their settings, and what else
// the model holds as it is; and how many modules that is. The model's paths may be the project's
// as given or with its links resolved: kotlin-lsp resolved ~/Code, a link to another disk, and
// left /var, a link to /private/var, as it was.
func ModelOf(whole map[string]any, project string) (map[string]any, int) {
	folders := []string{project}
	if real, err := filepath.EvalSymlinks(project); err == nil && real != project {
		folders = append(folders, real)
	}
	part := map[string]any{}
	for key, value := range whole {
		part[key] = value
	}
	var modules []any
	names, used := map[string]bool{}, map[string]map[string]bool{"library": {}, "sdk": {}}
	for _, each := range listOf(whole["modules"]) {
		module, _ := each.(map[string]any)
		if module == nil || !moduleIn(module, folders) {
			continue
		}
		modules = append(modules, module)
		names[stringOf(module["name"])] = true
		for _, dependency := range listOf(module["dependencies"]) {
			if fields, _ := dependency.(map[string]any); fields != nil && used[stringOf(fields["type"])] != nil {
				used[stringOf(fields["type"])][stringOf(fields["name"])] = true
			}
		}
	}
	part["modules"] = modules
	part["libraries"] = named(whole["libraries"], "name", used["library"])
	part["sdks"] = named(whole["sdks"], "name", used["sdk"])
	part["kotlinSettings"] = named(whole["kotlinSettings"], "module", names)
	part["javaSettings"] = named(whole["javaSettings"], "module", names)
	return part, len(modules)
}

// moduleIn tells whether a module of a model is a project's, by the project's folders (as given,
// and resolved): one of its content roots is in it, or else the build it comes from is. The content
// roots come first: a build copied from another checkout names that one.
func moduleIn(module map[string]any, folders []string) bool {
	for _, root := range listOf(module["contentRoots"]) {
		if fields, _ := root.(map[string]any); fields != nil && inFolder(stringOf(fields["path"]), folders) {
			return true
		}
	}
	return inFolder(stringOf(module["externalProjectPath"]), folders)
}

// homeMark is how kotlin-lsp writes the person's home folder at the start of a path of its model.
const homeMark = "<HOME>"

// inFolder tells whether a path of a model is one of some folders or in one; the model writes the
// person's home as homeMark, which it reads back itself.
func inFolder(path string, folders []string) bool {
	if rest, marked := strings.CutPrefix(path, homeMark); marked {
		if home, err := os.UserHomeDir(); err == nil {
			path = home + rest
		}
	}
	for _, folder := range folders {
		if path != "" && (path == folder || strings.HasPrefix(path, folder+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

// named are the entries of a list of a model whose field is one of some names; an empty list, not
// nil, so the model says there are none.
func named(list any, field string, names map[string]bool) []any {
	chosen := []any{}
	for _, each := range listOf(list) {
		if fields, _ := each.(map[string]any); fields != nil && names[stringOf(fields[field])] {
			chosen = append(chosen, each)
		}
	}
	return chosen
}

// Modules tells how many modules a model holds, of every project.
func Modules(model map[string]any) int { return len(listOf(model["modules"])) }

// listOf is a value of a model read as a list; nil when it is none.
func listOf(value any) []any {
	list, _ := value.([]any)
	return list
}

// stringOf is a value of a model read as text; "" when it is none.
func stringOf(value any) string {
	text, _ := value.(string)
	return text
}
