// Package sidebar is the tree pane: the projects open, their folders and files, the one selected,
// and the keys that move through them.
package sidebar

import (
	"fmt"
	"strings"

	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/java"
	"github.com/akrck02/code-review/internal/pane"
	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/code-review/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

type RowKind int

const (
	ProjectRow RowKind = iota
	FolderRow
	FileRow
	NoteRow // a line of explanation: projects hidden, a project still loading
)

// Window is what the sidebar needs from the window it sits in.
type Window interface {
	Look() theme.Look
	Keys() pane.Keys
	// ModalOpen tells whether a dialog or the finder is over the window, taking the input.
	ModalOpen() bool
	Projects() []*workspace.Project
	// ProjectIndex is a project's place in Projects, by its path; -1 when it is not open.
	ProjectIndex(path string) int
	// Hidden tells whether a project is left out of the list: one with no changes, while only the
	// projects with changes are shown.
	Hidden(project *workspace.Project) bool
	ShowEveryProject()
	// ToggleChangedFilter shows only the changes, projects and files, or everything again.
	ToggleChangedFilter()
	OpenFile(file workspace.FileReference)
	// OpenedFile is the file the viewer shows, if any.
	OpenedFile() (workspace.FileReference, bool)
	IsOpened(file workspace.FileReference) bool
	// Unsaved tells whether a file has edits not saved.
	Unsaved(file workspace.FileReference) bool
	FocusViewer()
	FocusSidebar()
	SidebarFocused() bool
	// ListEveryFile turns to a mode that lists every file, to show one the development did not
	// change.
	ListEveryFile()
	// ClassKind is what a Java or Kotlin file declares, once read; asking for one not read yet
	// has it read in the background.
	ClassKind(file workspace.FileReference) java.ClassKind
	// Loading tells whether projects are being read, and ReadingKinds whether class kinds are.
	Loading() bool
	ReadingKinds() bool
	// ShowServerFiles lists the files language servers wrote in a project, to open any of them.
	ShowServerFiles(project int)
}

// Row is a row of the sidebar.
type Row struct {
	Kind     RowKind
	Project  int
	Path     string // a folder's or a file's path in the project
	Label    string
	Depth    int
	Expanded bool
	Status   git.Status
	Changed  int // changed files inside a project or a folder
	Quiet    bool
	Role     workspace.FolderRole
}

type Sidebar struct {
	// Listing is the mode that is on, which says what the sidebar lists and how; the window sets it.
	Listing Listing
	// expanded keeps the folders and projects opened or closed by hand, by key; the others follow
	// the default (everything open when only the changes are listed, closed otherwise).
	expanded map[string]bool
	trees    map[treeKey]*workspace.Folder
	Rows     []Row
	// badges are where each project's badge of server files was painted last, by row
	badges   map[int][2]float32
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
	// Filter lists only the files whose path holds each of its words, and the projects named by
	// them, whatever their case; FilterAsked gives its field the keyboard on the next frame.
	Filter      string
	FilterAsked bool
}

type treeKey struct {
	project int
	mode    string
	changed bool
	filter  string
}

func New() Sidebar {
	return Sidebar{expanded: map[string]bool{}, trees: map[treeKey]*workspace.Folder{}, Stale: true, Width: 380, list: pane.Scroller{RowHeight: RowHeight}}
}

func (sidebar *Sidebar) Invalidate() {
	sidebar.trees = map[treeKey]*workspace.Folder{}
	sidebar.Stale = true
}

func (sidebar *Sidebar) expandedKey(project int, path string) string {
	return fmt.Sprintf("%s|%t|%d|%s", sidebar.Listing.Name(), sidebar.Listing.ChangesOnly(), project, path)
}

func (sidebar *Sidebar) isExpanded(window Window, project int, path string, isProject bool) bool {
	if sidebar.filtersFiles(window, project) {
		return true // the files found: all in sight
	}
	if open, chosen := sidebar.expanded[sidebar.expandedKey(project, path)]; chosen {
		return open
	}
	if _, chooses := sidebar.Listing.(Chooser); chooses || sidebar.Listing.ChangesOnly() {
		return true // a few files, chosen: all in sight
	}
	return isProject && len(window.Projects()) == 1
}

func (sidebar *Sidebar) tree(window Window, project int) *workspace.Folder {
	key := treeKey{project, sidebar.Listing.Name(), sidebar.listsChangesOnly(), ""}
	if sidebar.filtersFiles(window, project) {
		key.filter = sidebar.Filter
	}
	if tree, built := sidebar.trees[key]; built {
		return tree
	}
	model := window.Projects()[project]
	isChanged := func(path string) bool { _, changed := model.ChangeOf(path); return changed }
	paths := model.Files
	if sidebar.listsChangesOnly() {
		paths = model.ChangedPaths()
	}
	if chooser, chooses := sidebar.Listing.(Chooser); chooses {
		if chosen, own := chooser.ListedFiles(project); own {
			paths = chosen
		}
	}
	if key.filter != "" {
		paths = sidebar.found(paths)
	}
	// a Gradle or Maven project is shown as IntelliJ shows it: modules, source roots, packages
	build := workspace.BuildTree
	if java.IsProject(model.Files) {
		build = java.BuildTree
	}
	tree := build(paths, isChanged)
	sidebar.trees[key] = tree
	return tree
}

// Rebuild lays the projects out as rows, keeping the selected row on the same thing.
func (sidebar *Sidebar) Rebuild(window Window) {
	var previous Row
	hadSelection := sidebar.Selected >= 0 && sidebar.Selected < len(sidebar.Rows)
	if hadSelection {
		previous = sidebar.Rows[sidebar.Selected]
	}
	sidebar.Rows = sidebar.Rows[:0]
	if sidebar.Entered != "" && window.ProjectIndex(sidebar.Entered) < 0 {
		sidebar.Entered = "" // the project was closed
	}
	if sidebar.Entered != "" {
		sidebar.addEnteredProject(window)
	} else {
		sidebar.addEveryProject(window)
	}
	sidebar.Stale = false
	sidebar.list.Count = len(sidebar.Rows)
	if hadSelection {
		for index, row := range sidebar.Rows {
			if row.Kind == previous.Kind && row.Project == previous.Project && row.Path == previous.Path {
				sidebar.Selected = index
				return
			}
		}
	}
	sidebar.Selected = min(max(sidebar.Selected, 0), max(len(sidebar.Rows)-1, 0))
}

// addEnteredProject lists the content of the project entered, with no row for the project itself.
func (sidebar *Sidebar) addEnteredProject(window Window) {
	for index, project := range window.Projects() {
		if project.Path == sidebar.Entered {
			sidebar.addProjectContent(window, index, project, 0)
		}
	}
}

// addEveryProject lists each project not hidden, with its content when expanded, and says how many
// are hidden.
func (sidebar *Sidebar) addEveryProject(window Window) {
	hidden := 0
	for _, index := range sidebar.projectOrder(window) {
		project := window.Projects()[index]
		if listed, chosen := sidebar.chosenCount(index); chosen {
			if listed == 0 {
				continue // the listing chooses no file of it: not hidden, only not part of it
			}
		} else if window.Hidden(project) {
			hidden++
			continue
		}
		if !sidebar.listedByFilter(window, index) {
			continue
		}
		expanded := sidebar.isExpanded(window, index, "", true)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: ProjectRow, Project: index, Label: project.Name, Expanded: expanded, Changed: len(project.Changes),
			Quiet: project.Loaded && len(project.Changes) == 0,
		})
		if expanded {
			sidebar.addProjectContent(window, index, project, 1)
		}
	}
	if hidden > 0 {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: -1, Quiet: true,
			Label: fmt.Sprintf("%d projects without changes hidden (h shows them)", hidden)})
	}
}

