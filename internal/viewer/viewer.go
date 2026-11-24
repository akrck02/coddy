// Package viewer is the view of one file both modes share: a header naming it, and its text, as
// the changes or as the whole file, with a gutter, folds and marks.
package viewer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

type Viewer struct {
	Reference workspace.FileReference
	Project   *workspace.Project
	Document  *workspace.Document
	Request   int // counts the files asked for, so a slow read that was overtaken is dropped
	showing   bool

	Mode Mode
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
		ColourPause: colourPause, HoverPause: hoverPause}
}

// IsOpen tells whether a file is shown.
func (view *Viewer) IsOpen() bool { return view.showing }

// Close stops showing the file.
func (view *Viewer) Close() {
	view.showing = false
	view.Document = nil
	view.Edit = nil
	view.Request++
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
	if view.Document != nil {
		view.layOut()
	}
}

// RevealLine scrolls to a line of the file as it is now and marks it, opening the fold it is in
// when the diff is shown.
func (view *Viewer) RevealLine(number int) {
	view.GoTo = 0
	view.MarkedLine = number
	if view.Edit != nil { // the caret goes there too, at the column asked for
		view.Edit.Place(text.Position{Line: number - 1, Column: view.GoToColumn}, false)
	}
	view.GoToColumn = 0
	if !view.showsRows() {
		view.List.RevealNearTop(max(min(number-1, view.RowCount()-1), 0))
		return
	}
	for attempt := 0; attempt < 2; attempt++ {
		for index, row := range view.rows {
			if !row.IsFold() && row.Line.Kind != git.Removal && row.Line.NewNumber == number {
				view.List.RevealNearTop(index)
				return
			}
		}
		// the line is folded away: open its fold and look again
		for _, row := range view.rows {
			if !row.IsFold() {
				continue
			}
			for offset := 0; offset < row.Folded; offset++ {
				if line := view.diff[row.FoldStart+offset]; line.NewNumber == number {
					view.openedFolds[row.FoldStart] = true
				}
			}
		}
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

// layOut works out the rows to show for the mode.
func (view *Viewer) layOut() {
	document := view.Document
	view.rows, view.diff, view.Hunks, view.Inline, view.Marks = nil, nil, nil, nil, nil
	defer func() { view.List.Count = view.RowCount() }()
	if document == nil {
		return
	}
	if view.Mode == UsesView {
		view.layOutUses()
		return
	}
	if document.Changed {
		view.Added, view.Gone = workspace.Counts(document.Diff)
		view.Marks = workspace.LineMarks(document.Diff)
	}
	if view.Mode == DiffView && document.Changed {
		view.rows, view.diff = workspace.DiffRows(document.Diff, view.foldContext(), view.openedFolds), document.Diff
		view.Hunks = workspace.HunkStarts(view.rows)
		view.Inline = workspace.InlineChanges(view.rows, func(line git.DiffLine) []rune {
			if line.Kind == git.Removal {
				return []rune(lineAt(document.OldLines, line.OldNumber).Text())
			}
			return []rune(lineAt(document.NewLines, line.NewNumber).Text())
		})
		return
	}
	// in the file view, ] and [ go from one run of marked lines to the next
	for number := 1; number <= len(view.lines()); number++ {
		if view.Marks[number] != workspace.NoMark && view.Marks[number-1] == workspace.NoMark {
			view.Hunks = append(view.Hunks, number-1)
		}
	}
}

// layOutUses folds the lines away from the uses, as the diff folds the lines away from changes.
func (view *Viewer) layOutUses() {
	focus := map[int]bool{}
	for number := range view.Uses {
		focus[number] = true
	}
	view.rows, view.diff = workspace.FocusRows(len(view.lines()), focus, view.foldContext(), view.openedFolds)
	view.Hunks = workspace.FocusStarts(view.rows, focus)
}

// showsRows tells whether the view shows rows, folds among them, rather than the file's lines.
func (view *Viewer) showsRows() bool {
	return view.Mode == UsesView || (view.Mode == DiffView && view.Document != nil && view.Document.Changed)
}

// lines are the lines shown in the file view: as it is now, or as it was for a deleted file.
func (view *Viewer) lines() []highlight.Line {
	document := view.Document
	if document.Changed && document.Change.Status == git.Deleted {
		return document.OldLines
	}
	return document.NewLines
}

// RowCount is how many rows the file takes as shown.
func (view *Viewer) RowCount() int {
	if view.Document == nil {
		return 0
	}
	if view.showsRows() {
		return len(view.rows)
	}
	return view.fileLineCount()
}

// SetMode shows the file as its changes or whole; only a changed file has changes to show.
func (view *Viewer) SetMode(mode Mode) {
	if view.Document == nil || mode == view.Mode || (mode == DiffView && !view.Document.Changed) ||
		(mode == UsesView && len(view.Uses) == 0) {
		return
	}
	view.Mode = mode
	view.layOut()
	view.List.Offset = 0
	if mode != FileView && len(view.Hunks) > 0 {
		view.List.RevealNearTop(view.Hunks[0])
	}
}

// foldContext is how many unchanged lines the view keeps around a change: -1 for every one.
func (view *Viewer) foldContext() int {
	if view.unfolded[view.Mode] {
		return -1
	}
	return foldContext
}

// Unfolded tells whether the view shows every unchanged line.
func (view *Viewer) Unfolded() bool { return view.unfolded[view.Mode] }

// ToggleContext shows every unchanged line, or folds them again.
func (view *Viewer) ToggleContext() {
	view.unfolded[view.Mode] = !view.unfolded[view.Mode]
	view.layOut()
	view.List.Clamp()
}

// StepHunk jumps to the next (or previous) run of changes.
func (view *Viewer) StepHunk(step int) {
	if len(view.Hunks) == 0 {
		return
	}
	marker := view.List.RowOf(view.List.Area.Y + view.List.Area.Height/3)
	target := -1
	if step > 0 {
		for _, start := range view.Hunks {
			if start > marker {
				target = start
				break
			}
		}
	} else {
		for index := len(view.Hunks) - 1; index >= 0; index-- {
			if view.Hunks[index] < marker {
				target = view.Hunks[index]
				break
			}
		}
	}
	if target >= 0 {
		view.List.RevealNearTop(target)
	}
}

// ScrollLines scrolls by step lines; a pane.PageStep is a page, a pane.EndStep the top or the bottom.
func (view *Viewer) ScrollLines(step int) {
	list := &view.List
	page := list.Area.Height - list.RowHeight*2
	switch {
	case step >= pane.EndStep:
		list.Offset = list.ContentHeight()
	case step <= -pane.EndStep:
		list.Offset = 0
	case step >= pane.PageStep:
		list.Offset += page
	case step <= -pane.PageStep:
		list.Offset -= page
	default:
		list.Offset += float32(step) * list.RowHeight
	}
	list.Clamp()
}

// ScrollSideways scrolls by step times four characters.
func (view *Viewer) ScrollSideways(step int) {
	view.List.OffsetX += float32(step) * view.charWidth * sidewaysCharacters
	view.List.Clamp()
}

// sidewaysCharacters are how many characters one press of ← or → scrolls.
const sidewaysCharacters = 4

// OpenFoldInSight opens the first fold that can be seen.
func (view *Viewer) OpenFoldInSight() {
	first, last := view.List.Visible()
	for index := first; index < last && index < len(view.rows); index++ {
		if view.rows[index].IsFold() {
			view.openFold(index)
			return
		}
	}
}

func (view *Viewer) openFold(index int) {
	view.openedFolds[view.rows[index].FoldStart] = true
	view.layOut()
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

func (view *Viewer) buildEmpty(context *ui.Context, window Window) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), AlignX: ui.Center, AlignY: ui.Center, Gap: 14}, func() {
		context.IconSized("files", 40, palette.InkQuiet.WithAlpha(0.5))
		if !window.HasProjects() {
			context.Text("Open a repository, or a folder of repositories", ui.TextStyle{Color: palette.InkSoft})
			if context.PrimaryIconButton("folder-open", "Open folder") {
				window.OpenFolder()
			}
			return
		}
		context.Box(ui.Layout{Direction: ui.Vertical, Gap: 6}, func() {
			for _, hint := range [][2]string{
				{"Find a file", "⌘P"}, {"Next changed file", "n"}, {"Another mode", "c"}, {"Every key", "?"},
			} {
				context.Box(ui.Layout{Width: ui.Fixed(260), Gap: 12}, func() {
					context.Text(hint[0], ui.TextStyle{Color: palette.InkQuiet})
					context.Spacer()
					context.Text(hint[1], ui.TextStyle{Color: palette.InkSoft, Weight: draw2d.Mono})
				})
			}
		})
	})
}

