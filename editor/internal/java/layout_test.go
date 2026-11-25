package java

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/workspace"
)

func TestAGradleOrMavenProjectIsLaidOutAsIntelliJShowsIt(test *testing.T) {
	for _, build := range []string{"build.gradle", "pom.xml"} {
		paths := []string{build, "core/" + build, "core/src/main/java/com/example/lamp/Light.java"}
		layout := workspace.LayoutOf([]workspace.Layout{Layout}, paths)
		if layout != Layout {
			test.Fatalf("a project with %s took %T, want the Java layout", build, layout)
		}
		if core := layout.Tree(paths, nil).Folders[0]; core.Role != workspace.ModuleFolder {
			test.Errorf("in a project with %s, core has role %d, want a module", build, core.Role)
		}
	}
}

func TestAProjectWithoutABuildFileIsNotLaidOutAsJava(test *testing.T) {
	if Layout.Fits([]string{"lamp/light.go", "README.md"}) {
		test.Error("a Go project fits the Java layout")
	}
}
