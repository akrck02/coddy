package controls

import (
	"github.com/akrck02/coddy/editor/internal/theme"
	"path"

	"github.com/akrck02/coddy/editor/internal/icons"
	"github.com/akrck02/go-renderer/draw2d"
)

// FileIcon is the icon a file, by its name or its path, is drawn with: the icon theme's picture,
// when the look has one, and Lucide's icon tinted by the code's colours, by the kind of file.
func FileIcon(name string, look theme.Look) icons.Icon {
	icon := icons.Icon{Name: "file", Ink: look.Theme.InkQuiet, Opacity: 1}
	if kind, found := icons.KindOf(name); found {
		icon.Name, icon.Ink = kind.Icon, look.Syntax.RoleColour(kind.Role).WithAlpha(0.85)
	}
	if look.Icons != nil {
		icon.Picture = look.Icons.FilePicture(path.Base(name)) // Zed matches the file's name, not its folders
	}
	return icon
}

// FolderIcon is the icon a folder is drawn with, open or closed, the twin of FileIcon: the icon
// theme's picture for its name, when the look has one, and Lucide's folder.
func FolderIcon(name string, open bool, look theme.Look) icons.Icon {
	icon := icons.Icon{Name: "folder", Ink: look.Theme.InkQuiet, Opacity: 1}
	if open {
		icon.Name = "folder-open"
	}
	if look.Icons != nil {
		icon.Picture = look.Icons.FolderPicture(name, open)
	}
	return icon
}

// pictureDensity is how many pixels of an icon's picture a layout pixel holds: two, so that it
// stays sharp on a screen of twice the density, as Go Renderer draws its strokes.
const pictureDensity = 2

// DrawIcon draws an icon in a square of a size, at x and y: its picture, or Lucide's icon when it
// has none or it cannot be drawn.
func DrawIcon(list *draw2d.List, look theme.Look, icon icons.Icon, x, y, size float32) {
	if icon.Picture != "" && look.Pictures != nil {
		if picture := look.Pictures.Picture(icon.Picture, int(size)*pictureDensity); picture != nil {
			list.Image(picture, draw2d.Rectangle{X: x, Y: y, Width: size, Height: size}, 0, draw2d.Color{1, 1, 1, icon.Opacity})
			return
		}
	}
	list.Icon(icon.Name, x, y, size, icon.Ink)
}
