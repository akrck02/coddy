package keyring

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// NotAvailable is a system whose keyring Coddy cannot reach: one it does not know yet
	// (Windows), or one without the keyring's program installed (secret-tool on Linux).
	NotAvailable = failure.Define("keyring.not-available",
		"there is no keyring Coddy can keep secrets in on this computer (on Linux, install secret-tool, from libsecret)")
	// NotFound is a secret the keyring does not keep.
	NotFound = failure.Define("keyring.not-found", "the keyring keeps no such secret")
	// Failed is a keyring that answered with an error: locked, a prompt dismissed, no keyring
	// service running.
	Failed = failure.Define("keyring.failed", "the keyring could not do what was asked")
)
