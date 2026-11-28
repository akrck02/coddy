package uses

import (
	"context"
	"strconv"
	"testing"

	"github.com/akrck02/coddy/editor/internal/builds"
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/ui/mode"
	"github.com/akrck02/coddy/editor/internal/ui/pane"
	"github.com/akrck02/coddy/editor/internal/ui/sidebar"
	"github.com/akrck02/coddy/editor/internal/ui/theme"
	"github.com/akrck02/go-renderer/draw2d"
)

// window is a window of projects for the mode to work in, noting what it is asked.
type window struct {
	projects []*codebase.Project
	revealed []mode.Use
	toast    string
}

func (fake *window) Look() theme.Look                       { return theme.BuiltIn()[0].Look }
func (fake *window) Projects() []*codebase.Project          { return fake.projects }
func (fake *window) Hidden(*codebase.Project) bool          { return false }
func (fake *window) ChangedFiles() []codebase.FileReference { return nil }
func (fake *window) Shows(codebase.FileReference) bool      { return true }
func (fake *window) Reveal(codebase.FileReference)          {}
func (fake *window) FocusViewer()                           {}
func (fake *window) Target() (codebase.FileReference, bool, bool) {
	return codebase.FileReference{}, false, false
}
func (fake *window) ToggleChangedFilter()                               {}
func (fake *window) Loading() bool                                      { return false }
func (fake *window) Toast(words string)                                 { fake.toast = words }
func (fake *window) Relist()                                            {}
func (fake *window) Keys() pane.Keys                                    { return pane.Keys{} }
func (fake *window) Run(_ string, do func(stop context.Context) func()) { do(context.Background())() }
func (fake *window) RevealLine(file codebase.FileReference, line int) {
	fake.revealed = append(fake.revealed, mode.Use{File: file, Line: line})
}

func (fake *window) SearchUsesAgain() {}

// UsersOf says the Keeper class is used by quay's Dock, twice, and lighthouse's Lens.
func (fake *window) UsersOf(use mode.Use, done func(string, []mode.Use)) bool {
	done("Keeper", []mode.Use{
		{File: codebase.FileReference{Project: 0, Path: "Dock.java"}, Line: 5},
		{File: codebase.FileReference{Project: 1, Path: "Lens.java"}, Line: 8},
		{File: codebase.FileReference{Project: 0, Path: "Dock.java"}, Line: 7},
	})
	return true
}

// lamps is the uses mode with the uses of Lamp in three projects: its own, lighthouse, and two
// others, harbour with changes.
func lamps() (*Mode, *window) {
	fake := &window{projects: []*codebase.Project{
		{Name: "quay", Path: "/quay"},
		{Name: "lighthouse", Path: "/lighthouse"},
		{Name: "harbour", Path: "/harbour", Changes: []git.Change{{Path: "Keeper.java"}}},
	}}
	uses := New()
	uses.Start(fake)
	uses.SetUses("Lamp", 1, []mode.Use{
		{File: codebase.FileReference{Project: 0, Path: "Quay.java"}, Line: 4, Start: 4, End: 8},
		{File: codebase.FileReference{Project: 2, Path: "Keeper.java"}, Line: 9, Start: 2, End: 6},
		{File: codebase.FileReference{Project: 1, Path: "Lamp.java"}, Line: 3, Start: 13, End: 17},
		{File: codebase.FileReference{Project: 2, Path: "Keeper.java"}, Line: 2, Start: 7, End: 11},
	}, "")
	return uses, fake
}

func TestUsesGoTheNameOwnProjectFirstThenThoseWithChanges(test *testing.T) {
	uses, fake := lamps()
	for step := 0; step < 4; step++ {
		uses.step(1)
	}
	want := []string{"Lamp.java:3", "Keeper.java:2", "Keeper.java:9", "Quay.java:4"}
	for index, use := range fake.revealed {
		if got := use.File.Path + ":" + strconv.Itoa(use.Line); got != want[index] {
			test.Errorf("use %d is %s, want %s", index, got, want[index])
		}
	}
	if order := uses.ProjectOrder(); order[0] != 1 || order[1] != 2 {
		test.Errorf("the projects go %v", order)
	}
}

