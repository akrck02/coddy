// Package viewer is the view of one file both modes share: a header naming it, and its text, as
// the changes or as the whole file, with a gutter, folds and marks.
package viewer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/pane"
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
	// FileButton is the button the mode that is on puts in the header, such as Viewed;
	// PressFileButton presses it.
	FileButton(file workspace.FileReference) (Button, bool)
	PressFileButton(file workspace.FileReference)
	// BuildInsteadOfAFile builds what the window shows while no file is open, and tells whether it
	// had anything; if not, the view shows how to start.
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

type Viewer struct {
	Reference workspace.FileReference
	Project   *workspace.Project
	Document  *workspace.Document
	Request   int // counts the files asked for, so a slow read that was overtaken is dropped
	showing   bool

	Mode        Mode
	fullContext bool         // show every unchanged line instead of folding them
	openedFolds map[int]bool // folds opened by hand, by where they start in the diff

	rows  []workspace.Row // the diff's rows as shown
	Hunks []int
	Added int
	Gone  int
	// inline are the words that changed inside a replaced line, by row of the diff
	Inline map[int]workspace.Span
	// goTo is a line to show once the file is read, from the finder; markedLine is the line shown
	// that way, painted as the line the reader is on
	GoTo       int
	MarkedLine int
	// marks say, in the file view, which lines the development added or changed and where it
	// removed some, as Zed marks them in its gutter
	Marks map[int]workspace.Mark

	List pane.Scroller

	codeSize    float32
	rowHeight   float32
	charWidth   float32
	codeAscent  float32
	codeDescent float32
}

// New makes a view with no file open.
func New() Viewer {
	return Viewer{openedFolds: map[int]bool{}, codeSize: 13, rowHeight: codeRowHeight, List: pane.Scroller{RowHeight: codeRowHeight}}
}

// IsOpen tells whether a file is shown.
func (view *Viewer) IsOpen() bool { return view.showing }

// Close stops showing the file.
func (view *Viewer) Close() {
	view.showing = false
	view.Document = nil
	view.Request++
}

// Start begins showing a file, before its text has been read.
func (view *Viewer) Start(reference workspace.FileReference, project *workspace.Project) {
	if view.showing && view.Reference == reference && view.Document != nil {
		return
	}
	view.Request++
	view.Reference = reference
	view.Project = project
	view.MarkedLine = 0
	view.Document = nil
	view.showing = true
	view.openedFolds = map[int]bool{}
	view.List.Offset, view.List.OffsetX = 0, 0
}

// Show puts a file's text in the view once it has been read: as its diff when showChanges asks for
// it and the file changed, whole otherwise.
func (view *Viewer) Show(document *workspace.Document, showChanges bool) {
	keepPlace := view.Document != nil && view.Document.Path == document.Path
	view.Document = document
	if !keepPlace {
		view.Mode = FileView
		if document.Changed && showChanges {
			view.Mode = DiffView
		}
	}
	view.layOut()
	if view.GoTo > 0 {
		view.RevealLine(view.GoTo)
		return
	}
	if !keepPlace && view.Mode == DiffView && len(view.Hunks) > 0 {
		view.List.RevealNearTop(view.Hunks[0])
	}
}

