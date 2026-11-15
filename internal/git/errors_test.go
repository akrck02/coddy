package git

import (
	"errors"
	"testing"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, notReadOnly := run(".", "commit")
	_, failed := run(test.TempDir(), "rev-parse", "HEAD")
	_, _, unreadableHeader := parseHunkHeader("@@ nonsense")
	_, unreadableStart := hunkStart("-twelve", "-")
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a command off the read-only list", notReadOnly, CommandNotReadOnly},
		{"git failing outside a repository", failed, CommandFailed},
		{"a hunk header with too few fields", unreadableHeader, UnreadableDiff},
		{"a hunk start that is not a number", unreadableStart, UnreadableDiff},
	}
	for _, kind := range cases {
		if !errors.Is(kind.err, kind.code) {
			test.Errorf("%s gave %v, want the code %v", kind.name, kind.err, kind.code)
		}
	}
}
