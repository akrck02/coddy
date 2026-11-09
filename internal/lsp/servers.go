package lsp

import (
	"os"
	"path/filepath"
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
	// Indexes, when set, is the folder of its data folder where the server builds one index per
	// set of workspace folders (kotlin-lsp's workspaces); the reader keeps the KeepIndexes used
	// last and removes the others before it starts the server.
	Indexes     string
	KeepIndexes int
}

// Servers are the language servers the reader runs, found on the PATH.
var Servers = []Server{
	{Name: "gopls", Command: "gopls", Languages: []string{"go"}, RootMarkers: []string{"go.mod"},
		Install: "go install golang.org/x/tools/gopls@latest"},
	// jdtls reads Java only: Kotlin has kotlin-lsp, below
	{Name: "jdtls", Command: "jdtls", Languages: []string{"java"}, OnePerWindow: true, DataFlag: "-data",
		Install: "brew install jdtls", ReadyStatus: "ServiceReady",
		// a name declared in a jar (another repository of the big workspace, as a dependency) answers with a
		// jdt:// location, which the reader maps to that repository's source when it is open
		Options: map[string]any{"extendedClientCapabilities": map[string]any{"classFileContentsSupport": true}}},
	// JetBrains' server, still a preview. It imports each Gradle build first (minutes, cold) and
	// answers with nothing meanwhile, telling its work by $/progress; a name in a jar answers
	// jar:///…!/package/Class.class. Its index goes in the system path; Gradle's import writes
	// .gradle/ and build/ in the project, as a build does. It does not say it follows workspace
	// folders: it reads the projects open when it starts.
	{Name: "kotlin-lsp", Command: "kotlin-lsp", Arguments: []string{"--stdio"}, DataFlag: "--system-path",
		Languages: []string{"kotlin"}, OnePerWindow: true, Install: "brew install JetBrains/utils/kotlin-lsp",
		// an index per set of projects, about 5 GB for the big workspace's 44: keep two
		Indexes: "workspaces", KeepIndexes: 2},
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
