package window

import (
	"math"
	"strconv"

	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// serversMode is how the reader follows names: by their text alone while no server can answer,
// with servers that are still reading the projects, or with every running server ready.
type serversMode int

const (
	textOnly serversMode = iota
	serversLoading
	serversFull
)

// modeOf is the reader's mode by its servers, how far those still reading are (-1 when they
// cannot tell), and whether one cannot run.
func modeOf(views []serverView) (mode serversMode, fraction float32, broken bool) {
	ready, working, known, sum := 0, false, 0, float32(0)
	for _, view := range views {
		switch view.state {
		case serverReady:
			ready++
		case serverWorking, serverStarting:
			working = true
			if view.fraction >= 0 {
				known, sum = known+1, sum+view.fraction
			}
		case serverBroken:
			broken = true
		}
	}
	fraction = -1
	if known > 0 {
		fraction = sum / float32(known)
	}
	switch {
	case working:
		return serversLoading, fraction, broken
	case ready > 0:
		return serversFull, 1, broken
	}
	return textOnly, -1, broken
}

// modeWords say the mode in a few words, for the button's tooltip.
func modeWords(mode serversMode) string {
	title, explanation := modeTitle(mode)
	return title + ": " + explanation
}

// modeTitle is the mode's name and a short line of what it means for following names.
func modeTitle(mode serversMode) (string, string) {
	switch mode {
	case serversFull:
		return "Ready", "names are followed by what they mean"
	case serversLoading:
		return "Reading the projects", "uses found meanwhile by their text are marked ?"
	}
	return "Text only", "no server answers, so uses are found by their text, marked ?"
}

// buildServersButton is the status bar's button for the language servers: a bolt, crossed out
// while only the text answers, with a ring of how far the servers are while they read the
// projects, green once they are ready, and a warning's dot when one cannot run. A click shows
// each server and its state.
func (application *Window) buildServersButton(context *ui.Context) {
	views := application.serverViews()
	mode, fraction, broken := modeOf(views)
	palette := context.Theme
	hovered := ui.Look{Fill: application.look.Hover, Radius: 4}
	layout := ui.Layout{Padding: ui.PadXY(6, 2), Gap: 5, AlignY: ui.Center}
	if context.Clickable("Language servers", layout, ui.Looks{Hovered: hovered, Pressed: hovered}, func() {
		application.buildModeMark(context, mode, fraction, broken)
		if mode == serversLoading && fraction >= 0 {
			context.Text(strconv.Itoa(int(fraction*100))+" %", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
		}
	}) {
		application.serversListed = !application.serversListed
	}
	context.Popover("servers list", &application.serversListed, ui.Look{Fill: palette.Surface, Radius: 8, Shadow: true}, func() {
		application.buildServersList(context, views, mode)
	})
	if !application.serversListed {
		context.Tooltip(modeWords(mode))
	}
}

// markSize is the side of the servers' mark: its ring, with the bolt inside.
const markSize = 20

// buildModeMark draws the bolt and its ring for the mode.
func (application *Window) buildModeMark(context *ui.Context, mode serversMode, fraction float32, broken bool) {
	palette := context.Theme
	icon, colour := "zap-off", palette.InkQuiet
	switch mode {
	case serversLoading:
		icon, colour = "zap", palette.Primary
	case serversFull:
		icon, colour = "zap", palette.Positive
	}
	context.Paint(ui.Layout{Width: ui.Fixed(markSize), Height: ui.Fixed(markSize)}, func(area draw2d.Rectangle, drawing *draw2d.List) {
		track := application.look.Hover.Mix(palette.Ink, 0.12)
		if mode != textOnly {
			drawing.Stroke(arc(1), area.X, area.Y, 1, 1.6, track)
		}
		if mode == serversFull || mode == serversLoading && fraction > 0 {
			drawing.Stroke(arc(fraction), area.X, area.Y, 1, 1.6, colour)
		}
		drawing.Icon(icon, area.X+4, area.Y+4, 12, colour)
		if broken {
			dot := draw2d.Rectangle{X: area.X + area.Width - 6, Y: area.Y, Width: 6, Height: 6}
			drawing.Rectangle(dot, 3, palette.Warning, draw2d.Color{}, 0)
		}
	})
}

// arc is the part of the mark's ring for a fraction, from its top, clockwise.
func arc(fraction float32) *draw2d.Path {
	radius, centre := float64(markSize)/2-1, float64(markSize)/2
	steps := max(int(60*min(fraction, 1)), 1)
	path := &draw2d.Path{}
	for step := 0; step <= steps; step++ {
		angle := -math.Pi/2 + 2*math.Pi*float64(min(fraction, 1))*float64(step)/float64(steps)
		x, y := float32(centre+radius*math.Cos(angle)), float32(centre+radius*math.Sin(angle))
		if step == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	return path
}

// serversListWidth is the width of the servers' list; what does not fit in it ends with "…".
const serversListWidth = 340

// buildServersList is the popover of the servers' button: the mode in words, each server with its
// state, and the way to the servers panel.
func (application *Window) buildServersList(context *ui.Context, views []serverView, mode serversMode) {
	palette := context.Theme
	quiet := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	clipped := ui.Look{Clip: true}
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(serversListWidth), Padding: ui.Pad(12), Gap: 10}, clipped, func() {
		title, explanation := modeTitle(mode)
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, clipped, func() {
			context.Text(title, ui.TextStyle{Weight: draw2d.Bold})
			context.Text(capitalised(explanation), quiet)
		})
		for _, view := range views {
			application.buildServerLine(context, view, quiet)
		}
		if len(views) == 0 {
			context.Text("None runs: one starts when a file of its language opens.", quiet)
		}
		if context.Button("Details…") {
			application.serversListed, application.serversOpen = false, true
		}
	})
}

// buildServerLine is a server in the list: its dot, language and own name, and under them what it
// does.
func (application *Window) buildServerLine(context *ui.Context, view serverView, quiet ui.TextStyle) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center}, func() {
			buildDot(context, view.state, 8)
			context.Text(application.languageOf(view.name), ui.TextStyle{Weight: draw2d.Medium})
			context.Text(view.name, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet, Weight: draw2d.Mono})
		})
		context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.Padding{Left: 16}}, ui.Look{Clip: true}, func() {
			context.Text(view.headline, quiet)
		})
	})
}
