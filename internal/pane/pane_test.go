package pane

import (
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
