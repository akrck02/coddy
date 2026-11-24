package sidebar

import (
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// painting is what every row painted in one frame shares.
type painting struct {
	context  *ui.Context
	window   Window
	drawing  *draw2d.List
	look     theme.Look
	metrics  draw2d.TextMetrics
	pointerX float32
	hovered  int
	// classes reads what Java and Kotlin files declare, nil when the window does not
	classes ClassReader
}

// rowText is what a row's name is drawn with: where it starts, its words, its weight and its ink.
type rowText struct {
	x      float32
	label  string
	weight draw2d.Weight
	ink    draw2d.Color
}

// paint draws the rows in sight, or why there are none, and the scroll bar.
func (sidebar *Sidebar) paint(context *ui.Context, window Window, drawing *draw2d.List) {
	painter := sidebar.startPainting(context, window, drawing)
	area := sidebar.list.Area
	drawing.PushClip(area)
	first, last := sidebar.list.Visible()
	for index := first; index < last; index++ {
		sidebar.paintRow(painter, index)
	}
	if len(sidebar.Rows) == 0 {
		message := "No projects open: press o to open a folder"
		if window.Loading() {
			message = "Reading the projects…"
		}
		drawing.Text(message, area.X+16, area.Y+24, context.Theme.TextSize, draw2d.Regular, context.Theme.InkQuiet)
	}
	sidebar.list.PaintBar(drawing, painter.look.ScrollThumb)
	drawing.PopClip()
	if classes, reads := window.(ClassReader); reads && classes.ReadingKinds() {
		context.RequestFrame(0) // the next frame starts reading the classes that came into sight
	}
}

// startPainting gathers what every row of this frame needs: the look, the text's size and the
// row under the pointer, if no dialog takes the pointer.
func (sidebar *Sidebar) startPainting(context *ui.Context, window Window, drawing *draw2d.List) painting {
	pointerX, pointerY := context.Pointer()
	if sidebar.badges == nil {
		sidebar.badges, sidebar.leftOut = map[int][2]float32{}, map[int][2]float32{}
	}
	clear(sidebar.badges)
	clear(sidebar.leftOut)
	if sidebar.decoratedWithoutStatus == sidebar.ShowsStatus() {
		sidebar.decorateRows() // git's status was hidden or shown since the rows were made
	}
	hovered := -1
	if !window.ModalOpen() {
		hovered = sidebar.list.RowAt(pointerX, pointerY)
	}
	classes, _ := window.(ClassReader)
	return painting{
		context: context, window: window, drawing: drawing, look: window.Look(),
		metrics:  context.Measurer().MeasureText("Ag", context.Theme.TextSize, draw2d.Regular, 1),
		pointerX: pointerX, hovered: hovered, classes: classes,
	}
}

// paintRow draws one row: its background, its icons, what it shows on the right and its name.
func (sidebar *Sidebar) paintRow(painter painting, index int) {
	palette := painter.context.Theme
	area := sidebar.list.Area
	row := sidebar.Rows[index]
	top := sidebar.list.RowTop(index)
	rowArea := draw2d.Rectangle{X: area.X + 4, Y: top + 1, Width: area.Width - 8 - pane.ScrollBarWidth/2, Height: RowHeight - 2}
	sidebar.paintRowBackground(painter, index, rowArea)
	baseline := top + (RowHeight+painter.metrics.Ascent-painter.metrics.Descent)/2
	text := rowText{x: sidebar.rowLeft(row), label: row.Label, weight: draw2d.Regular, ink: palette.Ink}
	if row.Quiet {
		text.ink = palette.InkQuiet
	}
	iconTop := top + (RowHeight-treeIconSize)/2
	switch row.Kind {
	case ProjectRow, FolderRow:
		text = paintFolderIcons(painter, row, text, iconTop)
	case FileRow:
		text = paintFileIcons(painter, row, sidebar.decorations[index], text, iconTop)
	case NoteRow:
		text.ink = palette.InkQuiet
	}
	right := rowArea.Right() - 8
	if row.Kind == ProjectRow && index == painter.hovered {
		right = sidebar.paintEnterButton(painter, index, rowArea, top)
	}
	right = sidebar.paintRight(painter, row, index, right, baseline, top)
	painter.drawing.PushClip(draw2d.Rectangle{X: area.X, Y: top, Width: max(right-area.X-8, 0), Height: RowHeight})
	paintRowName(painter, row, text, baseline)
	painter.drawing.PopClip()
}

// paintRowBackground marks the row selected, a file open, or the row under the pointer.
func (sidebar *Sidebar) paintRowBackground(painter painting, index int, rowArea draw2d.Rectangle) {
	row := sidebar.Rows[index]
	look := painter.look
	reference := workspace.FileReference{Project: row.Project, Path: row.Path}
	isOpen := row.Kind == FileRow && painter.window.IsOpened(reference)
	switch {
	case index == sidebar.Selected && painter.window.SidebarFocused():
		painter.drawing.Rectangle(rowArea, 5, look.Chosen, look.FocusEdge, 1)
	case index == sidebar.Selected, isOpen:
		// the selection of a pane without the keyboard, quieter than the one with it
		painter.drawing.Rectangle(rowArea, 5, look.Hover.Mix(look.Chosen, 0.5), draw2d.Color{}, 0)
	case index == painter.hovered:
		painter.drawing.Rectangle(rowArea, 5, look.Hover, draw2d.Color{}, 0)
	}
}

// paintFolderIcons draws a project's or a folder's chevron and icon, and returns where its name
// starts and how heavy it is.
func paintFolderIcons(painter painting, row Row, text rowText, iconTop float32) rowText {
	palette, icons := painter.context.Theme, painter.look.Icons
	chevron := theme.Icon{Name: "chevron-right", Ink: palette.InkQuiet, Opacity: 1}
	if row.Expanded {
		chevron.Name = "chevron-down"
	}
	if icons != nil {
		chevron.Picture = icons.ChevronPicture(row.Expanded)
	}
	theme.DrawIcon(painter.drawing, painter.look, chevron, text.x, iconTop, treeIconSize)
	text.x += treeIconSize + 4
	name, ink := folderIcon(row, painter.look)
	folder := theme.Icon{Name: name, Ink: ink, Opacity: 1}
	switch {
	case row.Kind == ProjectRow:
		folder, text.weight = theme.Icon{Name: "git-branch", Ink: palette.InkQuiet, Opacity: 1}, draw2d.Medium
	case icons != nil: // an icon theme draws folders by their name, as Zed does: roles are Lucide's
		folder.Picture = icons.FolderPicture(row.Label, row.Expanded)
	}
	if row.Role == workspace.ModuleFolder {
		text.weight = draw2d.Medium
	}
	theme.DrawIcon(painter.drawing, painter.look, folder, text.x, iconTop, treeIconSize)
	text.x += treeIconSize + 7
	return text
}

// paintFileIcons draws a file's mark and icon, and returns where its name starts, the name and
// its ink.
func paintFileIcons(painter painting, row Row, decoration Decoration, text rowText, iconTop float32) rowText {
	palette := painter.context.Theme
	// when the mode marks files, the room a folder gives its chevron holds the mark
	switch decoration.Mark {
	case OpenMark:
		painter.drawing.Icon("circle", text.x, iconTop, treeIconSize, palette.InkQuiet)
	case DoneMark:
		text.ink = palette.InkQuiet
		painter.drawing.Icon("circle-check", text.x, iconTop, treeIconSize, palette.InkSoft)
	}
	text.x += treeIconSize + 4
	text.label = paintFileIcon(painter, row, text, iconTop)
	text.x += treeIconSize + 7
	// the name may say how the file changed, as Zed's project panel does; where every file
	// listed changed, the colour is kept for its counts
	if row.Status != git.Unchanged && decoration.Tinted {
		text.ink = theme.StatusColour(palette, row.Status)
	}
	return text
}

// classKind is what a file declares, UnknownKind when the window reads no classes.
func (painter painting) classKind(file workspace.FileReference) java.ClassKind {
	if painter.classes == nil {
		return java.UnknownKind
	}
	return painter.classes.ClassKind(file)
}

// paintFileIcon draws a file's icon and returns the name it is listed by. A Java or Kotlin class
// is named without its extension and drawn by its kind, as IntelliJ draws it, once the file has
// been read.
func paintFileIcon(painter painting, row Row, text rowText, iconTop float32) string {
	quiet := text.ink == painter.context.Theme.InkQuiet
	reference := workspace.FileReference{Project: row.Project, Path: row.Path}
	if kind := painter.classKind(reference); kind != java.UnknownKind && kind != java.NoClass {
		paintClassBadge(painter.drawing, painter.look, kind, text.x, iconTop, treeIconSize, quiet)
		return strings.TrimSuffix(strings.TrimSuffix(row.Label, ".java"), ".kt")
	}
	icon := theme.FileIcon(row.Label, painter.look)
	if quiet {
		icon = icon.Faded()
	}
	theme.DrawIcon(painter.drawing, painter.look, icon, text.x, iconTop, treeIconSize)
	return text.label
}

// paintEnterButton draws the button that enters the project under the pointer, and returns where
// the row's right side now ends.
func (sidebar *Sidebar) paintEnterButton(painter painting, index int, rowArea draw2d.Rectangle, top float32) float32 {
	button := draw2d.Rectangle{X: rowArea.Right() - 4 - enterButtonWidth, Y: top + 3, Width: enterButtonWidth, Height: RowHeight - 6}
	if sidebar.onEnter(index, painter.pointerX) {
		painter.drawing.Rectangle(button, 4, painter.look.Chosen, draw2d.Color{}, 0)
	}
	painter.drawing.Icon("arrow-right-to-line", button.X+(enterButtonWidth-14)/2, top+(RowHeight-14)/2, 14, painter.context.Theme.InkSoft)
	return button.X - 6
}

// paintRowName draws a row's name, and a project's branch after it.
func paintRowName(painter painting, row Row, text rowText, baseline float32) {
	palette := painter.context.Theme
	painter.drawing.Text(text.label, text.x, baseline, palette.TextSize, text.weight, text.ink)
	if row.Kind != ProjectRow {
		return
	}
	labelWidth := painter.context.Measurer().MeasureText(row.Label, palette.TextSize, text.weight, 1).Width
	project := painter.window.Projects()[row.Project]
	if project.Branch != "" {
		painter.drawing.Text(project.Branch, text.x+labelWidth+8, baseline, palette.SmallTextSize, draw2d.Regular, palette.InkQuiet)
	}
}

// rightSide draws small words on a row's right, from right to left.
type rightSide struct {
	painter  painting
	right    float32
	baseline float32
}

// text draws words in a colour left of what is already drawn.
func (side *rightSide) text(words string, colour draw2d.Color) {
	small := side.painter.context.Theme.SmallTextSize
	width := side.painter.context.Measurer().MeasureText(words, small, draw2d.Mono, 1).Width
	side.right -= width
	side.painter.drawing.Text(words, side.right, side.baseline, small, draw2d.Mono, colour)
	side.right -= 8
}

// paintRight paints what a row shows on its right, from right to left, and returns where it ends:
// a file's status letter, a project still being read, and what the mode adds.
func (sidebar *Sidebar) paintRight(painter painting, row Row, index int, right, baseline, top float32) float32 {
	palette := painter.context.Theme
	side := &rightSide{painter: painter, right: right, baseline: baseline}
	switch row.Kind {
	case FileRow:
		if painter.window.Unsaved(workspace.FileReference{Project: row.Project, Path: row.Path}) {
			side.text("●", palette.InkSoft)
		}
		if row.Status == git.Unchanged || !sidebar.ShowsStatus() {
			return side.right
		}
		side.text(row.Status.Letter(), theme.StatusColour(palette, row.Status))
	case ProjectRow:
		if !painter.window.Projects()[row.Project].Loaded {
			side.text("…", palette.InkQuiet)
			return side.right
		}
		sidebar.paintServerBadge(side, row, index, top)
		sidebar.paintLeftOutMark(side, row, index, top)
		if row.Changed == 0 {
			return side.right
		}
	case FolderRow:
	default:
		return side.right
	}
	paintDecoration(side, sidebar.decorations[index], top)
	return side.right
}

// decorate is what the mode adds to a row, without git's status where it is hidden.
func (sidebar *Sidebar) decorate(row Row) Decoration {
	decoration := sidebar.listing.Decorate(row)
	if sidebar.ShowsStatus() {
		return decoration
	}
	return decoration.withoutStatus()
}

// ShowsStatus tells whether rows say how their files changed: unless it is hidden, and always in
// a mode that lists only the changes, where they are what is read.
func (sidebar *Sidebar) ShowsStatus() bool {
	return !sidebar.HidesStatus || sidebar.listing.ChangesOnly()
}

// paintDecoration paints what the mode adds on a row's right: words, a bar of progress and a dot.
func paintDecoration(side *rightSide, decoration Decoration, top float32) {
	palette := side.painter.context.Theme
	drawing := side.painter.drawing
	for _, word := range decoration.Words {
		side.text(word.Text, word.Ink.colour(palette))
	}
	if progress := decoration.Progress; progress != nil {
		bar := draw2d.Rectangle{X: side.right - 28, Y: top + RowHeight/2 - 1.5, Width: 28, Height: 3}
		drawing.Rectangle(bar, 1.5, side.painter.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if progress.Total > 0 && progress.Done > 0 {
			done := bar
			done.Width = bar.Width * float32(progress.Done) / float32(progress.Total)
			drawing.Rectangle(done, 1.5, progress.Ink.colour(palette), draw2d.Color{}, 0)
		}
		side.right -= 36
	}
	if decoration.Dot {
		dot := draw2d.Rectangle{X: side.right - 6, Y: top + RowHeight/2 - 3, Width: 6, Height: 6}
		drawing.Rectangle(dot, 3, palette.Warning.WithAlpha(0.8), draw2d.Color{}, 0)
		side.right -= 14
	}
}
