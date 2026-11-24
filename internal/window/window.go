// Package window is the reader's window: projects and their files on the left, a file or its diff on
// the right, a finder over every file, all moved through from the keyboard. It only reads.
package window

import (
	"context"
	"time"

	"github.com/akrck02/code-review/internal/explorer"
	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/navigation"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/servers"
	"github.com/akrck02/code-review/internal/session"
	"github.com/akrck02/code-review/internal/settings"
	"github.com/akrck02/code-review/internal/sidebar"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/code-review/internal/work"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-ui/uidrive"
)

// Settings are what the reader starts with.
type Settings struct {
	Folders     []string // repositories, or folders of repositories, to open
	Base        string   // the ref every project is compared against; empty = each one's default branch
	WorkingTree bool     // count the changes not committed yet
	// Theme names the theme to start in, among Themes; when it is not there, Dark picks Zed's One
	// Dark or One Light.
	Theme string
	Dark  bool
	// Themes are the themes the settings offer; nil offers the reader's own.
	Themes []theme.Theme
	// ThemesFolder is the folder the settings say a person's own themes are read from.
	ThemesFolder string
	// IconTheme names the icon theme files and folders are drawn with, among IconThemes; "" or
	// one not there draws Lucide's icons, tinted by the theme.
	IconTheme string
	// IconThemes are the icon themes the settings offer, besides Lucide's.
	IconThemes []theme.IconTheme
	// IconThemesFolder is the folder the settings say a person's own icon themes are read from.
	IconThemesFolder string
	// Tell hears of problems a person may want to know about while the window runs, such as an
	// icon's picture that cannot be read; nil leaves them untold.
	Tell func(problem error)
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
	// SidebarFolded starts with the sidebar folded away, the file taking the window's width.
	SidebarFolded bool
	// HidesStatus starts with git's status (how files changed) left out of the sidebar, outside
	// the review.
	HidesStatus bool
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
	// session is what is open: the folders, their projects and bases, the documents read
	session      session.Session
	loading      int
	candidates   []finder.Candidate
	changedOrder []workspace.FileReference
	work         *work.Queue

	sidebar sidebar.Sidebar
	viewer  viewer.Viewer
	finder  finder.Finder
	base    baseDialog
	focus   focusedPane
	// sidebarFolded hides the sidebar; focusBefore is where the keyboard was in the last frame, to
	// bring the sidebar back when the keyboard goes to it
	sidebarFolded bool
	focusBefore   focusedPane

	// serversOpen shows the servers panel: what each language server does, project by project
	serversOpen bool
	// serversListed shows the status bar's popover of the servers and their states
	serversListed bool
	// serverDetails are the servers whose card shows its projects and messages, by name
	serverDetails map[string]bool
	pickerOpen    bool
	// themes are the themes the settings offer, and theme the one painting the window
	themes []theme.Theme
	theme  theme.Theme
	// iconThemes are the icon themes the settings offer, and iconTheme the one drawing files and
	// folders, nil for Lucide's; iconPictures keep each one's pictures, by its name
	iconThemes   []theme.IconTheme
	iconTheme    *theme.IconTheme
	iconPictures map[string]*theme.IconPictures
	tell         func(problem error)
	// settingsScreen shows the settings in place of the sidebar and the viewer, when open
	settingsScreen settings.Screen
	// counted is the mode's progress as counted last, and status the status bar's words: kept, as
	// they change far less often than frames are drawn
	counted progressCount
	status  statusWords
	// commandTable is every command, in the order keys are matched: see allCommands
	commandTable []command
	// overlays are the dialogs drawn over the window: see overlaysOf
	overlays []overlay
	// explorer shows a folder in the viewer's place, chosen in the breadcrumb
	explorer explorer.Explorer
	// onlyChanged hides the projects with no changes, in the sidebar and the finder
	onlyChanged bool
	// modes are the ways of looking at the workspace, in the rail's order; mode is the one on
	modes []mode.Mode
	mode  mode.Mode
	// powers are what the mode on can do besides what every mode does (turnOn); usesMode is the
	// mode that shows the uses of a name, if any, found once when the modes start
	powers   modePowers
	usesMode mode.UsesHolder
	look     theme.Look
	// classKinds are what the Java and Kotlin files declare, read as they come into sight;
	// kindsWanted are those asked for since the last frame
	classKinds  map[workspace.FileReference]java.ClassKind
	kindsWanted []workspace.FileReference
	tap         *inputTap
	keys        pane.Keys
	toast       string // a note to show, taken by the next frame
	// shownToast is the note shown, until toastUntil
	shownToast string
	toastUntil time.Time
	// waiting is the server a question asked waits for, still reading the projects, to say so
	// until the answer comes
	waiting *lsp.Client
	// uses makes the lists of uses, checked place by place across projects
	uses      navigation.Uses
	themeSet  bool
	questions questions
	servers   *servers.Fleet
	trail     navigation.Trail
	hover     navigation.HoverCard
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
		session:       session.New(settings.Base, settings.WorkingTree),
		work:          work.New(settings.Synchronous, wake),
		tap:           &inputTap{},
		onlyChanged:   settings.OnlyChanged,
		focus:         sidebarPane,
		sidebarFolded: settings.SidebarFolded,
		classKinds:    map[workspace.FileReference]java.ClassKind{},
	}
	application.servers = servers.New(servers.Settings{Table: settings.Servers, Wrote: settings.ServersWrote, Data: settings.ServerData},
		application, application.work, wake)
	application.overlays = overlaysOf(application)
	application.uses = navigation.NewUses(application, application.work)
	application.sidebar = newSidebar(settings)
	application.startThemes(settings)
	application.startModes(modes, settings.Mode)
	application.viewer = viewer.New()
	if settings.Synchronous {
		application.viewer.ColourPause = 0 // one frame, or a test: colour at once
		application.viewer.HoverPause = 0
	}
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
func (application *Window) Folders() []string { return application.session.Folders }

