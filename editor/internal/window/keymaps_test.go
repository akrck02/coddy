package window

import (
	"errors"
	"testing"

	"github.com/akrck02/coddy/editor/internal/keymap"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/settings"
	"github.com/akrck02/go-renderer/input"
)

// A profile names each command by its id: two commands with one id would share their keys.
func TestEveryCommandHasAnIdOfItsOwn(test *testing.T) {
	application, _ := readerOfUses(test)
	seen := map[string]string{}
	for _, each := range application.allCommands() {
		if each.id == "" {
			test.Errorf("%q has no id", each.description)
		}
		if other, taken := seen[each.id]; taken {
			test.Errorf("%q and %q share the id %q", each.description, other, each.id)
		}
		seen[each.id] = each.description
	}
}

// folderChooser stands in for the system's folder chooser: it counts how often it was opened, and
// answers with a folder, "" for none.
type folderChooser struct {
	opened int
	start  string
	answer string
}

func (chooser *folderChooser) choose(title, start string, chosen func(folder string)) bool {
	chooser.opened++
	chooser.start = start
	chosen(chooser.answer)
	return true
}

// keysReader is a reader whose profiles of keys are kept in a folder of the test, with a profile
// chosen in which Alt+O opens the folder chooser in place of ⌘O.
func keysReader(test *testing.T) (*Window, *folderChooser) {
	test.Helper()
	folder := test.TempDir()
	if err := keymap.Save(folder, keymap.Profile{Name: "Harbour", Keys: map[string][]string{"open-folder": {"Alt+O"}}}); err != nil {
		test.Fatal(err)
	}
	chooser := &folderChooser{}
	return newReader(Settings{Folders: []string{makeRepository(test)}, Synchronous: true, KeymapsFolder: folder, KeyProfile: "Harbour",
		ChooseFolder: chooser.choose}), chooser
}

func TestTheKeysOfTheProfileChosenRunTheCommands(test *testing.T) {
	application, chooser := keysReader(test)
	window := newWindow(test, application)
	press(window, input.KeyO, input.KeySuper)
	if chooser.opened != 0 {
		test.Error("⌘O, Coddy's key, opened the chooser in a profile that changed it")
	}
	press(window, input.KeyO, input.KeyAlt)
	if chooser.opened != 1 {
		test.Error("⌥O, the profile's key, did not open the chooser")
	}
}

func TestCoddysOwnKeysComeBackWhenItsProfileIsChosen(test *testing.T) {
	application, chooser := keysReader(test)
	window := newWindow(test, application)
	application.ChooseKeyProfile(keymap.BuiltIn)
	press(window, input.KeyO, input.KeySuper)
	if chooser.opened != 1 {
		test.Error("⌘O did not open the chooser with Coddy's own keys")
	}
}

// A key given to one command is taken from the one that had it, so no key runs two.
func TestAKeyGivenToACommandIsTakenFromTheOneThatHadIt(test *testing.T) {
	application, _ := keysReader(test)
	newWindow(test, application)
	command := pressOf(test, "Command+F")
	if conflict := application.KeyConflict("find-text", command); conflict == "" {
		test.Fatal("⌘F, the search in a project, is told as free")
	}
	if err := application.SetKey("find-text", 0, command); err != nil {
		test.Fatal(err)
	}
	if inProject, _ := application.commandOf("find-text-in-project"); len(inProject.keys) != 0 {
		test.Errorf("the search in a project keeps %v after ⌘F went to the search in every project", labels(inProject.keys))
	}
	if found, _ := application.commandOf("find-text"); len(found.keys) != 1 || found.keys[0].Label != "⌘F" {
		test.Errorf("the search in every project has %v, want ⌘F", labels(found.keys))
	}
}

func TestAKeyThatWouldTypeIsRefused(test *testing.T) {
	application, _ := keysReader(test)
	newWindow(test, application)
	if err := application.SetKey("find-text", 0, pressOf(test, "Shift+F")); !errors.Is(err, keymap.NotAChord) {
		test.Errorf("⇧F gave %v, want %v", err, keymap.NotAChord)
	}
}

func TestCoddysOwnKeysCannotChange(test *testing.T) {
	application, _ := keysReader(test)
	newWindow(test, application)
	application.ChooseKeyProfile(keymap.BuiltIn)
	if err := application.SetKey("find-text", 0, pressOf(test, "Alt+F")); err == nil {
		test.Error("a key of Coddy's own profile changed")
	}
}

func TestANewProfileIsACopyOfTheOneChosenAndIsKept(test *testing.T) {
	application, _ := keysReader(test)
	newWindow(test, application)
	if err := application.NewKeyProfile("Lagoon"); err != nil {
		test.Fatal(err)
	}
	profiles, _ := keymap.Load(application.keymapsFolder())
	if len(profiles) != 2 || application.KeyProfile() != "Lagoon" {
		test.Fatalf("profiles %v, chosen %q", profiles, application.KeyProfile())
	}
	if open, _ := application.commandOf("open-folder"); len(open.keys) != 1 || open.keys[0].Label != "⌥O" {
		test.Errorf("the copy opens a folder with %v, want ⌥O", labels(open.keys))
	}
	if err := application.RemoveKeyProfile(); err != nil {
		test.Fatal(err)
	}
	if profiles, _ := keymap.Load(application.keymapsFolder()); len(profiles) != 1 || application.KeyProfile() != keymap.BuiltIn {
		test.Errorf("after deleting it: profiles %v, chosen %q", profiles, application.KeyProfile())
	}
}

// Click, then press: the keys screen records the chord pressed as the command's key, and the
// window runs none of its commands meanwhile.
func TestAKeyClickedOnTheKeysScreenTakesTheChordPressedNext(test *testing.T) {
	application, chooser := keysReader(test)
	window := newWindow(test, application)
	press(window, input.KeyF1)
	if err := window.Click("Change ⌥O"); err != nil {
		test.Fatal(err)
	}
	window.Frames(2)
	press(window, input.KeyK, input.KeyAlt)
	if open, _ := application.commandOf("open-folder"); len(open.keys) != 1 || open.keys[0].Label != "⌥K" {
		test.Errorf("opening a folder has %v, want ⌥K", labels(open.keys))
	}
	if chooser.opened != 0 {
		test.Error("the keys pressed while recording ran a command")
	}
}

func TestTheKeysScreenListsTheActionsByTopic(test *testing.T) {
	application, _ := keysReader(test)
	newWindow(test, application)
	titles := map[string]bool{}
	for _, section := range application.KeySections() {
		titles[section.Title] = len(section.Rows) > 0
	}
	for _, title := range []string{"Projects and files", "Finding", "Changes and review", "Code", "Editing", "View", "Window", "Modes"} {
		if !titles[title] {
			test.Errorf("no action is listed under %q", title)
		}
	}
	var changed []settings.KeyRow
	for _, section := range application.KeySections() {
		for _, row := range section.Rows {
			if row.Changed {
				changed = append(changed, row)
			}
		}
	}
	if len(changed) != 1 || changed[0].ID != "open-folder" {
		test.Errorf("the rows told as changed are %v, want only open-folder", changed)
	}
}

// pressOf reads a chord written as a profile writes it.
func pressOf(test *testing.T, text string) pane.Press {
	test.Helper()
	press, err := keymap.Parse(text)
	if err != nil {
		test.Fatal(err)
	}
	return press
}
