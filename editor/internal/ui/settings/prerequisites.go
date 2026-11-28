package settings

import (
	"strconv"
	"strings"

	"github.com/akrck02/coddy/editor/internal/execution/programs"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/ui/controls"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

// prerequisitesWidth is the width of the sheet of a plugin's prerequisites, in pixels: room for
// an install command on one line
const prerequisitesWidth = 520

// stepNumberSize is the side of the round number of each step, in pixels
const stepNumberSize = 24

// prerequisites is the sheet that tells what a plugin needs before it works, while it shows: the
// plugin, and what to say of each program it misses, its words split once as the sheet opens.
type prerequisites struct {
	plugin *plugins.Plugin
	intro  []string
	steps  []prerequisite
}

// prerequisite is one program a plugin misses: its step's number, its name, what to do, and the
// command that installs it, "" when Coddy runs it or the plugin gives none, with its words to wrap.
type prerequisite struct {
	number       string
	program      string
	advice       []string
	command      string
	commandWords []string
}

// OpenPrerequisites shows what a plugin needs before it works, in a sheet over the plugins.
func (screen *Screen) OpenPrerequisites(plugin *plugins.Plugin) {
	sheet := &prerequisites{plugin: plugin, intro: strings.Fields(introOf(plugin))}
	for index, missing := range plugin.Missing {
		step := prerequisiteOf(missing)
		step.number = strconv.Itoa(index + 1)
		sheet.steps = append(sheet.steps, step)
	}
	screen.prerequisites = sheet
}

// introOf is the line under the sheet's heading: what the plugin waits for.
func introOf(plugin *plugins.Plugin) string {
	if len(plugin.Missing) == 1 {
		return plugin.Name + " starts working once this program is installed."
	}
	return plugin.Name + " starts working once these programs are installed."
}

// prerequisiteOf says what to do about one missing program: let Coddy install it, run its command
// in a terminal, or find it, when the plugin says not how.
func prerequisiteOf(missing plugins.Missing) prerequisite {
	switch {
	case missing.Runnable:
		return prerequisite{program: missing.Program, advice: strings.Fields("Coddy can install it for you.")}
	case missing.Command != "":
		return prerequisite{program: missing.Program, command: missing.Command, commandWords: strings.Fields(missing.Command),
			advice: strings.Fields(handInstallAdvice(missing.Program, missing.Command))}
	}
	return prerequisite{program: missing.Program,
		advice: strings.Fields("The plugin does not say how to install it here. Install it as its own site explains, so that it is on your PATH.")}
}

// handInstallAdvice says why a command is run by hand, in a terminal: it asks for a password,
// which a window has nowhere to type, or the program it runs is not on this computer.
func handInstallAdvice(program, command string) string {
	if programs.AsksForPassword(command) {
		return "Copy the command and run it in a terminal: it asks for your password."
	}
	installer := programs.Installer(command)
	return "This command uses " + installer + ", which is not on this computer. Install " + installer +
		" first, or get " + program + " another way."
}

// buildPrerequisites is the sheet of a plugin's prerequisites, when one is open: a heading with
// the plugin's badge, each program it misses as a numbered step, and Install when Coddy can do it.
func (screen *Screen) buildPrerequisites(context *ui.Context, window Window) {
	sheet := screen.prerequisites
	if sheet == nil {
		return
	}
	dismissed := context.Sheet("Prerequisites", ui.SheetOptions{Width: ui.Fixed(prerequisitesWidth)}, func() {
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 12, Padding: ui.Padding{Top: 4, Bottom: 12}}, func() {
			buildPrerequisitesHeading(context, window, sheet)
			for _, step := range sheet.steps {
				buildPrerequisite(context, window, step)
			}
		})
		buildPrerequisitesFooter(context, window, sheet.plugin)
	})
	if dismissed || len(sheet.plugin.Missing) == 0 && !sheet.plugin.Installing {
		screen.prerequisites = nil
	}
}

