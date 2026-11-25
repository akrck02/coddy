package session

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/workspace"
)

// makeFolders makes the folders and files of paths under a new temporary folder, and gives it
// back: a path ending in "/" is a folder, any other a file.
func makeFolders(test *testing.T, paths ...string) string {
	top := test.TempDir()
	for _, path := range paths {
		full := filepath.Join(top, filepath.FromSlash(path))
		folder := filepath.Dir(full)
		if strings.HasSuffix(path, "/") {
			folder = full
		}
		if err := os.MkdirAll(folder, 0o755); err != nil {
			test.Fatal(err)
		}
		if folder == full {
			continue
		}
		if err := os.WriteFile(full, []byte("text\n"), 0o644); err != nil {
			test.Fatal(err)
		}
	}
	return top
}

// repositoryNames are the names of the projects that are not plain folders.
func repositoryNames(opened Session) []string {
	var names []string
	for _, project := range opened.Projects {
		if !project.IsPlainFolder() {
			names = append(names, project.Name)
		}
	}
	return names
}

// With git turned off, a folder with a .git is no repository; one a plugin marks still is.
func TestWithGitOffOnlyTheFoldersAPluginMarksAreRepositories(test *testing.T) {
	folder := makeFolders(test, "harbour/.git/", "lighthouse/.hg/")
	opened := New("", true)
	opened.Marked = []Marked{{Marker: ".hg"}}
	opened.GitOff = true
	opened.AddFolders([]string{folder})
	if names := repositoryNames(opened); !slices.Equal(names, []string{"lighthouse"}) {
		test.Errorf("the repositories found are %v, want only lighthouse", names)
	}
	withGit := New("", true)
	withGit.AddFolders([]string{folder})
	if names := repositoryNames(withGit); !slices.Equal(names, []string{"harbour"}) {
		test.Errorf("with git on the repositories found are %v, want harbour", names)
	}
}

// A folder with no repository, in it or inside it, opens as a plain folder alone.
func TestAFolderWithNoRepositoryOpensAsAPlainFolder(test *testing.T) {
	folder := filepath.Join(makeFolders(test, "recipes/soups/"), "recipes")
	opened := New("", true)
	opened.AddFolders([]string{folder})
	if len(opened.Projects) != 1 || opened.Projects[0].Name != "recipes" || !opened.Projects[0].IsPlainFolder() {
		test.Errorf("the projects found are %+v, want recipes as a plain folder", opened.Projects)
	}
}

// A folder that is a repository opens as the repository alone, with no plain folder around it.
func TestARepositoryOpensAlone(test *testing.T) {
	folder := filepath.Join(makeFolders(test, "harbour/.git/", "harbour/main.go"), "harbour")
	opened := New("", true)
	opened.AddFolders([]string{folder})
	if len(opened.Projects) != 1 || opened.Projects[0].IsPlainFolder() || opened.Projects[0].Root != "" {
		test.Errorf("the projects found are %+v, want harbour alone", opened.Projects)
	}
}

// A folder inside a repository opened with it is not opened again as a plain folder.
func TestAFolderInsideARepositoryIsNotOpenedAgain(test *testing.T) {
	repository := filepath.Join(makeFolders(test, "harbour/.git/", "harbour/guides/"), "harbour")
	opened := New("", true)
	opened.AddFolders([]string{repository, filepath.Join(repository, "guides")})
	if len(opened.Projects) != 1 || opened.Projects[0].Name != "harbour" {
		test.Errorf("the projects found are %+v, want only harbour", opened.Projects)
	}
}

// Repositories a few folders down, as an archive unpacked into a folder of its own leaves them,
// are found, each knowing the folder opened as its root; a repository's own folders are not
// searched.
func TestRepositoriesSomeFoldersDownAreFound(test *testing.T) {
	downloads := makeFolders(test, "guide.pdf", "archive/projects/harbour/.git/", "archive/projects/harbour/vendor/lighthouse/.git/")
	opened := New("", true)
	opened.AddFolders([]string{downloads})
	if names := repositoryNames(opened); !slices.Equal(names, []string{"harbour"}) {
		test.Errorf("the repositories found are %v, want only harbour", names)
	}
	for _, project := range opened.Projects {
		if !project.IsPlainFolder() && project.Root != downloads {
			test.Errorf("%s was found in %q, want %q", project.Name, project.Root, downloads)
		}
	}
}

// The folder opened lists its loose files, and a repository's files only under the repository.
func TestALooseFileIsListedOnceAndARepositorysFilesOnlyUnderIt(test *testing.T) {
	downloads := makeFolders(test, "guide.pdf", "archive/notes.txt", "archive/harbour/.git/", "archive/harbour/main.go")
	opened := New("", true)
	opened.AddFolders([]string{downloads})
	var root *workspace.Project
	for _, project := range opened.Projects {
		if project.IsPlainFolder() {
			root = project
		}
	}
	if root == nil || root.Path != downloads {
		test.Fatalf("the projects found are %+v, want the folder opened among them", opened.Projects)
	}
	loaded := workspace.LoadProject(context.Background(), root.Path, opened.LoadOptions(root.Path))
	if want := []string{"archive/notes.txt", "guide.pdf"}; !slices.Equal(loaded.Files, want) {
		test.Errorf("the folder opened lists %v, want %v", loaded.Files, want)
	}
}

// A folder that holds nothing but repositories gives only them, with no plain folder above.
func TestAFolderOfRepositoriesAloneListsOnlyThem(test *testing.T) {
	code := makeFolders(test, "harbour/.git/", "tools/lighthouse/.git/", ".cache/notes.txt")
	opened := New("", true)
	opened.AddFolders([]string{code})
	if names := repositoryNames(opened); len(opened.Projects) != 2 || !slices.Equal(names, []string{"harbour", "lighthouse"}) {
		test.Errorf("the projects found are %v of %d, want harbour and lighthouse alone", names, len(opened.Projects))
	}
}
