package window

import (
	"fmt"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/english"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/servers"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// What the reader tells of the servers' work besides their state: the projects a server leaves
// out and why, about how long reading the projects has left, by how long it took the time before.

// leftOutCount is how many projects of the window some server leaves out.
func (application *Window) leftOutCount() int {
	count := 0
	for _, project := range application.session.Projects {
		if application.servers.CannotImport(project.Path) != "" {
			count++
		}
	}
	return count
}

// LeftOut says why the servers leave a project out, for the sidebar's mark; "" when none does.
func (application *Window) LeftOut(project int) string {
	if project < 0 || project >= len(application.session.Projects) {
		return ""
	}
	why := application.servers.CannotImport(application.session.Projects[project].Path)
	if why == "" {
		return ""
	}
	return english.Capitalised(why) + ". Its uses are found by their text, marked ?."
}

// timeLeft says about how long a working server has left, by how long it took the time before:
// "~40 s", "~3 min"; "" when it is ready, was never timed, or already took longer.
func timeLeft(view servers.View, now time.Time) string {
	if view.Client == nil || view.State == servers.Ready || view.State == servers.Broken {
		return ""
	}
	return leftWords(view.Client.LastReading(), now.Sub(view.Progress.Started))
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
func lastTime(view servers.View) string {
	if view.Client == nil || view.Client.LastReading() == 0 {
		return ""
	}
	return "last time " + servers.Clock(view.Client.LastReading())
}

// shortStatus is a working server in a few words, for the status bar: "Kotlin · importing 9/16",
// "Java · indexing 45 %".
func (application *Window) shortStatus(view servers.View) string {
	words := application.languageOf(view.Name) + " · " + strings.ToLower(strings.SplitN(view.Headline, " · ", 2)[0])
	done, total := servers.ProjectsDone(view.Progress)
	switch {
	case total > 1:
		words += fmt.Sprintf(" %d/%d", done, total)
	case view.Progress.Percent >= 0:
		words += fmt.Sprintf(" %d %%", view.Progress.Percent)
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
	words := "Waiting for " + servers.StatusOf(client.Server.Name, client.Progress(), now)
	if left := timeLeft(servers.RunningView(client), now); left != "" {
		words += " · " + left + " left"
	}
	return words
}

// The notes and cards over the window are a few lines of words, broken to fit their width.

// wrapLines breaks lines too wide for a width into lines that fit, between words or after a comma
// or an opening bracket, as a signature breaks best; a word wider than the width alone is cut.
// measure gives how wide a text is drawn.
func wrapLines(lines []string, width float32, measure func(string) float32) []string {
	var wrapped []string
	for _, line := range lines {
		for measure(line) > width && strings.TrimSpace(line) != "" {
			cut := breakBefore(line, width, measure)
			wrapped = append(wrapped, strings.TrimRight(line[:cut], " "))
			line = "    " + strings.TrimLeft(line[cut:], " ")
		}
		if strings.TrimSpace(line) != "" || measure(line) <= width {
			wrapped = append(wrapped, line) // a blank line stays; what is left of a broken one, if any
		}
	}
	return wrapped
}

// breakBefore is where a line too wide is broken: after the last space, comma or opening bracket
// that fits, or, with none, after the last character that fits; always after one, past its indent.
func breakBefore(line string, width float32, measure func(string) float32) int {
	fits, breakable := 0, 0
	for index, character := range line {
		end := index + len(string(character))
		if measure(line[:end]) > width {
			break
		}
		fits = end
		if strings.ContainsRune(" ,(<", character) && strings.TrimSpace(line[:end]) != "" {
			breakable = end
		}
	}
	indent := len(line) - len(strings.TrimLeft(line, " "))
	switch {
	case breakable > indent:
		return breakable
	case fits > indent:
		return fits
	}
	_, first := firstRune(line[indent:]) // not even a character fits: one goes, so the line ends
	return indent + first
}

// firstRune is a text's first character and its length in bytes.
func firstRune(text string) (rune, int) {
	for _, character := range text {
		return character, len(string(character))
	}
	return 0, 0
}
