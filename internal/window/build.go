package window

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/finder"
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

	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Background}, func() {
		application.buildTitleBar(context)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			application.buildPanes(context)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		application.buildStatusBar(context)
	})

	application.finder.Build(context, application)
	application.buildBaseDialog(context)
	application.buildHelp(context)
	application.buildPicker(context)
	application.buildQuestions(context)
	if application.toast != "" {
		context.Toast(application.toast, false)
		application.toast = ""
	}
}

// buildPanes lays the mode rail, the sidebar and the viewer side by side.
func (application *Window) buildPanes(context *ui.Context) {
	application.buildModeRail(context)
	context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
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
	application.viewer.Build(context, application)
}

func (application *Window) applyTheme(context *ui.Context) {
	application.setLook()
	context.Theme = application.look.Theme
}

func (application *Window) ModalOpen() bool {
	asked := application.questions
	return application.finder.Open || application.base.open || application.helpOpen || application.pickerOpen ||
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
	if view.Mode == viewer.DiffView {
		view.SetMode(viewer.FileView)
	} else {
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
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(48), Height: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 6, AlignX: ui.Center}, func() {
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
		if theme.ToolButton(context, "Keys (?)", "keyboard", false) {
			application.helpOpen = true
		}
	})
}

// buildTitleBar is the top of the window: the workspace on the left, a search field in the middle
// that opens the finder, and the comparison's settings on the right, as icons.
func (application *Window) buildTitleBar(context *ui.Context) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(44), Padding: ui.PadXY(14, 0), Gap: 8, AlignY: ui.Center}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center}, func() {
			context.Text(application.title(), ui.TextStyle{Weight: draw2d.Medium})
			if base := application.settings.Base; base != "" {
				context.Text("against "+base, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
			}
		})
		application.buildSearchField(context)
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, AlignY: ui.Center}, func() {
			context.Spacer()
			application.buildSettingButtons(context)
			application.buildMenu(context)
		})
	})
}

// buildSearchField is a pill, a shade off the bar with a hairline edge, lit towards the accent under
// the pointer, that opens the finder; the shortcut sits in a key cap of its own.
func (application *Window) buildSearchField(context *ui.Context) {
	palette := context.Theme
	look := application.look
	normal := ui.Look{Fill: look.Hover.WithAlpha(0.7), Radius: 17, Border: look.Chosen.WithAlpha(0.6), BorderWidth: 1}
	hovered := normal
	hovered.Fill, hovered.Border = look.Hover, palette.Primary.WithAlpha(0.45)
	search := ui.Layout{Width: ui.Fixed(400), Height: ui.Fixed(34), Padding: ui.Padding{Left: 14, Right: 6}, Gap: 10, AlignY: ui.Center}
	if context.Clickable("search files", search, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized("search", 15, palette.InkSoft)
		context.Text("Search files", ui.TextStyle{Color: palette.InkQuiet})
		context.Spacer()
		keyCap := ui.Look{Fill: look.Chosen.WithAlpha(0.55), Radius: 11}
		context.Frame(ui.Layout{Height: ui.Fixed(22), Padding: ui.PadXY(9, 0), AlignY: ui.Center}, keyCap, func() {
			context.Text("⌘P", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft, Weight: draw2d.Medium})
		})
	}) {
		application.finder.Show(finder.NamesScope)
	}
}

// buildSettingButtons are the window's settings: the projects listed, the changes counted, the
// base, and reading again.
func (application *Window) buildSettingButtons(context *ui.Context) {
	if theme.ToolButton(context, "Only the projects with changes (h)", "list-filter", application.onlyChanged) {
		application.setOnlyChanged(!application.onlyChanged)
	}
	if theme.ToolButton(context, "Count the changes not committed yet (w)", "file-pen-line", application.settings.WorkingTree) {
		application.settings.WorkingTree = !application.settings.WorkingTree
		application.clearDocuments()
		application.reloadAll()
		application.toast = workingTreeNote(application.settings.WorkingTree)
	}
	if theme.ToolButton(context, "Compare against another branch, tag or commit (b)", "git-branch", false) {
		application.openBaseDialog()
	}
	if theme.ToolButton(context, "Read the projects again (r)", "refresh-cw", false) {
		application.reloadAll()
	}
}

// buildMenu is the menu of what is done less often: opening a folder, the theme, the keys, closing.
func (application *Window) buildMenu(context *ui.Context) {
	context.MenuButton("ellipsis", "", func() {
		if context.MenuItem("folder-open", "Open folder…", "o") {
			application.pickerOpen = true
		}
		if context.MenuItem("sun-moon", "Light or dark", "t") {
			application.dark = !application.dark
			application.applyTheme(context)
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
		context.Text(fmt.Sprintf("%d projects · %d with changes · %d changed files",
			len(application.projects), application.projectsWithChanges(), len(application.changedOrder)), small)
		if viewed, total, words := application.progress(); total > 0 {
			context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
				application.buildProgress(context, viewed, total)
				context.Text(words, small)
			})
		}
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
	})
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
		context.IconSized("git-branch", 12, context.Theme.InkQuiet)
		context.Text(fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead), small)
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