// buildChange says how the file changed: its status, the path it came from, the lines added and
// gone.
func (view *Viewer) buildChange(context *ui.Context, compact bool) {
	palette := context.Theme
	change := view.Document.Change
	context.Box(ui.Layout{Width: ui.Fixed(4)}, nil)
	if !compact {
		context.Text(statusName(change.Status), ui.TextStyle{Size: palette.SmallTextSize, Color: theme.StatusColour(palette, change.Status)})
	}
	if change.OldPath != change.Path && !compact {
		context.Text("from "+change.OldPath, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
	}
	if view.Added > 0 {
		context.Text(view.added.Of(view.Added, addedWords), ui.TextStyle{Color: palette.Positive, Weight: draw2d.Mono, Size: palette.SmallTextSize})
	}
	if view.Gone > 0 {
		context.Text(view.gone.Of(view.Gone, goneWords), ui.TextStyle{Color: palette.Danger, Weight: draw2d.Mono, Size: palette.SmallTextSize})
	}
}

// buildViewButtons offers the ways to show a changed file: the changes' arrows, folding, the view
// and the mode's button. A narrow view shows the folding's icon without its word.
func (view *Viewer) buildViewButtons(context *ui.Context, window Window, compact bool) {
	context.Box(ui.Layout{Width: ui.Fixed(6)}, nil)
	if len(view.Hunks) > 0 {
		view.buildHunkArrows(context)
	}
	if view.Mode != FileView {
		icon, label, tip := "unfold-vertical", "Expand", "Show every unchanged line (f)"
		if view.Unfolded() {
			icon, label, tip = "fold-vertical", "Fold", "Fold the unchanged lines (f)"
		}
		if compact && theme.ToolButton(context, tip, icon, false) || !compact && theme.LabelledButton(context, tip, icon, label) {
			view.ToggleContext()
		}
	}
	view.buildViewSwitch(context)
	marker, marks := window.(Marker)
	if !marks {
		return
	}
	if button, shown := marker.FileButton(view.Reference); shown {
		if theme.ToolButton(context, button.Tip, button.Icon, button.On) {
			marker.PressFileButton(view.Reference)
		}
	}
}

// buildHunkArrows goes to the previous or the next run of changes, as [ and ] do.
func (view *Viewer) buildHunkArrows(context *ui.Context) {
	if theme.ToolButton(context, "Previous change (⇧F7, [)", "chevron-up", false) {
		view.StepHunk(-1)
	}
	if theme.ToolButton(context, "Next change (F7, ])", "chevron-down", false) {
		view.StepHunk(1)
	}
}

// buildViewSwitch switches between the changes, or the uses, and the whole file.
func (view *Viewer) buildViewSwitch(context *ui.Context) {
	other, choice := DiffView, theme.IconChoice{Icon: "file-diff", Name: "The changes (d)"}
	if len(view.Uses) > 0 {
		other, choice = UsesView, theme.IconChoice{Icon: "file-search", Name: "The uses (d)"}
	}
	chosen := 0
	if view.Mode == FileView {
		chosen = 1
	}
	if theme.IconSwitch(context, "view mode", &chosen, []theme.IconChoice{choice, {Icon: "file-code", Name: "The whole file (d)"}}) {
		if chosen == 1 {
			view.SetMode(FileView)
		} else {
			view.SetMode(other)
		}
	}
}

// splitPath cuts a path into its folder and its name.
func splitPath(path string) (folder, name string) {
	slash := strings.LastIndexByte(path, '/')
	if slash < 0 {
		return "", path
	}
	return path[:slash], path[slash+1:]
}

func statusName(status git.Status) string {
	switch status {
	case git.Added:
		return "Added"
	case git.Deleted:
		return "Deleted"
	case git.Renamed:
		return "Renamed"
	case git.Copied:
		return "Copied"
	case git.TypeChanged:
		return "Type changed"
	}
	return "Modified"
}

// measure measures a character of the code, the width every column is, when its size changed.
func (view *Viewer) measure(context *ui.Context) {
	if view.measuredSize == view.codeSize && view.charWidth > 0 {
		return
	}
	view.measuredSize = view.codeSize
	metrics := context.Measurer().MeasureText("M", view.codeSize, draw2d.Mono, 1)
	view.charWidth = metrics.Width
	view.codeAscent, view.codeDescent = metrics.Ascent, metrics.Descent
}

// gutterMarkWidth is the bar beside a line's number that says it was added or changed.
const gutterMarkWidth float32 = 3

// paint paints the visible lines: numbers in a gutter, coloured code, and the changes: in the
// file view as marks beside the numbers, in the diff on washes of green and red, stronger behind
// the words that changed.
func (view *Viewer) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	document := view.Document
	if document == nil {
		return
	}
	if document.Binary || (document.Error != "" && view.RowCount() == 0) {
		message := "A binary file: nothing to show"
		if !document.Binary {
			message = document.Error
		}
		drawing.Text(message, view.List.Area.X+24, view.List.Area.Y+32, context.Theme.TextSize, draw2d.Regular, context.Theme.InkQuiet)
		return
	}
	painter := view.startPainting(context, window, drawing)
	view.List.Width = 0
	drawing.PushClip(painter.area)
	first, last := view.List.Visible()
	for index := first; index < last; index++ {
		top := view.List.RowTop(index)
		baseline := top + (view.rowHeight+view.codeAscent-view.codeDescent)/2
		switch {
		case !view.showsRows():
			view.paintFileLine(painter, index, top, baseline)
		case view.rows[index].IsFold():
			view.paintFold(painter, view.rows[index], top, baseline)
		case view.Mode == UsesView:
			view.paintUseLine(painter, index, top, baseline)
		default:
			view.paintDiffLine(painter, index, top, baseline)
		}
	}
	view.List.PaintBar(drawing, painter.look.ScrollThumb)
	if view.Mode == UsesView || view.showsStatus(painter.window) { // the uses' marks are not git's
		view.paintChangeMarks(drawing, painter.look)
	}
	drawing.PopClip()
}

