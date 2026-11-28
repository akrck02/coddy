package navigation

import (
	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/ui/finder"
	"github.com/akrck02/coddy/editor/internal/ui/mode"
)

// Listed are the uses a search found so far, as the finder lists them.
func Listed(host Host, search *Search) []finder.Use {
	var uses []finder.Use
	for _, location := range search.found {
		if use, found := FinderUseOf(host, location); found {
			uses = append(uses, use)
		}
	}
	return uses
}

// FinderUseOf is a place a server gave as a use listed in the finder, when it is in a project open.
func FinderUseOf(host Host, location lsp.Location) (finder.Use, bool) {
	path := lsp.PathOf(location.URI)
	reference, found := ReferenceOf(host.Projects(), path)
	if !found {
		return finder.Use{}, false
	}
	start, end := location.Range.Start, location.Range.End
	text := host.LineOf(path, start.Line)
	column := lsp.CharacterColumn(text, start.Character)
	length := 0
	if end.Line == start.Line {
		length = lsp.CharacterColumn(text, end.Character) - column
	}
	byteColumn := len(string([]rune(text)[:min(column, len([]rune(text)))]))
	return finder.Use{Project: reference.Project, Path: reference.Path, Line: start.Line + 1,
		Column: byteColumn, Length: length, Text: text}, true
}

// ModeUses are the uses a search found, as the uses mode keeps them, and the probable ones not
// checked yet.
func ModeUses(host Host, search *Search) []mode.Use {
	var found []mode.Use
	for _, location := range search.found {
		if use, known := ModeUseOf(host, location); known {
			found = append(found, use)
		}
	}
	for key, location := range search.probable {
		if _, confirmed := search.found[key]; confirmed {
			continue
		}
		if use, known := ModeUseOf(host, location); known {
			use.Probable = true
			found = append(found, use)
		}
	}
	return found
}

// ModeUseOf is a place a server gave as a use, as the uses mode keeps it, when it is in a project
// open.
func ModeUseOf(host Host, location lsp.Location) (mode.Use, bool) {
	path := lsp.PathOf(location.URI)
	reference, found := ReferenceOf(host.Projects(), path)
	if !found {
		return mode.Use{}, false
	}
	start, end := location.Range.Start, location.Range.End
	text := host.LineOf(path, start.Line)
	column := lsp.CharacterColumn(text, start.Character)
	last := column
	if end.Line == start.Line {
		last = lsp.CharacterColumn(text, end.Character)
	}
	return mode.Use{File: reference, Line: start.Line + 1, Start: column, End: last}, true
}