// projectOrder is the order the projects are listed in: the listing's, when it chooses its files.
func (sidebar *Sidebar) projectOrder(window Window) []int {
	if chooser, chooses := sidebar.Listing.(Chooser); chooses {
		if order := chooser.ProjectOrder(); order != nil {
			return order
		}
	}
	order := make([]int, len(window.Projects()))
	for index := range order {
		order[index] = index
	}
	return order
}

// chosenCount is how many files of a project the listing chooses, when it chooses them.
func (sidebar *Sidebar) chosenCount(project int) (int, bool) {
	chooser, chooses := sidebar.Listing.(Chooser)
	if !chooses {
		return 0, false
	}
	paths, own := chooser.ListedFiles(project)
	return len(paths), own
}

// addProjectContent lists a project's tree at a depth, or why it cannot be listed yet.
func (sidebar *Sidebar) addProjectContent(window Window, index int, project *workspace.Project, depth int) {
	if !project.Loaded {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: index, Label: "Reading…", Depth: depth, Quiet: true})
		return
	}
	if project.Error != "" {
		sidebar.Rows = append(sidebar.Rows, Row{Kind: NoteRow, Project: index, Label: project.Error, Depth: depth, Quiet: true})
	}
	sidebar.addFolder(window, index, sidebar.tree(window, index), depth)
}

