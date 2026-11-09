package sidebar

import (
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Listing is what the sidebar asks the mode that is on: what to call the list, which files to
// list, and what each row shows besides its name.
type Listing interface {
	Name() string      // the sidebar's title
	ChangesOnly() bool // lists only the files the development changed
	Header() string    // words beside the title, such as "3 of 7 viewed"
	Decorate(row Row) Decoration
	// Press acts on a file's mark, such as marking it as viewed; moveOn goes on to the next file.
	Press(file workspace.FileReference, moveOn bool)
}

// Chooser is a listing that lists files of its own, not the projects': the files of a search, say.
// A project with none of them is left out, and the projects come in the order it gives.
type Chooser interface {
	// ListedFiles are the files of a project to list, when the listing chooses them (own).
	ListedFiles(project int) (paths []string, own bool)
	// ProjectOrder is the order of the projects, by index; nil keeps the window's.
	ProjectOrder() []int
}

// Mark is what a file row shows before its icon.
type Mark int

const (
	NoMark   Mark = iota
	OpenMark      // an empty circle: still to do
	DoneMark      // a ticked circle, the name quiet: done
)

// Ink is a colour of the theme a decoration is drawn in.
type Ink int

const (
	QuietInk Ink = iota
	WarningInk
	PositiveInk
	DangerInk
)

func (ink Ink) colour(palette ui.Theme) draw2d.Color {
	switch ink {
	case WarningInk:
		return palette.Warning
	case PositiveInk:
		return palette.Positive
	case DangerInk:
		return palette.Danger
	}
	return palette.InkQuiet
}

// Word is a short text on the right of a row.
type Word struct {
	Text string
	Ink  Ink
}

// Progress is a small bar of how much of a project is done.
type Progress struct {
	Done, Total int
	Ink         Ink
}

// Decoration is what a row shows besides its name.
type Decoration struct {
	Mark     Mark
	Tinted   bool      // the name in its status's colour
	Words    []Word    // on the right, from the right edge leftwards
	Progress *Progress // left of the words
	Dot      bool      // a folder holding changes
}
