// Package explorer shows a folder of a project as a file manager does: what it holds as tiles,
// a folder opened by choosing it, a file shown in the viewer's place.
package explorer

import (
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
				if explorer.buildTile(context, window, name, folderIcon(window.Look(), name)) {
					explorer.Show(window, explorer.Project, joined(explorer.Path, name))
				}
			}
			for _, name := range listing.Files {
				if explorer.buildTile(context, window, name, theme.FileIcon(name, window.Look())) {
					window.OpenFile(workspace.FileReference{Project: explorer.Project, Path: joined(explorer.Path, name)})
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

// folderIcon is a closed folder's icon: the icon theme's for its name, else Lucide's.
func folderIcon(look theme.Look, name string) theme.Icon {
	icon := theme.Icon{Name: "folder", Ink: look.Theme.InkQuiet, Opacity: 1}
	if look.Icons != nil {
		icon.Picture = look.Icons.FolderPicture(name, false)
	}
	return icon
}

// joined is the path of an entry of a folder.
func joined(folder, name string) string {
	if folder == "" {
		return name
	}
	return folder + "/" + name
}

// BuildTitle names the folder as a breadcrumb: its project, then each folder down to it, every one
// but the last a button that shows it. A path too long for the bar is cut at its end.
func (explorer *Explorer) BuildTitle(context *ui.Context, window Window) {
	palette := context.Theme
	paths := []string{""}
	if explorer.Path != "" {
		parts := strings.Split(explorer.Path, "/")
		for index := range parts {
			paths = append(paths, strings.Join(parts[:index+1], "/"))
		}
	}
	project := window.Projects()[explorer.Project]
	context.Frame(ui.Layout{Width: ui.Grow(), Gap: 6, AlignY: ui.Center}, ui.Look{Clip: true}, func() {
		for index, path := range paths {
			label := project.Name
			if path != "" {
				_, label, _ = cutLast(path)
			}
			if index == len(paths)-1 {
				context.Text(label, ui.TextStyle{Weight: draw2d.Medium})
				return
			}
			explorer.buildCrumb(context, window, label, path)
			context.IconSized("chevron-right", 12, palette.InkQuiet.WithAlpha(0.6))
		}
	})
}

// buildCrumb is a folder of the breadcrumb, a button that shows it.
func (explorer *Explorer) buildCrumb(context *ui.Context, window Window, label, path string) {
	hovered := ui.Look{Fill: context.Theme.Sunken, Radius: 4}
	name := "Show the folder " + path
	if path == "" {
		name = "Show the top of " + label
	}
	if context.Clickable(name, ui.Layout{Padding: ui.PadXY(3, 1), AlignY: ui.Center}, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		context.Text(label, ui.TextStyle{Color: context.Theme.InkQuiet})
	}) {
		explorer.Show(window, explorer.Project, path)
	}
	context.Tooltip(name)
}

// cutLast splits a path at its last "/": the folder before it and the name after it.
func cutLast(path string) (folder, name string, found bool) {
	at := strings.LastIndexByte(path, '/')
	if at < 0 {
		return "", path, false
	}
	return path[:at], path[at+1:], true
}
