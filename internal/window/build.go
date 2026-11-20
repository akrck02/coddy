package window

import (
	"fmt"
	"github.com/akrck02/code-review/internal/mode"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
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
	application.work.take()
	application.readClassKinds()
	application.typeInTheFile(context, application.handleKeys(context))
	application.unfoldForTheKeyboard()
	application.serveOpenFile()
	application.advanceUses()

	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Background}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			application.buildPanes(context)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		application.buildStatusBar(context)
	})

	application.finder.Build(context, application)
	application.buildBaseDialog(context)
	application.buildSettings(context)
	application.buildHelp(context)
	application.buildServers(context)
	application.buildPicker(context)
	application.buildQuestions(context)
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
	if !application.sidebarFolded {
		application.buildSidebar(context)
	}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, func() {
		application.buildTitleBar(context)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		if cover, covers := application.mode.(mode.Cover); !covers || !cover.BuildOverViewer(context) {
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

func (application *Window) applyTheme(context *ui.Context) {
	application.setLook()
	context.Theme = application.look.Theme
}

func (application *Window) ModalOpen() bool {
	asked := application.questions
	return application.finder.Open || application.base.open || application.helpOpen || application.settingsOpen || application.pickerOpen || application.serversOpen ||
		asked.unsavedOpen || asked.conflictOpen
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
// the files or reviewing the changes: one place per mode, always in sight, one key each.
func (application *Window) buildModeRail(context *ui.Context) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(modeRailWidth), Height: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 6, AlignX: ui.Center}, func() {
		for _, place := range application.modes {
			chosen := application.mode == place
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
				application.setMode(place)
			}
			context.Tooltip(place.Description())
		}
		context.Spacer()
		application.buildFoldButton(context)
		if theme.ToolButton(context, "Keys (?)", "keyboard", false) {
			application.helpOpen = true
		}
	})
}

// buildMenu is the menu of what is done less often: opening a folder, the theme, the keys, closing.
func (application *Window) buildMenu(context *ui.Context) {
	context.MenuButton("ellipsis", "", func() {
		if context.MenuItem("folder-open", "Open folder…", "o") {
			application.pickerOpen = true
		}
		if context.MenuItem("settings", "Settings…", "") {
			application.settingsOpen = true
		}
		if context.MenuItem("keyboard", "Keys", "?") {
			application.helpOpen = true
		}
		context.Separator()
		if context.MenuItem("x", "Close all projects", "") {
			application.closeAll()
		}
	})
}

func (application *Window) title() string {
	switch len(application.folders) {
	case 0:
		return "Code review"
	case 1:
		return baseName(application.folders[0])
	}
	return fmt.Sprintf("%s and %d more", baseName(application.folders[0]), len(application.folders)-1)
}

// buildStatusBar says where the development stands: its projects and changed files, how far a
// mode's work has gone, and where the open file's branch stands against its base.
func (application *Window) buildStatusBar(context *ui.Context) {
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(14, 0), Gap: 16, AlignY: ui.Center}, func() {
		application.buildCounts(context, small)
		if application.loading > 0 {
			context.Text(fmt.Sprintf("reading %d…", application.loading), small)
		}
		context.Spacer()
		if application.viewer.IsOpen() {
			application.buildBranch(context, small)
		}
		if !application.settings.WorkingTree {
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
		context.Box(ui.Layout{Gap: 5, AlignY: ui.Center}, func() {
			context.IconSized(icon, 13, context.Theme.InkQuiet)
			if more != nil {
				more()
			}
			context.Text(text, small)
		})
		context.Tooltip(words)
	}
	files, changed, projects := len(application.changedOrder), application.projectsWithChanges(), len(application.projects)
	counted("file-diff", strconv.Itoa(files), plural(files, "file")+" changed", nil)
	counted("folder-git-2", fmt.Sprintf("%d/%d", changed, projects), fmt.Sprintf("%d of %d projects have changes", changed, projects), nil)
	if viewed, total, words := application.progress(); total > 0 {
		counted("eye", fmt.Sprintf("%d/%d", viewed, total), words, func() { application.buildProgress(context, viewed, total) })
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
	for _, current := range application.projects {
		if current.Path == project.Path {
			project = current
		}
	}
	if project.Base != "" {
		context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
			context.IconSized("git-branch", 12, context.Theme.InkQuiet)
			context.Text(project.Branch+" ← "+project.Base, small)
		})
		context.Tooltip(fmt.Sprintf("%s is %s ahead of %s", project.Branch, plural(project.Ahead, "commit"), project.Base))
	}
}

