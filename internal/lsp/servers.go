package lsp

import (
	"os"
	"path/filepath"
	"strings"
)

// Server is a language server the reader knows how to run.
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
	// ReadyStatus, when set, is the language/status type the server sends once it has read the
	// projects (jdtls's ServiceReady); until then requests wait. Without it, a server is ready
	// once initialized.
	ReadyStatus string
	// Options are the server's own initializationOptions, as it documents them.
	Options map[string]any
	// Settings are the server's settings, by section as workspace/configuration asks them
	// ({"java": {"autobuild": …}}): given at initialize and to each question for a section.
	Settings map[string]any
	// Memory is the most memory the server's JVM may take ("2g"), given with MemoryFlag on its
	// command line, or in MemoryVariable, an environment variable of JVM options.
	Memory         string
	MemoryFlag     string
	MemoryVariable string
	// Indexes, when set, is the folder of its data folder where the server builds one index per
	// set of workspace folders (kotlin-lsp's workspaces); the reader keeps the KeepIndexes used
	// last and removes the others before it starts the server.
	Indexes     string
	KeepIndexes int
	// OwnGradle gives a server that imports Gradle builds a Gradle home of its own, in its data
	// folder, whose daemons stop soon after it and with it (gradle.go).
	OwnGradle bool
	// FoldersInOptions gives the workspace folders in initializationOptions too: jdtls reads them
	// there, and from the parameters only the first.
	FoldersInOptions bool
	// ReadsAll reads every project of the window with a file of its languages, not only those in
	// sight: a server whose index is per set of projects (Indexes) would build another, cold, each
	// time the filter changed what is in sight.
	ReadsAll bool
	// NeedsFree is how many bytes must be free on its data folder's disk for it to start: an index
	// that fills the disk breaks the server, and whatever else writes there.
	NeedsFree uint64
}

// Servers are the language servers the reader runs, found on the PATH.
var Servers = []Server{
	{Name: "gopls", Command: "gopls", Languages: []string{"go"}, RootMarkers: []string{"go.mod"},
		Install: "go install golang.org/x/tools/gopls@latest"},
	// jdtls reads Java only: Kotlin has kotlin-lsp, below. It writes nothing in the repositories: .project, .classpath and .settings go in its data
	// folder (a JVM property, read before the setting is), and no build fills bin/ or target/; a
	// build file changed is not imported again on its own.
	{Name: "jdtls", Command: "jdtls", Languages: []string{"java"}, OnePerWindow: true, DataFlag: "-data", FoldersInOptions: true,
		Arguments: []string{"--jvm-arg=-Djava.import.generatesMetadataFilesAtProjectRoot=false"},
		Install:   "brew install jdtls", ReadyStatus: "ServiceReady",
		// a name declared in a jar (another repository, as a dependency) answers with a
		// jdt:// location, which the reader maps to that repository's source when it is open
		Options: map[string]any{"extendedClientCapabilities": map[string]any{"classFileContentsSupport": true}},
		Settings: map[string]any{"java": map[string]any{
			"import":        map[string]any{"generatesMetadataFilesAtProjectRoot": false},
			"autobuild":     map[string]any{"enabled": false},
			"configuration": map[string]any{"updateBuildConfiguration": "disabled"},
		}},
		// its Gradle import (Buildship) honours GRADLE_USER_HOME: its daemons time out, as kotlin-lsp's
		Memory: "2g", MemoryFlag: "--jvm-arg=-Xmx", OwnGradle: true},
	// JetBrains' server, still a preview. It imports each Gradle build first (minutes, cold) and
	// answers with nothing meanwhile, telling its work by $/progress; a name in a jar answers
	// jar:///…!/package/Class.class. Its index goes in the system path; Gradle's import writes
	// .gradle/ and build/ in the project, as a build does. It follows workspace folders added and
	// removed, keeping the index of the set it started with.
	{Name: "kotlin-lsp", Command: "kotlin-lsp", Arguments: []string{"--stdio"}, DataFlag: "--system-path",
		Languages: []string{"kotlin"}, OnePerWindow: true, Install: "brew install JetBrains/utils/kotlin-lsp",
		// an index per set of projects it starts with, about 5 GB for 44 repositories': it reads them all,
		// whatever is in sight, so one index serves, and the reader keeps the one used last
		Indexes: "workspaces", KeepIndexes: 1, ReadsAll: true, OwnGradle: true, NeedsFree: 10 << 30,
		Memory: "2g", MemoryVariable: "IJ_JAVA_OPTIONS"},
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
