package sdk

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/akrck02/coddy/sdk/jsonrpc"
	"github.com/akrck02/go-renderer/failure"
)

// RunAction runs one of the manifest's actions. Its error is shown to the person.
type RunAction func(params ActionParams) (ActionResult, error)

// Initialize is given the values of the plugin's settings, by their ids, once its program starts
// and before any other request: a string, or a boolean for a "boolean" setting. A plugin keeps
// what it needs of them; it is started again when one changes.
type Initialize func(settings map[string]any)

// Handlers are what a plugin answers: each is left nil when its manifest does not contribute it.
type Handlers struct {
	// Initialize, when set, is given the plugin's settings as it starts.
	Initialize     Initialize
	RunAction      RunAction
	VersionControl VersionControl
	ReviewSource   ReviewSource
	DrawMode       DrawMode
}

// Serve answers Coddy over the program's standard input and output until Coddy closes them, then
// returns nil. The program must write nothing else to its standard output: logs go to standard
// error.
func Serve(handlers Handlers) error { return ServeOver(os.Stdin, os.Stdout, handlers) }

// ServeOver is Serve over any reader and writer, for tests. Requests are answered one at a time,
// so a plugin's handlers never run at once and need no locks of their own.
func ServeOver(reader io.Reader, writer io.Writer, handlers Handlers) error {
	var oneAtATime sync.Mutex
	conn := jsonrpc.NewConn(reader, writer, func(method string, params json.RawMessage) (any, error) {
		oneAtATime.Lock()
		defer oneAtATime.Unlock()
		return resultOf(handlers, method, params)
	})
	<-conn.Done()
	if errors.Is(conn.Err(), io.EOF) {
		return nil // Coddy closed the connection between messages: it is done with the plugin
	}
	return conn.Err()
}

// resultOf answers a method: its result, or an error, UnknownMethod for one it does not answer.
func resultOf(handlers Handlers, method string, params json.RawMessage) (any, error) {
	switch {
	case method == InitializeMethod:
		return initialized(handlers.Initialize, params)
	case method == RunActionMethod && handlers.RunAction != nil:
		return answered(method, params, handlers.RunAction)
	case method == ModeFrameMethod && handlers.DrawMode != nil:
		return answered(method, params, handlers.DrawMode)
	case handlers.ReviewSource != nil && (method == ReviewListMethod || method == ReviewCommentsMethod):
		return reviewResult(handlers.ReviewSource, method, params)
	case handlers.VersionControl != nil:
		return versionControlResult(handlers.VersionControl, method, params)
	}
	return nil, failure.Wrap(jsonrpc.UnknownMethod, method, nil)
}

// initialized answers initialize, giving the settings it carries to the plugin's Initialize.
// Parameters that cannot be read are answered all the same, with no settings: the version
// answered is what tells Coddy whether the plugin speaks its protocol.
func initialized(initialize Initialize, params json.RawMessage) (any, error) {
	if initialize != nil {
		var asked InitializeParams
		_ = json.Unmarshal(params, &asked) // see above: an initialize that cannot be read has no settings
		initialize(asked.Settings)
	}
	return InitializeResult{ProtocolVersion: ProtocolVersion}, nil
}

// versionControlResult answers a method of the version control.
func versionControlResult(control VersionControl, method string, params json.RawMessage) (any, error) {
	switch method {
	case BranchMethod:
		return answered(method, params, func(asked FolderParams) (string, error) { return control.Branch(asked.Folder) })
	case DefaultBaseMethod:
		return answered(method, params, func(asked FolderParams) (string, error) { return control.DefaultBase(asked.Folder) })
	case FilesMethod:
		return answered(method, params, func(asked FolderParams) ([]string, error) { return control.Files(asked.Folder) })
	case ChangesMethod:
		return answered(method, params, control.Changes)
	case ReadMethod:
		return answered(method, params, control.Read)
	case DiffMethod:
		return answered(method, params, control.Diff)
	}
	return nil, failure.Wrap(jsonrpc.UnknownMethod, method, nil)
}

// reviewResult answers a method of the review source.
func reviewResult(source ReviewSource, method string, params json.RawMessage) (any, error) {
	if method == ReviewListMethod {
		return answered(method, params, func(asked FolderParams) ([]Review, error) { return source.List(asked.Folder) })
	}
	return answered(method, params, source.Comments)
}

// answered reads a request's parameters and gives what handle answers.
func answered[Asked, Result any](method string, params json.RawMessage, handle func(Asked) (Result, error)) (any, error) {
	var asked Asked
	if err := json.Unmarshal(params, &asked); err != nil {
		return nil, failure.Wrap(jsonrpc.BadMessage, method, err)
	}
	return handle(asked)
}
