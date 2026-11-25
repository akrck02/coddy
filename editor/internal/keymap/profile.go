package keymap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/go-renderer/failure"
)

// BuiltIn is the name of Coddy's own profile: the keys its commands are given. It has no file and
// cannot be changed; a profile of the person's starts as a copy of it.
const BuiltIn = "Coddy"

// Profile is a set of keys that differ from Coddy's own: for each action changed, by its id, the
// chords that take the place of Coddy's keys, written as Text writes them (see Applied). An action
// not in Keys keeps Coddy's keys.
type Profile struct {
	Name string              `json:"name"`
	Keys map[string][]string `json:"keys"`
}

// Applied are an action's keys as a profile changes them: the chords written in its place, one for
// each of Coddy's keys, in their order, "" for a key taken away. A changed key keeps what Coddy's
// does (its step, whether it repeats); a chord that cannot be read is taken away.
func Applied(builtIn []pane.Press, texts []string) []pane.Press {
	presses := make([]pane.Press, 0, len(builtIn))
	for index, original := range builtIn {
		if index >= len(texts) {
			presses = append(presses, original)
			continue
		}
		press, err := Parse(texts[index])
		if err != nil {
			continue // taken away, or mistyped by hand: the key runs nothing
		}
		press.Step, press.Repeats = original.Step, original.Repeats
		// Esc and Tab answer whatever is held; a chord with modifiers asks for those
		press.AnyModifiers = original.AnyModifiers && !press.Command && !press.Alt && !press.Shift
		presses = append(presses, press)
	}
	return presses
}

// Folder is where the person's profiles are kept: keymaps/ in the reader's own folder.
func Folder() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "coddy", "keymaps")
}

// Load are the profiles of the .json files in a folder, by name. A file that cannot be read is told
// in problems and left out; a folder that is not there has none.
func Load(folder string) (profiles []Profile, problems []error) {
	if folder == "" {
		return nil, nil
	}
	paths, _ := filepath.Glob(filepath.Join(folder, "*.json")) // the pattern is well formed
	for _, path := range paths {
		profile, err := readProfile(path)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		profiles = append(profiles, profile)
	}
	slices.SortFunc(profiles, func(first, second Profile) int { return strings.Compare(first.Name, second.Name) })
	return profiles, problems
}

// readProfile reads one profile's file; its name is the file's.
func readProfile(path string) (Profile, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, failure.Wrap(NotAProfile, filepath.Base(path), err)
	}
	var profile Profile
	if err := json.Unmarshal(contents, &profile); err != nil {
		return Profile{}, failure.Wrap(NotAProfile, filepath.Base(path), err)
	}
	profile.Name = strings.TrimSuffix(filepath.Base(path), ".json")
	if profile.Keys == nil {
		profile.Keys = map[string][]string{}
	}
	return profile, nil
}

// CheckName tells whether a name can be a profile's: not empty, not Coddy's own, and fit for a
// file's name.
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" || strings.EqualFold(name, BuiltIn) || strings.ContainsAny(name, `/\:`) ||
		strings.HasPrefix(name, ".") {
		return failure.Wrap(BadName, name, nil)
	}
	return nil
}

// Save writes a profile to its file in a folder, making the folder if it is not there.
func Save(folder string, profile Profile) error {
	if err := CheckName(profile.Name); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return failure.Wrap(NotSaved, profile.Name, err)
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return failure.Wrap(NotSaved, folder, err)
	}
	if err := os.WriteFile(filepath.Join(folder, profile.Name+".json"), append(contents, '\n'), 0o644); err != nil {
		return failure.Wrap(NotSaved, profile.Name, err)
	}
	return nil
}

// Remove deletes a profile's file from a folder.
func Remove(folder, name string) error {
	if err := CheckName(name); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(folder, name+".json")); err != nil {
		return failure.Wrap(NotSaved, name, err)
	}
	return nil
}
