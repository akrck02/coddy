package window

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akrck02/code-review/internal/lsp"
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
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 20}, func() {
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

// buildServerList shows every server the window knows of: running, starting, missing or failed.
func (application *Window) buildServerList(context *ui.Context) {
	servers := &application.servers
	keys := make([]string, 0, len(servers.running))
	for key := range servers.running {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		application.buildServerCard(context, key, servers.running[key])
	}
	for _, part := range application.serverParts() {
		switch {
		case servers.missing[part.name]:
			application.buildServerNote(context, part.text, "Install it with: "+application.installOf(part.name))
		case servers.failed[part.name] != "":
			application.buildServerNote(context, part.text, servers.failed[part.name])
		case strings.HasSuffix(part.text, " resting"):
			application.buildServerNote(context, part.text, "Stopped while nobody asks it anything: following a name or asking its uses starts it again.")
		case strings.HasSuffix(part.text, " · starting"):
			application.buildServerNote(context, part.text, "Answering its first message.")
		}
	}
	if len(keys) == 0 && len(application.serverParts()) == 0 {
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

// buildServerNote is a server that does not run, and why.
func (application *Window) buildServerNote(context *ui.Context, title, why string) {
	context.Box(ui.Layout{Direction: ui.Vertical, Gap: 4}, func() {
		context.Text(title, ui.TextStyle{Weight: draw2d.Bold})
		context.Caption(why)
	})
}

// buildServerCard is one running server: what it does, its projects and its last messages.
func (application *Window) buildServerCard(context *ui.Context, key string, client *lsp.Client) {
	progress, now := client.Progress(), time.Now()
	palette := context.Theme
	small := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 6}, func() {
		context.Row(func() {
			context.Text(statusOf(client.Server.Name, progress, now), ui.TextStyle{Weight: draw2d.Bold})
			context.Spacer()
			if context.Button("Stop##" + key) {
				application.restServer(key, client) // the next question starts it again
			}
		})
		started := "started at " + progress.Started.Format("15:04:05") + " · " + plural(len(progress.Projects), "project")
		if usage := application.usageOf(key); usage != "" {
			started += " · " + usage
		}
		context.Text(started, small)
		for _, project := range progress.Projects {
			context.Box(ui.Layout{Gap: 12}, func() {
				context.Box(ui.Layout{Width: ui.Fixed(300)}, func() { context.Text(filepath.Base(project.Path), ui.TextStyle{}) })
				context.Box(ui.Layout{Width: ui.Fixed(90)}, func() { context.Text(project.State, small) })
				if project.Took > 0 {
					context.Text(clock(project.Took), small)
				}
			})
		}
		application.buildServerLog(context, progress.Log)
	})
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
