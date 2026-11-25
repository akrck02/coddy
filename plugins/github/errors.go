package main

import "github.com/akrck02/go-renderer/failure"

// The codes of the plugin's errors.
var (
	// CommandFailed is a gh command that failed: gh is not installed, not logged in, or the
	// folder is not a repository of GitHub.
	CommandFailed = failure.Define("github.command-failed", "gh could not read the repository")
	// UnreadableAnswer is an answer of gh that is not the JSON asked for.
	UnreadableAnswer = failure.Define("github.unreadable-answer", "gh answered in a form this plugin does not know")
)
