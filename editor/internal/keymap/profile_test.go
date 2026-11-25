package keymap

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/go-renderer/input"
)

func TestAProfileSavedIsLoadedBackWithItsKeys(test *testing.T) {
	folder := filepath.Join(test.TempDir(), "keymaps")
	saved := Profile{Name: "Harbour", Keys: map[string][]string{"find-text": {"Alt+F"}, "help": {}}}
	if err := Save(folder, saved); err != nil {
		test.Fatal(err)
	}
	profiles, problems := Load(folder)
	if len(problems) != 0 || len(profiles) != 1 || profiles[0].Name != "Harbour" {
		test.Fatalf("loaded %v with problems %v", profiles, problems)
	}
	if keys := profiles[0].Keys["find-text"]; len(keys) != 1 || keys[0] != "Alt+F" {
		test.Errorf("find-text has %v, want Alt+F", keys)
	}
}

func TestAChangedKeyTakesThePlaceOfCoddysAndKeepsWhichWayItGoes(test *testing.T) {
	builtIn := []pane.Press{{Key: input.KeyF7, Label: "F7", Step: 1}, {Key: input.KeyF7, Shift: true, Label: "⇧F7", Step: -1}}
	applied := Applied(builtIn, []string{"Alt+Down", ""})
	if len(applied) != 1 || Text(applied[0]) != "Alt+Down" || applied[0].Step != 1 {
		test.Errorf("applied %+v, want only Alt+Down going forward", applied)
	}
	if kept := Applied(builtIn, nil); len(kept) != 2 || kept[1].Label != "⇧F7" {
		test.Errorf("with nothing written, applied %+v, want Coddy's two keys", kept)
	}
}

func TestAProfileRemovedIsNoLongerLoaded(test *testing.T) {
	folder := test.TempDir()
	if err := Save(folder, Profile{Name: "Harbour"}); err != nil {
		test.Fatal(err)
	}
	if err := Remove(folder, "Harbour"); err != nil {
		test.Fatal(err)
	}
	if profiles, _ := Load(folder); len(profiles) != 0 {
		test.Errorf("loaded %v after it was removed", profiles)
	}
}

func TestCoddysOwnNameAndNamesUnfitForAFileAreRefused(test *testing.T) {
	for _, name := range []string{"", " ", "Coddy", "coddy", "a/b", ".hidden"} {
		if err := CheckName(name); !errors.Is(err, BadName) {
			test.Errorf("%q gave %v, want %v", name, err, BadName)
		}
	}
}

func TestEachErrorCarriesItsCode(test *testing.T) {
	folder := test.TempDir()
	if err := os.WriteFile(filepath.Join(folder, "broken.json"), []byte("{"), 0o600); err != nil {
		test.Fatal(err)
	}
	_, problems := Load(folder)
	_, notAChord := Parse("Hyper+F")
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a file that is not JSON", problems[0], NotAProfile},
		{"a chord of no keys", notAChord, NotAChord},
		{"Coddy's own name", CheckName(BuiltIn), BadName},
		{"a profile that is not there", Remove(folder, "Nowhere"), NotSaved},
		{"a folder inside a file", Save(filepath.Join(folder, "broken.json"), Profile{Name: "Harbour"}), NotSaved},
	}
	for _, each := range cases {
		if !errors.Is(each.err, each.code) {
			test.Errorf("%s gave %v, want %v", each.name, each.err, each.code)
		}
	}
}
