package plugins

import (
	"strings"

	"github.com/akrck02/coddy/editor/internal/execution/programs"
)

// Missing is a program a plugin requires that is not on this computer, with how to install it.
type Missing struct {
	Requirement
	// Command installs it here, "" when the plugin gives none for this system.
	Command string
	// Runnable tells whether Coddy can run Command itself: its installer is here, and it asks
	// for no password. Otherwise the person runs it in a terminal.
	Runnable bool
}

// CheckRequirements looks for the programs the plugin requires, and keeps those missing. It runs
// when the plugin is read and after an install, never in a frame: each looks through the PATH.
func (plugin *Plugin) CheckRequirements() {
	plugin.Missing = plugin.Missing[:0]
	plugin.Installable = false
	for _, requirement := range plugin.Requires {
		if _, err := programs.Find(requirement.Program); err == nil {
			continue
		}
		command := requirement.InstallCommand()
		missing := Missing{Requirement: requirement, Command: command, Runnable: programs.Runnable(command)}
		plugin.Missing = append(plugin.Missing, missing)
		plugin.Installable = plugin.Installable || missing.Runnable
	}
	plugin.Warning = plugin.warningWords()
}

// warningWords are what the settings warn of a plugin, made when its state changes: its last
// error, and each program it misses with how to get it; "" when all is well.
func (plugin *Plugin) warningWords() string {
	var warnings []string
	if plugin.Failed() {
		warnings = append(warnings, plugin.State)
	}
	for _, missing := range plugin.Missing {
		switch {
		case missing.Runnable:
			warnings = append(warnings, missing.Program+" is not installed: the download button installs it")
		case missing.Command != "":
			warnings = append(warnings, missing.Program+" is not installed: click to see how to install it")
		default:
			warnings = append(warnings, missing.Program+" is not installed, and the plugin says not how to install it here")
		}
	}
	return strings.Join(warnings, " · ")
}

// Ready tells whether the plugin works now: it is on, and nothing it requires is missing. The
// settings draw the others greyed, as resting.
func (plugin *Plugin) Ready() bool { return plugin.On && len(plugin.Missing) == 0 }
