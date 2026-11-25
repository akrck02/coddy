package window

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/akrck02/coddy/editor/internal/keyring"
	"github.com/akrck02/coddy/editor/internal/mode"
	"github.com/akrck02/coddy/editor/internal/pluginmode"
	"github.com/akrck02/coddy/editor/internal/plugins"
	"github.com/akrck02/coddy/editor/internal/programs"
	"github.com/akrck02/coddy/editor/internal/session"
	"github.com/akrck02/coddy/sdk"
	ui "github.com/akrck02/go-ui"
	platform "github.com/akrck02/go-ui/window"
)

// pluginWork is the key of the plugins' actions in work: one runs at a time, and a new one
// replaces the one still running.
const pluginWork = "plugin action"

// made is a file an action made, which the menu offers to open.
type made struct {
	path  string
	label string // the menu item, made once: "Open lamp.pdf"
}

// Plugins are the plugins installed, on or off, for the settings and to remember those on.
func (application *Window) Plugins() []*plugins.Plugin { return application.settings.Plugins }

// OpenPluginsFolder shows the folder plugins are installed in, in the computer's file manager,
// making it first when no plugin was installed yet.
func (application *Window) OpenPluginsFolder() {
	folder := application.settings.PluginsFolder
	if err := plugins.MakeFolder(folder); err != nil {
		application.toast = "The plugins folder could not be made: " + err.Error()
		return
	}
	if !platform.OpenLink(folder) {
		application.toast = "No program here opens " + folder
	}
}

// TurnPlugin turns a plugin on or off. Its language servers join or leave at the next start.
func (application *Window) TurnPlugin(plugin *plugins.Plugin, on bool) { plugin.Turn(on) }

// noKeyringNote is what the toast says when a secret cannot be saved for want of a keyring: a
// thing to install, not a failure of the person's.
const noKeyringNote = "Secrets are kept only in the system's keyring, and there is none here to keep this one: " +
	"on Linux, install secret-tool (libsecret)."

// ChoosePluginSetting sets one of a plugin's settings and saves it; its program starts again with
// it at its next use. What failed is told in a toast too.
func (application *Window) ChoosePluginSetting(plugin *plugins.Plugin, id string, value any) error {
	err := plugin.Choose(id, value)
	switch {
	case errors.Is(err, keyring.NotAvailable):
		application.toast = noKeyringNote
	case err != nil:
		application.toast = err.Error()
	}
	return err
}

// Restart closes the window, as closing it would, keeping what it keeps, and asks the program to
// start again: plugins turned on or off take part from the start.
func (application *Window) Restart() {
	application.restartAsked = true
	platform.Quit()
}

// RestartAsked tells whether the window was closed to start again, for the program to do so.
func (application *Window) RestartAsked() bool { return application.restartAsked }

// installWait is how long a program's install may take: go install builds it from its source
const installWait = 10 * time.Minute

// InstallMissing installs, in the background and one after another, the programs a plugin misses
// that Coddy can install itself, then looks again for what it misses and says how it went.
func (application *Window) InstallMissing(plugin *plugins.Plugin) {
	if plugin.Installing {
		return
	}
	var runnable []plugins.Missing // a copy: the frame goroutine changes the plugin's own list
	for _, missing := range plugin.Missing {
		if missing.Runnable {
			runnable = append(runnable, missing)
		}
	}
	plugin.Installing = true
	application.work.Run("install for "+plugin.Name, func(stop context.Context) func() {
		installing, cancel := context.WithTimeout(stop, installWait)
		defer cancel()
		err := installEach(installing, runnable)
		return func() {
			plugin.Installing = false
			plugin.CheckRequirements()
			if err != nil {
				application.toast = err.Error()
				return
			}
			application.toast = "Installed what " + plugin.Name + " needs"
		}
	})
}

// installEach runs the install of each program, stopping at the first that fails.
func installEach(stop context.Context, missing []plugins.Missing) error {
	for _, each := range missing {
		if err := programs.Install(stop, each.Command, each.Environment); err != nil {
			return err
		}
	}
	return nil
}

// markedVersionControls are the version controls of the plugins on, each with its marker; git,
// built in, reads its repositories itself. A plugin turned on later reads the folders opened
// after it.
func (application *Window) markedVersionControls() []session.Marked {
	var marked []session.Marked
	for _, plugin := range application.settings.Plugins {
		if control := plugin.Contributes.VersionControl; plugin.Ready() && !plugin.BuiltIn && control != nil {
			marked = append(marked, session.Marked{Marker: control.Marker, Control: plugin.VersionControl()})
		}
	}
	return marked
}

// pluginModes are the modes of the plugins on, for the rail. A plugin turned on later adds its
// mode at the next start, as its language servers.
func pluginModes(installed []*plugins.Plugin) []mode.Mode {
	var modes []mode.Mode
	for _, plugin := range installed {
		if plugin.Ready() && plugin.Contributes.Mode != nil {
			modes = append(modes, pluginmode.New(plugin))
		}
	}
	return modes
}

// stopPlugins ends every plugin's program, as the window closes.
func (application *Window) stopPlugins() {
	for _, plugin := range application.settings.Plugins {
		plugin.Stop()
	}
}

// buildPluginItems are the menu's items of the plugins on: each action, for the file open, and the
// file an action made last, to open.
func (application *Window) buildPluginItems(context *ui.Context) {
	if application.viewer.IsOpen() {
		for _, plugin := range application.settings.Plugins {
			if !plugin.Ready() {
				continue
			}
			for _, action := range plugin.Contributes.Actions {
				if context.MenuItem("puzzle", action.Title, "") {
					application.runAction(plugin, action.ID)
				}
			}
		}
	}
	if application.made.path != "" && context.MenuItem("external-link", application.made.label, "") {
		if !platform.OpenLink(application.made.path) {
			application.toast = "No program here opens " + filepath.Base(application.made.path)
		}
	}
}

// runAction asks a plugin to run an action on the file open, off the frame, and shows its answer.
func (application *Window) runAction(plugin *plugins.Plugin, action string) {
	reference := application.viewer.Reference
	if reference.Project < 0 || reference.Project >= len(application.session.Projects) {
		return
	}
	project := application.session.Projects[reference.Project]
	params := sdk.ActionParams{Action: action, File: filepath.Join(project.Path, reference.Path)}
	application.work.Run(pluginWork, func(stop context.Context) func() {
		result, err := plugin.Run(stop, params)
		return func() { application.showActionResult(plugin, result, err) }
	})
}

// showActionResult tells what an action did: its error with its code, its note, or the file it
// made, which the menu then offers to open.
func (application *Window) showActionResult(plugin *plugins.Plugin, result sdk.ActionResult, err error) {
	plugin.Tell(err)
	if err != nil {
		application.toast = err.Error()
		return
	}
	if result.Created != "" {
		name := filepath.Base(result.Created)
		application.made = made{path: result.Created, label: "Open " + name}
		application.toast = "Made " + name + ": open it from the menu"
	}
	if result.Note != "" {
		application.toast = result.Note
	}
}