// painting is what every line painted in one frame shares: where the gutter and the code are.
type painting struct {
	context     *ui.Context
	window      Window
	drawing     *draw2d.List
	look        theme.Look
	diff        bool
	area        draw2d.Rectangle
	codeArea    draw2d.Rectangle
	numberWidth float32
	gutter      float32
}

// startPainting measures the gutter, wide enough for the longest number, and the code beside it.
func (view *Viewer) startPainting(context *ui.Context, window Window, drawing *draw2d.List) painting {
	document := view.Document
	area := view.List.Area
	diff := view.Mode == DiffView && document.Changed
	numberColumns := len(strconv.Itoa(max(len(document.NewLines), len(document.OldLines), 1)))
	numberWidth := float32(numberColumns)*view.charWidth + 16
	gutter := numberWidth + 14
	if diff {
		gutter = numberWidth*2 + view.charWidth*2 + 6
	}
	codeLeft := area.X + gutter + 10
	view.codeLeft, view.codeWidth = codeLeft, max(area.Right()-codeLeft-pane.ScrollBarWidth, 0)
	return painting{
		context: context, window: window, drawing: drawing, look: window.Look(), diff: diff, area: area,
		codeArea:    draw2d.Rectangle{X: codeLeft, Y: area.Y, Width: max(area.Right()-codeLeft-pane.ScrollBarWidth, 0), Height: area.Height},
		numberWidth: numberWidth, gutter: gutter,
	}
}

