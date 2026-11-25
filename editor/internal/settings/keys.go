package settings

import (
	"errors"
	"strconv"
	"strings"

	"github.com/akrck02/coddy/editor/internal/keymap"
	"github.com/akrck02/coddy/editor/internal/pane"
	"github.com/akrck02/coddy/editor/internal/theme"
	"github.com/akrck02/go-renderer/draw2d"
	"github.com/akrck02/go-renderer/input"
	ui "github.com/akrck02/go-ui"
)

const (
	// keysListWidth is the widest the list of keys grows, so an action and its keys stay close
	keysListWidth = 860
	// keyCapsWidth is the room of an action's keys, on the right of its row
	keyCapsWidth = 300
	// alternativeKeysGap is the room between two keys that do the same, three times that between
	// the caps of one key
	alternativeKeysGap = 12
	// keyIconSize is the size of an action's icon, and of a section's, in pixels
	keyIconSize = 16
	// profileSheetWidth is the width of the sheet that names a profile, in pixels
	profileSheetWidth = 420
)

// KeySection is the actions of one topic: its title and icon, and a row for each action.
type KeySection struct {
	Title, Icon string
	Rows        []KeyRow
}

// KeyRow is an action as the keys screen shows it.
type KeyRow struct {
	// ID names the action to change its keys; "" for a key the editor answers itself, which stays
	ID          string
	Icon        string
	Description string
	// Where says where it works, when not anywhere: "In the sidebar", "In the Uses mode"
	Where string
	// Keys are how each of its keys is written, "⌘⇧F"; Changed tells the profile changed them
	Keys    []string
	Changed bool
}

// naming is what the sheet that names a profile does: nothing while it is closed.
type naming int

const (
	notNaming naming = iota
	namingNew
	renaming
)

// keysEditor is what the keys screen keeps between frames: the words searched for, the key being
// changed, and the sheet that names a profile.
type keysEditor struct {
	search string
	// topic is the section shown, a place in the window's KeySections
	topic int
	// recording is the action whose key is being changed, "" while none is, and slot which of its keys
	recording string
	slot      int
	// pending is a chord pressed for it that runs another action already, conflict that action's
	// description: both wait for Use it or Cancel
	pending  pane.Press
	conflict string
	// problem is why the last chord pressed, or the last change, was refused
	problem string
	// naming is what the sheet does, name what is typed in it and nameProblem why it was refused
	naming      naming
	name        string
	nameProblem string
}

// Recording tells whether the keys screen waits for the keys of an action: the window then runs
// none of its commands, so the keys pressed are recorded, not run.
func (screen *Screen) Recording() bool { return screen.Open && screen.keys.recording != "" }

// buildKeys is the keys section: the profile chosen, a search field, a chip for each topic and
// the actions of the one chosen, each with its icon and its keys drawn as caps; in a profile of the person's, a click on a key
// changes it.
func (screen *Screen) buildKeys(context *ui.Context, window Window) {
	editor := &screen.keys
	editable := window.KeyProfile() != keymap.BuiltIn
	if editor.recording != "" && editor.conflict == "" {
		editor.record(window)
	}
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Fixed(keysListWidth), Gap: 24}, func() {
		editor.buildProfiles(context, window, editable)
		context.Frame(ui.Layout{Width: ui.Grow(), Padding: ui.PadXY(12, 2), Gap: 8, AlignY: ui.Center},
			ui.Look{Fill: context.Theme.Sunken, Radius: 8}, func() {
				context.IconSized("search", keyIconSize, context.Theme.InkQuiet)
				context.EntryField("Search the keys", &editor.search,
					ui.TextFieldOptions{Placeholder: "Search an action or a key", Plain: true})
			})
		editor.buildTopics(context, window, editable)
	})
}

