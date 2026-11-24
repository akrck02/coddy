// Package viewer is the view of one file both modes share: a header naming it, and its text, as
// the changes or as the whole file, with a gutter, folds and marks.
package viewer

import (
	"context"
	"time"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/text"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// Mode is how the view shows a file.
type Mode int

const (
	DiffView Mode = iota // the changes, both sides interleaved
	FileView             // the file as it is now
	UsesView             // the file as it is now, its uses of a name marked and the rest folded
)

// foldContext is how many unchanged lines are kept around a change before the rest is folded.
const foldContext = 3

// Window is what the view needs from the window it sits in.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	// ModalOpen tells whether a dialog or the finder is over the window, taking the input.
	ModalOpen() bool
	FocusViewer()
	HasProjects() bool
	OpenFolder()
	// ShowFolder shows what a folder of a project holds in the viewer's place, "" its top.
	ShowFolder(project int, folder string)
	// Editing tells whether the keyboard is in the file being edited; ReadOnly whether editing is
	// off for every file.
	Editing() bool
	ReadOnly() bool
	// ShowsStatus tells whether git's status is shown: when not, a file read whole says nothing of
	// how it changed (a diff still shows its changes, being made of them).
	ShowsStatus() bool
	// Run starts background work under a key, replacing the work under that key still running.
	Run(key string, do func(stop context.Context) func())
}

// Marker is a window that puts a button of its own in the header for a file, such as Viewed.
type Marker interface {
	// FileButton is the button for a file, if it has one; PressFileButton presses it.
	FileButton(file workspace.FileReference) (button Button, shown bool)
	PressFileButton(file workspace.FileReference)
}

// Navigator is a window that knows where names are declared and what they are, as language
// servers do.
type Navigator interface {
	// GoToDefinition opens where the name at a place of the file open is declared.
	GoToDefinition(place Place)
	// Hover shows what the server says of the name at a place while the pointer rests on it
	// (resting), and stops showing it when the pointer leaves (not resting).
	Hover(place Place, resting bool)
}

// Page is a window with something of its own to show while no file is open.
type Page interface {
	// BuildInsteadOfAFile builds it and tells whether it had anything; if not, the view shows
	// how to start.
	BuildInsteadOfAFile(context *ui.Context) bool
}

// Button is a button of the header a mode adds, such as Viewed: its icon, its tooltip, and
// whether it is on.
type Button struct {
	Icon, Tip string
	On        bool
}

// codeRowHeight is a line of code: 13 pixels of a monospaced face on lines 1.7 times as tall, as roomy as
// Zed's buffer.
const codeRowHeight float32 = 22

// codeMargin is the room above the first line of code and below the last, in pixels, so the code
// does not touch the bar above it or the window's edge below.
const codeMargin float32 = 10

// Viewer is the view of the file open: what it shows, how (its Mode and the presentation chosen
// for it), where it is scrolled to, and the file being edited.
type Viewer struct {
	Reference workspace.FileReference
	Project   *workspace.Project
	Document  *workspace.Document
	Request   int // counts the files asked for, so a slow read that was overtaken is dropped
	showing   bool

	Mode Mode
	// shown is the presentation of the mode, chosen when the mode or the file changes; scrollMarks
	// are its marks along the scroll bar, worked out with its rows
	shown       presentation
	scrollMarks barMarks
	// unfolded are the views that show every unchanged line instead of folding them: the changes,
	// unless folded by hand; the uses fold the lines away from them
	unfolded    map[Mode]bool
	openedFolds map[int]bool // folds opened by hand, by where they start in the diff

	rows []workspace.Row // the diff's rows as shown
	diff []git.DiffLine  // the diff the rows show: the file's, or the file unchanged for its uses
	// Uses are where the file uses a name searched, by line from 1, as spans of characters: shown
	// in the uses view; nil when the mode that is on searches none
	Uses  map[int][]workspace.Span
	Hunks []int
	Added int
	Gone  int
	// inline are the words that changed inside a replaced line, by row of the diff
	Inline map[int]workspace.Span
	// goTo is a line to show once the file is read, from the finder; markedLine is the line shown
	// that way, painted as the line the reader is on
	GoTo       int
	GoToColumn int // the character of the line GoTo to put the caret at, from 0
	MarkedLine int
	// marks say, in the file view, which lines the development added or changed and where it
	// removed some, as Zed marks them in its gutter
	Marks map[int]workspace.Mark

	List pane.Scroller
	// crumbs are the breadcrumb laid out last, kept while the path and the room stay
	crumbs keptCrumbs
	// markdown tells whether the file open is Markdown, known once it is chosen
	markdown bool
	// measuredSize is the code's size the character was measured at; numbers and foldWords are
	// the words of line numbers and folds, made once each, as painting needs them every frame
	measuredSize float32
	numbers      map[int]string
	foldWords    map[foldLabel]string
	added, gone  pane.Words[int]

	// formatted shows a Markdown file as it reads, on from when one opens; formatting is its text
	// laid out last
	formatted  bool
	formatting *formatting

	// Edit is the file being edited, when it can be (see Editable); nil otherwise
	Edit *text.Buffer
	// coloured are the buffer's lines as last coloured, for its version colouredVersion
	coloured        []highlight.Line
	colouredVersion int
	indent          string // what Tab types: a tab, or spaces, as the file indents
	// editedFrom is what the file was when the edits started from it: saving checks the disk
	// against it, not against a file read again since
	editedFrom workspace.Stamp
	// reopening is the file read again where it is (Reopen), keeping its view
	reopening bool
	selecting bool // a press in the code is dragging a selection
	// ColourPause is how long typing must stop before the file is coloured again
	ColourPause time.Duration
	// HoverPause is how long the pointer rests on a name before the window is asked about it
	HoverPause time.Duration
	rest       resting
	// clicked is the place of the code clicked last, in any view: what ⌘B and ⌥F7 ask about
	// when there is no caret
	clicked     Place
	clickedSome bool
	// codeLeft and codeWidth are where the code was painted last, to find what a click is on
	codeLeft, codeWidth float32

	codeSize    float32
	rowHeight   float32
	zoomWheel   float32 // the wheel turned with Command, not yet a step of zoom
	charWidth   float32
	codeAscent  float32
	codeDescent float32
}