// paintFileLine paints a line of the whole file: its number, its mark and its code.
func (view *Viewer) paintFileLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	number := index + 1
	editing := view.Edit != nil && painter.window.Editing()
	numberInk := look.LineNumber
	if number == view.MarkedLine || (editing && index == view.Edit.Caret().Line) {
		drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}, 0, look.Hover, draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	view.paintNumber(drawing, number, area.X, painter.numberWidth, baseline, numberInk)
	if view.showsStatus(painter.window) {
		view.paintMark(drawing, look, view.Marks[number], area.X+painter.numberWidth+4, top)
	}
	if view.Edit != nil {
		view.paintSelection(painter, index, top)
	}
	view.paintCode(drawing, painter.look, view.fileLine(index), painter.codeArea, baseline)
	if editing {
		view.paintCaret(painter, index, top)
	}
}

// paintFold paints the row that stands for unchanged lines folded away.
func (view *Viewer) paintFold(painter painting, row workspace.Row, top, baseline float32) {
	palette, area := painter.context.Theme, painter.area
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	painter.drawing.Rectangle(rowArea, 0, painter.look.Hover.WithAlpha(0.6), draw2d.Color{}, 0)
	painter.drawing.Icon("unfold-vertical", area.X+painter.gutter-20, top+(view.rowHeight-12)/2, 12, palette.InkQuiet)
	label := view.foldWordsOf(foldLabel{row.Folded, view.Mode == UsesView})
	painter.drawing.Text(label, painter.codeArea.X, baseline, palette.SmallTextSize, draw2d.Regular, palette.InkQuiet)
}

