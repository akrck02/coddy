package navigation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/code-review/internal/lsp"
	"github.com/akrck02/code-review/internal/workspace"
)

func TestADeclarationInAJarOpensItsSourceInAProjectOpen(test *testing.T) {
	folder := test.TempDir()
	source := "package org.lighthouse.lamp;\n\nimport java.util.List;\n\n/** A lamp. */\npublic interface Lamp {\n    int beam();\n}\n"
	path := filepath.Join(folder, "src", "main", "java", "org", "lighthouse", "lamp", "Lamp.java")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(source), 0o644)
	projects := []*workspace.Project{{Name: "lamp", Path: folder, Loaded: true,
		Files: []string{"src/main/java/org/lighthouse/lamp/Lamp.java"}}}
	jar := lsp.Location{URI: "jdt://contents/lamp.jar/org.lighthouse.lamp/Lamp.java?=x", Range: lsp.Range{Start: lsp.Position{Line: 23}}}
	cases := []struct {
		name         string
		line, column int
	}{
		{"Lamp", 5, 17}, // the type, wherever the jar's line is
		{"beam", 6, 8},  // a member: the first line past the imports that holds it
	}
	for _, each := range cases {
		location, mapped := FromJar(projects, jar, each.name)
		start := location.Range.Start
		if !mapped || lsp.PathOf(location.URI) != path || start.Line != each.line || start.Character != each.column {
			test.Errorf("%s in the jar opens %s at %d:%d (mapped %t), want %d:%d", each.name, lsp.PathOf(location.URI),
				start.Line, start.Character, mapped, each.line, each.column)
		}
	}
	if _, mapped := FromJar(projects, lsp.Location{URI: "jdt://contents/other.jar/org.sea/Wave.java"}, "Wave"); mapped {
		test.Error("a class no project open holds was mapped")
	}
}

func TestAKotlinDeclarationInAJarOpensItsSource(test *testing.T) {
	folder := test.TempDir()
	files := map[string]string{
		"src/main/kotlin/org/lighthouse/lamp/Lamps.kt": "package org.lighthouse.lamp\n\nimport kotlin.math.max\n\ndata class Wick(val length: Int)\n\nobject Lamp {\n    fun beam() = 2\n}\n",
		"src/main/kotlin/org/lighthouse/lamp/Light.kt": "package org.lighthouse.lamp\n\nfun Wick.glow(): Int = length\n",
	}
	project := &workspace.Project{Name: "lamp", Path: folder, Loaded: true}
	for file, text := range files {
		full := filepath.Join(folder, filepath.FromSlash(file))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(text), 0o644)
		project.Files = append(project.Files, file)
	}
	projects := []*workspace.Project{project}
	cases := []struct {
		class, name, file string
		line, column      int
	}{
		{"Lamp", "Lamp", "Lamps.kt", 6, 7},    // an object, in a file named otherwise
		{"Lamp", "beam", "Lamps.kt", 7, 8},    // a function of it
		{"LightKt", "glow", "Light.kt", 2, 9}, // an extension, in the class Kotlin makes of Light.kt
	}
	for _, each := range cases {
		jar := lsp.Location{URI: "jar:///cache/lamp.jar!/org/lighthouse/lamp/" + each.class + ".class"}
		location, mapped := FromJar(projects, jar, each.name)
		start := location.Range.Start
		if !mapped || filepath.Base(lsp.PathOf(location.URI)) != each.file || start.Line != each.line || start.Character != each.column {
			test.Errorf("%s in the jar opens %s at %d:%d (mapped %t), want %s at %d:%d", each.name, lsp.PathOf(location.URI),
				start.Line, start.Character, mapped, each.file, each.line, each.column)
		}
	}
}
