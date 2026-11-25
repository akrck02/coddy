package window

import (
	"fmt"
	"github.com/akrck02/coddy/editor/internal/finder"
	"strings"
	"time"

	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/session"
	"github.com/akrck02/coddy/editor/internal/viewer"
	ui "github.com/akrck02/go-ui"
)

// Build describes the whole window for a frame; give it to window.Run.
func (application *Window) Build(context *ui.Context) {
	if !application.themeSet {
		application.applyTheme(context)
		application.themeSet = true
	}
	application.keys = pane.KeysOf(application.tap.state)
	windowOpen.Store(true)
	application.work.Take()
	application.readClassKinds()
	application.typeInTheFile(context, application.handleKeys(context))
	application.unfoldForTheKeyboard()
	application.servers.Serve(application.servedFile())
	application.uses.Advance()

	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Background}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			application.buildPanes(context)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		application.buildStatusBar(context)
	})

	application.buildOverlays(context)
	application.buildNotes(context)
	application.pointerX, application.pointerY = context.Pointer()
	if application.showToast(context, time.Now()) {
		return
	}
	if waiting := application.waiting; waiting != nil && !waiting.Ready() {
		context.Toast(waitingWords(waiting, time.Now()), false)
	}
}

// buildPanes lays the mode rail, the sidebar (unless it is folded away) and the viewer side by side,
// the rail and the sidebar from the top of the window, the title bar over the viewer alone.
func (application *Window) buildPanes(context *ui.Context) {
	application.buildModeRail(context)
	context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
	if application.settingsScreen.Open {
		application.settingsScreen.Build(context, application)
		return
	}
	if !application.sidebarFolded {
		application.buildSidebar(context)
	}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, func() {
		application.buildTitleBar(context)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		cover := application.powers.cover
		switch {
		case cover != nil && cover.BuildOverViewer(context):
		case application.folder.shown:
			application.folder.build(context)
		default:
			application.viewer.Build(context, application)
		}
	})
}

// modeRailWidth is the mode rail's width.
const modeRailWidth = 48

// viewerWidth is how wide the side right of the rail and the sidebar is, in a window of a width:
// the title bar's.
func (application *Window) viewerWidth(windowWidth float32) float32 {
	width := windowWidth - modeRailWidth - 1
	if !application.sidebarFolded {
		width -= application.sidebar.Width + 1
	}
	return max(width, 0)
}

// buildSidebar is the sidebar, as wide as it was left, with the line after it.
func (application *Window) buildSidebar(context *ui.Context) {
	// the sidebar never takes more than two fifths of the window, so the code keeps its room
	// when the window is made narrow
	windowWidth, _ := context.Size()
	widest := min(max(windowWidth*0.4, 220), 720)
	application.sidebar.Width = min(application.sidebar.Width, widest)
	context.Resizable("sidebar", ui.RightSide, &application.sidebar.Width, 220, widest,
		ui.Layout{Direction: ui.Vertical, Height: ui.Grow()}, func() {
			application.sidebar.Build(context, application)
		})
	context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
}

// applyTheme sets the window's look and gives its theme to the interface.
func (application *Window) applyTheme(context *ui.Context) {
	application.setLook()
	context.Theme = application.look.Theme
}

// enterCurrentProject takes the sidebar into the project of the selected row, or of the file open.
func (application *Window) enterCurrentProject() {
	if row, ok := application.sidebar.SelectedRow(); ok && row.Project >= 0 && application.focus == sidebarPane {
		application.sidebar.Enter(application, row.Project)
		return
	}
	if application.viewer.IsOpen() {
		application.sidebar.Enter(application, application.viewer.Reference.Project)
	}
}

// findInProject opens the finder on the content of the project you are in: the one entered, the
// one of the row chosen in the sidebar, or the one of the file open; with none, every project.
func (application *Window) findInProject() {
	project := application.currentProject()
	if project < 0 {
		application.finder.Show(finder.ContentScope)
		return
	}
	application.finder.ShowIn(finder.ContentScope, project, application.session.Projects[project].Name)
}

// currentProject is the place of the project you are in, -1 for none.
func (application *Window) currentProject() int {
	if application.sidebar.Entered != "" {
		return application.projectIndex(application.sidebar.Entered)
	}
	if row, found := application.sidebar.SelectedRow(); found && row.Project >= 0 && application.focus == sidebarPane {
		return row.Project
	}
	if application.viewer.IsOpen() {
		return application.viewer.Reference.Project
	}
	return -1
}

