package app

import (
	"fmt"
	"strings"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
	"github.com/akrck02/go-ui/window"
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
func (application *App) Build(context *ui.Context) {
	if !application.themeSet {
		application.applyTheme(context)
		application.themeSet = true
	}
	application.keys = keys{state: application.tap.state}
	windowOpen.Store(true)
	application.work.take()
	application.readClassKinds()
	application.handleKeys(context)

	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Background}, func() {
		application.buildTitleBar(context)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			application.buildModeRail(context)
			context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
			// the sidebar never takes more than two fifths of the window, so the code keeps its room
			// when the window is made narrow
			windowWidth, _ := context.Size()
			widest := min(max(windowWidth*0.4, 220), 720)
			application.sidebar.width = min(application.sidebar.width, widest)
			context.Resizable("sidebar", ui.RightSide, &application.sidebar.width, 220, widest,
				ui.Layout{Direction: ui.Vertical, Height: ui.Grow()}, func() {
					application.sidebar.build(context, application)
				})
			context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
			application.editor.build(context, application)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		application.buildStatusBar(context)
	})

	application.finder.build(context, application)
	application.buildBaseDialog(context)
	application.buildHelp(context)
	application.buildPicker(context)
	if application.toast != "" {
		context.Toast(application.toast, false)
		application.toast = ""
	}
}

func (application *App) applyTheme(context *ui.Context) {
	application.setLook()
	context.Theme = application.look.theme
}

func (application *App) modalOpen() bool {
	return application.finder.open || application.base.open || application.helpOpen || application.pickerOpen
}

// handleKeys answers the shortcuts. While a dialog is open its own keys are read by the dialog.
func (application *App) handleKeys(context *ui.Context) {
	keys := application.keys
	if keys.command() && keys.pressed(input.KeyQ) {
		window.Quit()
		return
	}
	if application.modalOpen() || context.WantsKeyboard() {
		return
	}
	switch {
	case keys.command() && keys.shift() && keys.pressed(input.KeyF), strings.Contains(keys.typedText(), "/"):
		application.finder.show(contentScope)
	case keys.command() && keys.pressed(input.KeyP), keys.plain(input.KeyP) && keys.shift():
		application.finder.show(namesScope)
	case keys.command() && keys.pressed(input.KeyO), keys.plain(input.KeyO):
		application.pickerOpen = true
	case keys.command() && keys.pressed(input.KeyR), keys.plain(input.KeyR):
		application.reloadAll()
		application.toast = "Reading the projects again"
	case keys.plain(input.KeyB):
		application.openBaseDialog()
	case application.sidebar.entered != "" && (keys.pressed(input.KeyEscape) || keys.pressed(input.KeyBackspace)):
		application.sidebar.leave(application)
		application.focus = sidebarPane
	case keys.plain(input.KeyG):
		application.sidebar.toggleChangedFilter()
	case keys.plain(input.KeyE):
		application.enterCurrentProject()
	case keys.pressed(input.KeyTab):
		application.toggleFocus()
	case keys.plain(input.KeyN):
		application.stepChangedFile(1)
	case keys.plain(input.KeyM):
		application.stepChangedFile(-1)
	case keys.pressed(input.KeyRightBracket):
		application.editor.stepHunk(1)
	case keys.pressed(input.KeyLeftBracket):
		application.editor.stepHunk(-1)
	case keys.plain(input.KeyD):
		application.toggleView()
	case keys.plain(input.KeyF):
		if application.editor.mode == diffView {
			application.editor.toggleContext()
		}
	case keys.plain(input.KeyC):
		application.setMode(1 - application.sidebar.mode)
	case keys.plain(input.Key1):
		application.setMode(filesMode)
	case keys.plain(input.Key2):
		application.setMode(reviewMode)
	case keys.plain(input.KeyV):
		if reference, ok := application.markTarget(); ok {
			application.toggleViewed(reference, application.focus == editorPane || application.editor.reference == reference)
		}
	case keys.plain(input.KeyH):
		application.setOnlyChanged(!application.onlyChanged)
	case keys.plain(input.KeyW):
		application.settings.WorkingTree = !application.settings.WorkingTree
		application.clearDocuments()
		application.reloadAll()
		application.toast = workingTreeNote(application.settings.WorkingTree)
	case keys.plain(input.KeyT):
		application.dark = !application.dark
		application.applyTheme(context)
	case keys.plain(input.KeyY):
		application.copyPath(context)
	case strings.Contains(keys.typedText(), "?") || keys.pressed(input.KeyF1):
		application.helpOpen = true
	case application.focus == sidebarPane:
		application.sidebar.handleKeys(application, keys)
	default:
		application.editor.handleKeys(keys)
	}
}

// enterCurrentProject takes the sidebar into the project of the selected row, or of the file open.
func (application *App) enterCurrentProject() {
	if row, ok := application.sidebar.selectedRow(); ok && row.project >= 0 && application.focus == sidebarPane {
		application.sidebar.enter(application, row.project)
		return
	}
	if application.editor.isOpen() {
		application.sidebar.enter(application, application.editor.reference.project)
	}
}

