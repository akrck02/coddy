package workspace

// Layout is how a kind of project lays its tree out: as its folders are on disk, or as an IDE of
// its language shows it (modules, source roots, packages). A new kind is a Layout in the package
// that knows it, and a line in the list the sidebar chooses from.
type Layout interface {
	// Fits tells whether a project's files are laid out this way.
	Fits(paths []string) bool
	// Tree makes the project's tree of the paths listed.
	Tree(paths []string, isChanged func(path string) bool) *Folder
}

// PlainLayout lays a project out as its folders are on disk; it fits every project.
var PlainLayout Layout = plainLayout{}

type plainLayout struct{}

func (plainLayout) Fits([]string) bool { return true }

func (plainLayout) Tree(paths []string, isChanged func(path string) bool) *Folder {
	return BuildTree(paths, isChanged)
}

// LayoutOf is the first of the layouts that fits a project's files, or PlainLayout when none does.
func LayoutOf(layouts []Layout, paths []string) Layout {
	for _, layout := range layouts {
		if layout.Fits(paths) {
			return layout
		}
	}
	return PlainLayout
}