// buildPrerequisitesHeading is the top of the sheet: the plugin's badge, "Almost ready", and what
// it waits for.
func buildPrerequisitesHeading(context *ui.Context, window Window, sheet *prerequisites) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 14, AlignY: ui.Center, Padding: ui.Padding{Bottom: 4}}, func() {
		buildBadge(context, window, sheet.plugin.Badge)
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 4}, func() {
			context.Text("Almost ready", ui.TextStyle{Size: palette.TextSize + 3, Weight: draw2d.Bold})
			buildWrapped(context, sheet.intro, ui.TextStyle{Color: palette.InkSoft})
		})
	})
}

// buildPrerequisite is one missing program as a step on a card of its own: its number, its name
// with a "Not installed" tag, what to do, and the command to copy.
func buildPrerequisite(context *ui.Context, window Window, step prerequisite) {
	palette := context.Theme
	context.Push(step.program)
	defer context.Pop()
	card := ui.Look{Fill: window.Look().Editor, Border: palette.Line, BorderWidth: 1, Radius: 10}
	context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.Pad(14), Gap: 12}, card, func() {
		buildStepNumber(context, step.number)
		context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 8}, func() {
			context.Box(ui.Layout{Gap: 8, AlignY: ui.Center}, func() {
				context.Text(step.program, ui.TextStyle{Size: palette.TextSize + 1, Weight: draw2d.Bold})
				buildTag(context, "Not installed", warningInk)
			})
			buildWrapped(context, step.advice, ui.TextStyle{Color: palette.InkSoft})
			if step.command != "" {
				buildCommand(context, step)
			}
		})
	})
}

// buildStepNumber is a step's number in a circle of the main colour's tint.
func buildStepNumber(context *ui.Context, number string) {
	palette := context.Theme
	circle := ui.Layout{Width: ui.Fixed(stepNumberSize), Height: ui.Fixed(stepNumberSize), AlignX: ui.Center, AlignY: ui.Center}
	context.Frame(circle, ui.Look{Fill: palette.PrimaryTint, Radius: stepNumberSize / 2}, func() {
		context.Text(number, ui.TextStyle{Size: palette.SmallTextSize, Weight: draw2d.Bold, Color: palette.Primary})
	})
}

// buildTag is a small word on a wash of its colour: a state beside a name.
func buildTag(context *ui.Context, word string, colour draw2d.Color) {
	context.Frame(ui.Layout{Padding: ui.PadXY(8, 2)}, ui.Look{Fill: colour.WithAlpha(0.15), Radius: 10}, func() {
		context.Text(word, ui.TextStyle{Size: context.Theme.SmallTextSize, Weight: draw2d.Medium, Color: colour})
	})
}

// buildCommand is the command that installs a program, to run in a terminal: after a prompt, in
// a code font on a sunken strip, wrapping when it is long, with a button that copies it.
func buildCommand(context *ui.Context, step prerequisite) {
	palette := context.Theme
	code := ui.TextStyle{Weight: draw2d.Mono}
	strip := ui.Look{Fill: palette.Sunken, Radius: 8}
	context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.Padding{Left: 12, Right: 4, Top: 6, Bottom: 6}, Gap: 8, AlignY: ui.Center}, strip, func() {
		context.Text("$", ui.TextStyle{Weight: draw2d.Mono, Color: palette.InkQuiet})
		buildWrapped(context, step.commandWords, code)
		if controls.LabelledButton(context, "Copy the command", "copy", "Copy") {
			context.Copy(step.command)
		}
	})
}

// buildPrerequisitesFooter is the sheet's last row, while Coddy can install something: Install,
// or what it is doing while it installs. The sheet's own cross closes it.
func buildPrerequisitesFooter(context *ui.Context, window Window, plugin *plugins.Plugin) {
	if !plugin.Installable && !plugin.Installing {
		return
	}
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center}, func() {
		context.Spacer()
		if plugin.Installing {
			context.Caption("Installing…")
			return
		}
		if context.PrimaryIconButton("download", "Install") {
			window.InstallMissing(plugin)
		}
	})
}