// buildTopics is a chip for each topic, with how many actions it lists, and the actions of the
// one chosen under them. While a search finds nothing in the topic chosen, the first that has
// some is shown.
func (editor *keysEditor) buildTopics(context *ui.Context, window Window, editable bool) {
	sections := window.KeySections()
	found := make([][]KeyRow, len(sections))
	for index, section := range sections {
		found[index] = matching(section, editor.search)
	}
	if editor.topic >= len(sections) || len(found[editor.topic]) == 0 {
		for index := range found {
			if len(found[index]) > 0 {
				editor.topic = index
				break
			}
		}
	}
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, Wrap: true}, func() {
		context.Push("topics")
		defer context.Pop()
		for index, section := range sections {
			count := strconv.Itoa(len(found[index]))
			if buildTabChip(context, window.Look(), section.Icon, section.Title, count, index == editor.topic) {
				editor.topic = index
			}
		}
	})
	if editor.topic >= len(sections) || len(found[editor.topic]) == 0 {
		context.Text("No action or key matches “"+editor.search+"”", ui.TextStyle{Color: context.Theme.InkSoft})
		return
	}
	editor.buildSection(context, window, sections[editor.topic], found[editor.topic], editable)
}

// matching are a section's rows whose action, place or keys hold the words searched for, all of
// them when the section's title does, or nothing is searched.
func matching(section KeySection, search string) []KeyRow {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" || strings.Contains(strings.ToLower(section.Title), search) {
		return section.Rows
	}
	var rows []KeyRow
	for _, row := range section.Rows {
		words := strings.ToLower(row.Description + " " + row.Where + " " + strings.Join(row.Keys, " "))
		if strings.Contains(words, search) {
			rows = append(rows, row)
		}
	}
	return rows
}

// buildProfiles is the card of profiles: one chip for each, the chosen one lit, the buttons that
// make, rename and delete one, and a line on how keys change.
func (editor *keysEditor) buildProfiles(context *ui.Context, window Window, editable bool) {
	palette := context.Theme
	card := ui.Look{Fill: palette.Sunken, Radius: 10}
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.Pad(16), Gap: 12}, card, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center}, func() {
			context.IconSized("keyboard", keyIconSize, palette.InkSoft)
			context.Text("Profile", ui.TextStyle{Weight: draw2d.Bold})
			context.Box(ui.Layout{Width: ui.Grow(), Gap: 6, Wrap: true, Padding: ui.Padding{Left: 8}}, func() {
				for _, name := range window.KeyProfiles() {
					look := ui.ChipLook{Fill: palette.Surface, Ink: palette.InkSoft}
					if name == window.KeyProfile() {
						look = ui.ChipLook{Fill: palette.PrimaryTint, Ink: palette.Primary}
					}
					if context.Chip("", name, look) {
						editor.stopRecording()
						window.ChooseKeyProfile(name)
					}
				}
			})
			if theme.LabelledButton(context, "Make a profile, a copy of this one", "plus", "New profile") {
				editor.openNaming(namingNew, "")
			}
			if editable && theme.ToolButton(context, "Rename this profile", "pencil", false) {
				editor.openNaming(renaming, window.KeyProfile())
			}
			if editable && theme.ToolButton(context, "Delete this profile", "trash-2", false) {
				editor.stopRecording()
				editor.problem = problemWords(window.RemoveKeyProfile())
			}
		})
		hint := "Click a key, then press the new one; Esc stops. A key that types a letter needs " + modifiers() + "."
		if !editable {
			hint = "Coddy's own keys stay as they are: make a new profile to change them."
		}
		context.Text(hint, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkSoft})
	})
}

// buildSection is the actions of a topic: a card with a row for each, a hairline between rows.
func (editor *keysEditor) buildSection(context *ui.Context, window Window, section KeySection, rows []KeyRow, editable bool) {
	palette := context.Theme
	card := ui.Look{Fill: palette.Sunken, Radius: 10}
	context.Frame(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(16, 4)}, card, func() {
		context.Push(section.Title)
		defer context.Pop()
		for index, row := range rows {
			if index > 0 {
				context.Frame(ui.Layout{Width: ui.Grow(), Height: ui.Fixed(1)}, ui.Look{Fill: palette.Line}, nil)
			}
			editor.buildRow(context, window, row, editable && row.ID != "")
		}
	})
}