// Enter takes the sidebar into a project, so only its tree is listed.
func (sidebar *Sidebar) Enter(window Window, project int) {
	if project < 0 || project >= len(window.Projects()) {
		return
	}
	sidebar.Entered = window.Projects()[project].Path
	sidebar.Selected = 0
	sidebar.list.Offset = 0
	sidebar.Rebuild(window)
	if file, open := window.OpenedFile(); open && file.Project == project {
		sidebar.RevealFile(window, file)
	}
}

// Leave goes back to every project, with the one left selected.
func (sidebar *Sidebar) Leave(window Window) {
	left := window.ProjectIndex(sidebar.Entered)
	sidebar.Entered = ""
	sidebar.Rebuild(window)
	for index, row := range sidebar.Rows {
		if row.Kind == ProjectRow && row.Project == left {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return
		}
	}
}

func (sidebar *Sidebar) addFolder(window Window, project int, folder *workspace.Folder, depth int) {
	model := window.Projects()[project]
	for _, child := range folder.Folders {
		expanded := sidebar.isExpanded(window, project, child.Path, false)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: FolderRow, Project: project, Path: child.Path, Label: child.Name, Depth: depth,
			Expanded: expanded, Changed: child.Changed, Role: child.Role,
		})
		if expanded {
			sidebar.addFolder(window, project, child, depth+1)
		}
	}
	for _, path := range folder.Files {
		change, _ := model.ChangeOf(path)
		sidebar.Rows = append(sidebar.Rows, Row{
			Kind: FileRow, Project: project, Path: path, Label: workspace.BaseName(path), Depth: depth, Status: change.Status,
		})
	}
}

// SetExpanded opens or closes a project or a folder.
func (sidebar *Sidebar) SetExpanded(row Row, open bool) {
	sidebar.expanded[sidebar.expandedKey(row.Project, row.Path)] = open
	sidebar.Stale = true
}

// RevealFile opens the folders around a file and selects it.
func (sidebar *Sidebar) RevealFile(window Window, reference workspace.FileReference) {
	project := window.Projects()[reference.Project]
	if sidebar.Entered != "" && sidebar.Entered != project.Path {
		sidebar.Entered = "" // a file of another project: back to every project to show it
	}
	_, changed := project.ChangeOf(reference.Path)
	if !changed && sidebar.onlyChanged {
		window.ShowEveryProject()
	}
	if sidebar.Listing.ChangesOnly() && !changed {
		window.ListEveryFile()
	}
	sidebar.expanded[sidebar.expandedKey(reference.Project, "")] = true
	folder := sidebar.tree(window, reference.Project)
	for {
		var inside *workspace.Folder
		for _, child := range folder.Folders {
			if strings.HasPrefix(reference.Path, child.Path+"/") {
				inside = child
				break
			}
		}
		if inside == nil {
			break
		}
		sidebar.expanded[sidebar.expandedKey(reference.Project, inside.Path)] = true
		folder = inside
	}
	sidebar.Rebuild(window)
	for index, row := range sidebar.Rows {
		if row.Kind == FileRow && row.Project == reference.Project && row.Path == reference.Path {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return
		}
	}
}

// activate does what Enter or a click does on a row: open a file, open or close a folder.
func (sidebar *Sidebar) activate(window Window, index int) {
	if index < 0 || index >= len(sidebar.Rows) {
		return
	}
	row := sidebar.Rows[index]
	switch row.Kind {
	case FileRow:
		window.OpenFile(workspace.FileReference{Project: row.Project, Path: row.Path})
	case ProjectRow, FolderRow:
		sidebar.SetExpanded(row, !row.Expanded)
	case NoteRow:
		if row.Project < 0 {
			window.ShowEveryProject()
		}
	}
}

// MoveSelection moves the selected row by step rows; a pane.PageStep is a page, a pane.EndStep the far end.
func (sidebar *Sidebar) MoveSelection(window Window, step int) {
	switch {
	case step >= pane.EndStep:
		sidebar.Selected = len(sidebar.Rows) - 1
	case step <= -pane.EndStep:
		sidebar.Selected = 0
	case step >= pane.PageStep:
		sidebar.Selected += sidebar.list.PageRows()
	case step <= -pane.PageStep:
		sidebar.Selected -= sidebar.list.PageRows()
	default:
		sidebar.Selected += step
	}
	sidebar.selectionMoved(window)
}

