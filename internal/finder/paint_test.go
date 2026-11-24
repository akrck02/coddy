package finder

import (
	"strings"
	"testing"
)

func TestAHitsLineIsShownWithoutItsIndentationItsMatchStillMarked(test *testing.T) {
	cases := []struct {
		name, text, match string
	}{
		{"an indented line", "        return queue.Run()", "queue"},
		{"a long line cut before its match", "func New(settings Settings, host Host, queue *work.Queue, wake func()) *Fleet {", "Queue"},
		{"a long line cut inside a letter", strings.Repeat("é", 30) + " the match", "match"},
	}
	for _, each := range cases {
		column := strings.Index(each.text, each.match)
		start := excerptStart(each.text, column)
		shown := each.text[start:]
		if strings.HasPrefix(shown, " ") {
			test.Errorf("%s: shown as %q, starting with a space", each.name, shown)
		}
		if !strings.HasPrefix(shown[column-start:], each.match) {
			test.Errorf("%s: the match is marked at %q, not at %q", each.name, shown[column-start:], each.match)
		}
	}
}
