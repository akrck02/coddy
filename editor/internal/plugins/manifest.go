// Package plugins finds the plugins a person installed, runs their programs and asks them to run
// their actions, over the protocol of the pluginsdk package. What a plugin adds is drawn by the
// window: this package knows nothing of it.
package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/akrck02/coddy/editor/internal/lsp"
	"github.com/akrck02/coddy/editor/internal/programs"
	"github.com/akrck02/go-renderer/failure"
)

// manifestName is the file in a plugin's folder that says what it adds.
const manifestName = "plugin.json"

// Manifest is what a plugin's plugin.json says. Fields it does not know are ignored.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	// Author is who made the plugin, shown on its card in the settings: "Coddy" for its own.
	Author string `json:"author"`
	// Icon stands for the plugin in the settings: one of Lucide's names ("coffee"), or an SVG or
	// PNG file of the plugin's folder ("icon.svg"). Empty draws its initial on a colour of its own.
	Icon string `json:"icon"`
	// Command runs the plugin's program, its first word a file of the plugin's folder or else a
	// program on the PATH. Every plugin has one: what it adds is done by its program.
	Command []string `json:"command"`
	// Speaks is the protocol its program speaks: SpeaksLanguageServer or SpeaksCoddy.
	Speaks string `json:"speaks"`
	// Requires are the programs the plugin needs on the computer, its own among them when it is
	// not in its folder (a language server), each with how to install it.
	Requires []Requirement `json:"requires"`
	// Settings are what a person can set for the plugin, in its settings: the plugin's program
	// is given their values when it starts.
	Settings    []Setting     `json:"settings"`
	Contributes Contributions `json:"contributes"`
}

// Requirement is a program a plugin needs, and how to install it on each system.
type Requirement struct {
	// Program is the program's name on the PATH: "pandoc".
	Program string `json:"program"`
	// Install are the commands that install it, by system as Go names them ("linux", "darwin",
	// "windows"). Coddy offers the first whose installer is on the computer, so one list serves
	// several Linux distributions: ["sudo dnf install pandoc", "sudo apt install pandoc"].
	Install map[string][]string `json:"install"`
	// Environment is added to Coddy's own while the install runs: ["GOTOOLCHAIN=auto"].
	Environment []string `json:"environment"`
}

// InstallCommand is the command that installs the program on this computer, "" when the plugin
// gives none for its system.
func (requirement Requirement) InstallCommand() string {
	return programs.Choose(requirement.Install[runtime.GOOS])
}

// The protocols a plugin's program may speak. Both go over the same connection; they differ in
// who knows what the program can do.
const (
	// SpeaksLanguageServer is the Language Server Protocol: the program is a language server
	// written for any editor, and what it can do is the protocol's own, known to Coddy already.
	// The plugin contributes only its languageServer.
	SpeaksLanguageServer = "lsp"
	// SpeaksCoddy is Coddy's plugin protocol (the pluginsdk package): the program is written for
	// Coddy, and the plugin contributes what it does.
	SpeaksCoddy = "coddy"
)

// Contributions are what a plugin adds to Coddy.
type Contributions struct {
	// LanguageServer, for a plugin that speaks the Language Server Protocol, is how Coddy runs it.
	LanguageServer *LanguageServer `json:"languageServer"`
	Actions        []Action        `json:"actions"`
	// VersionControl, when set, reads the repositories that hold its marker.
	VersionControl *VersionControlContribution `json:"versionControl"`
	// ReviewSource, when set, lists the pull requests of a project and their comments.
	ReviewSource *ReviewSourceContribution `json:"reviewSource"`
	// Mode, when set, is a mode of the plugin's own on the rail, its screen drawn by the plugin.
	Mode *ModeContribution `json:"mode"`
}

