package theme

import (
	"path"
	"strings"

	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/language"
	"github.com/akrck02/go-renderer/draw2d"
)

// fileKind is how a kind of file is drawn in a list: an icon for its shape, and a tint taken from
// the code's colours (by role), so the tints follow the light or dark theme and stay soft.
type fileKind struct {
	icon string
	role highlight.Role
}

// otherKinds are the kinds of file that are not a language, by extension: languages are in the
// language table.
var otherKinds = map[string]fileKind{
	".svg": {"file-image", highlight.Constant}, ".pdf": {"file-text", highlight.Property}, ".csv": {"file-spreadsheet", highlight.String},
	".png": {"file-image", highlight.Keyword}, ".jpg": {"file-image", highlight.Keyword}, ".jpeg": {"file-image", highlight.Keyword},
	".gif": {"file-image", highlight.Keyword}, ".webp": {"file-image", highlight.Keyword}, ".ico": {"file-image", highlight.Keyword},
	".ttf": {"file-type", highlight.Comment}, ".otf": {"file-type", highlight.Comment}, ".woff": {"file-type", highlight.Comment},
	".woff2": {"file-type", highlight.Comment},
	".zip":   {"file-archive", highlight.Comment}, ".jar": {"file-archive", highlight.Number}, ".gz": {"file-archive", highlight.Comment},
	".lock": {"file-lock", highlight.Comment}, ".pem": {"file-key", highlight.Constant}, ".key": {"file-key", highlight.Constant},
	".mp4": {"file-video", highlight.Keyword}, ".mp3": {"file-audio", highlight.Keyword}, ".wav": {"file-audio", highlight.Keyword},
}

// namedKinds are files known by their whole name that are not a language.
var namedKinds = map[string]fileKind{
	"gradlew": {"file-terminal", highlight.String}, ".gitignore": {"git-branch", highlight.Comment},
	".gitattributes": {"git-branch", highlight.Comment}, ".gitmodules": {"git-branch", highlight.Comment},
	"license": {"scroll-text", highlight.Comment}, "package.json": {"package", highlight.String},
	"go.mod": {"package", highlight.TypeName}, "go.sum": {"file-lock", highlight.Comment},
	"pom.xml": {"package", highlight.Property}, ".editorconfig": {"file-cog", highlight.Comment},
}

// Icon is what a file or a folder is drawn with: a picture of the icon theme chosen, or Lucide's
// icon in a tint, which also stands in for a picture that cannot be drawn.
type Icon struct {
	// Name is Lucide's icon, and Ink its tint.
	Name string
	Ink  draw2d.Color
	// Picture is the SVG file of the icon theme's picture; "" without an icon theme.
	Picture string
	// Opacity fades the picture, as the ink's own opacity fades Lucide's icon.
	Opacity float32
}

// Faded is the icon drawn fainter, for a file that matters less (one git ignores).
func (icon Icon) Faded() Icon {
	icon.Ink, icon.Opacity = icon.Ink.WithAlpha(0.5), icon.Opacity*0.5
	return icon
}

// FileIcon is the icon a file, by its name or its path, is drawn with: the icon theme's picture,
// when the look has one, and Lucide's icon tinted by the code's colours, by the kind of file.
func FileIcon(name string, look Look) Icon {
	icon := Icon{Name: "file", Ink: look.Theme.InkQuiet, Opacity: 1}
	if kind, found := kindOf(name); found {
		icon.Name, icon.Ink = kind.icon, look.Syntax.RoleColour(kind.role).WithAlpha(0.85)
	}
	if look.Icons != nil {
		icon.Picture = look.Icons.FilePicture(path.Base(name)) // Zed matches the file's name, not its folders
	}
	return icon
}

// pictureDensity is how many pixels of an icon's picture a layout pixel holds: two, so that it
// stays sharp on a screen of twice the density, as Go Renderer draws its strokes.
const pictureDensity = 2

// DrawIcon draws an icon in a square of a size, at x and y: its picture, or Lucide's icon when it
// has none or it cannot be drawn.
func DrawIcon(list *draw2d.List, look Look, icon Icon, x, y, size float32) {
	if icon.Picture != "" && look.Pictures != nil {
		if picture := look.Pictures.Picture(icon.Picture, int(size)*pictureDensity); picture != nil {
			list.Image(picture, draw2d.Rectangle{X: x, Y: y, Width: size, Height: size}, 0, draw2d.Color{1, 1, 1, icon.Opacity})
			return
		}
	}
	list.Icon(icon.Name, x, y, size, icon.Ink)
}

// kindOf is how a file is drawn: by its whole name, as a readme, by its language, or by its
// extension.
func kindOf(name string) (fileKind, bool) {
	lower := strings.ToLower(path.Base(name))
	if kind, found := namedKinds[lower]; found {
		return kind, true
	}
	if strings.HasPrefix(lower, "readme") {
		return fileKind{"book-open", highlight.Attribute}, true
	}
	if known, found := language.Of(lower); found && known.Icon != "" {
		return fileKind{known.Icon, highlight.Role(known.Tint)}, true
	}
	kind, found := otherKinds[path.Ext(lower)]
	return kind, found
}

// FolderIcon is the icon a folder is drawn with, open or closed, the twin of FileIcon: the icon
// theme's picture for its name, when the look has one, and Lucide's folder.
func FolderIcon(name string, open bool, look Look) Icon {
	icon := Icon{Name: "folder", Ink: look.Theme.InkQuiet, Opacity: 1}
	if open {
		icon.Name = "folder-open"
	}
	if look.Icons != nil {
		icon.Picture = look.Icons.FolderPicture(name, open)
	}
	return icon
}
