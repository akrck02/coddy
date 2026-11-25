package window

import (
	"context"

	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// classKind is what a Java or Kotlin file declares under its own name, once read. A file not read
// yet is asked for, to be read in the background with the others asked for in the same frame.
func (application *Window) classKind(reference workspace.FileReference) java.ClassKind {
	project := application.session.Projects[reference.Project]
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
func (application *Window) readClassKinds() {
	if len(application.kindsWanted) == 0 {
		return
	}
	wanted := application.kindsWanted
	application.kindsWanted = nil
	paths := make([]string, len(wanted))
	for index, reference := range wanted {
		paths[index] = application.session.Projects[reference.Project].Path
	}
	application.work.Run("", func(context.Context) func() {
		kinds := make([]java.ClassKind, len(wanted))
		for index, reference := range wanted {
			kinds[index] = java.ReadKind(paths[index], reference.Path)
		}
		return func() {
			for index, reference := range wanted {
				if reference.Project < len(application.session.Projects) && application.session.Projects[reference.Project].Path == paths[index] {
					application.classKinds[reference] = kinds[index]
				}
			}
		}
	})
}

// forgetClassKinds forgets what a project's files declare, when it is read again.
func (application *Window) forgetClassKinds(path string) {
	for reference := range application.classKinds {
		if reference.Project >= len(application.session.Projects) || application.session.Projects[reference.Project].Path == path {
			delete(application.classKinds, reference)
		}
	}
}
