package window

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/akrck02/code-review/internal/finder"
	"github.com/akrck02/code-review/internal/lsp"
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
	name := wordAt(application.viewer.LineText(place.Line), place.Column)
	application.searchUses(asked, application.positionOf(place), name, application.viewer.Reference.Project)
}

// searchUses searches the uses of a name at a place of a file its server reads.
func (application *Window) searchUses(asked asking, at lsp.Position, name string, home int) {
	search := &usesSearch{name: name, home: home, asked: asked, at: at}
	projects, options := application.inSightPaths(), application.wordSearch()
	application.waitFor(asked.client)
	application.work.run("references", func(stop cancellation) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		asked.tellFirst()
		busy := !asked.client.Ready()
		locations, err := asked.client.References(ctx, asked.path, at)
		declared, _ := asked.client.Definition(ctx, asked.path, at)
		hits := application.grepAll(stop, projects, name, options)
		return func() {
			application.waiting = nil
			if err != nil {
				application.toast = "The uses could not be found: " + err.Error()
				return
			}
			search.busy = busy
			application.startUses(search, locations, declared, hits)
		}
	})
}

// useOf is a place a server gave as a use listed in the finder, when it is in a project open.
func (application *Window) useOf(location lsp.Location) (finder.Use, bool) {
	path := lsp.PathOf(location.URI)
	reference, found := application.referenceOf(path)
	if !found {
		return finder.Use{}, false
	}
	start, end := location.Range.Start, location.Range.End
	text := lineOf(path, start.Line, application)
	column := lsp.CharacterColumn(text, start.Character)
	length := 0
	if end.Line == start.Line {
		length = lsp.CharacterColumn(text, end.Character) - column
	}
	byteColumn := len(string([]rune(text)[:min(column, len([]rune(text)))]))
	return finder.Use{Project: reference.Project, Path: reference.Path, Line: start.Line + 1,
		Column: byteColumn, Length: length, Text: text}, true
}

// wordAt is the name a character of a line is in: letters, digits and _.
func wordAt(line string, column int) string {
	characters := []rune(line)
	start, end := min(max(column, 0), len(characters)), min(max(column, 0), len(characters))
	isName := func(character rune) bool {
		return character == '_' || unicode.IsLetter(character) || unicode.IsDigit(character)
	}
	for start > 0 && isName(characters[start-1]) {
		start--
	}
	for end < len(characters) && isName(characters[end]) {
		end++
	}
	return string(characters[start:end])
}

// hoverCard is what a server said of the name the pointer rests on.
type hoverCard struct {
	place   viewer.Place
	resting bool
	text    string
	x, y    float32
}

// Hover asks the server of the file open about the name the pointer rests on, and drops the card
// once the pointer leaves it. It says nothing when there is no server: resting is not asking.
func (application *Window) Hover(place viewer.Place, resting bool) {
	if !resting {
		application.work.cancel("hover")
		application.hover = hoverCard{}
		return
	}
	asked, why := application.runningServer(false)
	if why != "" {
		return
	}
	at := application.positionOf(place)
	application.hover = hoverCard{place: place, resting: true, x: application.pointerX, y: application.pointerY}
	application.work.run("hover", func(stop cancellation) func() {
		ctx, cancel := context.WithTimeout(stop, requestWait)
		defer cancel()
		asked.tellFirst()
		text, err := asked.client.Hover(ctx, asked.path, at)
		return func() {
			if err == nil && application.hover.resting && application.hover.place == place {
				application.hover.text = text
			}
		}
	})
}

// markdownLink is a Markdown link, of which the card keeps the words.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// hoverLines are the lines of a hover's Markdown worth showing in a small card: no code fences,
// rules or emphasis marks, links as their words, and no more than a few.

func hoverLines(markdown string) []string {
	var lines []string
	for _, line := range strings.Split(markdown, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "```") || trimmed == "---" {
			continue // fences and rules: the card shows the words alone
		}
		line = markdownLink.ReplaceAllString(line, "$1")
		line = strings.ReplaceAll(strings.ReplaceAll(line, "**", ""), "`", "")
		if line == "" && (len(lines) == 0 || lines[len(lines)-1] == "") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

const (
	hoverLineLimit = 14
	hoverWidth     = 520
)

// buildHover shows the hover card under the pointer, kept inside the window, its long lines
// wrapped to its width.
func (application *Window) buildHover(context *ui.Context) {
	card := application.hover
	if !card.resting || application.ModalOpen() {
		return
	}
	width, height := context.Size()
	cardWidth := min(hoverWidth, width-16)
	palette := context.Theme
	size := palette.SmallTextSize + 1
	measure := func(text string) float32 { return context.Measurer().MeasureText(text, size, draw2d.Regular, 1).Width }
	lines := limitLines(wrapLines(hoverLines(card.text), cardWidth-24, measure))
	if len(lines) == 0 {
		return
	}
	tall := float32(len(lines))*20 + 20
	left := max(min(card.x+8, width-cardWidth-8), 8)
	top := card.y + 20
	if top+tall > height-8 {
		top = max(card.y-tall-8, 8)
	}
	options := ui.LayerOptions{AlignX: ui.Start, AlignY: ui.Start, Padding: ui.Padding{Left: left, Top: top}, Width: ui.Fixed(cardWidth)}
	look := ui.Look{Fill: palette.Surface, Border: application.look.Chosen, BorderWidth: 1, Radius: 8, Shadow: true}
	context.Overlay("hover", options, func() {
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(12, 10), Gap: 2}, look, func() {
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
