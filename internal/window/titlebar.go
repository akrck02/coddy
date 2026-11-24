package window

import (
	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/mode"
	"github.com/akrck02/code-review/internal/settings"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// compactBar is the bar's width under which it drops words to keep its buttons
	compactBar = 760
	// toolWidth is a tool button's width and the gap after it
	toolWidth = 34
	// barGap is the room between the bar's title and its buttons
	barGap = 8
	// searchName names the search button, its tooltip and its place in the tests
	searchName = "Search files (⌘P)"
)

// buildTitleBar is the top of the viewer's side, right of the rail and the sidebar, and follows
// what is in front: the file open, named by its path, or else the workspace; then the search, the
// file's buttons, the mode's own buttons and the menu.
func (application *Window) buildTitleBar(context *ui.Context) {
	windowWidth, _ := context.Size()
	barWidth := application.viewerWidth(windowWidth)
	compact := barWidth < compactBar
	file := application.showsFile()
	buttons := toolWidth + application.modeButtonsWidth() + toolWidth // the search and the menu
	if file {
		buttons += application.viewer.ButtonsWidth(context, compact, application.mode.ShowsChanges()) + 13
	}
	room := barWidth - 2*theme.BarPadding - barGap - buttons
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(theme.BarHeight), Padding: ui.PadXY(theme.BarPadding, 0), Gap: barGap, AlignY: ui.Center}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 6, AlignY: ui.Center}, func() {
			if application.showsFolder() {
				application.explorer.BuildTitle(context, application)
				return
			}
			if file {
				application.viewer.BuildTitle(context, application, room, compact)
				return
			}
			application.buildWorkspaceTitle(context)
		})
		context.Box(ui.Layout{Gap: 4, AlignY: ui.Center}, func() {
			if file {
				application.viewer.BuildButtons(context, application, compact, application.mode.ShowsChanges())
				context.Frame(ui.Layout{Width: ui.Fixed(1), Height: ui.Fixed(20)}, ui.Look{Fill: context.Theme.Line}, nil)
			}
			if theme.ToolButton(context, searchName, "search", false) {
				application.finder.Show(finder.NamesScope)
			}
			application.buildModeButtons(context)
			application.buildMenu(context)
		})
	})
}

// showsFile tells whether a file is in front: open, and not covered by a folder or by what the
// mode shows.
func (application *Window) showsFile() bool {
	if !application.viewer.IsOpen() || application.explorer.Open {
		return false
	}
	cover, covers := application.mode.(mode.Cover)
	return !covers || !cover.Covers()
}

// showsFolder tells whether a folder is in front: shown, and not covered by what the mode shows.
func (application *Window) showsFolder() bool {
	cover, covers := application.mode.(mode.Cover)
	return application.explorer.Open && (!covers || !cover.Covers())
}

// buildWorkspaceTitle names the workspace, and what it is compared against when that was chosen.
func (application *Window) buildWorkspaceTitle(context *ui.Context) {
	context.Text(application.title(), ui.TextStyle{Weight: draw2d.Medium})
	if base := application.settings.Base; base != "" {
		context.Text("against "+base, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkQuiet})
	}
}

// buildModeButtons are the settings the mode on uses: in a mode of changes, the changes counted and
// the base; in the others, whether files are edited. The settings themselves are in every mode.
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
	if theme.ToolButton(context, "Settings (⌘,)", "settings", application.settingsScreen.Open) {
		application.settingsScreen.Toggle(settings.Appearance)
	}
}

// modeButtonsWidth is how wide buildModeButtons is.
func (application *Window) modeButtonsWidth() float32 {
	if application.mode.ShowsChanges() {
		return 3 * toolWidth
	}
	return 2 * toolWidth
}