// OpenOrClose opens the selected folder, or goes into it, for a step forward; closes it, or goes
// to its parent, for a step back.
func (sidebar *Sidebar) OpenOrClose(window Window, step int) {
	if step > 0 {
		sidebar.expandOrEnter()
	} else {
		sidebar.collapseOrLeave()
	}
	sidebar.selectionMoved(window)
}

// OpenSelected opens the selected row: enters a project, opens a folder, or shows a file, moving
// the keyboard to it when asked.
func (sidebar *Sidebar) OpenSelected(window Window, moveToFile bool) {
	if sidebar.Rows[sidebar.Selected].Kind == ProjectRow && moveToFile {
		sidebar.Enter(window, sidebar.Rows[sidebar.Selected].Project)
		return
	}
	sidebar.activate(window, sidebar.Selected)
	if sidebar.Selected < len(sidebar.Rows) && sidebar.Rows[sidebar.Selected].Kind == FileRow && moveToFile {
		window.FocusViewer()
	}
	sidebar.selectionMoved(window)
}

// selectionMoved keeps the selection on a row, in sight, and shows the file it is on.
func (sidebar *Sidebar) selectionMoved(window Window) {
	sidebar.Selected = min(max(sidebar.Selected, 0), len(sidebar.Rows)-1)
	sidebar.list.Reveal(sidebar.Selected)
	if !sidebar.Stale {
		sidebar.previewSelected(window)
	}
}

// previewSelected opens the selected file as the selection moves, as a preview.
func (sidebar *Sidebar) previewSelected(window Window) {
	row := sidebar.Rows[sidebar.Selected]
	if row.Kind == FileRow {
		window.OpenFile(workspace.FileReference{Project: row.Project, Path: row.Path})
	}
}

func (sidebar *Sidebar) expandOrEnter() {
	row := sidebar.Rows[sidebar.Selected]
	if row.Kind != ProjectRow && row.Kind != FolderRow {
		return
	}
	if !row.Expanded {
		sidebar.SetExpanded(row, true)
		return
	}
	if sidebar.Selected+1 < len(sidebar.Rows) {
		sidebar.Selected++
	}
}

func (sidebar *Sidebar) collapseOrLeave() {
	row := sidebar.Rows[sidebar.Selected]
	if (row.Kind == ProjectRow || row.Kind == FolderRow) && row.Expanded {
		sidebar.SetExpanded(row, false)
		return
	}
	for index := sidebar.Selected - 1; index >= 0; index-- {
		if sidebar.Rows[index].Depth < row.Depth && sidebar.Rows[index].Kind != NoteRow {
			sidebar.Selected = index
			return
		}
	}
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

// buildHeader names the mode, says how far it has gone if it can, and offers its buttons.
func (sidebar *Sidebar) buildHeader(context *ui.Context, window Window) {
	palette := context.Theme
	title := sidebar.Listing.Name()
	titleStyle := ui.TextStyle{Weight: draw2d.Medium, Color: palette.InkSoft}
	context.Text(title, titleStyle)
	room := sidebar.headerRoom() - theme.TextWidth(context, title, titleStyle) - toolButtonWidth*2 - headerGap*3
	if header := sidebar.Listing.Header(); header != "" {
		progressStyle := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
		if progress := theme.FitText(context, header, progressStyle, room); progress != "" {
			context.Text(progress, progressStyle)
		}
	}
	context.Spacer()
	sidebar.buildChangedFilter(context, window)
	if theme.ToolButton(context, "Collapse all", "chevrons-down-up", false) {
		sidebar.collapseAll(window)
	}
}

// followPointer scrolls the rows with the wheel and the bar, and answers a click on a row: on its
// enter button, on its check, or anywhere else on it.
func (sidebar *Sidebar) followPointer(context *ui.Context, window Window) {
	onBar := sidebar.list.Follow(context, window.Keys(), context.Theme.ScrollStep)
	x, y := context.Pointer()
	index := sidebar.list.RowAt(x, y)
	if index < 0 || onBar || !window.Keys().Clicked() {
		return
	}
	sidebar.Selected = index
	window.FocusSidebar()
	row := sidebar.Rows[index]
	switch {
	case sidebar.onBadge(index, x):
		window.ShowServerFiles(row.Project)
	case sidebar.onEnter(index, x):
		sidebar.Enter(window, row.Project)
	case sidebar.onCheck(index, x):
		sidebar.Listing.Press(workspace.FileReference{Project: row.Project, Path: row.Path}, false)
	default:
		sidebar.activate(window, index)
	}
}

// buildEnteredHeader names the project the sidebar is in, with the way back to every project.
func (sidebar *Sidebar) buildEnteredHeader(context *ui.Context, window Window) {
	palette := context.Theme
	index := window.ProjectIndex(sidebar.Entered)
	if index < 0 {
		return
	}
	project := window.Projects()[index]
	if theme.ToolButton(context, "Every project (Esc)", "arrow-left", false) {
		sidebar.Leave(window)
		return
	}
	count, countStyle := sidebar.enteredCount(context, index, project)
	// what is left for the name and the branch once the buttons and the count have their room
	room := sidebar.headerRoom() - toolButtonWidth - headerGap
	if count != "" {
		room -= theme.TextWidth(context, count, countStyle) + headerGap
	}
	if !sidebar.Listing.ChangesOnly() {
		room -= toolButtonWidth + headerGap
	}
	nameStyle := ui.TextStyle{Weight: draw2d.Medium}
	name := theme.FitText(context, project.Name, nameStyle, room)
	context.Text(name, nameStyle)
	room -= theme.TextWidth(context, name, nameStyle) + headerGap
	branchStyle := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet}
	if branch := theme.FitText(context, project.Branch, branchStyle, room); len([]rune(branch)) > 3 {
		context.Text(branch, branchStyle)
	}
	context.Spacer()
	if count != "" {
		context.Text(count, countStyle)
	}
	sidebar.buildChangedFilter(context, window)
}

