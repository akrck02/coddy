package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/highlight"
	"github.com/akrck02/code-review/internal/review"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

// viewMode is how the editor shows a file.
type viewMode int

const (
	diffView viewMode = iota // the changes, both sides interleaved
	fileView                 // the file as it is now
)

// foldContext is how many unchanged lines are kept around a change before the rest is folded.
const foldContext = 3

// codeRowHeight is a line of code: 13 pixels of a monospaced face on lines 1.7 times as tall, as roomy as
// Zed's buffer.
const codeRowHeight float32 = 22

type editorState struct {
	reference fileReference
	project   *review.Project
	document  *review.Document
	request   int // counts the files asked for, so a slow read that was overtaken is dropped
	showing   bool

	mode        viewMode
	fullContext bool         // show every unchanged line instead of folding them
	openedFolds map[int]bool // folds opened by hand, by where they start in the diff

	rows  []review.Row // the diff's rows as shown
	hunks []int
	added int
	gone  int
	// inline are the words that changed inside a replaced line, by row of the diff
	inline map[int]review.Span
	// marks say, in the file view, which lines the development added or changed and where it
	// removed some, as Zed marks them in its gutter
	marks map[int]review.Mark

	list scroller

	codeSize    float32
	rowHeight   float32
	charWidth   float32
	codeAscent  float32
	codeDescent float32
}

func newEditor() editorState {
	return editorState{openedFolds: map[int]bool{}, codeSize: 13, rowHeight: codeRowHeight, list: scroller{rowHeight: codeRowHeight}}
}

func (editor *editorState) isOpen() bool { return editor.showing }

func (editor *editorState) close() {
	editor.showing = false
	editor.document = nil
	editor.request++
}

// start begins showing a file, before its text has been read.
func (editor *editorState) start(reference fileReference, project *review.Project) {
	if editor.showing && editor.reference == reference && editor.document != nil {
		return
	}
	editor.request++
	editor.reference = reference
	editor.project = project
	editor.document = nil
	editor.showing = true
	editor.openedFolds = map[int]bool{}
	editor.list.offset, editor.list.offsetX = 0, 0
}

// show puts a file's text in the editor once it has been read: as its diff when reviewing a
// changed file, whole otherwise.
func (editor *editorState) show(document *review.Document, reviewing bool) {
	keepPlace := editor.document != nil && editor.document.Path == document.Path
	editor.document = document
	if !keepPlace {
		editor.mode = fileView
		if document.Changed && reviewing {
			editor.mode = diffView
		}
	}
	editor.layOut()
	if !keepPlace && editor.mode == diffView && len(editor.hunks) > 0 {
		editor.list.revealNearTop(editor.hunks[0])
	}
}

// projectReloaded opens the file again when its project was read again, as it may have changed.
func (editor *editorState) projectReloaded(application *App, path string) {
	if editor.showing && editor.project != nil && editor.project.Path == path {
		offset := editor.list.offset
		editor.document = nil
		application.openFile(editor.reference)
		editor.list.offset = offset
	}
}

// layOut works out the rows to show for the mode.
func (editor *editorState) layOut() {
	document := editor.document
	editor.rows, editor.hunks, editor.inline, editor.marks = nil, nil, nil, nil
	defer func() { editor.list.count = editor.rowCount() }()
	if document == nil {
		return
	}
	if document.Changed {
		editor.added, editor.gone = review.Counts(document.Diff)
		editor.marks = review.LineMarks(document.Diff)
	}
	if editor.mode == diffView && document.Changed {
		context := foldContext
		if editor.fullContext {
			context = -1
		}
		editor.rows = review.DiffRows(document.Diff, context, editor.openedFolds)
		editor.hunks = review.HunkStarts(editor.rows)
		editor.inline = review.InlineChanges(editor.rows, func(line git.DiffLine) []rune {
			if line.Kind == git.Removal {
				return []rune(lineAt(document.OldLines, line.OldNumber).Text())
			}
			return []rune(lineAt(document.NewLines, line.NewNumber).Text())
		})
		return
	}
	// in the file view, ] and [ go from one run of marked lines to the next
	for number := 1; number <= len(editor.lines()); number++ {
		if editor.marks[number] != review.NoMark && editor.marks[number-1] == review.NoMark {
			editor.hunks = append(editor.hunks, number-1)
		}
	}
}

