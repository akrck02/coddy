package jsonrpc

import "github.com/akrck02/go-renderer/failure"

// The codes of the errors this package returns. Check them with errors.Is or failure.Is.
var (
	// Stopped is a connection that closed, or another program that ended, before answering.
	Stopped = failure.Define("jsonrpc.stopped", "the other program stopped")
	// RequestFailed is a request the other side answered with an error.
	RequestFailed = failure.Define("jsonrpc.request-failed", "the other program could not answer")
	// BadMessage is a message that does not follow the protocol: its header, its JSON, or the
	// parameters a handler expected.
	BadMessage = failure.Define("jsonrpc.bad-message", "a message could not be read")
	// MessageTooLarge is a message whose Content-Length is above largestMessage: it is refused
	// before its body is read.
	MessageTooLarge = failure.Define("jsonrpc.message-too-large", "a message is too large")
	// UnknownMethod is a request a handler does not answer.
	UnknownMethod = failure.Define("jsonrpc.unknown-method", "the request is not answered here")
)
