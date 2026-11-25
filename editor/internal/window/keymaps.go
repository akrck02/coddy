package window

import (
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/keymap"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/settings"
	"github.com/akrck02/coddy/editor/internal/viewer"
)

// topic is the section of the keys screen a command is listed in.
type topic int

const (
	topicProjects topic = iota
	topicFinding
	topicChanges
	topicCode
	topicEditing
	topicView
	topicWindow
	topicModes
)

// topicLooks are each topic's title and icon on the keys screen, in the order the sections show.
var topicLooks = [...]struct{ title, icon string }{
	topicProjects: {"Projects and files", "folder-tree"},
	topicFinding:  {"Finding", "search"},
	topicChanges:  {"Changes and review", "git-compare"},
	topicCode:     {"Code", "code"},
	topicEditing:  {"Editing", "pencil"},
	topicView:     {"View", "eye"},
	topicWindow:   {"Window", "app-window"},
	topicModes:    {"Modes", "layers"},
}

// placeNames say where a command works, when not anywhere, beside it on the keys screen.
var placeNames = map[place]string{inSidebar: "In the sidebar", inViewer: "In the file", editing: "While editing"}

// loadKeyProfiles reads the person's profiles of keys and chooses the one the settings name, if it
// is there. A profile that cannot be read is told, and left out.
func (application *Window) loadKeyProfiles() {
	profiles, problems := keymap.Load(application.keymapsFolder())
	for _, problem := range problems {
		application.tellProblem(problem)
	}
	application.keyProfiles = profiles
	application.keyProfile = ""
	if application.profileIndex(application.settings.KeyProfile) >= 0 {
		application.keyProfile = application.settings.KeyProfile
	}
}

// keymapsFolder is where the profiles of keys are kept.
func (application *Window) keymapsFolder() string {
	if application.settings.KeymapsFolder != "" {
		return application.settings.KeymapsFolder
	}
	return keymap.Folder()
}

// buildCommandTable makes the commands of every mode and of the window, with Coddy's own keys and
// with the keys of the profile chosen: once at the start, and when the profile or a key changes.
func (application *Window) buildCommandTable() {
	application.builtInTable = append(append(application.declaredCommands(true), commands...), application.modeCommands()...)
	table := slices.Clone(application.builtInTable)
	if index := application.profileIndex(application.keyProfile); index >= 0 {
		profile := application.keyProfiles[index]
		for each := range table {
			if texts, changed := profile.Keys[table[each].id]; changed {
				table[each].keys = keymap.Applied(table[each].keys, texts)
			}
		}
	}
	application.commandTable = table
}

// profileIndex is where a profile of a name is among the person's, -1 when it is not there.
func (application *Window) profileIndex(name string) int {
	return slices.IndexFunc(application.keyProfiles, func(profile keymap.Profile) bool { return profile.Name == name })
}

// KeySections are every command, by topic, with the keys of the profile chosen, for the keys
// screen; the editing keys the file answers itself close the editing section, and cannot change.
func (application *Window) KeySections() []settings.KeySection {
	sections := make([]settings.KeySection, len(topicLooks))
	for index, look := range topicLooks {
		sections[index] = settings.KeySection{Title: look.title, Icon: look.icon}
	}
	for index, each := range application.commandTable {
		row := settings.KeyRow{ID: each.id, Icon: each.icon, Description: each.description, Where: placeNames[each.place],
			Keys: labels(each.keys), Changed: !sameChords(each.keys, application.builtInTable[index].keys)}
		if each.owner != "" {
			row.Where = "In the " + each.owner + " mode"
		}
		if each.topic == topicEditing {
			row.Where = ""
		}
		sections[each.topic].Rows = append(sections[each.topic].Rows, row)
	}
	for _, key := range viewer.EditingKeys {
		sections[topicEditing].Rows = append(sections[topicEditing].Rows, settings.KeyRow{Icon: "type", Description: key.Description, Keys: key.Keys})
	}
	return sections
}

// labels are how the keys screen writes each of a command's keys.
func labels(presses []pane.Press) []string {
	written := make([]string, len(presses))
	for index, press := range presses {
		written[index] = press.Label
	}
	return written
}

// sameChords tells whether two lists of keys are the same keys, in the same order.
func sameChords(first, second []pane.Press) bool {
	return slices.EqualFunc(first, second, func(one, other pane.Press) bool { return keymap.Text(one) == keymap.Text(other) })
}

// KeyProfiles are the names of the profiles to choose from: Coddy's own first, then the person's.
func (application *Window) KeyProfiles() []string {
	names := []string{keymap.BuiltIn}
	for _, profile := range application.keyProfiles {
		names = append(names, profile.Name)
	}
	return names
}

// KeyProfile is the name of the profile chosen, to start with it the next time.
func (application *Window) KeyProfile() string {
	if application.keyProfile == "" {
		return keymap.BuiltIn
	}
	return application.keyProfile
}

// ChooseKeyProfile runs the commands with a profile's keys, from this frame on; Coddy's name, or
// one not there, chooses Coddy's own keys.
func (application *Window) ChooseKeyProfile(name string) {
	application.keyProfile = ""
	if application.profileIndex(name) >= 0 {
		application.keyProfile = name
	}
	application.buildCommandTable()
}

