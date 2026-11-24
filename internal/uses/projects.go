package uses

import (
	"context"
	"fmt"
	"github.com/akrck02/code-review/internal/workspace"
	"slices"
	"sort"
	"strings"

	"github.com/akrck02/code-review/internal/builds"
	ui "github.com/akrck02/go-ui"
)

const (
	layerGap   = 90 // between a layer of projects and the next
	projectGap = 24 // between two projects of a layer
	projectTop = 36 // a project box's height
)

// readBuilds reads, in the background, what each project's build depends on, once per set of
// projects read.
func (uses *Mode) readBuilds() {
	projects := uses.window.Projects()
	if uses.graphs.reading || slices.Equal(uses.graphs.readFor, projects) {
		return // a project read again is a new one, so the same pointers are the same projects
	}
	type reading struct {
		path  string
		files []string
	}
	var asked []reading
	for _, project := range projects {
		asked = append(asked, reading{project.Path, project.Files})
	}
	readFor := append([]*workspace.Project(nil), projects...)
	uses.graphs.reading = true
	uses.window.Run("read builds", func(stop context.Context) func() {
		read := make([]builds.Build, len(asked))
		for index, each := range asked {
			read[index] = builds.Read(each.path, each.files)
		}
		return func() {
			uses.graphs.builds, uses.graphs.links = read, builds.Links(read)
			uses.graphs.readFor, uses.graphs.reading = readFor, false
			uses.graphs.projectsKept = nil
			uses.graphs.projects.fitted = false
		}
	})
}

// layers puts each project in sight in a layer: those nothing depends on at the top, each other
// one a layer under the lowest project that depends on it.
func (uses *Mode) layers(inSight []int) [][]int {
	shown := map[int]bool{}
	for _, project := range inSight {
		shown[project] = true
	}
	layer := map[int]int{}
	var place func(project, depth int, path map[int]bool)
	place = func(project, depth int, path map[int]bool) {
		if path[project] || layer[project] > depth {
			return // a cycle, or placed lower already
		}
		layer[project] = depth
		path[project] = true
		for _, dependency := range uses.graphs.links[project] {
			if shown[dependency] {
				place(dependency, depth+1, path)
			}
		}
		delete(path, project)
	}
	for _, project := range inSight {
		if _, placed := layer[project]; !placed || layer[project] == 0 {
			place(project, layer[project], map[int]bool{})
		}
	}
	var layers [][]int
	for _, project := range inSight {
		for len(layers) <= layer[project] {
			layers = append(layers, nil)
		}
		layers[layer[project]] = append(layers[layer[project]], project)
	}
	return layers
}

// keptProjectsScene is the projects graph, laid out again only when the builds were read again,
// or the projects shown changed.
func (uses *Mode) keptProjectsScene() scene {
	shown := uses.graphs.projectsShown[:0]
	for _, project := range uses.window.Projects() {
		shown = append(shown, !uses.window.Hidden(project))
	}
	uses.graphs.projectsShown = shown
	if uses.graphs.projectsKept == nil || !slices.Equal(shown, uses.graphs.projectsKeptShown) {
		graph := uses.projectsScene()
		uses.graphs.projectsKept = &graph
		uses.graphs.projectsKeptShown = append(uses.graphs.projectsKeptShown[:0], shown...)
	}
	return *uses.graphs.projectsKept
}

// projectsScene lays the projects in sight out in layers, each with an arrow to each project it
// depends on; those with changes edged, those that use the name lit. A box's target is its project.
func (uses *Mode) projectsScene() scene {
	var inSight []int
	for index, project := range uses.window.Projects() {
		if !uses.window.Hidden(project) && index < len(uses.graphs.links) {
			inSight = append(inSight, index)
		}
	}
	counts := map[int]int{}
	for _, use := range uses.uses {
		counts[use.File.Project]++
	}
	var graph scene
	placed := map[int]int{} // the box of each project
	for depth, layer := range uses.layers(inSight) {
		sort.Slice(layer, func(first, second int) bool {
			return uses.window.Projects()[layer[first]].Name < uses.window.Projects()[layer[second]].Name
		})
		uses.placeLayer(&graph, placed, layer, float32(depth)*(projectTop+layerGap), counts)
	}
	for _, project := range inSight {
		for _, dependency := range uses.graphs.links[project] {
			if to, shown := placed[dependency]; shown {
				graph.arrows = append(graph.arrows, arrow{from: placed[project], to: to})
			}
		}
	}
	return graph
}

// placeLayer lays a layer of projects out in a row, centred on 0.
func (uses *Mode) placeLayer(graph *scene, placed map[int]int, layer []int, y float32, counts map[int]int) {
	widths, total := make([]float32, len(layer)), float32(0)
	for index, project := range layer {
		widths[index] = textWidth(uses.window.Projects()[project].Name) + 36
		if counts[project] > 0 {
			widths[index] += textWidth(fmt.Sprint(counts[project])) + 10
		}
		total += widths[index] + projectGap
	}
	x := -total / 2
	for index, project := range layer {
		model := uses.window.Projects()[project]
		kind := plainBox
		switch {
		case counts[project] > 0 && len(model.Changes) > 0:
			kind = litChanged
		case counts[project] > 0:
			kind = litBox
		case len(model.Changes) > 0:
			kind = changedBox
		}
		detail := ""
		if counts[project] > 0 {
			detail = fmt.Sprint(counts[project])
		}
		placed[project] = len(graph.boxes)
		graph.boxes = append(graph.boxes, box{x: x, y: y, width: widths[index], height: projectTop,
			label: model.Name, detail: detail, kind: kind, target: project})
		x += widths[index] + projectGap
	}
}

// buildProjectsGraph shows the repositories and what they depend on; what their builds say that
// could not be read is listed under it.
func (uses *Mode) buildProjectsGraph(context *ui.Context) {
	uses.readBuilds()
	if uses.graphs.links == nil {
		context.Caption("Reading the projects' builds…")
		return
	}
	graph := uses.keptProjectsScene()
	if clicked := uses.graphs.projects.build(context, uses.window.Keys(), uses.window.Look(), graph); clicked >= 0 {
		project := uses.window.Projects()[graph.boxes[clicked].target]
		uses.window.Toast(project.Name + " publishes " + strings.Join(uses.graphs.builds[graph.boxes[clicked].target].Publishes, ", "))
	}
	context.Caption(uses.notUnderstood() + "Edged: changed · lit: uses " + uses.name + " · the wheel zooms · drag to move · 0 fits it in again")
}

// notUnderstood says how many dependencies the builds declare in a way not read, if any.
func (uses *Mode) notUnderstood() string {
	count := 0
	for _, build := range uses.graphs.builds {
		count += len(build.NotUnderstood)
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("%d dependencies not understood (a catalog, or made in code) · ", count)
}