// LanguageServer is how Coddy runs a plugin whose program is a language server. Only languages
// are needed; the other fields tune a server that needs them, as Coddy's official plugins for
// Java and Kotlin do.
type LanguageServer struct {
	// Languages are the protocol's names of the languages it reads: ["python"].
	Languages []string `json:"languages"`
	// RootMarkers are files whose folder is a project's root for the server: ["pyproject.toml"].
	RootMarkers []string `json:"rootMarkers"`
	// DataFlag, when set, goes on the command line followed by a folder of Coddy's own where the
	// server keeps its data (jdtls's "-data").
	DataFlag string `json:"dataFlag"`
	// OnePerWindow runs one server for every project of the window, each a workspace folder,
	// instead of one per root.
	OnePerWindow bool `json:"onePerWindow"`
	// ReadsAll reads every project of the window with a file of its languages, not only those in
	// sight.
	ReadsAll bool `json:"readsAll"`
	// InitializationOptions are the server's own, as it documents them, and FoldersInOptions
	// gives the workspace folders there too (jdtls reads only the first otherwise).
	InitializationOptions map[string]any `json:"initializationOptions"`
	FoldersInOptions      bool           `json:"foldersInOptions"`
	// Settings are the server's settings by section, as workspace/configuration asks for them.
	Settings map[string]any `json:"settings"`
	Ready    ServerReady    `json:"ready"`
	Memory   ServerMemory   `json:"memory"`
	Index    ServerIndex    `json:"index"`
	Gradle   ServerGradle   `json:"gradle"`
}

// ServerReady is when a server has read the projects and answers; without either field, once it
// is initialized.
type ServerReady struct {
	// Status is the language/status type it sends once ready ("ServiceReady").
	Status string `json:"status"`
	// AfterWork makes it ready once the first work it reports ends.
	AfterWork bool `json:"afterWork"`
}

// ServerMemory is the most memory a server's JVM may take ("2g"), given after Flag on its command
// line ("--jvm-arg=-Xmx") or in Variable, an environment variable of JVM options.
type ServerMemory struct {
	Most     string `json:"most"`
	Flag     string `json:"flag"`
	Variable string `json:"variable"`
}

// ServerIndex is where a server that builds one index per set of projects keeps them, in its data
// folder, and how many Coddy keeps.
type ServerIndex struct {
	Folder string `json:"folder"`
	Keep   int    `json:"keep"`
	// Option is the initialization option that tells the server where to keep its index.
	Option string `json:"option"`
	// Retired are folders where an older Coddy kept indexes, removed as old indexes are.
	Retired []string `json:"retired"`
	// NeedsFreeGigabytes must be free on the disk of its data folder for it to start.
	NeedsFreeGigabytes uint64 `json:"needsFreeGigabytes"`
}

// ServerGradle is how Coddy helps a server that imports Gradle builds.
type ServerGradle struct {
	// KeepModels keeps the model of each project imported, to start with it while the build files
	// do not change.
	KeepModels bool `json:"keepModels"`
	// OwnHome gives the server a Gradle home of its own, whose daemons stop with it.
	OwnHome bool `json:"ownHome"`
	// After are the servers whose import the server waits for before it starts.
	After []string `json:"after"`
}

// bytesPerGigabyte turns a manifest's gigabytes into the bytes Coddy counts
const bytesPerGigabyte = 1 << 30

// Action is something a plugin does with the file open, listed in Coddy's menu.
type Action struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// TODO(akrck02): an action could say where it applies (a file, a selection, a project) and the
// keys that run it; every action applies to the file open, from the menu, until a plugin needs more.

// ReadManifest reads the plugin.json of a plugin's folder.
func ReadManifest(folder string) (Manifest, error) {
	path := filepath.Join(folder, manifestName)
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, failure.Wrap(BadManifest, path, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return Manifest{}, failure.Wrap(BadManifest, path, err)
	}
	if problem := manifest.check(); problem != "" {
		return Manifest{}, failure.Wrap(BadManifest, path+": "+problem, nil)
	}
	return manifest, nil
}

