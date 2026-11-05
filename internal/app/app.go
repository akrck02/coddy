// Package app is the reader's window: projects and their files on the left, a file or its diff on
// the right, a finder over every file, all moved through from the keyboard. It only reads.
package app

import (
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/code-review/internal/workspace"
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
	// Review starts in the review mode (the changed files, as diffs) instead of reading the files.
	Review bool
	// ReviewedPath is the file the files marked as viewed are kept in; empty keeps them only while
	// the reader runs.
	ReviewedPath string
	// Synchronous reads everything before the first frame instead of in the background: for
	// capturing a frame to a picture, where there is only one frame.
	Synchronous bool
}

// pane is the part of the window the keyboard moves in.
type pane int

const (
	sidebarPane pane = iota
	editorPane
)

// App is the reader. Its fields are only touched from the goroutine that builds the interface;
// work done elsewhere (reading repositories and files) runs as jobs of work, whose results come
// back to that goroutine.
type App struct {
	settings Settings
	folders  []string

	projects      []*workspace.Project
	projectBases  map[string]string // a base chosen for one project, by its path
	loading       int
	candidates    []review.Candidate
	changedOrder  []fileReference
	work          *work
	documentCache map[fileReference]*workspace.Document

	sidebar sidebarState
	editor  editorState
	finder  finderState
	base    baseDialog
	focus   pane

	helpOpen   bool
	pickerOpen bool
	dark       bool
	// onlyChanged hides the projects with no changes, in the sidebar and the finder
	onlyChanged bool
	// reviewed keeps the changed files marked as viewed
	reviewed *review.Reviewed
	look     look
	designs  designs
	// classKinds are what the Java and Kotlin files declare, read as they come into sight;
	// kindsWanted are those asked for since the last frame
	classKinds  map[fileReference]java.ClassKind
	kindsWanted []fileReference
	tap         *inputTap
	keys        keys
	toast       string
	themeSet    bool
}

// fileReference names a file of a project.
type fileReference struct {
	project int
	path    string
}

// New makes a reader for these settings; Driver must be given to the window it runs in.
func New(settings Settings) *App {
	application := &App{
		settings:      settings,
		projectBases:  map[string]string{},
		work:          newWork(settings.Synchronous),
		documentCache: map[fileReference]*workspace.Document{},
		tap:           &inputTap{},
		dark:          settings.Dark,
		onlyChanged:   settings.OnlyChanged,
		focus:         sidebarPane,
		designs:       newDesigns(),
		reviewed:      review.OpenReviewed(settings.ReviewedPath),
		classKinds:    map[fileReference]java.ClassKind{},
	}
	application.sidebar = newSidebar()
	if settings.Review {
		application.sidebar.mode = reviewMode
	}
	application.editor = newEditor()
	application.setLook()
	application.openFolders(settings.Folders)
	if settings.Open != "" {
		application.openByLabel(settings.Open)
	}
	if settings.Find != "" {
		application.finder.show(contentScope)
		application.finder.query = settings.Find
	}
	return application
}

// Driver is what lets the reader see the keys: give it to window.Options.Driver.
func (application *App) Driver() uidrive.Driver { return application.tap }

// Folders are the folders opened, to remember them for the next time.
func (application *App) Folders() []string { return application.folders }

// setLook picks the light or the dark look.
func (application *App) setLook() {
	if application.dark {
		application.look = application.designs.dark
	} else {
		application.look = application.designs.light
	}
}

// Closed tells the reader its window has closed, so work still running does not wake it.
func (application *App) Closed() { windowOpen.Store(false) }

// openFolders adds repositories, or the repositories inside folders, and reads them.
func (application *App) openFolders(folders []string) {
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
	application.documentCache = map[fileReference]*workspace.Document{}
	application.editor.close()
	application.reloadAll()
}

// closeAll forgets every project.
func (application *App) closeAll() {
	application.projects = nil
	application.folders = nil
	application.documentCache = map[fileReference]*workspace.Document{}
	application.editor.close()
	application.projectsChanged()
}

// reloadAll reads every project again: after a change of base, or to see new commits.
func (application *App) reloadAll() {
	for index := range application.projects {
		application.reload(index)
	}
	application.projectsChanged()
}

// reload reads one project again, on another goroutine.
func (application *App) reload(index int) {
	project := application.projects[index]
	path := project.Path
	options := workspace.LoadOptions{Base: application.baseFor(path), WorkingTree: application.settings.WorkingTree}
	application.projects[index] = &workspace.Project{Name: project.Name, Path: path}
	application.loading++
	application.work.run("project "+path, func(stop cancellation) func() {
		var loaded *workspace.Project
		if !application.work.read(stop, func() { loaded = workspace.LoadProject(path, options) }) {
			return nil
		}
		return func() {
			application.loading--
			application.forgetClassKinds(path)
			for position, current := range application.projects {
				if current.Path == path {
					application.projects[position] = loaded
				}
			}
			for reference := range application.documentCache {
				if reference.project < len(application.projects) && application.projects[reference.project].Path == path {
					delete(application.documentCache, reference)
				}
			}
			application.projectsChanged()
			application.editor.projectReloaded(application, path)
		}
	})
}

