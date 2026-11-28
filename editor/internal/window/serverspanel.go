package window

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/akrck02/coddy/editor/internal/controls"
	"github.com/akrck02/coddy/editor/internal/english"
	"github.com/akrck02/coddy/editor/internal/language"
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/servers"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	serversPanelWidth = 760
	// serversRoomAround is the window's height the panel leaves to its title, button and margins
	serversRoomAround = 180
	// serversLogLines is how many of a server's last messages the panel shows
	serversLogLines = 12
	// serversLogWidth is how many characters of a message the panel shows, on one line
	serversLogWidth = 100
)

// buildServers is the servers panel: for each language server, what it does, each project it
// reads with how far it is and how long it took, and its last messages.
func (application *Window) buildServers(context *ui.Context) {
	if !application.serversOpen {
		return
	}
	application.servers.Measure(time.Now())
	dismissed := context.Sheet("Language servers", ui.SheetOptions{Width: ui.Fixed(serversPanelWidth)}, func() {
		_, windowHeight := context.Size()
		context.ScrollArea("servers", ui.Layout{Width: ui.Grow(), Height: ui.Fixed(max(windowHeight-serversRoomAround, 200))}, func() {
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 12}, func() {
				application.buildServerList(context)
			})
		})
		context.Row(func() {
			context.Spacer()
			if context.PrimaryButton("Close") {
				application.serversOpen = false
			}
		})
	})
	if dismissed {
		application.serversOpen = false
	}
}

// buildServerList shows a card for every server the window knows of: running, starting, resting,
// missing or failed.
func (application *Window) buildServerList(context *ui.Context) {
	views := application.servers.Views()
	for _, view := range views {
		application.buildServerCard(context, view)
	}
	if len(views) == 0 {
		context.Caption("No language server runs: one starts when a file of its language is opened.")
	}
}

// languageOf is the language a server reads, as people call it ("Java" for jdtls); its own name
// for one the window does not know.
func (application *Window) languageOf(name string) string {
	if server, found := application.servers.Named(name); found && len(server.Languages) > 0 {
		return language.NameOfServerLanguage(server.Languages[0])
	}
	return name
}

// buildServerCard is one server: its state, a few words of what it does and how far it is, and
// on asking, its projects and its last messages.
func (application *Window) buildServerCard(context *ui.Context, view servers.View) {
	palette := context.Theme
	card := ui.Look{Fill: palette.Sunken, Radius: 8}
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(14), Gap: 8}, card, func() {
		application.buildCardHead(context, view)
		if view.State == servers.Working && view.Fraction >= 0 { // a bar of a length nobody knows would say 0 %
			application.buildBar(context, view.Fraction, dotColour(palette, view.State))
		}
		quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
		if view.Client == nil {
			measure := func(text string) float32 { return controls.TextWidth(context, text, quiet) }
			for _, line := range wrapLines([]string{view.Why}, serversPanelWidth-60, measure) {
				context.Text(line, quiet)
			}
			return
		}
		context.Text(application.cardFacts(view), quiet)
		application.buildGradleLine(context, view)
		buildModelsLine(context, view)
		buildLeftOut(context, view.LeftOut)
		if activity := strings.TrimSpace(view.Progress.Detail); activity != "" && view.State != servers.Ready {
			if fitted := controls.FitText(context, activity, quiet, serversPanelWidth-80); fitted != "" {
				context.Text(fitted, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
			}
		}
		application.buildCardDetails(context, view)
	})
}

// buildCardHead is a card's first line: the state's dot, the name, the few words, the time it has
// worked, and the button that stops a running server.
func (application *Window) buildCardHead(context *ui.Context, view servers.View) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 10, AlignY: ui.Center}, func() {
		buildDot(context, view.State, 10)
		context.Text(application.languageOf(view.Name), ui.TextStyle{Weight: draw2d.Bold})
		context.Text(view.Name, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
		context.Text(view.Headline, ui.TextStyle{Color: palette.InkSoft})
		context.Spacer()
		if elapsed := view.Elapsed(time.Now()); elapsed != "" {
			if last := lastTime(view); last != "" {
				elapsed += " · " + last
			}
			context.Text(elapsed, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
		}
		if view.Installable && context.PrimaryButton("Install##"+view.Name) {
			application.servers.Install(view.Name)
		}
		if view.Client != nil && context.Button("Stop##"+view.Key) {
			application.servers.Pause(view.Key, view.Client) // the next question starts it again
		}
	})
}

