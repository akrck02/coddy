package viewer

import (
	"fmt"
	"path"
	"strings"

	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

func (view *Viewer) buildEmpty(context *ui.Context, window Window) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), AlignX: ui.Center, AlignY: ui.Center, Gap: 14}, func() {
		context.IconSized("files", 40, palette.InkQuiet.WithAlpha(0.5))
		if !window.HasProjects() {
			context.Text("Open a repository, or a folder of repositories", ui.TextStyle{Color: palette.InkSoft})
			if context.PrimaryIconButton("folder-open", "Open folder") {
				window.OpenFolder()
			}
			return
		}
		context.Box(ui.Layout{Direction: ui.Vertical, Gap: 6}, func() {
			for _, hint := range [][2]string{
				{"Find a file", "⇧⇧"}, {"Next changed file", "⌘⇧N"}, {"Another mode", "⌘⇧0"}, {"Every key", "F1"},
			} {
				context.Box(ui.Layout{Width: ui.Fixed(260), Gap: 12}, func() {
					context.Text(hint[0], ui.TextStyle{Color: palette.InkQuiet})
					context.Spacer()
					context.Text(hint[1], ui.TextStyle{Color: palette.InkSoft, Weight: draw2d.Mono})
				})
			}
		})
	})
}

// buildChange says how the file changed: its status, the path it came from, the lines added and
// gone.
func (view *Viewer) buildChange(context *ui.Context, compact bool) {
	palette := context.Theme
	change := view.Document.Change
	context.Box(ui.Layout{Width: ui.Fixed(4)}, nil)
	if !compact {
		context.Text(statusName(change.Status), ui.TextStyle{Size: palette.SmallTextSize, Color: controls.StatusColour(palette, change.Status)})
	}
	if change.OldPath != change.Path && !compact {
		context.Text("from "+change.OldPath, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
	}
	if view.Added > 0 {
		context.Text(view.added.Of(view.Added, addedWords), ui.TextStyle{Color: palette.Positive, Weight: draw2d.Mono, Size: palette.SmallTextSize})
	}
	if view.Gone > 0 {
		context.Text(view.gone.Of(view.Gone, goneWords), ui.TextStyle{Color: palette.Danger, Weight: draw2d.Mono, Size: palette.SmallTextSize})
	}
}

// buildViewButtons offers the ways to show a changed file: the changes' arrows, folding, the view
// and the mode's button. A narrow view shows the folding's icon without its word.
func (view *Viewer) buildViewButtons(context *ui.Context, window Window, compact bool) {
	context.Box(ui.Layout{Width: ui.Fixed(6)}, nil)
	if len(view.Hunks) > 0 {
		view.buildHunkArrows(context)
	}
	if view.Mode != FileView {
		icon, label, tip := "unfold-vertical", "Expand", "Show every unchanged line (⌘U)"
		if view.Unfolded() {
			icon, label, tip = "fold-vertical", "Fold", "Fold the unchanged lines (⌘U)"
		}
		if compact && controls.ToolButton(context, tip, icon, false) || !compact && controls.LabelledButton(context, tip, icon, label) {
			view.ToggleContext()
		}
	}
	view.buildViewSwitch(context)
	marker, marks := window.(Marker)
	if !marks {
		return
	}
	if button, shown := marker.FileButton(view.Reference); shown {
		if controls.ToolButton(context, button.Tip, button.Icon, button.On) {
			marker.PressFileButton(view.Reference)
		}
	}
}

// buildHunkArrows goes to the previous or the next run of changes, as [ and ] do.
func (view *Viewer) buildHunkArrows(context *ui.Context) {
	if controls.ToolButton(context, "Previous change (⇧F7, [)", "chevron-up", false) {
		view.StepHunk(-1)
	}
	if controls.ToolButton(context, "Next change (F7, ])", "chevron-down", false) {
		view.StepHunk(1)
	}
}

// buildViewSwitch switches between the changes, or the uses, and the whole file.
func (view *Viewer) buildViewSwitch(context *ui.Context) {
	other, choice := DiffView, controls.IconChoice{Icon: "file-diff", Name: "The changes (⌘D)"}
	if len(view.Uses) > 0 {
		other, choice = UsesView, controls.IconChoice{Icon: "file-search", Name: "The uses (⌘D)"}
	}
	chosen := 0
	if view.Mode == FileView {
		chosen = 1
	}
	if controls.IconSwitch(context, "view mode", &chosen, []controls.IconChoice{choice, {Icon: "file-code", Name: "The whole file (⌘D)"}}) {
		if chosen == 1 {
			view.SetMode(FileView)
		} else {
			view.SetMode(other)
		}
	}
}

// splitPath cuts a path into its folder and its name.
func splitPath(slashed string) (folder, name string) {
	folder, name = path.Split(slashed)
	return strings.TrimSuffix(folder, "/"), name
}

func statusName(status git.Status) string {
	switch status {
	case git.Added:
		return "Added"
	case git.Deleted:
		return "Deleted"
	case git.Renamed:
		return "Renamed"
	case git.Copied:
		return "Copied"
	case git.TypeChanged:
		return "Type changed"
	}
	return "Modified"
}

func addedWords(count int) string { return fmt.Sprintf("+%d", count) }
func goneWords(count int) string  { return fmt.Sprintf("−%d", count) }
