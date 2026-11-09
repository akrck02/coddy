// Package window is the reader's window: projects and their files on the left, a file or its diff on
// the right, a finder over every file, all moved through from the keyboard. It only reads.
package window

import (
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-ui/uidrive"
)

// Settings are what the reader starts with.
type Settings struct {
	Folders     []string // repositories, or folders of repositories, to open
	Base        string   // the ref every project is compared against; empty = each one's default branch
	WorkingTree bool     // count the changes not committed yet
	Dark        bool
	// OnlyChanged hides the projects the development did not change, in the sidebar and the finder.
	OnlyChanged bool
	// Open names a file to open at the start, as "project/path/in/it".
	Open string
	// Find opens the finder at the start, searching the files' content for this text.
	Find string
	// Mode names the mode to start in; empty starts in the first.
	Mode string
	// Synchronous reads everything before the first frame instead of in the background: for
	// capturing a frame to a picture, where there is only one frame.
	Synchronous bool
	// ReadOnly stops editing: files are only read, as before editing came.
	ReadOnly bool
	// Servers are the language servers to run; nil runs lsp.Servers, found on the PATH.
	Servers []lsp.Server
	// ServersWrote is the file that keeps what servers wrote in the repositories, in the reader's
	// own folder; "" keeps it only in memory.
	ServersWrote string
	// ServerData is the folder the servers keep their indexes in, one folder each; "" is the
	// reader's own folder. kotlin-lsp's take gigabytes: another disk may be better.
	ServerData string
}

// focusedPane is the part of the window the keyboard moves in.
type focusedPane int

const (
	sidebarPane focusedPane = iota
	viewerPane
)

// Window is the reader. Its fields are only touched from the goroutine that builds the interface;
// work done elsewhere (reading repositories and files) runs as jobs of work, whose results come
// back to that goroutine.
type Window struct {
	settings Settings
	folders  []string

	projects      []*workspace.Project
	projectBases  map[string]string // a base chosen for one project, by its path
	loading       int
	candidates    []finder.Candidate
	changedOrder  []workspace.FileReference
	work          *work
	documentCache map[workspace.FileReference]*workspace.Document

	sidebar sidebar.Sidebar
	viewer  viewer.Viewer
	finder  finder.Finder
	base    baseDialog
	focus   focusedPane

	helpOpen bool
	// serversOpen shows the servers panel: what each language server does, project by project
	serversOpen bool
	pickerOpen  bool
	dark        bool
	// onlyChanged hides the projects with no changes, in the sidebar and the finder
	onlyChanged bool
	// modes are the ways of looking at the workspace, in the rail's order; mode is the one on
	modes   []mode.Mode
	mode    mode.Mode
	look    theme.Look
	designs theme.Designs
	// classKinds are what the Java and Kotlin files declare, read as they come into sight;
	// kindsWanted are those asked for since the last frame
	classKinds  map[workspace.FileReference]java.ClassKind
	kindsWanted []workspace.FileReference
	tap         *inputTap
	keys        pane.Keys
	toast       string
	// waiting is the server a question asked waits for, still reading the projects, to say so
	// until the answer comes
	waiting *lsp.Client
	// uses is the list of uses being made, checked place by place across projects
	uses      *usesSearch
	themeSet  bool
	questions questions
	servers   languageServers
	trail     trail
	hover     hoverCard
	// pointer is where the pointer was in the last frame, for a card shown under it
	pointerX, pointerY float32
}

// New makes a reader for these settings, with its modes in the rail's order; Driver must be given
// to the window it runs in. It needs one mode at least (NoMode).
func New(settings Settings, modes ...mode.Mode) (*Window, error) {
	if len(modes) == 0 {
		return nil, failure.Wrap(NoMode, "main registers the modes", nil)
	}
	application := &Window{
		settings:      settings,
		projectBases:  map[string]string{},
		work:          newWork(settings.Synchronous),
		documentCache: map[workspace.FileReference]*workspace.Document{},
		tap:           &inputTap{},
		dark:          settings.Dark,
		onlyChanged:   settings.OnlyChanged,
		focus:         sidebarPane,
		designs:       theme.NewDesigns(),
		servers:       newLanguageServers(settings.Servers, settings.ServersWrote),
		classKinds:    map[workspace.FileReference]java.ClassKind{},
	}
	application.sidebar = sidebar.New()
	application.sidebar.SetChangedFilter(settings.OnlyChanged)
	application.startModes(modes, settings.Mode)
	application.viewer = viewer.New()
	if settings.Synchronous {
		application.viewer.ColourPause = 0 // one frame, or a test: colour at once
		application.viewer.HoverPause = 0
	}
	application.setLook()
	application.openFolders(settings.Folders)
	if settings.Open != "" {
		application.openByLabel(settings.Open)
	}
	if settings.Find != "" {
		application.finder.Show(finder.ContentScope)
		application.finder.Query = settings.Find
	}
	return application, nil
}

