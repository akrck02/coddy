package settings

import (
	"strings"
	"testing"
)

func TestTheGridTakesAsManyColumnsAsCardsFit(test *testing.T) {
	cases := []struct {
		width   float32
		columns int
	}{
		{width: 200, columns: 1}, // too narrow even for one: one all the same
		{width: cardNarrowest, columns: 1},
		{width: 2*cardNarrowest + pluginsGap - 1, columns: 1},
		{width: 2*cardNarrowest + pluginsGap, columns: 2},
		{width: 1100, columns: 3},
	}
	for _, each := range cases {
		columns := pluginColumns(each.width)
		if columns != each.columns {
			test.Errorf("%v pixels take %d columns, want %d", each.width, columns, each.columns)
		}
		if columns > 1 && cardWidthOf(each.width, columns) < cardNarrowest {
			test.Errorf("%v pixels make cards %v wide, narrower than %d", each.width, cardWidthOf(each.width, columns), cardNarrowest)
		}
	}
}

// letters measures a text as a letter a pixel, for lines whose breaks are easy to follow.
func letters(text string) float32 { return float32(len([]rune(text))) }

func TestADescriptionTakesTwoLinesAtMost(test *testing.T) {
	cases := []struct {
		text, first, second string
	}{
		{"Reads Go.", "Reads Go.", ""},
		{"Reads Go with gopls today", "Reads Go with", "gopls today"},
		{"Reads Go with gopls, then finds every use of a name", "Reads Go with", "gopls, then…"},
		{"Unbreakablewordlongerthanaline and more", "Unbreakablewordlongerthanaline", "and more"},
	}
	for _, each := range cases {
		first, second := splitInTwoLines(each.text, 14, letters)
		if first != each.first || second != each.second {
			test.Errorf("%q breaks into %q and %q, want %q and %q", each.text, first, second, each.first, each.second)
		}
	}
}

// A command Coddy does not run says why: the password it asks for, or the installer missing.
func TestACommandToRunByHandSaysWhyCoddyDoesNotRunIt(test *testing.T) {
	if advice := handInstallAdvice("wick", "sudo dnf install wick"); !strings.Contains(advice, "password") {
		test.Errorf("a command run as the administrator says %q", advice)
	}
	if advice := handInstallAdvice("wick", "no-such-installer install wick"); !strings.Contains(advice, "uses no-such-installer") {
		test.Errorf("a command whose installer is missing says %q", advice)
	}
}
