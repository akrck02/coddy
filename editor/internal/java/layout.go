package java

import "github.com/akrck02/coddy/editor/internal/codebase"

// Layout lays a Gradle or Maven project out as IntelliJ shows it: modules, source roots and
// packages (see BuildTree).
var Layout codebase.Layout = layout{}

type layout struct{}

func (layout) Fits(paths []string) bool { return IsProject(paths) }

func (layout) Tree(paths []string, isChanged func(path string) bool) *codebase.Folder {
	return BuildTree(paths, isChanged)
}
