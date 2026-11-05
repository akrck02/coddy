package review

import (
	"testing"

	"github.com/akrck02/code-review/internal/workspace"
)

func TestTheFinderPrefersTheFileName(test *testing.T) {
	candidates := []Candidate{
		{Label: "api/src/order/service/Helper.kt"},
		{Label: "api/src/OrderService.kt"},
		{Label: "api/src/Other.kt", Changed: true},
	}
	matches := FuzzyFind(candidates, "ordsvc", 10)
	if len(matches) != 2 || matches[0].Candidate.Label != "api/src/OrderService.kt" {
		test.Fatalf("matches: %+v", matches)
	}
	if empty := FuzzyFind(candidates, "", 10); len(empty) != 1 || !empty[0].Candidate.Changed {
		test.Errorf("an empty query lists %+v, want the changed file", empty)
	}
}

func TestAJavaTreeMarksRootsAndJoinsPackages(test *testing.T) {
	tree := BuildJavaTree([]string{
		"build.gradle",
		"core/build.gradle",
		"core/src/main/java/app/coast/lamp/Lamp.java",
		"core/src/main/java/app/coast/lamp/beam/Beam.java",
		"core/src/main/resources/lamp.properties",
		"core/src/test/java/app/coast/lamp/LampTest.java",
	}, nil)
	core := tree.Folders[0]
	if core.Name != "core" || core.Role != workspace.ModuleFolder {
		test.Fatalf("the module is %q with role %d", core.Name, core.Role)
	}
	main := core.Folders[0].Folders[0] // core/src/main
	java, resources := main.Folders[0], main.Folders[1]
	if java.Role != workspace.SourceRoot || resources.Role != workspace.ResourceRoot {
		test.Errorf("src/main's roots are %d and %d", java.Role, resources.Role)
	}
	packages := java.Folders[0]
	if packages.Name != "app.coast.lamp" || packages.Role != workspace.PackageFolder || packages.Path != "core/src/main/java/app/coast/lamp" {
		test.Errorf("the packages are %q (%d) at %s", packages.Name, packages.Role, packages.Path)
	}
	testRoot := core.Folders[0].Folders[1].Folders[0]
	if testRoot.Role != workspace.TestRoot {
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
