package uses

import (
	"fmt"
	"path"
	"sort"

	"github.com/akrck02/code-review/internal/builds"
	"github.com/akrck02/code-review/internal/workspace"
	ui "github.com/akrck02/go-ui"
)

// graphs are what the two graphs keep between frames: their canvases, and the projects' builds as
// read last.
type graphs struct {
	name, projects canvas
	builds         []builds.Build
	links          [][]int
	readFor        string // the projects the builds were read for
	reading        bool
}

const (
	fileHeight = 26
	groupTop   = 34 // room for a project's name over its files
	groupGap   = 28
	sideGap    = 160 // between the name and the projects on either side
)

// nameScene lays out who uses the name: the name in the middle, the projects with uses on either
// side, one under the other, nearest first, each holding its files with their counts; an arrow
// goes from each file to the name. A file box's target is its first use.
func (uses *Mode) nameScene() scene {
	label := uses.name
	centre := box{x: -(textWidth(label) + 32) / 2, y: -20, width: textWidth(label) + 32, height: 40, label: label,
		kind: centreBox, target: -1}
	graph := scene{boxes: []box{centre}}
	sides := [2]float32{} // how far down each side is filled
	for order, project := range uses.projectsWithUses() {
		side := order % 2
		files := uses.filesOf(project)
		width := float32(180)
		for _, file := range files {
			width = max(width, textWidth(path.Base(file.Path))+70)
		}
		width = max(width, textWidth(uses.window.Projects()[project].Name)+30)
		height := groupTop + float32(len(files))*(fileHeight+6) + 6
		x := centre.x + centre.width + sideGap
		if side == 0 {
			x = centre.x - sideGap - width
		}
		graph.boxes = append(graph.boxes, box{x: x, y: sides[side], width: width, height: height,
			label: uses.window.Projects()[project].Name, kind: groupBox, target: -1})
		for row, file := range files {
			indexes := uses.byFile[file]
			graph.boxes = append(graph.boxes, box{x: x + 10, y: sides[side] + groupTop + float32(row)*(fileHeight+6),
				width: width - 20, height: fileHeight, label: path.Base(file.Path), detail: fmt.Sprint(len(indexes)),
				kind: litBox, target: indexes[0]})
			graph.arrows = append(graph.arrows, arrow{from: len(graph.boxes) - 1, to: 0})
		}
		sides[side] += height + groupGap
	}
	centreAt(graph.boxes[1:], sides) // both sides centred on the name
	return graph
}

// centreAt moves each side's boxes up so the side is centred on the name, at 0.
func centreAt(boxes []box, sides [2]float32) {
	for index := range boxes {
		side := 1
		if boxes[index].x < 0 {
			side = 0
		}
		boxes[index].y -= (sides[side] - groupGap) / 2
	}
}

// projectsWithUses are the projects with uses, in the order the uses go.
func (uses *Mode) projectsWithUses() []int {
	var projects []int
	seen := map[int]bool{}
	for _, use := range uses.uses {
		if !seen[use.File.Project] {
			seen[use.File.Project] = true
			projects = append(projects, use.File.Project)
		}
	}
	return projects
}

// filesOf are the files of a project with uses, by path.
func (uses *Mode) filesOf(project int) []workspace.FileReference {
	var files []workspace.FileReference
	for file := range uses.byFile {
		if file.Project == project {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(first, second int) bool { return files[first].Path < files[second].Path })
	return files
}

// buildNameGraph shows who uses the name; a click on a file opens it at its first use.
func (uses *Mode) buildNameGraph(context *ui.Context) {
	graph := uses.nameScene()
	if clicked := uses.graphs.name.build(context, uses.window.Keys(), uses.window.Look(), graph); clicked >= 0 {
		if target := graph.boxes[clicked].target; target >= 0 {
			uses.goTo(target)
		}
	}
	context.Caption("A click opens the file at its first use · the wheel zooms · drag to move · 0 fits it in again")
}
