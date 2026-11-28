package settings

import (
	"fmt"
	"strconv"

	"github.com/akrck02/coddy/editor/internal/format"
	"github.com/akrck02/coddy/editor/internal/workspace"
	"github.com/akrck02/go-renderer/draw2d"
	ui "github.com/akrck02/go-ui"
)

const (
	// memorySliderWidth is the widest the budget's slider grows, so a step stays a short move
	memorySliderWidth = 480
	// fileKilobytesLeast and fileKilobytesMost are about how much a usual file of code takes once
	// read with its two versions, its diff and its colours: what the files a budget holds are
	// counted by
	fileKilobytesLeast = 100
	fileKilobytesMost  = 350
	// kilobytesPerMegabyte is how many kilobytes a megabyte holds
	kilobytesPerMegabyte = 1024
	// fileCountStep is what the counts of files are rounded to: they are a guide, so a precise
	// number would promise more than it knows
	fileCountStep = 100
	// memorySliderLabel is what a screen reader, and a test, call the budget's slider
	memorySliderLabel = "Memory for the files kept"
)

// budgetName is a budget's name, in megabytes: "64 MB".
func budgetName(megabytes int) string { return fmt.Sprintf("%d MB", megabytes) }

// budgetFiles is about how many files of a usual size a budget holds: "about 200 to 600 files".
func budgetFiles(megabytes int) string {
	kilobytes := megabytes * kilobytesPerMegabyte
	return "about " + roughly(kilobytes/fileKilobytesMost) + " to " + roughly(kilobytes/fileKilobytesLeast) + " files"
}

// The names of the slider's two ends, written once.
var (
	leastBudgetName = budgetName(workspace.DocumentMegabytesLeast)
	mostBudgetName  = budgetName(workspace.DocumentMegabytesMost)
)

// roughly is a count rounded to the nearest hundred, never below one hundred, with a comma
// between its thousands: 1310 is "1,300".
func roughly(count int) string {
	rounded := max(fileCountStep, (count+fileCountStep/2)/fileCountStep*fileCountStep)
	const thousand = 1000
	thousands, rest := rounded/thousand, rounded%thousand
	if thousands == 0 {
		return strconv.Itoa(rest)
	}
	return fmt.Sprintf("%d,%03d", thousands, rest)
}

// documentUse is what the files kept take: how many, and about how many bytes.
type documentUse struct{ documents, bytes int }

// documentUseWords says what the files kept take, as the section shows it.
func documentUseWords(use documentUse) string {
	files := format.Count(use.documents, "file")
	kilobytes := use.bytes / kilobytesPerMegabyte
	if kilobytes < kilobytesPerMegabyte {
		return fmt.Sprintf("Kept now: %s, %d KB.", files, kilobytes)
	}
	return fmt.Sprintf("Kept now: %s, %.1f MB.", files, float64(kilobytes)/kilobytesPerMegabyte)
}

// buildMemory is the memory section: what the files read keep, a slider for the budget with the
// files it holds, and what is kept now.
func (screen *Screen) buildMemory(context *ui.Context, window Window) {
	palette := context.Theme
	soft := ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft}
	context.Text("Files kept in memory", ui.TextStyle{Weight: draw2d.Bold})
	buildNote(context, "Each file you open is kept with its two versions, its changes and its colours, so going back to it is instant. "+
		"When the memory chosen is full, the file opened longest ago is forgotten, and read again if you open it.")
	chosen := screen.slideBudget(window)
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(memorySliderWidth), Gap: 8}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 10, AlignY: ui.Center}, func() {
			context.Text(screen.budgetName.Of(chosen, budgetName), ui.TextStyle{Size: palette.HeadingSize, Weight: draw2d.Bold})
			context.Text(screen.budgetFiles.Of(chosen, budgetFiles), ui.TextStyle{Color: palette.InkSoft})
		})
		value := float32(chosen)
		if context.SliderStep(memorySliderLabel, &value, workspace.DocumentMegabytesLeast, workspace.DocumentMegabytesMost, workspace.DocumentMegabytesStep) {
			screen.slidTo = int(value)
		}
		context.Box(ui.Layout{Width: ui.Grow()}, func() {
			context.Text(leastBudgetName, soft)
			context.Box(ui.Layout{Width: ui.Grow()}, nil)
			context.Text(mostBudgetName, soft)
		})
	})
	documents, bytes := window.DocumentUse()
	context.Text(screen.useWords.Of(documentUse{documents, bytes}, documentUseWords), soft)
}

// slideBudget is the budget the memory section shows: the one the slider is being moved to, or the
// one in use. The budget moved to is only chosen once the pointer lets the slider go, so dragging
// down and up again on the way forgets no file; a key or a screen reader's step lets go at once.
func (screen *Screen) slideBudget(window Window) int {
	if screen.slidTo == 0 {
		return window.DocumentMegabytes()
	}
	if window.Keys().Holding() {
		return screen.slidTo
	}
	window.ChooseDocumentMegabytes(screen.slidTo)
	screen.slidTo = 0
	return window.DocumentMegabytes()
}
