package main

import "github.com/akrck02/go-renderer/failure"

// The codes of the plugin's errors. Their words reach the warning on the plugin's card in Coddy,
// so each says what to do.
var (
	// NoToken is a plugin with no token to read Azure DevOps with: none in its settings, none in
	// the environment.
	NoToken = failure.Define("azure-devops.no-token",
		"no token to read Azure DevOps with: set the token in the plugin's settings, or AZURE_DEVOPS_EXT_PAT")
	// Unauthorized is a token Azure DevOps refused: wrong, expired, or without the right to read
	// code. Azure answers such a token with 401, or with 203 and its sign-in page.
	Unauthorized = failure.Define("azure-devops.unauthorized",
		"Azure DevOps refused the token: set one that may read code (Code: Read) in the plugin's settings, or AZURE_DEVOPS_EXT_PAT")
	// NotAzure is a folder whose remote origin is not a repository of Azure DevOps, in any of the
	// forms the plugin reads.
	NotAzure = failure.Define("azure-devops.not-azure",
		"the remote origin is not a repository of Azure DevOps; for a server of your own, set its organization URL in the plugin's settings")
	// RequestFailed is any other failure to read Azure DevOps: no answer, an answer that is not
	// the JSON asked for, a status that is not 200.
	RequestFailed = failure.Define("azure-devops.request-failed", "Azure DevOps could not be read")
)
