package window

import (
	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// compactWindow is the window's width under which the bar drops words to keep its buttons
	compactWindow = 1100
	// toolWidth is a tool button's width and the gap after it
	toolWidth = 34
)

// buildTitleBar is the top of the window, and follows what is in front: the file open, named by
// its path, or else the workspace; the search; the file's buttons; the mode's own buttons and the
// menu.
func (application *Window) buildTitleBar(context *ui.Context) {
	windowWidth, _ := context.Size()
	compact := windowWidth < compactWindow
	search := float32(360)
	if compact {
		search = 220
	}
	file := application.showsFile()
	buttons := application.modeButtonsWidth() + toolWidth // the menu
	if file {
		buttons += application.viewer.ButtonsWidth(context, compact, application.mode.ShowsChanges()) + 13
	}
	// the search stays in the middle while the buttons fit in their half; it moves aside otherwise
	half := (windowWidth - 28 - 16 - search) / 2
	room, left := half, ui.Fixed(half)
	if buttons > half {
		room, left = 2*half-buttons, ui.Grow()
	}
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(44), Padding: ui.PadXY(14, 0), Gap: 8, AlignY: ui.Center}, func() {
		context.Box(ui.Layout{Width: left, Gap: 6, AlignY: ui.Center}, func() {
			if file {
				application.viewer.BuildTitle(context, room, compact)
				return
			}
			application.buildWorkspaceTitle(context)
		})
		application.buildSearchField(context, search)
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, AlignY: ui.Center}, func() {
			context.Spacer()
			if file {
				application.viewer.BuildButtons(context, application, compact, application.mode.ShowsChanges())
				context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Fixed(20)}, ui.Look{Fill: context.Theme.Line}, nil)
			}
			application.buildModeButtons(context)
			application.buildMenu(context)
		})
	})
}

// showsFile tells whether a file is in front: open, and not covered by what the mode shows.
func (application *Window) showsFile() bool {
	if !application.viewer.IsOpen() {
		return false
	}
	cover, covers := application.mode.(mode.Cover)
	return !covers || !cover.Covers()
}

// buildWorkspaceTitle names the workspace, and what it is compared against when that was chosen.
func (application *Window) buildWorkspaceTitle(context *ui.Context) {
	context.Text(application.title(), ui.TextStyle{Weight: draw2d.Medium})
	if base := application.settings.Base; base != "" {
		context.Text("against "+base, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkQuiet})
	}
}

// buildSearchField is a pill, a shade off the bar with a hairline edge, lit towards the accent under
// the pointer, that opens the finder; the shortcut sits in a key cap of its own.
func (application *Window) buildSearchField(context *ui.Context, width float32) {
	palette := context.Theme
	look := application.look
	normal := ui.Look{Fill: look.Hover.WithAlpha(0.7), Radius: 17, Border: look.Chosen.WithAlpha(0.6), BorderWidth: 1}
	hovered := normal
	hovered.Fill, hovered.Border = look.Hover, palette.Primary.WithAlpha(0.45)
	search := ui.Layout{Width: ui.Fixed(width), Height: ui.Fixed(34), Padding: ui.Padding{Left: 14, Right: 6}, Gap: 10, AlignY: ui.Center}
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

// buildModeButtons are the settings the mode on uses: in a mode of changes, the changes counted and
// the base; in the others, whether files are edited. Reading again is in every mode.
func (application *Window) buildModeButtons(context *ui.Context) {
	if application.mode.ShowsChanges() {
		if theme.ToolButton(context, "Count the changes not committed yet (w)", "file-pen-line", application.settings.WorkingTree) {
			application.keepingEdits(application.toggleWorkingTree)
		}
		if theme.ToolButton(context, "Compare against another branch, tag or commit (b)", "git-branch", false) {
			application.openBaseDialog()
		}
	} else {
		lock, tip := "lock-open", "Read-only: off, files can be edited (⌘⇧L)"
		if application.settings.ReadOnly {
			lock, tip = "lock", "Read-only: on, files are only read (⌘⇧L)"
		}
		if theme.ToolButton(context, tip, lock, application.settings.ReadOnly) {
			application.setReadOnly(!application.settings.ReadOnly)
		}
	}
	if theme.ToolButton(context, "Read the projects again (r)", "refresh-cw", false) {
		application.reloadAll()
	}
}

// modeButtonsWidth is how wide buildModeButtons is.
func (application *Window) modeButtonsWidth() float32 {
	if application.mode.ShowsChanges() {
		return 3 * toolWidth
	}
	return 2 * toolWidth
}
