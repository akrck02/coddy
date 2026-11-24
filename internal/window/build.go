package window

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/session"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// baseDialog chooses what the projects are compared against.
type baseDialog struct {
	open       bool
	focusNext  bool
	ref        string
	forAll     int // 0: the project of the file open (or selected), 1: every project
	forProject string
}

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
		cover, covers := application.mode.(mode.Cover)
		switch {
		case covers && cover.BuildOverViewer(context):
		case application.explorer.Open:
			application.explorer.Build(context, application)
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
		if context.MenuItem("folder-open", "Open folder…", "o") {
			application.pickerOpen = true
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
		context.Separator()
		if context.MenuItem("x", "Close all projects", "") {
			application.closeAll()
		}
	})
}

func (application *Window) title() string {
	switch len(application.session.Folders) {
	case 0:
		return "Code review"
	case 1:
		return session.BaseName(application.session.Folders[0])
	}
	return fmt.Sprintf("%s and %d more", session.BaseName(application.session.Folders[0]), len(application.session.Folders)-1)
}

// buildStatusBar says where the development stands: its projects and changed files, how far a
// mode's work has gone, and where the open file's branch stands against its base.
func (application *Window) buildStatusBar(context *ui.Context) {
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(14, 0), Gap: 16, AlignY: ui.Center}, func() {
		application.buildCounts(context, small)
		if application.loading > 0 {
			context.Text(application.status.loading.Of(application.loading, readingWords), small)
		}
		context.Spacer()
		if application.viewer.IsOpen() {
			application.buildBranch(context, small)
		}
		if !application.session.WorkingTree {
			context.Text("commits only", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Warning})
		}
		application.buildServersButton(context)
		if application.settings.ReadOnly {
			context.Text("read-only", small)
		}
	})
}

// buildCounts says, each with its icon, how many files changed, in how many of the projects, and
// how far the mode's work has gone; their tooltips say it in words.
func (application *Window) buildCounts(context *ui.Context, small ui.TextStyle) {
	counted := func(icon, text, words string, more func()) {
		// a place the pointer can rest on, which a tooltip needs: one after a plain box would
		// name the control before it, the menu's button
		context.Clickable(words, ui.Layout{Gap: 5, AlignY: ui.Center}, ui.Looks{}, func() {
			context.IconSized(icon, 13, context.Theme.InkQuiet)
			if more != nil {
				more()
			}
			context.Text(text, small)
		})
		context.Tooltip(words)
	}
	status := &application.status
	files, changed, projects := len(application.changedOrder), application.projectsWithChanges(), len(application.session.Projects)
	counted("file-diff", status.files.Of(files, strconv.Itoa), status.filesWords.Of(files, changedFilesWords), nil)
	counted("folder-git-2", status.projects.Of([2]int{changed, projects}, fraction),
		status.projectsWords.Of([2]int{changed, projects}, projectsWords), nil)
	if viewed, total, words := application.progress(); total > 0 {
		counted("eye", status.progress.Of([2]int{viewed, total}, fraction), words, func() { application.buildProgress(context, viewed, total) })
	}
}

