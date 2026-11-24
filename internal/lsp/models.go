package lsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/akrck02/code-review/internal/gradle"
	"github.com/akrck02/go-renderer/failure"
)

// A server that imports Gradle builds (kotlin-lsp) imports every project on every start, minutes
// for many projects, even with its index built. It can be given, instead, the model of a project
// it imported before, kept by the gradle package: the server starts with a stand-in folder holding
// the model for each project whose build files did not change, and the models of those it imports
// are kept once it is done. On the big workspace: ready in 13 s instead of 452 s, with no Gradle
// daemon.
// modelWait is how long a server is given to write the model it imported.
const modelWait = 2 * time.Minute

// chooseModels sees, before the server starts, which projects have a model kept for the build
// files they have now: those it reads from their model, through a stand-in folder; it imports the
// others with Gradle, and their models are kept once it is done (keepModels).
func (client *Client) chooseModels(projects []string) {
	client.standIns, client.toImport = map[string]string{}, map[string]string{}
	for _, project := range projects {
		fingerprint := gradle.Fingerprint(project)
		switch {
		case gradle.HasModel(client.data, project, fingerprint):
			client.standIns[project] = gradle.ModelFolder(client.data, project)
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
		tools[FileURI(standIn)] = gradle.ModelTool
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
	folder := gradle.ExportFolder(client.data)
	_ = os.RemoveAll(folder) // what is left is written over; MkdirAll says if the folder cannot be made
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return failure.Wrap(BadModel, folder, err)
	}
	defer os.RemoveAll(folder)
	ctx, cancel := context.WithTimeout(context.Background(), modelWait)
	defer cancel()
	arguments := map[string]any{"command": gradle.ExportCommand, "arguments": []string{folder}}
	if err := client.conn.Call(ctx, "workspace/executeCommand", arguments, nil); err != nil {
		return err
	}
	content, err := os.ReadFile(filepath.Join(folder, gradle.ModelFile))
	var whole map[string]any
	if err == nil {
		err = json.Unmarshal(content, &whole)
	}
	if err != nil {
		return failure.Wrap(BadModel, gradle.ModelFile, err)
	}
	for project, fingerprint := range client.toImport {
		if part, modules := gradle.ModelOf(whole, project); modules > 0 {
			if err := gradle.KeepModel(client.data, project, fingerprint, part); err != nil {
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
	if gradle.Modules(whole) == 0 {
		return true
	}
	for _, project := range client.Folders() { // a copy: the window changes the folders meanwhile
		if _, modules := gradle.ModelOf(whole, project); modules > 0 {
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