// lines are the lines shown in the file view: as it is now, or as it was for a deleted file.
func (editor *editorState) lines() []highlight.Line {
	document := editor.document
	if document.Changed && document.Change.Status == git.Deleted {
		return document.OldLines
	}
	return document.NewLines
}

func (editor *editorState) rowCount() int {
	if editor.document == nil {
		return 0
	}
	if editor.mode == diffView && editor.document.Changed {
		return len(editor.rows)
	}
	return len(editor.lines())
}

func (editor *editorState) setMode(mode viewMode) {
	if editor.document == nil || mode == editor.mode || (mode == diffView && !editor.document.Changed) {
		return
	}
	editor.mode = mode
	editor.layOut()
	editor.list.offset = 0
	if mode == diffView && len(editor.hunks) > 0 {
		editor.list.revealNearTop(editor.hunks[0])
	}
}

func (editor *editorState) toggleContext() {
	editor.fullContext = !editor.fullContext
	editor.layOut()
	editor.list.clamp()
}

// stepHunk jumps to the next (or previous) run of changes.
func (editor *editorState) stepHunk(step int) {
	if len(editor.hunks) == 0 {
		return
	}
	marker := int((editor.list.offset + editor.list.area.Height/3) / editor.list.rowHeight)
	target := -1
	if step > 0 {
		for _, start := range editor.hunks {
			if start > marker {
				target = start
				break
			}
		}
	} else {
		for index := len(editor.hunks) - 1; index >= 0; index-- {
			if editor.hunks[index] < marker {
				target = editor.hunks[index]
				break
			}
		}
	}
	if target >= 0 {
		editor.list.revealNearTop(target)
	}
}

// handleKeys scrolls while the editor has the keyboard.
func (editor *editorState) handleKeys(keys keys) {
	list := &editor.list
	step := list.rowHeight
	switch {
	case keys.typed(input.KeyDown) || keys.plain(input.KeyJ):
		list.offset += step
	case keys.typed(input.KeyUp) || keys.plain(input.KeyK):
		list.offset -= step
	case keys.typed(input.KeyPageDown) || keys.typed(input.KeySpace) && !keys.shift():
		list.offset += list.area.Height - step*2
	case keys.typed(input.KeyPageUp) || keys.typed(input.KeySpace) && keys.shift():
		list.offset -= list.area.Height - step*2
	case keys.pressed(input.KeyHome):
		list.offset = 0
	case keys.pressed(input.KeyEnd):
		list.offset = list.contentHeight()
	case keys.typed(input.KeyRight):
		list.offsetX += editor.charWidth * 4
	case keys.typed(input.KeyLeft):
		list.offsetX -= editor.charWidth * 4
	case keys.pressed(input.KeyEnter):
		editor.openFoldInSight()
	}
	list.clamp()
}

// openFoldInSight opens the first fold that can be seen.
func (editor *editorState) openFoldInSight() {
	first, last := editor.list.visible()
	for index := first; index < last && index < len(editor.rows); index++ {
		if editor.rows[index].IsFold() {
			editor.openFold(index)
			return
		}
	}
}

func (editor *editorState) openFold(index int) {
	editor.openedFolds[editor.rows[index].FoldStart] = true
	editor.layOut()
}

// build adds the editor: a header naming the file, and its text.
func (editor *editorState) build(context *ui.Context, application *App) {
	look := application.look
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, ui.Look{Fill: look.editor}, func() {
		if !editor.showing {
			if application.sidebar.mode == reviewMode && len(application.changedOrder) > 0 {
				application.buildOverview(context)
				return
			}
			editor.buildEmpty(context, application)
			return
		}
		editor.buildHeader(context, application)
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: look.hover}, nil)
		editor.measure(context)
		editor.list.rowHeight = editor.rowHeight
		editor.list.count = editor.rowCount()
		if !application.modalOpen() {
			onBar := editor.list.follow(context, application.keys, context.Theme.ScrollStep)
			x, y := context.Pointer()
			if application.keys.clicked() && editor.list.area.Contains(x, y) {
				application.focus = editorPane
				index := editor.list.rowAt(x, y)
				if !onBar && index >= 0 && editor.mode == diffView && index < len(editor.rows) && editor.rows[index].IsFold() {
					editor.openFold(index)
				}
			}
		}
		context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Grow()}, func(area draw2d.Rectangle, drawing *draw2d.List) {
			editor.list.place(area)
			editor.paint(context, application, drawing)
		})
	})
}