// enteredCount is what the mode counts in the project entered, and the style it is written in.
func (sidebar *Sidebar) enteredCount(context *ui.Context, index int, project *workspace.Project) (string, ui.TextStyle) {
	palette := context.Theme
	count, countStyle := "", ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.Warning}
	if len(project.Changes) == 0 {
		return count, countStyle
	}
	decoration := sidebar.Listing.Decorate(Row{Kind: ProjectRow, Project: index, Changed: len(project.Changes)})
	if len(decoration.Words) > 0 {
		count, countStyle.Color = decoration.Words[0].Text, decoration.Words[0].Ink.colour(palette)
	}
	if decoration.Progress != nil {
		countStyle.Color = palette.InkQuiet // the header keeps the bar's colour for the rows
	}
	return count, countStyle
}

// The sidebar's header: its padding, the gap between its parts and an icon button's width.
const (
	headerPadding   float32 = 14
	headerGap       float32 = 8
	toolButtonWidth float32 = 30
)

// headerRoom is the width inside the sidebar's header, less a little for the pane's resize edge.
func (sidebar *Sidebar) headerRoom() float32 {
	return sidebar.Width - 2*headerPadding - 4
}

// listsChangesOnly tells whether the tree lists only the changed files: always in a mode of changes
// only, and in the others when asked.
func (sidebar *Sidebar) listsChangesOnly() bool {
	return sidebar.Listing.ChangesOnly() || sidebar.onlyChanged
}

// SetChangedFilter lists only the changed files, or every file again, in a mode that lists every
// file; a mode of changes only lists them anyway.
func (sidebar *Sidebar) SetChangedFilter(on bool) {
	if sidebar.onlyChanged == on {
		return
	}
	sidebar.onlyChanged = on
	sidebar.Stale = true
	sidebar.list.Offset = 0
}

// buildChangedFilter is the button that shows only the changes: the projects with some, and in a
// mode that lists every file, the changed files.
func (sidebar *Sidebar) buildChangedFilter(context *ui.Context, window Window) {
	if theme.ToolButton(context, "Only the changes: projects and files (g)", "file-diff", sidebar.onlyChanged) {
		window.ToggleChangedFilter()
	}
}

// enterButtonWidth is the room the button that enters a project takes at the right of its row,
// shown while the pointer is on the row.
const enterButtonWidth float32 = 26

// onEnter tells whether a click at x on a row fell on the button that enters its project.
func (sidebar *Sidebar) onEnter(index int, x float32) bool {
	right := sidebar.list.Area.Right() - 4 - pane.ScrollBarWidth/2 - 4
	return sidebar.Rows[index].Kind == ProjectRow && x >= right-enterButtonWidth && x <= right
}

