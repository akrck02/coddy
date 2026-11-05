package app

import (
	"fmt"
	"strconv"

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

	list scroller

	codeSize    float32
	rowHeight   float32
	charWidth   float32
	codeAscent  float32
	codeDescent float32
}

func newEditor() editorState {
	return editorState{openedFolds: map[int]bool{}, codeSize: 13, rowHeight: 20, list: scroller{rowHeight: 20}}
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

// show puts a file's text in the editor once it has been read.
func (editor *editorState) show(document *review.Document) {
	keepPlace := editor.document != nil && editor.document.Path == document.Path
	editor.document = document
	if !keepPlace {
		editor.mode = fileView
		if document.Changed {
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
	editor.rows, editor.hunks = nil, nil
	defer func() { editor.list.count = editor.rowCount() }()
	if document == nil {
		return
	}
	if editor.mode == diffView && document.Changed {
		context := foldContext
		if editor.fullContext {
			context = -1
		}
		editor.rows = review.DiffRows(document.Diff, context, editor.openedFolds)
		editor.hunks = review.HunkStarts(editor.rows)
		editor.added, editor.gone = review.Counts(document.Diff)
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
	theme := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow()}, func() {
		editor.buildHeader(context, application)
		context.Separator()
		if !editor.showing {
			editor.buildEmpty(context, application)
			return
		}
		editor.measure(context)
		editor.list.rowHeight = editor.rowHeight
		editor.list.count = editor.rowCount()
		if !application.modalOpen() {
			onBar := editor.list.follow(context, application.keys, theme.ScrollStep)
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
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Height: ui.Grow(), AlignX: ui.Center, AlignY: ui.Center, Gap: 8}, func() {
		context.IconSized("git-compare", 48, context.Theme.InkQuiet)
		context.Heading("Nothing open")
		context.Caption("⌘P finds a file · n opens the next changed file · ? lists the keys")
		if len(application.projects) == 0 {
			if context.PrimaryIconButton("folder-open", "Open a folder") {
				application.pickerOpen = true
			}
		}
	})
}

func (editor *editorState) buildHeader(context *ui.Context, application *App) {
	theme := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(40), Padding: ui.PadXY(12, 0), Gap: 10, AlignY: ui.Center}, func() {
		if !editor.showing {
			context.Caption("No file open")
			return
		}
		document := editor.document
		context.Text(editor.project.Name, ui.TextStyle{Color: theme.InkQuiet})
		context.Text(editor.reference.path, ui.TextStyle{Weight: draw2d.Medium})
		if document == nil {
			context.Caption("Reading…")
			return
		}
		if document.Changed {
			change := document.Change
			context.Chip("", statusName(change.Status), ui.ChipLook{})
			if change.OldPath != change.Path {
				context.Caption("from " + change.OldPath)
			}
			context.Text(fmt.Sprintf("+%d", editor.added), ui.TextStyle{Color: theme.Positive, Weight: draw2d.Mono, Size: theme.SmallTextSize})
			context.Text(fmt.Sprintf("−%d", editor.gone), ui.TextStyle{Color: theme.Danger, Weight: draw2d.Mono, Size: theme.SmallTextSize})
		}
		context.Spacer()
		context.Caption(document.Language)
		if document.Changed {
			if editor.mode == diffView {
				icon, tip := "unfold-vertical", "Show every unchanged line (f)"
				if editor.fullContext {
					icon, tip = "fold-vertical", "Fold the unchanged lines (f)"
				}
				if context.FlatIconButton(icon, "") {
					editor.toggleContext()
				}
				context.Tooltip(tip)
			}
			mode := int(editor.mode)
			if iconSwitch(context, "view mode", &mode, []iconChoice{
				{"file-diff", "The changes (d)"}, {"file-code", "The whole file (d)"},
			}) {
				editor.setMode(viewMode(mode))
			}
		}
	})
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

// paint paints the visible lines: numbers in a gutter, coloured code, and for a diff the added
// and removed lines on green and red.
func (editor *editorState) paint(context *ui.Context, application *App, drawing *draw2d.List) {
	theme := context.Theme
	area := editor.list.area
	document := editor.document
	drawing.Rectangle(area, 0, theme.Sunken, draw2d.Color{}, 0) // Bubble's ground for code
	if document == nil {
		return
	}
	if document.Binary || (document.Error != "" && editor.rowCount() == 0) {
		message := "A binary file: nothing to show"
		if !document.Binary {
			message = document.Error
		}
		drawing.Text(message, area.X+16, area.Y+28, theme.TextSize, draw2d.Regular, theme.InkQuiet)
		return
	}
	diff := editor.mode == diffView && document.Changed
	numberColumns := len(strconv.Itoa(max(len(document.NewLines), len(document.OldLines), 1)))
	numberWidth := float32(numberColumns)*editor.charWidth + 12
	gutter := numberWidth
	if diff {
		gutter = numberWidth*2 + editor.charWidth*2
	}
	codeLeft := area.X + gutter + 8
	codeArea := draw2d.Rectangle{X: codeLeft, Y: area.Y, Width: max(area.Right()-codeLeft-scrollBarWidth, 0), Height: area.Height}
	editor.list.width = 0
	drawing.PushClip(area)
	first, last := editor.list.visible()
	added := theme.Positive.WithAlpha(0.16)
	removed := theme.Danger.WithAlpha(0.16)
	for index := first; index < last; index++ {
		top := editor.list.rowTop(index)
		baseline := top + (editor.rowHeight+editor.codeAscent-editor.codeDescent)/2
		rowArea := draw2d.Rectangle{X: area.X, Y: top, Width: area.Width, Height: editor.rowHeight}
		if !diff {
			editor.paintNumber(drawing, index+1, area.X, numberWidth, baseline, theme.InkQuiet)
			editor.paintCode(drawing, application, editor.lines()[index], codeArea, baseline)
			continue
		}
		row := editor.rows[index]
		if row.IsFold() {
			drawing.Rectangle(rowArea, 0, theme.Sunken.WithAlpha(0.5), draw2d.Color{}, 0)
			label := fmt.Sprintf("⋯  %d unchanged lines (click or Enter to show)", row.Folded)
			drawing.Text(label, codeLeft, baseline, theme.SmallTextSize, draw2d.Regular, theme.InkQuiet)
			continue
		}
		line := row.Line
		var code highlight.Line
		sign := " "
		switch line.Kind {
		case git.Insertion:
			drawing.Rectangle(rowArea, 0, added, draw2d.Color{}, 0)
			drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutter, Height: editor.rowHeight}, 0, added, draw2d.Color{}, 0)
			sign = "+"
			code = lineAt(document.NewLines, line.NewNumber)
		case git.Removal:
			drawing.Rectangle(rowArea, 0, removed, draw2d.Color{}, 0)
			drawing.Rectangle(draw2d.Rectangle{X: area.X, Y: top, Width: gutter, Height: editor.rowHeight}, 0, removed, draw2d.Color{}, 0)
			sign = "−"
			code = lineAt(document.OldLines, line.OldNumber)
		default:
			code = lineAt(document.NewLines, line.NewNumber)
		}
		editor.paintNumber(drawing, line.OldNumber, area.X, numberWidth, baseline, theme.InkQuiet)
		editor.paintNumber(drawing, line.NewNumber, area.X+numberWidth, numberWidth, baseline, theme.InkQuiet)
		signColour := theme.InkQuiet
		if line.Kind == git.Insertion {
			signColour = theme.Positive
		} else if line.Kind == git.Removal {
			signColour = theme.Danger
		}
		drawing.Text(sign, area.X+numberWidth*2+editor.charWidth/2, baseline, editor.codeSize, draw2d.Mono, signColour)
		editor.paintCode(drawing, application, code, codeArea, baseline)
	}
	// the gutter's edge
	drawing.Rectangle(draw2d.Rectangle{X: area.X + gutter, Y: area.Y, Width: 1, Height: area.Height}, 0, theme.Line.WithAlpha(0.5), draw2d.Color{}, 0)
	editor.list.paintBar(drawing, theme.InkQuiet.WithAlpha(0.4))
	editor.paintChangeMarks(drawing, theme)
	drawing.PopClip()
}

// paintChangeMarks marks where the changes are along the scroll bar, as a map of the file.
func (editor *editorState) paintChangeMarks(drawing *draw2d.List, theme ui.Theme) {
	if editor.mode != diffView || len(editor.rows) == 0 {
		return
	}
	bar := editor.list.barArea()
	count := float32(len(editor.rows))
	for index, row := range editor.rows {
		if row.IsFold() || row.Line.Kind == git.Context {
			continue
		}
		colour := theme.Positive
		if row.Line.Kind == git.Removal {
			colour = theme.Danger
		}
		y := bar.Y + float32(index)/count*bar.Height
		drawing.Rectangle(draw2d.Rectangle{X: bar.X - 3, Y: y, Width: 3, Height: max(bar.Height/count, 2)}, 0, colour.WithAlpha(0.8), draw2d.Color{}, 0)
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
	x := left + width - 6 - float32(len(text))*editor.charWidth
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
		drawing.Text(span.Text, x, baseline, editor.codeSize, draw2d.Mono, application.palette.Colour(span.Kind))
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
