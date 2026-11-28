package theme

import (
	"path"
	"strings"

	"github.com/akrck02/coddy/editor/internal/format/highlight"
	"github.com/akrck02/coddy/editor/internal/language"
	"github.com/akrck02/go-renderer/draw2d"
)

// Kind is how a kind of file is drawn with Lucide: an icon for its shape, and a tint taken from
// the code's colours (by role), so the tints follow the light or dark theme and stay soft.
type Kind struct {
	Icon string
	Role highlight.Role
}

// otherKinds are the kinds of file that are not a language, by extension: languages are in the
// language table.
var otherKinds = map[string]Kind{
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
var namedKinds = map[string]Kind{
	"gradlew": {"file-terminal", highlight.String}, ".gitignore": {"git-branch", highlight.Comment},
	".gitattributes": {"git-branch", highlight.Comment}, ".gitmodules": {"git-branch", highlight.Comment},
	"license": {"scroll-text", highlight.Comment}, "package.json": {"package", highlight.String},
	"go.mod": {"package", highlight.TypeName}, "go.sum": {"file-lock", highlight.Comment},
	"pom.xml": {"package", highlight.Property}, ".editorconfig": {"file-cog", highlight.Comment},
}

// KindOf is how a file, by its name or its path, is drawn with Lucide: by its whole name, as a
// readme, by its language, or by its extension; false for a file of no known kind.
func KindOf(name string) (Kind, bool) {
	lower := strings.ToLower(path.Base(name))
	if kind, found := namedKinds[lower]; found {
		return kind, true
	}
	if strings.HasPrefix(lower, "readme") {
		return Kind{"book-open", highlight.Attribute}, true
	}
	if known, found := language.Of(lower); found && known.Icon != "" {
		return Kind{known.Icon, highlight.Role(known.Tint)}, true
	}
	kind, found := otherKinds[path.Ext(lower)]
	return kind, found
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