// Driver is what lets the reader see the keys: give it to window.Options.Driver.
func (application *Window) Driver() uidrive.Driver { return application.tap }

// Folders are the folders opened, to remember them for the next time.
func (application *Window) Folders() []string { return application.folders }

// setLook picks the light or the dark look.
func (application *Window) setLook() {
	if application.dark {
		application.look = application.designs.Dark
	} else {
		application.look = application.designs.Light
	}
}

// Closed tells the reader its window has closed, so work still running does not wake it, and
// stops the language servers.
func (application *Window) Closed() {
	windowOpen.Store(false)
	application.stopServers()
}

// openFolders adds repositories, or the repositories inside folders, and reads them.
func (application *Window) openFolders(folders []string) {
	if application.viewer.Unsaved() {
		application.keepingEdits(func() { application.openFolders(folders) })
		return
	}
	known := map[string]bool{}
	for _, project := range application.projects {
		known[project.Path] = true
	}
	for _, folder := range folders {
		if !contains(application.folders, folder) {
			application.folders = append(application.folders, folder)
		}
	}
	for _, path := range git.Discover(folders) {
		if known[path] {
			continue
		}
		application.projects = append(application.projects, &workspace.Project{Name: baseName(path), Path: path})
	}
	sort.SliceStable(application.projects, func(first, second int) bool {
		return strings.ToLower(application.projects[first].Name) < strings.ToLower(application.projects[second].Name)
	})
	application.documentCache = map[workspace.FileReference]*workspace.Document{}
	application.viewer.Close()
	application.reloadAll()
}

// closeAll forgets every project.
func (application *Window) closeAll() {
	if application.viewer.Unsaved() {
		application.keepingEdits(application.closeAll)
		return
	}
	application.projects = nil
	application.folders = nil
	application.documentCache = map[workspace.FileReference]*workspace.Document{}
	application.viewer.Close()
	application.projectsChanged()
}

// reloadAll reads every project again: after a change of base, or to see new commits.
func (application *Window) reloadAll() {
	for index := range application.projects {
		application.reload(index)
	}
	application.projectsChanged()
}

// reload reads one project again, on another goroutine.
func (application *Window) reload(index int) {
	project := application.projects[index]
	path := project.Path
	options := application.loadOptions(path)
	application.projects[index] = &workspace.Project{Name: project.Name, Path: path}
	application.loading++
	settle := func() { application.loading-- } // also when a newer reading replaced this one
	snapshot, written := application.servers.snapshots[path], application.servers.written
	application.work.runSettling("project "+path, func(stop cancellation) func() {
		var loaded *workspace.Project
		var sorting error
		if !application.work.read(stop, func() {
			loaded = workspace.LoadProject(path, options)
			sorting = written.SortOut(loaded, snapshot, options.WorkingTree)
		}) {
			return nil
		}
		return func() {
			if sorting != nil {
				application.toast = sorting.Error()
			}
			application.forgetClassKinds(path)
			for position, current := range application.projects {
				if current.Path == path {
					application.projects[position] = loaded
				}
			}
			for reference := range application.documentCache {
				if reference.Project < len(application.projects) && application.projects[reference.Project].Path == path {
					delete(application.documentCache, reference)
				}
			}
			application.projectsChanged()
			application.viewer.Reopen(path, application.openFile)
		}
	}, settle)
}

// loadOptions are how a project is read: against its base, with the changes not committed yet or not.
func (application *Window) loadOptions(path string) workspace.LoadOptions {
	return workspace.LoadOptions{Base: application.baseFor(path), WorkingTree: application.settings.WorkingTree}
}

// baseFor is the base a project is compared against: its own, the one for all, or its default.
func (application *Window) baseFor(path string) string {
	if base, chosen := application.projectBases[path]; chosen {
		return base
	}
	return application.settings.Base
}

// projectsChanged rebuilds what is made from the projects: the finder's files, the order of the
// changed files and the sidebar's rows.
func (application *Window) projectsChanged() {
	application.servers.roots = map[string]string{} // a go.mod may have come or gone
	application.servers.inSight = map[string][]string{}
	application.candidates = application.candidates[:0]
	application.changedOrder = application.changedOrder[:0]
	for index, project := range application.projects {
		if application.hidden(project) {
			continue
		}
		for _, path := range project.Files {
			_, changed := project.ChangeOf(path)
			application.candidates = append(application.candidates, finder.Candidate{
				Project: index, Path: path, Label: project.Name + "/" + path, Changed: changed,
			})
		}
		tree := workspace.BuildTree(project.ChangedPaths(), nil)
		walkFiles(tree, func(path string) {
			application.changedOrder = append(application.changedOrder, workspace.FileReference{Project: index, Path: path})
		})
	}
	application.finder.Stale = true
	application.sidebar.Invalidate()
}

