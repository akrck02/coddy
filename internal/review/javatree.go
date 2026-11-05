package review

import (
	"github.com/akrck02/code-review/internal/workspace"
	"regexp"
	"strings"
)

// buildFiles mark a folder as a module.
var buildFiles = map[string]bool{
	"build.gradle": true, "build.gradle.kts": true, "pom.xml": true, "settings.gradle": true, "settings.gradle.kts": true,
}

// sourceRoot matches the folders Gradle and Maven take sources and resources from:
// src/<source set>/<language or resources>.
var sourceRoot = regexp.MustCompile(`(^|/)src/([A-Za-z0-9_-]+)/(java|kotlin|groovy|scala|resources|res|assets)$`)

// IsJavaProject tells whether a project's files are laid out as Gradle or Maven lay them out.
func IsJavaProject(paths []string) bool {
	for _, path := range paths {
		if buildFiles[workspace.BaseName(path)] || strings.Contains(path, "src/main/java/") || strings.Contains(path, "src/main/kotlin/") {
			return true
		}
	}
	return false
}

// BuildJavaTree makes a project's tree as IntelliJ shows a Java or Kotlin project: modules and
// source roots marked, folders outside the roots one by one, and inside a source root the
// packages that hold only one package joined with dots ("app.coast.platform.dal").
func BuildJavaTree(paths []string, isChanged func(path string) bool) *workspace.Folder {
	root := workspace.BuildFolders(paths, isChanged)
	markRoles(root, workspace.PlainFolder)
	return root
}

// markRoles marks each folder's role from where it is, and joins the packages of source roots.
func markRoles(folder *workspace.Folder, inside workspace.FolderRole) {
	for _, path := range folder.Files {
		if buildFiles[workspace.BaseName(path)] && folder.Path != "" {
			folder.Role = workspace.ModuleFolder
		}
	}
	for _, child := range folder.Folders {
		switch {
		case inside == workspace.SourceRoot || inside == workspace.TestRoot:
			child.Role = workspace.PackageFolder
			compactPackages(child)
			markRoles(child, inside)
			continue
		case inside == workspace.PlainFolder:
			if match := sourceRoot.FindStringSubmatch(child.Path); match != nil {
				test := strings.Contains(strings.ToLower(match[2]), "test")
				resources := match[3] == "resources" || match[3] == "res" || match[3] == "assets"
				switch {
				case resources && test:
					child.Role = workspace.TestResourceRoot
				case resources:
					child.Role = workspace.ResourceRoot
				case test:
					child.Role = workspace.TestRoot
				default:
					child.Role = workspace.SourceRoot
				}
				markRoles(child, child.Role)
				continue
			}
		}
		markRoles(child, inside)
	}
}

// compactPackages joins a package with its only package while it holds no files, with a dot.
func compactPackages(folder *workspace.Folder) {
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
	name := workspace.BaseName(path)
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

// IsClassFile tells whether a file is a Java or Kotlin source whose class can be read.
func IsClassFile(path string) bool {
	return strings.HasSuffix(path, ".java") || strings.HasSuffix(path, ".kt")
}
