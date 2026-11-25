package lsp

// officialServers are the servers of Coddy's official plugins (plugins/go, java and kotlin), as
// their plugin.json declares them, for the tests that run them or read their settings.
var officialServers = []Server{
	{Name: "gopls", Command: "gopls", Languages: []string{"go"}, RootMarkers: []string{"go.mod"},
		Install: "go install golang.org/x/tools/gopls@latest", InstallEnvironment: []string{"GOTOOLCHAIN=auto"}},
	{Name: "jdtls", Command: "jdtls", Languages: []string{"java"}, OnePerWindow: true, DataFlag: "-data", FoldersInOptions: true,
		Arguments: []string{"--jvm-arg=-Djava.import.generatesMetadataFilesAtProjectRoot=false"},
		Install:   "brew install jdtls", Ready: Readiness{Status: "ServiceReady"}, ReadsAll: true,
		Options: map[string]any{"extendedClientCapabilities": map[string]any{"classFileContentsSupport": true}},
		Settings: map[string]any{"java": map[string]any{
			"import":        map[string]any{"generatesMetadataFilesAtProjectRoot": false},
			"autobuild":     map[string]any{"enabled": false},
			"configuration": map[string]any{"updateBuildConfiguration": "disabled"},
		}},
		Memory: Memory{Most: "2g", Flag: "--jvm-arg=-Xmx"}, Import: Import{OwnGradle: true}},
	{Name: "kotlin-lsp", Command: "kotlin-lsp", Arguments: []string{"--stdio"}, DataFlag: "--system-path",
		Languages: []string{"kotlin"}, OnePerWindow: true, Install: "brew install JetBrains/utils/kotlin-lsp",
		Index: Index{Folder: "indexes", Keep: 1, Option: "indexDir", Retired: []string{"rocks"}, NeedsFree: 10 << 30}, ReadsAll: true,
		Import: Import{Models: true, OwnGradle: true, After: []string{"jdtls"}}, Ready: Readiness{AfterWork: true},
		Memory: Memory{Most: "2g", Variable: "IJ_JAVA_OPTIONS"}},
}
