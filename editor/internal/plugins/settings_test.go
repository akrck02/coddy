package plugins

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/coddy/editor/internal/keyring"
	"github.com/akrck02/coddy/sdk"
)

// A plugin's settings are what the settings screen offers, so each must be one it can show.
func TestAManifestsSettingsAreReadAndTheBadOnesRefused(test *testing.T) {
	folder := test.TempDir()
	install(test, folder, "fine", `{"name": "fine", "command": ["fine"], "speaks": "coddy", "settings": [
		{"id": "url", "title": "Organisation URL", "description": "Where the projects are", "type": "text", "default": "https://example.com"},
		{"id": "token", "title": "Token", "type": "secret"},
		{"id": "drafts", "title": "Drafts", "type": "boolean", "default": true},
		{"id": "state", "title": "State", "type": "choice", "choices": ["open", "all"], "default": "all"}]}`)
	manifest, err := ReadManifest(filepath.Join(folder, "fine"))
	if err != nil || len(manifest.Settings) != 4 {
		test.Fatalf("the settings were read as %+v (%v)", manifest.Settings, err)
	}
	if url := manifest.Settings[0]; url.Title != "Organisation URL" || url.Description != "Where the projects are" || url.Default != "https://example.com" {
		test.Errorf("the first setting was read as %+v", url)
	}
	if state := manifest.Settings[3]; len(state.Choices) != 2 || state.Choices[1] != "all" {
		test.Errorf("the choice was read as %+v", state)
	}
	bad := map[string]string{
		"no-id":         `[{"title": "URL", "type": "text"}]`,
		"repeated-id":   `[{"id": "url", "type": "text"}, {"id": "url", "type": "secret"}]`,
		"unknown-type":  `[{"id": "url", "type": "colour"}]`,
		"empty-choices": `[{"id": "state", "type": "choice"}]`,
	}
	for name, settings := range bad {
		install(test, folder, name, `{"name": "`+name+`", "command": ["a"], "speaks": "coddy", "settings": `+settings+`}`)
		if _, err := ReadManifest(filepath.Join(folder, name)); !errors.Is(err, BadManifest) {
			test.Errorf("%s was read with %v, want plugins.bad-manifest", name, err)
		}
	}
}

// settingsPlugin is a plugin of a folder of plugins with one setting of each type, read by Find.
func settingsPlugin(test *testing.T, plugins string) *Plugin {
	test.Helper()
	install(test, plugins, "lamp", `{"name": "lamp", "command": ["lamp"], "speaks": "coddy", "settings": [
		{"id": "host", "type": "text", "default": "lamps.example.com"},
		{"id": "token", "type": "secret"},
		{"id": "dim", "type": "boolean"},
		{"id": "colour", "type": "choice", "choices": ["warm", "cold"]}]}`)
	found := Find(plugins, nil)
	if len(found) != 1 || found[0].Failed() {
		test.Fatalf("found %+v", found)
	}
	return found[0]
}

// fakeKeyring keeps secrets in memory, by service and account, as the system's keyring would;
// with unavailable set it is a computer without a keyring.
type fakeKeyring struct {
	secrets     map[string]string
	unavailable bool
}

func (fake *fakeKeyring) Read(service, account string) (string, error) {
	secret, found := fake.secrets[service+" "+account]
	switch {
	case fake.unavailable:
		return "", keyring.NotAvailable
	case !found:
		return "", keyring.NotFound
	}
	return secret, nil
}

func (fake *fakeKeyring) Write(service, account, secret string) error {
	if fake.unavailable {
		return keyring.NotAvailable
	}
	if fake.secrets == nil {
		fake.secrets = map[string]string{}
	}
	fake.secrets[service+" "+account] = secret
	return nil
}

func (fake *fakeKeyring) Delete(service, account string) error {
	if fake.unavailable {
		return keyring.NotAvailable
	}
	delete(fake.secrets, service+" "+account)
	return nil
}

// A value set is kept across starts, and one never set is the manifest's default; the file says
// which servers the person uses, so only its owner can read it.
func TestASettingsValuesAreKeptAndReadBackWithTheirDefaults(test *testing.T) {
	plugins := filepath.Join(test.TempDir(), "plugins")
	plugin := settingsPlugin(test, plugins)
	plugin.secrets = &fakeKeyring{}
	want := map[string]any{"host": "lamps.example.com", "token": "", "dim": false, "colour": "warm"}
	for id, value := range want {
		if plugin.Values[id] != value {
			test.Errorf("before any is set, %s is %v, want %v", id, plugin.Values[id], value)
		}
	}
	if err := plugin.Choose("dim", true); err != nil {
		test.Fatal(err)
	}
	if err := plugin.Choose("colour", "cold"); err != nil {
		test.Fatal(err)
	}
	read := settingsPlugin(test, plugins)
	if read.Values["dim"] != true || read.Values["colour"] != "cold" || read.Values["host"] != "lamps.example.com" {
		test.Errorf("read back, the values are %v", read.Values)
	}
	file := filepath.Join(filepath.Dir(plugins), settingsFolderName, "lamp.json")
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != settingsFileMode {
		test.Errorf("the settings file is %v (%v), want mode %o", info, err, settingsFileMode)
	}
}

