package plugins

import (
	_ "embed" // the logos of the built-in plugins
	"slices"
)

// gitLogo is git's logo, from Simple Icons (https://simpleicons.org, CC0), in git's orange. The
// logo remains a trademark of its owner, used only to name what the plugin reads.
//
//go:embed pictures/git.svg
var gitLogo []byte

// GitName is the name of the built-in plugin that reads git repositories.
const GitName = "git"

// BuiltIn are the plugins compiled into Coddy, listed and turned on or off as the plugins
// installed are, but run inside it: git reads repositories with no program of a plugin between.
// Each is on unless named in off.
func BuiltIn(off []string) []*Plugin {
	git := &Plugin{BuiltIn: true, Manifest: Manifest{Name: GitName, Author: coddyAuthor, Icon: "git-branch",
		Description: "Reads git repositories: their branches, their changes and their history.",
		Requires: []Requirement{{Program: "git", Install: map[string][]string{
			"linux":   {"sudo dnf install git", "sudo apt install git", "sudo pacman -S git"},
			"darwin":  {"xcode-select --install", "brew install git"},
			"windows": {"winget install --id Git.Git --exact"},
		}}},
		Contributes: Contributions{VersionControl: &VersionControlContribution{Name: "Git", Marker: ".git"}}}}
	git.Badge = badgeOf(git.Name, git.Icon, "")
	git.Badge.Source = gitLogo
	git.Turn(!slices.Contains(off, GitName))
	git.CheckRequirements()
	git.Verified = isVerified(git)
	return []*Plugin{git}
}

// GitOff tells whether the built-in git plugin is among those given and not working, turned off or
// without the git program: folders are then not read as git repositories.
func GitOff(listed []*Plugin) bool {
	for _, plugin := range listed {
		if plugin.BuiltIn && plugin.Name == GitName {
			return !plugin.Ready()
		}
	}
	return false
}
