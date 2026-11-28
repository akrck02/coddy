// Package java knows how Java and Kotlin projects are laid out, as IntelliJ shows them: modules,
// source and resource roots, packages joined with dots, and what each source file declares.
package java

import (
	"regexp"
	"strings"

	"github.com/akrck02/coddy/editor/internal/builds"
	"github.com/akrck02/coddy/editor/internal/codebase"
	"github.com/akrck02/coddy/editor/internal/git"
	"github.com/akrck02/coddy/editor/internal/language"
)

// sourceRoot matches the folders Gradle and Maven take sources and resources from:
// src/<source set>/<language or resources>.
var sourceRoot = regexp.MustCompile(`(^|/)src/([A-Za-z0-9_-]+)/(java|kotlin|groovy|scala|resources|res|assets)$`)

// IsProject tells whether a project's files are laid out as Gradle or Maven lay them out.
func IsProject(paths []string) bool {
	for _, path := range paths {
		if builds.IsBuildFile(codebase.BaseName(path)) || strings.Contains(path, "src/main/java/") || strings.Contains(path, "src/main/kotlin/") {
			return true
		}
	}
	return false
}

// BuildTree makes a project's tree as IntelliJ shows a Java or Kotlin project: modules and
// source roots marked, folders outside the roots one by one, and inside a source root the
// packages that hold only one package joined with dots ("com.example.lighthouse.lamp").
func BuildTree(paths []string, isChanged func(path string) bool) *codebase.Folder {
	root := codebase.BuildFolders(paths, isChanged)
	markRoles(root, codebase.PlainFolder)
	return root
}

// markRoles marks each folder's role from where it is, and joins the packages of source roots.
func markRoles(folder *codebase.Folder, inside codebase.FolderRole) {
	for _, path := range folder.Files {
		if builds.IsBuildFile(codebase.BaseName(path)) && folder.Path != "" {
			folder.Role = codebase.ModuleFolder
		}
	}
	for _, child := range folder.Folders {
		switch {
		case inside == codebase.SourceRoot || inside == codebase.TestRoot:
			child.Role = codebase.PackageFolder
			compactPackages(child)
			markRoles(child, inside)
			continue
		case inside == codebase.PlainFolder:
			if match := sourceRoot.FindStringSubmatch(child.Path); match != nil {
				test := strings.Contains(strings.ToLower(match[2]), "test")
				resources := match[3] == "resources" || match[3] == "res" || match[3] == "assets"
				switch {
				case resources && test:
					child.Role = codebase.TestResourceRoot
				case resources:
					child.Role = codebase.ResourceRoot
				case test:
					child.Role = codebase.TestRoot
				default:
					child.Role = codebase.SourceRoot
				}
				markRoles(child, child.Role)
				continue
			}
		}
		markRoles(child, inside)
	}
}

// compactPackages joins a package with its only package while it holds no files, with a dot.
func compactPackages(folder *codebase.Folder) {
	for len(folder.Files) == 0 && len(folder.Folders) == 1 {
		only := folder.Folders[0]
		folder.Name += "." + only.Name
		folder.Path = only.Path
		folder.Folders = only.Folders
		folder.Files = only.Files
	}
}

// ClassKind is what a Java or Kotlin file declares under its own name.
type ClassKind int

const (
	UnknownKind ClassKind = iota
	ClassDeclaration
	InterfaceDeclaration
	EnumDeclaration
	AnnotationDeclaration
	RecordDeclaration
	ObjectDeclaration // a Kotlin object
	NoClass           // read, and declares nothing under its name: a file of functions
)

var declaration = regexp.MustCompile(`(?m)^[ \t]*(?:(?:public|private|protected|internal|abstract|final|sealed|open|static|data|inner|value|fun|strictfp|non-sealed)\s+)*(@interface|annotation\s+class|enum\s+class|enum|interface|record|class|object)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// KindOf reads what a source file declares under its own name (Lamp for Lamp.java).
func KindOf(path string, content []byte) ClassKind {
	name := codebase.BaseName(path)
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		name = name[:dot]
	}
	for _, match := range declaration.FindAllSubmatch(content, -1) {
		if string(match[2]) != name {
			continue
		}
		keyword := strings.Join(strings.Fields(string(match[1])), " ")
		switch keyword {
		case "interface", "fun interface":
			return InterfaceDeclaration
		case "enum", "enum class":
			return EnumDeclaration
		case "@interface", "annotation class":
			return AnnotationDeclaration
		case "record":
			return RecordDeclaration
		case "object":
			return ObjectDeclaration
		}
		return ClassDeclaration
	}
	return NoClass
}

// IsClassFile tells whether a file is a source of one class, whose kind can be read.
func IsClassFile(path string) bool { return language.IsClassFile(path) }

// headSize is how much of a source file is read to find what it declares: the package, the
// imports and the declaration fit in it nearly always.
const headSize = 24 * 1024

// ReadKind reads what a file of a repository's working tree declares; NoClass when it cannot be
// read.
func ReadKind(repository, path string) ClassKind {
	content, err := git.ReadWorkingFile(repository, path)
	if err != nil {
		return NoClass // a file gone or unreadable shows as a plain file
	}
	return KindOf(path, content[:min(len(content), headSize)])
}

// Letter is the letter IntelliJ marks a kind of class with; "" for a kind it does not mark.
func (kind ClassKind) Letter() string {
	return letters[kind]
}

var letters = map[ClassKind]string{
	ClassDeclaration:      "C",
	InterfaceDeclaration:  "I",
	EnumDeclaration:       "E",
	AnnotationDeclaration: "@",
	RecordDeclaration:     "R",
	ObjectDeclaration:     "O",
}
