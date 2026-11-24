package window

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/theme"
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
	application.measureServers(time.Now())
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
	views := application.serverViews()
	for _, view := range views {
		application.buildServerCard(context, view)
	}
	if len(views) == 0 {
		context.Caption("No language server runs: one starts when a file of its language is opened.")
	}
}

// installOf is the command that installs a server of the table.
func (application *Window) installOf(name string) string {
	for _, server := range application.servers.table {
		if server.Name == name {
			return server.Install
		}
	}
	return ""
}

// languageOf is the language a server reads, as people call it ("Java" for jdtls); its own name
// for one the window does not know.
func (application *Window) languageOf(name string) string {
	for _, server := range application.servers.table {
		if server.Name == name && len(server.Languages) > 0 {
			return languageNames[server.Languages[0]]
		}
	}
	return name
}

// languageNames are the protocol's names of languages as people write them.
var languageNames = map[string]string{"go": "Go", "java": "Java", "kotlin": "Kotlin", "typescript": "TypeScript",
	"javascript": "JavaScript", "python": "Python", "rust": "Rust"}

// buildServerCard is one server: its state, a few words of what it does and how far it is, and
// on asking, its projects and its last messages.
func (application *Window) buildServerCard(context *ui.Context, view serverView) {
	palette := context.Theme
	card := ui.Look{Fill: palette.Sunken, Radius: 8}
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(14), Gap: 8}, card, func() {
		application.buildCardHead(context, view)
		if view.state == serverWorking && view.fraction >= 0 { // a bar of a length nobody knows would say 0 %
			application.buildBar(context, view.fraction, dotColour(palette, view.state))
		}
		quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
		if view.client == nil {
			measure := func(text string) float32 { return theme.TextWidth(context, text, quiet) }
			for _, line := range wrapLines([]string{view.why}, serversPanelWidth-60, measure) {
				context.Text(line, quiet)
			}
			return
		}
		context.Text(application.cardFacts(view), quiet)
		application.buildGradleLine(context, view)
		buildModelsLine(context, view)
		buildLeftOut(context, view.leftOut)
		if activity := strings.TrimSpace(view.progress.Detail); activity != "" && view.state != serverReady {
			if fitted := theme.FitText(context, activity, quiet, serversPanelWidth-80); fitted != "" {
				context.Text(fitted, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
			}
		}
		application.buildCardDetails(context, view)
	})
}

// buildCardHead is a card's first line: the state's dot, the name, the few words, the time it has
// worked, and the button that stops a running server.
func (application *Window) buildCardHead(context *ui.Context, view serverView) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 10, AlignY: ui.Center}, func() {
		buildDot(context, view.state, 10)
		context.Text(application.languageOf(view.name), ui.TextStyle{Weight: draw2d.Bold})
		context.Text(view.name, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
		context.Text(view.headline, ui.TextStyle{Color: palette.InkSoft})
		context.Spacer()
		if elapsed := view.elapsed(time.Now()); elapsed != "" {
			if last := lastTime(view); last != "" {
				elapsed += " · " + last
			}
			context.Text(elapsed, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
		}
		if view.installable && context.PrimaryButton("Install##"+view.name) {
			application.installServer(view.name)
		}
		if view.client != nil && context.Button("Stop##"+view.key) {
			application.restServer(view.key, view.client) // the next question starts it again
		}
	})
}

// cardFacts are a running server's facts in a line: its projects, what it takes, when it started.
func (application *Window) cardFacts(view serverView) string {
	facts := plural(len(view.progress.Projects), "project")
	if usage := application.usageOf(view.key); usage != "" {
		facts += " · " + usage
	}
	return facts + " · since " + view.progress.Started.Format("15:04")
}

// buildCardDetails is the link that shows a server's projects and its last messages, and them.
func (application *Window) buildCardDetails(context *ui.Context, view serverView) {
	if application.serverDetails == nil {
		application.serverDetails = map[string]bool{}
	}
	open := application.serverDetails[view.name]
	label, icon := "Show details", "chevron-right"
	if open {
		label, icon = "Hide details", "chevron-down"
	}
	palette := context.Theme
	hovered := ui.Look{Fill: application.look.Hover, Radius: 4}
	if context.Clickable("details "+view.name, ui.Layout{Padding: ui.PadXY(4, 2), Gap: 4, AlignY: ui.Center}, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		context.IconSized(icon, 12, palette.Primary)
		context.Text(label, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary})
	}) {
		application.serverDetails[view.name] = !open
	}
	if open {
		application.buildServerProjects(context, view.progress.Projects)
		application.buildServerLog(context, view.progress.Log)
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
				took = clock(project.Took)
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

// plural writes a count of things: "1 project", "22 projects".
func plural(count int, thing string) string {
	if count == 1 {
		return "1 " + thing
	}
	return strconv.Itoa(count) + " " + thing + "s"
}

// buildGradleLine says what a server that imports Gradle builds does with Gradle: its daemons
// running, or those the reader stopped once it was done importing.
func (application *Window) buildGradleLine(context *ui.Context, view serverView) {
	if !view.client.Server.OwnGradle {
		return
	}
	palette := context.Theme
	quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	running := application.servers.usage[view.key].daemons
	released, at := view.client.Released()
	switch {
	case running > 0:
		context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
			context.IconSized("cog", 12, palette.InkSoft)
			context.Text(fmt.Sprintf("Gradle: %s of its own running, stopped once it is done; yours are left alone", plural(running, "daemon")), quiet)
		})
	case released > 0:
		context.Box(ui.Layout{Gap: 6, AlignY: ui.Center}, func() {
			context.IconSized("check", 12, palette.Positive)
			context.Text(fmt.Sprintf("Gradle stopped once done: %s released at %s", plural(released, "daemon"), at.Format("15:04")), quiet)
		})
	}
}

// buildModelsLine says, for a server given the models of its projects, how it read them: from
// their models, or imported with Gradle and their models kept.
func buildModelsLine(context *ui.Context, view serverView) {
	if !view.client.Server.Models {
		return
	}
	words, trouble := modelWords(view.client.Models())
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
		parts = append(parts, plural(fromModel, "project")+" from their models, no Gradle")
	}
	switch {
	case imported == 0:
	case why != "":
		parts = append(parts, plural(imported, "project")+" imported with Gradle; their models could not be kept: "+why)
	case keptAt.IsZero():
		parts = append(parts, plural(imported, "project")+" imported with Gradle; their models are kept once it is done")
	default:
		parts = append(parts, plural(imported, "project")+" imported with Gradle, models kept at "+keptAt.Format("15:04"))
	}
	return strings.Join(parts, " · "), why != ""
}

// buildLeftOut lists the projects a server leaves out, each with why: their uses are found by
// their text only.
func buildLeftOut(context *ui.Context, left []leftOutProject) {
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
	measure := func(text string) float32 { return theme.TextWidth(context, text, small) }
	for _, project := range left {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 12}, func() {
			context.Box(ui.Layout{Width: ui.Fixed(200)}, func() { context.Text(project.name, ui.TextStyle{Size: palette.SmallTextSize}) })
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow()}, func() {
				for _, line := range wrapLines([]string{capitalised(project.why) + "."}, serversPanelWidth-290, measure) {
					context.Text(line, small)
				}
			})
		})
	}
}