// startThemes chooses the theme and the icon theme the window starts in, among those offered.
func (application *Window) startThemes(settings Settings) {
	application.themes = settings.Themes
	if len(application.themes) == 0 {
		application.themes = theme.BuiltIn()
	}
	application.theme = theme.Find(application.themes, settings.Theme, settings.Dark)
	application.iconThemes, application.tell = settings.IconThemes, settings.Tell
	application.iconPictures = map[string]*theme.IconPictures{}
	application.iconTheme = findIconTheme(application.iconThemes, settings.IconTheme)
	application.setLook()
}

// setLook paints the window in the theme chosen, its files and folders drawn with the icon theme
// chosen, if any.
func (application *Window) setLook() {
	application.look = application.theme.Look
	if application.iconTheme != nil {
		application.look.Icons, application.look.Pictures = application.iconTheme, application.IconPictures(application.iconTheme)
	}
}

// ThemeName is the name of the theme chosen, to start in it the next time.
func (application *Window) ThemeName() string { return application.theme.Name }

// Closed tells the reader its window has closed, so work still running does not wake it, and
// stops the language servers.
func (application *Window) Closed() {
	windowOpen.Store(false)
	application.servers.Stop()
}

// openFolders adds repositories, or the repositories inside folders, and reads them.
func (application *Window) openFolders(folders []string) {
	if application.viewer.Unsaved() {
		application.keepingEdits(func() { application.openFolders(folders) })
		return
	}
	application.session.AddFolders(folders)
	application.viewer.Close()
	application.reloadAll()
}

// closeAll forgets every project.
func (application *Window) closeAll() {
	if application.viewer.Unsaved() {
		application.keepingEdits(application.closeAll)
		return
	}
	application.session.Clear()
	application.viewer.Close()
	application.projectsChanged()
}

// reloadAll reads every project again: after a change of base, or to see new commits.
func (application *Window) reloadAll() {
	for index := range application.session.Projects {
		application.reload(index)
	}
	application.projectsChanged()
}

// reload reads one project again, on another goroutine.
func (application *Window) reload(index int) {
	path := application.session.Unread(index)
	options := application.LoadOptions(path)
	application.loading++
	settle := func() { application.loading-- } // also when a newer reading replaced this one
	snapshot, written := application.servers.Snapshot(path), application.servers.Written()
	application.work.RunSettling("project "+path, func(stop context.Context) func() {
		var loaded *workspace.Project
		var sorting error
		if !application.work.Read(stop, func() {
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
			application.session.Loaded(loaded)
			application.projectsChanged()
			application.viewer.Reopen(path, application.openFile)
		}
	}, settle)
}

// LoadOptions are how a project is read: against its base, with the changes not committed yet or not.
func (application *Window) LoadOptions(path string) workspace.LoadOptions {
	return application.session.LoadOptions(path)
}

// projectsChanged rebuilds what is made from the projects: the finder's files, the order of the
// changed files and the sidebar's rows.
func (application *Window) projectsChanged() {
	application.servers.ProjectsChanged()
	application.candidates = application.candidates[:0]
	application.changedOrder = application.changedOrder[:0]
	for index, project := range application.session.Projects {
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
	application.explorer.Relist(application)
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
	if reference.Project < 0 || reference.Project >= len(application.session.Projects) {
		return
	}
	if application.viewer.Unsaved() && reference != application.viewer.Reference {
		application.keepingEdits(func() { application.openFile(reference) })
		return
	}
	project := application.session.Projects[reference.Project]
	application.explorer.Close()
	application.viewer.Start(reference, project)
	application.viewer.Uses = application.usesOfFile(reference)
	if document, cached := application.session.Document(reference); cached {
		application.viewer.Show(document, application.mode.ShowsChanges())
		return
	}
	workingTree := application.session.WorkingTree
	request := application.viewer.Request
	application.work.Run("", func(context.Context) func() {
		read := session.ReadDocument(project, reference, workingTree)
		return func() {
			application.session.Keep(read)
			if application.viewer.Request == request {
				application.viewer.Show(read.Document, application.mode.ShowsChanges())
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
		if application.inScope(reference) && !application.sidebar.LeavesOut(application.session.Projects[reference.Project], reference.Path) {
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

// projectIndex is where a project is in the list, by its path; -1 when it is not open.
func (application *Window) projectIndex(path string) int {
	for index, project := range application.session.Projects {
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
	return entered == "" || application.session.Projects[reference.Project].Path == entered
}

// projectsWithChanges counts the projects the development changed.
func (application *Window) projectsWithChanges() int {
	count := 0
	for _, project := range application.session.Projects {
		if len(project.Changes) > 0 {
			count++
		}
	}
	return count
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

// newSidebar is the sidebar as the settings start it: its filter of changes and git's status.
func newSidebar(settings Settings) sidebar.Sidebar {
	made := sidebar.New()
	made.SetChangedFilter(settings.OnlyChanged)
	made.HidesStatus = settings.HidesStatus
	return made
}

// HidesStatus tells whether git's status is left out of the sidebar, to remember it.
func (application *Window) HidesStatus() bool { return application.sidebar.HidesStatus }

// ShowsStatus tells whether git's status is shown now, in the sidebar and the file open: unless
// it is hidden, and always in a mode that lists only the changes.
func (application *Window) ShowsStatus() bool { return application.sidebar.ShowsStatus() }

// OnlyChanged tells whether only the projects with changes are shown, to remember it.
func (application *Window) OnlyChanged() bool { return application.onlyChanged }
