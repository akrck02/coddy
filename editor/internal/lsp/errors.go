package lsp

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// ServerNotFound is a language server that is not installed: its command is not on the PATH.
	ServerNotFound = failure.Define("lsp.server-not-found", "the language server is not installed")
	// BadModel is a model of the projects, exported by the server, that could not be read or kept.
	BadModel = failure.Define("lsp.bad-model", "the model of the projects could not be kept")
)
