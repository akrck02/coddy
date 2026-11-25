package uses

import (
	"fmt"
	"path"
	"sort"

	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/workspace"
)

// opened is a file of the name graph opened a level further: the declaration a use of it is in, a
// method or a class, and who uses that declaration.
type opened struct {
	declaration string
	users       []mode.Use
	asking      bool
}

const callerGap = 120 // between a file and the files that use its declaration

// open opens a file of the name graph a level further, asking its server who uses the declaration
// a use of it is in; or closes it again.
func (uses *Mode) open(file workspace.FileReference) {
	uses.forgetNameScene()
	if uses.graphs.opened[file] != nil {
		delete(uses.graphs.opened, file)
		uses.graphs.name.fitted = false
		return
	}
	host, can := uses.window.(Host)
	if !can || len(uses.byFile[file]) == 0 {
		return
	}
	if uses.graphs.opened == nil {
		uses.graphs.opened = map[workspace.FileReference]*opened{}
	}
	book := uses.graphs.opened // a new search starts another
	book[file] = &opened{asking: true}
	asked := host.UsersOf(uses.askedUse(file), func(declaration string, users []mode.Use) {
		uses.forgetNameScene()
		if declaration == "" {
			delete(book, file)
			return
		}
		book[file] = &opened{declaration: declaration, users: users}
		uses.graphs.name.fitted = false
		if len(users) == 0 {
			uses.window.Toast("No one else uses " + declaration)
		}
	})
	if !asked {
		delete(book, file)
	}
}

// askedUse is the use of a file to ask about: the one n went to, if it is there, or else its last,
// as the first is most often an import, in no declaration.
func (uses *Mode) askedUse(file workspace.FileReference) mode.Use {
	if uses.current >= 0 && uses.current < len(uses.uses) && uses.uses[uses.current].File == file {
		return uses.uses[uses.current]
	}
	indexes := uses.byFile[file]
	return uses.uses[indexes[len(indexes)-1]]
}

// fileLabel is a file box's label: its name, and the declaration it was opened on.
func (uses *Mode) fileLabel(file workspace.FileReference) string {
	label := path.Base(file.Path)
	switch further := uses.graphs.opened[file]; {
	case further == nil:
	case further.asking:
		label += " › …"
	default:
		label += " › " + further.declaration
	}
	return label
}

// placeCallers lays out, beside a file box, the files that use the declaration it was opened on,
// one box each with its count, an arrow from each to the file; below is how far down the outer
// column is filled, and is moved on.
func (uses *Mode) placeCallers(graph *scene, fileBox int, side int, below *float32) {
	file := uses.uses[graph.boxes[fileBox].target].File
	further := uses.graphs.opened[file]
	if further == nil || further.asking {
		return
	}
	files, byFile := callerFiles(further.users)
	beside := graph.boxes[fileBox]
	y := max(*below, beside.y)
	for _, caller := range files {
		label := path.Base(caller.Path)
		if caller.Project != file.Project {
			label = uses.window.Projects()[caller.Project].Name + " · " + label
		}
		width := max(180, textWidth(label)+70)
		x := beside.x + beside.width + callerGap
		if side == 0 {
			x = beside.x - callerGap - width
		}
		graph.boxes = append(graph.boxes, box{x: x, y: y, width: width, height: fileHeight, label: label,
			detail: fmt.Sprint(len(byFile[caller])), kind: plainBox, target: -1, reveals: true, use: byFile[caller][0]})
		graph.arrows = append(graph.arrows, arrow{from: len(graph.boxes) - 1, to: fileBox})
		y += fileHeight + 6
	}
	*below = y + 6
}

// callerFiles are the files of some uses, by project and path, and the uses of each.
func callerFiles(users []mode.Use) ([]workspace.FileReference, map[workspace.FileReference][]mode.Use) {
	byFile := map[workspace.FileReference][]mode.Use{}
	var files []workspace.FileReference
	for _, user := range users {
		if byFile[user.File] == nil {
			files = append(files, user.File)
		}
		byFile[user.File] = append(byFile[user.File], user)
	}
	sort.Slice(files, func(first, second int) bool {
		one, other := files[first], files[second]
		return one.Project < other.Project || one.Project == other.Project && one.Path < other.Path
	})
	return files, byFile
}