// New makes a view with no file open.
func New() Viewer {
	return Viewer{openedFolds: map[int]bool{}, unfolded: map[Mode]bool{DiffView: true}, codeSize: defaultCodeSize, rowHeight: codeRowHeight, List: pane.Scroller{RowHeight: codeRowHeight},
		ColourPause: colourPause, HoverPause: hoverPause, shown: wholeFile}
}

// IsOpen tells whether a file is shown.
func (view *Viewer) IsOpen() bool { return view.showing }

// Close stops showing the file.
func (view *Viewer) Close() {
	view.showing = false
	view.Document = nil
	view.Edit = nil
	view.Request++
	view.shown = view.presentationFor()
}

// Start begins showing a file, before its text has been read.
func (view *Viewer) Start(reference workspace.FileReference, project *workspace.Project) {
	if view.showing && view.Reference == reference && view.Document != nil {
		return
	}
	view.Request++
	if view.Reference != reference || !view.showing {
		view.Edit, view.selecting = nil, false // the same file read again keeps its edits
		view.reopening = false
	}
	view.Reference = reference
	view.markdown = isMarkdownPath(reference.Path)
	view.Project = project
	view.MarkedLine = 0
	view.Document = nil
	view.showing = true
	view.openedFolds = map[int]bool{}
	view.List.Offset, view.List.OffsetX = 0, 0
	view.clickedSome = false
	view.shown = view.presentationFor()
}

// Show puts a file's text in the view once it has been read: as its diff when showChanges asks for
// it and the file changed, whole otherwise.
func (view *Viewer) Show(document *workspace.Document, showChanges bool) {
	keepPlace := view.reopening || (view.Document != nil && view.Document.Path == document.Path)
	view.reopening = false
	view.Document = document
	view.keepEditing()
	if !keepPlace {
		view.formatted, view.formatting = view.IsMarkdown() && len(view.Uses) == 0 && view.GoTo == 0, nil
		view.Mode = FileView
		switch {
		case len(view.Uses) > 0:
			view.Mode = UsesView
		case document.Changed && showChanges:
			view.Mode = DiffView
		}
	}
	view.layOut()
	if view.GoTo > 0 {
		view.RevealLine(view.GoTo)
		return
	}
	if !keepPlace && view.Mode != FileView && len(view.Hunks) > 0 {
		view.List.RevealNearTop(view.Hunks[0])
	}
}

// SetUses gives the uses of the file open, or none, and shows them when the uses view is on.
func (view *Viewer) SetUses(uses map[int][]workspace.Span) {
	view.Uses = uses
	if view.Mode == UsesView && len(uses) == 0 {
		view.Mode = FileView
	}
	view.shown = view.presentationFor()
	if view.Document != nil {
		view.layOut()
	}
}

// Reopen opens the file again when its project was read again, as it may have changed, keeping
// where it was scrolled to.
func (view *Viewer) Reopen(path string, open func(file workspace.FileReference)) {
	if view.showing && view.Project != nil && view.Project.Path == path {
		offset, sideways := view.List.Offset, view.List.OffsetX
		view.Document = nil
		view.reopening = true
		open(view.Reference)
		view.List.Offset, view.List.OffsetX = offset, sideways
	}
}

// Build adds the view: the file's text (the window's bar names it, with BuildTitle and
// BuildButtons). With no file open it shows how to start, unless the window has something of its
// own to show there.
func (view *Viewer) Build(context *ui.Context, window Window) {
	look := window.Look()
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: look.Editor}, func() {
		if !view.showing {
			if page, hasPage := window.(Page); hasPage && page.BuildInsteadOfAFile(context) {
				return
			}
			view.buildEmpty(context, window)
			return
		}
		view.measure(context)
		if view.Formatted() {
			view.buildFormatted(context, window)
			return
		}
		view.List.RowHeight = view.rowHeight
		view.List.Margin = codeMargin
		view.List.Count = view.RowCount()
		if !window.ModalOpen() {
			view.followInput(context, window)
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			view.List.Place(area)
			view.paint(context, window, drawing)
		})
	})
}

// followInput scrolls, opens a fold clicked, follows a name ⌘-clicked to its declaration, and
// places the caret where the file being edited is clicked.
func (view *Viewer) followInput(context *ui.Context, window Window) {
	keys := window.Keys()
	view.followZoom(context, keys)
	onBar := view.List.Follow(context, keys, context.Theme.ScrollStep)
	x, y := context.Pointer()
	inside := keys.Clicked() && view.List.Area.Contains(x, y)
	if inside {
		window.FocusViewer()
	}
	clicked := inside && !onBar
	if clicked {
		index := view.List.RowAt(x, y)
		if index >= 0 && view.showsRows() && index < len(view.rows) && view.rows[index].IsFold() {
			view.openFold(index)
		}
	}
	if place, found := view.PlaceAt(x, y); clicked && found {
		view.clicked, view.clickedSome = place, true
		if navigator, navigates := window.(Navigator); navigates && keys.Command() {
			navigator.GoToDefinition(place)
			return
		}
	}
	view.hoverAt(context, window, x, y)
	if view.Editable() && !window.ReadOnly() && !onBar {
		view.followPointer(keys, x, y, clicked)
	}
}
