package window

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
	application := newReader(Settings{Synchronous: true})
	application.projects = []*workspace.Project{{Name: "lamp", Path: folder, Loaded: true,
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
		location, mapped := application.fromJar(jar, each.name)
		start := location.Range.Start
		if !mapped || lsp.PathOf(location.URI) != path || start.Line != each.line || start.Character != each.column {
			test.Errorf("%s in the jar opens %s at %d:%d (mapped %t), want %d:%d", each.name, lsp.PathOf(location.URI),
				start.Line, start.Character, mapped, each.line, each.column)
		}
	}
	if _, mapped := application.fromJar(lsp.Location{URI: "jdt://contents/other.jar/org.sea/Wave.java"}, "Wave"); mapped {
		test.Error("a class no project open holds was mapped")
	}
}
