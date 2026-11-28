package plugins

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/akrck02/coddy/editor/internal/plugins/keyring"
	"github.com/akrck02/go-renderer/failure"
)

// The types of a plugin's setting, as its manifest gives them.
const (
	// SettingText is a line of text: a URL, a name.
	SettingText = "text"
	// SettingSecret is a line of text shown masked: a token, a password.
	SettingSecret = "secret"
	// SettingBoolean is on or off.
	SettingBoolean = "boolean"
	// SettingChoice is one of the setting's choices.
	SettingChoice = "choice"
)

// settingsFolderName is the folder, beside the plugins folder, that keeps the values set for each
// plugin: one file each, named as the plugin.
const settingsFolderName = "plugin-settings"

// settingsFileMode lets only the person read the values set: they say which servers the person
// uses, though never a secret, which the keyring keeps.
const settingsFileMode = 0o600

// secretsService is the service the keyring keeps plugins' secrets under; each is the account
// "<plugin>/<setting id>".
const secretsService = "coddy"

// Keyring keeps the values of plugins' secret settings, never written to their files. The
// system's (the keyring package) is used unless a test gives a fake.
type Keyring interface {
	Read(service, account string) (string, error)
	Write(service, account, secret string) error
	Delete(service, account string) error
}

// systemKeyring is the system's keyring, through the keyring package.
type systemKeyring struct{}

// Read is keyring.Read.
func (systemKeyring) Read(service, account string) (string, error) {
	return keyring.Read(service, account)
}

// Write is keyring.Write.
func (systemKeyring) Write(service, account, secret string) error {
	return keyring.Write(service, account, secret)
}

// Delete is keyring.Delete.
func (systemKeyring) Delete(service, account string) error {
	return keyring.Delete(service, account)
}

// Setting is something a person can set for a plugin, as its manifest declares it.
type Setting struct {
	// ID names the setting in the values the plugin's program is given: "organisationURL".
	ID string `json:"id"`
	// Title is what the settings call it, and Description says what it is for, in a line.
	Title       string `json:"title"`
	Description string `json:"description"`
	// Type is SettingText, SettingSecret, SettingBoolean or SettingChoice.
	Type string `json:"type"`
	// Default is its value until the person sets one: a string, or a boolean for SettingBoolean.
	// Without one it is empty, off, or the first choice.
	Default any `json:"default"`
	// Choices are what a SettingChoice may be, in the order they are offered.
	Choices []string `json:"choices"`
}

// settingsProblem tells what is wrong in a manifest's settings, empty when nothing is.
func settingsProblem(settings []Setting) (problem string) {
	seen := map[string]bool{}
	for _, setting := range settings {
		switch {
		case setting.ID == "":
			return "a setting needs an id"
		case seen[setting.ID]:
			return "two settings have the id " + setting.ID
		case !slices.Contains([]string{SettingText, SettingSecret, SettingBoolean, SettingChoice}, setting.Type):
			return "the setting " + setting.ID + ` has a type that is not "text", "secret", "boolean" or "choice"`
		case setting.Type == SettingChoice && len(setting.Choices) == 0:
			return "the setting " + setting.ID + " is a choice without choices"
		}
		seen[setting.ID] = true
	}
	return ""
}

// valueOf is a value read for a setting when it is of the setting's type, or else the setting's
// default, or else the zero of its type: empty, off, the first choice.
func (setting Setting) valueOf(read any) any {
	for _, candidate := range []any{read, setting.Default} {
		switch value := candidate.(type) {
		case bool:
			if setting.Type == SettingBoolean {
				return value
			}
		case string:
			if setting.Type == SettingChoice && slices.Contains(setting.Choices, value) {
				return value
			}
			if setting.Type == SettingText || setting.Type == SettingSecret {
				return value
			}
		}
	}
	switch setting.Type {
	case SettingBoolean:
		return false
	case SettingChoice:
		return setting.Choices[0]
	}
	return ""
}

// settingsFolderOf is the folder of the values set for the plugins installed in a folder.
func settingsFolderOf(pluginsFolder string) string {
	return filepath.Join(filepath.Dir(pluginsFolder), settingsFolderName)
}

// readSettings reads the values set for the plugin, kept in a folder, and makes its Values: a
// value never set, or of another type than its setting, takes the default. It runs when the
// plugin is read, never in a frame.
func (plugin *Plugin) readSettings(folder string) error {
	if len(plugin.Settings) == 0 {
		return nil
	}
	plugin.settingsFile = filepath.Join(folder, plugin.Name+".json")
	plugin.chosen = map[string]any{}
	content, err := os.ReadFile(plugin.settingsFile)
	if err == nil {
		err = json.Unmarshal(content, &plugin.chosen)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		plugin.chosen = map[string]any{}
		plugin.Values = plugin.valuesWith(plugin.chosen)
		return failure.Wrap(BadSettings, plugin.settingsFile, err)
	}
	for _, setting := range plugin.Settings {
		if setting.Type == SettingSecret {
			delete(plugin.chosen, setting.ID) // an older Coddy wrote secrets here: the next save drops them
		}
	}
	plugin.Values = plugin.valuesWith(plugin.chosen)
	return nil
}