func (application *Window) openBaseDialog() {
	dialog := &application.base
	dialog.open = true
	dialog.focusNext = true
	dialog.forProject = ""
	if application.viewer.IsOpen() {
		dialog.forProject = application.viewer.Project.Path
	} else if row, ok := application.sidebar.SelectedRow(); ok && row.Project >= 0 {
		dialog.forProject = application.projects[row.Project].Path
	}
	dialog.ref = application.settings.Base
	if dialog.forProject != "" {
		if base, chosen := application.projectBases[dialog.forProject]; chosen {
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
		context.Tabs("base scope", &dialog.forAll, baseName(dialog.forProject), "Every project")
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
		application.settings.Base = ref
		application.projectBases = map[string]string{}
		application.clearDocuments()
		application.reloadAll()
		return
	}
	application.projectBases[dialog.forProject] = ref
	for index, project := range application.projects {
		if project.Path == dialog.forProject {
			application.reload(index)
		}
	}
	application.projectsChanged()
}

func (application *Window) buildHelp(context *ui.Context) {
	if !application.helpOpen {
		return
	}
	dismissed := context.Sheet("Keys", ui.SheetOptions{Width: ui.Fixed(helpSheetWidth)}, func() {
		context.Caption("The reader writes a file only when you save it, and git only reads: no commit, no push.")
		// the help scrolls when the window is too short for it
		_, windowHeight := context.Size()
		context.ScrollArea("help", ui.Layout{Width: ui.Grow(), Height: ui.Fixed(max(windowHeight-helpRoomAround, 200))}, func() {
			context.Box(ui.Layout{Width: ui.Grow(), Gap: helpColumnGap}, func() {
				context.Box(ui.Layout{Direction: ui.Vertical, Gap: 16}, func() {
					application.buildHelpColumn(context, "Anywhere", helpRows(commands, anywhere))
					application.buildHelpColumn(context, "Modes", helpRows(application.modeCommands(), anywhere))
					if own := application.ownCommands(); len(own) > 0 {
						application.buildHelpColumn(context, "In the "+application.mode.Name()+" mode", helpRows(own, anywhere))
					}
				})
				context.Box(ui.Layout{Direction: ui.Vertical, Gap: 16}, func() {
					application.buildHelpColumn(context, "In the sidebar", helpRows(commands, inSidebar))
					application.buildHelpColumn(context, "In the file", helpRows(commands, inViewer))
					application.buildHelpColumn(context, "While editing", append(helpRows(commands, editing), editingKeyRows()...))
				})
			})
		})
		context.Row(func() {
			context.Spacer()
			if context.PrimaryButton("Close") {
				application.helpOpen = false
			}
		})
	})
	if dismissed {
		application.helpOpen = false
	}
}

// buildHelpColumn lists the keys of one place, under its name.
func (application *Window) buildHelpColumn(context *ui.Context, title string, rows []helpRow) {
	context.Box(ui.Layout{Direction: ui.Vertical, Gap: 4}, func() {
		context.Text(title, ui.TextStyle{Weight: draw2d.Bold})
		for _, row := range rows {
			context.Box(ui.Layout{Gap: 12}, func() {
				keyStyle := ui.TextStyle{Weight: draw2d.Mono, Color: context.Theme.Primary}
				context.Box(ui.Layout{Width: ui.Fixed(helpKeysWidth), Gap: 10, Wrap: true}, func() {
					for _, key := range row.keys {
						context.Text(key, keyStyle)
					}
				})
				// go-ui draws a text on one line, so the words wrap as pieces of a box
				context.Box(ui.Layout{Width: ui.Fixed(helpTextWidth), Gap: 4, Wrap: true}, func() {
					for _, word := range strings.Fields(row.description) {
						context.Label(word)
					}
				})
			})
		}
	})
}

// helpKeysWidth and helpTextWidth are the help's columns of keys and of what they do, in points;
// the help shows two of each side by side.
const (
	helpKeysWidth = 130
	helpTextWidth = 290
	helpColumnGap = 32
	// helpRoomAround is the window's height the help leaves to its title, caption, button and margins
	helpRoomAround = 200
	helpSheetWidth = 2*(helpKeysWidth+12+helpTextWidth) + helpColumnGap + 48
)

func (application *Window) buildPicker(context *ui.Context) {
	if !application.pickerOpen {
		return
	}
	start := ""
	if len(application.folders) > 0 {
		start = application.folders[len(application.folders)-1]
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