// check tells what is wrong in a manifest read, empty when nothing is: every plugin runs a program, in one of the
// protocols, and contributes what that protocol can carry.
func (manifest Manifest) check() (problem string) {
	added := manifest.Contributes
	switch {
	case manifest.Name == "":
		return "no name"
	case len(manifest.Command) == 0:
		return "no command to run its program"
	case manifest.Speaks == SpeaksLanguageServer && (added.LanguageServer == nil || len(added.LanguageServer.Languages) == 0):
		return "a language server needs the languages it reads"
	case manifest.Speaks == SpeaksLanguageServer && (len(added.Actions) > 0 || added.VersionControl != nil ||
		added.ReviewSource != nil || added.Mode != nil):
		return "a language server contributes only its languageServer"
	case manifest.Speaks == SpeaksCoddy && added.LanguageServer != nil:
		return "a languageServer needs a program that speaks lsp"
	case manifest.Speaks != SpeaksLanguageServer && manifest.Speaks != SpeaksCoddy:
		return `speaks is "lsp" or "coddy"`
	case added.VersionControl != nil && added.VersionControl.Marker == "":
		return "a version control needs a marker"
	case added.Mode != nil && added.Mode.Name == "":
		return "a mode needs a name"
	case isIconFile(manifest.Icon) && !filepath.IsLocal(manifest.Icon):
		return "an icon file must be inside the plugin's folder"
	case slices.ContainsFunc(manifest.Requires, func(requirement Requirement) bool { return requirement.Program == "" }):
		return "a requirement needs its program"
	case len(manifest.Settings) > 0 && (!filepath.IsLocal(manifest.Name) || filepath.Base(manifest.Name) != manifest.Name):
		return "a plugin with settings needs a name that can name a file"
	}
	return settingsProblem(manifest.Settings)
}

// Server is the plugin's program as Coddy's fleet of language servers runs it, named as the
// program; found is false for a plugin that does not speak the Language Server Protocol.
func (manifest Manifest) Server() (server lsp.Server, found bool) {
	declared := manifest.Contributes.LanguageServer
	if manifest.Speaks != SpeaksLanguageServer || declared == nil || len(manifest.Command) == 0 {
		return lsp.Server{}, false
	}
	server = declared.server(manifest.Command)
	for _, requirement := range manifest.Requires {
		if requirement.Program == server.Name {
			server.Install, server.InstallEnvironment = requirement.InstallCommand(), requirement.Environment
		}
	}
	return server, true
}

// server is the language server as Coddy's fleet runs it, with the command that starts it.
func (declared LanguageServer) server(command []string) lsp.Server {
	return lsp.Server{
		Name: filepath.Base(command[0]), Command: command[0], Arguments: slices.Clone(command[1:]),
		Languages: declared.Languages, RootMarkers: declared.RootMarkers,
		DataFlag: declared.DataFlag, OnePerWindow: declared.OnePerWindow, ReadsAll: declared.ReadsAll,
		Options: declared.InitializationOptions, FoldersInOptions: declared.FoldersInOptions,
		Settings: declared.Settings,
		Ready:    lsp.Readiness{Status: declared.Ready.Status, AfterWork: declared.Ready.AfterWork},
		Memory:   lsp.Memory{Most: declared.Memory.Most, Flag: declared.Memory.Flag, Variable: declared.Memory.Variable},
		Index: lsp.Index{Folder: declared.Index.Folder, Keep: declared.Index.Keep, Option: declared.Index.Option,
			Retired: declared.Index.Retired, NeedsFree: declared.Index.NeedsFreeGigabytes * bytesPerGigabyte},
		Import: lsp.Import{Models: declared.Gradle.KeepModels, OwnGradle: declared.Gradle.OwnHome, After: declared.Gradle.After},
	}
}

// Find reads every plugin installed in a folder, one folder each, in the order of their folders;
// each is on unless named in off. A plugin whose manifest cannot be read is listed failed, named
// by its folder. The values set for their settings are read from plugin-settings/, beside the
// folder: <name>.json for each plugin.
func Find(folder string, off []string) []*Plugin {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil // no plugins folder: no plugin installed
	}
	var found []*Plugin
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pluginFolder := filepath.Join(folder, entry.Name())
		manifest, err := ReadManifest(pluginFolder)
		if err != nil {
			manifest.Name = entry.Name()
		}
		plugin := &Plugin{Manifest: manifest, Folder: pluginFolder, On: !slices.Contains(off, manifest.Name),
			Badge: badgeOf(manifest.Name, manifest.Icon, pluginFolder)}
		if err == nil {
			err = plugin.readSettings(settingsFolderOf(folder))
		}
		plugin.Tell(err)
		plugin.CheckRequirements()
		plugin.Verified = isVerified(plugin)
		found = append(found, plugin)
	}
	return found
}

// DefaultFolder is where plugins are installed: plugins/ in Coddy's own folder.
func DefaultFolder() string {
	config, err := os.UserConfigDir()
	if err != nil {
		return "" // no folder of the person's settings: no plugins are read
	}
	return filepath.Join(config, "coddy", "plugins")
}