func (editor *editorState) buildEmpty(context *ui.Context, application *App) {
	theme := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), AlignX: ui.Center, AlignY: ui.Center, Gap: 14}, func() {
		context.IconSized("files", 40, theme.InkQuiet.WithAlpha(0.5))
		if len(application.projects) == 0 {
			context.Text("Open a repository, or a folder of repositories", ui.TextStyle{Color: theme.InkSoft})
			if context.PrimaryIconButton("folder-open", "Open folder") {
				application.pickerOpen = true
			}
			return
		}
		context.Box(ui.Layout{Direction: ui.Vertical, Gap: 6}, func() {
			for _, hint := range [][2]string{
				{"Find a file", "⌘P"}, {"Next changed file", "n"}, {"Review the changes", "2"}, {"Every key", "?"},
			} {
				context.Box(ui.Layout{Width: ui.Fixed(260), Gap: 12}, func() {
					context.Text(hint[0], ui.TextStyle{Color: theme.InkQuiet})
					context.Spacer()
					context.Text(hint[1], ui.TextStyle{Color: theme.InkSoft, Weight: draw2d.Mono})
				})
			}
		})
	})
}

// buildHeader names the file as a path to it, the folders quiet and the name plain, and says how it
// changed; on its right, the ways to show it.
func (editor *editorState) buildHeader(context *ui.Context, application *App) {
	theme := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(16, 0), Gap: 6, AlignY: ui.Center}, func() {
		document := editor.document
		folder, name := splitPath(editor.reference.path)
		quiet := ui.TextStyle{Color: theme.InkQuiet}
		// a narrow editor keeps the file's name and its buttons, and drops the project, the
		// status's word and the language; the folder takes what is left, cut from its start
		compact := editor.list.area.Width > 0 && editor.list.area.Width < 700
		room := editor.list.area.Width - 32 - textWidth(context, name, ui.TextStyle{Weight: draw2d.Medium}) - 40
		if !compact {
			room -= textWidth(context, editor.project.Name, quiet) + 20
		}
		if document != nil {
			if !compact {
				room -= textWidth(context, document.Language, ui.TextStyle{Size: theme.SmallTextSize}) + 12
			}
			if document.Changed {
				room -= 230 // the counts, the fold, view and viewed buttons
				if !compact {
					room -= 70 // the status's word
				}
			}
		}
		if !compact {
			context.Text(editor.project.Name, quiet)
			context.IconSized("chevron-right", 12, theme.InkQuiet.WithAlpha(0.6))
		}
		if folder = fitTextEnd(context, folder, quiet, room); len([]rune(folder)) > 3 {
			context.Text(folder, quiet)
			context.IconSized("chevron-right", 12, theme.InkQuiet.WithAlpha(0.6))
		}
		nameInk := theme.Ink
		if document != nil && document.Changed {
			nameInk = statusColour(theme, document.Change.Status)
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
				context.Text(statusName(change.Status), ui.TextStyle{Size: theme.SmallTextSize, Color: statusColour(theme, change.Status)})
			}
			if change.OldPath != change.Path && !compact {
				context.Text("from "+change.OldPath, ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet})
			}
			if editor.added > 0 {
				context.Text(fmt.Sprintf("+%d", editor.added), ui.TextStyle{Color: theme.Positive, Weight: draw2d.Mono, Size: theme.SmallTextSize})
			}
			if editor.gone > 0 {
				context.Text(fmt.Sprintf("−%d", editor.gone), ui.TextStyle{Color: theme.Danger, Weight: draw2d.Mono, Size: theme.SmallTextSize})
			}
		}
		context.Spacer()
		if !compact {
			context.Text(document.Language, ui.TextStyle{Size: theme.SmallTextSize, Color: theme.InkQuiet})
		}
		if !document.Changed {
			return
		}
		context.Box(ui.Layout{Width: ui.Fixed(6)}, nil)
		if editor.mode == diffView {
			icon, tip := "unfold-vertical", "Show every unchanged line (f)"
			if editor.fullContext {
				icon, tip = "fold-vertical", "Fold the unchanged lines (f)"
			}
			if toolButton(context, tip, icon, false) {
				editor.toggleContext()
			}
		}
		mode := int(editor.mode)
		if iconSwitch(context, "view mode", &mode, []iconChoice{
			{"file-diff", "The changes (d)"}, {"file-code", "The whole file (d)"},
		}) {
			editor.setMode(viewMode(mode))
		}
		viewed := application.isViewed(editor.reference)
		icon := "circle"
		if viewed {
			icon = "circle-check"
		}
		if toolButton(context, "Viewed (v)", icon, viewed) {
			application.toggleViewed(editor.reference, true)
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

func (editor *editorState) measure(context *ui.Context) {
	metrics := context.Measurer().MeasureText("M", editor.codeSize, draw2d.Mono, 1)
	editor.charWidth = metrics.Width
	editor.codeAscent, editor.codeDescent = metrics.Ascent, metrics.Descent
}

// gutterMarkWidth is the bar beside a line's number that says it was added or changed.
const gutterMarkWidth float32 = 3

// paint paints the visible lines: numbers in a gutter, coloured code, and the changes: in the
// file view as marks beside the numbers, in the diff on washes of green and red, stronger behind
// the words that changed.
func (editor *editorState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	theme := context.Theme
	look := application.look
	area := editor.list.area
	document := editor.document
	if document == nil {
		return
	}
	if document.Binary || (document.Error != "" && editor.rowCount() == 0) {
		message := "A binary file: nothing to show"
		if !document.Binary {
			message = document.Error
		}
		drawing.Text(message, area.X+24, area.Y+32, theme.TextSize, draw2d.Regular, theme.InkQuiet)
		return
	}
	diff := editor.mode == diffView && document.Changed
	numberColumns := len(strconv.Itoa(max(len(document.NewLines), len(document.OldLines), 1)))
	numberWidth := float32(numberColumns)*editor.charWidth + 16
	gutter := numberWidth + 14
	if diff {
		gutter = numberWidth*2 + editor.charWidth*2 + 6
	}
	codeLeft := area.X + gutter + 10
	codeArea := draw2d.Rectangle{X: codeLeft, Y: area.Y, Width: max(area.Right()-codeLeft-scrollBarWidth, 0), Height: area.Height}
	editor.list.width = 0
	drawing.PushClip(area)
	first, last := editor.list.visible()
	for index := first; index < last; index++ {
		top := editor.list.rowTop(index)
		baseline := top + (editor.rowHeight+editor.codeAscent-editor.codeDescent)/2
		if !diff {
			number := index + 1
			editor.paintNumber(drawing, number, area.X, numberWidth, baseline, look.lineNumber)
			editor.paintMark(drawing, look, editor.marks[number], area.X+numberWidth+4, top)
			editor.paintCode(drawing, application, editor.lines()[index], codeArea, baseline)
			continue
		}
		row := editor.rows[index]
		rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: editor.rowHeight}
		if row.IsFold() {
			drawing.Rectangle(rowArea, 0, look.hover.WithAlpha(0.6), draw2d.Color{}, 0)
			drawing.Icon("unfold-vertical", area.X+gutter-20, top+(editor.rowHeight-12)/2, 12, theme.InkQuiet)
			label := fmt.Sprintf("%d unchanged lines", row.Folded)
			drawing.Text(label, codeLeft, baseline, theme.SmallTextSize, draw2d.Regular, theme.InkQuiet)
			continue
		}
		line := row.Line
		var code highlight.Line
		sign, signColour := "", theme.InkQuiet
		var lineWash, wordWash, edge draw2d.Color
		switch line.Kind {
		case git.Insertion:
			lineWash, wordWash, edge = look.createdLine, look.createdWord, look.created
			sign, signColour = "+", look.created
			code = lineAt(document.NewLines, line.NewNumber)
		case git.Removal:
			lineWash, wordWash, edge = look.deletedLine, look.deletedWord, look.deleted
			sign, signColour = "−", look.deleted
			code = lineAt(document.OldLines, line.OldNumber)
		default:
			code = lineAt(document.NewLines, line.NewNumber)
		}
		if lineWash.IsVisible() {
			drawing.Rectangle(rowArea, 0, lineWash, draw2d.Color{}, 0)
			drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutterMarkWidth, Height: editor.rowHeight}, 0, edge, draw2d.Color{}, 0)
			if span, found := editor.inline[index]; found && span.End > span.Start {
				x := codeLeft + float32(span.Start)*editor.charWidth - editor.list.offsetX
				word := draw2d.Rectangle{X: x, Y: top + 2, Width: float32(span.End-span.Start) * editor.charWidth, Height: editor.rowHeight - 4}
				drawing.PushClip(codeArea)
				drawing.Rectangle(word, 3, wordWash, draw2d.Color{}, 0)
				drawing.PopClip()
			}
		}
		numberInk := look.lineNumber
		if line.Kind != git.Context {
			numberInk = signColour.WithAlpha(0.75)
		}
		editor.paintNumber(drawing, line.OldNumber, area.X, numberWidth, baseline, numberInk)
		editor.paintNumber(drawing, line.NewNumber, area.X+numberWidth, numberWidth, baseline, numberInk)
		if sign != "" {
			drawing.Text(sign, area.X+numberWidth*2+editor.charWidth/2, baseline, editor.codeSize, draw2d.Mono, signColour)
		}
		editor.paintCode(drawing, application, code, codeArea, baseline)
	}
	editor.list.paintBar(drawing, look.scrollThumb)
	editor.paintChangeMarks(drawing, look)
	drawing.PopClip()
}