// NewKeyProfile makes a profile of the person's, a copy of the one chosen, saves it and chooses it.
func (application *Window) NewKeyProfile(name string) error {
	if application.profileIndex(name) >= 0 {
		return keymap.BadName
	}
	made := keymap.Profile{Name: name, Keys: map[string][]string{}}
	if index := application.profileIndex(application.keyProfile); index >= 0 {
		for id, texts := range application.keyProfiles[index].Keys {
			made.Keys[id] = slices.Clone(texts)
		}
	}
	if err := keymap.Save(application.keymapsFolder(), made); err != nil {
		return err
	}
	application.keyProfiles = append(application.keyProfiles, made)
	slices.SortFunc(application.keyProfiles, func(first, second keymap.Profile) int { return strings.Compare(first.Name, second.Name) })
	application.ChooseKeyProfile(name)
	return nil
}

// RenameKeyProfile gives the profile chosen another name; Coddy's own keeps its name.
func (application *Window) RenameKeyProfile(name string) error {
	index := application.profileIndex(application.keyProfile)
	if index < 0 || application.profileIndex(name) >= 0 {
		return keymap.BadName
	}
	renamed := application.keyProfiles[index]
	renamed.Name = name
	if err := keymap.Save(application.keymapsFolder(), renamed); err != nil {
		return err
	}
	if err := keymap.Remove(application.keymapsFolder(), application.keyProfile); err != nil {
		return err
	}
	application.keyProfiles[index] = renamed
	application.keyProfile = name
	return nil
}

// RemoveKeyProfile deletes the profile chosen and goes back to Coddy's own keys.
func (application *Window) RemoveKeyProfile() error {
	index := application.profileIndex(application.keyProfile)
	if index < 0 {
		return keymap.BadName
	}
	if err := keymap.Remove(application.keymapsFolder(), application.keyProfile); err != nil {
		return err
	}
	application.keyProfiles = slices.Delete(application.keyProfiles, index, index+1)
	application.ChooseKeyProfile("")
	return nil
}

// KeyConflict is what the keys run already, in a place the command of an id works in too: its
// description, "" when they run nothing there.
func (application *Window) KeyConflict(id string, press pane.Press) string {
	changed, found := application.commandOf(id)
	if !found {
		return ""
	}
	for _, each := range application.commandTable {
		if each.id == id || !shareKeyboard(changed, each) {
			continue
		}
		for _, other := range each.keys {
			if keymap.Text(other) == keymap.Text(press) {
				return each.description
			}
		}
	}
	return ""
}

// shareKeyboard tells whether two commands can answer the same keys at once: one works anywhere,
// or both in the same place, and neither is kept to a mode the other is not.
func shareKeyboard(first, second command) bool {
	samePlace := first.place == second.place || first.place == anywhere || second.place == anywhere
	return samePlace && (first.owner == "" || second.owner == "" || first.owner == second.owner)
}

// SetKey makes a chord the key of a command, in the place of its key at slot, in the profile
// chosen, and saves it; a command that ran the chord, where this one works, loses it. Coddy's own
// keys do not change, and a chord that could be typed by mistake is refused.
func (application *Window) SetKey(id string, slot int, press pane.Press) error {
	index := application.profileIndex(application.keyProfile)
	if index < 0 {
		return keymap.BadName
	}
	if !keymap.Safe(press) {
		return keymap.NotAChord
	}
	changed, found := application.commandOf(id)
	if !found || slot < 0 || slot >= len(changed.keys) {
		return keymap.NotAChord
	}
	profile := application.keyProfiles[index]
	for _, each := range application.commandTable {
		if each.id == id || !shareKeyboard(changed, each) {
			continue
		}
		for other, chord := range each.keys {
			if keymap.Text(chord) == keymap.Text(press) {
				profile.Keys[each.id] = writtenKeys(profile.Keys[each.id], each.keys)
				profile.Keys[each.id][other] = ""
			}
		}
	}
	profile.Keys[id] = writtenKeys(profile.Keys[id], changed.keys)
	profile.Keys[id][slot] = keymap.Text(press)
	return application.saveKeyProfile(profile)
}

// ResetKeys gives a command Coddy's own keys again, in the profile chosen.
func (application *Window) ResetKeys(id string) error {
	index := application.profileIndex(application.keyProfile)
	if index < 0 {
		return keymap.BadName
	}
	profile := application.keyProfiles[index]
	delete(profile.Keys, id)
	return application.saveKeyProfile(profile)
}

// saveKeyProfile writes a profile changed, and runs the commands with its keys from now on.
func (application *Window) saveKeyProfile(profile keymap.Profile) error {
	if err := keymap.Save(application.keymapsFolder(), profile); err != nil {
		return err
	}
	application.buildCommandTable()
	return nil
}

// writtenKeys are a command's keys as a profile writes them, one for each of Coddy's: those the
// profile wrote already, or the command's keys now.
func writtenKeys(written []string, keys []pane.Press) []string {
	if len(written) > 0 {
		return written
	}
	texts := make([]string, len(keys))
	for index, press := range keys {
		texts[index] = keymap.Text(press)
	}
	return texts
}

// commandOf is the command of an id, with the keys it has now.
func (application *Window) commandOf(id string) (command, bool) {
	for _, each := range application.commandTable {
		if each.id == id {
			return each, true
		}
	}
	return command{}, false
}
