// Package explorer shows a folder of a project as a file manager does: what it holds as tiles,
// a folder opened by choosing it, a file shown in the viewer's place.
package explorer

import (
	"path"
	"slices"
	"strings"

	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Window is what the explorer needs of the window around it.
type Window interface {
	Look() theme.Look
	Projects() []*workspace.Project
	OpenFile(file workspace.FileReference)
}

// Explorer is a folder shown in the viewer's place, and what it holds.
type Explorer struct {
	Open    bool
	Project int    // the folder's project, its place in Window.Projects
	Path    string // the folder's path in its project, "" for its top
	listing Listing
}

// Listing is what a folder holds right inside it: its folders' and its files' names, each sorted.
type Listing struct{ Folders, Files []string }

// Show opens a folder of a project in the explorer.
func (explorer *Explorer) Show(window Window, project int, path string) {
	explorer.Open, explorer.Project, explorer.Path = true, project, path
	explorer.Relist(window)
}

// Close goes back to what the viewer shows.
func (explorer *Explorer) Close() { explorer.Open = false }

// Relist reads what the folder holds again, after the project's files changed: it is kept, not
// read each frame.
func (explorer *Explorer) Relist(window Window) {
	projects := window.Projects()
	if !explorer.Open || explorer.Project >= len(projects) {
		explorer.Open = false
		return
	}
	explorer.listing = Contents(projects[explorer.Project].Files, explorer.Path)
}

// Contents is what a folder holds right inside it, from the paths of every file of its project.
func Contents(paths []string, folder string) Listing {
	prefix := ""
	if folder != "" {
		prefix = folder + "/"
	}
	var listing Listing
	for _, path := range paths {
		rest, inside := strings.CutPrefix(path, prefix)
		if !inside {
			continue
		}
		name, _, inFolder := strings.Cut(rest, "/")
		if !inFolder {
			listing.Files = append(listing.Files, name)
			continue
		}
		if !slices.Contains(listing.Folders, name) {
			listing.Folders = append(listing.Folders, name)
		}
	}
	slices.Sort(listing.Folders)
	slices.Sort(listing.Files)
	return listing
}

// tileWidth is a tile's width, its icon and name; iconSize the icon's, both in pixels.
const (
	tileWidth float32 = 112
	iconSize  float32 = 40
)

// Build shows what the folder holds, folders first, as tiles that wrap into rows.
func (explorer *Explorer) Build(context *ui.Context, window Window) {
	context.ScrollArea("explorer", ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), Padding: ui.Pad(24)}, func() {
		listing := explorer.listing
		if len(listing.Folders)+len(listing.Files) == 0 {
			context.Caption("This folder is empty")
			return
		}
		context.Box(ui.Layout{Width: ui.Grow(), Wrap: true, Gap: 8}, func() {
			for _, name := range listing.Folders {
				if explorer.buildTile(context, window, name, theme.FolderIcon(name, false, window.Look())) {
					explorer.Show(window, explorer.Project, path.Join(explorer.Path, name))
				}
			}
			for _, name := range listing.Files {
				if explorer.buildTile(context, window, name, theme.FileIcon(name, window.Look())) {
					window.OpenFile(workspace.FileReference{Project: explorer.Project, Path: path.Join(explorer.Path, name)})
				}
			}
		})
	})
}

// buildTile is an entry of the folder, its icon over its name, and tells whether it was chosen.
func (explorer *Explorer) buildTile(context *ui.Context, window Window, name string, icon theme.Icon) bool {
	palette := context.Theme
	hovered := ui.Look{Fill: palette.Sunken, Radius: 6}
	layout := ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(tileWidth), Padding: ui.Pad(8), Gap: 6, AlignX: ui.Center}
	chosen := context.Clickable("explorer/"+name, layout, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		context.Paint(ui.Layout{Width: ui.Fixed(iconSize), Height: ui.Fixed(iconSize)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			theme.DrawIcon(drawing, window.Look(), icon, area.X, area.Y, iconSize)
		})
		context.Frame(ui.Layout{Width: ui.Grow(), AlignX: ui.Center}, ui.Look{Clip: true}, func() {
			context.Text(name, ui.TextStyle{Size: context.Theme.SmallTextSize})
		})
	})
	context.Describe(ui.Description{Role: ui.ButtonRole, Label: name})
	return chosen
}

// BuildTitle names the folder as a breadcrumb: its project, then each folder down to it, every one
// but the last a button that shows it. A path too long for the bar is cut at its end.
func (explorer *Explorer) BuildTitle(context *ui.Context, window Window) {
	paths := []string{""}
	if explorer.Path != "" {
		parts := strings.Split(explorer.Path, "/")
		for index := range parts {
			paths = append(paths, strings.Join(parts[:index+1], "/"))
		}
	}
	project := window.Projects()[explorer.Project]
	context.Frame(ui.Layout{Width: ui.Grow(), Gap: 6, AlignY: ui.Center}, ui.Look{Clip: true}, func() {
		for index, folder := range paths {
			label := project.Name
			if folder != "" {
				label = path.Base(folder)
			}
			if index == len(paths)-1 {
				context.Text(label, ui.TextStyle{Weight: draw2d.Medium})
				return
			}
			explorer.buildCrumb(context, window, label, folder)
			theme.CrumbChevron(context)
		}
	})
}

// buildCrumb is a folder of the breadcrumb, a button that shows it.
func (explorer *Explorer) buildCrumb(context *ui.Context, window Window, label, path string) {
	name := "Show the folder " + path
	if path == "" {
		name = "Show the top of " + label
	}
	if theme.Crumb(context, label, name) {
		explorer.Show(window, explorer.Project, path)
	}
}