func TestTickingAUseCountsItSeenAndGoesOn(test *testing.T) {
	uses, fake := lamps()
	uses.step(1)
	uses.tick()
	if uses.Header() != "1 of 4 seen" || len(fake.revealed) != 2 {
		test.Errorf("after a tick the header says %q, %d uses opened", uses.Header(), len(fake.revealed))
	}
	decoration := uses.Decorate(sidebar.Row{Kind: sidebar.FileRow, Project: 1, Path: "Lamp.java"})
	if decoration.Mark != sidebar.DoneMark || decoration.Words[0].Text != "1" {
		test.Errorf("the file whose use was seen shows %+v", decoration)
	}
}

func TestTheGraphOfTheNamePutsItsUsersOnEitherSide(test *testing.T) {
	uses, _ := lamps()
	graph := uses.nameScene()
	files, groups, left, right := 0, 0, 0, 0
	for _, each := range graph.boxes[1:] {
		switch each.kind {
		case groupBox:
			groups++
			if each.x < 0 {
				left++
			} else {
				right++
			}
		case litBox:
			files++
		}
	}
	if groups != 3 || files != 3 || left != 2 || right != 1 || len(graph.arrows) != 3 {
		test.Errorf("%d projects (%d left, %d right), %d files and %d arrows", groups, left, right, files, len(graph.arrows))
	}
}

func TestProjectsNothingDependsOnAreAtTheTop(test *testing.T) {
	uses, _ := lamps()
	uses.graphs.builds = []builds.Build{{}, {}, {}}
	uses.graphs.links = [][]int{{1}, nil, {0, 1}} // quay needs lighthouse; harbour needs both
	layers := uses.layers([]int{0, 1, 2})
	if len(layers) != 3 || layers[0][0] != 2 || layers[1][0] != 0 || layers[2][0] != 1 {
		test.Errorf("the layers are %v, want [[2] [0] [1]]", layers)
	}
	graph := uses.projectsScene()
	if len(graph.boxes) != 3 || len(graph.arrows) != 3 {
		test.Errorf("%d projects, %d arrows", len(graph.boxes), len(graph.arrows))
	}
}

func TestACanvasFitsItsSceneAndFindsTheBoxUnderThePointer(test *testing.T) {
	uses, _ := lamps()
	graph := uses.nameScene()
	area := draw2d.Rectangle{X: 100, Y: 50, Width: 800, Height: 600}
	shown := canvas{area: area}
	shown.fit(graph)
	for _, each := range graph.boxes {
		left, top := shown.toScreen(each.x, each.y)
		if left < area.X || top < area.Y || left+each.width*shown.zoom > area.Right() || top+each.height*shown.zoom > area.Bottom() {
			test.Fatalf("a box is out of sight at %v, %v", left, top)
		}
	}
	centre := graph.boxes[0]
	x, y := shown.toScreen(centre.x+centre.width/2, centre.y+centre.height/2)
	if found := shown.boxAt(graph, x, y); found != 0 {
		test.Errorf("under the name's middle is box %d", found)
	}
}

func TestAFileOfTheNameGraphOpensALevelFurther(test *testing.T) {
	uses, fake := lamps()
	keeper := codebase.FileReference{Project: 2, Path: "Keeper.java"}
	uses.open(keeper)
	graph := uses.nameScene()
	var callers []box
	for _, each := range graph.boxes {
		if each.reveals {
			callers = append(callers, each)
		}
	}
	if len(callers) != 2 || callers[0].label != "quay · Dock.java" || callers[0].detail != "2" || callers[1].label != "lighthouse · Lens.java" {
		test.Fatalf("the callers are %+v", callers)
	}
	if len(graph.arrows) != 5 || !hasLabel(graph, "Keeper.java › Keeper") {
		test.Errorf("%d arrows, and the file is not labelled with its class", len(graph.arrows))
	}
	for _, each := range graph.boxes {
		if each.label == "Keeper.java › Keeper" && (each.x < 0) != (callers[0].x < 0) {
			test.Error("the callers are not on the file's side")
		}
	}
	uses.open(keeper)
	if graph := uses.nameScene(); len(graph.arrows) != 3 || fake.toast != "" {
		test.Errorf("closed again it has %d arrows, and toasts %q", len(graph.arrows), fake.toast)
	}
}