// RevealLine scrolls to a line of the file as it is now and marks it, opening the fold it is in
// when the diff is shown.
func (view *Viewer) RevealLine(number int) {
	view.GoTo = 0
	view.MarkedLine = number
	if view.Mode != DiffView || !view.Document.Changed {
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
				if line := view.Document.Diff[row.FoldStart+offset]; line.NewNumber == number {
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
		offset := view.List.Offset
		view.Document = nil
		open(view.Reference)
		view.List.Offset = offset
	}
}

// layOut works out the rows to show for the mode.
func (view *Viewer) layOut() {
	document := view.Document
	view.rows, view.Hunks, view.Inline, view.Marks = nil, nil, nil, nil
	defer func() { view.List.Count = view.RowCount() }()
	if document == nil {
		return
	}
	if document.Changed {
		view.Added, view.Gone = workspace.Counts(document.Diff)
		view.Marks = workspace.LineMarks(document.Diff)
	}
	if view.Mode == DiffView && document.Changed {
		context := foldContext
		if view.fullContext {
			context = -1
		}
		view.rows = workspace.DiffRows(document.Diff, context, view.openedFolds)
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
	if view.Mode == DiffView && view.Document.Changed {
		return len(view.rows)
	}
	return len(view.lines())
}

// SetMode shows the file as its changes or whole; only a changed file has changes to show.
func (view *Viewer) SetMode(mode Mode) {
	if view.Document == nil || mode == view.Mode || (mode == DiffView && !view.Document.Changed) {
		return
	}
	view.Mode = mode
	view.layOut()
	view.List.Offset = 0
	if mode == DiffView && len(view.Hunks) > 0 {
		view.List.RevealNearTop(view.Hunks[0])
	}
}

// ToggleContext shows every unchanged line, or folds them again.
func (view *Viewer) ToggleContext() {
	view.fullContext = !view.fullContext
	view.layOut()
	view.List.Clamp()
}

// StepHunk jumps to the next (or previous) run of changes.
func (view *Viewer) StepHunk(step int) {
	if len(view.Hunks) == 0 {
		return
	}
	marker := int((view.List.Offset + view.List.Area.Height/3) / view.List.RowHeight)
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

// Build adds the view: a header naming the file, and its text. With no file open it shows how to
// start, unless the window has something of its own to show there.
func (view *Viewer) Build(context *ui.Context, window Window) {
	look := window.Look()
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: look.Editor}, func() {
		if !view.showing {
			if window.BuildInsteadOfAFile(context) {
				return
			}
			view.buildEmpty(context, window)
			return
		}
		view.buildHeader(context, window)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: look.Hover}, nil)
		view.measure(context)
		view.List.RowHeight = view.rowHeight
		view.List.Count = view.RowCount()
		if !window.ModalOpen() {
			onBar := view.List.Follow(context, window.Keys(), context.Theme.ScrollStep)
			x, y := context.Pointer()
			if window.Keys().Clicked() && view.List.Area.Contains(x, y) {
				window.FocusViewer()
				index := view.List.RowAt(x, y)
				if !onBar && index >= 0 && view.Mode == DiffView && index < len(view.rows) && view.rows[index].IsFold() {
					view.openFold(index)
				}
			}
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			view.List.Place(area)
			view.paint(context, window, drawing)
		})
	})
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

