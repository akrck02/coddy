// Package builds reads what a project's builds say it publishes and depends on, as text: fast,
// writing nothing and running nothing, and enough for the plain declarations a build mostly has.
// What it cannot read, a coordinate made in code, it says it did not understand. Each build
// system is a Reader; adding one is a file of its own and a line in Readers.
package builds

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Build is what a project's build files say: the coordinates it publishes ("group:artifact"),
// those it depends on, and the declarations it could not read.
type Build struct {
	Publishes     []string
	Depends       []string
	NotUnderstood []string
}

// Reader reads the build files of one build system (Gradle, Maven).
type Reader interface {
	// Files are the names of its build files: "pom.xml".
	Files() []string
	// Read adds to a build what its files say, in the order the project lists them. folder names
	// the project when its build does not.
	Read(build *Build, folder string, files []File)
}

// File is a build file of a project: its path from the project's folder, with slashes, and its
// content.
type File struct {
	Path    string
	Content []byte
}

// Readers are the build systems read, each by its own Reader.
var Readers = []Reader{Gradle, Maven}

// notBuildFolders are folders whose files a build or a tool wrote: never a build to read.
var notBuildFolders = []string{"build", "target", "node_modules"}

// Read reads a project's build files among its files (paths from its folder, with slashes), at
// any depth but in a build's output, each by the reader of its build system.
func Read(folder string, files []string) Build {
	var build Build
	for _, reader := range Readers {
		reader.Read(&build, folder, readFiles(folder, files, reader.Files()))
	}
	build.tidy()
	return build
}

// readFiles reads the files of a project that are build files of the given names; one that cannot
// be read is left out, as if the build had no such file.
func readFiles(folder string, files, names []string) []File {
	var read []File
	for _, path := range files {
		if inBuildOutput(path) || !slices.Contains(names, filepath.Base(path)) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(path)))
		if err == nil {
			read = append(read, File{Path: path, Content: content})
		}
	}
	return read
}

// inBuildOutput tells whether a file of a project is in a folder a build or a tool wrote, or in a
// hidden one.
func inBuildOutput(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if slices.Contains(notBuildFolders, part) || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// IsBuildFile tells whether a file name is a build file of a build system read: "pom.xml".
func IsBuildFile(name string) bool {
	for _, reader := range Readers {
		if slices.Contains(reader.Files(), name) {
			return true
		}
	}
	return false
}

// tidy sorts each list and drops what is twice in it.
func (build *Build) tidy() {
	for _, list := range []*[]string{&build.Publishes, &build.Depends, &build.NotUnderstood} {
		sort.Strings(*list)
		*list = compact(*list)
	}
}

// compact drops the repeats of a sorted list.
func compact(list []string) []string {
	var kept []string
	for index, each := range list {
		if index == 0 || each != list[index-1] {
			kept = append(kept, each)
		}
	}
	return kept
}

// Links are, for each project (by its index among those given), the projects it depends on: those
// that publish a coordinate it depends on.
func Links(projects []Build) [][]int {
	publisher := map[string]int{}
	for index, build := range projects {
		for _, coordinate := range build.Publishes {
			publisher[coordinate] = index
		}
	}
	links := make([][]int, len(projects))
	for index, build := range projects {
		seen := map[int]bool{}
		for _, coordinate := range build.Depends {
			if other, found := publisher[coordinate]; found && other != index && !seen[other] {
				seen[other] = true
				links[index] = append(links[index], other)
			}
		}
		sort.Ints(links[index])
	}
	return links
}
