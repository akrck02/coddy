package app

import (
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
)

// classKind is what a Java or Kotlin file declares under its own name, once read. A file not read
// yet is asked for, to be read in the background with the others asked for in the same frame.
func (application *App) classKind(reference workspace.FileReference) java.ClassKind {
	project := application.projects[reference.Project]
	if !java.IsClassFile(reference.Path) {
		return java.UnknownKind
	}
	if change, changed := project.ChangeOf(reference.Path); changed && change.Status == git.Deleted {
		return java.UnknownKind // not on disk to read
	}
	if kind, known := application.classKinds[reference]; known {
		return kind
	}
	if application.settings.Synchronous { // one frame only: read it now
		kind := java.ReadKind(project.Path, reference.Path)
		application.classKinds[reference] = kind
		return kind
	}
	application.classKinds[reference] = java.UnknownKind
	application.kindsWanted = append(application.kindsWanted, reference)
	return java.UnknownKind
}

// readClassKinds reads, on another goroutine, the files whose kind the last frame asked for.
func (application *App) readClassKinds() {
	if len(application.kindsWanted) == 0 {
		return
	}
	wanted := application.kindsWanted
	application.kindsWanted = nil
	paths := make([]string, len(wanted))
	for index, reference := range wanted {
		paths[index] = application.projects[reference.Project].Path
	}
	application.work.run("", func(cancellation) func() {
		kinds := make([]java.ClassKind, len(wanted))
		for index, reference := range wanted {
			kinds[index] = java.ReadKind(paths[index], reference.Path)
		}
		return func() {
			for index, reference := range wanted {
				if reference.Project < len(application.projects) && application.projects[reference.Project].Path == paths[index] {
					application.classKinds[reference] = kinds[index]
				}
			}
		}
	})
}

// forgetClassKinds forgets what a project's files declare, when it is read again.
func (application *App) forgetClassKinds(path string) {
	for reference := range application.classKinds {
		if reference.Project >= len(application.projects) || application.projects[reference.Project].Path == path {
			delete(application.classKinds, reference)
		}
	}
}

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
func folderIcon(row treeRow, look theme.Look) (string, draw2d.Color) {
	quiet := look.Theme.InkQuiet
	open := row.expanded
	switch row.role {
	case workspace.ModuleFolder:
		return "package", look.Syntax.RoleColour(highlight.Function)
	case workspace.SourceRoot:
		return "folder-code", look.Syntax.RoleColour(highlight.Function)
	case workspace.TestRoot:
		return "folder-code", look.Syntax.RoleColour(highlight.String)
	case workspace.ResourceRoot:
		return "folder-cog", look.Syntax.RoleColour(highlight.Number)
	case workspace.TestResourceRoot:
		return "folder-cog", look.Syntax.RoleColour(highlight.String)
	case workspace.PackageFolder:
		return "folder-dot", quiet
	}
	if open {
		return "folder-open", quiet
	}
	return "folder", quiet
}
