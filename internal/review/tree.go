package review

import (
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
}

// BuildTree makes a tree of folders from file paths. isChanged tells which files to count as
// changed in each folder.
func BuildTree(paths []string, isChanged func(path string) bool) *Folder {
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
	for _, child := range root.Folders {
		compress(child)
	}
	return root
}

// folderOf finds or makes the folder at a path, and the folders above it.
func folderOf(byPath map[string]*Folder, path string) *Folder {
	if folder, known := byPath[path]; known {
		return folder
	}
	parent := folderOf(byPath, parentPath(path))
	folder := &Folder{Name: baseName(path), Path: path}
	parent.Folders = append(parent.Folders, folder)
	byPath[path] = folder
	return folder
}

func sortFolder(folder *Folder) {
	sort.Slice(folder.Folders, func(first, second int) bool {
		return strings.ToLower(folder.Folders[first].Name) < strings.ToLower(folder.Folders[second].Name)
	})
	sort.Slice(folder.Files, func(first, second int) bool {
		return strings.ToLower(baseName(folder.Files[first])) < strings.ToLower(baseName(folder.Files[second]))
	})
	for _, child := range folder.Folders {
		sortFolder(child)
	}
}

// compress joins a folder with its only child folder while it holds no files of its own.
func compress(folder *Folder) {
	for len(folder.Files) == 0 && len(folder.Folders) == 1 {
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

func baseName(path string) string {
	return path[strings.LastIndexByte(path, '/')+1:]
}

// BaseName is the last part of a path.
func BaseName(path string) string { return baseName(path) }
