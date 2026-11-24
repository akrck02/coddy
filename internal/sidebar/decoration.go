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
	Decorate(row Row) Decoration
}

// Headed is a listing with words beside the title, such as "3 of 7 viewed".
type Headed interface {
	Header() string
}

// Presser is a listing whose file rows have a mark to press, such as marking a file as viewed.
type Presser interface {
	// Press acts on a file's mark; moveOn goes on to the next file.
	Press(file workspace.FileReference, moveOn bool)
}

// DeletedHider is a listing that leaves out the files the development deleted: a mode for reading
// and editing what is there, where a file that is gone is only in the way.
type DeletedHider interface {
	HidesDeleted() bool
}

// Chooser is a listing that lists files of its own, not the projects': the files of a search, say.
// A project with none of them is left out, and the projects come in the order it gives.
type Chooser interface {
	// ListedFiles are the files of a project to list, when the listing chooses them (own).
	ListedFiles(project int) (paths []string, own bool)
	// ProjectOrder is the order of the projects, by index; nil keeps the window's.
	ProjectOrder() []int
}

// listingPowers are what a listing can do besides listing, each nil when it cannot: found once,
// when the listing is set, not at each row.
type listingPowers struct {
	headed  Headed
	presser Presser
	hider   DeletedHider
	chooser Chooser
}

// SetListing makes a listing the one the sidebar lists by, such as the mode turned on.
func (sidebar *Sidebar) SetListing(listing Listing) {
	sidebar.listing = listing
	sidebar.powers = listingPowers{}
	sidebar.powers.headed, _ = listing.(Headed)
	sidebar.powers.presser, _ = listing.(Presser)
	sidebar.powers.hider, _ = listing.(DeletedHider)
	sidebar.powers.chooser, _ = listing.(Chooser)
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
	// Status tells that the word says how files changed, as a count of changed files does: it is
	// hidden with the rest of git's status
	Status bool
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

// withoutStatus is a decoration with what tells how files changed left out: the tint, the dot and
// the words of status.
func (decoration Decoration) withoutStatus() Decoration {
	decoration.Tinted, decoration.Dot = false, false
	var words []Word
	for _, word := range decoration.Words {
		if !word.Status {
			words = append(words, word)
		}
	}
	decoration.Words = words
	return decoration
}