// cardFacts are a running server's facts in a line: its projects, what it takes, when it started.
func (application *Window) cardFacts(view servers.View) string {
	facts := english.Count(len(view.Progress.Projects), "project")
	if usage := application.servers.Usage(view.Key); usage != "" {
		facts += " · " + usage
	}
	return facts + " · since " + view.Progress.Started.Format("15:04")
}

// buildCardDetails is the link that shows a server's projects and its last messages, and them.
func (application *Window) buildCardDetails(context *ui.Context, view servers.View) {
	if application.serverDetails == nil {
		application.serverDetails = map[string]bool{}
	}
	open := application.serverDetails[view.Name]
	label, icon := "Show details", "chevron-right"
	if open {
		label, icon = "Hide details", "chevron-down"
	}
	palette := context.Theme
	hovered := ui.Look{Fill: application.look.Hover, Radius: 4}
	if context.Clickable("details "+view.Name, ui.Layout{Padding: ui.PadXY(4, 2), Gap: 4, AlignY: ui.Center}, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, 12, palette.Primary)
		context.Text(label, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary})
	}) {
		application.serverDetails[view.Name] = !open
	}
	if open {
		application.buildServerProjects(context, view.Progress.Projects)
		application.buildServerLog(context, view.Progress.Log)
	}
}

// buildServerProjects lists the projects a server reads, with how far it is with each.
func (application *Window) buildServerProjects(context *ui.Context, projects []lsp.ProjectProgress) {
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	for _, project := range projects {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 12}, func() {
			context.Box(ui.Layout{Width: ui.Grow()}, func() { context.Text(filepath.Base(project.Path), small) })
			context.Box(ui.Layout{Width: ui.Fixed(80)}, func() { context.Text(project.State, small) })
			took := ""
			if project.Took > 0 {
				took = servers.Clock(project.Took)
			}
			context.Box(ui.Layout{Width: ui.Fixed(44)}, func() { context.Text(took, small) })
		})
	}
}

// buildServerLog shows a server's last messages, errors in the warning colour.
func (application *Window) buildServerLog(context *ui.Context, log []lsp.Message) {
	if len(log) == 0 {
		return
	}
	palette := context.Theme
	context.Text("Last messages", ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Bold, Color: palette.InkSoft})
	for _, message := range log[max(len(log)-serversLogLines, 0):] {
		style := ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Mono, Color: palette.InkSoft}
		if message.Error {
			style.Color = palette.Warning
		}
		text := []rune(message.Text)
		if len(text) > serversLogWidth {
			text = append(text[:serversLogWidth-1], '…')
		}
		context.Text(message.At.Format("15:04:05")+"  "+string(text), style)
	}
}

// buildGradleLine says what a server that imports Gradle builds does with Gradle: its daemons
// running, or those the reader stopped once it was done importing.
func (application *Window) buildGradleLine(context *ui.Context, view servers.View) {
	if !view.Client.Server.Import.OwnGradle {
		return
	}
	palette := context.Theme
	quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	running := application.servers.Daemons(view.Key)
	released, at := view.Client.Released()
	switch {
	case running > 0:
		context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
			context.IconSized("cog", 12, palette.InkSoft)
			context.Text(fmt.Sprintf("Gradle: %s of its own running, stopped once it is done; yours are left alone", english.Count(running, "daemon")), quiet)
		})
	case released > 0:
		context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
			context.IconSized("check", 12, palette.Positive)
			context.Text(fmt.Sprintf("Gradle stopped once done: %s released at %s", english.Count(released, "daemon"), at.Format("15:04")), quiet)
		})
	}
}

