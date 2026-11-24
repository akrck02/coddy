package window

import (
	"github.com/akrck02/code-review/internal/settings"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/code-review/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	"github.com/akrck02/go-ui/uitest"
)

func TestTheSettingsChooseTheThemeAndPaintTheWindowInIt(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, Open: "lighthouse/src/lamp.go", ReadOnly: true})
	window := newWindow(test, application)
	if application.ThemeName() != theme.DefaultLight {
		test.Fatalf("with no theme chosen and not dark, the reader starts in %q, want %q", application.ThemeName(), theme.DefaultLight)
	}
	if err := window.Click("Settings (⌘,)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if !application.settingsScreen.Open || hasButton(window, "lamp.go") {
		test.Fatal("the rail's button does not show the settings in place of the file")
	}
	if err := window.Click("Dark Modern"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if application.ThemeName() != "Dark Modern" {
		test.Fatalf("after choosing Dark Modern the theme is %q", application.ThemeName())
	}
	if application.Look().Editor != theme.Find(application.themes, "Dark Modern", true).Look.Editor {
		test.Error("the window is not painted in Dark Modern")
	}
	if err := window.Click("Keys"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	if application.settingsScreen.Section != settings.Keys {
		test.Error("the list of sections does not show the keys")
	}
	press(window, input.KeyEscape)
	if application.settingsScreen.Open {
		test.Error("Esc leaves the settings open")
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

func TestInTheDarkTheReaderStartsInIntelliJDark(test *testing.T) {
	application := newReader(Settings{Synchronous: true, Dark: true})
	if application.ThemeName() != "IntelliJ Dark" {
		test.Errorf("with no theme chosen and dark, the reader starts in %q, not IntelliJ Dark", application.ThemeName())
	}
}

// The settings are no mode: the rail marks none while they show, and choosing a mode leaves them
// for it, the mode that was on included.
func TestChoosingAModeLeavesTheSettings(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true})
	window := newWindow(test, application)
	review := "Review (2): only the changed files, as diffs, marked as viewed one by one"
	// the first time from the files mode, the second from the review, already on
	for range 2 {
		application.settingsScreen.Toggle(settings.Appearance)
		window.Frames(2)
		if err := window.Click(review); err != nil {
			test.Fatal(err)
		}
		window.Frames(2)
		if application.settingsScreen.Open || application.mode.Name() != "Review" {
			test.Errorf("choosing the review leaves the settings open (%v) or the mode %s", application.settingsScreen.Open, application.mode.Name())
		}
	}
}

// harbourIconThemes are the made-up icon themes of the theme package's tests.
func harbourIconThemes(test *testing.T) []theme.IconTheme {
	test.Helper()
	themes, problems := theme.LoadIconThemes(filepath.Join("..", "theme", "testdata"))
	if len(themes) != 1 || len(problems) != 0 {
		test.Fatalf("the made-up icon themes read into %d themes, %v", len(themes), problems)
	}
	return themes
}

func TestChoosingAnIconThemePaintsTheSidebarWithIt(test *testing.T) {
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true,
		IconThemes: harbourIconThemes(test)})
	window := newWindow(test, application)
	if application.IconThemeName() != "" || drawnPictures(window) != 0 {
		test.Fatalf("with none chosen the reader starts in the icon theme %q, with pictures", application.IconThemeName())
	}
	if err := window.Click("Settings (⌘,)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	scrollTo(test, window, "One Dark", "File icons: Harbour")
	if err := window.Click("File icons: Harbour"); err != nil {
		test.Fatal(err)
	}
	press(window, input.KeyEscape)
	window.Frames(2)
	if application.IconThemeName() != "Harbour" || drawnPictures(window) == 0 {
		test.Errorf("after choosing Harbour the icon theme is %q and the sidebar draws %d pictures", application.IconThemeName(), drawnPictures(window))
	}
	reopened := newReader(Settings{Folders: application.Folders(), Synchronous: true, IconThemes: harbourIconThemes(test), IconTheme: "Harbour"})
	if reopened.IconThemeName() != "Harbour" {
		test.Errorf("the icon theme chosen is not taken back: %q", reopened.IconThemeName())
	}
}

// scrollTo turns the wheel over an element until another, further down the page, shows.
func scrollTo(test *testing.T, window *uitest.Window, over, wanted string) {
	test.Helper()
	for turns := 0; turns < 40; turns++ {
		if found, err := window.Element(wanted); err == nil && found.Visible {
			return
		}
		if err := window.Scroll(over, -3); err != nil {
			test.Fatal(err)
		}
	}
	test.Fatalf("%q does not show however far the page is scrolled", wanted)
}

// drawnPictures counts the pictures the last frame drew: the icon theme's icons.
func drawnPictures(window interface{ DrawList() *draw2d.List }) int {
	count := 0
	for _, command := range window.DrawList().Commands {
		if command.Kind == draw2d.ImageKind {
			count++
		}
	}
	return count
}

func TestANameTooLongForItsCardEndsInAnEllipsis(test *testing.T) {
	icons := harbourIconThemes(test)
	long := "Harbour at night, with every lamp along the pier lit"
	icons[0].Name = long
	application := newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, ReadOnly: true, IconThemes: icons})
	window := newWindow(test, application)
	if err := window.Click("Settings (⌘,)"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	scrollTo(test, window, "One Dark", "File icons: "+long)
	for turns := 0; turns < 10; turns++ { // to the page's end, so the card shows whole
		if err := window.Scroll("File icons: "+long, -3); err != nil {
			test.Fatal(err)
		}
	}
	card, err := window.Element("File icons: " + long)
	if err != nil {
		test.Fatal(err)
	}
	name := ""
	for _, command := range window.DrawList().Commands {
		if command.Kind == draw2d.TextKind && strings.HasPrefix(command.Text, "Harbour") {
			name = command.Text
		}
	}
	if name == long || !strings.HasSuffix(name, "…") {
		test.Errorf("the name is drawn as %q, past its card %v", name, card.Rect)
	}
}
