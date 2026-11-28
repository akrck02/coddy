package navigation

import (
	"regexp"
	"strings"

	"github.com/akrck02/coddy/editor/internal/ui/viewer"
)

// HoverCard is what a server said of the name the pointer rests on, and where the pointer was.
type HoverCard struct {
	Place   viewer.Place
	Resting bool
	Text    string
	X, Y    float32
}

// markdownLink is a Markdown link, of which the card keeps the words.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// HoverLines are the lines of a hover's Markdown worth showing in a small card: no code fences,
// rules or emphasis marks, links as their words, and no blank line twice.
func HoverLines(markdown string) []string {
	var lines []string
	for _, line := range strings.Split(markdown, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "```") || trimmed == "---" {
			continue // fences and rules: the card shows the words alone
		}
		line = markdownLink.ReplaceAllString(line, "$1")
		line = strings.ReplaceAll(strings.ReplaceAll(line, "**", ""), "`", "")
		if line == "" && (len(lines) == 0 || lines[len(lines)-1] == "") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