// buildHeader names the file as a path to it, the folders quiet and the name plain, and says how it
// changed; on its right, the ways to show it.
func (view *Viewer) buildHeader(context *ui.Context, window Window) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(16, 0), Gap: 6, AlignY: ui.Center}, func() {
		document := view.Document
		folder, name := splitPath(view.Reference.Path)
		quiet := ui.TextStyle{Color: palette.InkQuiet}
		// a narrow view keeps the file's name and its buttons, and drops the project, the
		// status's word and the language; the folder takes what is left, cut from its start
		compact := view.List.Area.Width > 0 && view.List.Area.Width < 700
		room := view.List.Area.Width - 32 - theme.TextWidth(context, name, ui.TextStyle{Weight: draw2d.Medium}) - 40
		if !compact {
			room -= theme.TextWidth(context, view.Project.Name, quiet) + 20
		}
		if document != nil {
			if !compact {
				room -= theme.TextWidth(context, document.Language, ui.TextStyle{Size: palette.SmallTextSize}) + 12
			}
			if document.Changed {
				room -= 230 // the counts, the fold, view and viewed buttons
				if !compact {
					room -= 70 // the status's word
				}
			}
		}
		if !compact {
			context.Text(view.Project.Name, quiet)
			context.IconSized("chevron-right", 12, palette.InkQuiet.WithAlpha(0.6))
		}
		if folder = theme.FitTextEnd(context, folder, quiet, room); len([]rune(folder)) > 3 {
			context.Text(folder, quiet)
			context.IconSized("chevron-right", 12, palette.InkQuiet.WithAlpha(0.6))
		}
		nameInk := palette.Ink
		if document != nil && document.Changed {
			nameInk = theme.StatusColour(palette, document.Change.Status)
		}
		context.Text(name, ui.TextStyle{Weight: draw2d.Medium, Color: nameInk})
		if document == nil {
			context.Caption("Reading…")
			return
		}
		if document.Changed {
			change := document.Change
			context.Box(ui.Layout{Width: ui.Fixed(4)}, nil)
			if !compact {
				context.Text(statusName(change.Status), ui.TextStyle{Size: palette.SmallTextSize, Color: theme.StatusColour(palette, change.Status)})
			}
			if change.OldPath != change.Path && !compact {
				context.Text("from "+change.OldPath, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
			}
			if view.Added > 0 {
				context.Text(fmt.Sprintf("+%d", view.Added), ui.TextStyle{Color: palette.Positive, Weight: draw2d.Mono, Size: palette.SmallTextSize})
			}
			if view.Gone > 0 {
				context.Text(fmt.Sprintf("−%d", view.Gone), ui.TextStyle{Color: palette.Danger, Weight: draw2d.Mono, Size: palette.SmallTextSize})
			}
		}
		context.Spacer()
		if !compact {
			context.Text(document.Language, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
		}
		if !document.Changed {
			return
		}
		context.Box(ui.Layout{Width: ui.Fixed(6)}, nil)
		if view.Mode == DiffView {
			icon, tip := "unfold-vertical", "Show every unchanged line (f)"
			if view.fullContext {
				icon, tip = "fold-vertical", "Fold the unchanged lines (f)"
			}
			if theme.ToolButton(context, tip, icon, false) {
				view.ToggleContext()
			}
		}
		mode := int(view.Mode)
		if theme.IconSwitch(context, "view mode", &mode, []theme.IconChoice{
			{Icon: "file-diff", Name: "The changes (d)"}, {Icon: "file-code", Name: "The whole file (d)"},
		}) {
			view.SetMode(Mode(mode))
		}
		if button, shown := window.FileButton(view.Reference); shown {
			if theme.ToolButton(context, button.Tip, button.Icon, button.On) {
				window.PressFileButton(view.Reference)
			}
		}
	})
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

func (view *Viewer) measure(context *ui.Context) {
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
	palette := context.Theme
	look := window.Look()
	area := view.List.Area
	document := view.Document
	if document == nil {
		return
	}
	if document.Binary || (document.Error != "" && view.RowCount() == 0) {
		message := "A binary file: nothing to show"
		if !document.Binary {
			message = document.Error
		}
		drawing.Text(message, area.X+24, area.Y+32, palette.TextSize, draw2d.Regular, palette.InkQuiet)
		return
	}
	diff := view.Mode == DiffView && document.Changed
	numberColumns := len(strconv.Itoa(max(len(document.NewLines), len(document.OldLines), 1)))
	numberWidth := float32(numberColumns)*view.charWidth + 16
	gutter := numberWidth + 14
	if diff {
		gutter = numberWidth*2 + view.charWidth*2 + 6
	}
	codeLeft := area.X + gutter + 10
	codeArea := draw2d.Rectangle{X: codeLeft, Y: area.Y, Width: max(area.Right()-codeLeft-pane.ScrollBarWidth, 0), Height: area.Height}
	view.List.Width = 0
	drawing.PushClip(area)
	first, last := view.List.Visible()
	for index := first; index < last; index++ {
		top := view.List.RowTop(index)
		baseline := top + (view.rowHeight+view.codeAscent-view.codeDescent)/2
		if !diff {
			number := index + 1
			if number == view.MarkedLine {
				drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}, 0, look.Hover, draw2d.Color{}, 0)
			}
			numberInk := look.LineNumber
			if number == view.MarkedLine {
				numberInk = look.ActiveLineNumber
			}
			view.paintNumber(drawing, number, area.X, numberWidth, baseline, numberInk)
			view.paintMark(drawing, look, view.Marks[number], area.X+numberWidth+4, top)
			view.paintCode(drawing, window, view.lines()[index], codeArea, baseline)
			continue
		}
		row := view.rows[index]
		rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: view.rowHeight}
		if row.IsFold() {
			drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.6), draw2d.Color{}, 0)
			drawing.Icon("unfold-vertical", area.X+gutter-20, top+(view.rowHeight-12)/2, 12, palette.InkQuiet)
			label := fmt.Sprintf("%d unchanged lines", row.Folded)
			drawing.Text(label, codeLeft, baseline, palette.SmallTextSize, draw2d.Regular, palette.InkQuiet)
			continue
		}
		line := row.Line
		var code highlight.Line
		sign, signColour := "", palette.InkQuiet
		var lineWash, wordWash, edge draw2d.Color
		switch line.Kind {
		case git.Insertion:
			lineWash, wordWash, edge = look.CreatedLine, look.CreatedWord, look.Created
			sign, signColour = "+", look.Created
			code = lineAt(document.NewLines, line.NewNumber)
		case git.Removal:
			lineWash, wordWash, edge = look.DeletedLine, look.DeletedWord, look.Deleted
			sign, signColour = "−", look.Deleted
			code = lineAt(document.OldLines, line.OldNumber)
		default:
			code = lineAt(document.NewLines, line.NewNumber)
		}
		if lineWash.IsVisible() {
			drawing.Rectangle(rowArea, 0, lineWash, draw2d.Color{}, 0)
			drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutterMarkWidth, Height: view.rowHeight}, 0, edge, draw2d.Color{}, 0)
			if span, found := view.Inline[index]; found && span.End > span.Start {
				x := codeLeft + float32(span.Start)*view.charWidth - view.List.OffsetX
				word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(span.End-span.Start) * view.charWidth, Height: view.rowHeight - 4}
				drawing.PushClip(codeArea)
				drawing.Rectangle(word, 3, wordWash, draw2d.Color{}, 0)
				drawing.PopClip()
			}
		}
		numberInk := look.LineNumber
		if line.Kind != git.Removal && line.NewNumber == view.MarkedLine && view.MarkedLine > 0 {
			drawing.Rectangle(rowArea, 0, look.Hover.WithAlpha(0.8), draw2d.Color{}, 0)
			numberInk = look.ActiveLineNumber
		}
		if line.Kind != git.Context {
			numberInk = signColour.WithAlpha(0.75)
		}
		view.paintNumber(drawing, line.OldNumber, area.X, numberWidth, baseline, numberInk)
		view.paintNumber(drawing, line.NewNumber, area.X+numberWidth, numberWidth, baseline, numberInk)
		if sign != "" {
			drawing.Text(sign, area.X+numberWidth*2+view.charWidth/2, baseline, view.codeSize, draw2d.Mono, signColour)
		}
		view.paintCode(drawing, window, code, codeArea, baseline)
	}
	view.List.PaintBar(drawing, look.ScrollThumb)
	view.paintChangeMarks(drawing, look)
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
	text := strconv.Itoa(number)
	x := left + width - 8 - float32(len(text))*view.charWidth
	drawing.Text(text, x, baseline, view.codeSize, draw2d.Mono, colour)
}

// paintCode paints a line's coloured pieces, shifted by the sideways scroll and cut at the area.
func (view *Viewer) paintCode(drawing *draw2d.List, window Window, line highlight.Line, area draw2d.Rectangle, baseline float32) {
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
		drawing.Text(span.Text, x, baseline, view.codeSize, draw2d.Mono, window.Look().Syntax.Colour(span.Kind))
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
