package plugins

import (
	"errors"
	"path/filepath"
	"testing"
)

// A plugin without an icon of its own keeps the same colour every time it is read, so people
// learn it, and plugins side by side do not all share one.
func TestTheAutomaticBadgeIsTheSameColourEachTimeAndDiffersBetweenNames(test *testing.T) {
	names := []string{"go", "java", "kotlin", "git", "lamp", "github"}
	colours := map[[4]float32]bool{}
	for _, name := range names {
		first, again := badgeOf(name, "", ""), badgeOf(name, "", "")
		if first.Colour != again.Colour {
			test.Errorf("%s is %v, then %v", name, first.Colour, again.Colour)
		}
		colours[first.Colour] = true
	}
	if len(colours) < 3 {
		test.Errorf("%d names share %d colours, want at least 3", len(names), len(colours))
	}
	if badge := badgeOf("kotlin", "", ""); badge.Initial != "K" || badge.Icon != "" || badge.File != "" {
		test.Errorf("kotlin's automatic badge is %+v, want its initial K", badge)
	}
}

func TestABadgeIsALucideIconOrAFileInsideThePluginsFolder(test *testing.T) {
	folder := filepath.Join("plugins", "lamp")
	cases := []struct {
		icon, lucide, file string
	}{
		{icon: "coffee", lucide: "coffee"},
		{icon: "icon.svg", file: filepath.Join(folder, "icon.svg")},
		{icon: "pictures/Icon.PNG", file: filepath.Join(folder, "pictures", "Icon.PNG")},
		{icon: "../other/icon.svg"},
		{icon: "/etc/icon.svg"},
	}
	for _, each := range cases {
		badge := badgeOf("lamp", each.icon, folder)
		if badge.Icon != each.lucide || badge.File != each.file || badge.Initial != "L" {
			test.Errorf("the icon %q makes %+v, want the icon %q and the file %q", each.icon, badge, each.lucide, each.file)
		}
	}
}

func TestAManifestWhoseIconFileLeavesItsFolderIsRefused(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "escape", `{"name": "escape", "icon": "../../icon.svg", "command": ["escape"], "speaks": "coddy"}`)
	install(test, folder, "drawn", `{"name": "drawn", "author": "Someone", "icon": "icon.svg", "command": ["drawn"], "speaks": "coddy"}`)
	found := Find(folder, nil)
	if len(found) != 2 {
		test.Fatalf("found %d plugins, want 2", len(found))
	}
	drawn, escape := found[0], found[1]
	if drawn.Failed() || drawn.Author != "Someone" || drawn.Badge.File != filepath.Join(folder, "drawn", "icon.svg") {
		test.Errorf("the plugin with its own icon is %q by %q, its badge %+v", drawn.State, drawn.Author, drawn.Badge)
	}
	if !errors.Is(escape.failed, BadManifest) || escape.Badge.File != "" {
		test.Errorf("the plugin whose icon leaves its folder is %q, its badge %+v", escape.State, escape.Badge)
	}
}

func TestOnlyWhatJoinsAtTheStartWaitsForARestart(test *testing.T) {
	cases := []struct {
		name  string
		added Contributions
		waits bool
	}{
		{"a language server", Contributions{LanguageServer: &LanguageServer{}}, true},
		{"a version control", Contributions{VersionControl: &VersionControlContribution{}}, true},
		{"a mode", Contributions{Mode: &ModeContribution{}}, true},
		{"an action", Contributions{Actions: []Action{{ID: "export"}}}, false},
		{"a review source", Contributions{ReviewSource: &ReviewSourceContribution{}}, false},
	}
	for _, each := range cases {
		plugin := &Plugin{Manifest: Manifest{Contributes: each.added}}
		if plugin.NeedsRestart() != each.waits {
			test.Errorf("turning %s on waits for a restart: %v, want %v", each.name, plugin.NeedsRestart(), each.waits)
		}
	}
}
