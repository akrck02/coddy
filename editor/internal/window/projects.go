package window

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/finder"
	"github.com/akrck02/coddy/editor/internal/plugins"
)

// openFolders adds repositories, or the repositories inside folders, and reads them.
func (application *Window) openFolders(folders []string) {
	if application.viewer.Unsaved() {
		application.keepingEdits(func() { application.openFolders(folders) })
		return
	}
	application.workspace.Marked = application.markedVersionControls()
	application.workspace.GitOff = plugins.GitOff(application.settings.Plugins)
	application.workspace.AddFolders(folders)
	application.viewer.Close()
	application.reloadAll()
}

// closeAll forgets every project.
func (application *Window) closeAll() {
	if application.viewer.Unsaved() {
		application.keepingEdits(application.closeAll)
		return
	}
	application.workspace.Clear()
	application.viewer.Close()
	application.projectsChanged()
}

// reloadAll reads every project again: after a change of base, or to see new commits.
func (application *Window) reloadAll() {
	for index := range application.workspace.Projects {
		application.reload(index)
	}
	application.projectsChanged()
}

// reload reads one project again, on another goroutine.
func (application *Window) reload(index int) {
	path := application.workspace.Unread(index)
	options := application.LoadOptions(path)
	application.loading++
	settle := func() { application.loading-- } // also when a newer reading replaced this one
	snapshot, written := application.servers.Snapshot(path), application.servers.Written()
	application.work.RunSettling("project "+path, func(stop context.Context) func() {
		var loaded *codebase.Project
		var sorting error
		if !application.work.Read(stop, func() {
			loaded = codebase.LoadProject(stop, path, options)
			sorting = written.SortOut(loaded, snapshot, options.WorkingTree)
		}) {
			return nil
		}
		return func() {
			if sorting != nil {
				application.toast = sorting.Error()
			}
			application.forgetClassKinds(path)
			application.workspace.Loaded(loaded)
			application.projectsChanged()
			application.viewer.Reopen(path, application.openFile)
		}
	}, settle)
}

// LoadOptions are how a project is read: against its base, with the changes not committed yet or not.
func (application *Window) LoadOptions(path string) codebase.LoadOptions {
	return application.workspace.LoadOptions(path)
}

// projectsChanged rebuilds what is made from the projects: the finder's files, the order of the
// changed files and the sidebar's rows.
func (application *Window) projectsChanged() {
	application.servers.ProjectsChanged()
	application.candidates = application.candidates[:0]
	application.changedOrder = application.changedOrder[:0]
	for index, project := range application.workspace.Projects {
		if application.hidden(project) {
			continue
		}
		for _, path := range project.Files {
			_, changed := project.ChangeOf(path)
			application.candidates = append(application.candidates, finder.Candidate{
				Project: index, Path: path, Label: project.Name + "/" + path, Changed: changed,
			})
		}
		tree := codebase.BuildTree(project.ChangedPaths(), nil)
		walkFiles(tree, func(path string) {
			application.changedOrder = append(application.changedOrder, codebase.FileReference{Project: index, Path: path})
		})
	}
	application.finder.Stale = true
	application.sidebar.Invalidate()
	application.folder.shown = application.folder.shown && application.folder.project < len(application.workspace.Projects)
}

// projectIndex is where a project is in the list, by its path; -1 when it is not open.
func (application *Window) projectIndex(path string) int {
	for index, project := range application.workspace.Projects {
		if project.Path == path {
			return index
		}
	}
	return -1
}

// inScope tells whether a file is among those n, m and v go through: every project's, or only
// those of the project the sidebar is in.
func (application *Window) inScope(reference codebase.FileReference) bool {
	entered := application.sidebar.Entered
	return entered == "" || application.workspace.Projects[reference.Project].Path == entered
}

// projectsWithChanges counts the projects the development changed.
func (application *Window) projectsWithChanges() int {
	count := 0
	for _, project := range application.workspace.Projects {
		if len(project.Changes) > 0 {
			count++
		}
	}
	return count
}

// hidden tells whether a project is left out of the sidebar and the finder: in the mode that shows
// only the projects with changes, one that was read and has none. A project still being read, or
// that could not be read, stays in sight.
func (application *Window) hidden(project *codebase.Project) bool {
	// a project where only a server wrote stays, to show its files (badge and list)
	return application.onlyChanged && project.Loaded && len(project.Changes) == 0 && len(project.ServerFiles) == 0 && project.Error == nil
}

// setOnlyChanged shows only the projects with changes, or every project.
func (application *Window) setOnlyChanged(only bool) {
	application.onlyChanged = only
	application.sidebar.SetChangedFilter(only)
	application.projectsChanged()
	if only {
		application.toast = "Only the changes: projects and files"
	} else {
		application.toast = "Every project and file"
	}
}
