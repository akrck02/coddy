package settings

import (
	"strings"

	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// keysListWidth is the widest the list of keys grows, so a command and its keys stay close
	keysListWidth = 860
	// keyCapsWidth is the room of a command's keys, on the right of its row
	keyCapsWidth = 300
)

// KeyGroup is the keys of one place: its name, and a row for each command.
type KeyGroup struct {
	Title string
	Rows  []KeyRow
}

// KeyRow is a command as the keys section shows it: its keys, those that do the same, and what it
// does.
type KeyRow struct {
	Keys        []string
	Description string
}

// buildKeys is the keys section: every command with its keys drawn as caps, grouped by where it
// works, each group a card of rows.
func buildKeys(context *ui.Context, window Window) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(keysListWidth), Gap: 24}, func() {
		context.Text("The reader writes a file only when you save it, and git only reads: no commit, no push.",
			ui.TextStyle{Color: palette.InkSoft})
		for _, group := range window.KeyGroups() {
			buildKeysGroup(context, group)
		}
	})
}

// buildKeysGroup is the keys of one place: its name, then a card with a row for each command, a
// hairline between rows.
func buildKeysGroup(context *ui.Context, group KeyGroup) {
	palette := context.Theme
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 10}, func() {
		context.Text(group.Title, ui.TextStyle{Size: palette.TextSize + 1, Weight: draw2d.Bold})
		card := ui.Look{Fill: palette.Sunken, Radius: 10}
		context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(16, 4)}, card, func() {
			for index, row := range group.Rows {
				if index > 0 {
					context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: palette.Line}, nil)
				}
				buildKeysRow(context, row)
			}
		})
	})
}

// buildKeysRow is a command: what it does on the left, its keys on the right, those that do the
// same apart by a dot.
func buildKeysRow(context *ui.Context, row KeyRow) {
	palette := context.Theme
	context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 24, AlignY: ui.Center}, func() {
		// go-ui draws a text on one line, so the words wrap as pieces of a box
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, Wrap: true}, func() {
			for _, word := range strings.Fields(row.Description) {
				context.Label(word)
			}
		})
		context.Box(ui.Layout{Width: ui.Fixed(keyCapsWidth), Gap: 8, Wrap: true, AlignX: ui.End, AlignY: ui.Center}, func() {
			for index, key := range row.Keys {
				if index > 0 { // between two keys that do the same, not pressed together
					context.Text("·", ui.TextStyle{Color: palette.InkQuiet})
				}
				theme.KeyCaps(context, key)
			}
		})
	})
}