// foldLabel is what a fold's words say: how many lines it holds, and whether they are lines
// without uses or unchanged.
type foldLabel struct {
	lines      int
	withoutUse bool
}

// foldWordsOf are a fold's words, made the first time a fold of that size shows.
func (view *Viewer) foldWordsOf(label foldLabel) string {
	if words, made := view.foldWords[label]; made {
		return words
	}
	if view.foldWords == nil {
		view.foldWords = map[foldLabel]string{}
	}
	words := fmt.Sprintf("%d unchanged lines", label.lines)
	if label.withoutUse {
		words = fmt.Sprintf("%d lines without uses", label.lines)
	}
	view.foldWords[label] = words
	return words
}

// lineStyle is how a line of the diff is drawn: its code, its sign and the colours of a change.
type lineStyle struct {
	code       highlight.Line
	sign       string
	signColour draw2d.Color
	lineWash   draw2d.Color
	wordWash   draw2d.Color
	edge       draw2d.Color
}

// styleOf picks a diff line's code from the side it is on, and its colours from how it changed.
func (view *Viewer) styleOf(line git.DiffLine, look theme.Look) lineStyle {
	document := view.Document
	switch line.Kind {
	case git.Insertion:
		return lineStyle{code: lineAt(document.NewLines, line.NewNumber), sign: "+", signColour: look.Created,
			lineWash: look.CreatedLine, wordWash: look.CreatedWord, edge: look.Created}
	case git.Removal:
		return lineStyle{code: lineAt(document.OldLines, line.OldNumber), sign: "−", signColour: look.Deleted,
			lineWash: look.DeletedLine, wordWash: look.DeletedWord, edge: look.Deleted}
	}
	return lineStyle{code: lineAt(document.NewLines, line.NewNumber)} // an unchanged line has no sign
}

