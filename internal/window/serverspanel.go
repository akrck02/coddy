package window

import (
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
			context.Text(view.why, quiet)
			return
		}
		context.Text(application.cardFacts(view), quiet)
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
			context.Text(elapsed, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
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
