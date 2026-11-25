package main

import (
	"testing"

	"github.com/akrck02/coddy/editor/internal/plugins"
)

// A plugin on that misses a program it requires is off: its server would only fail to start.
func TestOnlyTheLanguageServersOfThePluginsOnAndMissingNothingRun(test *testing.T) {
	declaring := func(name string) *plugins.Plugin {
		return &plugins.Plugin{Manifest: plugins.Manifest{Name: name, Command: []string{name + "-server"}, Speaks: plugins.SpeaksLanguageServer,
			Contributes: plugins.Contributions{LanguageServer: &plugins.LanguageServer{Languages: []string{"python"}}}}}
	}
	on, off, missing := declaring("on"), declaring("off"), declaring("missing")
	on.Turn(true)
	missing.Turn(true)
	missing.Requires = []plugins.Requirement{{Program: "no-such-wick-here"}}
	missing.CheckRequirements()
	servers := serversFor("", "", 0, []*plugins.Plugin{on, off, missing})
	if len(servers) != 1 || servers[0].Name != "on-server" {
		test.Errorf("the servers are %+v, want only the server of the plugin on", servers)
	}
	if capture := serversFor("frame.png", "", 0, []*plugins.Plugin{on}); len(capture) != 0 {
		test.Errorf("a capture runs %d servers, want none", len(capture))
	}
}

// Only the plugins turned off are kept, so a plugin installed later starts on.
func TestOnlyThePluginsOffAreRememberedForTheNextTime(test *testing.T) {
	first, second, third := &plugins.Plugin{}, &plugins.Plugin{}, &plugins.Plugin{}
	first.Name, second.Name, third.Name = "export-pdf", "python-lsp", "spell"
	first.Turn(true)
	third.Turn(true)
	if kept := pluginsOff([]*plugins.Plugin{first, second, third}); kept != "python-lsp" {
		test.Errorf("the plugins kept off are %q", kept)
	}
}

func TestTheFlagsSetTheMemoryAndIndexesOfThePluginServers(test *testing.T) {
	kotlin := &plugins.Plugin{Manifest: plugins.Manifest{Name: "kotlin", Command: []string{"kotlin-lsp"}, Speaks: plugins.SpeaksLanguageServer,
		Contributes: plugins.Contributions{LanguageServer: &plugins.LanguageServer{Languages: []string{"kotlin"},
			Memory: plugins.ServerMemory{Most: "2g"}, Index: plugins.ServerIndex{Folder: "indexes", Keep: 1}}}}}
	kotlin.Turn(true)
	servers := serversFor("", "6g", 3, []*plugins.Plugin{kotlin})
	if len(servers) != 1 || servers[0].Memory.Most != "6g" || servers[0].Index.Keep != 3 {
		test.Errorf("the servers are %+v, want kotlin-lsp with 6g and 3 indexes", servers)
	}
}
