// Package sidebar is the tree pane: the projects open, their folders and files, the one selected,
// and the keys that move through them.
package sidebar

import (
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/java"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Window is what the sidebar needs from the window it sits in.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	// ModalOpen tells whether a dialog or the finder is over the window, taking the input.
	ModalOpen() bool
	Projects() []*codebase.Project
	// ProjectIndex is a project's place in Projects, by its path; -1 when it is not open.
	ProjectIndex(path string) int
	// Hidden tells whether a project is left out of the list: one with no changes, while only the
	// projects with changes are shown.
	Hidden(project *codebase.Project) bool
	ShowEveryProject()
	// ToggleChangedFilter shows only the changes, projects and files, or everything again.
	ToggleChangedFilter()
	OpenFile(file codebase.FileReference)
	// ExploreFolder shows a folder chosen in the sidebar in the file's place, "" a project's top.
	ExploreFolder(project int, folder string)
	// OpenedFile is the file the viewer shows, if any.
	OpenedFile() (codebase.FileReference, bool)
	IsOpened(file codebase.FileReference) bool
	// Unsaved tells whether a file has edits not saved.
	Unsaved(file codebase.FileReference) bool
	FocusViewer()
	FocusSidebar()
	SidebarFocused() bool
	// ListEveryFile turns to a mode that lists every file, to show one the development did not
	// change.
	ListEveryFile()
	// Loading tells whether projects are being read.
	Loading() bool
}

// ClassReader is a window that reads what Java and Kotlin files declare, so their rows are drawn
// by their kind of class, as IntelliJ draws them.
type ClassReader interface {
	// ClassKind is what a file declares, once read; asking for one not read yet has it read in
	// the background. ReadingKinds tells whether some are being read.
	ClassKind(file codebase.FileReference) java.ClassKind
	ReadingKinds() bool
}

// ServerFiles is a window whose language servers wrote files of their own in projects.
type ServerFiles interface {
	// ShowServerFiles lists the files the servers wrote in a project, to open any of them.
	ShowServerFiles(project int)
}

// Sidebar is the tree pane: the rows listed, the one selected, and what was opened by hand.
type Sidebar struct {
	// listing is the mode that is on, which says what the sidebar lists and how: the window sets
	// it with SetListing, which finds once what else it can do (powers)
	listing Listing
	powers  listingPowers
	// expanded keeps the folders and projects opened or closed by hand, by key; the others follow
	// the default (everything open when only the changes are listed, closed otherwise).
	expanded map[expansionKey]bool
	// trees are each project's tree as listed, kept until the projects change; filters are the
	// filters whose trees are kept, the oldest first (trees.go)
	trees map[treeKey]*codebase.Folder
	// nesting are the paths of the plain folders whose trees hold the projects found in them, as
	// the rows are made now (nesting.go)
	nesting map[string]bool
	filters []string
	Rows    []Row
	// decorations are what the mode adds to each row, worked out when the rows are, as they only
	// change with them; decoratedWithoutStatus is whether git's status was left out of them
	decorations            []Decoration
	decoratedWithoutStatus bool
	// serverBadges are what each project row's badge says of the files servers wrote in it
	serverBadges []string
	// anyOpen tells whether a project or a folder listed is open, for the fold button
	anyOpen bool
	// rebuilds counts the times the rows were made: see Rebuilds
	rebuilds int
	// header is the listing's words beside the title, which change only with the rows
	header string
	// badges are where each project's badge of server files was painted last, by row
	badges map[int][2]float32
	// leftOut are where each project's mark of being left out of the servers was painted last
	leftOut  map[int][2]float32
	Stale    bool
	Selected int
	list     pane.Scroller
	Width    float32
	// entered is the project the sidebar was taken into, by its path: then only its tree is
	// listed, from its top, as if it were the only project open. Empty lists every project.
	Entered string
	// onlyChanged lists, in a mode that lists every file, only the files the development changed,
	// with the folders opened and closed as that mode has them: the window's filter of changes,
	// which also leaves out the projects with none.
	onlyChanged bool
	// HidesStatus leaves out how files changed (tints, letters, dots and counts) in the modes
	// that list every file, so git does not draw the eye while reading
	HidesStatus bool
	// Filter lists only the files whose path holds each of its words, and the projects named by
	// them, whatever their case; FilterAsked gives its field the keyboard on the next frame.
	Filter      string
	FilterAsked bool
}

// New is a sidebar with nothing listed yet, which lists the projects at its first frame.
func New() Sidebar {
	return Sidebar{expanded: map[expansionKey]bool{}, trees: map[treeKey]*codebase.Folder{}, Stale: true, Width: 380, list: pane.Scroller{RowHeight: RowHeight}}
}

// Invalidate forgets the trees, as the projects changed, and lists them again at the next frame.
func (sidebar *Sidebar) Invalidate() {
	sidebar.trees = map[treeKey]*codebase.Folder{}
	sidebar.filters = sidebar.filters[:0]
	sidebar.Stale = true
}

// RowHeight leaves air around each name: a tree of hundreds of files is read for long.
const RowHeight float32 = 28

const (
	indentWidth   float32 = 14
	treeIconSize  float32 = 15
	sidebarMargin float32 = 10
)

// Build adds the sidebar: a header naming the mode, and the rows.
func (sidebar *Sidebar) Build(context *ui.Context, window Window) {
	if sidebar.Stale {
		sidebar.Rebuild(window)
	}
	palette := context.Theme
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: palette.Surface}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(headerPadding, 0), Gap: headerGap, AlignY: ui.Center}, func() {
			if sidebar.Entered != "" {
				sidebar.buildEnteredHeader(context, window)
				return
			}
			sidebar.buildHeader(context, window)
		})
		sidebar.buildFilter(context, window)
		sidebar.list.RowHeight = RowHeight
		sidebar.list.Count = len(sidebar.Rows)
		if !window.ModalOpen() {
			sidebar.followPointer(context, window)
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			sidebar.list.Place(area)
			sidebar.paint(context, window, drawing)
		})
	})
}

// Rebuilds counts the times the rows were made. They are made again whenever what the listing
// shows changed (projects read, a file marked, a mode or a filter chosen), so what is worked out
// from the same things can be kept until it grows.
func (sidebar *Sidebar) Rebuilds() int { return sidebar.rebuilds }