// paintDiffLine paints a line of the diff: its wash, its two numbers, its sign and its code.
func (view *Viewer) paintDiffLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	line := view.rows[index].Line
	style := view.styleOf(line, look)
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	if style.lineWash.IsVisible() {
		view.paintWash(painter, index, style, rowArea)
	}
	numberInk := look.LineNumber
	if line.Kind != git.Removal && line.NewNumber == view.MarkedLine && view.MarkedLine > 0 {
		drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.8), draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	if line.Kind != git.Context {
		numberInk = style.signColour.WithAlpha(0.75)
	}
	view.paintNumber(drawing, line.OldNumber, area.X, painter.numberWidth, baseline, numberInk)
	view.paintNumber(drawing, line.NewNumber, area.X+painter.numberWidth, painter.numberWidth, baseline, numberInk)
	if style.sign != "" {
		drawing.Text(style.sign, area.X+painter.numberWidth*2+view.charWidth/2, baseline, view.codeSize, draw2d.Mono, style.signColour)
	}
	view.paintCode(drawing, painter.look, style.code, painter.codeArea, baseline)
}

// paintUseLine paints a line of the uses view: its number and code, washed when it uses the name,
// more strongly behind each use.
func (view *Viewer) paintUseLine(painter painting, index int, top, baseline float32) {
	look, area, drawing := painter.look, painter.area, painter.drawing
	number := view.rows[index].Line.NewNumber
	rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
	numberInk := look.LineNumber
	if spans := view.Uses[number]; len(spans) > 0 {
		drawing.Rectangle(rowArea, 0, look.Theme.PrimaryTint, draw2d.Color{}, 0)
		drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Theme.Primary, draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
		line := lineAt(view.lines(), number).Text()
		drawing.PushClip(painter.codeArea)
		for _, span := range spans {
			start, end := highlight.Column(line, span.Start), highlight.Column(line, span.End)
			x := painter.codeArea.X + float32(start)*view.charWidth - view.List.OffsetX
			word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(end-start) * view.charWidth, Height: view.rowHeight - 4}
			drawing.Rectangle(word, 3, look.Theme.Selection, draw2d.Color{}, 0)
		}
		drawing.PopClip()
	}
	if number == view.MarkedLine && view.MarkedLine > 0 {
		drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.8), draw2d.Color{}, 0)
		numberInk = look.ActiveLineNumber
	}
	view.paintNumber(drawing, number, area.X, painter.numberWidth, baseline, numberInk)
	view.paintCode(drawing, painter.look, lineAt(view.lines(), number), painter.codeArea, baseline)
}

// paintWash paints a changed line's background, its edge, and a stronger wash behind the words that
// changed.
func (view *Viewer) paintWash(painter painting, index int, style lineStyle, rowArea draw2d.Rectangle) {
	drawing, top := painter.drawing, rowArea.Y
	drawing.Rectangle(rowArea, 0, style.lineWash, draw2d.Color{}, 0)
	drawing.Rectangle(draw2d.Rectangle{X: rowArea.X, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, style.edge, draw2d.Color{}, 0)
	span, found := view.Inline[index]
	if !found || span.End <= span.Start {
		return
	}
	x := painter.codeArea.X + float32(span.Start)*view.charWidth - view.List.OffsetX
	word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(span.End-span.Start) * view.charWidth, Height: view.rowHeight - 4}
	drawing.PushClip(painter.codeArea)
	drawing.Rectangle(word, 3, style.wordWash, draw2d.Color{}, 0)
	drawing.PopClip()
}

