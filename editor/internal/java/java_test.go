package java

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/codebase"
)

func TestAJavaTreeMarksRootsAndJoinsPackages(test *testing.T) {
	tree := BuildTree([]string{
		"build.gradle",
		"core/build.gradle",
		"core/src/main/java/com/example/lighthouse/Lamp.java",
		"core/src/main/java/com/example/lighthouse/beam/Beam.java",
		"core/src/main/resources/lamp.properties",
		"core/src/test/java/com/example/lighthouse/LampTest.java",
	}, nil)
	core := tree.Folders[0]
	if core.Name != "core" || core.Role != codebase.ModuleFolder {
		test.Fatalf("the module is %q with role %d", core.Name, core.Role)
	}
	main := core.Folders[0].Folders[0] // core/src/main
	java, resources := main.Folders[0], main.Folders[1]
	if java.Role != codebase.SourceRoot || resources.Role != codebase.ResourceRoot {
		test.Errorf("src/main's roots are %d and %d", java.Role, resources.Role)
	}
	packages := java.Folders[0]
	if packages.Name != "com.example.lighthouse" || packages.Role != codebase.PackageFolder || packages.Path != "core/src/main/java/com/example/lighthouse" {
		test.Errorf("the packages are %q (%d) at %s", packages.Name, packages.Role, packages.Path)
	}
	testRoot := core.Folders[0].Folders[1].Folders[0]
	if testRoot.Role != codebase.TestRoot {
		test.Errorf("src/test/java has role %d", testRoot.Role)
	}
}

func TestTheKindOfAClassIsReadFromItsDeclaration(test *testing.T) {
	cases := map[string]ClassKind{
		"public interface Lamp extends Light {":               InterfaceDeclaration,
		"@SuppressWarnings(\"x\")\npublic final class Lamp {": ClassDeclaration,
		"enum class Lamp { ON, OFF }":                         EnumDeclaration,
		"public @interface Lamp {}":                           AnnotationDeclaration,
		"public record Lamp(int watts) {}":                    RecordDeclaration,
		"object Lamp {":                                       ObjectDeclaration,
		"data class Beam(val x: Int)\nfun lamp() = 1":         NoClass,
	}
	for source, want := range cases {
		if got := KindOf("src/Lamp.kt", []byte(source)); got != want {
			test.Errorf("%q read as %d, want %d", source, got, want)
		}
	}
}
