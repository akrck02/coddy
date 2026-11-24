package pane

import (
	"strconv"
	"testing"

	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
)

func TestARevealAskedBeforeTheListIsPlacedHappensOncePlaced(test *testing.T) {
	list := Scroller{RowHeight: 10, Count: 100}
	list.Reveal(50)
	if list.Offset != 0 {
		test.Fatalf("an unplaced list scrolled to %v", list.Offset)
	}
	list.Place(draw2d.Rectangle{Width: 200, Height: 100})
	first, last := list.Visible()
	if 50 < first || 50 >= last {
		test.Errorf("row 50 is not in sight once placed: rows %d to %d are", first, last)
	}
}

func TestTheScrollStaysInsideTheContent(test *testing.T) {
	list := Scroller{RowHeight: 10, Count: 5}
	list.Place(draw2d.Rectangle{Width: 200, Height: 100})
	list.Offset = 400
	list.Clamp()
	if list.Offset != 0 {
		test.Errorf("a list shorter than its view scrolled to %v", list.Offset)
	}
}

func TestKeysWithNoInputAnswerNo(test *testing.T) {
	keys := KeysOf(nil)
	if keys.Pressed(input.KeyEnter) || keys.Clicked() || keys.TypedText() != "" {
		test.Error("keys with no input yet answered yes")
	}
}

// A margin keeps room above the first row and below the last, so they do not touch the edges:
// the rows start lower, the content is taller by both margins, and a point in the margin is on
// no row.
func TestAMarginKeepsRoomAboveTheFirstRowAndBelowTheLast(test *testing.T) {
	list := Scroller{RowHeight: 20, Count: 10, Margin: 8}
	list.Place(draw2d.Rectangle{X: 0, Y: 100, Width: 300, Height: 100})
	if top := list.RowTop(0); top != 108 {
		test.Errorf("the first row starts at %v, want 108, a margin below the top", top)
	}
	if height := list.ContentHeight(); height != 216 {
		test.Errorf("the content is %v tall, want 216: ten rows and two margins", height)
	}
	if row := list.RowAt(10, 104); row != -1 {
		test.Errorf("a point in the top margin is on row %d", row)
	}
	if row := list.RowAt(10, 109); row != 0 {
		test.Errorf("a point just below the margin is on row %d, want 0", row)
	}
	list.Offset = list.ContentHeight() - list.Area.Height
	if bottom := list.RowTop(9) + list.RowHeight; bottom != list.Area.Bottom()-8 {
		test.Errorf("scrolled to the end, the last row ends at %v, want a margin above %v", bottom, list.Area.Bottom())
	}
	if first, last := list.Visible(); first != 5 || last != 10 {
		test.Errorf("scrolled to the end, rows %d to %d are in sight, want 5 to 10", first, last)
	}
}

func TestWordsAreWrittenAgainOnlyWhenTheirValueChanges(test *testing.T) {
	var words Words[int]
	writes := 0
	count := func(value int) string { writes++; return strconv.Itoa(value) + " files" }
	for range 3 {
		if got := words.Of(2, count); got != "2 files" {
			test.Fatalf("the words for 2 are %q", got)
		}
	}
	if got := words.Of(3, count); got != "3 files" || writes != 2 {
		test.Errorf("after 2 three times and 3 once: %q, written %d times, want 2", got, writes)
	}
}