func workingTreeNote(on bool) string {
	if on {
		return "Counting the changes not committed yet"
	}
	return "Comparing the last commit only"
}

func (application *App) toggleFocus() {
	if application.focus == sidebarPane && application.editor.isOpen() {
		application.focus = editorPane
		return
	}
	application.focus = sidebarPane
}

func (application *App) toggleView() {
	editor := &application.editor
	if editor.mode == diffView {
		editor.setMode(fileView)
	} else {
		editor.setMode(diffView)
	}
}

// copyPath puts the open file's path, from the project's top, on the clipboard.
func (application *App) copyPath(context *ui.Context) {
	if !application.editor.isOpen() {
		return
	}
	path := application.editor.project.Name + "/" + application.editor.reference.path
	context.Copy(path)
	application.toast = "Copied " + path
}

// buildModeRail is the strip on the far left that chooses what the reader is doing: reading the
// files, or reviewing the changes. Two places, always in sight, one key each.
func (application *App) buildModeRail(context *ui.Context) {
	theme := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(48), Height: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 6, AlignX: ui.Center}, func() {
		for _, place := range []struct {
			mode       sidebarMode
			icon, name string
		}{
			{filesMode, "folder-tree", "Files (1): read any file, the changes marked beside the lines"},
			{reviewMode, "git-pull-request", "Review (2): only the changed files, as diffs, marked as viewed one by one"},
		} {
			chosen := application.sidebar.mode == place.mode
			ink, normal := theme.InkQuiet, ui.Look{Radius: 8}
			if chosen {
				ink, normal = theme.Ink, ui.Look{Fill: application.look.chosen, Radius: 8}
			}
			hovered := normal
			if !chosen {
				hovered.Fill = application.look.hover
			}
			layout := ui.Layout{Width: ui.Fixed(36), Height: ui.Fixed(36), AlignX: ui.Center, AlignY: ui.Center}
			if context.Clickable(place.name, layout, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
				context.IconSized(place.icon, 20, ink)
			}) {
				application.setMode(place.mode)
			}
			context.Tooltip(place.name)
		}
		context.Spacer()
		if toolButton(context, "Keys (?)", "keyboard", false) {
			application.helpOpen = true
		}
	})
}

// buildTitleBar is the top of the window: the workspace on the left, a search field in the middle
// that opens the finder, and the comparison's settings on the right, as icons.
func (application *App) buildTitleBar(context *ui.Context) {
	theme := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(44), Padding: ui.PadXY(14, 0), Gap: 8, AlignY: ui.Center}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center}, func() {
			context.Text(application.title(), ui.TextStyle{Weight: draw2d.Medium})
			if base := application.settings.Base; base != "" {
				context.Text("against "+base, ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet})
			}
		})
		// a pill, a shade off the bar with a hairline edge, lit towards the accent under the
		// pointer; the shortcut sits in a key cap of its own
		look := application.look
		normal := ui.Look{Fill: look.hover.WithAlpha(0.7), Radius: 17, Border: look.chosen.WithAlpha(0.6), BorderWidth: 1}
		hovered := normal
		hovered.Fill, hovered.Border = look.hover, theme.Primary.WithAlpha(0.45)
		search := ui.Layout{Width: ui.Fixed(400), Height: ui.Fixed(34), Padding: ui.Padding{Left: 14, Right: 6}, Gap: 10, AlignY: ui.Center}
		if context.Clickable("search files", search, ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}, func() {
			context.IconSized("search", 15, theme.InkSoft)
			context.Text("Search files", ui.TextStyle{Color: theme.InkQuiet})
			context.Spacer()
			keyCap := ui.Look{Fill: look.chosen.WithAlpha(0.55), Radius: 11}
			context.Frame(ui.Layout{Height: ui.Fixed(22), Padding: ui.PadXY(9, 0), AlignY: ui.Center}, keyCap, func() {
				context.Text("⌘P", ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkSoft, Weight: draw2d.Medium})
			})
		}) {
			application.finder.show(namesScope)
		}
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, AlignY: ui.Center}, func() {
			context.Spacer()
			if toolButton(context, "Only the projects with changes (h)", "list-filter", application.onlyChanged) {
				application.setOnlyChanged(!application.onlyChanged)
			}
			if toolButton(context, "Count the changes not committed yet (w)", "file-pen-line", application.settings.WorkingTree) {
				application.settings.WorkingTree = !application.settings.WorkingTree
				application.clearDocuments()
				application.reloadAll()
				application.toast = workingTreeNote(application.settings.WorkingTree)
			}
			if toolButton(context, "Compare against another branch, tag or commit (b)", "git-branch", false) {
				application.openBaseDialog()
			}
			if toolButton(context, "Read the projects again (r)", "refresh-cw", false) {
				application.reloadAll()
			}
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
		})
	})
}

func (application *App) title() string {
	switch len(application.folders) {
	case 0:
		return "Code review"
	case 1:
		return baseName(application.folders[0])
	}
	return fmt.Sprintf("%s and %d more", baseName(application.folders[0]), len(application.folders)-1)
}