func workingTreeNote(on bool) string {
	if on {
		return "Counting the changes not committed yet"
	}
	return "Comparing the last commit only"
}

func (application *Window) toggleFocus() {
	if application.focus == sidebarPane && application.viewer.IsOpen() {
		application.focus = viewerPane
		return
	}
	application.focus = sidebarPane
}

func (application *Window) toggleView() {
	view := &application.viewer
	switch {
	case view.Mode != viewer.FileView:
		view.SetMode(viewer.FileView)
	case len(view.Uses) > 0:
		view.SetMode(viewer.UsesView)
	default:
		view.SetMode(viewer.DiffView)
	}
}

// copyPath puts the open file's path, from the project's top, on the clipboard.
func (application *Window) copyPath(context *ui.Context) {
	if !application.viewer.IsOpen() {
		return
	}
	path := application.viewer.Project.Name + "/" + application.viewer.Reference.Path
	context.Copy(path)
	application.toast = "Copied " + path
}

// buildModeRail is the strip on the far left that chooses what the reader is doing, such as reading
// the files or reviewing the changes: one place per mode, always in sight, one key each. The
// settings are no mode: while they show, none is marked, and choosing one leaves them.
func (application *Window) buildModeRail(context *ui.Context) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(modeRailWidth), Height: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 6, AlignX: ui.Center}, func() {
		for _, place := range application.modes {
			chosen := application.mode == place && !application.settingsScreen.Open
			ink, normal := palette.InkQuiet, ui.Look{Radius: 8}
			if chosen {
				ink, normal = palette.Ink, ui.Look{Fill: application.look.Chosen, Radius: 8}
			}
			hovered := normal
			if !chosen {
				hovered.Fill = application.look.Hover
			}
			layout := ui.Layout{Width: ui.Fixed(36), Height: ui.Fixed(36), AlignX: ui.Center, AlignY: ui.Center}
			if context.Clickable(place.Description(), layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
				context.IconSized(place.Icon(), 20, ink)
			}) {
				application.settingsScreen.Open = false
				application.setMode(place)
			}
			context.Tooltip(place.Description())
		}
		context.Spacer()
		application.buildFoldButton(context)
	})
}

// menuName is what the menu's button, an icon alone, is called by a screen reader
const menuName = "More"

// buildMenu is the menu of what is done less often: opening a folder, reading the projects again,
// git's status, closing.
func (application *Window) buildMenu(context *ui.Context) {
	context.MenuButton("ellipsis", "##"+menuName, func() {
		if context.MenuItem("folder-open", "Open folder…", "⌘O") {
			application.OpenFolder()
		}
		if context.MenuItem("refresh-cw", "Read the projects again", "r") {
			application.reloadAll()
		}
		status, statusIcon := "Hide git status", "eye-off"
		if application.sidebar.HidesStatus {
			status, statusIcon = "Show git status", "eye"
		}
		if context.MenuItem(statusIcon, status, "") {
			application.sidebar.HidesStatus = !application.sidebar.HidesStatus
		}
		application.buildPluginItems(context)
		application.buildReviewItems(context)
		context.Separator()
		if context.MenuItem("x", "Close all projects", "") {
			application.closeAll()
		}
	})
}

func (application *Window) title() string {
	switch len(application.session.Folders) {
	case 0:
		return "Coddy"
	case 1:
		return session.BaseName(application.session.Folders[0])
	}
	return fmt.Sprintf("%s and %d more", session.BaseName(application.session.Folders[0]), len(application.session.Folders)-1)
}

// showToast shows the last note for a few seconds, longer for a longer one, and draws again when it
// is due to go: shown for one frame, a note went with the next move of the mouse, too fast to read.
func (application *Window) showToast(context *ui.Context, now time.Time) bool {
	if application.toast != "" {
		application.shownToast, application.toastUntil = application.toast, now.Add(toastTime(application.toast))
		application.toast = ""
	}
	if application.shownToast == "" || !now.Before(application.toastUntil) {
		application.shownToast = ""
		return false
	}
	context.Toast(application.shownToast, false)
	context.RequestFrame(application.toastUntil.Sub(now))
	return true
}

// toastTime is how long a note shows: three seconds, and a little more for each word, up to eight.
func toastTime(note string) time.Duration {
	return min(3*time.Second+time.Duration(len(strings.Fields(note)))*250*time.Millisecond, 8*time.Second)
}
