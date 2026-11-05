package theme

import "testing"

func TestAFileIsKnownByItsNameBeforeItsExtension(test *testing.T) {
	look := NewDesigns().Dark
	cases := map[string]string{
		"lighthouse/Lamp.java":   "coffee",
		"build.gradle.kts":       "file-cog",
		"docs/README.md":         "book-open",
		"Dockerfile.release":     "container",
		"go.sum":                 "file-lock",
		"notes/without-a-kind.q": "file",
	}
	for name, want := range cases {
		if icon, _ := FileIcon(name, look); icon != want {
			test.Errorf("FileIcon(%q) = %q, want %q", name, icon, want)
		}
	}
}

func TestTheLightLookDeepensItsInks(test *testing.T) {
	designs := NewDesigns()
	light := designs.Light.Theme.Ink
	if light[0] >= 0x24/255.0 {
		test.Errorf("the light look's ink %v is not deeper than Zed's", light)
	}
}
