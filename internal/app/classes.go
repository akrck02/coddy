package app

import (
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/draw2d"
)

// classHead is how much of a source file is read to find what it declares: the package, the
// imports and the declaration fit in it nearly always.
const classHead = 24 * 1024

// classKind is what a Java or Kotlin file declares under its own name, once read. A file not read
// yet is asked for, to be read in the background with the others asked for in the same frame.
func (application *App) classKind(reference fileReference) review.ClassKind {
	project := application.projects[reference.project]
	if !review.IsClassFile(reference.path) {
		return review.UnknownKind
	}
	if change, changed := project.ChangeOf(reference.path); changed && change.Status == git.Deleted {
		return review.UnknownKind // not on disk to read
	}
	if kind, known := application.classKinds[reference]; known {
		return kind
	}
	if application.settings.Synchronous { // one frame only: read it now
		kind := review.NoClass
		if content, err := git.ReadWorkingFile(project.Path, reference.path); err == nil {
			kind = review.KindOf(reference.path, content[:min(len(content), classHead)])
		}
		application.classKinds[reference] = kind
		return kind
	}
	application.classKinds[reference] = review.UnknownKind
	application.kindsWanted = append(application.kindsWanted, reference)
	return review.UnknownKind
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
		paths[index] = application.projects[reference.project].Path
	}
	application.later(func() func() {
		kinds := make([]review.ClassKind, len(wanted))
		for index, reference := range wanted {
			content, err := git.ReadWorkingFile(paths[index], reference.path)
			if err != nil {
				kinds[index] = review.NoClass
				continue
			}
			kinds[index] = review.KindOf(reference.path, content[:min(len(content), classHead)])
		}
		return func() {
			for index, reference := range wanted {
				if reference.project < len(application.projects) && application.projects[reference.project].Path == paths[index] {
					application.classKinds[reference] = kinds[index]
				}
			}
		}
	})
}

// forgetClassKinds forgets what a project's files declare, when it is read again.
func (application *App) forgetClassKinds(path string) {
	for reference := range application.classKinds {
		if reference.project >= len(application.projects) || application.projects[reference.project].Path == path {
			delete(application.classKinds, reference)
		}
	}
}

// classLetters are the letters IntelliJ marks each kind of class with, and the colour of the code
// role each is drawn in.
var classLetters = map[review.ClassKind]struct {
	letter string
	role   highlight.Role
}{
	review.ClassDeclaration:      {"C", highlight.Function},
	review.InterfaceDeclaration:  {"I", highlight.String},
	review.EnumDeclaration:       {"E", highlight.Number},
	review.AnnotationDeclaration: {"@", highlight.String},
	review.RecordDeclaration:     {"R", highlight.Keyword},
	review.ObjectDeclaration:     {"O", highlight.TypeName},
}

// paintClassBadge draws a class's kind as IntelliJ does: its letter in a round badge.
func paintClassBadge(drawing *draw2d.List, look look, kind review.ClassKind, x, y, size float32, quiet bool) {
	mark := classLetters[kind]
	colour := look.syntax.RoleColour(mark.role)
	if quiet {
		colour = colour.WithAlpha(0.5)
	}
	badge := draw2d.Rectangle{X: x, Y: y, Width: size, Height: size}
	drawing.Rectangle(badge, size/2, colour.WithAlpha(colour[3]*0.22), colour, 1.2)
	letterSize := size * 0.7
	drawing.Text(mark.letter, x+size*0.5-letterSize*0.3, y+size*0.5+letterSize*0.36, letterSize, draw2d.Bold, colour)
}

// folderIcon is a folder's icon and its tint: by its role in a Java project, as IntelliJ marks
// source roots blue, test roots green and resources apart.
func folderIcon(row treeRow, look look) (string, draw2d.Color) {
	quiet := look.theme.InkQuiet
	open := row.expanded
	switch row.role {
	case review.ModuleFolder:
		return "package", look.syntax.RoleColour(highlight.Function)
	case review.SourceRoot:
		return "folder-code", look.syntax.RoleColour(highlight.Function)
	case review.TestRoot:
		return "folder-code", look.syntax.RoleColour(highlight.String)
	case review.ResourceRoot:
		return "folder-cog", look.syntax.RoleColour(highlight.Number)
	case review.TestResourceRoot:
		return "folder-cog", look.syntax.RoleColour(highlight.String)
	case review.PackageFolder:
		return "folder-dot", quiet
	}
	if open {
		return "folder-open", quiet
	}
	return "folder", quiet
}
