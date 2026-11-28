package codebase

import (
	"path"
	"sort"
	"strings"
)

// Folder is a folder of a project's tree. A chain of folders that each hold only one folder is
// one Folder named by the whole chain ("src/main/java/com/example"), as Java sources need.
type Folder struct {
	Name    string // its name, or the chain's names joined by "/"
	Path    string // from the project's top, without a trailing "/"
	Folders []*Folder
	Files   []string   // the paths of the files directly inside it
	Changed int        // how many changed files are inside it, at any depth
	Role    FolderRole // what it is to a build tool, in a Java project's tree
	// IsProject tells whether it is the place of a project of its own, inside a plain folder: the
	// project's rows go there, and it is never joined into a chain
	IsProject bool
}

// BuildTree makes a tree of folders from file paths. isChanged tells which files to count as
// changed in each folder.
func BuildTree(paths []string, isChanged func(path string) bool) *Folder {
	root := BuildFolders(paths, isChanged)
	for _, child := range root.Folders {
		compress(child)
	}
	return root
}

// BuildTreeAround makes a plain folder's tree from its file paths and the places of the projects
// inside it: each place is an empty folder that is the project's, and no chain swallows it.
func BuildTreeAround(paths, places []string, isChanged func(path string) bool) *Folder {
	root := BuildFolders(paths, isChanged)
	byPath := map[string]*Folder{}
	indexFolders(root, byPath)
	for _, place := range places {
		folderOf(byPath, place).IsProject = true
	}
	sortFolder(root)
	for _, child := range root.Folders {
		compress(child)
	}
	return root
}

// indexFolders adds a folder and every folder under it to byPath, by their paths.
func indexFolders(folder *Folder, byPath map[string]*Folder) {
	byPath[folder.Path] = folder
	for _, child := range folder.Folders {
		indexFolders(child, byPath)
	}
}

// BuildFolders lays the paths out as nested folders, sorted, one folder per level: the tree before
// any chain of lone folders is joined into one.
func BuildFolders(paths []string, isChanged func(path string) bool) *Folder {
	root := &Folder{}
	byPath := map[string]*Folder{"": root}
	for _, path := range paths {
		parent := folderOf(byPath, parentPath(path))
		parent.Files = append(parent.Files, path)
		if isChanged != nil && isChanged(path) {
			for folderPath := parentPath(path); ; folderPath = parentPath(folderPath) {
				byPath[folderPath].Changed++
				if folderPath == "" {
					break
				}
			}
		}
	}
	sortFolder(root)
	return root
}

// folderOf finds or makes the folder at a path, and the folders above it.
func folderOf(byPath map[string]*Folder, slashed string) *Folder {
	if folder, known := byPath[slashed]; known {
		return folder
	}
	parent := folderOf(byPath, parentPath(slashed))
	folder := &Folder{Name: path.Base(slashed), Path: slashed}
	parent.Folders = append(parent.Folders, folder)
	byPath[slashed] = folder
	return folder
}

func sortFolder(folder *Folder) {
	sort.Slice(folder.Folders, func(first, second int) bool {
		return strings.ToLower(folder.Folders[first].Name) < strings.ToLower(folder.Folders[second].Name)
	})
	sort.Slice(folder.Files, func(first, second int) bool {
		return strings.ToLower(path.Base(folder.Files[first])) < strings.ToLower(path.Base(folder.Files[second]))
	})
	for _, child := range folder.Folders {
		sortFolder(child)
	}
}

// compress joins a folder with its only child folder while it holds no files of its own, unless
// either is the place of a project.
func compress(folder *Folder) {
	for !folder.IsProject && len(folder.Files) == 0 && len(folder.Folders) == 1 && !folder.Folders[0].IsProject {
		only := folder.Folders[0]
		folder.Name += "/" + only.Name
		folder.Path = only.Path
		folder.Folders = only.Folders
		folder.Files = only.Files
	}
	for _, child := range folder.Folders {
		compress(child)
	}
}

func parentPath(path string) string {
	slash := strings.LastIndexByte(path, '/')
	if slash < 0 {
		return ""
	}
	return path[:slash]
}

// BaseName is the last part of a path.
func BaseName(slashed string) string { return path.Base(slashed) }

// FolderRole is what a folder is to a build tool, as IntelliJ marks it in its project tree.
type FolderRole int

const (
	PlainFolder      FolderRole = iota
	ModuleFolder                // holds a build file: a Gradle or Maven module
	SourceRoot                  // src/main/java, src/main/kotlin…: its folders are packages
	TestRoot                    // the same for tests
	ResourceRoot                // src/main/resources, an Android res folder
	TestResourceRoot            // src/test/resources
	PackageFolder               // a package inside a source root
)
