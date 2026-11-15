package window

import (
	"strings"
	"testing"
)

// byLetters measures a text as 10 pixels a character.
func byLetters(text string) float32 { return float32(len([]rune(text))) * 10 }

func TestALongSignatureBreaksAfterItsCommas(test *testing.T) {
	signature := "class Lamp(keeper: Keeper, harbour: Harbour, quay: Quay)"
	wrapped := wrapLines([]string{signature}, 300, byLetters)
	if len(wrapped) < 2 || !strings.HasSuffix(wrapped[0], ",") {
		test.Fatalf("it broke into %q", wrapped)
	}
	for _, line := range wrapped {
		if byLetters(line) > 300 {
			test.Errorf("%q is wider than the card", line)
		}
	}
	if joined := strings.Join(strings.Fields(strings.Join(wrapped, " ")), " "); joined != signature {
		test.Errorf("wrapped, it reads %q", joined)
	}
}

func TestAWordWiderThanTheCardIsCut(test *testing.T) {
	wrapped := wrapLines([]string{strings.Repeat("Lamp", 20)}, 100, byLetters)
	for _, line := range wrapped {
		if byLetters(line) > 100 || line == "" {
			test.Errorf("a line is %q", line)
		}
	}
}

func TestLinesThatFitAreLeftAsTheyAre(test *testing.T) {
	if wrapped := wrapLines([]string{"Lamp", ""}, 300, byLetters); len(wrapped) != 2 || wrapped[0] != "Lamp" {
		test.Errorf("it gave %q", wrapped)
	}
}

func TestACardTooNarrowForAnyCharacterStillEnds(test *testing.T) {
	if wrapped := wrapLines([]string{"Keeper, Lamp"}, 5, byLetters); len(wrapped) == 0 || len(wrapped) > 40 {
		test.Errorf("it gave %d lines", len(wrapped))
	}
}