// onCheck tells whether a click at x on a row fell on its mark, when the mode marks files.
func (sidebar *Sidebar) onCheck(index int, x float32) bool {
	row := sidebar.Rows[index]
	if row.Kind != FileRow || sidebar.Listing.Decorate(row).Mark == NoMark {
		return false
	}
	left := sidebar.rowLeft(row)
	return x >= left-4 && x <= left+treeIconSize+4
}

// rowLeft is where a row's chevron (or the room for it) starts.
func (sidebar *Sidebar) rowLeft(row Row) float32 {
	return sidebar.list.Area.X + sidebarMargin + float32(row.Depth)*indentWidth
}

func (sidebar *Sidebar) collapseAll(window Window) {
	for index := range window.Projects() {
		sidebar.expanded[sidebar.expandedKey(index, "")] = false
	}
	sidebar.Stale = true
}

// SelectedRow is the row selected, if any.
func (sidebar *Sidebar) SelectedRow() (Row, bool) {
	if sidebar.Selected < 0 || sidebar.Selected >= len(sidebar.Rows) {
		return Row{}, false
	}
	return sidebar.Rows[sidebar.Selected], true
}

// filtersFiles tells whether the filter chooses which files of a project are listed: it does, unless
// it is empty or names the project itself, which is then listed whole.
func (sidebar *Sidebar) filtersFiles(window Window, project int) bool {
	return sidebar.Filter != "" && !matchesFilter(sidebar.Filter, window.Projects()[project].Name)
}

// listedByFilter tells whether the filter lets a project be listed: by its name, or by a file of it.
func (sidebar *Sidebar) listedByFilter(window Window, project int) bool {
	if !sidebar.filtersFiles(window, project) {
		return true
	}
	return len(sidebar.tree(window, project).Folders)+len(sidebar.tree(window, project).Files) > 0
}

// found are the paths the filter finds.
func (sidebar *Sidebar) found(paths []string) []string {
	var kept []string
	for _, path := range paths {
		if matchesFilter(sidebar.Filter, path) {
			kept = append(kept, path)
		}
	}
	return kept
}

// matchesFilter tells whether a text holds each word of a filter, whatever their case.
func matchesFilter(filter, text string) bool {
	text = strings.ToLower(text)
	for _, word := range strings.Fields(strings.ToLower(filter)) {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

// SetFilter lists only what a filter finds, or everything again for an empty one.
func (sidebar *Sidebar) SetFilter(filter string) {
	sidebar.Filter = filter
	sidebar.Selected = 0
	sidebar.list.Offset = 0
	sidebar.Stale = true
}

// SelectFirstFile selects the first file listed, if any, and tells whether there was one.
func (sidebar *Sidebar) SelectFirstFile() bool {
	for index, row := range sidebar.Rows {
		if row.Kind == FileRow {
			sidebar.Selected = index
			sidebar.list.Reveal(index)
			return true
		}
	}
	return false
}

// buildFilter is the field that filters the files listed by their path: Enter takes the keyboard to
// the first file found, Esc leaves the field as it is.
func (sidebar *Sidebar) buildFilter(context *ui.Context, window Window) {
	palette := context.Theme
	pill := ui.Look{Fill: palette.Sunken, Radius: 6}
	context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.Padding{Left: headerPadding, Right: headerPadding, Bottom: 6}}, func() {
		context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(28), Padding: ui.PadXY(8, 0), Gap: 6, AlignY: ui.Center}, pill, func() {
			context.IconSized("search", 14, palette.InkQuiet)
			if sidebar.FilterAsked {
				context.FocusField("filter")
				sidebar.FilterAsked = false
			}
			filter := sidebar.Filter
			submitted := context.EntryField("filter", &filter, ui.TextFieldOptions{Placeholder: "Filter projects and files (⌘F)", Plain: true,
				Width: ui.Grow(), Size: palette.SmallTextSize})
			if filter != sidebar.Filter {
				sidebar.SetFilter(filter)
			}
			if sidebar.Filter != "" && theme.ToolButton(context, "Clear the filter", "x", false) {
				sidebar.SetFilter("")
			}
			if submitted {
				sidebar.Rebuild(window)
				sidebar.SelectFirstFile()
				window.FocusSidebar()
			}
		})
	})
}
