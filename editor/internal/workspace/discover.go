package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/git"
)

// repository is a folder found to be a project, the version control it is read through (nil for
// git), and the plain folder it was found in, when that folder is a project too.
type repository struct {
	path    string
	control codebase.VersionControl
	root    string
}

// discover turns the folders a person opened into projects. A folder that is a repository is
// itself. A folder that is not is a plain folder, read with no version control, and the
// repositories inside it, down to repositoryDepth folders, are projects of their own that it
// shows in its tree and does not list again; a folder that holds nothing but repositories gives
// only them; a folder inside a project adds no plain folder. The
// result has no project twice, and none open already.
func (workspace *Workspace) discover(folders []string) []repository {
	seen := map[string]bool{}
	var found []repository
	for _, project := range workspace.Projects {
		found = append(found, repository{path: project.Path, control: project.Control})
	}
	known := len(found)
	for _, folder := range folders {
		absolute, err := filepath.Abs(folder)
		if err != nil {
			continue // a folder with no absolute path cannot be read, so it holds no repository
		}
		before := len(found)
		searched := 0
		inside := workspace.repositoriesUnder(absolute, repositoryDepth, &searched)
		for _, candidate := range inside {
			if !seen[candidate.path] {
				seen[candidate.path] = true
				found = append(found, candidate)
			}
		}
		_, isRepository := workspace.repositoryAt(absolute)
		if isRepository || seen[absolute] || !isFolder(absolute) || insideAny(absolute, found[:before]) {
			continue
		}
		places := placesIn(absolute, inside)
		if len(places) > 0 && !holdsMore(absolute, places) {
			continue // a folder of repositories alone lists them, with no level above that adds nothing
		}
		seen[absolute] = true
		for index := before; index < len(found); index++ {
			found[index].root = absolute
		}
		found = append(found, repository{path: absolute, control: &codebase.FolderControl{LeftOut: places}})
	}
	return found[known:]
}

// placesIn are the paths of repositories inside a folder, from the folder and with slashes.
func placesIn(folder string, repositories []repository) []string {
	places := make([]string, 0, len(repositories))
	for _, inside := range repositories {
		if relative, err := filepath.Rel(folder, inside.path); err == nil {
			places = append(places, filepath.ToSlash(relative))
		}
	}
	return places
}

// holdsMore tells whether a folder holds anything that is not hidden besides the repositories at
// places inside it: a file, or a folder with no repository in it.
func holdsMore(folder string, places []string) bool {
	return holdsMoreAt(folder, "", places)
}

func holdsMoreAt(folder, at string, places []string) bool {
	entries, err := os.ReadDir(filepath.Join(folder, filepath.FromSlash(at)))
	if err != nil {
		return false // a folder that cannot be read shows nothing more
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := strings.TrimPrefix(at+"/"+entry.Name(), "/")
		switch {
		case !entry.IsDir():
			return true
		case slices.Contains(places, path):
		case slices.ContainsFunc(places, func(place string) bool { return strings.HasPrefix(place, path+"/") }):
			if holdsMoreAt(folder, path, places) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// insideAny tells whether a folder is one of the projects, or inside one.
func insideAny(folder string, projects []repository) bool {
	for _, project := range projects {
		relative, err := filepath.Rel(project.path, folder)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

const (
	// repositoryDepth is how many folders down a folder opened is searched for repositories: a
	// download unpacked as folder/folder/repository is still found.
	repositoryDepth = 3
	// searchedFolderLimit is how many folders the search reads at most, so that opening a home
	// folder by mistake does not read the whole disk.
	searchedFolderLimit = 2000
)

// repositoriesUnder finds the repositories in a folder, itself included, down to depth folders
// below it, in the order of their names. A repository's own folders and hidden folders are not
// searched; searched counts the folders read, up to searchedFolderLimit.
func (workspace *Workspace) repositoriesUnder(folder string, depth int, searched *int) []repository {
	if control, isRepository := workspace.repositoryAt(folder); isRepository {
		return []repository{{path: folder, control: control}}
	}
	if depth == 0 || *searched >= searchedFolderLimit {
		return nil
	}
	*searched++
	var found []repository
	for _, inside := range foldersInside(folder) {
		found = append(found, workspace.repositoriesUnder(inside, depth-1, searched)...)
	}
	return found
}

// repositoryAt tells whether a folder is a repository, and which version control reads it: git
// when it holds a .git and git is not off, else the first whose marker it holds.
func (workspace *Workspace) repositoryAt(folder string) (codebase.VersionControl, bool) {
	if !workspace.GitOff && git.IsRepository(folder) {
		return nil, true
	}
	for _, marked := range workspace.Marked {
		if _, err := os.Stat(filepath.Join(folder, marked.Marker)); err == nil {
			return marked.Control, true
		}
	}
	return nil, false
}

// isFolder tells whether a path is a folder that exists.
func isFolder(path string) bool {
	information, err := os.Stat(path)
	return err == nil && information.IsDir()
}

// foldersInside lists the folders directly inside a folder that are not hidden, sorted by name.
func foldersInside(folder string) []string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var inside []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			inside = append(inside, filepath.Join(folder, entry.Name()))
		}
	}
	sort.Strings(inside)
	return inside
}
