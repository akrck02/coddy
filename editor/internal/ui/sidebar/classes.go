package sidebar

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/format/highlight"
	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
)

// classRoles are the code roles whose colour each kind of class is drawn in, as IntelliJ tints
// its letters.
var classRoles = map[java.ClassKind]highlight.Role{
	java.ClassDeclaration:      highlight.Function,
	java.InterfaceDeclaration:  highlight.String,
	java.EnumDeclaration:       highlight.Number,
	java.AnnotationDeclaration: highlight.String,
	java.RecordDeclaration:     highlight.Keyword,
	java.ObjectDeclaration:     highlight.TypeName,
}

// paintClassBadge draws a class's kind as IntelliJ does: its letter in a round badge.
func paintClassBadge(drawing *draw2d.List, look theme.Look, kind java.ClassKind, x, y, size float32, quiet bool) {
	colour := look.Syntax.RoleColour(classRoles[kind])
	if quiet {
		colour = colour.WithAlpha(0.5)
	}
	badge := draw2d.Rectangle{X: x, Y: y, Width: size, Height: size}
	drawing.Rectangle(badge, size/2, colour.WithAlpha(colour[3]*0.22), colour, 1.2)
	letterSize := size * 0.7
	drawing.Text(kind.Letter(), x+size*0.5-letterSize*0.3, y+size*0.5+letterSize*0.36, letterSize, draw2d.Bold, colour)
}

// folderIcon is a folder's icon and its tint: by its role in a Java project, as IntelliJ marks
// source roots blue, test roots green and resources apart.
func folderIcon(row Row, look theme.Look) (string, draw2d.Color) {
	quiet := look.Theme.InkQuiet
	open := row.Expanded
	switch row.Role {
	case codebase.ModuleFolder:
		return "package", look.Syntax.RoleColour(highlight.Function)
	case codebase.SourceRoot:
		return "folder-code", look.Syntax.RoleColour(highlight.Function)
	case codebase.TestRoot:
		return "folder-code", look.Syntax.RoleColour(highlight.String)
	case codebase.ResourceRoot:
		return "folder-cog", look.Syntax.RoleColour(highlight.Number)
	case codebase.TestResourceRoot:
		return "folder-cog", look.Syntax.RoleColour(highlight.String)
	case codebase.PackageFolder:
		return "folder-dot", quiet
	}
	if open {
		return "folder-open", quiet
	}
	return "folder", quiet
}