// buildRow is an action: its icon, what it does and where, its keys on the right; under it, what waits on a key pressed for it.
func (editor *keysEditor) buildRow(context *ui.Context, window Window, row KeyRow, editable bool) {
	palette := context.Theme
	context.Push(row.ID + row.Description)
	defer context.Pop()
	context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Padding: ui.PadXY(0, 10), Gap: 8}, func() {
		context.Box(ui.Layout{Width: ui.Grow(), Gap: 12, AlignY: ui.Center}, func() {
			context.IconSized(row.Icon, keyIconSize, palette.InkSoft)
			context.Box(ui.Layout{Direction: ui.Vertical, Width: ui.Grow(), Gap: 2}, func() {
				// go-ui draws a text on one line, so the words wrap as pieces of a box
				context.Box(ui.Layout{Width: ui.Grow(), Gap: 4, Wrap: true}, func() {
					for _, word := range strings.Fields(row.Description) {
						context.Label(word)
					}
				})
				if row.Where != "" {
					context.Text(row.Where, ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
				}
			})
			editor.buildCaps(context, window, row, editable)
		})
		if editor.recording == row.ID && row.ID != "" {
			editor.buildWaiting(context, window)
		}
	})
}

// buildCaps is an action's keys as caps, those that do the same well apart; in a profile of the person's each is a button that
// starts recording its new keys, and a changed action has one that gives Coddy's back.
func (editor *keysEditor) buildCaps(context *ui.Context, window Window, row KeyRow, editable bool) {
	palette := context.Theme
	// keys that do the same stand further apart than the caps of one key, so no dot is needed
	// between them, and none is left alone where the line wraps
	context.Box(ui.Layout{Width: ui.Fixed(keyCapsWidth), Gap: alternativeKeysGap, Wrap: true, AlignX: ui.End, AlignY: ui.Center}, func() {
		if len(row.Keys) == 0 {
			context.Text("No key", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.InkQuiet})
		}
		for index, key := range row.Keys {
			if !editable {
				theme.KeyCaps(context, key)
				continue
			}
			editor.buildCapButton(context, row.ID, index, key)
		}
		if editable && row.Changed && theme.ToolButton(context, "Give it Coddy's keys again", "rotate-ccw", false) {
			editor.stopRecording()
			editor.problem = problemWords(window.ResetKeys(row.ID))
		}
	})
}

// buildCapButton is one key of an action as a button: a click records its new keys, and while it
// records it asks for them.
func (editor *keysEditor) buildCapButton(context *ui.Context, id string, slot int, key string) {
	palette := context.Theme
	recording := editor.recording == id && editor.slot == slot
	normal := ui.Look{Radius: 6}
	if recording {
		normal = ui.Look{Radius: 6, Fill: palette.PrimaryTint, Border: palette.Primary, BorderWidth: 1}
	}
	// as tall as a cap, so the row keeps its height while it records
	layout := ui.Layout{Height: ui.Fixed(theme.KeyCapHeight), AlignY: ui.Center}
	if recording {
		layout.Padding = ui.PadXY(10, 0)
	}
	hovered := normal
	hovered.Border, hovered.BorderWidth = palette.Primary, 1
	looks := ui.Looks{Normal: normal, Hovered: hovered, Pressed: hovered}
	name := "Change " + key
	clicked := context.Clickable(name, layout, looks, func() {
		if recording {
			context.Text("Press the keys…", ui.TextStyle{Size: palette.SmallTextSize, Color: palette.Primary})
			return
		}
		theme.KeyCaps(context, key)
	})
	context.Describe(ui.Description{Role: ui.ButtonRole, Label: name})
	context.Tooltip("Click, then press the new keys")
	if clicked {
		editor.stopRecording()
		editor.recording, editor.slot = id, slot
	}
}

