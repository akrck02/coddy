package window

import (
	"testing"

	"github.com/akrck02/code-review/internal/theme"
)

func TestTheSettingsChooseTheThemeAndPaintTheWindowInIt(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	if application.ThemeName() != theme.DefaultLight {
		test.Fatalf("with no theme chosen and not dark, the reader starts in %q, want %q", application.ThemeName(), theme.DefaultLight)
	}
	if err := window.Click("Settings…"); err == nil {
		test.Fatal("the settings are in the menu, not on the window")
	}
	application.settingsOpen = true
	window.Frames(2)
	if err := window.Click("One Light"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if err := window.Click("Darcula"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if application.ThemeName() != "Darcula" {
		test.Fatalf("after choosing Darcula the theme is %q", application.ThemeName())
	}
	if application.Look().Editor != theme.Find(application.themes, "Darcula", true).Look.Editor {
		test.Error("the window is not painted in Darcula")
	}
	if err := window.Click("Done"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if application.settingsOpen {
		test.Error("Done leaves the settings open")
	}
}

func TestTheThemeChosenLastTimeIsTheOneTheReaderStartsIn(test *testing.T) {
	application := newReader(Settings{Synchronous: true, Theme: "IntelliJ Light"})
	if application.ThemeName() != "IntelliJ Light" || application.Look().Editor != theme.Find(application.themes, "IntelliJ Light", false).Look.Editor {
		test.Errorf("the reader starts in %q", application.ThemeName())
	}
	gone := newReader(Settings{Synchronous: true, Theme: "A theme deleted since", Dark: false})
	if gone.ThemeName() != theme.DefaultLight {
		test.Errorf("with its theme gone, the reader starts in %q, want %q", gone.ThemeName(), theme.DefaultLight)
	}
}
