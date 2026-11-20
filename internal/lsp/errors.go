package lsp

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// ServerNotFound is a language server that is not installed: its command is not on the PATH.
	ServerNotFound = failure.Define("lsp.server-not-found", "the language server is not installed")
	// ServerStopped is a server that ended, or a connection closed, before answering.
	ServerStopped = failure.Define("lsp.server-stopped", "the language server stopped")
	// RequestFailed is a request the server answered with an error.
	RequestFailed = failure.Define("lsp.request-failed", "the language server could not answer")
	// BadMessage is a message that does not follow the protocol: its header or its JSON.
	BadMessage = failure.Define("lsp.bad-message", "a message from the language server could not be read")
	// BadModel is a model of the projects, exported by the server, that could not be read or kept.
	BadModel = failure.Define("lsp.bad-model", "the model of the projects could not be kept")
)