// paintMark paints a line's mark in the file view's gutter: a bar for a line added (green) or
// changed (amber), a small wedge where lines were removed (red).
func (editor *editorState) paintMark(drawing *draw2d.List, look look, mark review.Mark, x, top float32) {
	switch mark {
	case review.AddedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: editor.rowHeight}, 0, look.created, draw2d.Color{}, 0)
	case review.ModifiedMark:
		drawing.Rectangle(draw2d.Rectangle{X: x, Y: top, Width: gutterMarkWidth, Height: editor.rowHeight}, 0, look.modified, draw2d.Color{}, 0)
	case review.RemovedAbove:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top - 2, Width: 8, Height: 4}, 2, look.deleted, draw2d.Color{}, 0)
	case review.RemovedBelow:
		drawing.Rectangle(draw2d.Rectangle{X: x - 1, Y: top + editor.rowHeight - 2, Width: 8, Height: 4}, 2, look.deleted, draw2d.Color{}, 0)
	}
}

// paintChangeMarks marks where the changes are along the scroll bar, as a map of the file.
func (editor *editorState) paintChangeMarks(drawing *draw2d.List, look look) {
	bar := editor.list.barArea()
	mark := func(index, count int, colour draw2d.Color) {
		y := bar.Y + float32(index)/float32(count)*bar.Height
		drawing.Rectangle(draw2d.Rectangle{X: bar.X - 3, Y: y, Width: 3, Height: max(bar.Height/float32(count), 2)}, 0, colour, draw2d.Color{}, 0)
	}
	if editor.mode == diffView {
		for index, row := range editor.rows {
			switch {
			case row.IsFold() || row.Line.Kind == git.Context:
			case row.Line.Kind == git.Removal:
				mark(index, len(editor.rows), look.deleted)
			default:
				mark(index, len(editor.rows), look.created)
			}
		}
		return
	}
	count := len(editor.lines())
	for number, kind := range editor.marks {
		switch kind {
		case review.AddedMark:
			mark(number-1, count, look.created)
		case review.ModifiedMark:
			mark(number-1, count, look.modified)
		default:
			mark(number-1, count, look.deleted)
		}
	}
}

