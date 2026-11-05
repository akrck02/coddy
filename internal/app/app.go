// Package app is the reader's window: projects and their files on the left, a file or its diff on
// the right, a finder over every file, all moved through from the keyboard. It only reads.
package app

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-ui/uidrive"
	"github.com/akrck02/go-ui/window"
)

// Settings are what the reader starts with.
type Settings struct {
	Folders     []string // repositories, or folders of repositories, to open
	Base        string   // the ref every project is compared against; empty = each one's default branch
	WorkingTree bool     // count the changes not committed yet
	Dark        bool
	// Open names a file to open at the start, as "project/path/in/it".
	Open string
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
// work done elsewhere (reading repositories and files) comes back through results.
type App struct {
	settings Settings
	folders  []string

	projects      []*review.Project
	projectBases  map[string]string // a base chosen for one project, by its path
	loading       int
	candidates    []review.Candidate
	changedOrder  []fileReference
	results       chan func()
	documentCache map[fileReference]*review.Document

	sidebar sidebarState
	editor  editorState
	finder  finderState
	base    baseDialog
	focus   pane

	helpOpen   bool
	pickerOpen bool
	dark       bool
	palette    *highlight.Palette
	tap        *inputTap
	keys       keys
	toast      string
	themeSet   bool
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
		results:       make(chan func(), 256),
		documentCache: map[fileReference]*review.Document{},
		tap:           &inputTap{},
		dark:          settings.Dark,
		focus:         sidebarPane,
	}
	application.sidebar = newSidebar()
	application.editor = newEditor()
	application.setPalette()
	application.openFolders(settings.Folders)
	if settings.Open != "" {
		application.openByLabel(settings.Open)
	}
	return application
}

// Driver is what lets the reader see the keys: give it to window.Options.Driver.
func (application *App) Driver() uidrive.Driver { return application.tap }

// Folders are the folders opened, to remember them for the next time.
func (application *App) Folders() []string { return application.folders }

func (application *App) setPalette() {
	if application.dark {
		application.palette = highlight.NewPalette("github-dark")
	} else {
		application.palette = highlight.NewPalette("github")
	}
}

// later runs work on another goroutine and its result back on the interface's, waking the window.
func (application *App) later(work func() func()) {
	if application.settings.Synchronous {
		work()()
		return
	}
	go func() {
		apply := work()
		application.results <- apply
		application.wake()
	}()
}

// windowOpen is set once the window has drawn its first frame and cleared when it closes: the
// window can only be woken in between. Work that finishes before the first frame needs no waking,
// as that frame takes its result.
var windowOpen atomic.Bool

// wake asks the window for a frame, when there is a window to ask.
func (application *App) wake() {
	if !windowOpen.Load() {
		return
	}
	defer func() {
		_ = recover() // the window closed between the check and the call: there is nothing to wake
	}()
	window.Wake()
}

// Closed tells the reader its window has closed, so work still running does not wake it.
func (application *App) Closed() { windowOpen.Store(false) }

// takeResults applies what finished since the last frame.
func (application *App) takeResults() {
	for {
		select {
		case apply := <-application.results:
			apply()
		default:
			return
		}
	}
}

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
		application.projects = append(application.projects, &review.Project{Name: baseName(path), Path: path})
	}
	sort.SliceStable(application.projects, func(first, second int) bool {
		return strings.ToLower(application.projects[first].Name) < strings.ToLower(application.projects[second].Name)
	})
	application.documentCache = map[fileReference]*review.Document{}
	application.editor.close()
	application.reloadAll()
}

// closeAll forgets every project.
func (application *App) closeAll() {
	application.projects = nil
	application.folders = nil
	application.documentCache = map[fileReference]*review.Document{}
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

// readers limits how many repositories are read at once, so a workspace of many projects on a slow
// disk does not start dozens of git processes together.
var readers = make(chan struct{}, 6)

// reload reads one project again, on another goroutine.
func (application *App) reload(index int) {
	project := application.projects[index]
	path := project.Path
	options := review.LoadOptions{Base: application.baseFor(path), WorkingTree: application.settings.WorkingTree}
	application.projects[index] = &review.Project{Name: project.Name, Path: path}
	application.loading++
	application.later(func() func() {
		readers <- struct{}{}
		loaded := review.LoadProject(path, options)
		<-readers
		return func() {
			application.loading--
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
		for _, path := range project.Files {
			_, changed := project.ChangeOf(path)
			application.candidates = append(application.candidates, review.Candidate{
				Project: index, Path: path, Label: project.Name + "/" + path, Changed: changed,
			})
		}
		tree := review.BuildTree(changedPaths(project), nil)
		walkFiles(tree, func(path string) {
			application.changedOrder = append(application.changedOrder, fileReference{index, path})
		})
	}
	application.finder.stale = true
	application.sidebar.invalidate()
}

// walkFiles visits a tree's files in the order the sidebar shows them: folders first.
func walkFiles(folder *review.Folder, visit func(path string)) {
	for _, child := range folder.Folders {
		walkFiles(child, visit)
	}
	for _, path := range folder.Files {
		visit(path)
	}
}

func changedPaths(project *review.Project) []string {
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
		application.editor.show(document)
		return
	}
	workingTree := application.settings.WorkingTree
	request := application.editor.request
	application.later(func() func() {
		document := review.OpenDocument(project, reference.path, workingTree)
		return func() {
			if len(application.documentCache) > 64 {
				application.documentCache = map[fileReference]*review.Document{}
			}
			application.documentCache[reference] = document
			if application.editor.request == request {
				application.editor.show(document)
			}
		}
	})
}

// stepChangedFile opens the next (or previous) changed file of the whole development.
func (application *App) stepChangedFile(step int) {
	order := application.changedOrder
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
	application.documentCache = map[fileReference]*review.Document{}
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
