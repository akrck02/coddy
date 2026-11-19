package window

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// What the reader tells of the servers' work besides their state: the projects a server leaves
// out and why, about how long reading the projects has left, by how long it took the time before.

// leftOutProject is a project a server does not read, and why.
type leftOutProject struct{ name, why string }

// leftOutOf are the projects of the window a server leaves out, by name: those of its languages
// whose build it cannot import (lsp.CannotImport).
func (application *Window) leftOutOf(server lsp.Server) []leftOutProject {
	if !server.OwnGradle {
		return nil
	}
	var left []leftOutProject
	for _, project := range application.projects {
		if why := application.servers.cannotImport[project.Path]; why != "" && hasFileOf(project, server) {
			left = append(left, leftOutProject{name: filepath.Base(project.Path), why: why})
		}
	}
	sort.Slice(left, func(first, second int) bool { return left[first].name < left[second].name })
	return left
}

// leftOutCount is how many projects of the window some server leaves out.
func (application *Window) leftOutCount() int {
	count := 0
	for _, project := range application.projects {
		if application.servers.cannotImport[project.Path] != "" {
			count++
		}
	}
	return count
}

// LeftOut says why the servers leave a project out, for the sidebar's mark; "" when none does.
func (application *Window) LeftOut(project int) string {
	if project < 0 || project >= len(application.projects) {
		return ""
	}
	why := application.servers.cannotImport[application.projects[project].Path]
	if why == "" {
		return ""
	}
	return capitalised(why) + ". Its uses are found by their text, marked ?."
}

// timeLeft says about how long a working server has left, by how long it took the time before:
// "~40 s", "~3 min"; "" when it is ready, was never timed, or already took longer.
func timeLeft(view serverView, now time.Time) string {
	if view.client == nil || view.state == serverReady || view.state == serverBroken {
		return ""
	}
	return leftWords(view.client.LastReading(), now.Sub(view.progress.Started))
}

// leftWords say about how long is left of what took last the time before, elapsed in: "~45 s",
// "~3 min"; "" when it was never timed or is nearly done.
func leftWords(last, elapsed time.Duration) string {
	left := last - elapsed
	switch {
	case last == 0 || left < 5*time.Second:
		return ""
	case left < time.Minute:
		return fmt.Sprintf("~%d s", (int(left.Seconds())+4)/5*5)
	}
	return fmt.Sprintf("~%d min", int((left + 30*time.Second).Minutes()))
}

// lastTime says how long a server took the time before: "last time 1:12"; "" when unknown.
func lastTime(view serverView) string {
	if view.client == nil || view.client.LastReading() == 0 {
		return ""
	}
	return "last time " + clock(view.client.LastReading())
}

// shortStatus is a working server in a few words, for the status bar: "Kotlin · importing 9/16",
// "Java · indexing 45 %".
func (application *Window) shortStatus(view serverView) string {
	words := application.languageOf(view.name) + " · " + strings.ToLower(strings.SplitN(view.headline, " · ", 2)[0])
	done, total := projectsDone(view.progress)
	switch {
	case total > 1:
		words += fmt.Sprintf(" %d/%d", done, total)
	case view.progress.Percent >= 0:
		words += fmt.Sprintf(" %d %%", view.progress.Percent)
	}
	return words
}

// ShowServers shows the servers panel.
func (application *Window) ShowServers() { application.serversOpen = true }

// buildNotes are the notes the pointer resting on something shows: a name's hover card, or why a
// project is left out of the servers.
func (application *Window) buildNotes(context *ui.Context) {
	application.buildHover(context)
	application.buildLeftOutNote(context)
}

// leftOutNoteWidth is the width of the note that says why a project is left out of the servers.
const leftOutNoteWidth = 320

// buildLeftOutNote says, while the pointer rests on a project's mark of being left out of the
// servers in the sidebar, why it is.
func (application *Window) buildLeftOutNote(context *ui.Context) {
	if application.sidebarFolded || application.ModalOpen() {
		return
	}
	x, y := context.Pointer()
	project, found := application.sidebar.PointedLeftOut(x, y)
	if !found {
		return
	}
	palette := context.Theme
	size := palette.SmallTextSize + 1
	measure := func(text string) float32 { return context.Measurer().MeasureText(text, size, draw2d.Regular, 1).Width }
	lines := wrapLines([]string{application.LeftOut(project)}, leftOutNoteWidth-24, measure)
	width, _ := context.Size()
	options := ui.LayerOptions{AlignX: ui.Start, AlignY: ui.Start, Width: ui.Fixed(leftOutNoteWidth),
		Padding: ui.Padding{Left: max(min(x+8, width-leftOutNoteWidth-8), 8), Top: y + 18}}
	look := ui.Look{Fill: palette.Surface, Border: application.look.Chosen, BorderWidth: 1, Radius: 8, Shadow: true}
	context.Overlay("left out", options, func() {
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(12, 10), Gap: 2}, look, func() {
			context.Text("Text only for the Java and Kotlin servers", ui.TextStyle{Size: size, Weight: draw2d.Medium})
			for _, line := range lines {
				context.Text(line, ui.TextStyle{Size: size, Color: palette.InkSoft})
			}
		})
	})
}

// waitingWords say what a question waits for, and about how long is left: "Waiting for kotlin-lsp
// · importing (9 of 16) · 0:48 · ~40 s left".
func waitingWords(client *lsp.Client, now time.Time) string {
	words := "Waiting for " + statusOf(client.Server.Name, client.Progress(), now)
	if left := timeLeft(runningView("", client), now); left != "" {
		words += " · " + left + " left"
	}
	return words
}