// valuesWith are the value of every setting of the plugin, from those chosen or else defaults.
// A secret is never read from the file: it is empty, or what was typed since Coddy started,
// which the settings show; the program is given the keyring's as it starts.
func (plugin *Plugin) valuesWith(chosen map[string]any) map[string]any {
	values := make(map[string]any, len(plugin.Settings))
	for _, setting := range plugin.Settings {
		if setting.Type == SettingSecret {
			values[setting.ID], _ = plugin.Values[setting.ID].(string) // nil before any is typed: empty
			continue
		}
		values[setting.ID] = setting.valueOf(chosen[setting.ID])
	}
	return values
}

// settingOf is the plugin's setting of an id; found is false when it has none.
func (plugin *Plugin) settingOf(id string) (setting Setting, found bool) {
	index := slices.IndexFunc(plugin.Settings, func(each Setting) bool { return each.ID == id })
	if index < 0 {
		return Setting{}, false
	}
	return plugin.Settings[index], true
}

// keyring is where the plugin's secrets are kept: the system's, unless a test gave another.
func (plugin *Plugin) keyring() Keyring {
	if plugin.secrets == nil {
		return systemKeyring{}
	}
	return plugin.secrets
}

// secretAccount is the account of the keyring that keeps one of the plugin's secrets.
func (plugin *Plugin) secretAccount(id string) string { return plugin.Name + "/" + id }

// startingSettings are the values the plugin's program is given as it starts: its Values, each
// secret read from the keyring. It runs off the frame, as the program starts: a keyring may ask
// for its password, or be slow. A secret the keyring does not keep, or a computer without a
// keyring, where none could be kept, is empty.
func (plugin *Plugin) startingSettings() (map[string]any, error) {
	settings := make(map[string]any, len(plugin.Values))
	maps.Copy(settings, plugin.Values)
	for _, setting := range plugin.Settings {
		if setting.Type != SettingSecret {
			continue
		}
		secret, err := plugin.keyring().Read(secretsService, plugin.secretAccount(setting.ID))
		if err != nil && !errors.Is(err, keyring.NotFound) && !errors.Is(err, keyring.NotAvailable) {
			return nil, failure.Wrap(NotStarted, plugin.Name+": its secret "+setting.ID, err)
		}
		settings[setting.ID] = secret
	}
	return settings, nil
}

// Choose sets one of the plugin's settings and keeps it: a secret in the system's keyring, any
// other in its file, which only the person can read. Its program is stopped, so that its next use
// starts it with the new value. It runs on the frame goroutine, when the person changes a value,
// never at each frame.
func (plugin *Plugin) Choose(id string, value any) error {
	if plugin.Values[id] == value {
		return nil
	}
	if setting, _ := plugin.settingOf(id); setting.Type == SettingSecret {
		return plugin.chooseSecret(id, value)
	}
	chosen := maps.Clone(plugin.chosen)
	if chosen == nil {
		chosen = map[string]any{}
	}
	chosen[id] = value
	if err := writeSettings(plugin.settingsFile, chosen); err != nil {
		return err
	}
	values := plugin.valuesWith(chosen)
	plugin.lock.Lock()
	defer plugin.lock.Unlock()
	// a new map, not a change to the old: the program may be starting with the old one, off the frame
	plugin.chosen, plugin.Values = chosen, values
	plugin.stopLocked()
	return nil
}

// chooseSecret keeps one of the plugin's secrets in the keyring, or forgets it when it is empty.
// A computer without a keyring keeps no secret: it fails with keyring.NotAvailable, and there is
// no file to fall back to.
func (plugin *Plugin) chooseSecret(id string, value any) error {
	secret, _ := value.(string) // a secret is text; anything else is none
	account := plugin.secretAccount(id)
	var err error
	if secret == "" {
		err = plugin.keyring().Delete(secretsService, account)
	} else {
		err = plugin.keyring().Write(secretsService, account, secret)
	}
	if err != nil {
		return failure.Wrap(SettingsNotSaved, plugin.Name+": its secret "+id, err)
	}
	values := maps.Clone(plugin.Values)
	values[id] = secret
	plugin.lock.Lock()
	defer plugin.lock.Unlock()
	plugin.Values = values
	plugin.stopLocked()
	return nil
}

// writeSettings writes the values chosen for a plugin in its file: a new file only its owner
// can read, which takes the old one's place whole, so a value is never half written.
func writeSettings(path string, chosen map[string]any) error {
	if path == "" {
		return failure.Wrap(SettingsNotSaved, "a plugin without settings", nil)
	}
	content, err := json.MarshalIndent(chosen, "", "  ")
	if err != nil {
		return failure.Wrap(SettingsNotSaved, path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return failure.Wrap(SettingsNotSaved, path, err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*") // made with settingsFileMode
	if err != nil {
		return failure.Wrap(SettingsNotSaved, path, err)
	}
	_, err = file.Write(content)
	err = errors.Join(err, file.Chmod(settingsFileMode), file.Close())
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		_ = os.Remove(file.Name()) // the half-written copy; the old file is still whole
		return failure.Wrap(SettingsNotSaved, path, err)
	}
	return nil
}
