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
	application.takeResults()
	application.handleKeys(context)

	theme := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, func() {
		application.buildToolbar(context)
		context.Separator()
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func() {
			context.Resizable("sidebar", ui.RightSide, &application.sidebar.width, 220, 720,
				ui.Layout{Direction: ui.Vertical, Height: ui.Grow()}, func() {
					application.sidebar.build(context, application)
				})
			context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: theme.Line}, nil)
			application.editor.build(context, application)
		})
		context.Separator()
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
	if application.dark {
		context.Theme = ui.DarkTheme()
	} else {
		context.Theme = ui.LightTheme()
	}
	application.setPalette()
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
	case keys.command() && keys.pressed(input.KeyP), keys.plain(input.KeyP) && keys.shift():
		application.finder.show()
	case keys.command() && keys.pressed(input.KeyO), keys.plain(input.KeyO):
		application.pickerOpen = true
	case keys.command() && keys.pressed(input.KeyR), keys.plain(input.KeyR):
		application.reloadAll()
		application.toast = "Reading the projects again"
	case keys.plain(input.KeyB):
		application.openBaseDialog()
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
		application.sidebar.mode = 1 - application.sidebar.mode
		application.sidebar.stale = true
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

func (application *App) buildToolbar(context *ui.Context) {
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(44), Padding: ui.PadXY(10, 0), Gap: 6, AlignY: ui.Center}, func() {
		context.IconSized("git-compare", 18, context.Theme.Primary)
		context.Text(application.title(), ui.TextStyle{Weight: draw2d.Bold})
		context.Spacer()
		onlyIcon := "square"
		if application.onlyChanged {
			onlyIcon = "square-check"
		}
		if context.FlatIconButton(onlyIcon, "Only changed projects") {
			application.setOnlyChanged(!application.onlyChanged)
		}
		context.Tooltip("h: hide the projects the development did not change, in the sidebar and the finder")
		if context.FlatIconButton("search", "Find file") {
			application.finder.show()
		}
		context.Tooltip("⌘P")
		if context.FlatIconButton("git-branch", "Base") {
			application.openBaseDialog()
		}
		context.Tooltip("b: compare against another branch, tag or commit")
		workingIcon := "square"
		if application.settings.WorkingTree {
			workingIcon = "square-check"
		}
		if context.FlatIconButton(workingIcon, "Uncommitted") {
			application.settings.WorkingTree = !application.settings.WorkingTree
			application.clearDocuments()
			application.reloadAll()
		}
		context.Tooltip("w: count the changes not committed yet")
		if context.FlatIconButton("refresh-cw", "") {
			application.reloadAll()
		}
		context.Tooltip("r: read the projects again")
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

func (application *App) buildStatusBar(context *ui.Context) {
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(26), Padding: ui.PadXY(12, 0), Gap: 14, AlignY: ui.Center}, func() {
		changedFiles := len(application.changedOrder)
		context.Caption(fmt.Sprintf("%d projects · %d with changes · %d changed files",
			len(application.projects), application.projectsWithChanges(), changedFiles))
		if application.loading > 0 {
			context.Caption(fmt.Sprintf("reading %d…", application.loading))
		}
		if application.editor.isOpen() {
			project := application.editor.project
			for _, current := range application.projects {
				if current.Path == project.Path {
					project = current
				}
			}
			if project.Base != "" {
				context.Caption(fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead))
			}
		}
		context.Spacer()
		context.Caption("⌘P find · n/m next/previous change · [ ] hunks · d diff/file · Tab pane · ? keys")
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
	{"⌘P  or  Shift+P", "Find a file in any project"},
	{"n  /  m", "Next / previous changed file, across every project"},
	{"]  /  [", "Next / previous change in the file"},
	{"d", "Diff or the whole file"},
	{"f", "Fold the unchanged lines, or show them all"},
	{"Enter", "Open a fold (editor) · open a file or folder (sidebar)"},
	{"Tab", "Move between the sidebar and the editor"},
	{"↑ ↓ ← →  PgUp PgDn  Home End", "Move in the sidebar or scroll the file"},
	{"j  /  k  ·  Space", "Scroll a line · a page"},
	{"c", "Sidebar: changes only, or every file"},
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
