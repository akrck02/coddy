package settings

import (
	"fmt"

	"github.com/akrck02/code-review/internal/english"
	"github.com/akrck02/code-review/internal/session"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// memoryCardWidth is the width of a budget's card
	memoryCardWidth = 150
	// fileKilobytesLeast and fileKilobytesMost are about how much a usual file of code takes once
	// read with its two versions, its diff and its colours: what a budget's card counts files by
	fileKilobytesLeast = 100
	fileKilobytesMost  = 350
	// kilobytesPerMegabyte is how many kilobytes a megabyte holds
	kilobytesPerMegabyte = 1024
)

// memoryChoice is a budget the memory section offers: its megabytes, its name, how many files of a
// usual size it holds, and its card's label.
type memoryChoice struct {
	megabytes   int
	name, files string
	label       string // what a screen reader says of its card
}

// memoryChoices are the budgets offered, written once.
var memoryChoices = memoryChoicesOf(session.DocumentMegabyteChoices)

// memoryChoicesOf writes a card's words for each budget, in megabytes.
func memoryChoicesOf(budgets []int) []memoryChoice {
	choices := make([]memoryChoice, 0, len(budgets))
	for _, megabytes := range budgets {
		kilobytes := megabytes * kilobytesPerMegabyte
		files := fmt.Sprintf("about %d to %d files", kilobytes/fileKilobytesMost, kilobytes/fileKilobytesLeast)
		name := fmt.Sprintf("%d MB", megabytes)
		choices = append(choices, memoryChoice{megabytes: megabytes, name: name, files: files, label: "Keep " + name + " of files"})
	}
	return choices
}

// documentUse is what the files kept take: how many, and about how many bytes.
type documentUse struct{ documents, bytes int }

// documentUseWords says what the files kept take, as the section shows it.
func documentUseWords(use documentUse) string {
	files := english.Count(use.documents, "file")
	kilobytes := use.bytes / kilobytesPerMegabyte
	if kilobytes < kilobytesPerMegabyte {
		return fmt.Sprintf("Kept now: %s, %d KB.", files, kilobytes)
	}
	return fmt.Sprintf("Kept now: %s, %.1f MB.", files, float64(kilobytes)/kilobytesPerMegabyte)
}

// buildMemory is the memory section: what the files read keep, the budgets to choose from, the one
// in use marked, and what is kept now.
func (screen *Screen) buildMemory(context *ui.Context, window Window) {
	context.Text("Files kept in memory", ui.TextStyle{Weight: draw2d.Bold})
	buildNote(context, "Each file you open is kept with its two versions, its changes and its colours, so going back to it is instant. "+
		"When the memory chosen is full, the file opened longest ago is forgotten, and read again if you open it.")
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 16, Wrap: true}, func() {
		for _, choice := range memoryChoices {
			if buildMemoryCard(context, choice, choice.megabytes == window.DocumentMegabytes()) {
				window.ChooseDocumentMegabytes(choice.megabytes)
			}
		}
	})
	documents, bytes := window.DocumentUse()
	words := screen.useWords.Of(documentUse{documents, bytes}, documentUseWords)
	context.Text(words, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft})
}

// buildMemoryCard is a budget's card: its megabytes and how many files they hold, ringed in the
// accent while it is the one in use. It tells whether the card was clicked.
func buildMemoryCard(context *ui.Context, choice memoryChoice, chosen bool) bool {
	layout := ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(memoryCardWidth), Padding: ui.Pad(12), Gap: 6}
	clicked := context.Clickable(choice.name, layout, cardLooks(context.Theme, chosen), func() {
		buildCardName(context, choice.name, chosen)
		context.Box(ui.Layout{Padding: ui.PadXY(4, 0)}, func() {
			context.Text(choice.files, ui.TextStyle{Size: context.Theme.SmallTextSize, Color: context.Theme.InkSoft})
		})
	})
	context.Describe(ui.Description{Role: ui.ButtonRole, Label: choice.label, Selected: chosen})
	return clicked
}
