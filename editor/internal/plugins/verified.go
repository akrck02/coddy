package plugins

import "slices"

// coddyAuthor is the author Coddy's own plugins give.
const coddyAuthor = "Coddy"

// TODO(akrck02): a name and an author can be copied by anyone; verify a plugin by a signature of
// its folder once plugins are shared beyond Coddy's own repository.

// coddyPlugins are the plugins in Coddy's own repository, in plugins/: the official ones and the
// examples.
var coddyPlugins = []string{"go", "java", "kotlin", "github", "azure-devops", "python"}

// isVerified tells whether a plugin is one of Coddy's own: built in, or from its repository and
// saying so.
func isVerified(plugin *Plugin) bool {
	return plugin.BuiltIn || (plugin.Author == coddyAuthor && slices.Contains(coddyPlugins, plugin.Name))
}