func lineAt(lines []highlight.Line, number int) highlight.Line {
	if number < 1 || number > len(lines) {
		return highlight.Line{}
	}
	return lines[number-1]
}

func (editor *editorState) paintNumber(drawing *draw2d.List, number int, left, width, baseline float32, colour draw2d.Color) {
	if number <= 0 {
		return
	}
	text := strconv.Itoa(number)
	x := left + width - 8 - float32(len(text))*editor.charWidth
	drawing.Text(text, x, baseline, editor.codeSize, draw2d.Mono, colour)
}

// paintCode paints a line's coloured pieces, shifted by the sideways scroll and cut at the area.
func (editor *editorState) paintCode(drawing *draw2d.List, application *App, line highlight.Line, area draw2d.Rectangle, baseline float32) {
	width := float32(line.Columns) * editor.charWidth
	editor.list.width = max(editor.list.width, width)
	drawing.PushClip(area)
	for _, span := range line.Spans {
		x := area.X + float32(span.Column)*editor.charWidth - editor.list.offsetX
		if x > area.Right() {
			break
		}
		if x+float32(len(span.Text))*editor.charWidth < area.X || isBlank(span.Text) {
			continue
		}
		drawing.Text(span.Text, x, baseline, editor.codeSize, draw2d.Mono, application.look.syntax.Colour(span.Kind))
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
