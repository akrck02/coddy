package lsp

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/akrck02/go-renderer/failure"
)

// A server that imports Gradle builds (kotlin-lsp) imports every project on every start, minutes
// for many projects, even with its index built. It can be given, instead, the model of a project
// it imported before: its modules, libraries and settings, in a workspace.json, read by its json
// importer. The reader keeps that model for each project, with a fingerprint of the project's build
// files, and starts the server with a stand-in folder holding the model for each project whose
// build files did not change; the server reads the project's own files by their paths, written in
// the model. On the big workspace: ready in 13 s instead of 452 s, with no Gradle daemon.

const (
	modelsFolder    = "models"          // in the server's data folder, a folder per project
	modelFile       = "workspace.json"  // the model, where the server's json importer reads it
	fingerprintFile = "build-files"     // beside it, the fingerprint of the build files it is of
	exportFolder    = "export"          // in modelsFolder, where the server writes the whole model
	modelTool       = "json"            // kotlin-lsp's importer of a workspace.json
	exportCommand   = "exportWorkspace" // kotlin-lsp's command that writes the model it imported
)

// buildFiles are the names of the files of a Gradle build whose change makes a model old.
var buildFiles = []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts",
	"gradle.properties", "libs.versions.toml", "gradle-wrapper.properties"}

// notBuildFolders are folders no build file is looked for in: what builds and tools write.
var notBuildFolders = []string{"build", "out", "target", "node_modules"}

// Fingerprint is a hash of a project's build files, their places and their contents; "" when none
// was found.
func Fingerprint(project string) string {
	hash, found := md5.New(), false
	_ = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case entry.IsDir() && path != project && (strings.HasPrefix(entry.Name(), ".") || slices.Contains(notBuildFolders, entry.Name())):
			return filepath.SkipDir
		case entry.IsDir() || !slices.Contains(buildFiles, entry.Name()):
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

// modelFolder is the folder that stands in for a project, holding its model.
func modelFolder(dataFolder, project string) string {
	hash := md5.Sum([]byte(project))
	return filepath.Join(dataFolder, modelsFolder, hex.EncodeToString(hash[:]))
}

// hasModel tells whether a project has a model kept with the fingerprint its build files have now.
func hasModel(dataFolder, project, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	folder := modelFolder(dataFolder, project)
	kept, err := os.ReadFile(filepath.Join(folder, fingerprintFile))
	if err != nil || strings.TrimSpace(string(kept)) != fingerprint {
		return false
	}
	_, err = os.Stat(filepath.Join(folder, modelFile))
	return err == nil
}

// keepModel writes a project's model and the fingerprint of the build files it is of.
func keepModel(dataFolder, project, fingerprint string, model map[string]any) error {
	folder := modelFolder(dataFolder, project)
	content, err := json.Marshal(model)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(folder, modelFile), content, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, fingerprintFile), []byte(fingerprint+"\n"), 0o644)
}