// buildWaiting is the line under an action whose keys are recorded: why the keys pressed were
// refused, or which action they would take from, with Use them and Cancel.
func (editor *keysEditor) buildWaiting(context *ui.Context, window Window) {
	palette := context.Theme
	if editor.conflict == "" {
		if editor.problem != "" {
			context.Text(editor.problem, ui.TextStyle{Size: palette.SmallTextSize, Color: warningInk})
		}
		return
	}
	context.Box(ui.Layout{Width: ui.Grow(), Gap: 8, AlignY: ui.Center, Padding: ui.Padding{Left: keyIconSize + 12}}, func() {
		context.IconSized("triangle-alert", keyIconSize, warningInk)
		context.Text(editor.pending.Label+" runs “"+editor.conflict+"”: it would lose it",
			ui.TextStyle{Size: palette.SmallTextSize, Color: warningInk})
		context.Spacer()
		if context.FlatButton("Cancel") {
			editor.stopRecording()
		}
		if context.PrimaryButton("Use it") {
			id, slot, press := editor.recording, editor.slot, editor.pending
			editor.stopRecording()
			editor.problem = problemWords(window.SetKey(id, slot, press))
		}
	})
}

// record reads the keys pressed for the action being changed: Esc stops, a chord that could be
// typed by mistake is refused, one that runs another action waits for an answer, and any other
// becomes the action's key.
func (editor *keysEditor) record(window Window) {
	press, found := keymap.Recorded(window.Keys())
	if !found {
		return
	}
	if press.Key == input.KeyEscape && !press.Command && !press.Alt && !press.Shift {
		editor.stopRecording()
		return
	}
	if !keymap.Safe(press) {
		editor.problem = theme.KeyCapWords(press.Label) + " would type: hold " + modifiers() + " with it, or use a function key"
		return
	}
	if conflict := window.KeyConflict(editor.recording, press); conflict != "" {
		editor.pending, editor.conflict = press, conflict
		return
	}
	id, slot := editor.recording, editor.slot
	editor.stopRecording()
	editor.problem = problemWords(window.SetKey(id, slot, press))
}

// stopRecording forgets the key being changed and what waited on it.
func (editor *keysEditor) stopRecording() {
	editor.recording, editor.slot, editor.pending, editor.conflict, editor.problem = "", 0, pane.Press{}, "", ""
}

// openNaming opens the sheet that names a profile, with a name to start from.
func (editor *keysEditor) openNaming(what naming, name string) {
	editor.stopRecording()
	editor.naming, editor.name, editor.nameProblem = what, name, ""
}

// buildNaming is the sheet that names a new profile, or the one chosen, when it is open.
func (screen *Screen) buildNaming(context *ui.Context, window Window) {
	editor := &screen.keys
	if editor.naming == notNaming {
		return
	}
	title, button := "New profile", "Create"
	if editor.naming == renaming {
		title, button = "Rename the profile", "Rename"
	}
	done := false
	dismissed := context.Sheet(title, ui.SheetOptions{Width: ui.Fixed(profileSheetWidth)}, func() {
		palette := context.Theme
		if editor.naming == namingNew {
			context.Text("It starts as a copy of "+window.KeyProfile()+".", ui.TextStyle{Color: palette.InkSoft})
		}
		submitted := context.EntryField("Profile name", &editor.name, ui.TextFieldOptions{Placeholder: "The profile's name"})
		if editor.nameProblem != "" {
			context.Text(editor.nameProblem, ui.TextStyle{Size: palette.SmallTextSize, Color: warningInk})
		}
		context.Box(ui.Layout{Width: ui.Grow(), Padding: ui.Padding{Top: 8}}, func() {
			context.Spacer()
			if context.PrimaryButton(button) || submitted {
				done = editor.saveName(window)
			}
		})
	})
	if dismissed || done {
		editor.naming = notNaming
	}
}

// saveName makes or renames the profile with the name typed, and tells whether it could.
func (editor *keysEditor) saveName(window Window) bool {
	name := strings.TrimSpace(editor.name)
	err := window.NewKeyProfile(name)
	if editor.naming == renaming {
		err = window.RenameKeyProfile(name)
	}
	editor.nameProblem = problemWords(err)
	return err == nil
}

// modifiers are the keys that keep a letter from typing, as this system names them.
func modifiers() string { return theme.ModifierName("⌘") + " or " + theme.ModifierName("⌥") }

// problemWords say why a change of keys was refused, "" when it was not.
func problemWords(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, keymap.BadName):
		return "That name is taken, empty, or has a slash: choose another"
	case errors.Is(err, keymap.NotAChord):
		return "Those keys cannot run an action"
	default:
		return "The profile could not be saved in " + keymap.Folder()
	}
}