// paintMark paints a line's mark in the file view's gutter: a bar for a line added (green) or
// changed (amber), a small wedge where lines were removed (red).
func (view *Viewer) paintMark(drawing *draw2d.List, look theme.Look, mark workspace.Mark, x, top float32) {
	switch mark {
	case workspace.AddedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Created, draw2d.Color{}, 0)
	case workspace.ModifiedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, look.Modified, draw2d.Color{}, 0)
	case workspace.RemovedAbove:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top - 2, Width: 8, Height: 4}, 2, look.Deleted, draw2d.Color{}, 0)
	case workspace.RemovedBelow:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top + view.rowHeight - 2, Width: 8, Height: 4}, 2, look.Deleted, draw2d.Color{}, 0)
	}
}

// paintChangeMarks marks where the changes are along the scroll bar, as a map of the file.
func (view *Viewer) paintChangeMarks(drawing *draw2d.List, look theme.Look) {
	bar := view.List.BarArea()
	mark := func(index, count int, colour draw2d.Color) {
		y := bar.Y + float32(index)/float32(count)*bar.Height
		drawing.Rectangle(draw2d.Rectangle{X: bar.X - 3, Y: y, Width: 3, Height: max(bar.Height/float32(count), 2)}, 0, colour, draw2d.Color{}, 0)
	}
	if view.Mode == UsesView {
		for index, row := range view.rows {
			if !row.IsFold() && len(view.Uses[row.Line.NewNumber]) > 0 {
				mark(index, len(view.rows), look.Theme.Primary)
			}
		}
		return
	}
	if view.Mode == DiffView {
		for index, row := range view.rows {
			switch {
			case row.IsFold() || row.Line.Kind == git.Context:
			case row.Line.Kind == git.Removal:
				mark(index, len(view.rows), look.Deleted)
			default:
				mark(index, len(view.rows), look.Created)
			}
		}
		return
	}
	count := len(view.lines())
	for number, kind := range view.Marks {
		switch kind {
		case workspace.AddedMark:
			mark(number-1, count, look.Created)
		case workspace.ModifiedMark:
			mark(number-1, count, look.Modified)
		default:
			mark(number-1, count, look.Deleted)
		}
	}
}

func lineAt(lines []highlight.Line, number int) highlight.Line {
	if number < 1 || number > len(lines) {
		return highlight.Line{}
	}
	return lines[number-1]
}

func (view *Viewer) paintNumber(drawing *draw2d.List, number int, left, width, baseline float32, colour draw2d.Color) {
	if number <= 0 {
		return
	}
	text := view.numberText(number)
	x := left + width - 8 - float32(len(text))*view.charWidth
	drawing.Text(text, x, baseline, view.codeSize, draw2d.Mono, colour)
}

// numberText is a line's number in words, each made once, the first time its line shows.
func (view *Viewer) numberText(number int) string {
	if text, made := view.numbers[number]; made {
		return text
	}
	if view.numbers == nil {
		view.numbers = map[int]string{}
	}
	text := strconv.Itoa(number)
	view.numbers[number] = text
	return text
}

// paintCode paints a line's coloured pieces, shifted by the sideways scroll and cut at the area.
func (view *Viewer) paintCode(drawing *draw2d.List, look theme.Look, line highlight.Line, area draw2d.Rectangle, baseline float32) {
	width := float32(line.Columns) * view.charWidth
	view.List.Width = max(view.List.Width, width)
	drawing.PushClip(area)
	for _, span := range line.Spans {
		x := area.X + float32(span.Column)*view.charWidth - view.List.OffsetX
		if x > area.Right() {
			break
		}
		if x+float32(len(span.Text))*view.charWidth < area.X || isBlank(span.Text) {
			continue
		}
		drawing.Text(span.Text, x, baseline, view.codeSize, draw2d.Mono, look.Syntax.Colour(span.Kind))
	}
	drawing.PopClip()
}

func isBlank(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] != ' ' {
			return false
		}
	}
	return true
}

func addedWords(count int) string { return fmt.Sprintf("+%d", count) }
func goneWords(count int) string  { return fmt.Sprintf("−%d", count) }
