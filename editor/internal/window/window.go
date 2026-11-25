// Package window is the reader's window: projects and their files on the left, a file or its diff on
// the right, a finder over every file, all moved through from the keyboard. It only reads.
package window

import (
	"sync/atomic"
	"time"

	"github.com/akrck02/coddy/editor/internal/finder"
	"github.com/akrck02/coddy/editor/internal/icons"
	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/keymap"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/navigation"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/servers"
	"github.com/akrck02/coddy/editor/internal/session"
	"github.com/akrck02/coddy/editor/internal/settings"
	"github.com/akrck02/coddy/editor/internal/sidebar"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/coddy/editor/internal/viewer"
	"github.com/akrck02/coddy/editor/internal/work"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/failure"
	"github.com/akrck02/go-ui/uidrive"
	platform "github.com/akrck02/go-ui/window"
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
	IconThemes []icons.Theme
	// IconThemesFolder is the folder the settings say a person's own icon themes are read from.
	IconThemesFolder string
	// KeyProfile names the profile of keys to start with, among those in KeymapsFolder; "" or one
	// not there starts with Coddy's own keys.
	KeyProfile string
	// KeymapsFolder is where the profiles of keys are kept; "" keeps them in the reader's own folder.
	KeymapsFolder string
	// ChooseFolder opens the system's folder chooser, as go-ui's window.ChooseFolder does: it
	// calls chosen once, on another goroutine, with the folder chosen or "", and tells whether
	// there is a chooser. nil has none.
	ChooseFolder func(title, start string, chosen func(folder string)) bool
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
	// Servers are the language servers to run, found on the PATH: those of the plugins on.
	Servers []lsp.Server
	// ServersWrote is the file that keeps what servers wrote in the repositories, in the reader's
	// own folder; "" keeps it only in memory.
	ServersWrote string
	// ServerData is the folder the servers keep their indexes in, one folder each; "" is the
	// reader's own folder. kotlin-lsp's take gigabytes: another disk may be better.
	ServerData string
	// DocumentMegabytes is how much memory the files read may keep, kept within the session's
	// least and most in whole steps; 0 keeps the default.
	DocumentMegabytes int
	// Plugins are the plugins installed, on or off; their actions are in the menu.
	Plugins []*plugins.Plugin
	// PluginsFolder is the folder plugins are installed in, which the settings open.
	PluginsFolder string
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
	// made is the file a plugin's action made last, which the menu offers to open
	made made
	// reviewing lists the pull requests to choose from; review is the one chosen
	reviewing reviewDialog
	review    pullRequest

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
	// ring is the servers' mark's ring, kept between frames (serversbutton.go)
	ring markRing
	// serverDetails are the servers whose card shows its projects and messages, by name
	serverDetails map[string]bool
	// themes are the themes the settings offer, and theme the one painting the window
	themes []theme.Theme
	theme  theme.Theme
	// iconThemes are the icon themes the settings offer, and iconTheme the one drawing files and
	// folders, nil for Lucide's; iconPictures keep each one's pictures, by its name
	iconThemes   []icons.Theme
	iconTheme    *icons.Theme
	iconPictures map[string]*icons.Pictures
	// pluginPictures keep the pictures of the plugins' badges
	pluginPictures *icons.Pictures
	tell           func(problem error)
	// settingsScreen shows the settings in place of the sidebar and the viewer, when open
	settingsScreen settings.Screen
	// counted is the mode's progress as counted last, and status the status bar's words: kept, as
	// they change far less often than frames are drawn
	counted progressCount
	status  statusWords
	// commandTable is every command, in the order keys are matched, with the keys of the profile
	// chosen: see allCommands; builtInTable is the same with Coddy's own keys
	commandTable []command
	builtInTable []command
	// keyProfiles are the person's profiles of keys, keyProfile the one chosen ("" for Coddy's)
	keyProfiles []keymap.Profile
	keyProfile  string
	// overlays are the dialogs drawn over the window: see overlaysOf
	overlays []overlay
	// folder is a folder chosen in the sidebar, shown in the file's place
	folder chosenFolder
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
	// restartAsked is set by Restart: the program starts again once the window closes
	restartAsked bool
}

// New makes a reader for these settings, with its modes in the rail's order and after them those
// of the plugins on; Driver must be given
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
	application.session.SetDocumentMegabytes(settings.DocumentMegabytes)
	application.servers = servers.New(servers.Settings{Table: settings.Servers, Wrote: settings.ServersWrote, Data: settings.ServerData},
		application, application.work, wake)
	application.overlays = overlaysOf(application)
	application.uses = navigation.NewUses(application, application.work)
	application.sidebar = newSidebar(settings)
	application.startThemes(settings)
	application.startModes(append(modes, pluginModes(settings.Plugins)...), settings.Mode)
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
	application.iconPictures = map[string]*icons.Pictures{}
	application.pluginPictures = icons.NewPictures(application.tellProblem)
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
// stops the language servers and the plugins.
func (application *Window) Closed() {
	windowOpen.Store(false)
	application.servers.Stop()
	application.stopPlugins()
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

// windowOpen is set once the window has drawn its first frame and cleared when it closes: the
// window can only be woken in between. Work that finishes before the first frame needs no waking,
// as that frame takes its result.
var windowOpen atomic.Bool

// wake asks the window for a frame, when there is a window to ask.
func wake() {
	if !windowOpen.Load() {
		return
	}
	defer func() {
		// TODO(akrck02): go-renderer's Wake panics once GLFW is terminated; fix it there and drop
		// this recover. The window closed between the check and the call: there is nothing to wake.
		_ = recover() // see the TODO above
	}()
	platform.Wake()
}