// modelOf is a project's part of the model the server exported for all its folders: the modules
// whose content is in the project, the libraries and SDKs they use, their settings, and what else
// the model holds as it is; and how many modules that is. The model's paths may be the project's
// as given or with its links resolved: kotlin-lsp resolved ~/Code, a link to another disk, and
// left /var, a link to /private/var, as it was.
func modelOf(whole map[string]any, project string) (map[string]any, int) {
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

func listOf(value any) []any {
	list, _ := value.([]any)
	return list
}

func stringOf(value any) string {
	text, _ := value.(string)
	return text
}

// modelWait is how long a server is given to write the model it imported.
const modelWait = 2 * time.Minute

// chooseModels sees, before the server starts, which projects have a model kept for the build
// files they have now: those it reads from their model, through a stand-in folder; it imports the
// others with Gradle, and their models are kept once it is done (keepModels).
func (client *Client) chooseModels(projects []string) {
	client.standIns, client.toImport = map[string]string{}, map[string]string{}
	for _, project := range projects {
		fingerprint := Fingerprint(project)
		switch {
		case hasModel(client.data, project, fingerprint):
			client.standIns[project] = modelFolder(client.data, project)
		case fingerprint != "":
			client.toImport[project] = fingerprint
		}
	}
}

// serverFolder is the folder a project is given to the server as: its stand-in when it is read
// from its model, else its own.
func (client *Client) serverFolder(project string) string {
	client.lock.Lock()
	defer client.lock.Unlock()
	if standIn, found := client.standIns[project]; found {
		return standIn
	}
	return project
}

// projectOf is the project a folder the server names stands for: itself, unless it is a stand-in.
func (client *Client) projectOf(folder string) string {
	for project, standIn := range client.standIns {
		if standIn == folder {
			return project
		}
	}
	return folder
}

// buildTools tells kotlin-lsp to read each stand-in folder with its json importer.
func (client *Client) buildTools() map[string]string {
	tools := map[string]string{}
	for _, standIn := range client.standIns {
		tools[FileURI(standIn)] = modelTool
	}
	return tools
}

// keepModels waits for the server to be done importing the projects it imports with Gradle, then
// keeps the model of each.
func (client *Client) keepModels() {
	ticker := time.NewTicker(gradleCheck)
	defer ticker.Stop()
	for !client.settled(time.Now()) {
		select {
		case <-client.exited:
			return
		case <-ticker.C:
		}
	}
	err := client.exportModels()
	client.lock.Lock()
	client.modelsAt, client.modelsErr = time.Now(), ""
	if err != nil {
		client.modelsErr = err.Error()
	}
	client.lock.Unlock()
	client.changed()
}

// exportModels asks the server for the model it imported, in a folder of the reader's, and keeps
// each imported project's part of it. A project of no module there was not imported: it is
// imported with Gradle again next time.
func (client *Client) exportModels() error {
	folder := filepath.Join(client.data, modelsFolder, exportFolder)
	_ = os.RemoveAll(folder)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return failure.Wrap(BadModel, folder, err)
	}
	defer os.RemoveAll(folder)
	ctx, cancel := context.WithTimeout(context.Background(), modelWait)
	defer cancel()
	arguments := map[string]any{"command": exportCommand, "arguments": []string{folder}}
	if err := client.conn.Call(ctx, "workspace/executeCommand", arguments, nil); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(folder, modelFile))
	var whole map[string]any
	if err == nil {
		err = json.Unmarshal(content, &whole)
	}
	if err != nil {
		return failure.Wrap(BadModel, modelFile, err)
	}
	for project, fingerprint := range client.toImport {
		if part, modules := modelOf(whole, project); modules > 0 {
			if err := keepModel(client.data, project, fingerprint, part); err != nil {
				return failure.Wrap(BadModel, project, err)
			}
		}
	}
	if !client.anyProjectIn(whole) {
		return failure.Wrap(BadModel, "no project of the model exported", nil)
	}
	return nil
}

// anyProjectIn tells whether some project of the server, imported or read from its model, has
// modules in a model it exported, or the model has none: modules of no project are paths written
// in a way the reader does not know. Projects whose import failed are just not there.
func (client *Client) anyProjectIn(whole map[string]any) bool {
	if len(listOf(whole["modules"])) == 0 {
		return true
	}
	for _, project := range client.Folders() { // a copy: the window changes the folders meanwhile
		if _, modules := modelOf(whole, project); modules > 0 {
			return true
		}
	}
	return false
}

// Models says how the server read its projects: how many from their model, how many it imported,
// and when their models were kept last and why not, "" when they were.
func (client *Client) Models() (fromModel, imported int, keptAt time.Time, why string) {
	client.lock.Lock()
	defer client.lock.Unlock()
	return len(client.standIns), len(client.toImport), client.modelsAt, client.modelsErr
}