// walkFiles visits a tree's files in the order the sidebar shows them: folders first.
func walkFiles(folder *workspace.Folder, visit func(path string)) {
	for _, child := range folder.Folders {
		walkFiles(child, visit)
	}
	for _, path := range folder.Files {
		visit(path)
	}
}

// openFile opens a file of a project in the viewer, reading it on another goroutine. Another
// file asks first about the edits not saved of the one open.
func (application *Window) openFile(reference workspace.FileReference) {
	if reference.Project < 0 || reference.Project >= len(application.projects) {
		return
	}
	if application.viewer.Unsaved() && reference != application.viewer.Reference {
		application.keepingEdits(func() { application.openFile(reference) })
		return
	}
	project := application.projects[reference.Project]
	application.viewer.Start(reference, project)
	if document, cached := application.documentCache[reference]; cached {
		application.viewer.Show(document, application.mode.ShowsChanges())
		return
	}
	workingTree := application.settings.WorkingTree
	request := application.viewer.Request
	application.work.run("", func(cancellation) func() {
		document := workspace.OpenDocument(project, reference.Path, workingTree)
		return func() {
			if len(application.documentCache) > 64 {
				application.documentCache = map[workspace.FileReference]*workspace.Document{}
			}
			application.documentCache[reference] = document
			if application.viewer.Request == request {
				application.viewer.Show(document, application.mode.ShowsChanges())
			}
		}
	})
}

// openFileAt opens a file and shows one of its lines, marked; 0 shows its top (or first change).
func (application *Window) openFileAt(reference workspace.FileReference, line int) {
	application.viewer.GoTo = line
	application.openFile(reference)
	view := &application.viewer
	if view.GoTo > 0 && view.Document != nil && view.Reference == reference {
		view.RevealLine(view.GoTo)
	}
}

// stepChangedFile opens the next (or previous) changed file of the whole development.
func (application *Window) stepChangedFile(step int) {
	var order []workspace.FileReference
	for _, reference := range application.changedOrder {
		if application.inScope(reference) {
			order = append(order, reference)
		}
	}
	if len(order) == 0 {
		return
	}
	current := -1
	for index, reference := range order {
		if reference == application.viewer.Reference && application.viewer.IsOpen() {
			current = index
			break
		}
	}
	next := current + step
	if current < 0 && step < 0 {
		next = len(order) - 1
	}
	next = (next + len(order)) % len(order)
	application.openFile(order[next])
	application.sidebar.RevealFile(application, order[next])
}

func contains(list []string, wanted string) bool {
	for _, item := range list {
		if item == wanted {
			return true
		}
	}
	return false
}

func baseName(path string) string {
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndexByte(path, '/')+1:]
}

// projectIndex is where a project is in the list, by its path; -1 when it is not open.
func (application *Window) projectIndex(path string) int {
	for index, project := range application.projects {
		if project.Path == path {
			return index
		}
	}
	return -1
}

// inScope tells whether a file is among those n, m and v go through: every project's, or only
// those of the project the sidebar is in.
func (application *Window) inScope(reference workspace.FileReference) bool {
	entered := application.sidebar.Entered
	return entered == "" || application.projects[reference.Project].Path == entered
}

// projectsWithChanges counts the projects the development changed.
func (application *Window) projectsWithChanges() int {
	count := 0
	for _, project := range application.projects {
		if len(project.Changes) > 0 {
			count++
		}
	}
	return count
}

// clearDocuments forgets the files read, so they are read again.
func (application *Window) clearDocuments() {
	application.documentCache = map[workspace.FileReference]*workspace.Document{}
}

// openByLabel opens a file named as "project/path/in/it", once the projects have been read.
func (application *Window) openByLabel(label string) {
	for _, candidate := range application.candidates {
		if candidate.Label == label {
			reference := workspace.FileReference{Project: candidate.Project, Path: candidate.Path}
			application.openFile(reference)
			application.sidebar.RevealFile(application, reference)
			application.focus = viewerPane
			return
		}
	}
	application.toast = "No file " + label + " in the projects open"
}

// hidden tells whether a project is left out of the sidebar and the finder: in the mode that shows
// only the projects with changes, one that was read and has none. A project still being read, or
// that could not be read, stays in sight.
func (application *Window) hidden(project *workspace.Project) bool {
	// a project where only a server wrote stays, to show its files (badge and list)
	return application.onlyChanged && project.Loaded && len(project.Changes) == 0 && len(project.ServerFiles) == 0 && project.Error == ""
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

// OnlyChanged tells whether only the projects with changes are shown, to remember it.
func (application *Window) OnlyChanged() bool { return application.onlyChanged }