// buildModelsLine says, for a server given the models of its projects, how it read them: from
// their models, or imported with Gradle and their models kept.
func buildModelsLine(context *ui.Context, view servers.View) {
	if !view.Client.Server.Import.Models {
		return
	}
	words, trouble := modelWords(view.Client.Models())
	if words == "" {
		return
	}
	palette := context.Theme
	icon, colour := "check", palette.Positive
	if trouble {
		icon, colour = "triangle-alert", palette.Warning
	}
	context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
		context.IconSized(icon, 12, colour)
		context.Text(words, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
	})
}

// modelWords say how a server read its projects, and whether their models could not be kept:
// "16 projects from their models · 3 imported with Gradle, models kept at 15:04".
func modelWords(fromModel, imported int, keptAt time.Time, why string) (string, bool) {
	var parts []string
	if fromModel > 0 {
		parts = append(parts, english.Count(fromModel, "project")+" from their models, no Gradle")
	}
	switch {
	case imported == 0:
	case why != "":
		parts = append(parts, english.Count(imported, "project")+" imported with Gradle; their models could not be kept: "+why)
	case keptAt.IsZero():
		parts = append(parts, english.Count(imported, "project")+" imported with Gradle; their models are kept once it is done")
	default:
		parts = append(parts, english.Count(imported, "project")+" imported with Gradle, models kept at "+keptAt.Format("15:04"))
	}
	return strings.Join(parts, " · "), why != ""
}

// buildLeftOut lists the projects a server leaves out, each with why: their uses are found by
// their text only.
func buildLeftOut(context *ui.Context, left []servers.LeftOutProject) {
	if len(left) == 0 {
		return
	}
	palette := context.Theme
	context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: palette.Line}, nil)
	context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
		context.IconSized("zap-off", 12, palette.Warning)
		context.Text(fmt.Sprintf("Left out of this server (%d): text only", len(left)), ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Medium, Color: palette.Warning})
	})
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	measure := func(text string) float32 { return controls.TextWidth(context, text, small) }
	for _, project := range left {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 12}, func() {
			context.Box(ui.Layout{Width: ui.Fixed(200)}, func() { context.Text(project.Name, ui.TextStyle{Size: palette.SmallTextSize}) })
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow()}, func() {
				for _, line := range wrapLines([]string{english.Capitalised(project.Why) + "."}, serversPanelWidth-290, measure) {
					context.Text(line, small)
				}
			})
		})
	}
}

// dotColour is the colour of a server's state: green when ready, the accent while it works, quiet
// while it starts or rests, the warning's when it cannot run.
func dotColour(palette ui.Theme, state servers.State) draw2d.Color {
	switch state {
	case servers.Ready:
		return palette.Positive
	case servers.Working:
		return palette.Primary
	case servers.Broken:
		return palette.Warning
	}
	return palette.InkQuiet
}

// buildDot is a server's state as a small round mark.
func buildDot(context *ui.Context, state servers.State, size float32) {
	colour := dotColour(context.Theme, state)
	context.Paint(ui.Layout{Width: ui.Fixed(size), Height: ui.Fixed(size)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		if state == servers.Resting {
			drawing.Rectangle(area, size/2, draw2d.Color{}, colour, 1.5)
			return
		}
		drawing.Rectangle(area, size/2, colour, draw2d.Color{}, 0)
	})
}

// buildBar is how far a server is, as a thin bar across its card.
func (application *Window) buildBar(context *ui.Context, fraction float32, colour draw2d.Color) {
	palette := context.Theme
	context.Paint(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(4)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		drawing.Rectangle(area, 2, application.look.Hover.Mix(palette.Ink, 0.12), draw2d.Color{}, 0)
		if fraction > 0 {
			done := area
			done.Width = area.Width * min(fraction, 1)
			drawing.Rectangle(done, 2, colour, draw2d.Color{}, 0)
		}
	})
}
