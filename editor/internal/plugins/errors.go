package plugins

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// BadManifest is a plugin.json that could not be read, or says too little to use.
	BadManifest = failure.Define("plugins.bad-manifest", "the plugin's plugin.json could not be read")
	// NotStarted is a plugin whose program could not be started.
	NotStarted = failure.Define("plugins.not-started", "the plugin's program could not be started")
	// Crashed is a plugin whose program ended, or stopped answering in the protocol, while it was
	// used. It starts again the next time it is used.
	Crashed = failure.Define("plugins.crashed", "the plugin stopped")
	// Timeout is a plugin that did not answer in time.
	Timeout = failure.Define("plugins.timeout", "the plugin did not answer in time")
	// BadSettings is a file of the values set for a plugin that could not be read: the plugin
	// takes its defaults until a value is set again.
	BadSettings = failure.Define("plugins.bad-settings", "the plugin's settings could not be read")
	// SettingsNotSaved is a value set for a plugin that could not be written to its file.
	SettingsNotSaved = failure.Define("plugins.settings-not-saved", "the plugin's settings could not be saved")
	// ProtocolVersion is a plugin that speaks another version of the protocol than Coddy.
	ProtocolVersion = failure.Define("plugins.protocol-version", "the plugin speaks another version of the protocol")
)
