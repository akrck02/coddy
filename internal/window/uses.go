package window

import (
	"context"
	"sort"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/git"
	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/navigation"
	"github.com/akrck02/code-review/internal/viewer"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// usesAsked lists every use of the name at the caret, or clicked last, in the finder's panel: those
// the file's server knows, then those in other projects in sight, checked one by one.
func (application *Window) usesAsked() {
	place, found := application.viewer.PlaceAsked()
	if !found {
		application.toast = "Click a name first, or place the caret on it"
		return
	}
	asked, ready := application.readyServer()
	if !ready {
		return
	}
	name := navigation.WordAt(application.viewer.LineText(place.Line), place.Column)
	application.searchUses(navigation.Question{Name: name, Home: application.viewer.Reference.Project,
		Asked: asked, At: application.positionOf(place)})
}

// searchUses searches the uses of a name at a place of a file its server reads.
func (application *Window) searchUses(question navigation.Question) {
	asked, at := question.Asked, question.At
	projects, options := application.inSightPaths(), application.wordSearch()
	application.waitFor(asked.Client)
	application.work.Run("references", func(stop context.Context) func() {
		ctx, cancel := context.WithTimeout(stop, navigation.RequestWait)
		defer cancel()
		asked.TellFirst()
		answer := navigation.Answer{Busy: !asked.Client.Ready()}
		locations, err := asked.Client.References(ctx, asked.Path, at)
		answer.Locations = locations
		answer.Declared, _ = asked.Client.Definition(ctx, asked.Path, at) // no declaration: only the server's uses are listed
		answer.Hits = navigation.Grep(stop, application.work, projects, question.Name, options)
		return func() {
			application.waiting = nil
			if err != nil {
				application.toast = "The uses could not be found: " + err.Error()
				return
			}
			application.uses.Begin(question, answer)
		}
	})
}

// inSightPaths are the folders of the projects in sight that are read.
func (application *Window) inSightPaths() []string {
	var paths []string
	for index, project := range application.projects {
		if application.Searchable(index) {
			paths = append(paths, project.Path)
		}
	}
	return paths
}

// wordSearch searches for a name as a whole word in the files a server of the table reads.
func (application *Window) wordSearch() git.GrepOptions {
	options := git.GrepOptions{WorkingTree: application.settings.WorkingTree, Word: true}
	for extension, language := range lsp.LanguageIDs() {
		if application.servers.Reads(language) {
			options.Files = append(options.Files, "*"+extension)
		}
	}
	sort.Strings(options.Files)
	return options
}

// Hover asks the server of the file open about the name the pointer rests on, and drops the card
// once the pointer leaves it. It says nothing when there is no server: resting is not asking.
func (application *Window) Hover(place viewer.Place, resting bool) {
	if !resting {
		application.work.Cancel("hover")
		application.hover = navigation.HoverCard{}
		return
	}
	asked, why := application.runningServer(false)
	if why != "" {
		return
	}
	at := application.positionOf(place)
	application.hover = navigation.HoverCard{Place: place, Resting: true, X: application.pointerX, Y: application.pointerY}
	application.work.Run("hover", func(stop context.Context) func() {
		ctx, cancel := context.WithTimeout(stop, navigation.RequestWait)
		defer cancel()
		asked.TellFirst()
		text, err := asked.Client.Hover(ctx, asked.Path, at)
		return func() {
			if err == nil && application.hover.Resting && application.hover.Place == place {
				application.hover.Text = text
			}
		}
	})
}

const (
	hoverLineLimit = 14
	hoverWidth     = 520
	// hoverPaddingX and hoverPaddingY are the room around the card's text, in pixels: more above
	// and below than a line's own air, so the first and last lines do not touch the edge
	hoverPaddingX = 12
	hoverPaddingY = 14
	// hoverLineHeight is how tall each line of the card is, its gap included
	hoverLineHeight = 20
)

// buildHover shows the hover card under the pointer, kept inside the window, its long lines
// wrapped to its width.
func (application *Window) buildHover(context *ui.Context) {
	card := application.hover
	if !card.Resting || application.ModalOpen() {
		return
	}
	width, height := context.Size()
	cardWidth := min(hoverWidth, width-16)
	palette := context.Theme
	size := palette.SmallTextSize + 1
	measure := func(text string) float32 { return context.Measurer().MeasureText(text, size, draw2d.Regular, 1).Width }
	lines := limitLines(wrapLines(navigation.HoverLines(card.Text), cardWidth-2*hoverPaddingX, measure))
	if len(lines) == 0 {
		return
	}
	tall := float32(len(lines))*hoverLineHeight + 2*hoverPaddingY
	left := max(min(card.X+8, width-cardWidth-8), 8)
	top := card.Y + 20
	if top+tall > height-8 {
		top = max(card.Y-tall-8, 8)
	}
	options := ui.LayerOptions{AlignX: ui.Start, AlignY: ui.Start, Padding: ui.Padding{Left: left, Top: top}, Width: ui.Fixed(cardWidth)}
	look := ui.Look{Fill: palette.Surface, Border: application.look.Chosen, BorderWidth: 1, Radius: 8, Shadow: true}
	context.Overlay("hover", options, func() {
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(hoverPaddingX, hoverPaddingY), Gap: 2}, look, func() {
			for _, line := range lines {
				context.Text(line, ui.TextStyle{Size: size, Color: palette.Ink})
			}
		})
	})
}

// limitLines keeps a card's first lines, an ellipsis after them when there were more.
func limitLines(lines []string) []string {
	if len(lines) > hoverLineLimit {
		return append(lines[:hoverLineLimit], "…")
	}
	return lines
}

// ShowServerFiles lists the files language servers wrote in a project in the finder's panel, to
// open any of them.
func (application *Window) ShowServerFiles(project int) {
	if project < 0 || project >= len(application.projects) {
		return
	}
	var files []finder.Use
	for _, file := range application.projects[project].ServerFiles {
		// the reader cannot tell who wrote a file outside it: it says what it saw
		note := "new while " + file.Server + " ran: by it, or by you outside the reader"
		if file.Existed {
			note = "changed while " + file.Server + " ran: by it, or by you outside the reader"
		}
		files = append(files, finder.Use{Project: project, Path: file.Path, Line: 1, Text: note})
	}
	application.finder.ShowUses(application, "the files servers wrote", files)
}
