package window

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/sidebar"
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
			application.sidebar.Width = min(application.sidebar.Width, widest)
			context.Resizable("sidebar", ui.RightSide, &application.sidebar.Width, 220, widest,
				ui.Layout{Direction: ui.Vertical, Height: ui.Grow()}, func() {
					application.sidebar.Build(context, application)
				})
			context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Grow()}, ui.Look{Fill: context.Theme.Line}, nil)
			application.editor.Build(context, application)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: context.Theme.Line}, nil)
		application.buildStatusBar(context)
	})

	application.finder.Build(context, application)
	application.buildBaseDialog(context)
	application.buildHelp(context)
	application.buildPicker(context)
	if application.toast != "" {
		context.Toast(application.toast, false)
		application.toast = ""
	}
}

func (application *Window) applyTheme(context *ui.Context) {
	application.setLook()
	context.Theme = application.look.Theme
}

func (application *Window) ModalOpen() bool {
	return application.finder.Open || application.base.open || application.helpOpen || application.pickerOpen
}

// enterCurrentProject takes the sidebar into the project of the selected row, or of the file open.
func (application *Window) enterCurrentProject() {
	if row, ok := application.sidebar.SelectedRow(); ok && row.Project >= 0 && application.focus == sidebarPane {
		application.sidebar.Enter(application, row.Project)
		return
	}
	if application.editor.IsOpen() {
		application.sidebar.Enter(application, application.editor.Reference.Project)
	}
}

func workingTreeNote(on bool) string {
	if on {
		return "Counting the changes not committed yet"
	}
	return "Comparing the last commit only"
}

func (application *Window) toggleFocus() {
	if application.focus == sidebarPane && application.editor.IsOpen() {
		application.focus = editorPane
		return
	}
	application.focus = sidebarPane
}

func (application *Window) toggleView() {
	editor := &application.editor
	if editor.Mode == viewer.DiffView {
		editor.SetMode(viewer.FileView)
	} else {
		editor.SetMode(viewer.DiffView)
	}
}

// copyPath puts the open file's path, from the project's top, on the clipboard.
func (application *Window) copyPath(context *ui.Context) {
	if !application.editor.IsOpen() {
		return
	}
	path := application.editor.Project.Name + "/" + application.editor.Reference.Path
	context.Copy(path)
	application.toast = "Copied " + path
}

// buildModeRail is the strip on the far left that chooses what the reader is doing: reading the
// files, or reviewing the changes. Two places, always in sight, one key each.
func (application *Window) buildModeRail(context *ui.Context) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(48), Height: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 6, AlignX: ui.Center}, func() {
		for _, place := range []struct {
			mode       sidebar.Mode
			icon, name string
		}{
			{sidebar.FilesMode, "folder-tree", "Files (1): read any file, the changes marked beside the lines"},
			{sidebar.ReviewMode, "git-pull-request", "Review (2): only the changed files, as diffs, marked as viewed one by one"},
		} {
			chosen := application.sidebar.Mode == place.mode
			ink, normal := palette.InkQuiet, ui.Look{Radius: 8}
			if chosen {
				ink, normal = palette.Ink, ui.Look{Fill: application.look.Chosen, Radius: 8}
			}
			hovered := normal
			if !chosen {
				hovered.Fill = application.look.Hover
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
		// a pill, a shade off the bar with a hairline edge, lit towards the accent under the
		// pointer; the shortcut sits in a key cap of its own
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
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, AlignY: ui.Center}, func() {
			context.Spacer()
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

func (application *Window) title() string {
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
func (application *Window) buildStatusBar(context *ui.Context) {
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(14, 0), Gap: 16, AlignY: ui.Center}, func() {
		context.Text(fmt.Sprintf("%d projects · %d with changes · %d changed files",
			len(application.projects), application.projectsWithChanges(), len(application.changedOrder)), small)
		if viewed, total := application.reviewProgress(); total > 0 {
			context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
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
				context.Text(fmt.Sprintf("%d of %d viewed", viewed, total), small)
			})
		}
		if application.loading > 0 {
			context.Text(fmt.Sprintf("reading %d…", application.loading), small)
		}
		context.Spacer()
		if application.editor.IsOpen() {
			project := application.editor.Project
			for _, current := range application.projects {
				if current.Path == project.Path {
					project = current
				}
			}
			if project.Base != "" {
				context.IconSized("git-branch", 12, palette.InkQuiet)
				context.Text(fmt.Sprintf("%s ← %s · %d commits", project.Branch, project.Base, project.Ahead), small)
			}
		}
		if !application.settings.WorkingTree {
			context.Text("commits only", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Warning})
		}
	})
}

func (application *Window) openBaseDialog() {
	dialog := &application.base
	dialog.open = true
	dialog.focusNext = true
	dialog.forProject = ""
	if application.editor.IsOpen() {
		dialog.forProject = application.editor.Project.Path
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

func (application *Window) buildHelp(context *ui.Context) {
	if !application.helpOpen {
		return
	}
	dismissed := context.Sheet("Keys", ui.SheetOptions{Width: ui.Fixed(helpSheetWidth)}, func() {
		context.Caption("The reader never changes a repository: it only runs git commands that read.")
		context.Box(ui.Layout{Width: ui.Grow(), Gap: helpColumnGap}, func() {
			application.buildHelpColumn(context, "Anywhere", helpRows(anywhere))
			context.Box(ui.Layout{Direction: ui.Vertical, Gap: 16}, func() {
				application.buildHelpColumn(context, "In the sidebar", helpRows(inSidebar))
				application.buildHelpColumn(context, "In the file", helpRows(inEditor))
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
				context.Box(ui.Layout{Width: ui.Fixed(helpKeysWidth)}, func() {
					context.Text(row.keys, ui.TextStyle{Weight: draw2d.Mono, Color: context.Theme.Primary})
				})
				context.Box(ui.Layout{Width: ui.Fixed(helpTextWidth)}, func() {
					context.Label(row.description)
				})
			})
		}
	})
}

// helpKeysWidth and helpTextWidth are the help's columns of keys and of what they do, in points;
// the help shows two of each side by side.
const (
	helpKeysWidth  = 150
	helpTextWidth  = 250
	helpColumnGap  = 32
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