// baseFor is the base a project is compared against: its own, the one for all, or its default.
func (application *App) baseFor(path string) string {
	if base, chosen := application.projectBases[path]; chosen {
		return base
	}
	return application.settings.Base
}

// projectsChanged rebuilds what is made from the projects: the finder's files, the order of the
// changed files and the sidebar's rows.
func (application *App) projectsChanged() {
	application.candidates = application.candidates[:0]
	application.changedOrder = application.changedOrder[:0]
	for index, project := range application.projects {
		if application.hidden(project) {
			continue
		}
		for _, path := range project.Files {
			_, changed := project.ChangeOf(path)
			application.candidates = append(application.candidates, review.Candidate{
				Project: index, Path: path, Label: project.Name + "/" + path, Changed: changed,
			})
		}
		tree := workspace.BuildTree(changedPaths(project), nil)
		walkFiles(tree, func(path string) {
			application.changedOrder = append(application.changedOrder, fileReference{index, path})
		})
	}
	application.finder.stale = true
	application.sidebar.invalidate()
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

func changedPaths(project *workspace.Project) []string {
	paths := make([]string, len(project.Changes))
	for index, change := range project.Changes {
		paths[index] = change.Path
	}
	return paths
}

// openFile opens a file of a project in the editor, reading it on another goroutine.
func (application *App) openFile(reference fileReference) {
	if reference.project < 0 || reference.project >= len(application.projects) {
		return
	}
	project := application.projects[reference.project]
	application.editor.start(reference, project)
	if document, cached := application.documentCache[reference]; cached {
		application.editor.show(document, application.sidebar.mode == reviewMode)
		return
	}
	workingTree := application.settings.WorkingTree
	request := application.editor.request
	application.work.run("", func(cancellation) func() {
		document := workspace.OpenDocument(project, reference.path, workingTree)
		return func() {
			if len(application.documentCache) > 64 {
				application.documentCache = map[fileReference]*workspace.Document{}
			}
			application.documentCache[reference] = document
			if application.editor.request == request {
				application.editor.show(document, application.sidebar.mode == reviewMode)
			}
		}
	})
}

// openFileAt opens a file and shows one of its lines, marked; 0 shows its top (or first change).
func (application *App) openFileAt(reference fileReference, line int) {
	application.editor.goTo = line
	application.openFile(reference)
	editor := &application.editor
	if editor.goTo > 0 && editor.document != nil && editor.reference == reference {
		editor.revealLine(editor.goTo)
	}
}

// stepChangedFile opens the next (or previous) changed file of the whole development.
func (application *App) stepChangedFile(step int) {
	var order []fileReference
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
		if reference == application.editor.reference && application.editor.isOpen() {
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
	application.sidebar.revealFile(application, order[next])
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
func (application *App) projectIndex(path string) int {
	for index, project := range application.projects {
		if project.Path == path {
			return index
		}
	}
	return -1
}

// inScope tells whether a file is among those n, m and v go through: every project's, or only
// those of the project the sidebar is in.
func (application *App) inScope(reference fileReference) bool {
	entered := application.sidebar.entered
	return entered == "" || application.projects[reference.project].Path == entered
}

// projectsWithChanges counts the projects the development changed.
func (application *App) projectsWithChanges() int {
	count := 0
	for _, project := range application.projects {
		if len(project.Changes) > 0 {
			count++
		}
	}
	return count
}

// clearDocuments forgets the files read, so they are read again.
func (application *App) clearDocuments() {
	application.documentCache = map[fileReference]*workspace.Document{}
}

// openByLabel opens a file named as "project/path/in/it", once the projects have been read.
func (application *App) openByLabel(label string) {
	for _, candidate := range application.candidates {
		if candidate.Label == label {
			reference := fileReference{candidate.Project, candidate.Path}
			application.openFile(reference)
			application.sidebar.revealFile(application, reference)
			application.focus = editorPane
			return
		}
	}
	application.toast = "No file " + label + " in the projects open"
}

// hidden tells whether a project is left out of the sidebar and the finder: in the mode that shows
// only the projects with changes, one that was read and has none. A project still being read, or
// that could not be read, stays in sight.
func (application *App) hidden(project *workspace.Project) bool {
	return application.onlyChanged && project.Loaded && len(project.Changes) == 0 && project.Error == ""
}

// setOnlyChanged shows only the projects with changes, or every project.
func (application *App) setOnlyChanged(only bool) {
	application.onlyChanged = only
	application.projectsChanged()
	if only {
		application.toast = "Only the projects with changes"
	} else {
		application.toast = "Every project"
	}
}

// OnlyChanged tells whether only the projects with changes are shown, to remember it.
func (application *App) OnlyChanged() bool { return application.onlyChanged }
