package lsp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/akrck02/code-review/internal/language"
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

// Servers are the language servers the reader runs, found on the PATH.
var Servers = []Server{
	{Name: "gopls", Command: "gopls", Languages: language.ServerLanguages(language.Go), RootMarkers: []string{"go.mod"},
		Install: "go install golang.org/x/tools/gopls@latest",
		// the latest gopls may need a newer Go than the one installed, which a Linux's own Go
		// (GOTOOLCHAIN=local) would not download
		InstallEnvironment: []string{"GOTOOLCHAIN=auto"}},
	// jdtls reads Java only: Kotlin has kotlin-lsp, below. It writes nothing in the repositories: .project, .classpath and .settings go in its data
	// folder (a JVM property, read before the setting is), and no build fills bin/ or target/; a
	// build file changed is not imported again on its own.
	{Name: "jdtls", Command: "jdtls", Languages: language.ServerLanguages(language.Java), OnePerWindow: true, DataFlag: "-data", FoldersInOptions: true,
		Arguments: []string{"--jvm-arg=-Djava.import.generatesMetadataFilesAtProjectRoot=false"},
		Install:   "brew install jdtls", Ready: Readiness{Status: "ServiceReady"},
		// every Java project of the window from the start: jdtls drops from its workspace the
		// projects it was not started with, and imports each one added later with Gradle again
		ReadsAll: true,
		// a name declared in a jar (another repository, as a dependency) answers with a
		// jdt:// location, which the reader maps to that repository's source when it is open
		Options: map[string]any{"extendedClientCapabilities": map[string]any{"classFileContentsSupport": true}},
		Settings: map[string]any{"java": map[string]any{
			"import":        map[string]any{"generatesMetadataFilesAtProjectRoot": false},
			"autobuild":     map[string]any{"enabled": false},
			"configuration": map[string]any{"updateBuildConfiguration": "disabled"},
		}},
		// its Gradle import (Buildship) honours GRADLE_USER_HOME: its daemons time out, as kotlin-lsp's
		Memory: Memory{Most: "2g", Flag: "--jvm-arg=-Xmx"}, Import: Import{OwnGradle: true}},
	// JetBrains' server, still a preview. It imports each Gradle build first (minutes, cold) and
	// answers with nothing meanwhile, telling its work by $/progress; a name in a jar answers
	// jar:///…!/package/Class.class. Gradle's import writes .gradle/ and build/ in the project, as
	// a build does. It follows workspace folders added and removed.
	{Name: "kotlin-lsp", Command: "kotlin-lsp", Arguments: []string{"--stdio"}, DataFlag: "--system-path",
		Languages: language.ServerLanguages(language.Kotlin), OnePerWindow: true, Install: "brew install JetBrains/utils/kotlin-lsp",
		// up to 263 it built an index per set of projects it started with, in the system path's
		// workspaces/; since 271 that went to the person's caches, ~/Library/Caches/JetBrains/
		// IntelliJServer/workspaces, one per set again, 12 GB in a day. Given indexDir it keeps its
		// database there. One database for every set does not do: it keeps the projects' model with
		// paths by the place of their folder among those it started with ($WORKSPACE_7$/src/main),
		// and started with fewer folders it cannot read them and hangs, answering nothing. So one
		// per set, in indexes/, the last kept; rocks/ was the one for every set
		Index: Index{Folder: "indexes", Keep: 1, Option: "indexDir", Retired: []string{"rocks"}, NeedsFree: 10 << 30}, ReadsAll: true,
		Import: Import{Models: true, OwnGradle: true, After: []string{"jdtls"}}, Ready: Readiness{AfterWork: true},
		// no -Dcom.jetbrains.ls.imports.gradle.daemon.no.idle.timeout: it gives each daemon an idle
		// time of 1 ms, so none was used twice (148 times on the big workspace); the reader stops its daemons
		// itself once the import is done (gradle.go)
		Memory: Memory{Most: "2g", Variable: "IJ_JAVA_OPTIONS"}},
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
