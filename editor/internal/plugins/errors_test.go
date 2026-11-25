package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akrck02/coddy/sdk"
)

func TestEachErrorCarriesItsCode(test *testing.T) {
	_, badManifest := ReadManifest(test.TempDir())
	missing := &Plugin{Manifest: Manifest{Name: "missing", Command: []string{"no-such-plugin-program"}}, Folder: test.TempDir()}
	_, notStarted := missing.Run(context.Background(), sdk.ActionParams{Action: "note"})
	_, otherVersion := fake(test, "other-version").Run(context.Background(), sdk.ActionParams{Action: "note"})
	soon, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, timeout := fake(test).Run(soon, sdk.ActionParams{Action: "slow"})
	_, crashed := fake(test).Run(context.Background(), sdk.ActionParams{Action: "crash"})
	settingsFolder := test.TempDir()
	if err := os.WriteFile(filepath.Join(settingsFolder, "lamp.json"), []byte("{"), 0o600); err != nil {
		test.Fatal(err)
	}
	lamp := &Plugin{Manifest: Manifest{Name: "lamp", Settings: []Setting{{ID: "dim", Type: SettingBoolean}}}}
	badSettings := lamp.readSettings(settingsFolder)
	lamp.settingsFile = filepath.Join(settingsFolder, "lamp.json", "inside a file")
	notSaved := lamp.Choose("dim", true)
	cases := []struct {
		name string
		err  error
		code error
	}{
		{"a folder without plugin.json", badManifest, BadManifest},
		{"a program that is not there", notStarted, NotStarted},
		{"a plugin of another version", otherVersion, ProtocolVersion},
		{"an action that takes too long", timeout, Timeout},
		{"a program that ends while it runs an action", crashed, Crashed},
		{"a settings file that is not JSON", badSettings, BadSettings},
		{"a settings file that cannot be written", notSaved, SettingsNotSaved},
	}
	for _, each := range cases {
		if !errors.Is(each.err, each.code) {
			test.Errorf("%s gave %v, want %v", each.name, each.err, each.code)
		}
	}
}
