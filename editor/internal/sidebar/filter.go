package sidebar

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/controls"
	ui "github.com/akrck02/go-ui"
)

// listsChangesOnly tells whether the tree lists only the changed files: always in a mode of changes
// only, and in the others when asked.
func (sidebar *Sidebar) listsChangesOnly() bool {
	return sidebar.listing.ChangesOnly() || sidebar.onlyChanged
}

// SetChangedFilter lists only the changed files, or every file again, in a mode that lists every
// file; a mode of changes only lists them anyway.
func (sidebar *Sidebar) SetChangedFilter(on bool) {
	if sidebar.onlyChanged == on {
		return
	}
	sidebar.onlyChanged = on
	sidebar.Stale = true
	sidebar.list.Offset = 0
}

// buildChangedFilter is the button that shows only the changes: the projects with some, and in a
// mode that lists every file, the changed files.
func (sidebar *Sidebar) buildChangedFilter(context *ui.Context, window Window) {
	if controls.ToolButton(context, "Only the changes: projects and files (⌘⇧G)", "file-diff", sidebar.onlyChanged) {
		window.ToggleChangedFilter()
	}
}

// filtersFiles tells whether the filter chooses which files of a project are listed: it does, unless
// it is empty or names the project itself, which is then listed whole.
func (sidebar *Sidebar) filtersFiles(window Window, project int) bool {
	return sidebar.Filter != "" && !matchesFilter(sidebar.Filter, window.Projects()[project].Name)
}

// listedByFilter tells whether the filter lets a project be listed: by its name, or by a file of it.
func (sidebar *Sidebar) listedByFilter(window Window, project int) bool {
	if !sidebar.filtersFiles(window, project) {
		return true
	}
	return len(sidebar.tree(window, project).Folders)+len(sidebar.tree(window, project).Files) > 0
}

// found are the paths the filter finds.
func (sidebar *Sidebar) found(paths []string) []string {
	var kept []string
	for _, path := range paths {
		if matchesFilter(sidebar.Filter, path) {
			kept = append(kept, path)
		}
	}
	return kept
}

// matchesFilter tells whether a text holds each word of a filter, whatever their case.
func matchesFilter(filter, text string) bool {
	text = strings.ToLower(text)
	for _, word := range strings.Fields(strings.ToLower(filter)) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// SetFilter lists only what a filter finds, or everything again for an empty one.
func (sidebar *Sidebar) SetFilter(filter string) {
	sidebar.Filter = filter
	sidebar.Selected = 0
	sidebar.list.Offset = 0
	sidebar.Stale = true
}

// buildFilter is the field that filters the files listed by their path: Enter takes the keyboard to
// the first file found, Esc leaves the field as it is.
func (sidebar *Sidebar) buildFilter(context *ui.Context, window Window) {
	palette := context.Theme
	pill := ui.Look{Fill: palette.Sunken, Radius: 6}
	context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.Padding{Left: headerPadding, Right: headerPadding, Bottom: 6}}, func() {
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(8, 0), Gap: 6, AlignY: ui.Center}, pill, func() {
			context.IconSized("search", 14, palette.InkQuiet)
			if sidebar.FilterAsked {
				context.FocusField("filter")
				sidebar.FilterAsked = false
			}
			filter := sidebar.Filter
			submitted := context.EntryField("filter", &filter, ui.TextFieldOptions{Placeholder: "Filter projects and files (⌥F)", Plain: true,
				Width: ui.Grow(), Size: palette.SmallTextSize})
			if filter != sidebar.Filter {
				sidebar.SetFilter(filter)
			}
			if sidebar.Filter != "" && controls.ToolButton(context, "Clear the filter", "x", false) {
				sidebar.SetFilter("")
			}
			if submitted {
				sidebar.Rebuild(window)
				sidebar.SelectFirstFile()
				window.FocusSidebar()
			}
		})
	})
}