// hasLabel tells whether a scene has a box with a label.
func hasLabel(graph scene, label string) bool {
	for _, each := range graph.boxes {
		if each.label == label {
			return true
		}
	}
	return false
}

func TestUsesNoServerConfirmedAreCountedApart(test *testing.T) {
	uses, _ := lamps()
	found := append([]mode.Use{}, uses.uses...)
	found = append(found, mode.Use{File: codebase.FileReference{Project: 2, Path: "Pier.kt"}, Line: 5, Start: 10, End: 14, Probable: true})
	uses.SetUses("Lamp", 1, found, "")
	harbour := uses.Decorate(sidebar.Row{Kind: sidebar.ProjectRow, Project: 2})
	if len(harbour.Words) == 0 || harbour.Words[0].Text != "2+1?" {
		test.Errorf("harbour is counted %+v, want 2+1?", harbour.Words)
	}
	if header := uses.Header(); header != "0 of 5 seen · 1 probable" {
		test.Errorf("the header says %q", header)
	}
	pier := codebase.FileReference{Project: 2, Path: "Pier.kt"}
	if uses.fileCount(pier) != "1?" || uses.fileKind(pier) != plainBox {
		test.Errorf("Pier.kt's box says %q, drawn as %d", uses.fileCount(pier), uses.fileKind(pier))
	}
}

// Zooming draws the graph's text at a few sizes only, each drawn once and kept by the renderer.
func TestTheGraphsTextTakesFewSizesWhateverTheZoom(test *testing.T) {
	sizes := map[float32]bool{}
	for zoom := float32(0.05); zoom <= 4; zoom *= 1.01 {
		size := textSize(zoom)
		sizes[size] = true
		if size > boxText*zoom {
			test.Errorf("at zoom %.2f the text is %v, larger than its box's %v", zoom, size, boxText*zoom)
		}
	}
	if len(sizes) > len(textSizes)+1 {
		test.Errorf("zooming draws the text at %d sizes", len(sizes))
	}
	if textSize(0.3) != 0 {
		test.Error("text too small to read is drawn")
	}
}

// The arrows are drawn as small pictures kept from frame to frame: drawing again, or zooming a step
// within a level, draws none anew, and none is large however far the graph is zoomed in.
func TestTheGraphsArrowsAreDrawnOnceAndKept(test *testing.T) {
	graph := scene{boxes: []box{{x: 0, y: 0, width: 120, height: 40}, {x: 900, y: 700, width: 120, height: 40}}, arrows: []arrow{{0, 1}}}
	look := theme.BuiltIn()[0].Look
	shown := &canvas{zoom: 1, area: draw2d.Rectangle{Width: 1200, Height: 900}}
	pictures := func() map[*draw2d.Image]bool {
		drawing := &draw2d.List{}
		shown.paint(drawing, look, graph)
		found := map[*draw2d.Image]bool{}
		for _, command := range drawing.Commands {
			if command.Kind == draw2d.ImageKind {
				found[command.Picture] = true
			}
		}
		return found
	}
	shown.zoom = 1.2
	_ = pictures()
	drawn := len(shown.pieces)
	if drawn == 0 {
		test.Fatal("no arrow was drawn")
	}
	for _, zoom := range []float32{1.2, 1.3, 1.4} { // the same frame again, then wheel steps within a level
		shown.zoom = zoom
		_ = pictures()
		if len(shown.pieces) != drawn {
			test.Errorf("at zoom %.2f, %d pieces were drawn anew", zoom, len(shown.pieces)-drawn)
		}
	}
	for zoom := float32(0.1); zoom <= 4; zoom *= 1.12 {
		shown.zoom = zoom
		for picture := range pictures() {
			if width, height := picture.Size(); width*height > 160*160 {
				test.Errorf("at zoom %.2f a piece of an arrow is %d×%d", zoom, width, height)
			}
		}
	}
}
