package window

import "strings"

// wrapLines breaks lines too wide for a width into lines that fit, between words or after a comma
// or an opening bracket, as a signature breaks best; a word wider than the width alone is cut.
// measure gives how wide a text is drawn.
func wrapLines(lines []string, width float32, measure func(string) float32) []string {
	var wrapped []string
	for _, line := range lines {
		for measure(line) > width && strings.TrimSpace(line) != "" {
			cut := breakBefore(line, width, measure)
			wrapped = append(wrapped, strings.TrimRight(line[:cut], " "))
			line = "    " + strings.TrimLeft(line[cut:], " ")
		}
		if strings.TrimSpace(line) != "" || measure(line) <= width {
			wrapped = append(wrapped, line) // a blank line stays; what is left of a broken one, if any
		}
	}
	return wrapped
}

// breakBefore is where a line too wide is broken: after the last space, comma or opening bracket
// that fits, or, with none, after the last character that fits; always after one, past its indent.
func breakBefore(line string, width float32, measure func(string) float32) int {
	fits, breakable := 0, 0
	for index, character := range line {
		end := index + len(string(character))
		if measure(line[:end]) > width {
			break
		}
		fits = end
		if strings.ContainsRune(" ,(<", character) && strings.TrimSpace(line[:end]) != "" {
			breakable = end
		}
	}
	indent := len(line) - len(strings.TrimLeft(line, " "))
	switch {
	case breakable > indent:
		return breakable
	case fits > indent:
		return fits
	}
	_, first := firstRune(line[indent:]) // not even a character fits: one goes, so the line ends
	return indent + first
}

// firstRune is a text's first character and its length in bytes.
func firstRune(text string) (rune, int) {
	for _, character := range text {
		return character, len(string(character))
	}
	return 0, 0
}
