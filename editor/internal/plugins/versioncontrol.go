package plugins

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/sdk"
)

// readSeconds is how long a plugin may take to answer what Coddy reads through it: listing the
// files of a large repository, or the pull requests of a remote, takes a while.
const readSeconds = 60

// VersionControlContribution is a version control a plugin gives, and the file or folder that
// marks its repositories (".hg").
type VersionControlContribution struct {
	Name   string `json:"name"`
	Marker string `json:"marker"`
}

// VersionControl reads repositories through the plugin, which must contribute a version control.
// Its methods wait for the plugin, so they run off the frame, as git's commands do.
func (plugin *Plugin) VersionControl() codebase.VersionControl { return pluginControl{plugin} }

type pluginControl struct{ plugin *Plugin }

func (control pluginControl) Branch(folder string) (string, error) {
	var branch string
	err := control.ask(context.Background(), sdk.BranchMethod, sdk.FolderParams{Folder: folder}, &branch)
	return branch, err
}

func (control pluginControl) DefaultBase(folder string) (string, error) {
	var base string
	err := control.ask(context.Background(), sdk.DefaultBaseMethod, sdk.FolderParams{Folder: folder}, &base)
	return base, err
}

func (control pluginControl) Files(stop context.Context, folder string) ([]string, error) {
	var files []string
	err := control.ask(stop, sdk.FilesMethod, sdk.FolderParams{Folder: folder}, &files)
	return files, err
}

func (control pluginControl) Changes(stop context.Context, folder string, comparison git.Comparison) ([]git.Change, error) {
	var answered []sdk.Change
	err := control.ask(stop, sdk.ChangesMethod, compareParams(folder, comparison), &answered)
	changes := make([]git.Change, 0, len(answered))
	for _, change := range answered {
		changes = append(changes, changeOf(change))
	}
	return changes, err
}

func (control pluginControl) ReadAt(folder, revision, path string) ([]byte, error) {
	var text string
	asked := sdk.ReadParams{Folder: folder, Revision: revision, Path: path}
	err := control.ask(context.Background(), sdk.ReadMethod, asked, &text)
	return []byte(text), err
}

func (control pluginControl) Diff(folder string, comparison git.Comparison, change git.Change) (git.FileDiff, error) {
	var unified string
	asked := sdk.DiffParams{CompareParams: compareParams(folder, comparison), Path: change.Path, OldPath: change.OldPath}
	if err := control.ask(context.Background(), sdk.DiffMethod, asked, &unified); err != nil {
		return git.FileDiff{}, err
	}
	return git.ParseUnifiedDiff([]byte(unified))
}

// ask calls a method of the plugin as a version control.
// TODO(akrck02): a version control's failure shows on its project, not yet in the plugin's state
// in the settings, which Tell keeps on the frame goroutine.
func (control pluginControl) ask(stop context.Context, method string, params, result any) error {
	return control.plugin.ask(stop, method, params, result)
}

func compareParams(folder string, comparison git.Comparison) sdk.CompareParams {
	return sdk.CompareParams{Folder: folder, Base: comparison.MergeBase, WorkingTree: comparison.WorkingTree}
}

// changeOf is a change as the plugin tells it, as git's are kept: a file not renamed has its own
// path as its old one.
func changeOf(change sdk.Change) git.Change {
	converted := git.Change{Status: git.Modified, Path: change.Path, OldPath: change.OldPath, Untracked: change.Untracked}
	if change.Status != "" {
		converted.Status = git.StatusOf(change.Status[0])
	}
	if converted.OldPath == "" {
		converted.OldPath = change.Path
	}
	return converted
}