// buildProgress is a small bar of how far the mode's work has gone, green once it is done.
func (application *Window) buildProgress(context *ui.Context, viewed, total int) {
	palette := context.Theme
	colour := palette.InkQuiet
	if viewed == total {
		colour = palette.Positive
	}
	context.Paint(ui.Layout{Width: ui.Fixed(56), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, application.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if viewed > 0 {
			done := area
			done.Width = area.Width * float32(viewed) / float32(total)
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
}

// buildBranch says where the open file's branch stands against its base, as the project is now.
func (application *Window) buildBranch(context *ui.Context, small ui.TextStyle) {
	project := application.viewer.Project
	for _, current := range application.session.Projects {
		if current.Path == project.Path {
			project = current
		}
	}
	if project.Base == "" {
		return
	}
	branch := branchState{project.Branch, project.Base, project.Ahead}
	context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
		context.IconSized("git-branch", 12, context.Theme.InkQuiet)
		context.Text(application.status.branch.Of(branch, branchWords), small)
	})
	context.Tooltip(application.status.branchWords.Of(branch, aheadWords))
}

// statusWords are the status bar's words, each kept until what it says changes.
type statusWords struct {
	loading, files, filesWords pane.Words[int]
	projects, projectsWords    pane.Words[[2]int]
	progress                   pane.Words[[2]int]
	branch, branchWords        pane.Words[branchState]
}

// branchState is where a project's branch stands: its name, its base and how far ahead it is.
type branchState struct {
	branch, base string
	ahead        int
}

func readingWords(count int) string        { return fmt.Sprintf("reading %d…", count) }
func changedFilesWords(count int) string   { return plural(count, "file") + " changed" }
func fraction(parts [2]int) string         { return fmt.Sprintf("%d/%d", parts[0], parts[1]) }
func branchWords(state branchState) string { return state.branch + " ← " + state.base }

func projectsWords(parts [2]int) string {
	return fmt.Sprintf("%d of %d projects have changes", parts[0], parts[1])
}

func aheadWords(state branchState) string {
	return fmt.Sprintf("%s is %s ahead of %s", state.branch, plural(state.ahead, "commit"), state.base)
}

func (application *Window) openBaseDialog() {
	dialog := &application.base
	dialog.open = true
	dialog.focusNext = true
	dialog.forProject = ""
	if application.viewer.IsOpen() {
		dialog.forProject = application.viewer.Project.Path
	} else if row, ok := application.sidebar.SelectedRow(); ok && row.Project >= 0 {
		dialog.forProject = application.session.Projects[row.Project].Path
	}
	dialog.ref = application.session.Base
	if dialog.forProject != "" {
		if base, chosen := application.session.BaseOf(dialog.forProject); chosen {
			dialog.ref = base
		}
	}
	if dialog.forProject == "" {
		dialog.forAll = 1
	}
}

func (application *Window) buildBaseDialog(context *ui.Context) {
	dialog := &application.base
	if !dialog.open {
		return
	}
	apply := false
	dismissed := context.Sheet("Compare against", ui.SheetOptions{}, func() {
		apply = application.buildBaseSheet(context)
	})
	if dismissed {
		dialog.open = false
	}
	if apply {
		dialog.open = false
		application.applyBase()
	}
}

// buildBaseSheet is what the base dialog holds: the field, which projects, and its buttons. It
// tells whether the base was asked for.
func (application *Window) buildBaseSheet(context *ui.Context) bool {
	dialog := &application.base
	apply := false
	context.Caption("A branch, a tag or a commit. Empty: each project's default branch (origin/main, master…).")
	if dialog.focusNext {
		context.FocusField("base")
		dialog.focusNext = false
	}
	if context.EntryField("base", &dialog.ref, ui.TextFieldOptions{Placeholder: "origin/master", Weight: draw2d.Mono}) {
		apply = true
	}
	if dialog.forProject != "" {
		context.Tabs("base scope", &dialog.forAll, session.BaseName(dialog.forProject), "Every project")
	}
	context.Row(func() {
		context.Spacer()
		if context.Button("Cancel") {
			dialog.open = false
		}
		if context.PrimaryButton("Compare") {
			apply = true
		}
	})
	return apply
}

// applyBase compares every project, or the one the dialog was opened for, against the base typed.
func (application *Window) applyBase() {
	dialog := &application.base
	ref := strings.TrimSpace(dialog.ref)
	if dialog.forAll == 1 || dialog.forProject == "" {
		application.session.SetBaseForAll(ref)
		application.reloadAll()
		return
	}
	application.session.SetBase(dialog.forProject, ref)
	for index, project := range application.session.Projects {
		if project.Path == dialog.forProject {
			application.reload(index)
		}
	}
	application.projectsChanged()
}

func (application *Window) buildPicker(context *ui.Context) {
	if !application.pickerOpen {
		return
	}
	start := ""
	if len(application.session.Folders) > 0 {
		start = application.session.Folders[len(application.session.Folders)-1]
	}
	path, closed := context.FilePicker(ui.FilePickerOptions{Title: "Open a repository or a folder of repositories", Folder: true, Start: start})
	if path != "" {
		application.pickerOpen = false
		application.openFolders([]string{path})
		return
	}
	if closed {
		application.pickerOpen = false
	}
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
