package lsp

import (
	"os"
	"path/filepath"
	"strings"
)

// Server is a language server the reader knows how to run. The servers come from the plugins
// on, each declared in its plugin.json (the plugins package).
type Server struct {
	Name    string // as the reader shows it: "gopls"
	Command string
	// Arguments go on the command line; DataFlag, when set, is followed by a folder of the
	// reader's own where the server keeps its index (jdtls's -data).
	Arguments []string
	DataFlag  string
	Languages []string // the protocol's names of the languages it reads
	// OnePerWindow runs one server for every project of the window, each a workspace folder
	// (jdtls); otherwise one runs per root, the folder holding one of RootMarkers (gopls).
	OnePerWindow bool
	RootMarkers  []string
	Install      string   // what to run to install it, said when it is missing
	Environment  []string // added to the reader's own, as "NAME=value"
	// InstallEnvironment is added to the reader's own environment while Install runs
	InstallEnvironment []string
	// Options are the server's own initializationOptions, as it documents them.
	Options map[string]any
	// Settings are the server's settings, by section as workspace/configuration asks them
	// ({"java": {"autobuild": …}}): given at initialize and to each question for a section.
	Settings map[string]any
	// FoldersInOptions gives the workspace folders in initializationOptions too: jdtls reads them
	// there, and from the parameters only the first.
	FoldersInOptions bool
	// ReadsAll reads every project of the window with a file of its languages, not only those in
	// sight: a server whose index is per set of projects (Index) would build another, cold, each
	// time the filter changed what is in sight.
	ReadsAll bool
	Ready    Readiness
	Memory   Memory
	Index    Index
	Import   Import
}

// Readiness is when a server has read the projects and answers. Without either field, a server is
// ready once initialized.
type Readiness struct {
	// Status, when set, is the language/status type the server sends once it has read the
	// projects (jdtls's ServiceReady); until then requests wait.
	Status string
	// AfterWork makes a server without Status ready once the first work it reports ends
	// (kotlin-lsp, whose import begins seconds after it is initialized, and which answers with
	// nothing meanwhile), or after workGrace if it reports none.
	AfterWork bool
}

// Memory is what a server's JVM may take, and how it is told.
type Memory struct {
	// Most is the most memory the server's JVM may take ("2g"), given with Flag on its command
	// line, or in Variable, an environment variable of JVM options.
	Most     string
	Flag     string
	Variable string
	// JVMOptions are more options of the server's JVM, given in Variable after its memory.
	JVMOptions []string
}

// Index is where a server keeps its index in its data folder, and how much of it the reader keeps.
type Index struct {
	// Folder, when set, is the folder of its data folder where the server builds one index per
	// set of workspace folders (kotlin-lsp's workspaces); the reader keeps the Keep used last and
	// removes the others before it starts the server.
	Folder string
	Keep   int
	// Option, when set, is the initialization option that names where the server keeps its
	// index: the reader gives it a folder of Folder named after the set of folders it starts with
	// (indexFolder). kotlin-lsp 271 keeps them in the person's caches otherwise
	// (~/Library/Caches/JetBrains/IntelliJServer), whatever --system-path says, where the reader
	// neither keeps one nor removes the old.
	Option string
	// Retired are folders of its data folder where an older reader kept its indexes, removed as
	// old indexes are.
	Retired []string
	// NeedsFree is how many bytes must be free on its data folder's disk for it to start: an index
	// that fills the disk breaks the server, and whatever else writes there.
	NeedsFree uint64
}

// Import is how a server that imports Gradle builds is helped with them (the gradle package).
type Import struct {
	// Models keeps, for each project the server imports with Gradle, the model it imported, and
	// starts it with that model instead while the project's build files do not change: kotlin-lsp's
	// json importer and its exportWorkspace (models.go).
	Models bool
	// OwnGradle gives the server a Gradle home of its own, in its data folder, whose daemons stop
	// soon after it and with it (gradle.go).
	OwnGradle bool
	// After are the servers whose import of the projects it waits for before it starts: two
	// imports at once, each with its Gradle daemons, take more memory than a laptop has.
	After []string
}

// setting is the value of a section of settings, "java" or "java.import", if there is one.
func (server Server) setting(section string) any {
	var found any = server.Settings
	for _, name := range strings.Split(section, ".") {
		level, nested := found.(map[string]any)
		if !nested {
			return nil
		}
		found = level[name]
	}
	return found
}

// ServerFor is the server of a language among those given, if one reads it.
func ServerFor(servers []Server, language string) (Server, bool) {
	for _, server := range servers {
		for _, each := range server.Languages {
			if each == language {
				return server, true
			}
		}
	}
	return Server{}, false
}

// RootOf is the root a server runs in for a file: the nearest folder above it, inside the project,
// that holds one of the server's root markers; the project itself when none does.
func (server Server) RootOf(project, file string) string {
	if server.OnePerWindow {
		return ""
	}
	for folder := filepath.Dir(file); ; folder = filepath.Dir(folder) {
		for _, marker := range server.RootMarkers {
			if _, err := os.Stat(filepath.Join(folder, marker)); err == nil {
				return folder
			}
		}
		if rel, err := filepath.Rel(project, folder); err != nil || rel == "." || filepath.Dir(folder) == folder {
			return project
		}
	}
}