// buildStatusBar says where the development stands: its projects and changed files, how much of
// it has been viewed, and where the open file's branch stands against its base.
func (application *App) buildStatusBar(context *ui.Context) {
	theme := context.Theme
	small := ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkSoft}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(14, 0), Gap: 16, AlignY: ui.Center}, func() {
		context.Text(fmt.Sprintf("%d projects · %d with changes · %d changed files",
			len(application.projects), application.projectsWithChanges(), len(application.changedOrder)), small)
		if viewed, total := application.reviewProgress(); total > 0 {
			context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
				colour := theme.InkQuiet
				if viewed == total {
					colour = theme.Positive
				}
				context.Paint(ui.Layout{Width: ui.Fixed(56), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
					drawing.Rectangle(area, 2, application.look.hover.Mix(theme.Ink, 0.12), draw2d.Color{}, 0)
					if viewed > 0 {
						done := area
						done.Width = area.Width * float32(viewed) / float32(total)
						drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
					}
				})
				context.Text(fmt.Sprintf("%d of %d viewed", viewed, total), small)
			})
		}
		if application.loading > 0 {
			context.Text(fmt.Sprintf("reading %d…", application.loading), small)
		}
		context.Spacer()
		if application.editor.isOpen() {
			project := application.editor.project
			for _, current := range application.projects {
				if current.Path == project.Path {
					project = current
				}
			}
			if project.Base != "" {
				context.IconSized("git-branch", 12, theme.InkQuiet)
				context.Text(fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead), small)
			}
		}
		if !application.settings.WorkingTree {
			context.Text("commits only", ui.TextStyle{Size: theme.SmallTextSize, Color: theme.Warning})
		}
	})
}

func (application *App) openBaseDialog() {
	dialog := &application.base
	dialog.open = true
	dialog.focusNext = true
	dialog.forProject = ""
	if application.editor.isOpen() {
		dialog.forProject = application.editor.project.Path
	} else if row, ok := application.sidebar.selectedRow(); ok && row.project >= 0 {
		dialog.forProject = application.projects[row.project].Path
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

func (application *App) buildBaseDialog(context *ui.Context) {
	dialog := &application.base
	if !dialog.open {
		return
	}
	apply := false
	dismissed := context.Sheet("Compare against", ui.SheetOptions{}, func() {
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
	})
	if dismissed {
		dialog.open = false
	}
	if !apply {
		return
	}
	dialog.open = false
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

var helpKeys = [][2]string{
	{"⌘P  or  Shift+P", "Find a file by its name, in any project"},
	{"⌘⇧F  or  /", "Find a text in the files of every project (Tab switches in the finder)"},
	{"n  /  m", "Next / previous changed file, across every project"},
	{"]  /  [", "Next / previous change in the file"},
	{"1  /  2  ·  c", "Files mode (read any file) / Review mode (the changes, as diffs)"},
	{"Enter (on a project)  ·  e", "Enter the project: the sidebar lists only its tree"},
	{"g", "Files mode: only the changed files, or every file"},
	{"Esc  ·  Backspace", "Back to every project"},
	{"v", "Mark the file as viewed and open the next one not viewed"},
	{"d", "Diff or the whole file"},
	{"f", "Fold the unchanged lines, or show them all"},
	{"Enter", "Open a fold (editor) · open a file or folder (sidebar)"},
	{"Tab", "Move between the sidebar and the editor"},
	{"↑ ↓ ← →  PgUp PgDn  Home End", "Move in the sidebar or scroll the file"},
	{"j  /  k  ·  Space", "Scroll a line · a page"},
	{"h", "Only the projects with changes, or every project (sidebar and finder)"},
	{"b", "Compare against another branch, tag or commit"},
	{"w", "Count the changes not committed yet, or only commits"},
	{"r  or  ⌘R", "Read every project again"},
	{"o  or  ⌘O", "Open a repository or a folder of repositories"},
	{"y", "Copy the open file's path"},
	{"t", "Light or dark"},
	{"⌘Q", "Quit"},
}

func (application *App) buildHelp(context *ui.Context) {
	if !application.helpOpen {
		return
	}
	dismissed := context.Sheet("Keys", ui.SheetOptions{}, func() {
		context.Caption("The reader never changes a repository: it only runs git commands that read.")
		for _, entry := range helpKeys {
			context.Box(ui.Layout{Width: ui.Grow(), Gap: 12}, func() {
				context.Box(ui.Layout{Width: ui.Fixed(230)}, func() {
					context.Text(entry[0], ui.TextStyle{Weight: draw2d.Mono, Color: context.Theme.Primary})
				})
				context.Label(entry[1])
			})
		}
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

func (application *App) buildPicker(context *ui.Context) {
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

func (sidebar *sidebarState) selectedRow() (treeRow, bool) {
	if sidebar.selected < 0 || sidebar.selected >= len(sidebar.rows) {
		return treeRow{}, false
	}
	return sidebar.rows[sidebar.selected], true
}