// A file can be copied, backed up or read by another program of the person's; a secret goes to
// the keyring instead, under Coddy's service and the plugin's account.
func TestASecretIsKeptInTheKeyringAndNeverInTheSettingsFile(test *testing.T) {
	plugins := filepath.Join(test.TempDir(), "plugins")
	plugin := settingsPlugin(test, plugins)
	keeping := &fakeKeyring{}
	plugin.secrets = keeping
	if err := plugin.Choose("dim", true); err != nil {
		test.Fatal(err)
	}
	if err := plugin.Choose("token", "lamp oil"); err != nil {
		test.Fatal(err)
	}
	if keeping.secrets["coddy lamp/token"] != "lamp oil" {
		test.Errorf("the keyring keeps %v", keeping.secrets)
	}
	if plugin.Values["token"] != "lamp oil" || plugin.Values["dim"] != true {
		test.Errorf("the values shown are %v", plugin.Values)
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(plugins), settingsFolderName, "lamp.json"))
	if err != nil || strings.Contains(string(content), "lamp oil") || strings.Contains(string(content), "token") {
		test.Errorf("the settings file holds %s (%v)", content, err)
	}
	if read := settingsPlugin(test, plugins); read.Values["token"] != "" {
		test.Errorf("read back, the secret is %v, want it read only from the keyring", read.Values["token"])
	}
	if err := plugin.Choose("token", ""); err != nil || len(keeping.secrets) != 0 {
		test.Errorf("an emptied secret left %v in the keyring (%v)", keeping.secrets, err)
	}
}

// There is no file to fall back to: a secret written in one would be read by anyone who has it.
func TestASecretOnAComputerWithoutAKeyringIsNotSaved(test *testing.T) {
	plugins := filepath.Join(test.TempDir(), "plugins")
	plugin := settingsPlugin(test, plugins)
	plugin.secrets = &fakeKeyring{unavailable: true}
	if err := plugin.Choose("token", "lamp oil"); !errors.Is(err, keyring.NotAvailable) {
		test.Errorf("saving a secret without a keyring gave %v, want keyring.not-available", err)
	}
	if plugin.Values["token"] != "" {
		test.Errorf("the secret not saved is shown as %v", plugin.Values["token"])
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(plugins), settingsFolderName, "lamp.json")); !errors.Is(err, fs.ErrNotExist) {
		test.Errorf("a settings file was written: %v", err)
	}
}

// The secret is read as the program starts, off the frame, and given to it with the others.
func TestASecretReachesThePluginFromTheKeyringAsItStarts(test *testing.T) {
	plugin := fake(test)
	plugin.Settings = []Setting{{ID: "greeting", Type: SettingSecret}}
	plugin.secrets = &fakeKeyring{secrets: map[string]string{"coddy fake/greeting": "psst"}}
	if err := plugin.readSettings(test.TempDir()); err != nil {
		test.Fatal(err)
	}
	greeted, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "greeting"})
	if err != nil || greeted.Note != "psst" {
		test.Errorf("the plugin was given %+v (%v), want the keyring's secret", greeted, err)
	}
}

// The program reads its settings only as it starts, so a change must start it again.
func TestAChangedValueReachesThePluginAtItsNextStart(test *testing.T) {
	plugin := fake(test)
	plugin.Settings = []Setting{{ID: "greeting", Type: SettingText, Default: "hello"}}
	if err := plugin.readSettings(test.TempDir()); err != nil {
		test.Fatal(err)
	}
	greeted, err := plugin.Run(context.Background(), sdk.ActionParams{Action: "greeting"})
	if err != nil || greeted.Note != "hello" {
		test.Fatalf("before a change, the plugin greeted %+v (%v)", greeted, err)
	}
	if err := plugin.Choose("greeting", "good evening"); err != nil {
		test.Fatal(err)
	}
	greeted, err = plugin.Run(context.Background(), sdk.ActionParams{Action: "greeting"})
	if err != nil || greeted.Note != "good evening" {
		test.Errorf("after a change, the plugin greeted %+v (%v)", greeted, err)
	}
}
